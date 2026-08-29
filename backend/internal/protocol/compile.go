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
		TUICCompiler{},
		ShadowQUICCompiler{},
		GenericCompiler{kind: "snell"},
		AnyTLSCompiler{},
		MieruCompiler{},
		GenericCompiler{kind: "sudoku"},
		GenericCompiler{kind: "trusttunnel"},
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
		// strip panel-only / client-export-only keys
		if strings.HasPrefix(k, "_") || k == "access_profile" || k == "encryption" ||
			k == "transport_layer" || k == "security_layer" {
			continue
		}
		dst[k] = v
	}
}

func managedKeys() map[string]struct{} {
	return map[string]struct{}{
		"name": {}, "type": {}, "port": {}, "listen": {}, "proxy": {}, "rule": {},
		"routing-mark": {}, "udp": {}, "tls": {}, "users": {}, "flow": {},
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
		// Explicit empty credential state must not fall back to config users.
		if hasCredState {
			return nil
		}
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
	if hasCredState {
		return nil
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
