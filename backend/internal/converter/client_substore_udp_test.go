package converter

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// Verified against Sub-Store 2.44.31 ClashMeta producer across all 12
// exportable protocols: a proxy without an explicit udp value is emitted with
// udp:true, and TCP-based trojan/vless/vmess carry network:tcp.
func TestClientProxyDefaultsMatchSubStore(t *testing.T) {
	creds := []user.Credential{{Username: "alice", Password: "secret"}}
	cases := []struct {
		protocol, config string
		needCreds        bool
		wantNetwork      string
	}{
		{"shadowsocks", `{"cipher":"aes-256-gcm","password":"secret"}`, false, ""},
		{"vmess", `{"cipher":"auto"}`, true, "tcp"},
		{"vless", `{}`, true, "tcp"},
		{"trojan", `{"password":"secret"}`, true, "tcp"},
		{"hysteria2", `{"password":"secret"}`, true, ""},
		{"tuic", `{"token":"secret"}`, true, ""},
	}

	for _, tc := range cases {
		t.Run(tc.protocol, func(t *testing.T) {
			var c []user.Credential
			if tc.needCreds {
				if tc.protocol == "vmess" || tc.protocol == "vless" {
					c = []user.Credential{{Username: "alice", Password: "secret", UUID: "b831381d-6324-4d53-ad4f-8cda48b30811"}}
				} else {
					c = creds
				}
			}
			proxies, err := listenerToProxies(models.Listener{
				Name: tc.protocol, Protocol: tc.protocol, Port: "443", Config: tc.config,
			}, "example.com", c)
			if err != nil {
				t.Fatalf("%s: %v", tc.protocol, err)
			}
			if len(proxies) == 0 {
				t.Fatal("no proxy")
			}
			p := proxies[0]
			if p["udp"] != true {
				t.Errorf("%s udp = %v, want true", tc.protocol, p["udp"])
			}
			if tc.wantNetwork != "" && p["network"] != tc.wantNetwork {
				t.Errorf("%s network = %v, want %s", tc.protocol, p["network"], tc.wantNetwork)
			}
		})
	}
}

// Documented Sub-Store alignment behaviour: the listener UDP toggle is not a
// per-client explicit udp:false. When it is off, the client proxy still gets
// udp:true, matching Sub-Store's "default udp on" semantics. This is an
// intentional consequence of aligning with Sub-Store (user confirmed).
func TestClientProxyUDPDefaultEvenWhenListenerUDPOff(t *testing.T) {
	proxies, err := listenerToProxies(models.Listener{
		Name: "ss-noudp", Protocol: "shadowsocks", Port: "443", UDP: false,
		Config: `{"cipher":"aes-256-gcm","password":"secret"}`,
	}, "example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) == 0 {
		t.Fatal("no proxy")
	}
	if proxies[0]["udp"] != true {
		t.Errorf("udp = %v, want true (Sub-Store default)", proxies[0]["udp"])
	}
}
