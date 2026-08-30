package node

import (
	"fmt"
	"net"
	"net/url"
)

// Extended Mihomo protocols use their documented/simple sharing forms where
// available. These links are intended for clients that understand the scheme;
// they are not silently downgraded to another protocol.
func snellURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	psk, _ := cfg["psk"].(string)
	if psk == "" {
		return nil, fmt.Errorf("snell listener requires psk for URI export")
	}
	params := map[string]string{"psk": psk}
	if v := stringValue(cfg["version"], "4"); v != "" {
		params["version"] = v
	}
	if obfs, ok := cfg["obfs-opts"].(map[string]interface{}); ok {
		if mode := stringValue(obfs["mode"], ""); mode != "" {
			params["obfs"] = mode
		}
		if host := stringValue(obfs["host"], ""); host != "" {
			params["obfs-host"] = host
		}
	}
	if b, ok := cfg["udp"].(bool); ok && b {
		params["udp"] = "1"
	}
	return []string{addName(query("snell://"+net.JoinHostPort(host, port), params), name)}, nil
}

func shadowQUICURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	// ShadowQUIC uses array-form users: [{username, password}] per official docs.
	rows := userRows(cfg)
	if len(rows) == 0 {
		return nil, fmt.Errorf("shadowquic listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		username, _ := row["username"].(string)
		password, _ := row["password"].(string)
		if password == "" {
			return nil, fmt.Errorf("shadowquic user %q has empty password", username)
		}
		params := map[string]string{}
		for _, key := range []string{"sni", "alpn", "quic-versions", "congestion-controller", "up", "down", "cwnd", "bbr-profile", "max-datagram-frame-size", "max-open-streams", "recv-window-conn", "recv-window"} {
			if v := stringValue(cfg[key], ""); v != "" {
				params[key] = v
			}
		}
		if b, ok := cfg["zero-rtt"].(bool); ok && b {
			params["zero-rtt"] = "1"
		}
		if b, ok := cfg["udp-over-stream"].(bool); ok && b {
			params["udp-over-stream"] = "1"
		}
		if b, ok := cfg["disable-mtu-discovery"].(bool); ok && b {
			params["disable-mtu-discovery"] = "1"
		}
		result = append(result, addName(query("shadowquic://"+url.PathEscape(username)+":"+url.PathEscape(password)+"@"+net.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func mieruURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	users := userMap(cfg)
	if len(users) == 0 {
		return nil, fmt.Errorf("mieru listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(users))
	for username, raw := range users {
		password, ok := raw.(string)
		if !ok || password == "" {
			return nil, fmt.Errorf("mieru user %q has empty password", username)
		}
		params := map[string]string{}
		for _, key := range []string{"transport", "multiplexing", "handshake-mode", "traffic-pattern"} {
			if v := stringValue(cfg[key], ""); v != "" {
				params[key] = v
			}
		}
		if v := stringValue(cfg["port-range"], ""); v != "" {
			params["portRange"] = v
		}
		result = append(result, addName(query("mieru://"+url.PathEscape(username)+":"+url.PathEscape(password)+"@"+net.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func sudokuURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	key, _ := cfg["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("sudoku listener requires key for URI export")
	}
	params := map[string]string{"key": key}
	for _, keyName := range []string{"aead-method", "padding-min", "padding-max", "table-type", "custom-table", "handshake-timeout", "httpmask"} {
		if v := stringValue(cfg[keyName], ""); v != "" {
			params[keyName] = v
		}
	}
	if b, ok := cfg["enable-pure-downlink"].(bool); ok && b {
		params["enable-pure-downlink"] = "1"
	}
	return []string{addName(query("sudoku://"+net.JoinHostPort(host, port), params), name)}, nil
}

func trustTunnelURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	// TrustTunnel uses array-form users: [{username, password}] per official docs.
	rows := userRows(cfg)
	if len(rows) == 0 {
		return nil, fmt.Errorf("trusttunnel listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		username, _ := row["username"].(string)
		password, _ := row["password"].(string)
		if password == "" {
			return nil, fmt.Errorf("trusttunnel user %q has empty password", username)
		}
		params := map[string]string{}
		for _, key := range []string{"client-fingerprint", "health-check", "sni", "alpn", "congestion-controller", "bbr-profile", "max-connections", "min-streams", "max-streams"} {
			if v := stringValue(cfg[key], ""); v != "" {
				params[key] = v
			}
		}
		for _, key := range []string{"udp", "quic", "skip-cert-verify", "name-cert-verify"} {
			if b, ok := cfg[key].(bool); ok && b {
				params[key] = "1"
			}
		}
		result = append(result, addName(query("trusttunnel://"+url.PathEscape(username)+":"+url.PathEscape(password)+"@"+net.JoinHostPort(host, port), params), name))
	}
	return result, nil
}
