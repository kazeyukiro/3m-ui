package converter

import "testing"

// The client YAML is built from the same listener config, so a stale flow has to
// be dropped here too: copyTransport resolves the transport into `network`, and
// anything other than raw TCP cannot carry Vision.
func TestCopyTransportDropsFlowOnNonTCPNetwork(t *testing.T) {
	cases := []struct {
		name     string
		src      map[string]interface{}
		wantNet  string
		wantFlow bool
	}{
		{"tcp keeps flow", map[string]interface{}{}, "", true},
		{"ws drops flow", map[string]interface{}{"ws-path": "/ws"}, "ws", false},
		{"grpc drops flow", map[string]interface{}{"grpc-service-name": "svc"}, "grpc", false},
		{"xhttp drops flow", map[string]interface{}{"xhttp-config": map[string]interface{}{"path": "/x"}}, "xhttp", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dst := map[string]interface{}{"flow": "xtls-rprx-vision"}
			copyTransport(dst, c.src)

			if got := dst["network"]; got != c.wantNet && !(c.wantNet == "" && got == nil) {
				t.Fatalf("network = %v, want %q", got, c.wantNet)
			}
			_, present := dst["flow"]
			if present != c.wantFlow {
				t.Fatalf("flow present = %v, want %v: %#v", present, c.wantFlow, dst)
			}
		})
	}
}
