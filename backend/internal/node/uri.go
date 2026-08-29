package node

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
)

func ClientURIs(listener models.Listener, host string) ([]string, error) {
	host = netutil.NormalizeHost(host)
	if host == "" {
		host = netutil.NormalizeHost(listener.PublicHost)
	}
	if host == "" {
		host = normalizeExportHost("", listener.BindAddress, listener.Listen)
	}
	if host == "" {
		return nil, fmt.Errorf("cannot determine public host for listener; set public_host / server.public_url or access via a public hostname (IPv4/IPv6 supported)")
	}
	cfg, err := decodeURIConfig(listener.Config)
	if err != nil {
		return nil, err
	}
	cfg["_listener-tls"] = listener.TLS
	cfg["_listener-udp"] = listener.UDP
	// m-ui style Access Profile overrides for share/subscription client links.
	if sni := strings.TrimSpace(listener.AccessSNI); sni != "" {
		cfg["sni"] = sni
		cfg["servername"] = sni
	}
	if fp := strings.TrimSpace(listener.ClientFingerprint); fp != "" {
		cfg["client-fingerprint"] = fp
		cfg["fingerprint"] = fp
	}
	if alpn := strings.TrimSpace(listener.AccessALPN); alpn != "" {
		cfg["alpn"] = alpn
	}
	port := strings.TrimSpace(listener.PublicPort)
	if port == "" {
		port = strings.TrimSpace(listener.Port)
	}
	if strings.ContainsAny(port, ",-") {
		return nil, fmt.Errorf("URI export requires a single listener port; ranges and port lists are not representable in a share URI")
	}
	switch strings.ToLower(listener.Protocol) {
	case "shadowsocks":
		return shadowsocksURIs(listener.Name, host, port, cfg)
	case "snell":
		return snellURIs(listener.Name, host, port, cfg)
	case "vless":
		return vlessURIs(listener.Name, host, port, cfg)
	case "vmess":
		return vmessURIs(listener.Name, host, port, cfg)
	case "trojan":
		return trojanURIs(listener.Name, host, port, cfg)
	case "hysteria2":
		return hysteria2URIs(listener.Name, host, port, cfg)
	case "tuic":
		return tuicURIs(listener.Name, host, port, cfg)
	case "shadowquic":
		return shadowQUICURIs(listener.Name, host, port, cfg)
	case "anytls":
		return anytlsURIs(listener.Name, host, port, cfg)
	case "mieru":
		return mieruURIs(listener.Name, host, port, cfg)
	case "sudoku":
		return sudokuURIs(listener.Name, host, port, cfg)
	case "trusttunnel":
		return trustTunnelURIs(listener.Name, host, port, cfg)
	default:
		return nil, fmt.Errorf("URI export is not supported for listener protocol %q", listener.Protocol)
	}
}

func decodeURIConfig(raw string) (map[string]interface{}, error) {
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

func userMap(cfg map[string]interface{}) map[string]interface{} {
	if users, ok := cfg["users"].(map[string]interface{}); ok {
		return users
	}
	return nil
}

func userRows(cfg map[string]interface{}) []map[string]interface{} {
	users, _ := cfg["users"].([]interface{})
	rows := make([]map[string]interface{}, 0, len(users))
	for _, raw := range users {
		if row, ok := raw.(map[string]interface{}); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func query(base string, values map[string]string) string {
	q := url.Values{}
	for k, v := range values {
		if v != "" {
			q.Set(k, v)
		}
	}
	if encoded := q.Encode(); encoded != "" {
		return base + "?" + encoded
	}
	return base
}

func addName(uri, name string) string {
	if name == "" {
		return uri
	}
	// Remark fragment; PathEscape matches common panel clients (incl. 3x-ui style).
	return uri + "#" + url.PathEscape(name)
}
