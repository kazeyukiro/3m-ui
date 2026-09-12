package converter

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClientSubscriptionDocumentSplitRules(t *testing.T) {
	doc := clientSubscriptionDocument(
		[]map[string]interface{}{{"name": "n1", "type": "ss"}},
		[]string{"n1"},
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
