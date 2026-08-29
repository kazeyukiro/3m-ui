package converter

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

func ResolveServerAddress(cfg *config.Config, req *http.Request) string {
	if envURL := os.Getenv("PUBLIC_URL"); envURL != "" {
		return cleanURLHost(envURL)
	}
	if cfg != nil && cfg.Server.PublicURL != "" {
		return cleanURLHost(cfg.Server.PublicURL)
	}
	if req != nil && req.Host != "" {
		return cleanURLHost(req.Host)
	}
	return "127.0.0.1"
}

// ResolveListenerServer prefers per-listener PublicHost (IPv4/IPv6/domain),
// then the global public URL / request host. Used for share links and client YAML.
func ResolveListenerServer(cfg *config.Config, req *http.Request, l models.Listener) string {
	if h := netutil.NormalizeHost(l.PublicHost); h != "" && !netutil.IsUnspecifiedBind(h) {
		return h
	}
	return ResolveServerAddress(cfg, req)
}

// ResolveListenerPort prefers PublicPort when set (NAT / dual-stack publish).
func ResolveListenerPort(l models.Listener) string {
	if p := strings.TrimSpace(l.PublicPort); p != "" {
		return p
	}
	return strings.TrimSpace(l.Port)
}

func cleanURLHost(raw string) string {
	u := netutil.NormalizeHost(raw)
	if u == "" || strings.ContainsAny(u, "\r\n/\\") {
		return "127.0.0.1"
	}
	return u
}

func GetSubscriptionURL(cfg *config.Config, req *http.Request, token string, target string) string {
	var base string
	if envURL := os.Getenv("PUBLIC_URL"); envURL != "" {
		base = strings.TrimSpace(envURL)
	} else if cfg != nil && cfg.Server.PublicURL != "" {
		base = strings.TrimSpace(cfg.Server.PublicURL)
	} else if req != nil && req.Host != "" {
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		} else if proto := req.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
			scheme = "https"
		}
		base = fmt.Sprintf("%s://%s", scheme, req.Host)
	} else {
		base = "http://127.0.0.1:8080"
	}
	base = strings.TrimSuffix(base, "/")
	pathToken := url.PathEscape(token)
	if target == "" {
		return fmt.Sprintf("%s/api/v1/client/sub/%s", base, pathToken)
	}
	return fmt.Sprintf("%s/api/v1/client/sub/%s?target=%s", base, pathToken, url.QueryEscape(strings.ToLower(strings.TrimSpace(target))))
}

func GenerateRawConfig(db *gorm.DB, token models.AccessToken, req *http.Request) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if token.ListenerID == 0 {
		return nil, fmt.Errorf("access token is not bound to a listener")
	}
	var listener models.Listener
	if err := db.First(&listener, token.ListenerID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("listener not found")
		}
		return nil, fmt.Errorf("failed to fetch listener: %w", err)
	}
	host := ResolveListenerServer(nil, req, listener)
	creds := []user.Credential{}
	// Access tokens historically carried a single credential payload; keep empty here
	// and let listenerToProxies fall back to config-embedded users when needed.
	proxies, err := listenerToProxies(listener, host, creds)
	if err != nil {
		return nil, err
	}
	cfg := map[string]interface{}{
		"proxies": proxies,
		"proxy-groups": []map[string]interface{}{
			{"name": "PROXY", "type": "select", "proxies": func() []string {
				names := make([]string, 0, len(proxies))
				for _, p := range proxies {
					if n, ok := p["name"].(string); ok {
						names = append(names, n)
					}
				}
				return names
			}()},
		},
		"rules": []string{"MATCH,PROXY"},
	}
	return yaml.Marshal(cfg)
}
