package config

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

func TestServerOutboundAPIPersistsProtocolOptions(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	r := gin.New()
	RegisterServerOutboundRoutes(r.Group("/config"), db)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	w := request("POST", "/config/server-outbounds", `{"proxy":{"name":"edge","type":"vless","server":"example.com","port":443,"uuid":"11111111-1111-4111-8111-111111111111","tls":true,"ws-opts":{"path":"/proxy","headers":{"Host":"cdn.example.com"}}},"enabled":true}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	w = request("GET", "/config/server-outbounds", "")
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var entries []struct {
		Proxy   ProxyEntry `json:"proxy"`
		Enabled bool       `json:"enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Proxy.Name != "edge" || !entries[0].Enabled {
		t.Fatalf("entries: %#v", entries)
	}
	ws := entries[0].Proxy.Options["ws-opts"].(map[string]interface{})
	if ws["path"] != "/proxy" {
		t.Fatalf("lost websocket options: %#v", ws)
	}
}

func TestServerOutboundProtocolFixtures(t *testing.T) {
	db, request := outboundTestAPI(t)
	raw, err := os.ReadFile("testdata/server_outbounds.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Protocol string     `json:"protocol"`
		Proxy    ProxyEntry `json:"proxy"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		body, _ := json.Marshal(ServerOutbound{Proxy: fixture.Proxy, Enabled: true})
		request("POST", "/config/server-outbounds", string(body), 201)
	}
	cfg := generatedOutboundConfig(t, db)
	proxies := cfg["proxies"].([]interface{})
	if len(proxies) != len(fixtures) {
		t.Fatalf("generated %d of %d protocols", len(proxies), len(fixtures))
	}
	for i, fixture := range fixtures {
		got, _ := json.Marshal(proxies[i])
		want, _ := json.Marshal(fixture.Proxy)
		var gotMap, wantMap map[string]interface{}
		json.Unmarshal(got, &gotMap)
		json.Unmarshal(want, &wantMap)
		// Peer-only WireGuard has no root endpoint; its zero-value root fields
		// are ignored by Mihomo, while the peers carry the actual endpoints.
		if fixture.Proxy.Server == "" {
			delete(gotMap, "server")
			delete(wantMap, "server")
			delete(gotMap, "port")
			delete(wantMap, "port")
		}
		got, _ = json.Marshal(gotMap)
		want, _ = json.Marshal(wantMap)
		if string(got) != string(want) {
			t.Fatalf("%s fields changed: got %s, want %s", fixture.Protocol, got, want)
		}
	}
	if binary := os.Getenv("MIHOMO_TEST_BINARY"); binary != "" {
		for i, fixture := range fixtures {
			t.Run(fixture.Proxy.Name, func(t *testing.T) {
				cfg["proxies"] = []interface{}{proxies[i]}
				yamlConfig, err := yaml.Marshal(cfg)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				path := filepath.Join(dir, "config.yaml")
				if err := os.WriteFile(path, yamlConfig, 0600); err != nil {
					t.Fatal(err)
				}
				if output, err := exec.Command(binary, "-t", "-d", dir, "-f", path).CombinedOutput(); err != nil {
					t.Fatalf("Mihomo rejects %s: %v\n%s", fixture.Protocol, err, output)
				}
			})
		}
	}
}

func outboundTestAPI(t *testing.T) (*gorm.DB, func(string, string, string, int) []byte) {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	r := gin.New()
	RegisterServerOutboundRoutes(r.Group("/config"), db)
	return db, func(method, path, body string, status int) []byte {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("%s %s: want %d, got %d: %s", method, path, status, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
}

func generatedOutboundConfig(t *testing.T, db *gorm.DB) map[string]interface{} {
	t.Helper()
	raw, err := NewConfigEngine(db).GenerateFinalConfig()
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "disabledOutboundNames") || strings.Contains(raw, "managed:") {
		t.Fatalf("panel metadata leaked: %s", raw)
	}
	return cfg
}

func TestServerOutboundRenameUpdatesTargetsWithoutChangingPayloads(t *testing.T) {
	db, request := outboundTestAPI(t)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"edge","type":"http","server":"example.com","port":443},"enabled":true}`, 201)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"chain","type":"socks5","server":"example.com","port":1080,"dialer-proxy":"edge"},"enabled":true}`, 201)
	request("PATCH", "/config/server-routing", `{"rules":["DOMAIN,edge,edge","AND,((DOMAIN,edge),(DST-PORT,443)),edge","IP-CIDR,10.0.0.0/8,edge,no-resolve","MATCH,DIRECT"],"proxyGroups":[{"name":"select","type":"select","proxies":["edge","DIRECT"]}],"ruleProviders":[{"name":"rules","type":"http","behavior":"domain","url":"https://example.com/rules.yaml","proxy":"edge"}]}`, 200)
	request("PUT", "/config/server-outbounds?name=edge", `{"proxy":{"name":"renamed","type":"http","server":"example.com","port":443},"enabled":true}`, 200)
	cfg := generatedOutboundConfig(t, db)
	rules := cfg["rules"].([]interface{})
	for i, want := range []string{"DOMAIN,edge,renamed", "AND,((DOMAIN,edge),(DST-PORT,443)),renamed", "IP-CIDR,10.0.0.0/8,renamed,no-resolve"} {
		if rules[i] != want {
			t.Fatalf("rule %d: %v", i, rules[i])
		}
	}
	group := cfg["proxy-groups"].([]interface{})[0].(map[string]interface{})
	if group["proxies"].([]interface{})[0] != "renamed" {
		t.Fatal("group target not renamed")
	}
	provider := cfg["rule-providers"].(map[string]interface{})["rules"].(map[string]interface{})
	if provider["proxy"] != "renamed" {
		t.Fatal("rule-provider target not renamed")
	}
	chain := cfg["proxies"].([]interface{})[1].(map[string]interface{})
	if chain["dialer-proxy"] != "renamed" {
		t.Fatal("dialer target not renamed")
	}
}

func TestServerOutboundReferencedMutationsAndBatchAreAtomic(t *testing.T) {
	_, request := outboundTestAPI(t)
	for _, name := range []string{"first", "used"} {
		request("POST", "/config/server-outbounds", `{"proxy":{"name":"`+name+`","type":"http","server":"example.com","port":443},"enabled":true}`, 201)
	}
	request("PATCH", "/config/server-routing", `{"rules":["MATCH,used"]}`, 200)
	request("DELETE", "/config/server-outbounds?name=used", "", 409)
	request("POST", "/config/server-outbounds/enabled", `{"names":["first","used"],"enabled":false}`, 409)
	var entries []ServerOutbound
	json.Unmarshal(request("GET", "/config/server-outbounds", "", 200), &entries)
	if len(entries) != 2 || !entries[0].Enabled || !entries[1].Enabled {
		t.Fatalf("partial mutation: %#v", entries)
	}
}

func TestServerRoutingSavePreservesOutboundsAndDeletionClearsConfig(t *testing.T) {
	db, request := outboundTestAPI(t)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"edge","type":"http","server":"example.com","port":443},"enabled":true}`, 201)
	request("POST", "/config/server-outbounds/enabled", `{"names":["edge"],"enabled":false}`, 200)
	request("PATCH", "/config/server-routing", `{"rules":["MATCH,DIRECT"],"warpDomains":[],"warpGlobal":false,"ruleProviders":[]}`, 200)
	var entries []ServerOutbound
	json.Unmarshal(request("GET", "/config/server-outbounds", "", 200), &entries)
	if len(entries) != 1 || entries[0].Enabled {
		t.Fatalf("routing save lost definition/state: %#v", entries)
	}
	if proxies := generatedOutboundConfig(t, db)["proxies"].([]interface{}); len(proxies) != 0 {
		t.Fatalf("disabled proxy generated: %#v", proxies)
	}
	request("PATCH", "/config/server-routing", `{"rules":["MATCH,edge"]}`, 409)
	request("POST", "/config/server-outbounds/enabled", `{"names":["edge"],"enabled":true}`, 200)
	if proxies := generatedOutboundConfig(t, db)["proxies"].([]interface{}); len(proxies) != 1 {
		t.Fatal("enabled proxy missing")
	}
	request("DELETE", "/config/server-outbounds?name=edge", "", 200)
	if proxies := generatedOutboundConfig(t, db)["proxies"].([]interface{}); len(proxies) != 0 {
		t.Fatal("deleted proxy remains in config")
	}
}

func TestWARPIsManagedRedactedAndCannotBeDeletedWhileReferenced(t *testing.T) {
	db, request := outboundTestAPI(t)
	legacy := DefaultServerRouting()
	legacy.Proxies = []ProxyEntry{{Name: "WARP", Type: "http", Server: "example.com", Port: 443}, {Name: "WARP", Type: "http", Server: "example.org", Port: 443}}
	if err := SaveServerRouting(db, legacy); err != nil {
		t.Fatal(err)
	}
	acc := &system.WARPAccount{PrivateKey: "eCtXsJZ27+4PbhDkHnB923tkUn2Gj59wZw5wFA75MnU=", PeerPublicKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=", EndpointHost: "engage.cloudflareclient.com", EndpointPort: 2408, AddressV4: "172.16.0.2", AccessToken: "account-secret"}
	if err := system.SaveWARPAccount(db, acc); err != nil {
		t.Fatal(err)
	}
	body := request("GET", "/config/server-outbounds", "", 200)
	if bytes.Contains(body, []byte(acc.PrivateKey)) || bytes.Contains(body, []byte(acc.AccessToken)) {
		t.Fatal("WARP list leaks secrets")
	}
	var entries []ServerOutbound
	json.Unmarshal(body, &entries)
	if len(entries) != 1 || !entries[0].Managed || entries[0].Proxy.Name != "WARP" {
		t.Fatalf("WARP not managed: %s", body)
	}
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"WARP","type":"http","server":"s","port":443},"enabled":true}`, 400)
	request("DELETE", "/config/server-outbounds?name=WARP", "", 400)
	request("PATCH", "/config/server-routing", `{"warpGlobal":true}`, 200)
	if err := DeleteServerWARP(db); err == nil {
		t.Fatal("deleted WARP with global routing enabled")
	}
	if proxies := generatedOutboundConfig(t, db)["proxies"].([]interface{}); len(proxies) != 1 {
		t.Fatal("WARP missing or duplicated")
	}
	request("PATCH", "/config/server-routing", `{"warpGlobal":false,"rules":["MATCH,DIRECT"]}`, 200)
	if err := DeleteServerWARP(db); err != nil {
		t.Fatal(err)
	}
	if proxies := generatedOutboundConfig(t, db)["proxies"].([]interface{}); len(proxies) != 0 {
		t.Fatal("deleted WARP remains")
	}
}

func TestServerOutboundRejectsInvalidDefinitions(t *testing.T) {
	_, request := outboundTestAPI(t)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"edge","type":"http","server":"s","port":443}}`, 400)
	request("POST", "/config/server-outbounds/enabled", `{"names":["edge"]}`, 400)
	for _, proxy := range []string{
		`{"name":"DIRECT","type":"http","server":"s","port":443}`,
		`{"name":"bad,name","type":"http","server":"s","port":443}`,
		`{"name":"edge","type":"http","server":"s","port":0}`,
		`{"name":"edge","type":"http","server":"s","port":65536}`,
		`{"name":"edge","type":"http","server":"s","port":443.5}`,
		`{"name":"edge","type":"http","server":"https://example.com","port":443}`,
		`{"name":"edge","type":"vless","server":"s","port":443}`,
		`{"name":"edge","type":"tuic","server":"s","port":443,"token":""}`,
		`{"name":"edge","type":"unknown","server":"s","port":443}`,
		`{"name":"edge","type":"http","server":"s","port":443,"enabled":true}`,
	} {
		request("POST", "/config/server-outbounds", `{"proxy":`+proxy+`,"enabled":true}`, 400)
	}
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"edge","type":"http","server":"s","port":443},"enabled":true}`, 201)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"edge","type":"http","server":"s","port":443},"enabled":true}`, 400)
	request("PATCH", "/config/server-routing", `{"proxyGroups":[{"name":"edge","type":"select","proxies":["DIRECT"]}]}`, 400)
}

func TestExistingManualWARPCanBeRenamed(t *testing.T) {
	db, request := outboundTestAPI(t)
	legacy := DefaultServerRouting()
	legacy.Proxies = []ProxyEntry{{Name: "WARP", Type: "http", Server: "example.com", Port: 443}}
	legacy.Rules = []string{"MATCH,WARP"}
	if err := SaveServerRouting(db, legacy); err != nil {
		t.Fatal(err)
	}
	request("PUT", "/config/server-outbounds?name=WARP", `{"proxy":{"name":"manual","type":"http","server":"example.com","port":443},"enabled":true}`, 200)
	if rules := generatedOutboundConfig(t, db)["rules"].([]interface{}); rules[0] != "MATCH,manual" {
		t.Fatalf("manual WARP rename lost references: %v", rules)
	}
}

func TestManualProxyFragmentProducesActionableOwnershipError(t *testing.T) {
	db, _ := outboundTestAPI(t)
	if err := db.Create(&models.Config{Name: "manual-egress", Type: "custom", Enabled: true, Content: "proxies:\n  - name: old-edge\n    type: http\n    server: example.com\n    port: 443\n"}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := NewConfigEngine(db).GenerateFinalConfig()
	if err == nil || !strings.Contains(err.Error(), "manual-egress") || !strings.Contains(err.Error(), "出站管理") {
		t.Fatalf("expected actionable fragment conflict, got %v", err)
	}
}
