package protocol

import (
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// DecodeNodeModel builds a strongly-typed NodeModel from a persisted Listener
// and optional runtime credentials (preferred over config-embedded users).
func DecodeNodeModel(l models.Listener, users []UserCred) (NodeModel, error) {
	cfg, err := ParseConfigJSON(l.Config)
	if err != nil {
		return NodeModel{}, err
	}
	n := NodeModel{
		Name:        l.Name,
		Protocol:    strings.ToLower(strings.TrimSpace(l.Protocol)),
		Listen:      firstNonEmpty(l.Listen, l.BindAddress, "0.0.0.0"),
		Port:        firstNonEmpty(strings.TrimSpace(l.PublicPort), strings.TrimSpace(l.Port)),
		PublicHost:  strings.TrimSpace(l.PublicHost),
		PublicPort:  strings.TrimSpace(l.PublicPort),
		AccessSNI:   strings.TrimSpace(l.AccessSNI),
		Fingerprint: strings.TrimSpace(l.ClientFingerprint),
		AccessALPN:  strings.TrimSpace(l.AccessALPN),
		Enabled:     l.Enabled,
		UDP:         l.UDP,
		TLS:         l.TLS,
		Users:       users,
	}
	if n.Port == "" {
		n.Port = strings.TrimSpace(l.Port)
	}

	// Prefer access profile SNI/fp over config when set.
	sni := n.AccessSNI
	if sni == "" {
		sni = strFrom(cfg, "sni", "servername")
	}
	fp := n.Fingerprint
	if fp == "" {
		fp = strFrom(cfg, "client-fingerprint", "fingerprint")
	}

	switch n.Protocol {
	case "vless":
		n.VLESS = &VLESSSpec{
			Encryption:  strDefault(cfg, "encryption", "none"),
			Flow:        strFrom(cfg, "flow"),
			Transport:   decodeTransport(cfg),
			Reality:     decodeReality(cfg),
			SkipCert:    boolFrom(cfg, "skip-cert-verify"),
			SNI:         sni,
			Fingerprint: fp,
		}
	case "vmess":
		n.VMess = &VMessSpec{
			Cipher:      strDefault(cfg, "cipher", "auto"),
			AlterID:     intFrom(cfg, "alterId"),
			Transport:   decodeTransport(cfg),
			Reality:     decodeReality(cfg),
			SkipCert:    boolFrom(cfg, "skip-cert-verify"),
			SNI:         sni,
			Fingerprint: fp,
		}
	case "trojan":
		n.Trojan = &TrojanSpec{
			Transport:   decodeTransport(cfg),
			Reality:     decodeReality(cfg),
			SkipCert:    boolFrom(cfg, "skip-cert-verify"),
			SNI:         sni,
			Fingerprint: fp,
		}
	case "shadowsocks":
		n.Shadowsocks = &ShadowsocksSpec{
			Cipher:   strFrom(cfg, "cipher"),
			Password: strFrom(cfg, "password"),
			UDP:      n.UDP || boolFrom(cfg, "udp"),
		}
	case "hysteria2":
		n.Hysteria2 = &Hysteria2Spec{
			SNI:          sni,
			SkipCert:     boolFrom(cfg, "skip-cert-verify"),
			Obfs:         strFrom(cfg, "obfs"),
			ObfsPassword: strFrom(cfg, "obfs-password"),
			Up:           strFrom(cfg, "up"),
			Down:         strFrom(cfg, "down"),
		}
	default:
		n.Generic = cfg
	}

	if len(n.Users) == 0 {
		// Fall back to config-embedded users for share/export of legacy rows.
		for _, row := range normalizeUsersValue(cfg["users"]) {
			n.Users = append(n.Users, UserCred{
				Username: strMap(row, "username"),
				Password: strMap(row, "password"),
				UUID:     strMap(row, "uuid"),
				Flow:     strMap(row, "flow"),
			})
		}
		if n.Protocol == "shadowsocks" && n.Shadowsocks != nil && n.Shadowsocks.Password != "" && len(n.Users) == 0 {
			n.Users = []UserCred{{Password: n.Shadowsocks.Password}}
		}
	}
	return n, nil
}

func decodeTransport(cfg map[string]interface{}) TransportSpec {
	t := TransportSpec{Network: "tcp"}
	if v := strFrom(cfg, "ws-path"); v != "" {
		t.Network = "ws"
		t.WSPath = v
		if headers, ok := cfg["ws-headers"].(map[string]interface{}); ok {
			t.WSHost = strMap(headers, "Host")
		}
	}
	if v := strFrom(cfg, "grpc-service-name"); v != "" {
		t.Network = "grpc"
		t.GRPCService = v
	}
	if xhttp, ok := cfg["xhttp-config"].(map[string]interface{}); ok {
		t.Network = "xhttp"
		t.XHTTPPath = strMap(xhttp, "path")
	}
	return t
}

func decodeReality(cfg map[string]interface{}) *RealitySpec {
	raw, ok := cfg["reality-config"].(map[string]interface{})
	if !ok || raw == nil {
		return nil
	}
	r := &RealitySpec{
		PublicKey:  strMap(raw, "public-key"),
		PrivateKey: strMap(raw, "private-key"),
	}
	if s, ok := firstString(raw["short-id"]); ok {
		r.ShortID = s
	}
	if s, ok := firstString(raw["server-names"]); ok {
		r.ServerName = s
	}
	return r
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func strFrom(cfg map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := cfg[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func strDefault(cfg map[string]interface{}, key, def string) string {
	if v := strFrom(cfg, key); v != "" {
		return v
	}
	return def
}

func strMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func boolFrom(cfg map[string]interface{}, key string) bool {
	if b, ok := cfg[key].(bool); ok {
		return b
	}
	return false
}

func intFrom(cfg map[string]interface{}, key string) int {
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		return n
	default:
		return 0
	}
}

func firstString(v interface{}) (string, bool) {
	if s, ok := v.(string); ok {
		return s, s != ""
	}
	if a, ok := v.([]interface{}); ok && len(a) > 0 {
		s, _ := a[0].(string)
		return s, s != ""
	}
	if a, ok := v.([]string); ok && len(a) > 0 {
		return a[0], a[0] != ""
	}
	return "", false
}
