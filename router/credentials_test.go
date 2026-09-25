package router

import (
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
)

func TestChromeStripchatImportRoutesToCorrectSettings(t *testing.T) {
	cfg := &entity.Config{Domain: "https://chaturbate.com/", Cookies: "cb_session=cb", UserAgent: "CB/test", StripchatCookies: "session=sc; cf_clearance=old"}
	req := &UpdateConfigRequest{Cookies: cfg.Cookies, UserAgent: cfg.UserAgent,
		BrowserExport: `curl 'https://stripchat.com/api/front/v2/models/username/example/cam' -b 'cf_clearance=new' -H 'user-agent: SC/test'`}
	if err := updateBrowserCredentials(cfg, req); err != nil {
		t.Fatal(err)
	}
	if cfg.Cookies != "cb_session=cb" || cfg.UserAgent != "CB/test" || cfg.StripchatCookies != "session=sc; cf_clearance=new" || cfg.StripchatUserAgent != "SC/test" {
		t.Fatalf("wrong import destination or lost saved cookies: %#v", cfg)
	}
}

func TestInvalidImportDoesNotModifyCredentials(t *testing.T) {
	for _, raw := range []string{
		`curl 'https://chaturbate.com/' -b 'session=wrong-site'`,
		`curl 'https://stripchat.com.example.org/' -b 'session=wrong-site'`,
		`curl 'https://stripchat.com/' -b 'unfinished`,
	} {
		cfg := entity.Config{Cookies: "cb=old", UserAgent: "CB/test", StripchatCookies: "sc=old"}
		before := cfg
		req := &UpdateConfigRequest{Cookies: "cb=new", StripchatBrowserExport: raw}
		if err := updateBrowserCredentials(&cfg, req); err == nil {
			t.Fatalf("expected import rejection for %q", raw)
		}
		if cfg != before {
			t.Fatal("failed import changed saved config")
		}
	}
}

func TestManualStripchatCookiesCanBeCleared(t *testing.T) {
	cfg := &entity.Config{Cookies: "cb=old", StripchatCookies: "sc=old", StripchatUserAgent: "SC/test"}
	empty := ""
	req := &UpdateConfigRequest{Cookies: cfg.Cookies, StripchatCookies: &empty, StripchatUserAgent: &empty}
	if err := updateBrowserCredentials(cfg, req); err != nil {
		t.Fatal(err)
	}
	if cfg.Cookies != "cb=old" || cfg.StripchatCookies != "" || cfg.StripchatUserAgent != "" {
		t.Fatal("clearing Stripchat credentials changed the wrong settings")
	}
}
