package node

import (
	"net"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/credentials"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

func ValidateNode(l *models.Listener) error {
	l.Name = strings.TrimSpace(l.Name)
	if l.Name == "" {
		return fmt.Errorf("listener name cannot be empty")
	}
	proto := strings.ToLower(strings.TrimSpace(l.Protocol))
	if proto == "" {
		proto = strings.ToLower(strings.TrimSpace(l.Type))
	}
	if !mihomoConfig.IsMihomoListenerProtocol(proto) {
		return fmt.Errorf("unsupported Mihomo listener protocol: %s", proto)
	}
	l.Protocol, l.Type = proto, proto

	if !isValidPortString(l.Port) {
		return fmt.Errorf("port must be a valid port number, range (e.g. 8080-8090), or comma-separated list")
	}

	l.BindAddress = strings.TrimSpace(l.BindAddress)
	if l.BindAddress == "" {
		l.BindAddress = strings.TrimSpace(l.Listen)
	}
	if l.BindAddress == "" {
		l.BindAddress = "0.0.0.0"
	}
	// Strip brackets; accept IPv4 / IPv6 (including :: and 0.0.0.0).
	l.BindAddress = strings.Trim(l.BindAddress, "[]")
	if net.ParseIP(l.BindAddress) == nil {
		return fmt.Errorf("bind address must be a valid IPv4 or IPv6 address (got %q)", l.BindAddress)
	}
	if h := strings.TrimSpace(l.PublicHost); h != "" {
		// Public host may be domain or IP — normalize bracketed IPv6.
		l.PublicHost = strings.Trim(h, "[]")
	}

	if l.RoutingMark < 0 {
		return fmt.Errorf("routing-mark must be non-negative")
	}

	if err := credentials.EnsureListenerCredentials(l); err != nil {
		return err
	}
	cfg, err := decodeConfig(l.Config)
	if err != nil {
		return err
	}
	if err := validateSchema(proto, cfg); err != nil {
		return err
	}
	return validateProtocolSpecific(proto, cfg)
}

func isValidPortString(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		for _, p := range parts {
			if !isValidPortString(strings.TrimSpace(p)) {
				return false
			}
		}
		return true
	}
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		if len(parts) != 2 {
			return false
		}
		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil || start < 1 || end > 65535 || start > end {
			return false
		}
		return true
	}
	port, err := strconv.Atoi(s)
	if err != nil || port < 1 || port > 65535 {
		return false
	}
	return true
}

func decodeConfig(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid listener configuration: %w", err)
	}
	if cfg == nil {
		return map[string]interface{}{}, nil
	}
	return cfg, nil
}

func validateSchema(proto string, cfg map[string]interface{}) error {
	schema, ok := mihomoConfig.GetMihomoListenerSchema(proto)
	if !ok {
		return fmt.Errorf("no listener schema registered for protocol %s", proto)
	}
	return validateObject(proto, "", cfg, schema.Fields, schema.NestedFields)
}

func validateObject(proto, prefix string, value map[string]interface{}, allowed map[string]struct{}, nested map[string]map[string]struct{}) error {
	for key, child := range value {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%s listener: unsupported field %q", proto, joinField(prefix, key))
		}
		if childMap, ok := child.(map[string]interface{}); ok {
			if key == "users" && mapUserProtocols[proto] {
				continue
			}
			childAllowed, hasNestedSchema := nested[key]
			if !hasNestedSchema {
				return fmt.Errorf("%s listener: field %q does not accept an object", proto, joinField(prefix, key))
			}
			childNested := make(map[string]map[string]struct{})
			for nestedKey, nestedAllowed := range nested {
				if strings.HasPrefix(nestedKey, key+".") {
					childNested[strings.TrimPrefix(nestedKey, key+".")] = nestedAllowed
				}
			}
			if err := validateObject(proto, joinField(prefix, key), childMap, childAllowed, childNested); err != nil {
				return err
			}
		}
	}
	return nil
}

var mapUserProtocols = map[string]bool{"anytls": true, "hysteria2": true, "mieru": true, "tuic": true}

func joinField(prefix, field string) string {
	if prefix == "" {
		return field
	}
	return prefix + "." + field
}

func validateProtocolSpecific(proto string, cfg map[string]interface{}) error {
	switch proto {
	case "anytls":
		return validateAnyTLS(cfg)
	case "snell":
		if value, ok := numeric(cfg["version"]); ok && (value < 1 || value > 5) {
			return fmt.Errorf("snell version must be between 1 and 5")
		}
	case "hysteria2":
		if obfs, ok := cfg["obfs"].(string); ok && obfs != "" && obfs != "salamander" {
			return fmt.Errorf("hysteria2 obfs must be salamander")
		}
		// Official mihomo Hysteria2 inbound requires certificate/private-key; there is no allow-insecure field.
		if !hasCertificatePair(cfg) {
			return fmt.Errorf("hysteria2 listener requires certificate and private-key")
		}
		if users, ok := cfg["users"].(map[string]interface{}); !ok || len(users) == 0 {
			return fmt.Errorf("hysteria2 listener requires at least one user")
		}
	case "trusttunnel":
		if !hasCertificatePair(cfg) {
			return fmt.Errorf("trusttunnel listener requires certificate and private-key")
		}
	case "tuic":
		users, token := hasNonEmpty(cfg["users"]), hasNonEmpty(cfg["token"])
		if users == token {
			return fmt.Errorf("tuic listener must configure exactly one of users (TUIC V5) or token (TUIC V4)")
		}
		if users {
			if values, ok := cfg["users"].(map[string]interface{}); !ok || len(values) == 0 {
				return fmt.Errorf("tuic V5 listener users must be a username-to-password map")
			}
		}
	case "vless", "vmess":
		if err := validateServerTLSModes(proto, cfg); err != nil {
			return err
		}
		users, ok := cfg["users"].([]interface{})
		if !ok || len(users) == 0 {
			return fmt.Errorf("%s listener requires at least one user", proto)
		}
		for i, user := range users {
			if err := validateUserRow(proto, i, user, true); err != nil {
				return err
			}
		}
	case "trojan":
		if err := validateServerTLSModes(proto, cfg); err != nil {
			return err
		}
		users, ok := cfg["users"].([]interface{})
		if !ok || len(users) == 0 {
			return fmt.Errorf("%s listener requires at least one user", proto)
		}
		for i, user := range users {
			if err := validateUserRow(proto, i, user, false); err != nil {
				return err
			}
		}
	case "shadowquic":
		users, ok := cfg["users"].([]interface{})
		if !ok || len(users) == 0 {
			return fmt.Errorf("%s listener requires at least one user", proto)
		}
		for i, user := range users {
			if err := validateUserRow(proto, i, user, false); err != nil {
				return err
			}
		}
	case "mieru":
		if users, ok := cfg["users"].(map[string]interface{}); !ok || len(users) == 0 {
			return fmt.Errorf("mieru listener requires at least one user")
		}
	case "sudoku":
		min, minOK := numeric(cfg["padding-min"])
		max, maxOK := numeric(cfg["padding-max"])
		if minOK && maxOK && max < min {
			return fmt.Errorf("sudoku padding-max must be greater than or equal to padding-min")
		}
	}
	return nil
}

func validateAnyTLS(cfg map[string]interface{}) error {
	cert, key := hasString(cfg["certificate"]), hasString(cfg["private-key"])
	if cert != key {
		return fmt.Errorf("anytls listener requires certificate and private-key together")
	}
	shadow, res, jls := enabledObject(cfg["shadow-tls"]), enabledObject(cfg["res-tls"]), enabledObject(cfg["jls-config"])
	secureModes := 0
	if shadow {
		secureModes++
	}
	if res {
		secureModes++
	}
	if jls {
		secureModes++
	}
	if cert {
		secureModes++
	}
	if secureModes > 1 {
		return fmt.Errorf("anytls listener TLS modes are mutually exclusive: use certificate/private-key, shadow-tls, res-tls, or jls-config")
	}
	if !boolValue(cfg["allow-insecure"]) && secureModes == 0 {
		return fmt.Errorf("anytls listener requires certificate/private-key, shadow-tls, res-tls, jls-config, or allow-insecure=true")
	}
	if auth, ok := cfg["client-auth-type"].(string); ok && (auth == "verify-if-given" || auth == "require-and-verify") && !hasString(cfg["client-auth-cert"]) {
		return fmt.Errorf("anytls listener client-auth-cert is required for %s", auth)
	}
	users, ok := cfg["users"].(map[string]interface{})
	if !ok || len(users) == 0 {
		return fmt.Errorf("anytls listener requires at least one user")
	}
	for username, password := range users {
		if strings.TrimSpace(username) == "" || !hasString(password) {
			return fmt.Errorf("anytls listener users must contain non-empty username/password pairs")
		}
	}
	return nil
}

func enabledObject(v interface{}) bool {
	m, ok := v.(map[string]interface{})
	return ok && boolValue(m["enable"])
}

func validateUserRow(proto string, index int, raw interface{}, uuidMode bool) error {
	row, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%s listener users[%d] must be an object", proto, index)
	}
	// Protocol-specific field bans with explicit messages expected by tests
	// and matching official Mihomo listener schemas.
	if proto == "vmess" {
		if _, ok := row["flow"]; ok {
			return fmt.Errorf("vmess listener users[%d] does not support flow", index)
		}
	}
	if proto == "vless" {
		if _, ok := row["alterId"]; ok {
			return fmt.Errorf("vless listener users[%d] does not support alterId", index)
		}
	}
	allowed := map[string]struct{}{"username": {}}
	if uuidMode {
		allowed["uuid"] = struct{}{}
		if proto == "vmess" {
			allowed["alterId"] = struct{}{}
		} else {
			allowed["flow"] = struct{}{}
		}
	} else {
		allowed["password"] = struct{}{}
	}
	for key := range row {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%s listener users[%d]: unsupported field %q", proto, index, key)
		}
	}
	if !hasString(row["username"]) {
		return fmt.Errorf("%s listener users[%d] requires username", proto, index)
	}
	if uuidMode && !hasString(row["uuid"]) {
		return fmt.Errorf("%s listener users[%d] requires uuid", proto, index)
	}
	if !uuidMode && !hasString(row["password"]) {
		return fmt.Errorf("%s listener users[%d] requires password", proto, index)
	}
	return nil
}

// validateServerTLSModes enforces official Mihomo listener TLS exclusivity:
// certificate/private-key must appear together, and must not be combined with
// reality-config (or other exclusive TLS modes when present).
func validateServerTLSModes(proto string, cfg map[string]interface{}) error {
	cert, key := hasString(cfg["certificate"]), hasString(cfg["private-key"])
	if cert != key {
		return fmt.Errorf("%s listener requires certificate and private-key together", proto)
	}
	if cert {
		if _, hasReality := cfg["reality-config"]; hasReality {
			return fmt.Errorf("%s listener mutually exclusive TLS modes: certificate/private-key cannot be used with reality-config", proto)
		}
		if _, hasMirror := cfg["tlsmirror-config"]; hasMirror {
			return fmt.Errorf("%s listener mutually exclusive TLS modes: certificate/private-key cannot be used with tlsmirror-config", proto)
		}
	}
	return nil
}

func hasCertificatePair(cfg map[string]interface{}) bool {
	return hasString(cfg["certificate"]) && hasString(cfg["private-key"])
}

func hasString(value interface{}) bool {
	s, ok := value.(string)
	return ok && strings.TrimSpace(s) != ""
}

func hasNonEmpty(value interface{}) bool {
	if value == nil {
		return false
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() > 0
	default:
		return true
	}
}

func boolValue(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func numeric(v interface{}) (float64, bool) {
	n, ok := v.(float64)
	return n, ok
}
