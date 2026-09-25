package internal

import (
	"net/http"
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/server"
)

func TestRequestCredentialsStayOnTheirSite(t *testing.T) {
	old := server.Config
	t.Cleanup(func() { server.Config = old })
	server.Config = &entity.Config{
		Domain: "https://chaturbate.com/", Cookies: "cb_session=cb", UserAgent: "CB/test",
		StripchatCookies: "Cookie: sc_session=sc; cf_clearance=clear", StripchatUserAgent: "SC/test",
	}
	for _, tc := range []struct{ host, cookies, ua string }{
		{"stripchat.com", "sc_session=sc; cf_clearance=clear", "SC/test"},
		{"www.stripchat.com", "sc_session=sc; cf_clearance=clear", "SC/test"},
		{"chaturbate.com", "cb_session=cb", "CB/test"},
		{"www.chaturbate.com", "cb_session=cb", "CB/test"},
		{"media-hls.doppiocdn.net", "", "SC/test"},
		{"mmp.doppiocdn.com", "", "SC/test"},
		{"static-proxy.strpst.com", "", "SC/test"},
		{"notstripchat.com", "", "CB/test"},
		{"stripchat.com.example.org", "", "CB/test"},
		{"chaturbate.com.example.org", "", "CB/test"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "https://"+tc.host+"/", nil)
			NewReq().SetRequestHeaders(req)
			if got := req.Header.Get("Cookie"); got != tc.cookies {
				t.Fatalf("Cookie = %q, want %q", got, tc.cookies)
			}
			if got := req.Header.Get("User-Agent"); got != tc.ua {
				t.Fatalf("User-Agent = %q, want %q", got, tc.ua)
			}
		})
	}
	server.Config.StripchatCookies = ""
	req, _ := http.NewRequest(http.MethodGet, "https://stripchat.com/", nil)
	NewReq().SetRequestHeaders(req)
	if req.Header.Get("Cookie") != "" {
		t.Fatal("empty Stripchat credentials must not fall back to Chaturbate cookies")
	}
}

func TestLegacyStripchatDomainWorkaround(t *testing.T) {
	old := server.Config
	t.Cleanup(func() { server.Config = old })
	server.Config = &entity.Config{Domain: "https://stripchat.com/", Cookies: "legacy=demo"}
	req, _ := http.NewRequest(http.MethodGet, "https://stripchat.com/", nil)
	NewReq().SetRequestHeaders(req)
	if req.Header.Get("Cookie") != "legacy=demo" {
		t.Fatal("legacy --domain workaround stopped working")
	}
}
