//go:build linux

package mihomo

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// INET_DIAG_SKV6ONLY is returned by Linux for IPv6 sockets. Reading it avoids
// guessing dual-stack behavior from the system default or from a TCP dial that
// might reach a different process's IPv4 socket. The map is keyed by inode.
func ipv6OnlySockets(ctx context.Context, protocol byte) (map[uint32]bool, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	request := make([]byte, 72) // nlmsghdr + inet_diag_req_v2
	native := binary.NativeEndian
	native.PutUint32(request[0:4], uint32(len(request)))
	native.PutUint16(request[4:6], 20) // SOCK_DIAG_BY_FAMILY
	native.PutUint16(request[6:8], unix.NLM_F_REQUEST|unix.NLM_F_DUMP)
	native.PutUint32(request[8:12], 1)
	request[16], request[17] = unix.AF_INET6, protocol
	state := uint32(1 << 10) // TCP_LISTEN
	if protocol == unix.IPPROTO_UDP {
		state = 1 << 7
	} // TCP_CLOSE: unconnected UDP
	native.PutUint32(request[20:24], state)
	if err := unix.Sendto(fd, request, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	result := map[uint32]bool{}
	buffer := make([]byte, 65536)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, _, flags, _, err := unix.Recvmsg(fd, buffer, nil, 0)
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			timeout := 100
			if deadline, ok := ctx.Deadline(); ok {
				remaining := time.Until(deadline)
				if remaining <= 0 {
					return nil, context.DeadlineExceeded
				}
				if remaining < 100*time.Millisecond {
					timeout = int((remaining + time.Millisecond - 1) / time.Millisecond)
				}
			}
			_, pollErr := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, timeout)
			if pollErr != nil && pollErr != unix.EINTR {
				return nil, pollErr
			}
			continue
		}
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return nil, err
		}
		if flags&unix.MSG_TRUNC != 0 {
			return nil, fmt.Errorf("socket diagnostics truncated")
		}
		for offset := 0; offset+16 <= n; {
			length := int(native.Uint32(buffer[offset : offset+4]))
			if length < 16 || offset+length > n {
				return nil, fmt.Errorf("invalid socket diagnostics")
			}
			typ := native.Uint16(buffer[offset+4 : offset+6])
			if native.Uint16(buffer[offset+6:offset+8])&unix.NLM_F_DUMP_INTR != 0 {
				return nil, fmt.Errorf("socket diagnostics interrupted")
			}
			if typ == unix.NLMSG_DONE {
				return result, nil
			}
			if typ == unix.NLMSG_ERROR {
				return nil, fmt.Errorf("socket diagnostics unavailable")
			}
			body := buffer[offset+16 : offset+length]
			if typ == 20 && len(body) >= 72 {
				inode := native.Uint32(body[68:72])
				for pos := 72; pos+4 <= len(body); {
					attrLength := int(native.Uint16(body[pos : pos+2]))
					if attrLength < 4 || pos+attrLength > len(body) {
						return nil, fmt.Errorf("invalid socket attribute")
					}
					if native.Uint16(body[pos+2:pos+4]) == 11 && attrLength >= 5 {
						result[inode] = body[pos+4] != 0
					}
					pos += (attrLength + 3) &^ 3
				}
			}
			offset += (length + 3) &^ 3
		}
	}
}
