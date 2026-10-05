package config

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func mkListener(name, protocol, addr, port string) models.Listener {
	return models.Listener{
		Name:        name,
		Protocol:    protocol,
		BindAddress: addr,
		Port:        port,
		Enabled:     true,
		Config:      `{}`,
	}
}

// A TCP-bound listener (vless) and a UDP-bound listener (tuic) on the same
// address:port are independent sockets and must not be flagged as a conflict,
// matching the creation-time guard. Regression test for the generation path
// that previously rejected this coexistence.
func TestValidateListenerEndpointsAllowsTCPandUDPOnSamePort(t *testing.T) {
	listeners := []models.Listener{
		mkListener("节点2", "vless", "0.0.0.0", "16201"),
		mkListener("节点5", "tuic", "0.0.0.0", "16201"),
	}
	if err := validateListenerEndpoints(listeners); err != nil {
		t.Fatalf("TCP+UDP on same address:port wrongly flagged as conflict: %v", err)
	}
}

// Two TCP-bound listeners on the same address:port must still conflict.
func TestValidateListenerEndpointsRejectsTwoTCPOnSamePort(t *testing.T) {
	listeners := []models.Listener{
		mkListener("节点2", "vless", "0.0.0.0", "16201"),
		mkListener("节点5", "vless", "0.0.0.0", "16201"),
	}
	if err := validateListenerEndpoints(listeners); err == nil {
		t.Fatal("expected conflict between two TCP listeners on 16201, got nil")
	}
}

// Two UDP-bound listeners on the same address:port must still conflict.
func TestValidateListenerEndpointsRejectsTwoUDPOnSamePort(t *testing.T) {
	listeners := []models.Listener{
		mkListener("节点2", "tuic", "0.0.0.0", "16201"),
		mkListener("节点5", "hysteria2", "0.0.0.0", "16201"),
	}
	if err := validateListenerEndpoints(listeners); err == nil {
		t.Fatal("expected conflict between two UDP listeners on 16201, got nil")
	}
}
