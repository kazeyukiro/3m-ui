package config

// MihomoListenerProtocols is the set of listener protocols exposed by the
// unified 3m-ui Node page. Tunnel, TUN, tproxy/redir/mixed and similar
// local-capture-only endpoints stay excluded. HTTP and SOCKS are included as
// standard Mihomo listeners (wiki: inbound/listeners/http|socks).
var MihomoListenerProtocols = []string{
	"shadowsocks",
	"snell",
	"vmess",
	"vless",
	"trojan",
	"hysteria2",
	"tuic",
	"tuic-v4",
	"tuic-v5",
	"shadowquic",
	"anytls",
	"mieru",
	"sudoku",
	"trusttunnel",
	"http",
	"socks",
}

func IsMihomoListenerProtocol(protocol string) bool {
	for _, p := range MihomoListenerProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}
