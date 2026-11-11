package protocol

import (
	"fmt"
	"strings"
)

// Shadowsocks multi-user support (mihomo v1.19.34).
//
// Upstream references (v1.19.34 tag):
//   - listener/config/shadowsocks.go: Users map[string]string `yaml:"users"`
//   - listener/sing_shadowsocks/server.go newMultiService: for AEAD methods
//     config.Password must be empty and each user password is used directly;
//     for 2022-blake3-aes-* methods config.Password is the server PSK (iPSK)
//     and each user password is the user PSK (uPSK), so the client password
//     is "iPSK:uPSK". The chacha variants are rejected there with
//     os.ErrInvalid -> "users is not supported for method".
//   - docs/config.yaml (shadowsocks-in-1): same rules in the Chinese comments.
//
// The panel treats `users` as an explicit opt-in: a listener without a
// non-empty users map keeps the historical single-password behaviour byte for
// byte. Nothing is inferred from how many users are bound.

// ssAEADCiphers mirrors shadowaead.List from the upstream sing-shadowsocks
// dependency (MetaCubeX/sing-shadowsocks shadowaead/protocol.go).
var ssAEADCiphers = map[string]bool{
	"aes-128-gcm":             true,
	"aes-192-gcm":             true,
	"aes-256-gcm":             true,
	"chacha20-ietf-poly1305":  true,
	"xchacha20-ietf-poly1305": true,
	"chacha8-ietf-poly1305":   true,
	"xchacha8-ietf-poly1305":  true,
	"rabbit128-poly1305":      true,
	"aes-128-ccm":             true,
	"aes-192-ccm":             true,
	"aes-256-ccm":             true,
	"aes-128-gcm-siv":         true,
	"aes-256-gcm-siv":         true,
	"aegis-128l":              true,
	"aegis-256":               true,
	"aez-384":                 true,
	"deoxys-ii-256-128":       true,
	"lea-128-gcm":             true,
	"lea-192-gcm":             true,
	"lea-256-gcm":             true,
	"ascon128":                true,
	"ascon128a":               true,
}

// ss2022MultiUserCiphers mirrors the case list of
// shadowaead_2022/service_multi.go NewMultiService. The chacha variants are
// intentionally absent: that constructor cannot identify users for them.
var ss2022MultiUserCiphers = map[string]bool{
	"2022-blake3-aes-128-gcm": true,
	"2022-blake3-aes-256-gcm": true,
	"2022-blake3-aes-128-ccm": true,
	"2022-blake3-aes-256-ccm": true,
}

// IsSS2022MultiUserCipher reports whether cipher is a 2022-blake3 method that
// supports a users map (and therefore needs a top-level server PSK).
func IsSS2022MultiUserCipher(cipher string) bool {
	return ss2022MultiUserCiphers[strings.ToLower(strings.TrimSpace(cipher))]
}

// IsSSAEADCipher reports whether cipher is a legacy AEAD method that supports
// a users map with an empty top-level password.
func IsSSAEADCipher(cipher string) bool {
	return ssAEADCiphers[strings.ToLower(strings.TrimSpace(cipher))]
}

// SSConfigUsers returns the explicitly configured users map. The panel stores
// it as decoded JSON (map[string]interface{}); callers may also build
// map[string]string programmatically.
func SSConfigUsers(cfg map[string]interface{}) (map[string]string, bool) {
	if cfg == nil {
		return nil, false
	}
	raw, ok := cfg["users"]
	if !ok || raw == nil {
		return nil, false
	}
	out := make(map[string]string)
	switch m := raw.(type) {
	case map[string]string:
		for k, v := range m {
			if strings.TrimSpace(k) != "" {
				out[k] = v
			}
		}
	case map[string]interface{}:
		for k, v := range m {
			s, ok := v.(string)
			if ok && strings.TrimSpace(k) != "" {
				out[k] = s
			}
		}
	default:
		return nil, false
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// ValidateSSMultiUser validates an explicit multi-user config and returns the
// users map plus its mode ("aead" or "2022"). A config without a users map is
// a no-op and keeps the single-password behaviour.
func ValidateSSMultiUser(cfg map[string]interface{}) (map[string]string, string, error) {
	users, ok := SSConfigUsers(cfg)
	if !ok {
		return nil, "", nil
	}
	cipher := strings.ToLower(strings.TrimSpace(strValue(cfg["cipher"])))
	password := strings.TrimSpace(strValue(cfg["password"]))
	for name, pass := range users {
		if strings.TrimSpace(pass) == "" {
			return nil, "", fmt.Errorf("shadowsocks user %q has an empty password", name)
		}
	}
	switch {
	case IsSS2022MultiUserCipher(cipher):
		if password == "" {
			return nil, "", fmt.Errorf("shadowsocks multi-user with %s requires a top-level password (server PSK)", cipher)
		}
		return users, "2022", nil
	case IsSSAEADCipher(cipher):
		if password != "" {
			return nil, "", fmt.Errorf("shadowsocks multi-user with %s requires an empty top-level password; each client uses its own password from users", cipher)
		}
		return users, "aead", nil
	default:
		if strings.HasPrefix(cipher, "2022-blake3-chacha") {
			return nil, "", fmt.Errorf("shadowsocks %s does not support the users field (chacha variants cannot identify users)", cipher)
		}
		return nil, "", fmt.Errorf("shadowsocks method %q does not support the users field; use an AEAD or 2022-blake3-aes-* method", cipher)
	}
}

// ResolveSSClientPassword builds the client-side password for one subscriber
// of a multi-user Shadowsocks listener. It returns false when the config is
// not multi-user or the username is not present in the users map.
func ResolveSSClientPassword(cfg map[string]interface{}, username string) (string, bool) {
	users, ok := SSConfigUsers(cfg)
	if !ok {
		return "", false
	}
	userPSK, ok := users[strings.TrimSpace(username)]
	if !ok || strings.TrimSpace(userPSK) == "" {
		return "", false
	}
	cipher := strings.ToLower(strings.TrimSpace(strValue(cfg["cipher"])))
	if IsSS2022MultiUserCipher(cipher) {
		serverPSK := strings.TrimSpace(strValue(cfg["password"]))
		if serverPSK == "" {
			return "", false
		}
		return serverPSK + ":" + userPSK, true
	}
	return userPSK, true
}

func strValue(v interface{}) string {
	s, _ := v.(string)
	return s
}
