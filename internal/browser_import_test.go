package internal

import "testing"

func TestParseBrowserImport(t *testing.T) {
	for _, tc := range []struct {
		name, raw, cookies, ua, host string
	}{
		{"chrome bash", "curl 'https://stripchat.com/api/front/v2/models/username/example/cam' \\\n  -b 'session=demo; cf_clearance=clear' \\\n  -H 'user-agent: Chrome/test'", "session=demo; cf_clearance=clear", "Chrome/test", "stripchat.com"},
		{"firefox", `curl 'https://chaturbate.com/api/' -H 'Cookie: session=demo; tbu_room=omit' -H 'User-Agent: Firefox/test'`, "session=demo", "Firefox/test", "chaturbate.com"},
		{"windows cmd", "curl.exe \"https://stripchat.com/\" ^\r\n -b \"session=demo; token=two\" ^\r\n -H \"User-Agent: Chrome/test\"", "session=demo; token=two", "Chrome/test", "stripchat.com"},
		{"long options", `curl --url=https://stripchat.com/ --cookie='session=a=b==' --user-agent='Chrome/test'`, "session=a=b==", "Chrome/test", "stripchat.com"},
		{"chrome cmd escaped quotes", `curl ^"https://stripchat.com/^" -b ^"session=demo; token=two^" -H ^"sec-ch-ua: ^\^"Chromium^\^"^" -H ^"user-agent: Chrome/test^"`, "session=demo; token=two", "Chrome/test", "stripchat.com"},
		{"raw headers", "Cookie: session=demo; token=two\r\nUser-Agent: Browser/test", "session=demo; token=two", "Browser/test", ""},
		{"quoted apostrophe", `curl 'https://stripchat.com/' -b 'token=it'\''s' -H 'user-agent: Browser/test'`, "token=it's", "Browser/test", "stripchat.com"},
		{"ansi quoting", `curl 'https://stripchat.com/' -b $'token=it\'s' -H $'user-agent: Browser/test'`, "token=it's", "Browser/test", "stripchat.com"},
		{"literal substitutions", `curl 'https://stripchat.com/' -b 'token=$(never_execute)' -H 'user-agent: $HOME'`, "token=$(never_execute)", "$HOME", "stripchat.com"},
		{"cookie file is not read", `curl 'https://stripchat.com/' -b @cookies.txt -A Browser/test`, "", "Browser/test", "stripchat.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBrowserImport(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			want := BrowserImport{Cookies: tc.cookies, UserAgent: tc.ua, Host: tc.host}
			if got != want {
				t.Fatalf("got %#v; want %#v", got, want)
			}
		})
	}
}

func TestBrowserImportRejectsIncompleteOrUnsupportedInput(t *testing.T) {
	for _, raw := range []string{
		`curl 'https://stripchat.com/' -b 'unfinished`,
		`curl 'https://stripchat.com/' -b`,
		`curl 'https://stripchat.com/' -b cookies.txt`,
		`Invoke-WebRequest https://stripchat.com/`,
		`curl 'https://stripchat.com/' -H 'Accept: application/json'`,
	} {
		if _, err := ParseBrowserImport(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}
