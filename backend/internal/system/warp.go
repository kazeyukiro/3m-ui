package system

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// warpWireGuardSpec is the structured shape of the Mihomo WireGuard outbound
// we render for Cloudflare WARP. Keeping it as a struct lets yaml.v3 handle
// escaping / indentation, which prevents a malicious operator-supplied
// private_key or address from breaking out of the YAML scalar and injecting
// sibling keys.
// Field names align with mihomo's wireguard outbound schema documented at
// https://wiki.metacubex.one/en/config/proxies/wg/ — in particular, IPv4 and
// IPv6 are separate fields (ip + ipv6), NOT a comma-joined string.
type warpWireGuardSpec struct {
	PrivateKey string `yaml:"private-key,omitempty"`
	Server     string `yaml:"server"`
	Port       int    `yaml:"port"`
	IP         string `yaml:"ip"`             // IPv4 address (no CIDR)
	IPv6       string `yaml:"ipv6,omitempty"` // IPv6 address (no CIDR)
	PublicKey  string `yaml:"public-key"`
	UDP        bool   `yaml:"udp"`
	Reserved   []int  `yaml:"reserved,omitempty"`
	MTU        int    `yaml:"mtu"`
}

type warpProxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

type warpConfig struct {
	Proxies []map[string]interface{} `yaml:"proxies"`
	Groups  []warpProxyGroup         `yaml:"proxy-groups,omitempty"`
}

// validateWARPField rejects shell-significant / YAML-significant characters in
// a WARP WireGuard field. WireGuard private keys and addresses are restricted
// to base64 (key) and dotted-quad/CIDR (address); anything else is suspicious.
func validateWARPField(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '+' || r == '/' || r == '=': // base64 alphabet
		case r == ':' || r == '.' || r == '-' || r == '_':
		case r == ',': // reserved list separator (kept simple)
		default:
			return false
		}
	}
	return true
}

// WARPMasqueTemplate returns a Mihomo YAML fragment for Cloudflare WARP using
// the MASQUE protocol (RFC 9298 / HTTP/3 CONNECT-UDP). Cloudflare WARP now
// defaults to MASQUE per the registration API's `policy.tunnel_protocol`
// field — this template emits the schema documented at
// https://wiki.metacubex.one/config/proxies/masque/
//
// Key differences vs WireGuard WARP:
//   - `ip` / `ipv6` carry a CIDR (/32 and /128) per mihomo's masque schema
//   - No `reserved` field (that's wireguard-only)
//   - Same X25519 keypair (Curve25519 private/public key from WARP register)
//   - Optional `network` field: "" (default UDP) | "h2" | "h3-l4proxy"
//   - Optional `congestion-controller`: "bbr" recommended for WARP
func WARPMasqueTemplate(privateKey, ipv4, ipv6, network string) (string, error) {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey != "" {
		if !validateWARPField(privateKey) {
			return "", fmt.Errorf("invalid private_key: must be base64 / wireguard-safe")
		}
	}
	ipv4 = strings.TrimSpace(ipv4)
	ipv6 = strings.TrimSpace(ipv6)
	if ipv4 == "" && ipv6 == "" {
		ipv4 = "172.16.0.2"
	}
	// masque requires CIDR form. Append /32 / /128 if caller passed bare IP.
	ipv4 = ensureCIDR(ipv4, "/32")
	ipv6 = ensureCIDR(ipv6, "/128")
	if !validateWARPField(ipv4) {
		return "", fmt.Errorf("invalid IPv4 address: %q", ipv4)
	}
	if !validateWARPField(ipv6) {
		return "", fmt.Errorf("invalid IPv6 address: %q", ipv6)
	}
	key := privateKey
	if key == "" {
		key = "YOUR_WARP_PRIVATE_KEY"
	}
	network = strings.TrimSpace(network)
	if network != "" && network != "h2" && network != "h3-l4proxy" {
		return "", fmt.Errorf("invalid network %q: must be '', 'h2', or 'h3-l4proxy'", network)
	}
	proxy := map[string]interface{}{
		"name":        "WARP-Masque",
		"type":        "masque",
		"server":      "engage.cloudflareclient.com",
		"port":        2408,
		"ip":          ipv4,
		"private-key": key,
		"public-key":  "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		"udp":         true,
		"mtu":         1280,
	}
	if ipv6 != "" {
		proxy["ipv6"] = ipv6
	}
	if network != "" {
		proxy["network"] = network
	}
	cfg := warpConfig{
		Proxies: []map[string]interface{}{proxy},
		Groups: []warpProxyGroup{
			{Name: "WARP-Masque-OUT", Type: "select", Proxies: []string{"WARP-Masque", "DIRECT"}},
		},
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	header := "# Cloudflare WARP (MASQUE) outbound for Mihomo\n"
	footer := "\n# rules:\n#   - MATCH,WARP-Masque-OUT\n"
	return header + string(out) + footer, nil
}

// ensureCIDR appends suffix if s does not already contain a '/'.
func ensureCIDR(s, suffix string) string {
	if s == "" {
		return s
	}
	if strings.IndexByte(s, '/') >= 0 {
		return s
	}
	return s + suffix
}

// WARPTemplate returns a Mihomo WireGuard outbound for Cloudflare WARP
// (aligned with RomanovCaesar/m-ui warpOutbound).
func WARPTemplate(privateKey, peerPublicKey, server string, port int, ipv4, ipv6 string, reserved []int) (string, error) {
	privateKey = strings.TrimSpace(privateKey)
	peerPublicKey = strings.TrimSpace(peerPublicKey)
	server = strings.TrimSpace(server)
	if privateKey == "" {
		return "", fmt.Errorf("private_key required")
	}
	if !validateWARPField(privateKey) {
		return "", fmt.Errorf("invalid private_key")
	}
	if peerPublicKey == "" {
		return "", fmt.Errorf("peer public_key required")
	}
	if !validateWARPField(peerPublicKey) {
		return "", fmt.Errorf("invalid peer public_key")
	}
	if server == "" {
		server = "engage.cloudflareclient.com"
	}
	if port < 1 || port > 65535 {
		port = 2408
	}
	ipv4 = strings.TrimSpace(ipv4)
	ipv6 = strings.TrimSpace(ipv6)
	if ipv4 == "" && ipv6 == "" {
		return "", fmt.Errorf("at least one of ipv4/ipv6 required")
	}
	if ipv4 != "" && !validateWARPField(ipv4) {
		return "", fmt.Errorf("invalid IPv4 %q", ipv4)
	}
	if ipv6 != "" && !validateWARPField(ipv6) {
		return "", fmt.Errorf("invalid IPv6 %q", ipv6)
	}
	proxy := map[string]interface{}{
		"name":        "WARP",
		"type":        "wireguard",
		"server":      server,
		"port":        port,
		"private-key": privateKey,
		"public-key":  peerPublicKey,
		"udp":         true,
		"mtu":         1420,
		"allowed-ips": []string{"0.0.0.0/0", "::/0"},
	}
	if ipv4 != "" {
		proxy["ip"] = ipv4
	}
	if ipv6 != "" {
		proxy["ipv6"] = ipv6
	}
	if len(reserved) == 3 {
		proxy["reserved"] = reserved
	}
	cfg := warpConfig{
		Proxies: []map[string]interface{}{proxy},
		Groups: []warpProxyGroup{
			{Name: "WARP-OUT", Type: "select", Proxies: []string{"WARP", "DIRECT"}},
		},
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	header := "# Cloudflare WARP (WireGuard) — m-ui compatible outbound for Mihomo\n"
	footer := "\n# rules:\n#   - MATCH,WARP-OUT\n"
	return header + string(out) + footer, nil
}
