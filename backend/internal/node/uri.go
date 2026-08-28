package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"golang.org/x/crypto/curve25519"
)

// ClientURIs builds share URIs for a listener. host is the public address clients connect to.
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
