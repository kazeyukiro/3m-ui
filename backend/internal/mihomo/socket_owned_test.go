package mihomo

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func procAddressForTest(address string, port uint16) string {
	ip := net.ParseIP(address)
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	var result string
	for i := 0; i < len(ip); i += 4 {
		result += fmt.Sprintf("%08X", binary.NativeEndian.Uint32(ip[i:i+4]))
	}
	return fmt.Sprintf("%s:%04X", result, port)
}

func procRowForTest(address, remote, state string, inode int) string {
	return fmt.Sprintf("0: %s %s %s 00000000:00000000 00:00000000 00000000 1000 0 %d 1\n", address, remote, state, inode)
}

func TestReadOwnedSocketsFiltersOwnershipAndConnectionState(t *testing.T) {
	dir := t.TempDir()
	for _, child := range []string{"fd", "net"} {
		if err := os.Mkdir(filepath.Join(dir, child), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for fd, inode := range []int{11, 12, 13} {
		if err := os.Symlink(fmt.Sprintf("socket:[%d]", inode), filepath.Join(dir, "fd", fmt.Sprint(fd))); err != nil {
			t.Fatal(err)
		}
	}
	local := procAddressForTest("127.0.0.1", 12000)
	remote := procAddressForTest("0.0.0.0", 0)
	tcp := "header\n" + procRowForTest(local, remote, "0A", 11) +
		procRowForTest(local, remote, "0A", 99) + // belongs to another PID
		procRowForTest(local, remote, "01", 12) // established TCP
	udp := "header\n" + procRowForTest(local, remote, "07", 13) +
		procRowForTest(local, procAddressForTest("127.0.0.2", 53), "01", 12)
	for name, content := range map[string]string{"tcp": tcp, "udp": udp} {
		if err := os.WriteFile(filepath.Join(dir, "net", name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := readOwnedSockets(context.Background(), dir, 42)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	l := runtimeListener{Type: "shadowsocks", Listen: "127.0.0.1", Port: "12000", UDP: true}
	if result := inspectListener(l, 42, got, nil); result.State != "listening" {
		t.Fatalf("owned TCP/UDP missing: %+v", result)
	}
	// An unreadable/non-link descriptor must propagate an error, not silently
	// produce an incomplete ownership list. This also works when tests run as root.
	if err := os.WriteFile(filepath.Join(dir, "fd", "4"), []byte("not a link"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readOwnedSockets(context.Background(), dir, 42); err == nil {
		t.Fatal("descriptor inspection error was swallowed")
	}
}

func TestProcSocketAddressesAndIPv6(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "0.0.0.0", "::", "::1", "2001:db8::1234", "::ffff:192.0.2.1"} {
		got, err := procSocketAddress(procAddressForTest(address, 443))
		if err != nil || got.Port != 443 || !net.ParseIP(got.IP).Equal(net.ParseIP(address)) {
			t.Fatalf("%s: %+v %v", address, got, err)
		}
	}
	for _, raw := range []string{"", "0100007F", "0100007G:0050", "0100007F:10000", "0100007F:xyz"} {
		if _, err := procSocketAddress(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	row := procRowForTest(procAddressForTest("::", 443), procAddressForTest("::", 0), "0A", 123)
	got, err := readSocketTable(context.Background(), strings.NewReader("header\n"+row), 42, syscall.SOCK_STREAM, map[uint64]uint32{123: 5})
	if err != nil || len(got) != 1 || got[0].Laddr.IP != "::" || got[0].inode != 123 {
		t.Fatalf("IPv6 table: %+v %v", got, err)
	}
}

func TestSocketInspectionCancellationAndMalformedTables(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readOwnedSockets(ctx, t.TempDir(), 42); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	row := procRowForTest(procAddressForTest("127.0.0.1", 443), procAddressForTest("0.0.0.0", 0), "0A", 123)
	if _, err := readSocketTable(ctx, strings.NewReader("header\n"+row), 42, syscall.SOCK_STREAM, map[uint64]uint32{123: 5}); !errors.Is(err, context.Canceled) {
		t.Fatalf("table ignored cancellation: %v", err)
	}
	for _, table := range []string{"", "header\ninvalid\n", "header\n" + strings.Repeat("x", 65537)} {
		if _, err := readSocketTable(context.Background(), strings.NewReader(table), 42, syscall.SOCK_STREAM, nil); err == nil {
			t.Fatal("invalid/truncated table accepted")
		}
	}
}
