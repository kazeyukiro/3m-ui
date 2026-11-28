package acme

import (
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

// RuntimeState describes how the panel is actually serving requests right
// now. It is deliberately separate from the saved settings: enabled=true
// only means "configured", not "HTTPS is up" (issue #106). The UI uses this
// to distinguish configured / running / fallback instead of showing a green
// "SSL on" for a setting that never took effect.
type RuntimeState struct {
	State       string `json:"state"`       // off | configured | running | fallback
	Protocol    string `json:"protocol"`    // http | https
	ListenAddr  string `json:"listen_addr"` // effective listener address
	EntryURL    string `json:"entry_url"`   // full panel URL, including web_path
	Detail      string `json:"detail"`      // short human-readable explanation
	FallbackErr string `json:"fallback_err,omitempty"`
}

var (
	runtimeMu      sync.RWMutex
	runtimeCurrent = RuntimeState{State: "off", Protocol: "http"}
)

func SetRuntime(s RuntimeState) {
	runtimeMu.Lock()
	runtimeCurrent = s
	runtimeMu.Unlock()
}

func GetRuntime() RuntimeState {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return runtimeCurrent
}

// PublicURLAndWebPath reads the externally visible base URL and the panel
// base path from the loaded config. PublicURL is what a NAT / reverse-proxy
// user actually opens in a browser, so it takes precedence over local
// listeners when building entry URLs and HTTPS redirects.
func PublicURLAndWebPath() (string, string) {
	var publicURL, webPath string
	if cfg := config.GlobalConfig; cfg != nil {
		publicURL = strings.TrimSpace(cfg.Server.PublicURL)
		webPath = config.NormalizeWebPath(cfg.Server.WebPath)
	}
	return publicURL, webPath
}

// EffectiveEntryURL builds the URL an operator should use after a restart.
// publicURL wins when configured (the external port may differ from the local
// listener); otherwise domain + the port parsed from listenAddr is used.
func EffectiveEntryURL(s Settings, publicURL, webPath, listenAddr string, fallbackPort int, https bool) string {
	scheme := "http"
	if https {
		scheme = "https"
	}
	base := strings.TrimSpace(publicURL)
	if base != "" {
		if !strings.Contains(base, "://") {
			base = scheme + "://" + base
		} else if https && strings.HasPrefix(base, "http://") {
			base = "https://" + strings.TrimPrefix(base, "http://")
		} else if !https && strings.HasPrefix(base, "https://") {
			base = "http://" + strings.TrimPrefix(base, "https://")
		}
	} else {
		host := strings.TrimSpace(s.Domain)
		if host == "" {
			host = "127.0.0.1"
		}
		port := ""
		if _, p, err := net.SplitHostPort(listenAddr); err == nil {
			port = p
		}
		if port == "" {
			port = strconv.Itoa(fallbackPort)
		}
		if (https && port == "443") || (!https && port == "80") {
			base = scheme + "://" + host
		} else {
			base = scheme + "://" + net.JoinHostPort(strings.Trim(host, "[]"), port)
		}
	}
	base = strings.TrimRight(base, "/")
	if wp := strings.Trim(webPath, "/"); wp != "" {
		base += "/" + wp
	}
	return base + "/"
}
