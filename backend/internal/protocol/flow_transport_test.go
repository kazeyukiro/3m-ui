package protocol

import (
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestTransportSpecCarriesFlow(t *testing.T) {
	cases := []struct {
		network string
		want    bool
	}{
		{"", true}, // empty means tcp
		{"tcp", true},
		{"TCP", true}, // case-insensitive
		{"raw", true},
		{"ws", false},
		{"grpc", false},
		{"xhttp", false},
		{"h2", false},
	}
	for _, c := range cases {
		if got := (TransportSpec{Network: c.network}).CarriesFlow(); got != c.want {
			t.Errorf("TransportSpec{Network:%q}.CarriesFlow() = %v, want %v", c.network, got, c.want)
		}
	}
}

func TestTransportCarriesFlowFromConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]interface{}
		want bool
	}{
		{"empty config", map[string]interface{}{}, true},
		{"ws path", map[string]interface{}{"ws-path": "/ws"}, false},
		{"blank ws path is not configured", map[string]interface{}{"ws-path": "  "}, true},
		{"grpc service", map[string]interface{}{"grpc-service-name": "svc"}, false},
		{"xhttp block", map[string]interface{}{"xhttp-config": map[string]interface{}{"path": "/x"}}, false},
		{"empty xhttp block is not configured", map[string]interface{}{"xhttp-config": map[string]interface{}{}}, true},
		{"mkcp block", map[string]interface{}{"mkcp-config": map[string]interface{}{"mtu": 1350}}, false},
		{"mekya block", map[string]interface{}{"mekya-config": map[string]interface{}{"enable": true}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TransportCarriesFlow(c.cfg); got != c.want {
				t.Fatalf("TransportCarriesFlow(%v) = %v, want %v", c.cfg, got, c.want)
			}
		})
	}
}

// A listener whose transport was switched to ws keeps its flow in storage; the
// exported share must not advertise it.
func TestVLESSShareOmitsFlowForWebSocket(t *testing.T) {
	const uuid = "00000000-0000-0000-0000-000000000001"
	cases := []struct {
		name     string
		config   string
		wantFlow bool
	}{
		{"tcp keeps flow", `{"flow":"xtls-rprx-vision"}`, true},
		{"ws drops flow", `{"flow":"xtls-rprx-vision","ws-path":"/ws"}`, false},
		{"grpc drops flow", `{"flow":"xtls-rprx-vision","grpc-service-name":"svc"}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := models.Listener{
				Name: "n", Protocol: "vless", Type: "vless", Port: "443",
				BindAddress: "0.0.0.0", Enabled: true, Config: c.config,
			}
			shares, err := ExportShares(l, "example.com", []UserCred{{Username: "u", UUID: uuid}})
			if err != nil {
				t.Fatalf("ExportShares: %v", err)
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

// The server-side config must not be handed an impossible combination either:
// Mihomo rejects a Vision flow on a non-TCP listener outright.
func TestVLESSCompileSkipsStaleFlowForWebSocket(t *testing.T) {
	cfg := map[string]interface{}{
		"flow":    "xtls-rprx-vision",
		"ws-path": "/ws",
	}
	users := []map[string]interface{}{
		{"username": "u", "uuid": "00000000-0000-0000-0000-000000000001"},
	}
	applyListenerFlow(cfg, users)
	if _, present := users[0]["flow"]; present {
		t.Fatalf("flow must not be applied on a ws listener: %#v", users[0])
	}

	// A raw listener still inherits the listener-level default.
	applyListenerFlow(map[string]interface{}{"flow": "xtls-rprx-vision"}, users)
	if users[0]["flow"] != "xtls-rprx-vision" {
		t.Fatalf("raw listener must keep the default flow: %#v", users[0])
	}
}

// The VMess client YAML lost the ws Host header: it built ws-opts with the path
// only, so a listener that pins a Host was unreachable through the exported
// config while the identical vless YAML worked.
func TestVMessYAMLCarriesWebSocketHost(t *testing.T) {
	l := models.Listener{
		Name: "n", Protocol: "vmess", Type: "vmess", Port: "443",
		BindAddress: "0.0.0.0", Enabled: true,
		Config: `{"ws-path":"/ws","ws-headers":{"Host":"cdn.example.com"}}`,
	}
	shares, err := ExportShares(l, "example.com", []UserCred{{Username: "u", UUID: "00000000-0000-0000-0000-000000000001"}})
	if err != nil {
		t.Fatalf("ExportShares: %v", err)
	}
	if len(shares) == 0 {
		t.Fatal("expected a share")
	}
	yaml := shares[0].ClientYAML
	for _, want := range []string{"network: ws", "path: /ws", "Host: cdn.example.com"} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("client YAML missing %q:\n%s", want, yaml)
		}
	}
}
