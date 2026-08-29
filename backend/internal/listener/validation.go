package listener

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

func ValidateModel(l *models.Listener) error {
	if l == nil {
		return fmt.Errorf("listener is required")
	}
	l.Name = strings.TrimSpace(l.Name)
	if l.Name == "" {
		return fmt.Errorf("listener name is required")
	}
	protocol := strings.ToLower(strings.TrimSpace(l.Protocol))
	legacyType := strings.ToLower(strings.TrimSpace(l.Type))
	if protocol == "" {
		protocol = legacyType
	}
	if protocol == "" || !dbconfig.IsMihomoListenerProtocol(protocol) {
		return fmt.Errorf("unsupported Mihomo listener protocol %q", protocol)
	}
	if legacyType != "" && protocol != legacyType {
		return fmt.Errorf("listener protocol %q does not match type %q", protocol, legacyType)
	}
	l.Protocol = protocol
	if strings.TrimSpace(l.Port) == "" {
		return fmt.Errorf("listener port is required")
	}
	if !validPort(l.Port) {
		return fmt.Errorf("listener %q has invalid port %q", l.Name, l.Port)
	}
	if address := strings.TrimSpace(firstNonEmpty(l.BindAddress, l.Listen)); address != "" {
		if net.ParseIP(address) == nil {
			return fmt.Errorf("listener %q has invalid bind address %q; use an IPv4 or IPv6 address", l.Name, address)
		}
		l.BindAddress = address
	}
	if err := sanitizeListenerConfigJSON(l); err != nil {
		return err
	}
	if err := dbconfig.ValidateListenerConfig(protocol, l.Config); err != nil {
		return fmt.Errorf("listener %q: %w", l.Name, err)
	}
	if l.TLS {
		if !dbconfig.ListenerSupportsTLS(protocol) {
			return fmt.Errorf("listener %q: TLS is not supported for protocol %q", l.Name, protocol)
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func validPort(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return false
		}
		if strings.Contains(part, "-") {
			r := strings.SplitN(part, "-", 2)
			if len(r) != 2 {
				return false
			}
			a, errA := strconv.Atoi(strings.TrimSpace(r[0]))
			b, errB := strconv.Atoi(strings.TrimSpace(r[1]))
			if errA != nil || errB != nil || a < 1 || b > 65535 || a > b {
				return false
			}
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil || p < 1 || p > 65535 {
			return false
		}
	}
	return true
}

// PortRangesOverlap reports whether two comma-separated port specifications
// share at least one port. It is used by the service before a listener is
// persisted so conflicts are rejected before Mihomo sees the configuration.
func PortRangesOverlap(a, b string) bool {
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
	var result [][2]int
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			p := strings.SplitN(part, "-", 2)
			a, errA := strconv.Atoi(strings.TrimSpace(p[0]))
			b, errB := strconv.Atoi(strings.TrimSpace(p[1]))
			if errA == nil && errB == nil {
				result = append(result, [2]int{a, b})
			}
			continue
		}
		if p, err := strconv.Atoi(part); err == nil {
			result = append(result, [2]int{p, p})
		}
	}
	return result
}

// AddressesConflict uses conservative socket-binding semantics. Wildcards
// conflict with any address of the same family; an IPv4 and IPv6 address are
// considered independent sockets. Since the Listener model only accepts IP
// addresses, hostname ambiguity is avoided at the trust boundary.
func AddressesConflict(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		a = "0.0.0.0"
	}
	if b == "" {
		b = "0.0.0.0"
	}
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

func sanitizeListenerConfigJSON(l *models.Listener) error {
	if l == nil || strings.TrimSpace(l.Config) == "" {
		return nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
		return fmt.Errorf("invalid config json: %w", err)
	}
	delete(cfg, "security_layer")
	delete(cfg, "transport_layer")
	delete(cfg, "access_profile")
	if raw, ok := cfg["reality-config"].(map[string]interface{}); ok && raw != nil {
		delete(raw, "public-key")
		delete(raw, "public_key")
		cfg["reality-config"] = raw
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	l.Config = string(b)
	return nil
}
