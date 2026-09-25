package stripchat

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/internal"
	"github.com/HeapOfChaos/goondvr/server"
)

// Keep the production URLs and HTTP client in these tests; only route their
// connections to a local TLS server. No live site or browser is involved.
func mockStripchatAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	ts := httptest.NewTLSServer(handler)
	oldTransport, oldConfig := http.DefaultTransport, server.Config
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, ts.Listener.Addr().String())
	}
	transport.Proxy = nil
	http.DefaultTransport = transport
	server.Config = &entity.Config{BrowserMode: "off"}
	t.Cleanup(func() {
		http.DefaultTransport, server.Config = oldTransport, oldConfig
		transport.CloseIdleConnections()
		ts.Close()
	})
}

func TestFetchStreamUsesNumericCamAPIWithoutBrowser(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		t.Run(fmt.Sprint(authenticated), func(t *testing.T) {
			var paths []string
			mockStripchatAPI(t, func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				wantCookie := ""
				if authenticated {
					wantCookie = "sc=example"
				}
				if r.Header.Get("Cookie") != wantCookie {
					t.Errorf("wrong site credentials on %s", r.URL.Path)
				}
				wantUA := internal.DefaultStripchatUserAgent
				if authenticated {
					wantUA = "Browser/test"
				}
				if r.Header.Get("User-Agent") != wantUA {
					t.Errorf("wrong User-Agent on %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/front/users/user-ids/example":
					fmt.Fprint(w, `{"id":123456}`)
				case "/api/front/v2/models/123456/cam":
					fmt.Fprint(w, `{"cam":{"streamName":"123456","isCamActive":true,"topic":"Public room","viewServers":{"flashphoner-hls":"hls-01"}},"user":{"user":{"id":123456,"isOnline":true,"isLive":true,"status":"public"}}}`)
				default:
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
					w.WriteHeader(http.StatusTeapot)
				}
			})
			server.Config.Cookies = "cb=example"
			if authenticated {
				server.Config.StripchatCookies = "sc=example"
				server.Config.StripchatUserAgent = "Browser/test"
			}
			info, err := New().FetchStream(context.Background(), internal.NewReq(), "example")
			if err != nil {
				t.Fatal(err)
			}
			if info.HLSURL != "https://b-hls-01.doppiocdn.com/hls/123456/master_123456.m3u8" {
				t.Fatalf("unexpected HLS URL: %s", info.HLSURL)
			}
			if len(paths) != 2 {
				t.Fatalf("expected ID lookup and cam request, got %v", paths)
			}
		})
	}
}

func TestFetchStreamNumericAPIOffline(t *testing.T) {
	mockStripchatAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "user-ids") {
			fmt.Fprint(w, `{"id":123456}`)
		} else {
			fmt.Fprint(w, `{"cam":{"isCamActive":true},"user":{"user":{"isOnline":false,"isLive":false,"status":"off"}}}`)
		}
	})
	_, err := New().FetchStream(context.Background(), internal.NewReq(), "example")
	if !errors.Is(err, internal.ErrChannelOffline) {
		t.Fatalf("expected offline, got %v", err)
	}
}

func TestFetchStreamRejectsInvalidUserID(t *testing.T) {
	for _, body := range []string{`{}`, `{"id":0}`, `{"id":-1}`, `{"id":"123"}`, `<html>unavailable</html>`} {
		t.Run(body, func(t *testing.T) {
			requests := 0
			mockStripchatAPI(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				fmt.Fprint(w, body)
			})
			_, err := New().FetchStream(context.Background(), internal.NewReq(), "example")
			if err == nil || errors.Is(err, internal.ErrChannelOffline) || requests != 1 {
				t.Fatalf("invalid ID must fail before cam lookup: err=%v requests=%d", err, requests)
			}
		})
	}
}

func TestFetchStreamUserIDHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{{http.StatusNotFound, internal.ErrNotFound}, {http.StatusTeapot, internal.ErrCloudflareBlocked}} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			mockStripchatAPI(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Server", "cloudflare")
				w.WriteHeader(tc.status)
			})
			_, err := New().FetchStream(context.Background(), internal.NewReq(), "example")
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}
