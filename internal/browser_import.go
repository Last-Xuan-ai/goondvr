package internal

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// BrowserImport contains only the request credentials, never executable code.
type BrowserImport struct {
	Cookies   string
	UserAgent string
	Host      string
}

// ParseBrowserImport accepts Chrome/Firefox Copy as cURL (bash or cmd) and
// plain Cookie/User-Agent header lines. It never executes commands or reads files.
func ParseBrowserImport(raw string) (BrowserImport, error) {
	var result BrowserImport
	setHeader := func(header string) {
		name, value, ok := strings.Cut(header, ":")
		if !ok {
			return
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "cookie":
			result.Cookies = SanitizeCookieString(value)
		case "user-agent":
			result.UserAgent = strings.TrimSpace(value)
		}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result, nil
	}
	if strings.HasPrefix(strings.ToLower(raw), "cookie:") || strings.HasPrefix(strings.ToLower(raw), "user-agent:") {
		for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
			setHeader(line)
		}
	} else {
		args, err := splitBrowserCommand(raw)
		if err != nil || len(args) == 0 || (args[0] != "curl" && args[0] != "curl.exe") {
			return result, fmt.Errorf("use Copy as cURL (bash or cmd), or paste Cookie and User-Agent header lines")
		}
		for i := 1; i < len(args); i++ {
			arg := args[i]
			flag, value, attached := strings.Cut(arg, "=")
			if !attached {
				flag = arg
			}
			switch flag {
			case "-H", "--header", "-b", "--cookie", "-A", "--user-agent", "--url":
				if !attached {
					i++
					if i >= len(args) {
						return BrowserImport{}, fmt.Errorf("incomplete cURL option")
					}
					value = args[i]
				}
				switch flag {
				case "-H", "--header":
					setHeader(value)
				case "-b", "--cookie":
					// curl also accepts filenames here; imports only accept inline cookies.
					if !strings.HasPrefix(value, "@") && strings.Contains(value, "=") {
						result.Cookies = SanitizeCookieString(value)
					}
				case "-A", "--user-agent":
					result.UserAgent = value
				case "--url":
					result.Host = browserImportHost(value)
				}
			default:
				if host := browserImportHost(arg); host != "" {
					result.Host = host
				}
			}
		}
	}
	if result.Cookies == "" && result.UserAgent == "" {
		return BrowserImport{}, fmt.Errorf("no Cookie or User-Agent found in the browser request")
	}
	return result, nil
}

func browserImportHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// Tokenize browser exports without invoking a shell. Quoted text is literal;
// even shell substitutions are just data. Support POSIX quotes, ANSI-C quotes,
// and cmd's caret escaping/line continuations used by Chrome on Windows.
func splitBrowserCommand(raw string) ([]string, error) {
	cmdStyle := strings.Contains(raw, "^\n") || strings.Contains(raw, "^\r\n") || strings.Contains(raw, "^\"") || strings.HasPrefix(raw, "curl.exe ")
	if cmdStyle {
		// Chrome's cmd export wraps arguments in ^"...^" and escapes inner
		// quotes as ^\^". Remove the cmd escape layer before tokenizing.
		var unescaped strings.Builder
		for i := 0; i < len(raw); i++ {
			if raw[i] == '^' && i+1 < len(raw) {
				i++
				if raw[i] == '\r' && i+1 < len(raw) && raw[i+1] == '\n' {
					i++
					continue
				}
				if raw[i] == '\n' {
					continue
				}
			}
			unescaped.WriteByte(raw[i])
		}
		raw = unescaped.String()
	}
	var args []string
	var word strings.Builder
	var quote byte
	started, ansi := false, false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if quote == 0 && strings.ContainsRune(" \t\r\n", rune(c)) {
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		if quote == 0 && c == '$' && i+1 < len(raw) && raw[i+1] == '\'' {
			quote, ansi, started = '\'', true, true
			i++
			continue
		}
		if quote == 0 && (c == '\'' || c == '"') {
			quote, started = c, true
			continue
		}
		if c == quote && quote != 0 {
			quote, ansi = 0, false
			continue
		}
		if ansi && c == '\\' {
			r, _, tail, err := strconv.UnquoteChar(raw[i:], '\'')
			if err != nil {
				return nil, fmt.Errorf("invalid quoted escape")
			}
			i = len(raw) - len(tail) - 1
			word.WriteRune(r)
			continue
		}
		if c == '\\' && quote != '\'' {
			if i+1 == len(raw) {
				return nil, fmt.Errorf("incomplete escape")
			}
			next := raw[i+1]
			if next == '\r' && i+2 < len(raw) && raw[i+2] == '\n' {
				i += 2
				continue
			}
			if next == '\n' {
				i++
				continue
			}
			if quote == 0 || strings.ContainsRune("\\\"$`", rune(next)) {
				word.WriteByte(next)
				started = true
				i++
				continue
			}
		}
		word.WriteByte(c)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	if started {
		args = append(args, word.String())
	}
	return args, nil
}
