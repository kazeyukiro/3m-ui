package netutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// metadataIPs are well-known cloud metadata endpoints that must never be
// reached by server-side fetches, even when private targets are explicitly
// allowed for lab use. They are blocked in addition to the broader
// link-local / private ranges covered by IsBlockedIP.
var metadataIPs = []net.IP{
	net.ParseIP("169.254.169.254"), // AWS / GCP / Azure / Alibaba link-local metadata
	net.ParseIP("fd00:ec2::254"),   // AWS IMDSv6 (IPv6) metadata
}

// AllowPrivateTargets reports whether loopback / RFC1918 / link-local
// destinations are permitted for outbound server-side fetches. This is a
// lab-only escape hatch. Cloud metadata endpoints remain blocked regardless.
//
// It honours THREE_M_UI_ALLOW_PRIVATE when set, and falls back to the legacy
// THREE_M_UI_CLUSTER_ALLOW_PRIVATE name so existing operator configurations
// keep working.
func AllowPrivateTargets() bool {
	if v := strings.TrimSpace(os.Getenv("THREE_M_UI_ALLOW_PRIVATE")); v != "" {
		return v == "1"
	}
	return strings.TrimSpace(os.Getenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE")) == "1"
}

// IsMetadataIP reports whether ip is a known cloud metadata endpoint.
func IsMetadataIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, m := range metadataIPs {
		if ip.Equal(m) {
			return true
		}
	}
	return false
}

// IsBlockedIP reports whether ip is a destination that server-side fetches
// must not reach by default: loopback, private (RFC1918/unique-local),
// link-local, unspecified, or a known cloud metadata endpoint.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if IsMetadataIP(ip) {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// AssertIPAllowed returns an error when ip is a destination that server-side
// fetches must not reach. Cloud metadata endpoints are always blocked; other
// private/loopback/link-local ranges are allowed only when AllowPrivateTargets
// is enabled (lab use).
func AssertIPAllowed(ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("resolved IP is invalid")
	}
	if IsMetadataIP(ip) {
		return fmt.Errorf("IP %s is blocked (cloud metadata)", ip)
	}
	if IsBlockedIP(ip) && !AllowPrivateTargets() {
		return fmt.Errorf("IP %s is private/loopback/link-local; set THREE_M_UI_ALLOW_PRIVATE=1 to allow", ip)
	}
	return nil
}

// AssertHostAllowed validates that host (a hostname or literal IP) resolves
// only to permitted destinations. A DNS lookup is performed when host is not
// a literal IP; every resolved address must pass AssertIPAllowed.
//
// Note: hostname validation alone is vulnerable to DNS rebinding between
// validation and connect time. Callers that go on to open a TCP connection
// must use SafeDialContext (or NewSafeHTTPClient), which re-checks every
// resolved address immediately before dialing.
func AssertHostAllowed(host string) error {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return fmt.Errorf("host is empty")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		if AllowPrivateTargets() {
			return nil
		}
		return fmt.Errorf("host %q is not allowed (set THREE_M_UI_ALLOW_PRIVATE=1 for lab use)", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		return AssertIPAllowed(ip)
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		// DNS failure is not a security verdict; the dial-time check in
		// SafeDialContext is the authoritative gate. Return nil so callers
		// can proceed to dial (which will fail or be validated there).
		return nil
	}
	for _, ip := range addrs {
		if err := AssertIPAllowed(ip); err != nil {
			return fmt.Errorf("host %q: %w", host, err)
		}
	}
	return nil
}

// SafeDialContext binds SSRF validation to the actual TCP connection. Hostname
// validation alone is vulnerable to DNS rebinding between validation and
// connect time, so every resolved address is checked immediately before
// dialing. Private/link-local/metadata addresses are never dialed unless the
// explicit lab override is enabled; cloud metadata endpoints remain blocked.
func SafeDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid dial address %q: %w", address, err)
		}
		if ip := net.ParseIP(host); ip != nil {
			if err := AssertIPAllowed(ip); err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}

		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", host, err)
		}
		var lastErr error
		for _, ip := range ips {
			if err := AssertIPAllowed(ip); err != nil {
				lastErr = err
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, fmt.Errorf("connect to %q: %w", host, lastErr)
		}
		return nil, fmt.Errorf("no allowed addresses for %q", host)
	}
}

// NewSafeHTTPClient returns an *http.Client whose transport validates every
// dialled address against the SSRF policy and whose redirects are re-checked
// against AssertHostAllowed. Use it for any server-side fetch of an
// operator/user-supplied URL (external subscriptions, cluster sync, …).
// Hardcoded upstream fetches (releases, geo files) do not need it.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:       http.ProxyFromEnvironment,
		DialContext: SafeDialContext(dialer),
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL == nil || req.URL.Hostname() == "" {
				return fmt.Errorf("redirect target has no host")
			}
			if err := AssertHostAllowed(req.URL.Hostname()); err != nil {
				return fmt.Errorf("redirect target blocked: %w", err)
			}
			return nil
		},
	}
}
