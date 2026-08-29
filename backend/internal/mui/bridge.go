package mui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
	muiprotocol "github.com/kazeyukiro/3m-ui/backend/internal/mui/protocol"
	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"
)

// Cred is a panel-side credential used when adapting Listeners to m-ui nodes.
type Cred struct {
	Username string
	Password string
	UUID     string
	Flow     string
}

// ListenerToNode adapts a 3m-ui Listener + credentials into an m-ui domain.Node.
func ListenerToNode(l models.Listener, creds []Cred) (domain.Node, error) {
	cfg := map[string]interface{}{}
	if strings.TrimSpace(l.Config) != "" {
		if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
			return domain.Node{}, fmt.Errorf("listener config: %w", err)
		}
	}
	proto := domain.ProtocolKind(strings.ToLower(strings.TrimSpace(l.Protocol)))
	nodeID := fmt.Sprintf("%d", l.ID)
	node := domain.Node{
		ID:            nodeID,
		Name:          l.Name,
		Enabled:       l.Enabled,
		ListenAddress: firstNonEmpty(l.Listen, l.BindAddress, "0.0.0.0"),
		// Listen port must come from the listener bind port, not PublicPort (NAT map).
		Port:          firstNonEmpty(strings.TrimSpace(l.Port), "0"),
		Protocol:      proto,
		SchemaVersion: domain.NodeSchemaVersion,
	}

	// Access profile
	pubPort := uint16(0)
	if p := firstNonEmpty(l.PublicPort, l.Port); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			pubPort = uint16(n)
		}
	}
	profile := domain.AccessProfile{
		ID:            nodeID + "-default",
		NodeID:        nodeID,
		Name:          "default",
		Default:       true,
		PublicHost:    strings.TrimSpace(l.PublicHost),
		PublicPort:    pubPort,
		ServerName:    strings.TrimSpace(l.AccessSNI),
		Fingerprint:   strings.TrimSpace(l.ClientFingerprint),
		AllowInsecure: boolCfg(cfg, "skip-cert-verify"),
	}
	if profile.ServerName == "" {
		profile.ServerName = strCfg(cfg, "sni", "servername")
	}
	if profile.Fingerprint == "" {
		profile.Fingerprint = strCfg(cfg, "client-fingerprint", "fingerprint")
	}
	if profile.Fingerprint == "" {
		profile.Fingerprint = domain.ClientFingerprint
	}
	node.AccessProfiles = []domain.AccessProfile{profile}

	// Users from panel credentials
	for i, c := range creds {
		u := domain.NodeUser{
			ID:      fmt.Sprintf("%s-u%d", nodeID, i+1),
			NodeID:  nodeID,
			Name:    firstNonEmpty(c.Username, fmt.Sprintf("user-%d", i+1)),
			Enabled: true,
		}
		switch proto {
		case domain.ProtocolVLESS:
			u.VLESS = &domain.VLESSCredential{UUID: c.UUID, Flow: c.Flow}
		case domain.ProtocolVMess:
			u.VMess = &domain.VMessCredential{UUID: c.UUID, Cipher: "auto"}
		case domain.ProtocolTrojan:
			u.Trojan = &domain.TrojanCredential{Password: c.Password}
		case domain.ProtocolHysteria2:
			u.Hysteria2 = &domain.Hysteria2Credential{Password: c.Password}
		case domain.ProtocolShadowsocks:
			u.Shadowsocks = &domain.ShadowsocksCredential{Password: c.Password}
		}
		node.Users = append(node.Users, u)
	}

	// Fall back to users embedded in listener config JSON when no panel credentials.
	if len(node.Users) == 0 {
		if rawUsers, ok := cfg["users"].([]interface{}); ok {
			for i, item := range rawUsers {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				u := domain.NodeUser{
					ID:      fmt.Sprintf("%s-cfg%d", nodeID, i+1),
					NodeID:  nodeID,
					Name:    firstNonEmpty(strMap(m, "username"), strMap(m, "name"), fmt.Sprintf("user-%d", i+1)),
					Enabled: true,
				}
				switch proto {
				case domain.ProtocolVLESS:
					u.VLESS = &domain.VLESSCredential{UUID: strMap(m, "uuid"), Flow: strMap(m, "flow")}
				case domain.ProtocolVMess:
					u.VMess = &domain.VMessCredential{UUID: strMap(m, "uuid"), Cipher: "auto"}
				case domain.ProtocolTrojan:
					u.Trojan = &domain.TrojanCredential{Password: strMap(m, "password")}
				case domain.ProtocolHysteria2:
					u.Hysteria2 = &domain.Hysteria2Credential{Password: strMap(m, "password")}
				case domain.ProtocolShadowsocks:
					u.Shadowsocks = &domain.ShadowsocksCredential{Password: strMap(m, "password")}
				}
				node.Users = append(node.Users, u)
			}
		}
	}

	if proto == domain.ProtocolShadowsocks && len(node.Users) == 0 {
		if pass := strCfg(cfg, "password"); pass != "" {
			node.Users = append(node.Users, domain.NodeUser{
				ID: nodeID + "-ss", NodeID: nodeID, Name: "default", Enabled: true,
				Shadowsocks: &domain.ShadowsocksCredential{Password: pass},
			})
		}
	}

	// Protocol specs from flat Mihomo listener JSON
	switch proto {
	case domain.ProtocolVLESS:
		node.VLESS = decodeVLESS(cfg, l.TLS)
	case domain.ProtocolVMess:
		node.VMess = decodeVMess(cfg, l.TLS)
	case domain.ProtocolTrojan:
		node.Trojan = decodeTrojan(cfg, l.TLS)
	case domain.ProtocolShadowsocks:
		node.Shadowsocks = decodeSS(cfg)
	case domain.ProtocolHysteria2:
		node.Hysteria2 = decodeHy2(cfg)
	default:
		return domain.Node{}, fmt.Errorf("protocol %q is not supported by m-ui port", proto)
	}
	return node, nil
}

// BuildShare adapts a listener and builds an m-ui share for the first/default user,
// or for all users when building URI lists.
func BuildShares(l models.Listener, publicHost string, creds []Cred) ([]muiprotocol.Share, error) {
	node, err := ListenerToNode(l, creds)
	if err != nil {
		return nil, err
	}
	if publicHost != "" {
		for i := range node.AccessProfiles {
			if node.AccessProfiles[i].Default {
				node.AccessProfiles[i].PublicHost = publicHost
			}
		}
	}
	profile, ok := node.DefaultAccessProfile()
	if !ok {
		return nil, fmt.Errorf("access profile missing")
	}
	if profile.PublicHost == "" {
		profile.PublicHost = publicHost
	}
	enrichAccessProfileFromNode(&profile, node, l)
	flowDefault := listenerFlowHint(l)
	state := domain.DesiredState{AsOf: time.Now().UTC(), PublicHost: profile.PublicHost}
	reg := muiprotocol.DefaultRegistry()
	out := make([]muiprotocol.Share, 0, len(node.Users))
	for _, user := range node.Users {
		// Ensure user/profile node id match for registry checks.
		user.NodeID = node.ID
		profile.NodeID = node.ID
		if user.VLESS != nil && strings.TrimSpace(user.VLESS.Flow) == "" && flowDefault != "" {
			user.VLESS.Flow = flowDefault
		}
		s, err := reg.BuildShare(state, node, user, profile)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// CompileListener compiles a listener via m-ui protocol modules into a YAML-ready map.
func CompileListener(l models.Listener, creds []Cred) (map[string]interface{}, error) {
	node, err := ListenerToNode(l, creds)
	if err != nil {
		return nil, err
	}
	compiled, err := muiprotocol.DefaultRegistry().Compile(context.Background(), node, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	// m-ui listener structs use yaml tags; JSON would emit PascalCase field names.
	raw, err := yaml.Marshal(compiled)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func decodeVLESS(cfg map[string]interface{}, tls bool) *domain.VLESSSpec {
	spec := &domain.VLESSSpec{
		Decryption: strCfg(cfg, "encryption", "decryption"),
		Handler:    decodeHandler(cfg),
		Security:   decodeSecurity(cfg, tls),
	}
	if spec.Decryption == "" {
		spec.Decryption = "none"
	}
	return spec
}

func decodeVMess(cfg map[string]interface{}, tls bool) *domain.VMessSpec {
	return &domain.VMessSpec{Handler: decodeHandler(cfg), Security: decodeSecurity(cfg, tls)}
}

func decodeTrojan(cfg map[string]interface{}, tls bool) *domain.TrojanSpec {
	return &domain.TrojanSpec{Handler: decodeHandler(cfg), Security: decodeSecurity(cfg, tls)}
}

func decodeSS(cfg map[string]interface{}) *domain.ShadowsocksSpec {
	return &domain.ShadowsocksSpec{
		Cipher: strCfg(cfg, "cipher"),
		UDP:    boolCfg(cfg, "udp"),
	}
}

func decodeHy2(cfg map[string]interface{}) *domain.Hysteria2Spec {
	return &domain.Hysteria2Spec{
		Certificate:  strCfg(cfg, "certificate"),
		PrivateKey:   strCfg(cfg, "private-key"),
		Up:           strCfg(cfg, "up"),
		Down:         strCfg(cfg, "down"),
		Obfs:         strCfg(cfg, "obfs"),
		ObfsPassword: strCfg(cfg, "obfs-password"),
	}
}

func decodeHandler(cfg map[string]interface{}) domain.VLESSHandlerSpec {
	h := domain.VLESSHandlerSpec{Type: domain.VLESSHandlerRaw}
	if path := strCfg(cfg, "ws-path"); path != "" {
		h.Type = domain.VLESSHandlerWebSocket
		h.WebSocket = &domain.WebSocketSpec{Path: path}
	}
	if svc := strCfg(cfg, "grpc-service-name"); svc != "" {
		h.Type = domain.VLESSHandlerGRPC
		h.GRPC = &domain.GRPCSpec{ServiceName: svc}
	}
	if xhttp, ok := cfg["xhttp-config"].(map[string]interface{}); ok {
		h.Type = domain.VLESSHandlerXHTTP
		h.XHTTP = &domain.XHTTPConfig{Path: strMap(xhttp, "path"), Host: strMap(xhttp, "host"), Mode: strMap(xhttp, "mode")}
	}
	return h
}

func decodeSecurity(cfg map[string]interface{}, tlsFlag bool) domain.VLESSSecuritySpec {
	if raw, ok := cfg["reality-config"].(map[string]interface{}); ok && raw != nil {
		shortIDs := []string{}
		if s, ok := raw["short-id"].(string); ok && s != "" {
			shortIDs = []string{s}
		} else if arr, ok := raw["short-id"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					shortIDs = append(shortIDs, s)
				}
			}
		}
		names := []string{}
		if arr, ok := raw["server-names"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					names = append(names, s)
				}
			}
		}
		priv := strMap(raw, "private-key")
		pub := strMap(raw, "public-key")
		if pub == "" && priv != "" {
			if derived, err := deriveRealityPublicKey(priv); err == nil {
				pub = derived
			}
		}
		return domain.VLESSSecuritySpec{
			Type: domain.VLESSSecurityReality,
			Reality: &domain.RealityConfig{
				PrivateKey:  priv,
				PublicKey:   pub,
				ShortIDs:    shortIDs,
				ServerNames: names,
				Destination: strMap(raw, "dest"),
			},
		}
	}
	if tlsFlag || strCfg(cfg, "certificate") != "" {
		return domain.VLESSSecuritySpec{
			Type: domain.VLESSSecurityTLS,
			TLS: &domain.TLSConfig{
				Certificate:   strCfg(cfg, "certificate"),
				PrivateKey:    strCfg(cfg, "private-key"),
				AllowInsecure: boolCfg(cfg, "skip-cert-verify"),
			},
		}
	}
	return domain.VLESSSecuritySpec{Type: domain.VLESSSecurityNone}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func strCfg(cfg map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := cfg[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
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

func boolCfg(cfg map[string]interface{}, key string) bool {
	b, _ := cfg[key].(bool)
	return b
}


func enrichAccessProfileFromNode(profile *domain.AccessProfile, node domain.Node, l models.Listener) {
	if profile == nil {
		return
	}
	if profile.Fingerprint == "" {
		profile.Fingerprint = firstNonEmpty(strings.TrimSpace(l.ClientFingerprint), domain.ClientFingerprint)
	}
	if profile.ServerName == "" {
		profile.ServerName = strings.TrimSpace(l.AccessSNI)
	}
	if profile.ServerName == "" && node.VLESS != nil && node.VLESS.Security.Reality != nil {
		if names := node.VLESS.Security.Reality.ServerNames; len(names) > 0 {
			profile.ServerName = names[0]
		}
	}
	if profile.ServerName == "" && node.VMess != nil && node.VMess.Security.Reality != nil {
		if names := node.VMess.Security.Reality.ServerNames; len(names) > 0 {
			profile.ServerName = names[0]
		}
	}
	if profile.ServerName == "" && node.Trojan != nil && node.Trojan.Security.Reality != nil {
		if names := node.Trojan.Security.Reality.ServerNames; len(names) > 0 {
			profile.ServerName = names[0]
		}
	}
	ensureRealityPublicKey(node.VLESS)
	ensureRealityPublicKeyVMess(node.VMess)
	ensureRealityPublicKeyTrojan(node.Trojan)
}

func ensureRealityPublicKey(spec *domain.VLESSSpec) {
	if spec == nil || spec.Security.Reality == nil {
		return
	}
	r := spec.Security.Reality
	if r.PublicKey == "" && r.PrivateKey != "" {
		if pub, err := deriveRealityPublicKey(r.PrivateKey); err == nil {
			r.PublicKey = pub
		}
	}
}

func ensureRealityPublicKeyVMess(spec *domain.VMessSpec) {
	if spec == nil || spec.Security.Reality == nil {
		return
	}
	r := spec.Security.Reality
	if r.PublicKey == "" && r.PrivateKey != "" {
		if pub, err := deriveRealityPublicKey(r.PrivateKey); err == nil {
			r.PublicKey = pub
		}
	}
}

func ensureRealityPublicKeyTrojan(spec *domain.TrojanSpec) {
	if spec == nil || spec.Security.Reality == nil {
		return
	}
	r := spec.Security.Reality
	if r.PublicKey == "" && r.PrivateKey != "" {
		if pub, err := deriveRealityPublicKey(r.PrivateKey); err == nil {
			r.PublicKey = pub
		}
	}
}

func listenerFlowHint(l models.Listener) string {
	if strings.TrimSpace(l.Config) == "" {
		return ""
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
		return ""
	}
	return strCfg(cfg, "flow")
}

func deriveRealityPublicKey(private string) (string, error) {
	private = strings.TrimSpace(private)
	if private == "" {
		return "", fmt.Errorf("empty private key")
	}
	var raw []byte
	var err error
	for _, decode := range []func(string) ([]byte, error){
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
	} {
		raw, err = decode(private)
		if err == nil && len(raw) == 32 {
			break
		}
		raw = nil
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid Reality private key")
	}
	pub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(pub), nil
}
