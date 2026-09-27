package node

import (
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

const testUUID = "00000000-0000-0000-0000-000000000001"

// XTLS Vision is TCP-only. A listener migrated to ws/grpc/xhttp keeps its flow in
// storage, and exporting it produced links that claim Vision over a transport
// that cannot carry it — clients either refuse to connect or silently misbehave.
// These pin that a stale flow never reaches a share URI, while a genuine TCP
// listener keeps its flow.
func TestVlessURIsOmitFlowForNonTCPTransports(t *testing.T) {
	cases := []struct {
		name     string
		cfg      map[string]interface{}
		wantType string
		wantFlow bool
	}{
		{
			name:     "raw keeps flow",
			cfg:      map[string]interface{}{"flow": "xtls-rprx-vision"},
			wantType: "tcp", wantFlow: true,
		},
		{
			name:     "ws drops flow",
			cfg:      map[string]interface{}{"flow": "xtls-rprx-vision", "ws-path": "/ws"},
			wantType: "ws", wantFlow: false,
		},
		{
			name:     "grpc drops flow",
			cfg:      map[string]interface{}{"flow": "xtls-rprx-vision", "grpc-service-name": "svc"},
			wantType: "grpc", wantFlow: false,
		},
		{
			name: "xhttp drops flow",
			cfg: map[string]interface{}{
				"flow": "xtls-rprx-vision",
				"xhttp-config": map[string]interface{}{
					"path": "/xhttp",
				},
			},
			wantType: "xhttp", wantFlow: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// vlessURIs reads the flow from the user row; the listener-level
			// value is merged into rows by ClientURIsWithCredentials, which is
			// covered separately below.
			row := map[string]interface{}{"username": "u", "uuid": testUUID}
			if flow, ok := c.cfg["flow"].(string); ok {
				row["flow"] = flow
			}
			cfg := map[string]interface{}{
				"users": []interface{}{row},
			}
			for _, key := range []string{"ws-path", "grpc-service-name", "xhttp-config"} {
				if v, ok := c.cfg[key]; ok {
					cfg[key] = v
				}
			}

			uris, err := vlessURIs("node", "example.com", "443", cfg)
			if err != nil {
				t.Fatalf("vlessURIs: %v", err)
			}
			if len(uris) != 1 {
				t.Fatalf("expected 1 URI, got %d: %v", len(uris), uris)
			}
			uri := uris[0]
			if !strings.Contains(uri, "type="+c.wantType) {
				t.Fatalf("expected type=%s in %q", c.wantType, uri)
			}
			if got := strings.Contains(uri, "flow="); got != c.wantFlow {
				t.Fatalf("flow present = %v, want %v: %s", got, c.wantFlow, uri)
			}
		})
	}
}

// The listener-level flow is copied onto every user before the URI is built.
// That copy used to be made on protocol alone, which reintroduced the value the
// form had deliberately dropped for ws.
func TestClientURIsWithCredentialsDropFlowForWebSocket(t *testing.T) {
	creds := []user.Credential{{Username: "u", UUID: testUUID}}

	ws := models.Listener{
		Name: "ws-node", Protocol: "vless", Type: "vless", Port: "443",
		BindAddress: "0.0.0.0", Enabled: true,
		Config: `{"flow":"xtls-rprx-vision","ws-path":"/ws"}`,
	}
	uris, err := ClientURIsWithCredentials(ws, "example.com", creds)
	if err != nil {
		t.Fatalf("ClientURIsWithCredentials: %v", err)
	}
	if len(uris) == 0 {
		t.Fatal("expected at least one URI")
	}
	for _, uri := range uris {
		if strings.Contains(uri, "flow=") {
			t.Fatalf("ws listener must not export a flow: %s", uri)
		}
		if !strings.Contains(uri, "type=ws") {
			t.Fatalf("expected a ws URI, got %s", uri)
		}
	}

	raw := models.Listener{
		Name: "tcp-node", Protocol: "vless", Type: "vless", Port: "443",
		BindAddress: "0.0.0.0", Enabled: true,
		Config: `{"flow":"xtls-rprx-vision"}`,
	}
	uris, err = ClientURIsWithCredentials(raw, "example.com", creds)
	if err != nil {
		t.Fatalf("ClientURIsWithCredentials: %v", err)
	}
	if len(uris) == 0 {
		t.Fatal("expected at least one URI")
	}
	if !strings.Contains(uris[0], "flow=xtls-rprx-vision") {
		t.Fatalf("tcp listener must keep its flow: %s", uris[0])
	}
}
