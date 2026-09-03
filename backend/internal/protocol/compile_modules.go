package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

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
	// Per official MetaCubeX wiki (inbound-vless), the listener YAML does NOT
	// carry a top-level `udp` field — mihomo allows UDP by default when absent.
	delete(m, "udp")
	// Official VLESS listener has no top-level "tls" field; TLS is implied by
	// certificate/private-key, reality-config, or wrappers — never emit tls: true.
	delete(m, "tls")
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
	// Per official MetaCubeX wiki (inbound-vmess), the listener YAML does NOT
	// carry a top-level `udp` field — mihomo allows UDP by default when absent.
	delete(m, "udp")
	// Official VMess listener: no top-level tls boolean.
	delete(m, "tls")
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
	// Per official MetaCubeX wiki (inbound-trojan), the listener YAML does NOT
	// carry a top-level `udp` field — mihomo allows UDP by default when absent.
	delete(m, "udp")
	// Official Trojan listener: no top-level tls boolean.
	delete(m, "tls")
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

type TUICCompiler struct{ kind string }

func (t TUICCompiler) Kind() string {
	if t.kind != "" {
		return t.kind
	}
	return "tuic"
}
func (TUICCompiler) Capability() ProtocolCapability { return tuicCapability() }
func (t TUICCompiler) Compile(in CompileInput) (map[string]interface{}, error) {
	m := baseMap(in)
	// Mihomo's listener type is always "tuic" regardless of v4/v5.
	m["type"] = "tuic"
	copyConfigPassthrough(m, in.Config, managedKeys())
	// Mihomo expects token as a slice. Legacy configs may store a single string.
	if tok, ok := m["token"].(string); ok {
		m["token"] = []string{tok}
	}
	kind := t.Kind()
	if kind == "tuic-v4" {
		// TUIC v4: token-based auth. Ensure token is present.
		if _, hasToken := m["token"]; !hasToken {
			// Generate a default token if none set.
			m["token"] = []string{randomToken()}
		}
		// Remove users — v4 uses token, not users.
		delete(m, "users")
	} else {
		// TUIC v5 (or generic tuic): users map{UUID: PASSWORD}.
		// If token is present, it takes precedence (v4 compat mode).
		if _, hasToken := m["token"]; !hasToken {
			users := asUsersMapUUID(in.Config, in.Users, in.HasCredentialState)
			if len(users) > 0 {
				m["users"] = users
			}
		}
	}
	// Sensible defaults when operator left advanced QUIC fields empty.
	if _, ok := m["alpn"]; !ok {
		m["alpn"] = []string{"h3"}
	}
	if _, ok := m["congestion-controller"]; !ok {
		m["congestion-controller"] = "bbr"
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
	if _, ok := m["congestion-controller"]; !ok {
		m["congestion-controller"] = "cubic"
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
	// Mihomo expects network as a slice (e.g. ["tcp", "udp"]).
	// Legacy configs may store it as a string; convert to slice.
	if net, ok := m["network"].(string); ok {
		m["network"] = []string{net}
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

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
