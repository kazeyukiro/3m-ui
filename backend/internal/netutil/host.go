package netutil

import (
	"net"
	"net/url"
	"strings"
)

// NormalizeHost strips brackets and optional :port, returning a bare host or IP.
func NormalizeHost(raw string) string {
	h := strings.TrimSpace(raw)
	if h == "" {
		return ""
	}
	if strings.Contains(h, "://") {
		if u, err := url.Parse(h); err == nil && u.Hostname() != "" {
			h = u.Hostname()
		}
	}
	// Bracketed IPv6 with optional port: [2001:db8::1]:443
	if strings.HasPrefix(h, "[") {
		if end := strings.Index(h, "]"); end > 0 {
			inner := h[1:end]
			rest := h[end+1:]
			if rest == "" || strings.HasPrefix(rest, ":") {
				return inner
			}
		}
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	return strings.Trim(h, "[]")
}

// IsUnspecifiedBind reports whether the address is a wildcard listen address.
func IsUnspecifiedBind(host string) bool {
	h := NormalizeHost(host)
	if h == "" || h == "*" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsUnspecified()
}

// IsLoopbackHost reports loopback IPv4/IPv6/localhost names.
func IsLoopbackHost(host string) bool {
	h := NormalizeHost(host)
	if h == "" {
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// IsIP reports whether host is a literal IPv4 or IPv6 address.
func IsIP(host string) bool {
	return net.ParseIP(NormalizeHost(host)) != nil
}

// IsIPv6 reports whether host is a literal IPv6 address.
func IsIPv6(host string) bool {
	ip := net.ParseIP(NormalizeHost(host))
	return ip != nil && ip.To4() == nil
}

// JoinHostPort is net.JoinHostPort with host normalization (strips accidental brackets).
func JoinHostPort(host, port string) string {
	return net.JoinHostPort(NormalizeHost(host), strings.TrimSpace(port))
}

// FormatURLHost returns host suitable for URL authority (brackets IPv6).
func FormatURLHost(host string, port string) string {
	h := NormalizeHost(host)
	if port == "" {
		if IsIPv6(h) {
			return "[" + h + "]"
		}
		return h
	}
	return net.JoinHostPort(h, port)
}
