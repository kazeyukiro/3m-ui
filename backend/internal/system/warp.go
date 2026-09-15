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

// WARPTemplate returns a Mihomo YAML fragment for Cloudflare WARP (WireGuard)
// outbound — WARP helper. Operators paste private_key / addresses from
// `warp-cli` or wgcf. Structured marshalling prevents injection through
// operator-supplied strings.
//
// Per https://wiki.metacubex.one/en/config/proxies/wg/ the IPv4 and IPv6
// addresses are emitted as separate fields (`ip` + `ipv6`), not a
// comma-joined string. The reserved list is a []int of decoded bytes
// (Cloudflare's client_id is base64-encoded).
func WARPTemplate(privateKey, ipv4, ipv6 string, reserved []int) (string, error) {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey != "" {
		if !validateWARPField(privateKey) {
			return "", fmt.Errorf("invalid private_key: must be base64 / wireguard-safe")
		}
	}
	ipv4 = strings.TrimSpace(ipv4)
	ipv6 = strings.TrimSpace(ipv6)
	if ipv4 == "" && ipv6 == "" {
		ipv4 = "172.16.0.2" // WARP default IPv4 if API returned nothing
	}
	if ipv4 != "" && !validateWARPField(ipv4) {
		return "", fmt.Errorf("invalid IPv4 address: %q", ipv4)
	}
	if ipv6 != "" && !validateWARPField(ipv6) {
		return "", fmt.Errorf("invalid IPv6 address: %q", ipv6)
	}
	key := privateKey
	if key == "" {
		key = "YOUR_WARP_PRIVATE_KEY"
	}
	// Build the proxy map directly (yaml.v3 marshals map keys alphabetically,
	// which is acceptable for Mihomo's schema — order is not semantically
	// significant for wireguard outbounds).
	proxy := map[string]interface{}{
		"name":        "WARP",
		"type":        "wireguard",
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
	if len(reserved) > 0 {
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
	header := "# Cloudflare WARP outbound for Mihomo (paste into config / routing)\n"
	footer := "\n# Example rule (optional):\n# rules:\n#   - MATCH,WARP-OUT\n"
	return header + string(out) + footer, nil
}
