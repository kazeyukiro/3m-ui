package config_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"gopkg.in/yaml.v3"
)

func TestVisualDNSRoundTripAndGeneration(t *testing.T) {
	const input = `{
		"enable": true, "listen": "127.0.0.1:5353", "enhancedMode": "fake-ip",
		"ipv6": false, "cacheAlgorithm": "arc", "preferH3": false,
		"useHosts": false, "useSystemHosts": false, "respectRules": true,
		"defaultNameserver": ["223.5.5.5"],
		"nameserver": ["https://dns.example/dns-query#DIRECT&ecs=1.1.1.0/24"],
		"fallback": ["tls://1.1.1.1"], "fallbackLazyQuery": false,
		"proxyServerNameserver": ["223.5.5.5"],
		"proxyServerNameserverPolicy": {"node.example": "223.5.5.5"},
		"directNameserver": ["system"], "directNameserverFollowPolicy": false,
		"nameserverPolicy": {"+.lan": "system", "rule-set:cn": ["223.5.5.5", "tls://1.1.1.1"]},
		"fakeIPRange": "198.18.0.1/16", "fakeIPRange6": "fdfe:dcba:9876::1/64",
		"fakeIPFilterMode": "rule", "fakeIPFilter": ["DOMAIN,example.com,real-ip", "MATCH,fake-ip"],
		"fakeIPTTL": 1,
		"fallbackFilter": {"geoip": false, "geoipCode": "CN", "geosite": ["gfw"], "ipcidr": ["240.0.0.0/4"], "domain": ["+.example.com"]}
	}`
	vc := mihomoConfig.DefaultVisualConfig()
	if err := json.Unmarshal([]byte(input), &vc.DNS); err != nil {
		t.Fatal(err)
	}
	db := lowMemoryTestDB(t)
	if err := mihomoConfig.SaveVisualConfig(db, vc); err != nil {
		t.Fatal(err)
	}
	got, err := mihomoConfig.GetVisualConfig(db)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got.DNS)
	if err != nil {
		t.Fatal(err)
	}
	var wantJSON, gotJSON map[string]interface{}
	_ = json.Unmarshal([]byte(input), &wantJSON)
	_ = json.Unmarshal(encoded, &gotJSON)
	if !reflect.DeepEqual(wantJSON, gotJSON) {
		t.Fatalf("DNS settings changed after save/load:\nwant %s\ngot  %s", input, encoded)
	}
	out, err := mihomoConfig.NewConfigEngine(db).GenerateFinalConfig()
	if err != nil {
		t.Fatal(err)
	}
	var final map[string]interface{}
	if err := yaml.Unmarshal([]byte(out), &final); err != nil {
		t.Fatal(err)
	}
	const expected = `
enable: true
listen: 127.0.0.1:5353
enhanced-mode: fake-ip
ipv6: false
cache-algorithm: arc
prefer-h3: false
use-hosts: false
use-system-hosts: false
respect-rules: true
default-nameserver: [223.5.5.5]
nameserver: ["https://dns.example/dns-query#DIRECT&ecs=1.1.1.0/24"]
fallback: ["tls://1.1.1.1"]
fallback-lazy-query: false
proxy-server-nameserver: [223.5.5.5]
proxy-server-nameserver-policy: {node.example: 223.5.5.5}
direct-nameserver: [system]
direct-nameserver-follow-policy: false
nameserver-policy:
  '+.lan': system
  'rule-set:cn': [223.5.5.5, 'tls://1.1.1.1']
fake-ip-range: 198.18.0.1/16
fake-ip-range6: fdfe:dcba:9876::1/64
fake-ip-filter-mode: rule
fake-ip-filter: ['DOMAIN,example.com,real-ip', 'MATCH,fake-ip']
fake-ip-ttl: 1
fallback-filter:
  geoip: false
  geoip-code: CN
  geosite: [gfw]
  ipcidr: [240.0.0.0/4]
  domain: ['+.example.com']
`
	var wantYAML map[string]interface{}
	if err := yaml.Unmarshal([]byte(expected), &wantYAML); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wantYAML, final["dns"]) {
		t.Fatalf("generated DNS differs from the core configuration:\n%s", out)
	}
}

func TestVisualDNSClearedFieldsStayUnset(t *testing.T) {
	db := lowMemoryTestDB(t)
	vc := mihomoConfig.DefaultVisualConfig()
	vc.DNS = mihomoConfig.VisualDNS{Enable: false}
	if err := mihomoConfig.SaveVisualConfig(db, vc); err != nil {
		t.Fatal(err)
	}
	got, err := mihomoConfig.GetVisualConfig(db)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got.DNS)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"enable":false}` {
		t.Fatalf("cleared DNS fields were replaced by defaults: %s", encoded)
	}
}

func TestUnsavedDNSMatchesCoreAndExistingDNSEnabledValueSurvives(t *testing.T) {
	db := lowMemoryTestDB(t)
	vc, err := mihomoConfig.GetVisualConfig(db)
	if err != nil {
		t.Fatal(err)
	}
	if vc.DNS.Enable {
		t.Fatal("an unconfigured DNS form must match the core's disabled DNS")
	}
	vc.DNS.Enable = true
	if err := mihomoConfig.SaveVisualConfig(db, vc); err != nil {
		t.Fatal(err)
	}
	got, err := mihomoConfig.GetVisualConfig(db)
	if err != nil {
		t.Fatal(err)
	}
	if !got.DNS.Enable {
		t.Fatal("existing enabled DNS must remain enabled")
	}
}

func TestVisualDNSRejectsInvalidSettingsBeforeSaving(t *testing.T) {
	db := lowMemoryTestDB(t)
	for _, tc := range []struct{ input, field string }{
		{`{"enhancedMode":"invalid"}`, "enhanced-mode"},
		{`{"cacheAlgorithm":"invalid"}`, "cache-algorithm"},
		{`{"fakeIPFilterMode":"invalid"}`, "fake-ip-filter-mode"},
		{`{"listen":"127.0.0.1:99999"}`, "listen"},
		{`{"fakeIPRange":"::1/64"}`, "fake-ip-range"},
		{`{"fakeIPRange6":"198.18.0.1/16"}`, "fake-ip-range6"},
		{`{"nameserverPolicy":{"example.com":true}}`, "nameserver-policy"},
		{`{"proxyServerNameserverPolicy":{"example.com":[1]}}`, "proxy-server-nameserver-policy"},
		{`{"enable":true,"respectRules":true}`, "proxy-server-nameserver"},
		{`{"fallbackFilter":{"ipcidr":["invalid"]}}`, "fallback-filter.ipcidr"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			vc := mihomoConfig.DefaultVisualConfig()
			if err := json.Unmarshal([]byte(tc.input), &vc.DNS); err != nil {
				t.Fatal(err)
			}
			if err := mihomoConfig.SaveVisualConfig(db, vc); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("expected %s validation error, got %v", tc.field, err)
			}
		})
	}
}

func TestPanelDNSPrecedenceAndWARP(t *testing.T) {
	for _, customFirst := range []bool{false, true} {
		t.Run(fmt.Sprint("custom-first=", customFirst), func(t *testing.T) {
			db := lowMemoryTestDB(t)
			custom := models.Config{Name: "custom-dns", Type: "custom", Enabled: true, Content: "dns:\n  enable: true\n  listen: 127.0.0.1:9999\n"}
			if customFirst {
				if err := db.Create(&custom).Error; err != nil {
					t.Fatal(err)
				}
			}
			vc := mihomoConfig.DefaultVisualConfig()
			vc.DNS.Enable, vc.DNS.Listen = false, "127.0.0.1:5353"
			if err := mihomoConfig.SaveVisualConfig(db, vc); err != nil {
				t.Fatal(err)
			}
			if !customFirst {
				if err := db.Create(&custom).Error; err != nil {
					t.Fatal(err)
				}
			}
			check := func(wantEnabled bool) {
				t.Helper()
				out, err := mihomoConfig.NewConfigEngine(db).GenerateFinalConfig()
				if err != nil {
					t.Fatal(err)
				}
				var final map[string]interface{}
				if err := yaml.Unmarshal([]byte(out), &final); err != nil {
					t.Fatal(err)
				}
				dns := final["dns"].(map[string]interface{})
				if dns["enable"] != wantEnabled || dns["listen"] != "127.0.0.1:5353" {
					t.Fatalf("expected panel DNS (enabled=%v), got %#v", wantEnabled, dns)
				}
			}
			check(false)
			if err := system.SaveWARPAccount(db, &system.WARPAccount{PrivateKey: "server-only-key", EndpointHost: "warp.example", EndpointPort: 2408}); err != nil {
				t.Fatal(err)
			}
			sr := mihomoConfig.DefaultServerRouting()
			sr.WarpGlobal = true
			if err := mihomoConfig.SaveServerRouting(db, sr); err != nil {
				t.Fatal(err)
			}
			check(true)
		})
	}
}
