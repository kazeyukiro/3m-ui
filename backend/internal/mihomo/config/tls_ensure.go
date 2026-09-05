package config

import (
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
)

// ensureListenerTLSMaterial fills empty certificate/private-key for listeners
// that Mihomo requires to present server TLS credentials.
//
// Uses certutil.GenerateSelfSigned so all panel-issued PEMs share Org=3m-ui,
// 10y validity, and multi-SAN (SNI / PublicHost / localhost / IPs).
func ensureListenerTLSMaterial(protocol string, cfg map[string]interface{}) error {
	if cfg == nil {
		return nil
	}
	proto := strings.ToLower(strings.TrimSpace(protocol))
	if s, ok := cfg["certificate"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "certificate")
	}
	if s, ok := cfg["private-key"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "private-key")
	}
	if s, ok := cfg["private_key"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "private_key")
	}
	cert, _ := cfg["certificate"].(string)
	key, _ := cfg["private-key"].(string)
	if key == "" {
		key, _ = cfg["private_key"].(string)
	}
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		delete(cfg, "allow-insecure")
		return nil
	}
	if strings.TrimSpace(cert) != "" || strings.TrimSpace(key) != "" {
		delete(cfg, "certificate")
		delete(cfg, "private-key")
		delete(cfg, "private_key")
	}
	if !listenerProtocolNeedsCert(proto, cfg) {
		return nil
	}
	hints := certutil.HostHintsFromConfig(cfg)
	host := "localhost"
	if len(hints) > 0 {
		host = hints[0]
	}
	certPEM, keyPEM, err := certutil.GenerateSelfSigned(host, hints...)
	if err != nil {
		return fmt.Errorf("listener %s: %w", proto, err)
	}
	cfg["certificate"] = certPEM
	cfg["private-key"] = keyPEM
	delete(cfg, "allow-insecure")
	return nil
}

func listenerProtocolNeedsCert(proto string, cfg map[string]interface{}) bool {
	if _, ok := cfg["reality-config"]; ok {
		return false
	}
	if b, ok := cfg["allow-insecure"].(bool); ok && b {
		switch proto {
		case "anytls", "trojan", "vmess", "vless":
			return false
		}
	}
	if layer, _ := cfg["security_layer"].(string); strings.EqualFold(strings.TrimSpace(layer), "none") {
		switch proto {
		case "vless", "vmess", "trojan":
			return false
		}
	}
	if layer, _ := cfg["security_layer"].(string); strings.EqualFold(strings.TrimSpace(layer), "reality") {
		return false
	}
	if completeShadowTLS(cfg) || completeResTLS(cfg) || completeJLS(cfg) {
		return false
	}
	if _, ok := cfg["tlsmirror-config"]; ok && cfg["tlsmirror-config"] != nil {
		return false
	}
	switch proto {
	case "hysteria2", "anytls", "tuic", "tuic-v4", "tuic-v5", "trusttunnel", "trojan", "vless", "vmess":
		return true
	default:
		return false
	}
}

func completeJLS(cfg map[string]interface{}) bool {
	jls, ok := cfg["jls-config"].(map[string]interface{})
	if !ok || jls == nil {
		return false
	}
	dest, _ := jls["dest"].(string)
	if strings.TrimSpace(dest) == "" {
		return false
	}
	return hasJLSUsers(jls)
}

func hasJLSUsers(jls map[string]interface{}) bool {
	raw, ok := jls["users"]
	if !ok || raw == nil {
		return false
	}
	switch u := raw.(type) {
	case map[string]interface{}:
		return len(u) > 0
	case []interface{}:
		return len(u) > 0
	default:
		return false
	}
}

func completeResTLS(cfg map[string]interface{}) bool {
	rt, ok := cfg["res-tls"].(map[string]interface{})
	if !ok || rt == nil {
		return false
	}
	if en, ok := rt["enable"].(bool); ok && !en {
		return false
	}
	dest, _ := rt["dest"].(string)
	return strings.TrimSpace(dest) != ""
}

func completeShadowTLS(cfg map[string]interface{}) bool {
	st, ok := cfg["shadow-tls"].(map[string]interface{})
	if !ok || st == nil {
		return false
	}
	if en, ok := st["enable"].(bool); ok && !en {
		return false
	}
	if p, _ := st["password"].(string); strings.TrimSpace(p) != "" {
		return true
	}
	if _, ok := st["users"]; ok {
		return true
	}
	if hs, ok := st["handshake"].(map[string]interface{}); ok {
		if d, _ := hs["dest"].(string); strings.TrimSpace(d) != "" {
			return true
		}
	}
	return false
}
