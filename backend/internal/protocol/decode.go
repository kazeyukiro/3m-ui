package protocol

import (
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
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
	alpn := splitCSV(n.AccessALPN)
	if len(alpn) == 0 {
		alpn = stringListFrom(cfg, "alpn")
	}
	skipCert := certutil.ShouldSkipCertVerify(cfg)

	switch n.Protocol {
	case "vless":
		n.VLESS = &VLESSSpec{
			Encryption:  strDefault(cfg, "encryption", "none"),
			Flow:        strFrom(cfg, "flow"),
			Transport:   decodeTransport(cfg),
			Reality:     decodeReality(cfg),
			SkipCert:    skipCert,
			SNI:         sni,
			Fingerprint: fp,
			ALPN:        alpn,
		}
	case "vmess":
		n.VMess = &VMessSpec{
			Cipher:      strDefault(cfg, "cipher", "auto"),
			AlterID:     intFrom(cfg, "alterId"),
			Transport:   decodeTransport(cfg),
			Reality:     decodeReality(cfg),
			SkipCert:    skipCert,
			SNI:         sni,
			Fingerprint: fp,
			ALPN:        alpn,
		}
	case "trojan":
		n.Trojan = &TrojanSpec{
			Transport:   decodeTransport(cfg),
			Reality:     decodeReality(cfg),
			SkipCert:    skipCert,
			SNI:         sni,
			Fingerprint: fp,
			ALPN:        alpn,
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
			SkipCert:     skipCert,
			Obfs:         strFrom(cfg, "obfs"),
			ObfsPassword: strFrom(cfg, "obfs-password"),
			Up:           strFrom(cfg, "up"),
			Down:         strFrom(cfg, "down"),
			ALPN:         alpn,
		}
	default:
		n.Generic = cfg
	}

	if len(n.Users) == 0 {
		// Fall back to config-embedded users for share/export of legacy rows.
		for _, row := range normalizeUsersValue(cfg["users"]) {
			u := UserCred{
				Username: strMap(row, "username"),
				Password: strMap(row, "password"),
				UUID:     strMap(row, "uuid"),
				Flow:     strMap(row, "flow"),
			}
			// TUIC v5 / Hysteria-style map is {UUID: password}; normalize puts UUID in username.
			if u.UUID == "" && u.Username != "" && looksLikeUUID(u.Username) {
				u.UUID = u.Username
			}
			n.Users = append(n.Users, u)
		}
		if n.Protocol == "shadowsocks" && n.Shadowsocks != nil && n.Shadowsocks.Password != "" && len(n.Users) == 0 {
			n.Users = []UserCred{{Password: n.Shadowsocks.Password}}
		}
		// Snell: top-level psk (wiki proxies/snell).
		if n.Protocol == "snell" && len(n.Users) == 0 {
			if psk := strFrom(cfg, "psk"); psk != "" {
				n.Users = []UserCred{{Password: psk}}
			}
		}
		// Sudoku: top-level key.
		if n.Protocol == "sudoku" && len(n.Users) == 0 {
			if key := strFrom(cfg, "key"); key != "" {
				n.Users = []UserCred{{Password: key}}
			}
		}
		// TUIC v4: token only from Config. TUIC v5: never replace panel/config users with token.
		if n.Protocol == "tuic-v4" {
			var toks []UserCred
			for _, tok := range stringListFrom(cfg, "token") {
				tok = strings.TrimSpace(tok)
				if tok != "" {
					toks = append(toks, UserCred{Password: tok})
				}
			}
			if len(toks) > 0 {
				n.Users = toks
			}
		} else if n.Protocol == "tuic" && len(stringListFrom(cfg, "token")) > 0 && len(n.Users) == 0 {
			// Legacy token-only "tuic" without users.
			for _, tok := range stringListFrom(cfg, "token") {
				tok = strings.TrimSpace(tok)
				if tok != "" {
					n.Users = append(n.Users, UserCred{Password: tok})
				}
			}
		} else if n.Protocol == "tuic-v5" || n.Protocol == "tuic" {
			if len(n.Users) == 0 {
				if users, ok := cfg["users"].(map[string]interface{}); ok {
					for uuid, pw := range users {
						if s, ok := pw.(string); ok && strings.TrimSpace(s) != "" {
							n.Users = append(n.Users, UserCred{UUID: uuid, Username: uuid, Password: strings.TrimSpace(s)})
						}
					}
				}
			}
		}
	}
	return n, nil
}

func looksLikeUUID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
	}
	return true
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

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func stringListFrom(cfg map[string]interface{}, key string) []string {
	v, ok := cfg[key]
	if !ok {
		return nil
	}
	switch a := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(a))
		for _, item := range a {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(a))
		for _, s := range a {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		return splitCSV(a)
	default:
		return nil
	}
}
