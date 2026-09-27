package mui

import (
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

const flowTestUUID = "00000000-0000-0000-0000-000000000001"

// The m-ui share path is what the node API serves first; it must not hand out a
// Vision flow over a transport that cannot carry it.
func TestBuildSharesOmitFlowForWebSocket(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		wantFlow bool
	}{
		{"raw keeps flow", `{"flow":"xtls-rprx-vision"}`, true},
		{"ws drops flow", `{"flow":"xtls-rprx-vision","ws-path":"/ws"}`, false},
		{"grpc drops flow", `{"flow":"xtls-rprx-vision","grpc-service-name":"svc"}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := models.Listener{
				Name: "n", Protocol: "vless", Type: "vless", Port: "443",
				BindAddress: "0.0.0.0", Enabled: true, Config: c.config,
			}
			shares, err := BuildShares(l, "example.com", []Cred{{Username: "u", UUID: flowTestUUID}})
			if err != nil {
				t.Fatalf("BuildShares: %v", err)
			}
			if len(shares) == 0 {
				t.Fatal("expected at least one share")
			}
			uri := shares[0].URI
			if strings.TrimSpace(uri) == "" {
				t.Fatalf("empty share URI: %#v", shares[0])
			}
			if got := strings.Contains(uri, "flow="); got != c.wantFlow {
				t.Fatalf("flow present = %v, want %v: %s", got, c.wantFlow, uri)
			}
		})
	}
}
