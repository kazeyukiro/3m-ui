package listener

import (
	"fmt"
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
	if l.Port == "" {
		return fmt.Errorf("listener port is required")
	}
	if !validPort(l.Port) {
		return fmt.Errorf("listener %q has invalid port %q", l.Name, l.Port)
	}
	if err := dbconfig.ValidateListenerConfig(protocol, l.Config); err != nil {
		return fmt.Errorf("listener %q: %w", l.Name, err)
	}
	return nil
}

func validPort(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" { return false }
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			r := strings.SplitN(part, "-", 2)
			if len(r) != 2 { return false }
			a, errA := strconv.Atoi(strings.TrimSpace(r[0]))
			b, errB := strconv.Atoi(strings.TrimSpace(r[1]))
			if errA != nil || errB != nil || a < 1 || b > 65535 || a > b { return false }
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil || p < 1 || p > 65535 { return false }
	}
	return true
}
