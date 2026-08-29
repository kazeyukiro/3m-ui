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
type warpWireGuardSpec struct {
	PrivateKey string `yaml:"private-key,omitempty"`
	Server     string `yaml:"server"`
	Port       int    `yaml:"port"`
	IP         string `yaml:"ip"`
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

// parseWARPReserved converts "1,2,3" or "[1,2,3]" into []int. Empty input
// returns nil (reserved omitted from the YAML).
func parseWARPReserved(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
			return nil, fmt.Errorf("invalid reserved entry %q", p)
		}
		if n < 0 || n > 255 {
			return nil, fmt.Errorf("reserved entry %d out of byte range", n)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// WARPTemplate returns a Mihomo YAML fragment for Cloudflare WARP (WireGuard)
// outbound — WARP helper. Operators paste private_key / addresses from
// `warp-cli` or wgcf. Structured marshalling prevents injection through
// operator-supplied strings.
func WARPTemplate(privateKey, address, reserved string) (string, error) {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey != "" {
		if !validateWARPField(privateKey) {
			return "", fmt.Errorf("invalid private_key: must be base64 / wireguard-safe")
		}
	}
	address = strings.TrimSpace(address)
	if address == "" {
		address = "172.16.0.2/32"
	} else if !validateWARPField(address) {
		return "", fmt.Errorf("invalid address: must be host/CIDR")
	}
	reservedList, err := parseWARPReserved(reserved)
	if err != nil {
		return "", err
	}
	ipAddr := address
	if i := strings.IndexByte(ipAddr, '/'); i >= 0 {
		ipAddr = ipAddr[:i]
	}
	key := privateKey
	if key == "" {
		key = "YOUR_WARP_PRIVATE_KEY"
	}
	wg := warpWireGuardSpec{
		PrivateKey: key,
		Server:     "engage.cloudflareclient.com",
		Port:       2408,
		IP:         ipAddr,
		PublicKey:  "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		UDP:        true,
		Reserved:   reservedList,
		MTU:        1280,
	}
	_ = wg // structured shape reference; marshalled via the generic map below
	// yaml.Marshal produces a map under the top-level struct fields; we need
	// it as an entry of a proxies: list. Re-marshal as a generic map so the
	// ordering and shape matches Mihomo's expected fragment.
	proxy := map[string]interface{}{
		"name":        "WARP",
		"type":        "wireguard",
		"server":      wg.Server,
		"port":        wg.Port,
		"ip":          wg.IP,
		"private-key": wg.PrivateKey,
		"public-key":  wg.PublicKey,
		"udp":         wg.UDP,
		"mtu":         wg.MTU,
	}
	if reservedList != nil {
		proxy["reserved"] = reservedList
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
