package stripchat

import (
	"net/http"
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/server"
)

func TestPlayerPageUsesStripchatCredentials(t *testing.T) {
	old := server.Config
	t.Cleanup(func() { server.Config = old })
	server.Config = &entity.Config{Cookies: "cb=demo", UserAgent: "CB/test", StripchatCookies: "sc=demo", StripchatUserAgent: "SC/test"}
	req, _ := http.NewRequest(http.MethodGet, "https://stripchat.com/", nil)
	setStripchatBrowserHeaders(req)
	if req.Header.Get("Cookie") != "sc=demo" || req.Header.Get("User-Agent") != "SC/test" {
		t.Fatal("player discovery omitted or mixed up the Stripchat credentials")
	}
	if req.Header.Get("Sec-Fetch-Mode") != "navigate" {
		t.Fatal("player discovery lost its document request headers")
	}
}
