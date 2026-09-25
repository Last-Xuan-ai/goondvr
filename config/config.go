package config

import (
	"github.com/HeapOfChaos/goondvr/entity"
	"github.com/urfave/cli/v2"
)

// New initializes a new Config struct with values from the CLI context.
func New(c *cli.Context) (*entity.Config, error) {
	cfg := &entity.Config{
		Version:             c.App.Version,
		Username:            c.String("username"),
		Site:                entity.NormalizeSite(c.String("site")),
		AdminUsername:       c.String("admin-username"),
		AdminPassword:       c.String("admin-password"),
		Framerate:           c.Int("framerate"),
		Resolution:          c.Int("resolution"),
		Pattern:             c.String("pattern"),
		MaxDuration:         c.Int("max-duration"),
		MaxFilesize:         c.Int("max-filesize"),
		Port:                c.String("port"),
		Interval:            c.Int("interval"),
		Cookies:             c.String("cookies"),
		UserAgent:           c.String("user-agent"),
		Domain:              c.String("domain"),
		CompletedDir:        c.String("completed-dir"),
		FinalizeMode:        entity.NormalizeFinalizeMode(c.String("finalize-mode")),
		FFmpegEncoder:       c.String("ffmpeg-encoder"),
		FFmpegContainer:     c.String("ffmpeg-container"),
		FFmpegQuality:       c.Int("ffmpeg-quality"),
		FFmpegPreset:        c.String("ffmpeg-preset"),
		Debug:               c.Bool("debug"),
		BrowserMode:         entity.NormalizeBrowserMode(c.String("browser-mode")),
		BrowserPath:         c.String("browser-path"),
		BrowserProfileDir:   c.String("browser-profile-dir"),
		BrowserHelperURL:    c.String("browser-helper-url"),
		BrowserHelperToken:  c.String("browser-helper-token"),
		BrowserHelperServer: c.Bool("browser-helper"),
		BrowserHelperBind:   c.String("browser-helper-bind"),
		BrowserDebugPort:    c.Int("browser-debug-port"),
		BrowserBootstrap:    c.Bool("browser-bootstrap"),
		BrowserBootstrapURL: c.String("browser-bootstrap-url"),
		StripchatPDKey:      c.String("stripchat-pdkey"),
	}
	// Generic CLI credentials belong to the explicitly selected site.
	// Persisted Chaturbate settings must not become Stripchat credentials.
	if cfg.Site == "stripchat" {
		cfg.Cookies, cfg.UserAgent = "", ""
	}
	applyCredentialOverrides(cfg, c)
	return cfg, nil
}

// ApplyExplicitOverrides reapplies command-line flags that were explicitly set
// by the user after persisted settings have been loaded.
func ApplyExplicitOverrides(cfg *entity.Config, c *cli.Context) {
	if c.IsSet("username") {
		cfg.Username = c.String("username")
	}
	if c.IsSet("site") {
		cfg.Site = entity.NormalizeSite(c.String("site"))
	}
	if c.IsSet("admin-username") {
		cfg.AdminUsername = c.String("admin-username")
	}
	if c.IsSet("admin-password") {
		cfg.AdminPassword = c.String("admin-password")
	}
	if c.IsSet("framerate") {
		cfg.Framerate = c.Int("framerate")
	}
	if c.IsSet("resolution") {
		cfg.Resolution = c.Int("resolution")
	}
	if c.IsSet("pattern") {
		cfg.Pattern = c.String("pattern")
	}
	if c.IsSet("max-duration") {
		cfg.MaxDuration = c.Int("max-duration")
	}
	if c.IsSet("max-filesize") {
		cfg.MaxFilesize = c.Int("max-filesize")
	}
	if c.IsSet("port") {
		cfg.Port = c.String("port")
	}
	if c.IsSet("interval") {
		cfg.Interval = c.Int("interval")
	}
	applyCredentialOverrides(cfg, c)
	if c.IsSet("domain") {
		cfg.Domain = c.String("domain")
	}
	if c.IsSet("completed-dir") {
		cfg.CompletedDir = c.String("completed-dir")
	}
	if c.IsSet("finalize-mode") {
		cfg.FinalizeMode = entity.NormalizeFinalizeMode(c.String("finalize-mode"))
	}
	if c.IsSet("ffmpeg-encoder") {
		cfg.FFmpegEncoder = c.String("ffmpeg-encoder")
	}
	if c.IsSet("ffmpeg-container") {
		cfg.FFmpegContainer = c.String("ffmpeg-container")
	}
	if c.IsSet("ffmpeg-quality") {
		cfg.FFmpegQuality = c.Int("ffmpeg-quality")
	}
	if c.IsSet("ffmpeg-preset") {
		cfg.FFmpegPreset = c.String("ffmpeg-preset")
	}
	if c.IsSet("debug") {
		cfg.Debug = c.Bool("debug")
	}
	if c.IsSet("browser-mode") {
		cfg.BrowserMode = entity.NormalizeBrowserMode(c.String("browser-mode"))
	}
	if c.IsSet("browser-path") {
		cfg.BrowserPath = c.String("browser-path")
	}
	if c.IsSet("browser-profile-dir") {
		cfg.BrowserProfileDir = c.String("browser-profile-dir")
	}
	if c.IsSet("browser-helper-url") {
		cfg.BrowserHelperURL = c.String("browser-helper-url")
	}
	if c.IsSet("browser-helper-token") {
		cfg.BrowserHelperToken = c.String("browser-helper-token")
	}
	if c.IsSet("browser-helper") {
		cfg.BrowserHelperServer = c.Bool("browser-helper")
	}
	if c.IsSet("browser-helper-bind") {
		cfg.BrowserHelperBind = c.String("browser-helper-bind")
	}
	if c.IsSet("browser-debug-port") {
		cfg.BrowserDebugPort = c.Int("browser-debug-port")
	}
	if c.IsSet("browser-bootstrap") {
		cfg.BrowserBootstrap = c.Bool("browser-bootstrap")
	}
	if c.IsSet("browser-bootstrap-url") {
		cfg.BrowserBootstrapURL = c.String("browser-bootstrap-url")
	}
	if c.IsSet("stripchat-pdkey") {
		cfg.StripchatPDKey = c.String("stripchat-pdkey")
	}
}

// Site-specific flags take precedence over the generic CLI aliases.
func applyCredentialOverrides(cfg *entity.Config, c *cli.Context) {
	cookies, userAgent := &cfg.Cookies, &cfg.UserAgent
	if cfg.Site == "stripchat" {
		cookies, userAgent = &cfg.StripchatCookies, &cfg.StripchatUserAgent
	}
	if c.IsSet("cookies") {
		*cookies = c.String("cookies")
	}
	if c.IsSet("user-agent") {
		*userAgent = c.String("user-agent")
	}
	if c.IsSet("stripchat-cookies") {
		cfg.StripchatCookies = c.String("stripchat-cookies")
	}
	if c.IsSet("stripchat-user-agent") {
		cfg.StripchatUserAgent = c.String("stripchat-user-agent")
	}
}
