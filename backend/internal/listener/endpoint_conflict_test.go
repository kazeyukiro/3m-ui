package listener

import (
	"path/filepath"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// A TCP listener (vless) and a UDP listener (tuic) on the same address:port must
// not be flagged as a conflict: they bind independent sockets at the OS level.
// The old guard rejected the second one with "conflicts on 0.0.0.0:443".
func TestEnsureEndpointAvailableTCPvsUDP(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() }()

	svc := NewService(db, filepath.Join(t.TempDir(), "config.yaml"), nil)

	tcp := &models.Listener{Name: "vless-443", Protocol: "vless", Port: "443", BindAddress: "0.0.0.0", Enabled: true, Config: `{}`}
	if err := svc.Create(tcp); err != nil {
		t.Fatalf("seed TCP listener: %v", err)
	}

	// UDP on the same address:port must be allowed (independent socket).
	udp := &models.Listener{Name: "tuic-443", Protocol: "tuic", Port: "443", BindAddress: "0.0.0.0", Enabled: true, Config: `{"token":["aaaaaaaaaaaaaaaa"]}`}
	if err := svc.ensureEndpointAvailable(udp); err != nil {
		t.Fatalf("TCP+UDP on 443 wrongly flagged as conflict: %v", err)
	}
	if err := svc.Create(udp); err != nil {
		t.Fatalf("create UDP listener on 443: %v", err)
	}

	// Two TCP on the same address:port must still conflict (checked vs the seeded vless).
	tcp2 := &models.Listener{Name: "vless-443-b", Protocol: "vless", Port: "443", BindAddress: "0.0.0.0", Enabled: true, Config: `{}`}
	if err := svc.ensureEndpointAvailable(tcp2); err == nil {
		t.Fatal("expected conflict between two TCP listeners on 443, got nil")
	}

	// Two UDP on the same address:port must still conflict (checked vs the seeded tuic).
	udp2 := &models.Listener{Name: "hy2-443", Protocol: "hysteria2", Port: "443", BindAddress: "0.0.0.0", Enabled: true, Config: `{"password":"test-pass"}`}
	if err := svc.ensureEndpointAvailable(udp2); err == nil {
		t.Fatal("expected conflict between two UDP listeners on 443, got nil")
	}
}

// End-to-end through Create: the UDP listener must stay bound to 443 (not be
// silently reassigned to a free port by the conflict-retry path).
func TestCreateKeepsUDPOnSamePort(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() }()

	svc := NewService(db, filepath.Join(t.TempDir(), "config.yaml"), nil)

	tcp := &models.Listener{Name: "vless-443", Protocol: "vless", Port: "443", BindAddress: "0.0.0.0", Enabled: true, Config: `{}`}
	if err := svc.Create(tcp); err != nil {
		t.Fatalf("seed TCP listener: %v", err)
	}
	udp := &models.Listener{Name: "tuic-443", Protocol: "tuic", Port: "443", BindAddress: "0.0.0.0", Enabled: true, Config: `{"token":["aaaaaaaaaaaaaaaa"]}`}
	if err := svc.Create(udp); err != nil {
		t.Fatalf("UDP listener on 443 rejected: %v", err)
	}
	if udp.Port != "443" {
		t.Fatalf("UDP listener was reassigned off 443 (got port %q); TCP+UDP should coexist", udp.Port)
	}
}
