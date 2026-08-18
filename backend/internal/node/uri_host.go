package node

import (
	"net"
	"net/url"
	"strings"
)

// normalizeExportHost picks a public hostname for share links.
func normalizeExportHost(requestHost, bind, listen string) string {
	return normalizeExportHostPrefer(requestHost, bind, listen, "")
}

// normalizeExportHostPrefer priority: publicURL > request Host > bind/listen.
// Loopback is only used as a last resort so local panels can still export.
func normalizeExportHostPrefer(requestHost, bind, listen, publicURL string) string {
	candidates := []string{}
	if publicURL != "" {
		candidates = append(candidates, publicURL)
	}
	candidates = append(candidates, requestHost, bind, listen)

	var loopback string
	for _, raw := range candidates {
		h := strings.TrimSpace(raw)
		if h == "" {
			continue
		}
		if strings.Contains(h, "://") {
			if u, err := url.Parse(h); err == nil && u.Hostname() != "" {
				h = u.Hostname()
			}
		}
		if host, _, err := net.SplitHostPort(h); err == nil {
			h = host
		} else if u, err := url.Parse("//" + h); err == nil && u.Hostname() != "" {
			h = u.Hostname()
		}
		h = strings.Trim(h, "[]")
		if h == "" || h == "0.0.0.0" || h == "::" || h == "*" {
			continue
		}
		if h == "127.0.0.1" || h == "::1" || strings.EqualFold(h, "localhost") {
			if loopback == "" {
				loopback = h
			}
			continue
		}
		return h
	}
	return loopback
}
