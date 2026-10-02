package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDiagnosticLogsRedactSecrets(t *testing.T) {
	input := "download https://user:password@cdn.example/token-in-path/video?key=secret#fragment failed\nCookie: session=secret\nAuthorization: Bearer access-token\n"
	out := RedactLogMessage(input)
	for _, secret := range []string{"password", "token-in-path", "key=secret", "session=secret", "access-token", "fragment"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret leaked: %s", secret)
		}
	}
	if !strings.Contains(out, "cdn.example") {
		t.Fatal("lost useful CDN hostname")
	}
}

func TestDiagnosticLogRotatesAndReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "goondvr.log")
	var console bytes.Buffer
	w, err := newDiagnosticWriter(path, 100, 3, &console)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		if _, err := fmt.Fprintf(w, "event=%02d recording stalled on video\n", i); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.file.Close()
	files, err := filepath.Glob(path + "*")
	if err != nil || len(files) != 4 {
		t.Fatalf("unbounded backups: %v, %v", files, err)
	}
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil || st.Size() > 100 {
			t.Fatalf("bad log size for %s", f)
		}
		if st.Mode().Perm()&0077 != 0 {
			t.Fatalf("log is readable by other users: %s", f)
		}
	}
	before, _ := os.ReadFile(path)
	w2, err := newDiagnosticWriter(path, 100, 3, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.file.Close()
	_, _ = w2.Write([]byte("restarted\n"))
	after, _ := os.ReadFile(path)
	if !bytes.Contains(after, before) || !bytes.Contains(after, []byte("restarted")) {
		t.Fatal("restart truncated log")
	}
}

func TestDiagnosticLogConcurrentWritersAndDiskFailure(t *testing.T) {
	var console bytes.Buffer
	w, err := newDiagnosticWriter(filepath.Join(t.TempDir(), "goondvr.log"), 100000, 3, &console)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = w.Write([]byte("channel event\n"))
			}
		}()
	}
	wg.Wait()
	_ = w.file.Close()
	// A closed/full/unavailable diagnostic file must not block recording.
	for i := 0; i < 2; i++ {
		if n, err := w.Write([]byte("still recording\n")); err != nil || n != 16 {
			t.Fatalf("log error propagated: %d %v", n, err)
		}
	}
	if strings.Count(console.String(), "WARN diagnostic log unavailable") != 1 {
		t.Fatal("disk failure warning must be emitted once")
	}
}
