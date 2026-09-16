package listener

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

// QuickCreateInput is the minimal payload for one-click listener creation.
type QuickCreateInput struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
}

// SeedQuickConfig returns a minimal panel config for protocol so AutofillListenerDefaults
// can mint credentials, REALITY keys, and TLS material. Operators only supply name + protocol.
func SeedQuickConfig(protocol string) map[string]interface{} {
	cfg := map[string]interface{}{}
	switch protocol {
	case "vless":
		cfg["security_layer"] = "reality"
		cfg["transport_layer"] = "raw"
		cfg["flow"] = "xtls-rprx-vision"
		// dest is required by autofillReality; provide a widely used default target.
		cfg["reality-config"] = map[string]interface{}{
			"dest":         "www.microsoft.com:443",
			"server-names": []string{"www.microsoft.com"},
		}
	case "vmess", "trojan":
		cfg["security_layer"] = "reality"
		cfg["transport_layer"] = "raw"
		cfg["reality-config"] = map[string]interface{}{
			"dest":         "www.microsoft.com:443",
			"server-names": []string{"www.microsoft.com"},
		}
	case "shadowsocks":
		cfg["cipher"] = "aes-128-gcm"
	case "snell":
		cfg["version"] = 4
	case "shadowquic":
	case "tuic-v4":
		// Wiki inbound tuic-v4: token list + TLS (cert/token filled by autofill).
		// Do NOT set token: [] — empty slice is truthy in compile and yields broken auth.
		cfg["alpn"] = []string{"h3"}
		cfg["congestion-controller"] = "bbr"
	case "tuic-v5", "tuic":
		// Wiki inbound tuic-v5: users UUID→password + TLS.
		cfg["alpn"] = []string{"h3"}
		cfg["congestion-controller"] = "bbr"
	}
	return cfg
}

func protocolDefaultUDP(protocol string) bool {
	switch protocol {
	case "shadowsocks", "hysteria2", "tuic", "tuic-v4", "tuic-v5", "shadowquic":
		return true
	default:
		return false
	}
}

// allocateFreePort picks an unused TCP port in 10000–60000.
// Caller must hold s.mu when concurrent creates are possible.
// exclude lists ports already tried in this request so retries cannot redraw them.
func (s *Service) allocateFreePort(exclude ...int) (string, error) {
	var list []models.Listener
	if err := s.db.Find(&list).Error; err != nil {
		return "", fmt.Errorf("list listeners for port allocation: %w", err)
	}
	used := map[int]struct{}{}
	for _, reserved := range []int{22, 53, 80, 443, 8080, 8443, 9090} {
		used[reserved] = struct{}{}
	}
	for _, e := range exclude {
		if e > 0 {
			used[e] = struct{}{}
		}
	}
	for _, l := range list {
		ranges, ok := portRanges(l.Port)
		if !ok {
			continue
		}
		for _, r := range ranges {
			for p := r[0]; p <= r[1]; p++ {
				used[p] = struct{}{}
			}
		}
	}
	const minP, maxP = 10000, 60000
	span := maxP - minP + 1
	for attempt := 0; attempt < 300; attempt++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(span)))
		if err != nil {
			return "", err
		}
		p := minP + int(n.Int64())
		if _, taken := used[p]; !taken {
			return strconv.Itoa(p), nil
		}
	}
	return "", fmt.Errorf("no free port in %d-%d", minP, maxP)
}

// QuickCreate builds a fully working listener from name + protocol only.
func (s *Service) QuickCreate(in QuickCreateInput) (*models.Listener, error) {
	name := strings.TrimSpace(in.Name)
	protocol := strings.ToLower(strings.TrimSpace(in.Protocol))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if protocol == "" || !dbconfig.IsMihomoListenerProtocol(protocol) {
		return nil, fmt.Errorf("unsupported protocol %q", protocol)
	}
	cfg := SeedQuickConfig(protocol)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	l := &models.Listener{
		Name:              name,
		Protocol:          protocol,
		Type:              protocol,
		Port:              "0", // Create allocates under lock
		BindAddress:       "0.0.0.0",
		Enabled:           true,
		UDP:               protocolDefaultUDP(protocol),
		Config:            string(raw),
		ClientFingerprint: "chrome",
		Status:            "inactive",
	}
	if err := s.Create(l); err != nil {
		return nil, err
	}
	return l, nil
}
