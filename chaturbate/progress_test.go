package chaturbate

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/internal"
	"github.com/HeapOfChaos/goondvr/server"
)

func setupProgressTest(t *testing.T) {
	t.Helper()
	old := server.Config
	server.Config = &entity.Config{}
	t.Cleanup(func() { server.Config = old })
}

func testPlaylist(base string) *Playlist {
	return &Playlist{
		PlaylistURL: base + "/index.m3u8", RootURL: base + "/",
		testPollDelay: 10 * time.Millisecond, testStallTimeout: 250 * time.Millisecond,
		Logf: func(string, ...any) {},
	}
}

func TestStalledStreamsReturnForRediscovery(t *testing.T) {
	setupProgressTest(t)
	for _, scenario := range []string{"sequence-reset", "stale-playlist", "segment-404", "empty-playlist"} {
		t.Run(scenario, func(t *testing.T) {
			polls, writes := 0, 0
			var messages []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/index.m3u8" {
					polls++
					seq := 1000
					if scenario == "sequence-reset" && polls > 1 {
						seq = polls - 1
					} else if scenario == "segment-404" {
						seq += polls
					}
					fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:%d\n", seq)
					if scenario != "empty-playlist" {
						fmt.Fprintf(w, "#EXTINF:2,\nchunk_%d.ts\n", seq)
					}
					return
				}
				if scenario == "segment-404" {
					w.WriteHeader(http.StatusNotFound)
				} else {
					fmt.Fprint(w, "media")
				}
			}))
			defer ts.Close()
			p := testPlaylist(ts.URL)
			p.Logf = func(f string, args ...any) { messages = append(messages, fmt.Sprintf(f, args...)) }
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := p.WatchSegments(ctx, func([]byte, float64) error { writes++; return nil })
			if !errors.Is(err, internal.ErrStreamStalled) || ctx.Err() != nil {
				t.Fatalf("must recover without waiting for caller cancellation: %v", err)
			}
			wantWrites := 1
			if scenario == "segment-404" || scenario == "empty-playlist" {
				wantWrites = 0
			}
			if writes != wantWrites || polls < 2 {
				t.Fatalf("polls=%d writes=%d, want writes=%d", polls, writes, wantWrites)
			}
			if scenario == "sequence-reset" && !strings.Contains(strings.Join(messages, "\n"), "moved backwards") {
				t.Fatal("sequence regression not logged")
			}
			if scenario == "segment-404" && !strings.Contains(strings.Join(messages, "\n"), "404") {
				t.Fatal("segment failure not logged")
			}
		})
	}
}

func TestTransientSegmentFailureRetriedOnNextPoll(t *testing.T) {
	setupProgressTest(t)
	requests, writes := 0, 0
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.m3u8" {
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:2,\nchunk_0.ts\n")
			return
		}
		requests++
		if requests <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "not media")
		} else {
			fmt.Fprint(w, "real media")
		}
	}))
	defer ts.Close()
	p := testPlaylist(ts.URL)
	p.testStallTimeout = 3 * time.Second
	err := p.WatchSegments(ctx, func(b []byte, _ float64) error {
		if string(b) != "real media" {
			t.Errorf("HTTP error body written as media: %q", b)
		}
		writes++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || writes != 1 || requests != 4 {
		t.Fatalf("failed segment was lost: writes=%d requests=%d err=%v", writes, requests, err)
	}
}

func TestSequenceZeroAndOpaqueURIsAreNotRepeated(t *testing.T) {
	setupProgressTest(t)
	writes := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.m3u8" {
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:2,\nalpha.mp4\n#EXTINF:2,\nbeta.mp4\n")
		} else {
			fmt.Fprint(w, "media")
		}
	}))
	defer ts.Close()
	err := testPlaylist(ts.URL).WatchSegments(context.Background(), func([]byte, float64) error { writes++; return nil })
	if !errors.Is(err, internal.ErrStreamStalled) || writes != 2 {
		t.Fatalf("overlapping window replayed: writes=%d err=%v", writes, err)
	}
}

func TestCancellationInterruptsPlaylistRetryWait(t *testing.T) {
	setupProgressTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		cancel()
	}))
	defer ts.Close()
	p := testPlaylist(ts.URL)
	p.testPollDelay = time.Minute
	start := time.Now()
	err := p.WatchSegments(ctx, func([]byte, float64) error { return nil })
	if !errors.Is(err, context.Canceled) || time.Since(start) > time.Second {
		t.Fatalf("cancellation waited for retries: %v elapsed=%s", err, time.Since(start))
	}
}

func TestProgressDeadlineMovesOnlyAfterMediaWrites(t *testing.T) {
	for _, missing := range []string{"video", "audio"} {
		t.Run(missing, func(t *testing.T) {
			g := newStreamProgress(context.Background(), 180*time.Millisecond, true, func(string, ...any) {})
			defer g.close()
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			deadline := time.After(2 * time.Second)
			for {
				select {
				case <-g.ctx.Done():
					if cause := context.Cause(g.ctx); !errors.Is(cause, internal.ErrStreamStalled) || !strings.Contains(cause.Error(), "track="+missing) {
						t.Fatalf("wrong stalled track: %v", cause)
					}
					return
				case <-ticker.C:
					for _, name := range []string{"video", "audio"} {
						if name != missing {
							g.written(name, 10, 2)
						}
					}
				case <-deadline:
					t.Fatal("healthy track hid stalled track")
				}
			}
		})
	}
}

// Minimal synthetic fMP4 boxes exercise the actual two-track download/mux path.
func progressTestInit() []byte {
	tkhd := make([]byte, 84)
	binary.BigEndian.PutUint32(tkhd[12:], 1)
	mdhd := make([]byte, 24)
	binary.BigEndian.PutUint32(mdhd[12:], 1000)
	trak := makeMP4Box("trak", append(makeMP4Box("tkhd", tkhd), makeMP4Box("mdia", makeMP4Box("mdhd", mdhd))...))
	return makeMP4Box("moov", append(makeMP4Box("mvhd", make([]byte, 100)), trak...))
}

func progressTestSegment() []byte {
	tfhd := make([]byte, 8)
	binary.BigEndian.PutUint32(tfhd[4:], 1)
	tfdt := make([]byte, 8)
	binary.BigEndian.PutUint32(tfdt[4:], 2000)
	traf := makeMP4Box("traf", append(makeMP4Box("tfhd", tfhd), makeMP4Box("tfdt", tfdt)...))
	return append(makeMP4Box("moof", traf), makeMP4Box("mdat", []byte("sample"))...)
}

func TestMuxedStreamsDetectEitherMissingTrack(t *testing.T) {
	setupProgressTest(t)
	for _, stripchatStyle := range []bool{false, true} {
		for _, missing := range []string{"video", "audio"} {
			t.Run(fmt.Sprintf("stripchat=%v/missing=%s", stripchatStyle, missing), func(t *testing.T) {
				var mu sync.Mutex
				polls := map[string]int{}
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					if strings.HasSuffix(r.URL.Path, ".m3u8") {
						track := "video"
						if strings.Contains(r.URL.Path, "audio") {
							track = "audio"
						}
						polls[track]++
						seq := polls[track]
						if track == missing {
							seq = 1
						}
						fmt.Fprintf(w, "#EXTM3U\n#EXT-X-VERSION:6\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:%d\n#EXT-X-MAP:URI=\"/init.mp4\"\n#EXTINF:2,\n/seg_%d.m4s\n", seq, seq)
					} else if r.URL.Path == "/init.mp4" {
						_, _ = w.Write(progressTestInit())
					} else {
						_, _ = w.Write(progressTestSegment())
					}
				}))
				defer ts.Close()
				p := testPlaylist(ts.URL)
				p.AudioPlaylistURL = ts.URL + "/audio.m3u8"
				if stripchatStyle {
					p.PlaylistURL += "?fixture=doppiocdn"
				}
				mediaWrites := 0
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				err := p.WatchSegments(ctx, func(b []byte, _ float64) error {
					if _, ok := extractMoofFirstTfdt(b); ok {
						mediaWrites++
					}
					return nil
				})
				if !errors.Is(err, internal.ErrStreamStalled) || !strings.Contains(err.Error(), "track="+missing) || mediaWrites < 2 {
					t.Fatalf("mux recovery failed: writes=%d err=%v", mediaWrites, err)
				}
			})
		}
	}
}

func TestHealthyStreamDoesNotTripWatchdog(t *testing.T) {
	g := newStreamProgress(context.Background(), 200*time.Millisecond, false, func(string, ...any) {})
	defer g.close()
	started := time.Now()
	for time.Since(started) < 600*time.Millisecond {
		g.written("video", 10, 2)
		time.Sleep(30 * time.Millisecond)
		if g.ctx.Err() != nil {
			t.Fatalf("healthy stream canceled: %v", context.Cause(g.ctx))
		}
	}
}

func TestEndedPlaylistRefreshesAfterWritingFinalSegments(t *testing.T) {
	setupProgressTest(t)
	writes := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.m3u8" {
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nchunk_0.ts\n#EXT-X-ENDLIST\n")
		} else {
			fmt.Fprint(w, "media")
		}
	}))
	defer ts.Close()
	err := testPlaylist(ts.URL).WatchSegments(context.Background(), func([]byte, float64) error { writes++; return nil })
	if writes != 1 || err == nil || !strings.Contains(err.Error(), "playlist ended") || errors.Is(err, internal.ErrStreamStalled) {
		t.Fatalf("end list not handled: writes=%d err=%v", writes, err)
	}
}
