package config

import (
	"errors"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/system"
)

func TestOutboundCheckProfilePreservesDialerChain(t *testing.T) {
	db, request := outboundTestAPI(t)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"upstream","type":"socks5","server":"upstream.example","port":1080},"enabled":true}`, 201)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"edge","type":"vless","server":"edge.example","port":443,"uuid":"test-uuid","tls":true,"dialer-proxy":"upstream"},"enabled":true}`, 201)
	request("POST", "/config/server-outbounds", `{"proxy":{"name":"unrelated","type":"socks5","server":"other.example","port":1080},"enabled":true}`, 201)
	profile, err := ServerOutboundCheckProfile(db, "edge")
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Enabled || len(profile.Proxies) != 2 || len(profile.Groups) != 0 {
		t.Fatalf("wrong isolated profile: %+v", profile)
	}
	edge := profile.Proxies[0]
	if edge["name"] != "edge" || edge["dialer-proxy"] != "upstream" || edge["tls"] != true || edge["uuid"] != "test-uuid" || profile.Proxies[1]["name"] != "upstream" {
		t.Fatalf("test route or protocol settings changed: %+v", profile)
	}
}

func TestOutboundCheckProfileIncludesGroupsAndManagedWARP(t *testing.T) {
	db, _ := outboundTestAPI(t)
	if err := system.SaveWARPAccount(db, &system.WARPAccount{PrivateKey: "server-only-key", EndpointHost: "warp.example", EndpointPort: 2408}); err != nil {
		t.Fatal(err)
	}
	if err := SaveServerRouting(db, ServerRoutingConfig{
		Proxies: []ProxyEntry{{Name: "edge", Type: "socks5", Server: "edge.example", Port: 1080, Options: map[string]any{"dialer-proxy": "egress"}}},
		Groups:  []GroupEntry{{Name: "egress", Type: "select", Proxies: []string{"WARP"}}},
	}); err != nil {
		t.Fatal(err)
	}
	profile, err := ServerOutboundCheckProfile(db, "edge")
	if err != nil || len(profile.Proxies) != 2 || len(profile.Groups) != 1 || profile.Groups[0]["name"] != "egress" || profile.Proxies[1]["private-key"] != "server-only-key" || profile.Proxies[1]["dialer-proxy"] != "DIRECT" {
		t.Fatalf("missing group or managed WARP route: %+v, %v", profile, err)
	}
	profile, err = ServerOutboundCheckProfile(db, "WARP")
	if err != nil || !profile.Enabled || len(profile.Proxies) != 1 || profile.Proxies[0]["type"] != "wireguard" {
		t.Fatalf("managed WARP cannot be tested: %+v, %v", profile, err)
	}
}

func TestOutboundCheckProfileRejectsUnavailableDependencies(t *testing.T) {
	for _, tc := range []struct {
		name        string
		dialer      string
		disabled    []string
		wantEnabled bool
		wantError   error
	}{
		{name: "disabled", disabled: []string{"edge"}},
		{name: "missing", dialer: "missing", wantError: ErrInvalidOutbound},
		{name: "disabled dependency", dialer: "upstream", disabled: []string{"upstream"}, wantError: ErrInvalidOutbound},
		{name: "cycle", dialer: "edge", wantError: ErrInvalidOutbound},
		{name: "direct dialer", dialer: "DIRECT", wantEnabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := outboundTestAPI(t)
			if err := SaveServerRouting(db, ServerRoutingConfig{
				Proxies: []ProxyEntry{
					{Name: "edge", Type: "socks5", Server: "edge.example", Port: 1080, Options: map[string]any{"dialer-proxy": tc.dialer}},
					{Name: "upstream", Type: "socks5", Server: "upstream.example", Port: 1080},
				},
				DisabledOutboundNames: tc.disabled,
			}); err != nil {
				t.Fatal(err)
			}
			profile, err := ServerOutboundCheckProfile(db, "edge")
			if !errors.Is(err, tc.wantError) || profile.Enabled != tc.wantEnabled {
				t.Fatalf("enabled=%v error=%v", profile.Enabled, err)
			}
			if _, err := ServerOutboundCheckProfile(db, "absent"); !errors.Is(err, ErrOutboundNotFound) {
				t.Fatalf("missing outbound error: %v", err)
			}
		})
	}
}
