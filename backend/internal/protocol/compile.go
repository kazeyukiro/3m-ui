package protocol

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// UserCred is a protocol-agnostic credential injected at compile time.
type UserCred struct {
	Username string
	Password string
	UUID     string
	Flow     string
}

// CompileInput is the normalized node state passed to protocol modules.
type CompileInput struct {
	Name               string
	Protocol           string
	Listen             string
	Port               interface{} // int or official ports string
	UDP                bool
	TLS                bool
	Proxy              string
	Rule               string
	RoutingMark        int
	Config             map[string]interface{}
	Users              []UserCred
	HasCredentialState bool // true when creds map has this listener key (even if empty)
}

// Module is a protocol compiler (m-ui style).
type Module interface {
	Kind() string
	Compile(in CompileInput) (map[string]interface{}, error)
	Capability() ProtocolCapability
}

// Registry maps protocol kind → Module.
type Registry struct {
	modules map[string]Module
}

func NewRegistry(mods ...Module) Registry {
	r := Registry{modules: make(map[string]Module, len(mods))}
	for _, m := range mods {
		if m == nil {
			continue
		}
		r.modules[strings.ToLower(m.Kind())] = m
	}
	return r
}

func DefaultCompileRegistry() Registry {
	return NewRegistry(
		VLESSCompiler{},
		VMessCompiler{},
		TrojanCompiler{},
		ShadowsocksCompiler{},
		Hysteria2Compiler{},
		TUICCompiler{kind: "tuic"},
		TUICCompiler{kind: "tuic-v4"},
		TUICCompiler{kind: "tuic-v5"},
		ShadowQUICCompiler{},
		GenericCompiler{kind: "snell"},
		AnyTLSCompiler{},
		MieruCompiler{},
		GenericCompiler{kind: "sudoku"},
		TrustTunnelCompiler{},
	)
}

func (r Registry) Compile(in CompileInput) (map[string]interface{}, error) {
	kind := strings.ToLower(strings.TrimSpace(in.Protocol))
	m, ok := r.modules[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported protocol %q", kind)
	}
	return m.Compile(in)
}

func (r Registry) Has(kind string) bool {
	_, ok := r.modules[strings.ToLower(strings.TrimSpace(kind))]
	return ok
}

// ParseConfigJSON decodes listener config JSON into a map.
func ParseConfigJSON(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("invalid config json: %w", err)
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

func baseMap(in CompileInput) map[string]interface{} {
	m := map[string]interface{}{
		"name":   in.Name,
		"type":   strings.ToLower(in.Protocol),
		"port":   in.Port,
		"listen": in.Listen,
	}
	if in.Proxy != "" {
		m["proxy"] = in.Proxy
	}
	if in.Rule != "" {
		m["rule"] = in.Rule
	}
	if in.RoutingMark > 0 {
		m["routing-mark"] = in.RoutingMark
	}
	return m
}

func copyConfigPassthrough(dst, cfg map[string]interface{}, skip map[string]struct{}) {
	for k, v := range cfg {
		if _, ok := skip[k]; ok {
			continue
		}
		// strip panel-only / client-export-only keys.
		// NOTE: "encryption" is a CLIENT-side field per proxies-vless wiki.
		// It is preserved in the panel Config map for client YAML emission
		// (converter/client.go reads p["encryption"]) but stripped from
		// the listener config.yaml via clientOnlyListenerKey. The server-side
		// counterpart is "decryption" only per inbound-vless wiki.
		if strings.HasPrefix(k, "_") || k == "access_profile" ||
			k == "transport_layer" || k == "security_layer" {
			continue
		}
		// Client-side TLS/hostname fields must not appear on listeners
		// (wiki inbound docs use certificate/reality/wrappers, not sni/skip-cert-verify).
		if clientOnlyListenerKey(k) {
			continue
		}
		dst[k] = v
	}
}

func clientOnlyListenerKey(k string) bool {
	switch k {
	case "sni", "servername", "skip-cert-verify", "name-cert-verify",
		"fingerprint", "client-fingerprint", "reality-opts",
		"shadow-tls-opts", "restls-opts", "jls-opts", "ss-opts",
		"plugin", "plugin-opts", "encryption":
		return true
	default:
		return false
	}
}

func managedKeys() map[string]struct{} {
	return map[string]struct{}{
		"name": {}, "type": {}, "port": {}, "listen": {}, "proxy": {}, "rule": {},
		"routing-mark": {}, "udp": {}, "tls": {}, "users": {}, "flow": {},
		"alterId": {},
	}
}

func asUsersArray(cfg map[string]interface{}, fromCreds []UserCred, field string, hasCredState bool) []map[string]interface{} {
	var out []map[string]interface{}
	if len(fromCreds) > 0 {
		out = make([]map[string]interface{}, 0, len(fromCreds))
		for _, c := range fromCreds {
			u := map[string]interface{}{}
			switch field {
			case "uuid":
				if c.UUID != "" {
					u["uuid"] = c.UUID
				}
				if c.Flow != "" {
					u["flow"] = c.Flow
				}
			case "password":
				if c.Username != "" {
					u["username"] = c.Username
					u["password"] = c.Password
				} else {
					u["password"] = c.Password
					if c.UUID != "" {
						u["uuid"] = c.UUID
					}
				}
			}
			if len(u) > 0 {
				out = append(out, u)
			}
		}
	} else {
		// Fall back to config users when panel credentials are empty.
		// Previously, hasCredState=true with empty fromCreds returned nil,
		// which caused Mihomo validation failure ("unset fields: users").
		// Now we fall back to config users so the listener is at least valid.
		// Panel-managed users take precedence when available; config users
		// are a safety net for decrypt failures, inactive users, etc.
		if raw, ok := cfg["users"]; ok {
			out = normalizeUsersValue(raw)
		}
	}
	if flow, ok := cfg["flow"].(string); ok && strings.TrimSpace(flow) != "" {
		for _, user := range out {
			if _, exists := user["flow"]; !exists {
				user["flow"] = flow
			}
		}
	}
	// Propagate top-level alterId to each user object. The official Mihomo
	// schema for VMess expects alterId inside users[].alterId; without it,
	// per-user alterId is silently dropped when panel credentials are bound.
	if alterId, ok := cfg["alterId"]; ok && alterId != nil {
		for _, user := range out {
			if _, exists := user["alterId"]; !exists {
				user["alterId"] = alterId
			}
		}
	}
	return out
}

// asUsersMap returns users as a map{username: password} for protocols whose
// Mihomo listener schema expects that shape (hysteria2, anytls, mieru).
func asUsersMap(cfg map[string]interface{}, fromCreds []UserCred, hasCredState bool) map[string]interface{} {
	if len(fromCreds) > 0 {
		out := make(map[string]interface{}, len(fromCreds))
		for _, c := range fromCreds {
			name := c.Username
			if name == "" {
				name = "default"
			}
			out[name] = c.Password
		}
		return out
	}
	// Fall back to config users when panel credentials are empty.
	// See asUsersArray for rationale.
	if raw, ok := cfg["users"]; ok {
		switch users := raw.(type) {
		case map[string]interface{}:
			return users
		case map[interface{}]interface{}:
			out := make(map[string]interface{}, len(users))
			for k, v := range users {
				out[fmt.Sprint(k)] = fmt.Sprint(v)
			}
			return out
		}
	}
	return nil
}

// asUsersMapUUID returns users as a map{uuid: password} for protocols whose
// Mihomo listener schema expects that shape (TUIC v5: users is a map keyed by UUID).
// Falls back to Username when UUID is empty (legacy panel users without UUID).
func asUsersMapUUID(cfg map[string]interface{}, fromCreds []UserCred, hasCredState bool) map[string]interface{} {
	if len(fromCreds) > 0 {
		out := make(map[string]interface{}, len(fromCreds))
		for _, c := range fromCreds {
			key := c.UUID
			if key == "" {
				key = c.Username
			}
			if key == "" {
				key = "default"
			}
			out[key] = c.Password
		}
		return out
	}
	if raw, ok := cfg["users"]; ok {
		switch users := raw.(type) {
		case map[string]interface{}:
			return users
		case map[interface{}]interface{}:
			out := make(map[string]interface{}, len(users))
			for k, v := range users {
				out[fmt.Sprint(k)] = fmt.Sprint(v)
			}
			return out
		}
	}
	return nil
}

func normalizeUsersValue(value interface{}) []map[string]interface{} {
	switch users := value.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(users))
		for _, item := range users {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]interface{}:
		return users
	case map[string]interface{}:
		out := make([]map[string]interface{}, 0, len(users))
		for username, password := range users {
			out = append(out, map[string]interface{}{"username": username, "password": fmt.Sprint(password)})
		}
		return out
	case map[interface{}]interface{}:
		out := make([]map[string]interface{}, 0, len(users))
		for rawUsername, rawPassword := range users {
			out = append(out, map[string]interface{}{"username": fmt.Sprint(rawUsername), "password": fmt.Sprint(rawPassword)})
		}
		return out
	default:
		return nil
	}
}

func portValue(port string) interface{} {
	s := strings.TrimSpace(port)
	if p, err := strconv.Atoi(s); err == nil {
		return p
	}
	return s
}
