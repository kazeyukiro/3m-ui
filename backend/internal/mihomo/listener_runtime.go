package mihomo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	psnet "github.com/shirou/gopsutil/v4/net"
	"gopkg.in/yaml.v3"
)

// ListenerRuntime describes local socket ownership, not protocol health or
// reachability through a firewall, container port mapping or public network.
type ListenerRuntime struct {
	ID              uint              `json:"id"`
	State           string            `json:"state"`
	Reason          string            `json:"reason"`
	CheckedAt       time.Time         `json:"checked_at"`
	Endpoints       []RuntimeEndpoint `json:"endpoints"`
	ConnectionCheck *ConnectionCheck  `json:"connection_check,omitempty"`
}

type RuntimeEndpoint struct {
	Network string `json:"network"`
	Address string `json:"address"`
	Port    uint32 `json:"port"`
	Bound   bool   `json:"bound"`
}

type runtimeListener struct {
	Name      string         `yaml:"name"`
	Type      string         `yaml:"type"`
	Listen    string         `yaml:"listen"`
	Port      string         `yaml:"port"`
	UDP       bool           `yaml:"udp"`
	Transport string         `yaml:"transport"`
	KcpTun    map[string]any `yaml:"kcp-tun"`
}

type runtimeConnection struct {
	psnet.ConnectionStat
	inode       uint64
	familyKnown bool
	ipv6Only    bool
}

func runtimeListeners(content string) ([]runtimeListener, error) {
	var config struct {
		Listeners []runtimeListener `yaml:"listeners"`
	}
	err := yaml.Unmarshal([]byte(content), &config)
	return config.Listeners, err
}

func (l runtimeListener) endpoints() ([]RuntimeEndpoint, error) {
	networks := []string{"tcp"}
	switch l.Type {
	case "hysteria2", "tuic", "shadowquic":
		networks = []string{"udp"}
	case "mieru":
		if strings.EqualFold(l.Transport, "UDP") {
			networks = []string{"udp"}
		}
	case "shadowsocks":
		if enabled, _ := l.KcpTun["enable"].(bool); enabled {
			return nil, fmt.Errorf("unsupported transport")
		}
		if l.UDP {
			networks = append(networks, "udp")
		}
	case "snell", "vmess", "vless", "trojan", "anytls", "sudoku", "trusttunnel":
	default:
		return nil, fmt.Errorf("unsupported transport")
	}
	address := strings.TrimSpace(l.Listen)
	if address == "" {
		address = "0.0.0.0"
	}
	if net.ParseIP(address) == nil {
		return nil, fmt.Errorf("unsupported address")
	}
	var endpoints []RuntimeEndpoint
	seen := map[int]bool{}
	for _, part := range strings.Split(l.Port, ",") {
		bounds := strings.SplitN(strings.TrimSpace(part), "-", 2)
		start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
		if err != nil || start < 1 || start > 65535 {
			return nil, fmt.Errorf("invalid port")
		}
		end := start
		if len(bounds) == 2 {
			end, err = strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil || end < start || end > 65535 {
				return nil, fmt.Errorf("invalid port range")
			}
		}
		for port := start; port <= end; port++ {
			if seen[port] {
				continue
			}
			seen[port] = true
			for _, network := range networks {
				endpoints = append(endpoints, RuntimeEndpoint{Network: network, Address: address, Port: uint32(port)})
			}
		}
	}
	return endpoints, nil
}

// inspectEndpoints checks every prepared endpoint against the shared index.
// A hole in the middle of a range must still be reported as a binding failure.
func inspectEndpoints(endpoints []RuntimeEndpoint, endpointErr error, sockets socketIndex, inspectionErr error, details bool) ListenerRuntime {
	result := ListenerRuntime{State: "unknown", Reason: "inspection_unavailable", CheckedAt: time.Now().UTC(), Endpoints: []RuntimeEndpoint{}}
	if endpointErr != nil {
		result.Reason = "unsupported_transport"
		return result
	}
	if details {
		result.Endpoints = make([]RuntimeEndpoint, len(endpoints))
		copy(result.Endpoints, endpoints)
	}
	if inspectionErr != nil {
		return result
	}
	result.State, result.Reason = "listening", "sockets_bound"
	for i, e := range endpoints {
		state := sockets[endpointKey(e.Network, e.Address, e.Port)]
		if details {
			result.Endpoints[i].Bound = state == socketBound
		}
		switch state {
		case socketBound:
		case socketFamilyUnknown:
			if result.State != "not_listening" {
				result.State, result.Reason = "unknown", "address_family_unverified"
			}
		default:
			result.State, result.Reason = "not_listening", "socket_missing"
		}
	}
	return result
}

func (s *Service) runtimePID() int {
	if s.pm == nil {
		return 0
	}
	s.pm.mu.Lock()
	defer s.pm.mu.Unlock()
	if !s.pm.isRunning() {
		return 0
	}
	return s.pm.pid
}

func (s *Service) ListenerRuntime(listeners []models.Listener, details bool) []ListenerRuntime {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	results := make([]ListenerRuntime, 0, len(listeners))
	pid := s.runtimePID()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connections, inspectionErr := ownedConnections(ctx, pid)
	sockets := indexSockets(pid, connections)
	content, configErr := s.cm.ReadConfig()
	configHash := sha256.Sum256([]byte(content))
	configured, parseErr := runtimeListeners(content)
	byName := make(map[string]runtimeListener, len(configured))
	for _, l := range configured {
		byName[l.Name] = l
	}
	for _, l := range listeners {
		result := ListenerRuntime{ID: l.ID, State: "unknown", Reason: "inspection_unavailable", CheckedAt: time.Now().UTC(), Endpoints: []RuntimeEndpoint{}}
		switch {
		case !l.Enabled:
			result.State, result.Reason = "disabled", "disabled"
		case pid == 0:
			result.State, result.Reason = "not_listening", "core_stopped"
		case configErr != nil || parseErr != nil:
			result.Reason = "config_unavailable"
		default:
			if configured, ok := byName[l.Name]; ok {
				endpoints, endpointErr := configured.endpoints()
				result = inspectEndpoints(endpoints, endpointErr, sockets, inspectionErr, details)
				result.ID = l.ID
			} else {
				result.State, result.Reason = "not_listening", "config_not_applied"
			}
		}
		result.ConnectionCheck = s.cachedConnectionResult(l, pid, configHash)
		results = append(results, result)
	}
	return results
}

// Called while applyMu is held. A live process alone does not prove that its
// individual listeners started: Mihomo may log a bind error and keep running.
func (s *Service) waitForListeners(content string) error {
	listeners, err := runtimeListeners(content)
	if err != nil {
		return err
	}
	// Prepare the immutable expectations once, not on every retry.
	expected := make([][]RuntimeEndpoint, len(listeners))
	for i, l := range listeners {
		expected[i], _ = l.endpoints()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return waitForListenerSockets(ctx, listeners, expected, s.runtimePID, ownedConnections)
}

func waitForListenerSockets(ctx context.Context, listeners []runtimeListener, expected [][]RuntimeEndpoint, pidFunc func() int, inspect func(context.Context, int) ([]runtimeConnection, error)) error {
	var missing string
	for {
		if ctx.Err() != nil {
			if missing != "" {
				return fmt.Errorf("%s", missing)
			}
			return nil // No completed inspection: unknown, not proof of a bind failure.
		}
		pid := pidFunc()
		if pid == 0 {
			return fmt.Errorf("Mihomo stopped before listener readiness could be verified")
		}
		connections, inspectionErr := inspect(ctx, pid)
		if inspectionErr != nil {
			if ctx.Err() != nil && missing != "" {
				return fmt.Errorf("%s", missing)
			}
			return nil
		}
		sockets := indexSockets(pid, connections)
		missing = ""
		for i, l := range listeners {
			for _, endpoint := range expected[i] {
				if sockets[endpointKey(endpoint.Network, endpoint.Address, endpoint.Port)] == 0 {
					missing = fmt.Sprintf("listener %q did not start on %s %s; check port conflicts and binding permissions", l.Name, endpoint.Network, net.JoinHostPort(endpoint.Address, strconv.Itoa(int(endpoint.Port))))
					break
				}
			}
			if missing != "" {
				break
			}
		}
		if missing == "" {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%s", missing)
		case <-timer.C:
		}
	}
}
