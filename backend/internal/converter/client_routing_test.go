package converter

import (
	"strings"
	"testing"

	mihomocfg "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gopkg.in/yaml.v3"
)

func TestClientSubscriptionDocumentSplitRules(t *testing.T) {
	doc := clientSubscriptionDocument(
		[]map[string]interface{}{{"name": "n1", "type": "ss"}},
		[]string{"n1"},
		nil,
	)
	raw, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"GEOIP,CN,DIRECT",
		"GEOSITE,cn,DIRECT",
		"MATCH,PROXY",
		"name: PROXY",
		"name: AUTO",
		"type: url-test",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}

func TestClientSubscriptionUsesVisualGroups(t *testing.T) {
	v := &mihomocfg.VisualConfig{
		Groups: []mihomocfg.GroupEntry{
			{Name: "香港", Type: "select", Proxies: []string{"DIRECT"}},
			{Name: "代理", Type: "select", Proxies: []string{"香港", "DIRECT"}},
			{Name: "AI", Type: "select", Proxies: []string{"香港", "DIRECT"}},
		},
		Rules: []string{
			"GEOSITE,cn,DIRECT",
			"GEOSITE,openai,AI",
			"MATCH,代理",
		},
	}
	doc := clientSubscriptionDocument(
		[]map[string]interface{}{{"name": "node-a", "type": "vless"}},
		[]string{"node-a"},
		v,
	)
	raw, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"name: 香港",
		"name: 代理",
		"name: AI",
		"node-a",
		"MATCH,代理",
		"GEOSITE,openai,AI",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}
