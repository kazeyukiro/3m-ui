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
		// Always block cloud metadata endpoints.
		if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
			return fmt.Errorf("base_url IP %s is blocked (cloud metadata)", ip)
		}
		if isBlockedClusterIP(ip) && !clusterAllowPrivate() {
			return fmt.Errorf("base_url IP %s is private/loopback/link-local; set THREE_M_UI_CLUSTER_ALLOW_PRIVATE=1 to allow", ip)
		}
		return nil
	}
	// Resolve hostnames and reject if any answer is private (SSRF hardening).
	addrs, err := net.LookupIP(host)
	if err != nil {
		// Let connectivity checks surface DNS errors later; only enforce when we can resolve.
		return nil
	}
	for _, ip := range addrs {
		if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
			return fmt.Errorf("base_url host %q resolves to blocked metadata address", host)
		}
		if isBlockedClusterIP(ip) && !clusterAllowPrivate() {
			return fmt.Errorf("base_url host %q resolves to private/loopback address %s; set THREE_M_UI_CLUSTER_ALLOW_PRIVATE=1 to allow", host, ip)
		}
	}
	return nil
}

func isBlockedClusterIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
