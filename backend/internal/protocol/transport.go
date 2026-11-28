package protocol

import "strings"

// udpBoundKinds are the listener protocol kinds whose inbound binds a UDP socket
// in Mihomo. Every other kind binds a TCP listening socket. The proxy `udp`
// toggle exposed on the listener (UDP relay over an established connection) is
// unrelated to the listening socket and is intentionally ignored here.
var udpBoundKinds = map[string]struct{}{
	"tuic":       {},
	"tuic-v4":    {},
	"tuic-v5":    {},
	"hysteria":   {},
	"hysteria2":  {},
	"shadowquic": {},
	"mieru":      {},
}

// ListenerTransports reports the L4 transports a listener binds a socket on,
// derived from its protocol kind. It returns ["udp"] for UDP-bound protocols and
// ["tcp"] for everything else. Callers use this so a TCP listener on port 443 and
// a UDP listener on port 443 are treated as independent sockets rather than a
// port conflict — they are separate endpoints at the OS level.
func ListenerTransports(protocol string) []string {
	kind := strings.ToLower(strings.TrimSpace(protocol))
	if _, ok := udpBoundKinds[kind]; ok {
		return []string{"udp"}
	}
	return []string{"tcp"}
}
