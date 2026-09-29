package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const warpAccountKey = "warp-account"
const WARPProxyName = "WARP"

// Cloudflare WARP WireGuard peer public key (shared by all consumer accounts).
const warpCloudflarePublicKey = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="

// WARPAccount is the persisted Cloudflare WARP device .
type WARPAccount struct {
	DeviceID      string    `json:"device_id"`
	AccessToken   string    `json:"access_token"`
	LicenseKey    string    `json:"license_key,omitempty"`
	PrivateKey    string    `json:"private_key"`
	LocalPublic   string    `json:"local_public,omitempty"`
	PeerPublicKey string    `json:"peer_public_key"`
	EndpointHost  string    `json:"endpoint_host"`
	EndpointPort  int       `json:"endpoint_port"`
	AddressV4     string    `json:"address_v4,omitempty"`
	AddressV6     string    `json:"address_v6,omitempty"`
	Reserved      []int     `json:"reserved,omitempty"`
	ClientID      string    `json:"client_id,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WARPAccountView is safe for API responses (token redacted).
type WARPAccountView struct {
	Configured    bool      `json:"configured"`
	DeviceID      string    `json:"device_id,omitempty"`
	LicenseKey    string    `json:"license_key,omitempty"`
	AddressV4     string    `json:"address_v4,omitempty"`
	AddressV6     string    `json:"address_v6,omitempty"`
	EndpointHost  string    `json:"endpoint_host,omitempty"`
	EndpointPort  int       `json:"endpoint_port,omitempty"`
	PeerPublicKey string    `json:"peer_public_key,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	ProxyName     string    `json:"proxy_name"`
}

func GetWARPAccount(db *gorm.DB) (*WARPAccount, error) {
	if db == nil {
		return nil, nil
	}
	var row models.PanelSetting
	err := db.Where("key = ?", warpAccountKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Value) == "" {
		return nil, nil
	}
	var acc WARPAccount
	if err := json.Unmarshal([]byte(row.Value), &acc); err != nil {
		return nil, fmt.Errorf("parse warp-account: %w", err)
	}
	if strings.TrimSpace(acc.PrivateKey) == "" {
		return nil, nil
	}
	return &acc, nil
}

func SaveWARPAccount(db *gorm.DB, acc *WARPAccount) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if acc == nil {
		return DeleteWARPAccount(db)
	}
	acc.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(acc)
	if err != nil {
		return err
	}
	var row models.PanelSetting
	err = db.Where("key = ?", warpAccountKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.PanelSetting{Key: warpAccountKey, Value: string(raw)}).Error
	}
	if err != nil {
		return err
	}
	row.Value = string(raw)
	return db.Save(&row).Error
}

func DeleteWARPAccount(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	return db.Where("key = ?", warpAccountKey).Delete(&models.PanelSetting{}).Error
}

func AccountFromRegister(res *WARPRegisterResult) *WARPAccount {
	if res == nil {
		return nil
	}
	epHost := res.EndpointHost
	if epHost == "" {
		epHost = "engage.cloudflareclient.com"
	}
	epPort := res.EndpointPort
	if epPort <= 0 {
		epPort = 2408
	}
	peer := strings.TrimSpace(res.PeerPublicKey)
	if peer == "" {
		peer = warpCloudflarePublicKey
	}
	return &WARPAccount{
		DeviceID:      res.DeviceID,
		AccessToken:   res.AccessToken,
		LicenseKey:    res.LicenseKey,
		PrivateKey:    res.PrivateKey,
		LocalPublic:   res.PublicKey,
		PeerPublicKey: peer,
		EndpointHost:  epHost,
		EndpointPort:  epPort,
		AddressV4:     stripCIDR(res.Address),
		AddressV6:     stripCIDR(res.IPv6),
		Reserved:      res.Reserved,
		ClientID:      res.ClientID,
		UpdatedAt:     time.Now().UTC(),
	}
}

func stripCIDR(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "/"); i > 0 {
		return s[:i]
	}
	return s
}

func (a *WARPAccount) View() WARPAccountView {
	if a == nil {
		return WARPAccountView{Configured: false, ProxyName: WARPProxyName}
	}
	return WARPAccountView{
		Configured:    true,
		DeviceID:      a.DeviceID,
		LicenseKey:    a.LicenseKey,
		AddressV4:     a.AddressV4,
		AddressV6:     a.AddressV6,
		EndpointHost:  a.EndpointHost,
		EndpointPort:  a.EndpointPort,
		PeerPublicKey: a.PeerPublicKey,
		UpdatedAt:     a.UpdatedAt,
		ProxyName:     WARPProxyName,
	}
}

// ProxyMap builds a Mihomo wireguard outbound map named WARP.
// Always sets Cloudflare's peer public-key and allowed-ips so DOMAIN rules
// can actually dial through the tunnel (matching WARPTemplate).
func (a *WARPAccount) ProxyMap() (map[string]interface{}, error) {
	if a == nil || strings.TrimSpace(a.PrivateKey) == "" {
		return nil, fmt.Errorf("WARP account not configured")
	}
	host := strings.TrimSpace(a.EndpointHost)
	if host == "" {
		host = "engage.cloudflareclient.com"
	}
	port := a.EndpointPort
	if port <= 0 {
		port = 2408
	}
	pk := strings.TrimSpace(a.PeerPublicKey)
	if pk == "" {
		pk = warpCloudflarePublicKey
	}
	ip := stripCIDR(a.AddressV4)
	if ip == "" {
		ip = "172.16.0.2"
	}
	m := map[string]interface{}{
		"name":               WARPProxyName,
		"type":               "wireguard",
		"server":             host,
		"port":               port,
		"private-key":        a.PrivateKey,
		"public-key":         pk,
		"ip":                 ip,
		"udp":                true,
		"mtu":                1280,
		"allowed-ips":        []string{"0.0.0.0/0", "::/0"},
		"remote-dns-resolve": true,
	}
	if v6 := stripCIDR(a.AddressV6); v6 != "" {
		m["ipv6"] = v6
	}
	if len(a.Reserved) > 0 {
		// Mihomo expects up to 3 reserved bytes for WARP.
		res := a.Reserved
		if len(res) > 3 {
			res = res[:3]
		}
		m["reserved"] = res
	}
	return m, nil
}
