package router

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/HeapOfChaos/goondvr/internal"
)

// Parse every import before updating config, so a malformed request cannot
// partially overwrite saved credentials. Never include input values in errors.
func updateBrowserCredentials(cfg *entity.Config, req *UpdateConfigRequest) error {
	cookies := internal.MergeCookieUpdate(cfg.Cookies, req.Cookies)
	userAgent := req.UserAgent
	stripchatCookies, stripchatUA := cfg.StripchatCookies, cfg.StripchatUserAgent
	if req.StripchatCookies != nil {
		stripchatCookies = internal.MergeCookieUpdate(stripchatCookies, *req.StripchatCookies)
	}
	if req.StripchatUserAgent != nil {
		stripchatUA = *req.StripchatUserAgent
	}
	for _, input := range []struct {
		raw       string
		stripchat bool
	}{{req.BrowserExport, false}, {req.StripchatBrowserExport, true}} {
		if strings.TrimSpace(input.raw) == "" {
			continue
		}
		parsed, err := internal.ParseBrowserImport(input.raw)
		if err != nil {
			return err
		}
		isStripchat := parsed.Host == "stripchat.com" || strings.HasSuffix(parsed.Host, ".stripchat.com")
		if parsed.Host != "" {
			if input.stripchat && !isStripchat {
				return fmt.Errorf("the Stripchat import must be a request to stripchat.com")
			}
			domain, _ := url.Parse(cfg.Domain)
			host := "chaturbate.com"
			if domain != nil && domain.Hostname() != "" {
				host = strings.ToLower(domain.Hostname())
			}
			if !isStripchat && parsed.Host != host && !strings.HasSuffix(parsed.Host, "."+host) {
				return fmt.Errorf("copy a request to the site itself, not a CDN or unrelated website")
			}
		}
		destCookies, destUA := &cookies, &userAgent
		if input.stripchat || isStripchat {
			destCookies, destUA = &stripchatCookies, &stripchatUA
		}
		if parsed.Cookies != "" {
			*destCookies = internal.MergeCookieUpdate(*destCookies, parsed.Cookies)
		}
		if parsed.UserAgent != "" {
			*destUA = parsed.UserAgent
		}
	}
	cfg.Cookies, cfg.UserAgent = cookies, userAgent
	cfg.StripchatCookies, cfg.StripchatUserAgent = stripchatCookies, stripchatUA
	return nil
}
