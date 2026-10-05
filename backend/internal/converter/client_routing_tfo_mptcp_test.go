package converter

import (
	"strings"
	"testing"

	mihomocfg "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gopkg.in/yaml.v3"
)

// TestClientSubscriptionIncludesProxyTFOMPTCP proves that proxy-level
// `tfo` / `mptcp` options are carried into the generated *client*
// subscription document (not just the serving config), so a phone/Clash
// client connecting to a 3m-ui node gets the same TCP Fast Open / MPTCP
// settings. Regression guard for "客户端配置字段映射准确".
func TestClientSubscriptionIncludesProxyTFOMPTCP(t *testing.T) {
	visual := &mihomocfg.VisualConfig{
		Proxies: []mihomocfg.ProxyEntry{
			{
				Name:    "NODE-A",
				Type:    "ss",
				Server:  "example.com",
				Port:    8388,
				Options: map[string]interface{}{"tfo": true, "mptcp": true},
			},
		},
	}
	doc := clientSubscriptionDocument(
		[]map[string]interface{}{},
		[]string{},
		visual,
	)
	raw, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "tfo: true") {
		t.Fatalf("client subscription missing tfo:\n%s", s)
	}
	if !strings.Contains(s, "mptcp: true") {
		t.Fatalf("client subscription missing mptcp:\n%s", s)
	}
}

func TestClientSubscriptionStripsInboundTfo(t *testing.T) {
	visual := &mihomocfg.VisualConfig{
		InboundTfo:   true,
		InboundMPTCP: true,
		Proxies: []mihomocfg.ProxyEntry{
			{
				Name:    "NODE-A",
				Type:    "ss",
				Server:  "example.com",
				Port:    8388,
				Options: map[string]interface{}{"tfo": true, "inbound-tfo": true, "inbound-mptcp": true},
			},
		},
	}
	doc := clientSubscriptionDocument(nil, nil, visual)
	raw, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "inbound-tfo") || strings.Contains(s, "inbound-mptcp") {
		t.Fatalf("server general keys must not appear in client subscription:\n%s", s)
	}
	if !strings.Contains(s, "tfo: true") {
		t.Fatalf("client proxy tfo should remain:\n%s", s)
	}
}
