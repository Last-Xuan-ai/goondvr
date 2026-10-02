package internal

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	logURL        = regexp.MustCompile("(?i)https?://[^\\s<>\"'`]+")
	logCredential = regexp.MustCompile(`(?im)\b(?:cookie|set-cookie|authorization|proxy-authorization)\s*[:=][^\r\n]*`)
)

// RedactLogMessage keeps CDN hostnames useful for diagnostics, but drops signed
// URL paths, query strings, userinfo and credential headers from ordinary logs.
// Deliberate --debug HTTP dumps are separate and should never be shared as-is.
func RedactLogMessage(message string) string {
	message = logURL.ReplaceAllStringFunc(message, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return "[redacted URL]"
		}
		return u.Scheme + "://" + u.Hostname() + "/[redacted]"
	})
	return logCredential.ReplaceAllStringFunc(message, func(raw string) string {
		i := strings.IndexAny(raw, ":=")
		return raw[:i+1] + " [redacted]"
	})
}

type diagnosticWriter struct {
	mu          sync.Mutex
	path        string
	file        *os.File
	size, limit int64
	backups     int
	console     io.Writer
	warned      bool
}

// EnableDiagnosticLog tees standard log events to a bounded, private local log.
// Errors opening the initial log are returned to the caller; later disk failures
// are reported to the console and must never interrupt an active recording.
func EnableDiagnosticLog(path string) (func(), error) {
	previous := log.Writer()
	w, err := newDiagnosticWriter(path, 5*1024*1024, 3, previous)
	if err != nil {
		return nil, err
	}
	log.SetOutput(w)
	return func() {
		log.SetOutput(previous)
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.file != nil {
			_ = w.file.Close()
			w.file = nil
		}
	}, nil
}

func newDiagnosticWriter(path string, limit int64, backups int, console io.Writer) (*diagnosticWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	w := &diagnosticWriter{path: path, limit: limit, backups: backups, console: console}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *diagnosticWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file, w.size = f, st.Size()
	return nil
}

func (w *diagnosticWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	w.file = nil
	oldest := fmt.Sprintf("%s.%d", w.path, w.backups)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := w.backups - 1; i >= 1; i-- {
		if err := os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		return err
	}
	return w.open()
}

func (w *diagnosticWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	text := RedactLogMessage(string(p))
	if len(text) > 16*1024 {
		text = text[:16*1024] + " [log entry truncated]\n"
	}
	_, _ = io.WriteString(w.console, text)
	if w.file == nil {
		return len(p), nil
	}
	var err error
	if w.size > 0 && w.size+int64(len(text)) > w.limit {
		err = w.rotate()
	}
	if err == nil {
		var n int
		n, err = io.WriteString(w.file, text)
		w.size += int64(n)
	}
	if err != nil && !w.warned {
		w.warned = true
		fmt.Fprintf(w.console, "WARN diagnostic log unavailable: %v; recording continues with console logs\n", err)
	}
	return len(p), nil
}
