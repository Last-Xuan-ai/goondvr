package chaturbate

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/HeapOfChaos/goondvr/internal"
	"github.com/grafov/m3u8"
)

const mediaStallTimeout = 45 * time.Second

type trackProgress struct {
	lastWrite                 time.Time
	segments, bytes, failures int64
	mediaSeconds              float64
	lastFailureLog            time.Time
	lastError                 string
	playlistSeen              bool
	mediaSequence             uint64
}

// streamProgress watches writes, not HTTP successes. A healthy audio track must
// not keep a dead video track (or vice versa) looking like a working recording.
type streamProgress struct {
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelCauseFunc
	timer      *time.Timer
	timeout    time.Duration
	tracks     map[string]*trackProgress
	order      []string
	lastReport time.Time
	logf       func(string, ...any)
}

func (p *Playlist) diagnostic(format string, args ...any) {
	if p.Logf != nil {
		p.Logf(format, args...)
	} else {
		log.Printf("HLS "+format, args...)
	}
}

func newStreamProgress(ctx context.Context, timeout time.Duration, separateAudio bool, logf func(string, ...any)) *streamProgress {
	ctx, cancel := context.WithCancelCause(ctx)
	now := time.Now()
	g := &streamProgress{
		ctx: ctx, cancel: cancel, timeout: timeout, logf: logf,
		tracks: make(map[string]*trackProgress), order: []string{"video"}, lastReport: now,
	}
	if separateAudio {
		g.order = append(g.order, "audio")
	}
	for _, name := range g.order {
		g.tracks[name] = &trackProgress{lastWrite: now}
	}
	g.timer = time.AfterFunc(timeout, g.check)
	return g
}

func (g *streamProgress) check() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ctx.Err() != nil {
		return
	}
	now := time.Now()
	next := g.timeout
	for _, name := range g.order {
		t := g.tracks[name]
		remaining := g.timeout - now.Sub(t.lastWrite)
		if remaining <= 0 {
			g.cancel(fmt.Errorf("%w: track=%s no_media_for=%s written_segments=%d failed_downloads=%d media_sequence=%d last_error=%s",
				internal.ErrStreamStalled, name, now.Sub(t.lastWrite).Round(time.Second), t.segments, t.failures, t.mediaSequence, t.lastError))
			return
		}
		if remaining < next {
			next = remaining
		}
	}
	g.timer.Reset(next)
}

func (g *streamProgress) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancel(context.Canceled)
	g.timer.Stop()
}

func (g *streamProgress) written(track string, size int, duration float64) {
	if size == 0 {
		return
	}
	g.mu.Lock()
	t := g.tracks[track]
	t.lastWrite = time.Now()
	t.segments++
	t.bytes += int64(size)
	t.mediaSeconds += duration
	var report string
	if time.Since(g.lastReport) >= time.Minute {
		g.lastReport = time.Now()
		for _, name := range g.order {
			s := g.tracks[name]
			report += fmt.Sprintf(" track=%s segments=%d bytes=%d media_seconds=%.1f idle=%s failed_downloads=%d",
				name, s.segments, s.bytes, s.mediaSeconds, time.Since(s.lastWrite).Round(time.Second), s.failures)
		}
	}
	g.mu.Unlock()
	if report != "" {
		g.logf("recording progress:%s", report)
	}
}

func (g *streamProgress) failed(track string, seq int, err error) {
	g.problem(track, "segment", seq, err)
}

func (g *streamProgress) problem(track, resource string, seq int, err error) {
	g.mu.Lock()
	t := g.tracks[track]
	t.failures++
	t.lastError = internal.RedactLogMessage(err.Error())
	report := t.lastFailureLog.IsZero() || time.Since(t.lastFailureLog) >= 30*time.Second
	if report {
		t.lastFailureLog = time.Now()
	}
	count, message := t.failures, t.lastError
	g.mu.Unlock()
	if report && g.ctx.Err() == nil {
		g.logf("%s download failed: track=%s sequence=%d failed_downloads=%d error=%s", resource, track, seq, count, message)
	}
}

func (g *streamProgress) playlist(track string, p *m3u8.MediaPlaylist) {
	g.mu.Lock()
	t := g.tracks[track]
	old := t.mediaSequence
	regressed := t.playlistSeen && p.SeqNo < old
	t.mediaSequence, t.playlistSeen = p.SeqNo, true
	g.mu.Unlock()
	if regressed {
		g.logf("playlist sequence moved backwards: track=%s previous=%d current=%d; stalled writes will trigger a fresh stream lookup", track, old, p.SeqNo)
	}
}

func waitForPoll(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}

func (p *Playlist) pollDelay() time.Duration {
	if p.testPollDelay > 0 {
		return p.testPollDelay
	}
	return time.Second
}

// SeqId is assigned by the playlist decoder even for sequence zero or URIs
// without a numeric suffix. Using it for the fallback avoids downloading the
// same multi-segment window again on every poll.
func mediaSegmentSequence(s *m3u8.MediaSegment) int {
	if seq := internal.SegmentSeq(s.URI); seq >= 0 {
		return seq
	}
	return int(s.SeqId)
}
