package listener

import (
	"net"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
)

func portRanges(value string) ([][2]int, bool) {
	var out [][2]int
	for _, raw := range strings.Split(value, ",") {
		part := strings.TrimSpace(raw)
		if part == "" {
			return nil, false
		}
		bounds := strings.Split(part, "-")
		if len(bounds) > 2 {
			return nil, false
		}
		start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
		if err != nil || start < 1 || start > 65535 {
			return nil, false
		}
		end := start
		if len(bounds) == 2 {
			end, err = strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil || end < start || end > 65535 {
				return nil, false
			}
		}
		out = append(out, [2]int{start, end})
	}
	return out, true
}

func portsOverlap(a, b string) bool {
	ra, oka := portRanges(a)
	rb, okb := portRanges(b)
	if !oka || !okb {
		return false
	}
	for _, x := range ra {
		for _, y := range rb {
			if x[0] <= y[1] && y[0] <= x[1] {
				return true
			}
		}
	}
	return false
}

func firstListenerAddress(l models.Listener) string {
	if strings.TrimSpace(l.Listen) != "" {
		return strings.TrimSpace(l.Listen)
	}
	if strings.TrimSpace(l.BindAddress) != "" {
		return strings.TrimSpace(l.BindAddress)
	}
	return "0.0.0.0"
}

func listenerAddressesConflict(a, b string) bool {
	a = strings.Trim(strings.TrimSpace(a), "[]")
	b = strings.Trim(strings.TrimSpace(b), "[]")
	if a == "" {
		a = "0.0.0.0"
	}
	if b == "" {
		b = "0.0.0.0"
	}
	if a == "0.0.0.0" || a == "::" || b == "0.0.0.0" || b == "::" {
		return true
	}
	ia := net.ParseIP(a)
	ib := net.ParseIP(b)
	if ia == nil || ib == nil {
		return strings.EqualFold(a, b)
	}
	return ia.Equal(ib)
}

// listenersShareTransport reports whether two listeners bind a common L4 transport
// on their listening socket (per protocol.ListenerTransports). A TCP listener and a
// UDP listener on the same address:port are independent sockets and must not be
// reported as a conflict, so they return false here.
func listenersShareTransport(a, b models.Listener) bool {
	for _, x := range protocol.ListenerTransports(a.Protocol) {
		for _, y := range protocol.ListenerTransports(b.Protocol) {
			if x == y {
				return true
			}
		}
	}
	return false
}
