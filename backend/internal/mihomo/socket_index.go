package mihomo

import (
	"net/netip"
	"syscall"
)

type socketKey struct {
	network string
	address netip.Addr
	port    uint32
}

type socketState uint8

const (
	socketBound socketState = iota + 1
	socketFamilyUnknown
)

type socketIndex map[socketKey]socketState

func endpointKey(network, address string, port uint32) socketKey {
	ip, _ := netip.ParseAddr(address)
	return socketKey{network, ip.Unmap(), port}
}

// Build one index per observation, shared by all listeners and port ranges.
func indexSockets(pid int, connections []runtimeConnection) socketIndex {
	result := make(socketIndex, len(connections))
	for _, conn := range connections {
		if conn.Pid != int32(pid) {
			continue
		}
		network := "tcp"
		switch {
		case conn.Type == syscall.SOCK_STREAM && conn.Status == "LISTEN":
		case conn.Type == syscall.SOCK_DGRAM && conn.Raddr.Port == 0:
			network = "udp"
		default:
			continue
		}
		key := endpointKey(network, conn.Laddr.IP, conn.Laddr.Port)
		if !key.address.IsValid() {
			continue
		}
		result[key] = socketBound
		if key.address != netip.IPv6Unspecified() {
			continue
		}
		key.address = netip.IPv4Unspecified()
		if conn.familyKnown && !conn.ipv6Only {
			result[key] = socketBound
		} else if !conn.familyKnown && result[key] != socketBound {
			result[key] = socketFamilyUnknown
		}
	}
	return result
}
