package config

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestAccessSNIIsOnlyACertificateHint(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, reality := range []bool{false, true} {
			cfg := map[string]interface{}{"users": []map[string]string{{"username": "test", "uuid": "6f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}}}
			if legacy {
				cfg["sni"] = "test.example.com"
			}
			if reality {
				cfg["reality-config"] = map[string]interface{}{"dest": "test.example.com:443", "private-key": "test-key", "server-names": []string{"test.example.com"}, "short-id": []string{"abcdef"}}
			}
			raw, _ := json.Marshal(cfg)
			l := models.Listener{Name: "test", Protocol: "vless", Port: "12345", BindAddress: "0.0.0.0", Enabled: true, AccessSNI: "test.example.com", Config: string(raw)}
			listeners, err := generateListeners(nil, []models.Listener{l}, nil)
			if err != nil || len(listeners) != 1 {
				t.Fatalf("legacy=%v reality=%v: %v", legacy, reality, err)
			}
			if _, ok := listeners[0]["sni"]; ok {
				t.Fatal("client SNI leaked into inbound")
			}
			if !reality {
				cert, _ := listeners[0]["certificate"].(string)
				block, _ := pem.Decode([]byte(cert))
				if block == nil {
					t.Fatal("no certificate")
				}
				parsed, err := x509.ParseCertificate(block.Bytes)
				if err != nil || parsed.VerifyHostname(l.AccessSNI) != nil {
					t.Fatal("certificate lost access SNI hint")
				}
			}
		}
	}
}

func TestUnrelatedInvalidSNIRemainsRejected(t *testing.T) {
	_, err := generateListeners(nil, []models.Listener{{Name: "bad", Protocol: "vless", Port: "12345", Enabled: true, AccessSNI: "access.example.com", Config: `{"sni":"different.example.com"}`}}, nil)
	if err == nil {
		t.Fatal("unrelated invalid inbound field was silently removed")
	}
}
