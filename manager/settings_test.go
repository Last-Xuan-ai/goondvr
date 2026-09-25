package manager

import (
	"os"
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/server"
)

func TestSiteCredentialsSurviveSettingsRoundTrip(t *testing.T) {
	oldConfig := server.Config
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Config = oldConfig; _ = os.Chdir(oldDir) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	server.Config = &entity.Config{Cookies: "cb=demo", UserAgent: "CB/test", StripchatCookies: "sc=demo", StripchatUserAgent: "SC/test"}
	if err := SaveSettings(); err != nil {
		t.Fatal(err)
	}
	server.Config = &entity.Config{}
	if err := LoadSettings(); err != nil {
		t.Fatal(err)
	}
	if server.Config.Cookies != "cb=demo" || server.Config.UserAgent != "CB/test" || server.Config.StripchatCookies != "sc=demo" || server.Config.StripchatUserAgent != "SC/test" {
		t.Fatal("site credentials were lost on reload")
	}
	if err := os.WriteFile(settingsFile, []byte(`{"cookies":"legacy=demo","user_agent":"legacy/test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	server.Config = &entity.Config{}
	if err := LoadSettings(); err != nil {
		t.Fatal(err)
	}
	if server.Config.Cookies != "legacy=demo" || server.Config.StripchatCookies != "" {
		t.Fatal("legacy settings were incorrectly reassigned to Stripchat")
	}
}
