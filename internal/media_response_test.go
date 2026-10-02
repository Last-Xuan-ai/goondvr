package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/server"
)

func TestMediaErrorsAreNotWrittenAsVideo(t *testing.T) {
	old := server.Config
	server.Config = &entity.Config{}
	t.Cleanup(func() { server.Config = old })
	for _, tc := range []struct {
		name              string
		code              int
		contentType, body string
		wantError         bool
	}{
		{"unavailable", 503, "text/plain", "temporarily unavailable", true},
		{"empty", 200, "video/mp4", "", true},
		{"html", 200, "text/html", "<html>gateway failure</html>", true},
		{"media", 200, "video/mp4", "media bytes", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()
			b, err := NewMediaReq().GetBytes(context.Background(), ts.URL)
			if (err != nil) != tc.wantError {
				t.Fatalf("err=%v want error=%v", err, tc.wantError)
			}
			if tc.wantError && len(b) > 0 {
				t.Fatal("error body escaped to recording pipeline")
			}
		})
	}
}
