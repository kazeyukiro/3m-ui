package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Credential struct{ Username, Password, UUID string }

var CredentialProvider func() (map[uint][]Credential, error)

type ConfigEngine struct{ db *gorm.DB }

func NewConfigEngine(db *gorm.DB) *ConfigEngine { return &ConfigEngine{db: db} }

func (ce *ConfigEngine) GenerateFinalConfig() (string, error) {
	if ce == nil || ce.db == nil {
		return "", fmt.Errorf("config engine database is not initialized")
	}
	baseBytes, err := yaml.Marshal(GetDefaultTemplate())
	if err != nil {
		return "", err
	}
	var merged map[string]interface{}
	if err := yaml.Unmarshal(baseBytes, &merged); err != nil {
		return "", err
	}
	var customFragments []models.Config
	if err := ce.db.Where("enabled = ?", true).Find(&customFragments).Error; err != nil {
		return "", fmt.Errorf("load custom config fragments: %w", err)
	}
	for _, fragment := range customFragments {
		var fragMap map[string]interface{}
		if err := yaml.Unmarshal([]byte(fragment.Content), &fragMap); err != nil {
			return "", fmt.Errorf("invalid custom config %q: %w", fragment.Name, err)
		}
		// Listeners are owned by the panel DB; fragments must not inject them
		// or Mihomo will report duplicate listener names after merge.
		delete(fragMap, "listeners")
		for k, v := range fragMap {
			merged[k] = v
		}
	}
	var listeners []models.Listener
	if err := ce.db.Where("enabled = ?", true).Find(&listeners).Error; err != nil {
		return "", fmt.Errorf("load enabled listeners: %w", err)
	}
	if err := validateListenerEndpoints(listeners); err != nil {
		return "", err
	}
	credentials := make(map[uint][]Credential)
	if CredentialProvider != nil {
		credentials, err = CredentialProvider()
		if err != nil {
			return "", fmt.Errorf("load listener credentials: %w", err)
		}
	}
	generated, err := generateListeners(ce.db, listeners, credentials)
	if err != nil {
		return "", err
	}
	merged["listeners"] = generated
	finalBytes, err := yaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("serialize final configuration: %w", err)
	}
	return string(finalBytes), nil
}

func validateListenerEndpoints(listeners []models.Listener) error {
	seenNames := make(map[string]struct{}, len(listeners))
	for i := range listeners {
		if err := validateListenerEndpoint(&listeners[i]); err != nil {
			return err
		}
		name := strings.TrimSpace(listeners[i].Name)
		if name == "" {
			return fmt.Errorf("listener id=%d has empty name", listeners[i].ID)
		}
		if _, ok := seenNames[name]; ok {
			return fmt.Errorf("duplicate listener name %q — rename or delete the extra entry in 节点管理", name)
		}
		seenNames[name] = struct{}{}
		for j := i + 1; j < len(listeners); j++ {
			if !portsOverlap(listeners[i].Port, listeners[j].Port) {
				continue
			}
			a := firstListenerAddress(listeners[i])
			b := firstListenerAddress(listeners[j])
			if listenerAddressesConflict(a, b) {
				return fmt.Errorf("listeners %q and %q have conflicting bind address/port ranges (%s:%s and %s:%s)", listeners[i].Name, listeners[j].Name, a, listeners[i].Port, b, listeners[j].Port)
			}
		}
	}
	return nil
}

func validateListenerEndpoint(l *models.Listener) error {
	address := firstListenerAddress(*l)
	if net.ParseIP(address) == nil {
		return fmt.Errorf("listener %q has invalid bind address %q", l.Name, address)
	}
	if !isValidPortString(l.Port) {
		return fmt.Errorf("listener %q has invalid port %q", l.Name, l.Port)
	}
	return nil
}

func firstListenerAddress(l models.Listener) string {
	if v := strings.Trim(strings.TrimSpace(l.BindAddress), "[]"); v != "" {
		return v
	}
	if v := strings.Trim(strings.TrimSpace(l.Listen), "[]"); v != "" {
		return v
	}
	return "0.0.0.0"
}

func portsOverlap(a, b string) bool {
	for _, ar := range parsePortRanges(a) {
		for _, br := range parsePortRanges(b) {
			if ar[0] <= br[1] && br[0] <= ar[1] {
				return true
			}
		}
	}
	return false
}

func parsePortRanges(raw string) [][2]int {
	var out [][2]int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			p := strings.SplitN(part, "-", 2)
			a, ea := strconv.Atoi(strings.TrimSpace(p[0]))
			b, eb := strconv.Atoi(strings.TrimSpace(p[1]))
			if ea == nil && eb == nil {
				out = append(out, [2]int{a, b})
			}
			continue
		}
		if p, err := strconv.Atoi(part); err == nil {
			out = append(out, [2]int{p, p})
		}
	}
	return out
}

func listenerAddressesConflict(a, b string) bool {
	ia, ib := net.ParseIP(a), net.ParseIP(b)
	if ia == nil || ib == nil {
		return false
	}
	if ia.Equal(ib) {
		return true
	}
	if ia.To4() != nil && ib.To4() != nil {
		return ia.IsUnspecified() || ib.IsUnspecified()
	}
	if ia.To4() == nil && ib.To4() == nil {
		return ia.IsUnspecified() || ib.IsUnspecified()
	}
	return false
}

func generateListeners(db *gorm.DB, listeners []models.Listener, creds map[uint][]Credential) ([]map[string]interface{}, error) {
	reg := protocol.DefaultCompileRegistry()
	result := make([]map[string]interface{}, 0, len(listeners))
	var skipped []string
	for _, l := range listeners {
		if !l.Enabled {
			continue
		}
		protocolName := strings.ToLower(strings.TrimSpace(l.Protocol))
		if protocolName == "" {
			protocolName = strings.ToLower(strings.TrimSpace(l.Type))
		}
		if !IsMihomoListenerProtocol(protocolName) {
			skipped = append(skipped, fmt.Sprintf("%s: unsupported protocol %q", l.Name, protocolName))
			continue
		}
		if !isValidPortString(l.Port) {
			skipped = append(skipped, fmt.Sprintf("%s: invalid port %q", l.Name, l.Port))
			continue
		}
		listen := firstListenerAddress(l)
		var portVal interface{} = strings.TrimSpace(l.Port)
		if p, err := strconv.Atoi(strings.TrimSpace(l.Port)); err == nil {
			portVal = p
		}
		configMap, err := decodeListenerConfig(l.Config)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", l.Name, err))
			continue
		}
		// Drop half-filled wrappers first so TLS cert ensure is not skipped.
		sanitizeIncompleteTLSWrappers(configMap)
		if err := ensureListenerTLSMaterial(protocolName, configMap); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", l.Name, err))
			continue
		}
		if patched, mErr := json.Marshal(configMap); mErr == nil {
			l.Config = string(patched)
			// Persist sanitized/autofilled config so incomplete wrappers and new
			// certs stick across reloads (does not rotate existing complete certs).
			if db != nil && l.ID != 0 {
				_ = db.Model(&models.Listener{}).Where("id = ?", l.ID).Update("config", string(patched)).Error
			}
		}
		if err := ValidateListenerConfig(protocolName, l.Config); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", l.Name, err))
			continue
		}

		listenerCreds, hasCredState := creds[l.ID]
		users := make([]protocol.UserCred, 0, len(listenerCreds))
		for _, c := range listenerCreds {
			users = append(users, protocol.UserCred{
				Username: c.Username,
				Password: c.Password,
				UUID:     c.UUID,
			})
		}

		in := protocol.CompileInput{
			Name:               l.Name,
			Protocol:           protocolName,
			Listen:             listen,
			Port:               portVal,
			UDP:                l.UDP,
			TLS:                l.TLS,
			Proxy:              l.Proxy,
			Rule:               l.Rule,
			RoutingMark:        l.RoutingMark,
			Config:             configMap,
			Users:              users,
			HasCredentialState: hasCredState,
		}
		compiled, err := reg.Compile(in)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", l.Name, err))
			continue
		}
		sanitizeIncompleteTLSWrappers(compiled)
		result = append(result, compiled)
	}
	if len(skipped) > 0 {
		log.Printf("3m-ui: skipped %d listener(s) during config generation: %s", len(skipped), strings.Join(skipped, "; "))
	}
	if len(result) == 0 && len(skipped) > 0 {
		return nil, fmt.Errorf("no valid listeners to generate; skipped: %s", strings.Join(skipped, "; "))
	}
	return result, nil
}

func isValidPortString(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, ",") {
		for _, p := range strings.Split(s, ",") {
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
		return err1 == nil && err2 == nil && start >= 1 && end <= 65535 && start <= end
	}
	port, err := strconv.Atoi(s)
	return err == nil && port >= 1 && port <= 65535
}

func normalizeListenerUserList(value interface{}) ([]map[string]interface{}, error) {
	switch users := value.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(users))
		for i, item := range users {
			user, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("users[%d] must be an object", i)
			}
			result = append(result, user)
		}
		return result, nil
	case map[string]interface{}:
		result := make([]map[string]interface{}, 0, len(users))
		for username, password := range users {
			result = append(result, map[string]interface{}{"username": username, "password": fmt.Sprint(password)})
		}
		return result, nil
	case map[interface{}]interface{}:
		result := make([]map[string]interface{}, 0, len(users))
		for rawUsername, rawPassword := range users {
			result = append(result, map[string]interface{}{"username": fmt.Sprint(rawUsername), "password": fmt.Sprint(rawPassword)})
		}
		return result, nil
	default:
		return nil, fmt.Errorf("users must be a list of username/password objects")
	}
}

func decodeListenerConfig(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &configMap); err != nil {
		return nil, fmt.Errorf("invalid listener configuration (must be valid JSON): %w", err)
	}
	if configMap == nil {
		configMap = map[string]interface{}{}
	}
	return configMap, nil
}

func listenerFieldIsManaged(key string) bool {
	switch key {
	case "users", "username", "password", "uuid", "flow", "alterId", "tls", "servername", "sni", "skip-cert-verify", "name-cert-verify", "fingerprint", "client-fingerprint", "reality-opts", "shadow-tls-opts", "restls-opts", "jls-opts", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "mkcp-opts", "certificate", "private-key", "private_key":
		return true
	default:
		return false
	}
}

func copyServerTLSFields(dst, src map[string]interface{}) {
	if value, ok := src["certificate"]; ok {
		dst["certificate"] = value
	}
	if value, ok := src["private-key"]; ok {
		dst["private-key"] = value
	} else if value, ok := src["private_key"]; ok {
		dst["private-key"] = value
	}
}
func copyOption(dst, src map[string]interface{}, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
}
func listenerHasUDPOption(protocol string) bool {
	switch protocol {
	case "shadowsocks", "snell", "vmess", "vless", "trojan", "anytls", "trusttunnel":
		return true
	default:
		return false
	}
}
func listenerSupportsTLS(protocol string) bool {
	switch protocol {
	case "vmess", "vless", "trojan", "anytls", "mieru", "trusttunnel":
		return true
	default:
		return false
	}
}
func ListenerSupportsTLS(protocol string) bool {
	return listenerSupportsTLS(strings.ToLower(strings.TrimSpace(protocol)))
}
