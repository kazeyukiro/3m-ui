package config

// MihomoListenerProtocols is the set of listener protocols exposed by the
// unified 3m-ui Node page. Local proxy endpoints (socks/http/tproxy/redir/
// mixed), Tunnel, TUN and Hysteria2 Realm are intentionally excluded because
// they do not represent a directly distributable client proxy node.
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
}

func IsMihomoListenerProtocol(protocol string) bool {
	for _, p := range MihomoListenerProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}
