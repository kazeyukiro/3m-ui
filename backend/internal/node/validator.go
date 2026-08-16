package node

import (
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
