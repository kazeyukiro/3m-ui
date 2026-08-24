package listener

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"golang.org/x/crypto/curve25519"
)

// QuickSetupInput is the one-click inbound wizard payload.
// Operators pick a preset and may override generated credentials / port / SNI.
type QuickSetupInput struct {
	Preset      string `json:"preset" binding:"required"` // vless-reality | shadowsocks | hysteria2 | trojan-reality
	Name        string `json:"name"`
	Port        int    `json:"port"` // 0 = auto
	BindAddress string `json:"bind_address"`
	PublicHost  string `json:"public_host"`
	PublicPort  string `json:"public_port"`
	SNI         string `json:"sni"` // Reality dest / server-names
	UUID        string `json:"uuid"`
	Password    string `json:"password"`
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	ShortID     string `json:"short_id"`
	Flow        string `json:"flow"`
	Method      string `json:"method"` // shadowsocks
	Enabled     *bool  `json:"enabled"`
}

// QuickSetupResult is returned after creating the listener.
type QuickSetupResult struct {
	Listener *models.Listener  `json:"listener"`
	Hints    map[string]string `json:"hints"` // non-secret tips for the operator
}

func (s *Service) QuickSetup(in QuickSetupInput) (*QuickSetupResult, error) {
	preset := strings.ToLower(strings.TrimSpace(in.Preset))
	if preset == "" {
		return nil, fmt.Errorf("preset is required")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "quick-" + preset
	}
	bind := strings.TrimSpace(in.BindAddress)
	if bind == "" {
		bind = "0.0.0.0"
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	port := in.Port
	if port <= 0 {
		p, err := s.pickFreePort()
		if err != nil {
			return nil, err
		}
		port = p
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid port")
	}

	var protocol string
	var cfg map[string]interface{}
	hints := map[string]string{}

	switch preset {
	case "vless-reality", "reality", "vless":
		protocol = "vless"
		sni := strings.TrimSpace(in.SNI)
		if sni == "" {
			sni = "www.microsoft.com"
		}
		uid := strings.TrimSpace(in.UUID)
		if uid == "" {
			uid = uuid.NewString()
		}
		flow := strings.TrimSpace(in.Flow)
		if flow == "" {
			flow = "xtls-rprx-vision"
		}
		priv, pub, err := resolveRealityKeys(in.PrivateKey, in.PublicKey)
		if err != nil {
			return nil, err
		}
		sid := strings.TrimSpace(in.ShortID)
		if sid == "" {
			sid = randomHex(8)
		}
		// Official mihomo RealityConfig: short-id is []string; public-key is client-only.
		cfg = map[string]interface{}{
			"users": []map[string]interface{}{
				{"username": "default", "uuid": uid, "flow": flow},
			},
			"reality-config": map[string]interface{}{
				"dest":         sni + ":443",
				"private-key":  priv,
				"short-id":     []string{sid},
				"server-names": []string{sni},
			},
		}
		hints["uuid"] = uid
		hints["public_key"] = pub
		hints["short_id"] = sid
		hints["sni"] = sni
		hints["flow"] = flow
		hints["note"] = "VLESS + REALITY + Vision. No domain/cert required. Edit dest/SNI if needed."

	case "shadowsocks", "ss", "ss2022":
		protocol = "shadowsocks"
		method := strings.TrimSpace(in.Method)
		if method == "" {
			method = "2022-blake3-aes-128-gcm"
		}
		pass := strings.TrimSpace(in.Password)
		if pass == "" {
			pass = ssPasswordForCipher(method)
		}
		cfg = map[string]interface{}{
			"cipher":   method,
			"password": pass,
			"udp":      true,
		}
		hints["password"] = pass
		hints["method"] = method
		hints["note"] = "Shadowsocks. Change cipher/password in advanced config if needed."

	case "hysteria2", "hy2":
		protocol = "hysteria2"
		pass := strings.TrimSpace(in.Password)
		if pass == "" {
			pass = randomPassword(16)
		}
		cfg = map[string]interface{}{
			"users": map[string]interface{}{
				"default": pass,
			},
			// Operators should attach real certs later; allow-insecure for lab only.
			"ignore-client-bandwidth": true,
		}
		hints["password"] = pass
		hints["note"] = "Hysteria2 skeleton. Add certificate + private-key (or TLS via reverse proxy) before production use."

	case "trojan-reality":
		protocol = "trojan"
		sni := strings.TrimSpace(in.SNI)
		if sni == "" {
			sni = "www.microsoft.com"
		}
		pass := strings.TrimSpace(in.Password)
		if pass == "" {
			pass = randomPassword(16)
		}
		priv, pub, err := resolveRealityKeys(in.PrivateKey, in.PublicKey)
		if err != nil {
			return nil, err
		}
		sid := strings.TrimSpace(in.ShortID)
		if sid == "" {
			sid = randomHex(8)
		}
		cfg = map[string]interface{}{
			"users": []map[string]interface{}{
				{"username": "default", "password": pass},
			},
			"reality-config": map[string]interface{}{
				"dest":         sni + ":443",
				"private-key":  priv,
				"short-id":     []string{sid},
				"server-names": []string{sni},
			},
		}
		hints["password"] = pass
		hints["public_key"] = pub
		hints["short_id"] = sid
		hints["sni"] = sni
		hints["note"] = "Trojan + REALITY. Fine-tune server-names / dest as needed."

	default:
		return nil, fmt.Errorf("unknown preset %q (supported: vless-reality, shadowsocks, hysteria2, trojan-reality)", preset)
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	l := &models.Listener{
		Name:        name,
		Protocol:    protocol,
		Type:        protocol,
		Port:        strconv.Itoa(port),
		BindAddress: bind,
		Enabled:     enabled,
		Config:      string(raw),
		PublicHost:  strings.TrimSpace(in.PublicHost),
		PublicPort:  strings.TrimSpace(in.PublicPort),
	}
	if err := s.Create(l); err != nil {
		return nil, err
	}
	return &QuickSetupResult{Listener: l, Hints: hints}, nil
}

func (s *Service) pickFreePort() (int, error) {
	used := map[string]struct{}{}
	var list []models.Listener
	if err := s.db.Select("port").Find(&list).Error; err != nil {
		return 0, err
	}
	for _, l := range list {
		used[strings.TrimSpace(l.Port)] = struct{}{}
	}
	// Prefer 443 then random high ports.
	candidates := []int{443, 8443, 2053, 2083, 2087, 2096}
	for _, p := range candidates {
		if _, ok := used[strconv.Itoa(p)]; !ok {
			return p, nil
		}
	}
	buf := make([]byte, 2)
	for i := 0; i < 64; i++ {
		if _, err := rand.Read(buf); err != nil {
			return 0, err
		}
		p := 10000 + int(buf[0])<<8 + int(buf[1])%20000
		if p > 65535 {
			p = 10000 + p%50000
		}
		if _, ok := used[strconv.Itoa(p)]; !ok {
			return p, nil
		}
	}
	return 0, fmt.Errorf("could not find a free port")
}

func resolveRealityKeys(privateKey, publicKey string) (priv, pub string, err error) {
	priv = strings.TrimSpace(privateKey)
	pub = strings.TrimSpace(publicKey)
	if priv != "" {
		if pub == "" {
			pub, err = derivePublicKey(priv)
			if err != nil {
				return "", "", err
			}
		}
		return priv, pub, nil
	}
	// Generate X25519 key pair (32-byte seed), encode as raw base64 (no padding).
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return "", "", err
	}
	// Clamp like X25519 private keys.
	seed[0] &= 248
	seed[31] &= 127
	seed[31] |= 64
	pubBytes, err := curve25519.X25519(seed[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	priv = base64.RawURLEncoding.EncodeToString(seed[:])
	pub = base64.RawURLEncoding.EncodeToString(pubBytes)
	return priv, pub, nil
}

func derivePublicKey(private string) (string, error) {
	var raw []byte
	var err error
	for _, decode := range []func(string) ([]byte, error){
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
	} {
		raw, err = decode(strings.TrimSpace(private))
		if err == nil && len(raw) == 32 {
			break
		}
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid private key: need 32 bytes")
	}
	pub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(pub), nil
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomPassword(n int) string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out)
}

// ssPasswordForCipher generates a password matching MetaCubeX SS docs:
// 2022-blake3-aes-128-gcm → 16 random bytes base64;
// 2022-blake3-aes-256-gcm / 2022-blake3-chacha20-poly1305 → 32 random bytes base64;
// other ciphers → arbitrary string.
func ssPasswordForCipher(method string) string {
	n := 0
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "2022-blake3-aes-128-gcm":
		n = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		n = 32
	default:
		return randomPassword(16)
	}
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}
