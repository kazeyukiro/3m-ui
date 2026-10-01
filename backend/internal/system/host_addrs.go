package system

import (
	"net"
	"sort"
)

// HostAddresses returns non-loopback, non-link-local unicast IPs currently
// assigned to the host (IPv4 first, then IPv6). Used by the dashboard IP strip.
func HostAddresses() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var v4, v6 []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			s := ip.String()
			if s == "" {
				continue
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			if ip.To4() != nil {
				v4 = append(v4, s)
			} else {
				v6 = append(v6, s)
			}
		}
	}
	sort.Strings(v4)
	sort.Strings(v6)
	out := make([]string, 0, len(v4)+len(v6))
	out = append(out, v4...)
	out = append(out, v6...)
	return out
}
