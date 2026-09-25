package config

import (
	"flag"
	"testing"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/urfave/cli/v2"
)

func TestStripchatCLIOverridesDoNotReplaceChaturbateCredentials(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"generic aliases", []string{"--site", "stripchat", "--cookies", "session=sc", "--user-agent", "SC/test"}, "session=sc"},
		{"site-specific wins", []string{"--site", "stripchat", "--cookies", "session=old", "--stripchat-cookies", "session=sc", "--stripchat-user-agent", "SC/test"}, "session=sc"},
		{"dedicated flags in web mode", []string{"--stripchat-cookies", "session=sc", "--stripchat-user-agent", "SC/test"}, "session=sc"},
		{"explicit clear", []string{"--site", "stripchat", "--cookies", "", "--user-agent", "SC/test"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			for _, name := range []string{"site", "cookies", "user-agent", "stripchat-cookies", "stripchat-user-agent"} {
				set.String(name, "", "")
			}
			if err := set.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			ctx := cli.NewContext(cli.NewApp(), set, nil)
			cfg, err := New(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Cookies != "" || cfg.StripchatCookies != tc.want {
				t.Fatalf("initial credentials incorrectly scoped: %#v", cfg)
			}
			// Simulate settings loaded after initial CLI parsing.
			cfg = &entity.Config{Site: cfg.Site, Cookies: "session=cb", UserAgent: "CB/test", StripchatCookies: "stale=1"}
			ApplyExplicitOverrides(cfg, ctx)
			if cfg.Cookies != "session=cb" || cfg.UserAgent != "CB/test" || cfg.StripchatCookies != tc.want || cfg.StripchatUserAgent != "SC/test" {
				t.Fatalf("credential override lost site isolation: %#v", cfg)
			}
		})
	}
}
