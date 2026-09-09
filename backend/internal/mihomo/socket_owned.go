package mihomo

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	psnet "github.com/shirou/gopsutil/v4/net"
)

// Read ownership once and preserve inspection errors. In particular, denied
// readlink access must not become an empty list and a false binding failure.
func ownedConnections(ctx context.Context, pid int) ([]runtimeConnection, error) {
	if runtime.GOOS != "linux" || pid <= 0 {
		return nil, fmt.Errorf("socket inspection unavailable")
	}
	connections, err := readOwnedSockets(ctx, filepath.Join("/proc", strconv.Itoa(pid)), pid)
	if err != nil {
		return nil, err
	}
	families := map[uint32]map[uint32]bool{}
	for i := range connections {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn := &connections[i]
		if conn.Laddr.IP != "::" {
			continue
		}
		if _, found := families[conn.Type]; !found {
			protocol := byte(syscall.IPPROTO_TCP)
			if conn.Type == syscall.SOCK_DGRAM {
				protocol = syscall.IPPROTO_UDP
			}
			families[conn.Type], _ = ipv6OnlySockets(ctx, protocol)
		}
		// The diagnostic protocol exposes a 32-bit inode. Do not truncate a
		// larger proc inode and accidentally associate it with another socket.
		if conn.inode <= 1<<32-1 {
			conn.ipv6Only, conn.familyKnown = families[conn.Type][uint32(conn.inode)]
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return connections, nil
}

func readOwnedSockets(ctx context.Context, procDir string, pid int) ([]runtimeConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fds, err := os.Open(filepath.Join(procDir, "fd"))
	if err != nil {
		return nil, err
	}
	defer fds.Close()
	inodes := map[uint64]uint32{}
	for {
		entries, readErr := fds.ReadDir(128)
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			link, err := os.Readlink(filepath.Join(procDir, "fd", entry.Name()))
			if os.IsNotExist(err) {
				continue // A connection may close while enumerating descriptors.
			}
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
				continue
			}
			inode, err := strconv.ParseUint(link[8:len(link)-1], 10, 64)
			if err != nil {
				return nil, err
			}
			fd, err := strconv.ParseUint(entry.Name(), 10, 32)
			if err != nil {
				return nil, err
			}
			inodes[inode] = uint32(fd)
		}
		if readErr == io.EOF {
			break
		}
	}
	var result []runtimeConnection
	for _, table := range []struct {
		name string
		kind uint32
	}{
		{"tcp", syscall.SOCK_STREAM}, {"tcp6", syscall.SOCK_STREAM},
		{"udp", syscall.SOCK_DGRAM}, {"udp6", syscall.SOCK_DGRAM},
	} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := os.Open(filepath.Join(procDir, "net", table.name))
		if os.IsNotExist(err) && strings.HasSuffix(table.name, "6") {
			continue // Linux may have IPv6 disabled.
		}
		if err != nil {
			return nil, err
		}
		connections, err := readSocketTable(ctx, file, pid, table.kind, inodes)
		file.Close()
		if err != nil {
			return nil, err
		}
		result = append(result, connections...)
	}
	return result, ctx.Err()
}

func readSocketTable(ctx context.Context, reader io.Reader, pid int, kind uint32, inodes map[uint64]uint32) ([]runtimeConnection, error) {
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("missing socket table header")
	}
	var result []runtimeConnection
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			return nil, fmt.Errorf("invalid socket table row")
		}
		// Ignore established TCP and connected UDP before parsing addresses.
		if kind == syscall.SOCK_STREAM && fields[3] != "0A" {
			continue
		}
		if kind == syscall.SOCK_DGRAM && fields[3] != "07" {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, err
		}
		fd, owned := inodes[inode]
		if !owned {
			continue
		}
		local, err := procSocketAddress(fields[1])
		if err != nil {
			return nil, err
		}
		remote, err := procSocketAddress(fields[2])
		if err != nil {
			return nil, err
		}
		if kind == syscall.SOCK_DGRAM && remote.Port != 0 {
			continue
		}
		status := ""
		if kind == syscall.SOCK_STREAM {
			status = "LISTEN"
		}
		result = append(result, runtimeConnection{ConnectionStat: psnet.ConnectionStat{
			Pid: int32(pid), Fd: fd, Type: kind, Status: status, Laddr: local, Raddr: remote,
		}, inode: inode})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, ctx.Err()
}

func procSocketAddress(raw string) (psnet.Addr, error) {
	host, port, ok := strings.Cut(raw, ":")
	if !ok || (len(host) != 8 && len(host) != 32) {
		return psnet.Addr{}, fmt.Errorf("invalid socket address")
	}
	ip := make(net.IP, len(host)/2)
	// /proc represents each 32-bit address word in host byte order.
	for i := 0; i < len(host); i += 8 {
		word, err := strconv.ParseUint(host[i:i+8], 16, 32)
		if err != nil {
			return psnet.Addr{}, err
		}
		binary.NativeEndian.PutUint32(ip[i/2:i/2+4], uint32(word))
	}
	n, err := strconv.ParseUint(port, 16, 16)
	if err != nil {
		return psnet.Addr{}, err
	}
	return psnet.Addr{IP: ip.String(), Port: uint32(n)}, nil
}
