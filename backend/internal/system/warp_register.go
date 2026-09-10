package system

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/curve25519"
)

// WARPRegisterResult is the material needed to build a Mihomo WARP outbound.
type WARPRegisterResult struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	Address    string `json:"address"`
	Reserved   string `json:"reserved,omitempty"`
	YAML       string `json:"yaml"`
}

type cfRegRequest struct {
	Key          string `json:"key"`
	InstallID    string `json:"install_id"`
	FCMToken     string `json:"fcm_token"`
	TOS          string `json:"tos"`
	Model        string `json:"model"`
	SerialNumber string `json:"serial_number"`
	Locale       string `json:"locale"`
}

type cfRegResponse struct {
	Result struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Model  string `json:"model"`
		Config struct {
			Peers []struct {
				PublicKey string `json:"public_key"`
				Endpoint  struct {
					Host string `json:"host"`
					V4   string `json:"v4"`
					V6   string `json:"v6"`
				} `json:"endpoint"`
			} `json:"peers"`
			Interface struct {
				Addresses struct {
					V4 string `json:"v4"`
					V6 string `json:"v6"`
				} `json:"addresses"`
			} `json:"interface"`
			ClientID string `json:"client_id"`
		} `json:"config"`
		Token   string `json:"token"`
		Account struct {
			AccountType string `json:"account_type"`
		} `json:"account"`
	} `json:"result"`
}

func genWireGuardKeyPair() (privB64, pubB64 string, err error) {
	var priv [32]byte
	if _, err = rand.Read(priv[:]); err != nil {
		return "", "", err
	}
	// clamp
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)
	return base64.StdEncoding.EncodeToString(priv[:]), base64.StdEncoding.EncodeToString(pub[:]), nil
}

// RegisterWARP performs a Cloudflare WARP device registration (wgcf-style) and
// returns private key, assigned addresses, and a ready Mihomo YAML fragment.
func RegisterWARP() (*WARPRegisterResult, error) {
	priv, pub, err := genWireGuardKeyPair()
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(cfRegRequest{
		Key:          pub,
		InstallID:    "",
		FCMToken:     "",
		TOS:          time.Now().UTC().Format(time.RFC3339),
		Model:        "3m-ui",
		SerialNumber: "",
		Locale:       "en_US",
	})
	req, err := http.NewRequest(http.MethodPost, "https://api.cloudflareclient.com/v0a2158/reg", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("warp register request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("warp register HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed cfRegResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("warp register decode: %w", err)
	}
	v4 := parsed.Result.Config.Interface.Addresses.V4
	v6 := parsed.Result.Config.Interface.Addresses.V6
	addr := v4
	if v6 != "" {
		if addr != "" {
			addr = addr + "," + v6
		} else {
			addr = v6
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("warp register: empty interface addresses")
	}
	reserved := ""
	if cid := parsed.Result.Config.ClientID; cid != "" {
		// client_id is often base64 3-byte reserved; pass through as-is when short
		reserved = cid
	}
	yaml, err := WARPTemplate(priv, addr, reserved)
	if err != nil {
		return nil, err
	}
	return &WARPRegisterResult{
		PrivateKey: priv,
		PublicKey:  pub,
		Address:    addr,
		Reserved:   reserved,
		YAML:       yaml,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
