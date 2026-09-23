package system

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// Cloudflare consumer registration API (same endpoint family as m-ui / 3x-ui / wgcf).
const warpAPIBase = "https://api.cloudflareclient.com/v0a2158"

// WARPRegisterResult is the material needed to build a Mihomo WARP outbound.
type WARPRegisterResult struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"` // peer (server) public key
	Address    string `json:"address"`
	IPv6       string `json:"ipv6,omitempty"`
	Reserved   []int  `json:"reserved,omitempty"`
	Server     string `json:"server,omitempty"`
	Port       int    `json:"port,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	YAML       string `json:"yaml"`
	MasqueYAML string `json:"masque_yaml,omitempty"`
}

type warpDevice struct {
	ID      string           `json:"id"`
	Token   string           `json:"token,omitempty"`
	Name    string           `json:"name"`
	Model   string           `json:"model"`
	Enabled bool             `json:"enabled"`
	Account warpAccountInfo  `json:"account"`
	Config  warpTunnelConfig `json:"config"`
}

type warpAccountInfo struct {
	License     string `json:"license"`
	AccountType string `json:"account_type"`
	Role        string `json:"role"`
	PremiumData uint64 `json:"premium_data"`
	Quota       uint64 `json:"quota"`
	Usage       uint64 `json:"usage"`
}

type warpTunnelConfig struct {
	ClientID  string `json:"client_id"`
	Interface struct {
		Addresses struct {
			V4 string `json:"v4"`
			V6 string `json:"v6"`
		} `json:"addresses"`
	} `json:"interface"`
	Peers []struct {
		PublicKey string `json:"public_key"`
		Endpoint  struct {
			Host string `json:"host"`
			V4   string `json:"v4"`
			V6   string `json:"v6"`
		} `json:"endpoint"`
	} `json:"peers"`
}

type warpHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func defaultWarpClient() *warpHTTPClient {
	return &warpHTTPClient{
		baseURL: warpAPIBase,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *warpHTTPClient) request(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("warp: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("CF-Client-Version", "a-7.21-0721")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("warp: connect Cloudflare failed (check outbound network): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("warp: Cloudflare HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return fmt.Errorf("warp: response read failed or too large")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("warp: invalid response object")
	}
	var envelope struct {
		Success *bool             `json:"success"`
		Errors  []json.RawMessage `json:"errors"`
		Result  json.RawMessage   `json:"result"`
	}
	if err = json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("warp: invalid JSON")
	}
	if envelope.Success != nil && !*envelope.Success || len(envelope.Errors) > 0 {
		return fmt.Errorf("warp: Cloudflare rejected the request")
	}
	if len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		data = envelope.Result
	}
	if output != nil {
		if err = json.Unmarshal(data, output); err != nil {
			return fmt.Errorf("warp: decode device: %w", err)
		}
	}
	return nil
}

func decodeWarpKey(value string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(data) != 32 {
		return nil, fmt.Errorf("WireGuard key must be 32-byte base64")
	}
	return data, nil
}

func warpAddress(value string, ipv4 bool) (string, error) {
	if value == "" {
		return "", nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		if prefix, prefixErr := netip.ParsePrefix(value); prefixErr == nil {
			address = prefix.Addr()
			err = nil
		}
	}
	if err != nil || address.Is4() != ipv4 {
		return "", fmt.Errorf("invalid tunnel address %q", value)
	}
	return address.String(), nil
}

func valueOr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// RegisterWARP registers a Cloudflare WARP device (m-ui / 3x-ui style) and
// returns a Mihomo WireGuard outbound YAML fragment.
func RegisterWARP() (*WARPRegisterResult, error) {
	return registerWARPWithClient(context.Background(), defaultWarpClient())
}

func registerWARPWithClient(ctx context.Context, client *warpHTTPClient) (*WARPRegisterResult, error) {
	if client == nil {
		client = defaultWarpClient()
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("warp: generate key: %w", err)
	}
	privB64 := base64.StdEncoding.EncodeToString(key.Bytes())
	pubB64 := base64.StdEncoding.EncodeToString(key.PublicKey().Bytes())

	input := map[string]any{
		"key":   pubB64,
		"tos":   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"type":  "PC",
		"model": "3m-ui",
		"name":  "3m-ui",
	}
	var device warpDevice
	if err = client.request(ctx, http.MethodPost, "/reg", input, &device); err != nil {
		return nil, err
	}
	if device.ID == "" {
		return nil, fmt.Errorf("warp: empty device id")
	}
	if _, err = decodeWarpKey(privB64); err != nil {
		return nil, err
	}

	cfg := device.Config
	if len(cfg.Peers) == 0 {
		return nil, fmt.Errorf("warp: no WireGuard peer yet — retry registration")
	}
	peer := cfg.Peers[0]
	if _, err = decodeWarpKey(peer.PublicKey); err != nil {
		return nil, fmt.Errorf("warp: peer public-key invalid")
	}
	endpoint := valueOr(peer.Endpoint.Host, valueOr(peer.Endpoint.V4, peer.Endpoint.V6))
	host, portText, err := net.SplitHostPort(endpoint)
	port, portErr := strconv.Atoi(portText)
	if err != nil || portErr != nil || port < 1 || port > 65535 || host == "" || strings.ContainsAny(host, "/\r\n \t") {
		// Fallback: host-only endpoint + default WARP port
		host = strings.TrimSpace(endpoint)
		if host == "" || strings.ContainsAny(host, "/\r\n \t") {
			return nil, fmt.Errorf("warp: invalid peer endpoint %q", endpoint)
		}
		port = 2408
	}

	reservedRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cfg.ClientID))
	if err != nil || len(reservedRaw) != 3 {
		return nil, fmt.Errorf("warp: client_id must be 3-byte base64 (got %q)", cfg.ClientID)
	}
	reserved := []int{int(reservedRaw[0]), int(reservedRaw[1]), int(reservedRaw[2])}

	v4, err := warpAddress(cfg.Interface.Addresses.V4, true)
	if err != nil {
		return nil, err
	}
	v6, err := warpAddress(cfg.Interface.Addresses.V6, false)
	if err != nil {
		return nil, err
	}
	if v4 == "" && v6 == "" {
		return nil, fmt.Errorf("warp: empty interface addresses")
	}

	yamlStr, err := WARPTemplate(privB64, peer.PublicKey, host, port, v4, v6, reserved)
	if err != nil {
		return nil, err
	}
	masqueYAML, _ := WARPMasqueTemplate(privB64, v4, v6, "")

	return &WARPRegisterResult{
		PrivateKey: privB64,
		PublicKey:  peer.PublicKey,
		Address:    v4,
		IPv6:       v6,
		Reserved:   reserved,
		Server:     host,
		Port:       port,
		DeviceID:   device.ID,
		YAML:       yamlStr,
		MasqueYAML: masqueYAML,
	}, nil
}
