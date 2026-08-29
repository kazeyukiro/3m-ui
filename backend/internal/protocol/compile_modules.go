package protocol

import "fmt"

// GenericCompiler passes config through with optional users injection.
type GenericCompiler struct{ kind string }

func (g GenericCompiler) Kind() string { return g.kind }
func (g GenericCompiler) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: g.kind, Label: g.kind}
}
func (g GenericCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	if in.UDP {
		m["udp"] = true
	}
	if in.TLS {
		m["tls"] = true
	}
	return m, nil
}

type VLESSCompiler struct{}

func (VLESSCompiler) Kind() string                   { return "vless" }
func (VLESSCompiler) Capability() ProtocolCapability { return vlessCapability() }
func (VLESSCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	skip := managedKeys()
	copyConfigPassthrough(m, in.Config, skip)
	if in.UDP {
		m["udp"] = true
	}
	// Reality: never set top-level tls
	if _, hasReality := in.Config["reality-config"]; hasReality {
		delete(m, "tls")
	} else if in.TLS {
		m["tls"] = true
	} else {
		delete(m, "tls")
	}
	users := asUsersArray(in.Config, in.Users, "uuid", in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}

type VMessCompiler struct{}

func (VMessCompiler) Kind() string                   { return "vmess" }
func (VMessCompiler) Capability() ProtocolCapability { return vmessCapability() }
func (VMessCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	if in.UDP {
		m["udp"] = true
	}
	if _, hasReality := in.Config["reality-config"]; hasReality {
		delete(m, "tls")
	} else if in.TLS {
		m["tls"] = true
	} else {
		delete(m, "tls")
	}
	users := asUsersArray(in.Config, in.Users, "uuid", in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}

type TrojanCompiler struct{}

func (TrojanCompiler) Kind() string                   { return "trojan" }
func (TrojanCompiler) Capability() ProtocolCapability { return trojanCapability() }
func (TrojanCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	if in.UDP {
		m["udp"] = true
	}
	if _, hasReality := in.Config["reality-config"]; hasReality {
		delete(m, "tls")
	} else if in.TLS {
		m["tls"] = true
	} else {
		delete(m, "tls")
	}
	users := asUsersArray(in.Config, in.Users, "password", in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}

type ShadowsocksCompiler struct{}

func (ShadowsocksCompiler) Kind() string                   { return "shadowsocks" }
func (ShadowsocksCompiler) Capability() ProtocolCapability { return shadowsocksCapability() }
func (ShadowsocksCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	if in.UDP {
		m["udp"] = true
	}
	if len(in.Users) > 1 {
		return nil, fmt.Errorf("shadowsocks supports one password; %d credentials bound", len(in.Users))
	}
	if len(in.Users) == 1 && in.Users[0].Password != "" {
		m["password"] = in.Users[0].Password
	}
	return m, nil
}

type Hysteria2Compiler struct{}

func (Hysteria2Compiler) Kind() string                   { return "hysteria2" }
func (Hysteria2Compiler) Capability() ProtocolCapability { return hysteria2Capability() }
func (Hysteria2Compiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	users := asUsersMap(in.Config, in.Users, in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}

type TUICCompiler struct{}

func (TUICCompiler) Kind() string                   { return "tuic" }
func (TUICCompiler) Capability() ProtocolCapability { return tuicCapability() }
func (TUICCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	// TUIC v4 uses `token` (array of strings) — passed through by copyConfigPassthrough.
	// TUIC v5 uses `users` as a map{UUID: PASSWORD} (verified against Mihomo wiki).
	// We emit the v5 map form when credentials are bound. If the operator configured
	// `token` directly in the listener config, it takes precedence (passthrough).
	if _, hasToken := in.Config["token"]; !hasToken {
		users := asUsersMapUUID(in.Config, in.Users, in.HasCredentialState)
		if len(users) > 0 {
			m["users"] = users
		}
	}
	return m, nil
}

type ShadowQUICCompiler struct{}

func (ShadowQUICCompiler) Kind() string                   { return "shadowquic" }
func (ShadowQUICCompiler) Capability() ProtocolCapability { return shadowquicCapability() }
func (ShadowQUICCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	users := asUsersArray(in.Config, in.Users, "password", in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}

type TrustTunnelCompiler struct{}

func (TrustTunnelCompiler) Kind() string { return "trusttunnel" }
func (TrustTunnelCompiler) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: "trusttunnel", Label: "trusttunnel"}
}
func (TrustTunnelCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	// TrustTunnel uses users: [{username, password}] (array form, verified against Mihomo wiki).
	users := asUsersArray(in.Config, in.Users, "password", in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}

type AnyTLSCompiler struct{}

func (AnyTLSCompiler) Kind() string { return "anytls" }
func (AnyTLSCompiler) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: "anytls", Label: "anytls"}
}
func (AnyTLSCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	if in.UDP {
		m["udp"] = true
	}
	users := asUsersMap(in.Config, in.Users, in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}

type MieruCompiler struct{}

func (MieruCompiler) Kind() string { return "mieru" }
func (MieruCompiler) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: "mieru", Label: "mieru"}
}
func (MieruCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	copyConfigPassthrough(m, in.Config, managedKeys())
	users := asUsersMap(in.Config, in.Users, in.HasCredentialState)
	if len(users) > 0 {
		m["users"] = users
	}
	return m, nil
}
