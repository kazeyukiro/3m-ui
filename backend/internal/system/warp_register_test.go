package system

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWARPTemplate_MUIShape(t *testing.T) {
	yaml, err := WARPTemplate(
		"CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		"engage.cloudflareclient.com",
		2408,
		"172.16.0.2",
		"2606:4700:110:8216:dac5:4a83:49de:6997",
		[]int{171, 91, 109},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"type: wireguard",
		"ip: 172.16.0.2",
		"private-key: CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"public-key: bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		"server: engage.cloudflareclient.com",
		"port: 2408",
		"mtu: 1420",
		"reserved:",
	} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("missing %q in:\n%s", want, yaml)
		}
	}
}

func TestRegisterWARP_WithMockServer(t *testing.T) {
	device := map[string]any{
		"id":    "f0844485-ee81-4c51-a022-a86549ebb070",
		"token": "test-token",
		"name":  "3m-ui",
		"model": "3m-ui",
		"account": map[string]any{
			"license":      "aaaaaaaa-bbbbbbbb-cccccccc",
			"account_type": "free",
		},
		"config": map[string]any{
			"client_id": "qVtt",
			"interface": map[string]any{
				"addresses": map[string]any{
					"v4": "172.16.0.2",
					"v6": "2606:4700:110::1",
				},
			},
			"peers": []map[string]any{
				{
					"public_key": "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
					"endpoint": map[string]any{
						"host": "engage.cloudflareclient.com:2408",
						"v4":   "162.159.192.1:2408",
					},
				},
			},
		},
	}
	body, _ := json.Marshal(device)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/reg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	client := &warpHTTPClient{baseURL: srv.URL, httpClient: srv.Client()}
	res, err := registerWARPWithClient(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if res.Address != "172.16.0.2" {
		t.Fatalf("address=%q", res.Address)
	}
	if len(res.Reserved) != 3 {
		t.Fatalf("reserved=%v", res.Reserved)
	}
	if !strings.Contains(res.YAML, "type: wireguard") {
		t.Fatalf("yaml:\n%s", res.YAML)
	}
	if !strings.Contains(res.YAML, "public-key: bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=") {
		t.Fatalf("peer key missing:\n%s", res.YAML)
	}
	if res.Server != "engage.cloudflareclient.com" || res.Port != 2408 {
		t.Fatalf("endpoint %s:%d", res.Server, res.Port)
	}
}

func TestRegisterWARP_RejectsBadEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"nope"}]}`))
	}))
	defer srv.Close()
	client := &warpHTTPClient{baseURL: srv.URL, httpClient: srv.Client()}
	if _, err := registerWARPWithClient(context.Background(), client); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeWarpKey(t *testing.T) {
	if _, err := decodeWarpKey("CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo="); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeWarpKey("bad"); err == nil {
		t.Fatal("expected error")
	}
}
