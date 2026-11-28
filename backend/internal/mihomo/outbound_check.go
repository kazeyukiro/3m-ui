package mihomo

import (
	"context"
	"fmt"
	"net"
	"time"

	mihomoconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

// CheckOutboundConnection tests saved settings in an isolated client without
// applying or modifying the serving core's configuration.
func (s *Service) CheckOutboundConnection(ctx context.Context, db *gorm.DB, name string) (ConnectionCheck, error) {
	profile, err := mihomoconfig.ServerOutboundCheckProfile(db, name)
	if err != nil {
		return ConnectionCheck{}, err
	}
	check := ConnectionCheck{State: "unknown", Source: "local", Steps: []ConnectionStep{}}
	finish := func(reason string) (ConnectionCheck, error) {
		check.Reason = reason
		check.CheckedAt = time.Now().UTC()
		check.ExpiresAt = check.CheckedAt.Add(connectionResultTTL)
		return check, nil
	}
	if !profile.Enabled {
		check.State = "disabled"
		return finish("disabled")
	}
	if !s.connectionCheckMu.TryLock() {
		return finish("check_busy")
	}
	defer s.connectionCheckMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var binary string
	if s.pm == nil {
		return finish("client_start_failed")
	}
	s.pm.mu.Lock()
	binary = s.pm.binaryPath
	s.pm.mu.Unlock()
	proxy := profile.Proxies[0]
	if server, _ := proxy["server"].(string); server != "" {
		check.Address = net.JoinHostPort(server, fmt.Sprint(proxy["port"]))
	}
	check.Steps = append(check.Steps, ConnectionStep{"client_config", "passed", "client_config_ready"})
	runClientCheck(ctx, binary, profile.Proxies, profile.Groups, name, &check, connectionTestTargets, s.pm.allowedBinary)
	return finish(check.Reason)
}
