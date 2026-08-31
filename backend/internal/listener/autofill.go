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
	case "hysteria2", "anytls", "mieru":
		// map form users (username -> password)
		autofillUsersMap(cfg)
	case "trojan", "shadowquic", "trusttunnel":
		// array form users [{username, password}]
		autofillUsersArray(cfg)
	case "tuic":
		// TUIC v4 uses token (array of strings); v5 uses users [{username, password}].
		autofillTUICUsers(cfg)
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
		// Mihomo requires a valid snell version (1-4). Default to 4 when unset
		// so listeners created from the panel are not rejected by mihomo.
		// Stored as a JSON number (int) so it round-trips to YAML `version: 4`
		// and passes the numeric() validator in node.ValidateListenerConfig.
		if !versionIsSet(cfg["version"]) {
			cfg["version"] = 4
		}
	case "sudoku":
		// Sudoku uses top-level `key` for authentication (no `users`).
		if k, _ := cfg["key"].(string); strings.TrimSpace(k) == "" {
			b := make([]byte, 32)
			_, _ = rand.Read(b)
			cfg["key"] = base64.StdEncoding.EncodeToString(b)
		}
		if m, _ := cfg["aead-method"].(string); strings.TrimSpace(m) == "" {
			cfg["aead-method"] = "chacha20-poly1305"
		}
	}

	// REALITY material when security uses reality-config or panel layer is reality.
	if needsReality(cfg) {
		if err := autofillReality(cfg); err != nil {
			return err
		}
	}

	// Protocols that speak TLS at the listener layer need certificate + private-key
	// unless an alternate security wrapper (REALITY / shadow-tls / …) is configured.
	// Empty strings are treated as unset (Mihomo rejects present-but-empty fields).
	if err := ensureServerTLSCertificate(cfg, proto); err != nil {
		return err
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
	// Incomplete JLS blocks make Mihomo reject the whole config:
	// "jls-config has unset fields: dest, users".
	if jls, ok := cfg["jls-config"].(map[string]interface{}); ok {
		dest, _ := jls["dest"].(string)
		if strings.TrimSpace(dest) == "" || !jlsHasUsers(jls) {
			delete(cfg, "jls-config")
		}
	}
	if ju, ok := cfg["jls-upstream"].(map[string]interface{}); ok {
		addr, _ := ju["addr"].(string)
		if strings.TrimSpace(addr) == "" {
			delete(cfg, "jls-upstream")
		}
	}
	if st, ok := cfg["shadow-tls"].(map[string]interface{}); ok {
		// Drop enable-only / missing handshake.dest
		dest := ""
		if hs, ok := st["handshake"].(map[string]interface{}); ok {
			dest, _ = hs["dest"].(string)
		}
		pass, _ := st["password"].(string)
		if strings.TrimSpace(dest) == "" || (strings.TrimSpace(pass) == "" && st["users"] == nil) {
			delete(cfg, "shadow-tls")
		}
	}
	if rt, ok := cfg["res-tls"].(map[string]interface{}); ok {
		if dest, _ := rt["dest"].(string); strings.TrimSpace(dest) == "" {
			delete(cfg, "res-tls")
		}
	}
	if tm, ok := cfg["tlsmirror-config"].(map[string]interface{}); ok {
		dest, _ := tm["dest"].(string)
		pk, _ := tm["primary-key"].(string)
		if strings.TrimSpace(dest) == "" || strings.TrimSpace(pk) == "" {
			delete(cfg, "tlsmirror-config")
		}
	}
	// If a cert pair is present, clear allow-insecure (mutually exclusive modes).
	cert, _ := cfg["certificate"].(string)
	key, _ := cfg["private-key"].(string)
	if key == "" {
		key, _ = cfg["private_key"].(string)
	}
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		delete(cfg, "allow-insecure")
	}
}

func jlsHasUsers(m map[string]interface{}) bool {
	raw, ok := m["users"]
	if !ok || raw == nil {
		return false
	}
	switch users := raw.(type) {
	case []interface{}:
		for _, item := range users {
			u, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			user, _ := u["username"].(string)
			pass, _ := u["password"].(string)
			if strings.TrimSpace(user) != "" && strings.TrimSpace(pass) != "" {
				return true
			}
		}
	case []map[string]interface{}:
		for _, u := range users {
			user, _ := u["username"].(string)
			pass, _ := u["password"].(string)
			if strings.TrimSpace(user) != "" && strings.TrimSpace(pass) != "" {
				return true
			}
		}
	}
	return false
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

// autofillUsersMap fills credentials for protocols whose users field is a
// map of username -> password (hysteria2, anytls, mieru).
func autofillUsersMap(cfg map[string]interface{}) {
	users, ok := cfg["users"].(map[string]interface{})
	if !ok || len(users) == 0 {
		cfg["users"] = map[string]interface{}{"default": randomPassword(16)}
		return
	}
	for k, v := range users {
		if s, ok := v.(string); !ok || strings.TrimSpace(s) == "" {
			users[k] = randomPassword(16)
		}
	}
	cfg["users"] = users
}

// autofillUsersArray fills credentials for protocols whose users field is an
// array of {username, password} objects (trojan, shadowquic, trusttunnel).
func autofillUsersArray(cfg map[string]interface{}) {
	users := normalizeUsersSlice(cfg["users"])
	if len(users) == 0 {
		cfg["users"] = []map[string]interface{}{{"username": "default", "password": randomPassword(16)}}
		return
	}
	for _, u := range users {
		if pass, _ := u["password"].(string); strings.TrimSpace(pass) == "" {
			u["password"] = randomPassword(16)
		}
	}
	cfg["users"] = users
}

// autofillTUICUsers fills credentials for TUIC listeners. TUIC v4 uses a token
// array (left untouched when set); TUIC v5 uses a `users` map keyed by UUID /
// username (matching the Mihomo TUIC compiler `asUsersMapUUID`, which reads
// `cfg["users"]` as a map - never as an array). When neither token nor a users
// map is present we default to the v5 map form with a single `default` user.
func autofillTUICUsers(cfg map[string]interface{}) {
	// TUIC v4 uses `token` (array of strings); v5 uses `users` as map{UUID: PASSWORD}.
	// If token is set, leave it (v4). Otherwise default to v5 map form.
	if _, hasToken := cfg["token"]; hasToken {
		return
	}
	// If users already exists as a map, fill empty passwords.
	if users, ok := cfg["users"].(map[string]interface{}); ok && len(users) > 0 {
		for k, v := range users {
			if s, ok := v.(string); !ok || strings.TrimSpace(s) == "" {
				users[k] = randomPassword(16)
			}
		}
		cfg["users"] = users
		return
	}
	// Also handle map[interface{}]interface{} (from YAML/JSON decode)
	if users, ok := cfg["users"].(map[interface{}]interface{}); ok && len(users) > 0 {
		out := make(map[string]interface{}, len(users))
		for k, v := range users {
			key := fmt.Sprint(k)
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				out[key] = s
			} else {
				out[key] = randomPassword(16)
			}
		}
		cfg["users"] = out
		return
	}
	// No users map exists - create default v5 map form with a real UUID key
	// (TUIC clients reject non-UUID uuid fields).
	cfg["users"] = map[string]interface{}{uuid.New().String(): randomPassword(16)}
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

// versionIsSet reports whether the snell `version` field is meaningfully set.
// It accepts both string ("4") and numeric (int/float64) representations
// produced by the panel or by json.Unmarshal. An empty string or absent key
// counts as unset so the autofill can apply a safe default.
func versionIsSet(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(t) != ""
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return true
	default:
		return false
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


// ensureServerTLSCertificate fills certificate/private-key with a self-signed
// pair when the protocol requires server TLS material and neither field is set.
// REALITY / shadow-tls / res-tls / jls / tlsmirror listeners are skipped.
func ensureServerTLSCertificate(cfg map[string]interface{}, proto string) error {
	if cfg == nil {
		return nil
	}
	if s, ok := cfg["certificate"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "certificate")
	}
	if s, ok := cfg["private-key"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "private-key")
	}
	if s, ok := cfg["private_key"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "private_key")
	}

	cert, _ := cfg["certificate"].(string)
	key, _ := cfg["private-key"].(string)
	if key == "" {
		key, _ = cfg["private_key"].(string)
	}
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		return nil
	}
	if strings.TrimSpace(cert) != "" || strings.TrimSpace(key) != "" {
		delete(cfg, "certificate")
		delete(cfg, "private-key")
		delete(cfg, "private_key")
	}
	if !protocolNeedsServerCertificate(proto, cfg) {
		return nil
	}

	host := "localhost"
	if sni, _ := cfg["sni"].(string); strings.TrimSpace(sni) != "" {
		host = strings.TrimSpace(sni)
	} else if names := asStringSlice(cfg["server-names"]); len(names) > 0 {
		host = names[0]
	}
	certPEM, keyPEM, err := generateSelfSignedTLS(host)
	if err != nil {
		return fmt.Errorf("generate self-signed certificate for %s: %w", proto, err)
	}
	cfg["certificate"] = certPEM
	cfg["private-key"] = keyPEM
	// Normal TLS with cert — not the nginx/caddy "allow-insecure" mode.
	delete(cfg, "allow-insecure")
	return nil
}

func protocolNeedsServerCertificate(proto string, cfg map[string]interface{}) bool {
	if _, ok := cfg["reality-config"]; ok {
		return false
	}
	// Incomplete wrapper toggles must not suppress cert generation.
	if jls, ok := cfg["jls-config"].(map[string]interface{}); ok {
		dest, _ := jls["dest"].(string)
		if strings.TrimSpace(dest) != "" && jlsHasUsers(jls) {
			return false
		}
	}
	if enabledMap(cfg, "shadow-tls") {
		return false
	}
	if rt, ok := cfg["res-tls"].(map[string]interface{}); ok {
		if dest, _ := rt["dest"].(string); strings.TrimSpace(dest) != "" {
			return false
		}
	}
	if _, ok := cfg["tlsmirror-config"]; ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "hysteria2", "anytls", "mieru", "tuic", "trusttunnel", "trojan":
		return true
	default:
		return false
	}
}

func enabledMap(cfg map[string]interface{}, key string) bool {
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return false
	}
	if m, ok := raw.(map[string]interface{}); ok {
		if en, ok := m["enable"].(bool); ok {
			return en
		}
		if en, ok := m["enabled"].(bool); ok {
			return en
		}
		return len(m) > 0
	}
	return true
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
