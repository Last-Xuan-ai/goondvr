package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/manager"
	"github.com/HeapOfChaos/goondvr/server"
)

func TestSettingsFormImportsAndPersistsChromeCredentials(t *testing.T) {
	oldConfig, oldManager := server.Config, server.Manager
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		server.Config, server.Manager = oldConfig, oldManager
		_ = os.Chdir(oldDir)
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	server.Config = &entity.Config{Domain: "https://chaturbate.com/", Cookies: "cb=demo", UserAgent: "CB/test"}
	server.Manager = &manager.Manager{}
	r := SetupRouter()
	form := url.Values{
		"cookies": {"cb=demo"}, "user_agent": {"CB/test"},
		"stripchat_cookies": {""}, "stripchat_user_agent": {""},
		"stripchat_browser_export": {`curl 'https://stripchat.com/api/' -b 'sc=demo; token=a=b' -H 'user-agent: SC/test'`},
	}
	req := httptest.NewRequest(http.MethodPost, "/update_config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("save returned %d: %s", w.Code, w.Body)
	}
	server.Config = &entity.Config{}
	if err := manager.LoadSettings(); err != nil {
		t.Fatal(err)
	}
	if server.Config.Cookies != "cb=demo" || server.Config.StripchatCookies != "sc=demo; token=a=b" || server.Config.StripchatUserAgent != "SC/test" {
		t.Fatal("saved form lost or mixed up site credentials")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `name="stripchat_cookies"`) || !strings.Contains(w.Body.String(), "sc=demo; token=a=b") {
		t.Fatal("settings page did not render the saved Stripchat credentials")
	}
}
