package converter

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func TestVLESSClientUsesEncryptionNotDecryption(t *testing.T) {
	l := models.Listener{
		Name: "vless-enc", Protocol: "vless", Port: "443", Enabled: true,
		Config: `{"flow":"xtls-rprx-vision","decryption":"server-decryption-only","encryption":"client-encryption-pair"}`,
	}
	creds := []user.Credential{{Username: "u1", UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	p := proxies[0]
	if p["encryption"] != "client-encryption-pair" {
		t.Fatalf("expected client encryption pair, got %#v", p["encryption"])
	}
	if p["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flow: %#v", p["flow"])
	}
}

func TestVLESSClientDoesNotCopyDecryption(t *testing.T) {
	l := models.Listener{
		Name: "vless-enc", Protocol: "vless", Port: "443", Enabled: true,
		Config: `{"flow":"xtls-rprx-vision","decryption":"server-decryption-only"}`,
	}
	creds := []user.Credential{{Username: "u1", UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if proxies[0]["encryption"] != nil {
		t.Fatalf("must not copy decryption→encryption, got %#v", proxies[0]["encryption"])
	}
}
