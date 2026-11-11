package node

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// mihomo v1.19.34 raises the Snell version ceiling to 6 and adds a v6-only
// `mode` field (default / unshaped / unsafe-raw), forbids obfs http/tls on
// v6, and requires a 12..255 byte psk on v6 (docs/config.yaml).
func TestSnellV6Validation(t *testing.T) {
	cases := []struct {
		name    string
		cfg     string
		wantErr string
	}{
		{"v6 default", `{"psk":"abcdefghijkl","version":6,"mode":"default"}`, ""},
		{"v6 unshaped", `{"psk":"abcdefghijkl","version":6,"mode":"unshaped"}`, ""},
		{"v6 unsafe-raw", `{"psk":"abcdefghijkl","version":6,"mode":"unsafe-raw"}`, ""},
		{"v6 no mode", `{"psk":"abcdefghijkl","version":6}`, ""},
		{"legacy v5", `{"psk":"x","version":5}`, ""},
		{"mode on v5", `{"psk":"x","version":5,"mode":"default"}`, "only supported on version 6"},
		{"mode without version", `{"psk":"x","mode":"default"}`, "only supported on version 6"},
		{"invalid mode", `{"psk":"abcdefghijkl","version":6,"mode":"bogus"}`, "default, unshaped, or unsafe-raw"},
		{"version 7", `{"psk":"abcdefghijkl","version":7}`, "between 1 and 6"},
		{"v6 obfs http", `{"psk":"abcdefghijkl","version":6,"obfs-opts":{"mode":"http","host":"bing.com"}}`, "does not support obfs mode http"},
		{"v6 obfs tls", `{"psk":"abcdefghijkl","version":6,"obfs-opts":{"mode":"tls","host":"bing.com"}}`, "does not support obfs mode tls"},
		{"v6 short psk", `{"psk":"short","version":6}`, "12 to 255 bytes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateNode(validNode("snell", c.cfg))
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error containing %q, got %v", c.wantErr, err)
				}
			}
		})
	}
}

func TestSnellURIsCarriesV6Mode(t *testing.T) {
	uris, err := snellURIs("s", "1.2.3.4", "443", map[string]interface{}{
		"psk": "abcdefghijkl", "version": 6, "mode": "unsafe-raw",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 1 || !strings.Contains(uris[0], "mode=unsafe-raw") {
		t.Fatalf("snell URI must carry v6 mode: %v", uris)
	}
}

func ssSubscriptionURI(t *testing.T, cfgJSON string, username string) []string {
	t.Helper()
	l := models.Listener{
		Name: "ss", Protocol: "shadowsocks", Type: "shadowsocks", Port: "443",
		BindAddress: "0.0.0.0", PublicHost: "example.com", Enabled: true,
		Config: cfgJSON,
	}
	uris, err := ClientURIsWithCredentials(l, "example.com", []user.Credential{
		{Username: username, Password: "panel-password"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return uris
}

func TestSSSubscriptionURI_AEADMultiUser(t *testing.T) {
	uris := ssSubscriptionURI(t, `{"cipher":"aes-128-gcm","users":{"alice":"pw-a"}}`, "alice")
	want := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:pw-a"))
	if !strings.Contains(strings.Join(uris, "\n"), want) {
		t.Fatalf("subscription must carry alice's users password: %v", uris)
	}
	panel := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:panel-password"))
	if strings.Contains(strings.Join(uris, "\n"), panel) {
		t.Fatal("subscription must not fall back to the panel user password")
	}
}

func TestSSSubscriptionURI_2022MultiUser(t *testing.T) {
	uris := ssSubscriptionURI(t,
		`{"cipher":"2022-blake3-aes-128-gcm","password":"srv-psk","users":{"alice":"u-psk"}}`, "alice")
	want := base64.StdEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:srv-psk:u-psk"))
	if !strings.Contains(strings.Join(uris, "\n"), want) {
		t.Fatalf("subscription must carry serverPSK:userPSK: %v", uris)
	}
}
