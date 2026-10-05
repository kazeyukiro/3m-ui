package listener

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestListenersShareTransport(t *testing.T) {
	cases := []struct {
		name string
		a, b models.Listener
		want bool
	}{
		{"tcp+tcp", models.Listener{Protocol: "vless"}, models.Listener{Protocol: "trojan"}, true},
		{"udp+udp", models.Listener{Protocol: "tuic"}, models.Listener{Protocol: "hysteria2"}, true},
		{"tcp+udp", models.Listener{Protocol: "vless"}, models.Listener{Protocol: "tuic"}, false},
		{"udp+tcp", models.Listener{Protocol: "mieru"}, models.Listener{Protocol: "shadowsocks"}, false},
		{"vless+tuic", models.Listener{Protocol: "vless"}, models.Listener{Protocol: "tuic-v4"}, false},
	}
	for _, c := range cases {
		if got := listenersShareTransport(c.a, c.b); got != c.want {
			t.Fatalf("%s: listenersShareTransport = %v, want %v", c.name, got, c.want)
		}
	}
}
