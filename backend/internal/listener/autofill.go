package listener

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"golang.org/x/crypto/curve25519"
)

// AutofillListenerDefaults fills empty credential / REALITY fields on create
// so operators need not use the removed one-click wizard.
func AutofillListenerDefaults(l *models.Listener) error {
	if l == nil {
		return nil
	}
	proto := strings.ToLower(strings.TrimSpace(l.Protocol))
	if proto == "" {
		proto = strings.ToLower(strings.TrimSpace(l.Type))
	}
	cfg := map[string]interface{}{}
	if strings.TrimSpace(l.Config) != "" {
		if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
			return fmt.Errorf("invalid config json: %w", err)
		}
	}

	switch proto {
	case "vless", "vmess":
		if err := autofillUUIDUsers(cfg); err != nil {
			return err
		}
	case "trojan", "hysteria2", "anytls", "mieru", "tuic", "shadowquic", "sudoku", "trusttunnel":
		autofillPasswordUsers(cfg)
	case "shadowsocks":
		cipher, _ := cfg["cipher"].(string)
		if cipher == "" {
			cipher = "aes-128-gcm"
			cfg["cipher"] = cipher
		}
		if pass, _ := cfg["password"].(string); strings.TrimSpace(pass) == "" {
			cfg["password"] = ssPasswordForCipher(cipher)
		}
	case "snell":
		if psk, _ := cfg["psk"].(string); strings.TrimSpace(psk) == "" {
			cfg["psk"] = randomPassword(16)
		}
	}

	// REALITY material when security uses reality-config or panel layer is reality.
	if needsReality(cfg) {
		if err := autofillReality(cfg); err != nil {
			return err
		}
	}

	// Hysteria2: optional self-signed cert when neither certificate nor private-key set.
	if proto == "hysteria2" {
		cert, _ := cfg["certificate"].(string)
		key, _ := cfg["private-key"].(string)
		if strings.TrimSpace(cert) == "" && strings.TrimSpace(key) == "" {
			host := "localhost"
			if sni, _ := cfg["sni"].(string); strings.TrimSpace(sni) != "" {
				host = strings.TrimSpace(sni)
			}
			certPEM, keyPEM, err := generateSelfSignedTLS(host)
			if err != nil {
				return err
			}
			cfg["certificate"] = certPEM
			cfg["private-key"] = keyPEM
		}
	}

	sanitizeServerConfig(cfg)

	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	l.Config = string(raw)
	return nil
}

// sanitizeServerConfig drops panel-only and client-only keys that must not
// reach Mihomo listener validation / config generation.
func sanitizeServerConfig(cfg map[string]interface{}) {
	delete(cfg, "security_layer")
	delete(cfg, "transport_layer")
	delete(cfg, "access_profile")
	delete(cfg, "sni") // client/export hint; server uses reality dest / cert SNI separately
	if raw, ok := cfg["reality-config"].(map[string]interface{}); ok && raw != nil {
		delete(raw, "public-key")
		delete(raw, "public_key")
		cfg["reality-config"] = raw
	}
}

func needsReality(cfg map[string]interface{}) bool {
	if _, ok := cfg["reality-config"]; ok {
		return true
	}
	if s, ok := cfg["security_layer"].(string); ok && strings.EqualFold(s, "reality") {
		return true
	}
	return false
}

func autofillReality(cfg map[string]interface{}) error {
	raw, _ := cfg["reality-config"].(map[string]interface{})
	if raw == nil {
		raw = map[string]interface{}{}
	}
	priv, _ := raw["private-key"].(string)
	// public-key may be present from older clients; only use it to skip re-derive, never persist.
	pubHint, _ := raw["public-key"].(string)
	priv, _, err := resolveRealityKeys(priv, pubHint)
	if err != nil {
		return err
	}
	raw["private-key"] = priv
	// Server listener schema (MetaCubeX): private-key, short-id, server-names, dest, limit-fallback-*.
	// public-key is client-only and rejected by ValidateListenerConfig.
	delete(raw, "public-key")
	delete(raw, "public_key")
	if dest, _ := raw["dest"].(string); strings.TrimSpace(dest) == "" {
		if sni, _ := cfg["sni"].(string); strings.TrimSpace(sni) != "" {
			raw["dest"] = strings.TrimSpace(sni) + ":443"
		} else {
			raw["dest"] = "www.microsoft.com:443"
		}
	}
	if names := asStringSlice(raw["server-names"]); len(names) == 0 {
		dest, _ := raw["dest"].(string)
		host := strings.Split(dest, ":")[0]
		if host == "" {
			host = "www.microsoft.com"
		}
		raw["server-names"] = []string{host}
	}
	if ids := asStringSlice(raw["short-id"]); len(ids) == 0 {
		raw["short-id"] = []string{randomHex(8)}
	}
	cfg["reality-config"] = raw
	return nil
}

func autofillUUIDUsers(cfg map[string]interface{}) error {
	users := normalizeUsersSlice(cfg["users"])
	if len(users) == 0 {
		users = []map[string]interface{}{{"uuid": uuid.NewString()}}
	} else {
		for _, u := range users {
			if uid, _ := u["uuid"].(string); strings.TrimSpace(uid) == "" {
				u["uuid"] = uuid.NewString()
			}
		}
	}
	cfg["users"] = users
	return nil
}

func autofillPasswordUsers(cfg map[string]interface{}) {
	// map form users (username -> password)
	if um, ok := cfg["users"].(map[string]interface{}); ok {
		for k, v := range um {
			if s, ok := v.(string); !ok || strings.TrimSpace(s) == "" {
				um[k] = randomPassword(16)
			}
		}
		if len(um) == 0 {
			cfg["users"] = map[string]interface{}{"default": randomPassword(16)}
		} else {
			cfg["users"] = um
		}
		return
	}
	users := normalizeUsersSlice(cfg["users"])
	if len(users) == 0 {
		// single password field
		if pass, _ := cfg["password"].(string); strings.TrimSpace(pass) == "" {
			cfg["password"] = randomPassword(16)
		}
		return
	}
	for _, u := range users {
		if pass, _ := u["password"].(string); strings.TrimSpace(pass) == "" {
			u["password"] = randomPassword(16)
		}
	}
	cfg["users"] = users
}

func normalizeUsersSlice(v interface{}) []map[string]interface{} {
	switch users := v.(type) {
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
	default:
		return nil
	}
}

func asStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{t}
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// GenerateMaterial returns auto-generated secrets for the create-node form.
func GenerateMaterial(kind, cipher string) (map[string]string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "uuid":
		return map[string]string{"uuid": uuid.NewString()}, nil
	case "password":
		return map[string]string{"password": randomPassword(16)}, nil
	case "ss-password":
		return map[string]string{"password": ssPasswordForCipher(cipher)}, nil
	case "short-id":
		return map[string]string{"short_id": randomHex(8)}, nil
	case "reality":
		priv, pub, err := resolveRealityKeys("", "")
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"private_key": priv,
			"public_key":  pub,
			"short_id":    randomHex(8),
		}, nil
	default:
		return nil, fmt.Errorf("unknown generate kind %q", kind)
	}
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
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return "", "", err
	}
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

func generateSelfSignedTLS(host string) (certPEM, keyPEM string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host, Organization: []string{"3m-ui"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM, nil
}
