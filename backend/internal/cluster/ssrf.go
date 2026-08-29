package cluster

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// THREE_M_UI_CLUSTER_ALLOW_PRIVATE=1 allows loopback / RFC1918 targets (lab only).
func clusterAllowPrivate() bool {
	return os.Getenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE") == "1"
}

func assertClusterHostAllowed(host string) error {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		if clusterAllowPrivate() {
			return nil
		}
		return fmt.Errorf("base_url host %q is not allowed (set THREE_M_UI_CLUSTER_ALLOW_PRIVATE=1 for lab use)", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		return assertClusterIPAllowed(ip)
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return nil
	}
	for _, ip := range addrs {
		if err := assertClusterIPAllowed(ip); err != nil {
			return fmt.Errorf("base_url host %q: %w", host, err)
		}
	}
	return nil
}

// assertClusterIPAllowed is deliberately stricter than the URL parser. It is
// also used immediately before dialing so DNS rebinding cannot turn an
// already-validated hostname into a private or metadata endpoint.
func assertClusterIPAllowed(ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("base_url resolved to an invalid IP")
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
		return fmt.Errorf("base_url IP %s is blocked (cloud metadata)", ip)
	}
	if isBlockedClusterIP(ip) && !clusterAllowPrivate() {
		return fmt.Errorf("base_url IP %s is private/loopback/link-local; set THREE_M_UI_CLUSTER_ALLOW_PRIVATE=1 to allow", ip)
	}
	return nil
}

func isBlockedClusterIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
