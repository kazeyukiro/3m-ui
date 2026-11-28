package netutil

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestIsMetadataIP(t *testing.T) {
	for _, ip := range []string{"169.254.169.254", "fd00:ec2::254"} {
		if !IsMetadataIP(net.ParseIP(ip)) {
			t.Fatalf("%s must be flagged as metadata", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "127.0.0.1"} {
		if IsMetadataIP(net.ParseIP(ip)) {
			t.Fatalf("%s must NOT be flagged as metadata", ip)
		}
	}
	if IsMetadataIP(nil) {
		t.Fatal("nil must not be metadata")
	}
}

func TestAssertIPAllowed(t *testing.T) {
	// Metadata endpoints remain blocked even when private targets are enabled.
	t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "")
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "1")
	if err := AssertIPAllowed(net.ParseIP("169.254.169.254")); err == nil {
		t.Fatal("metadata endpoint must be blocked")
	}
	if err := AssertIPAllowed(net.ParseIP("fd00:ec2::254")); err == nil {
		t.Fatal("IPv6 metadata endpoint must be blocked")
	}
	// Lab mode allows private/loopback.
	if err := AssertIPAllowed(net.ParseIP("127.0.0.1")); err != nil {
		t.Fatalf("loopback should be allowed in lab mode: %v", err)
	}
	if err := AssertIPAllowed(net.ParseIP("10.0.0.1")); err != nil {
		t.Fatalf("private should be allowed in lab mode: %v", err)
	}

	// Public IPs are always allowed.
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "0")
	if err := AssertIPAllowed(net.ParseIP("1.1.1.1")); err != nil {
		t.Fatalf("public IP should be allowed: %v", err)
	}

	// Private blocked in normal mode.
	if err := AssertIPAllowed(net.ParseIP("127.0.0.1")); err == nil {
		t.Fatal("loopback must be blocked in normal mode")
	}
	if err := AssertIPAllowed(net.ParseIP("10.0.0.1")); err == nil {
		t.Fatal("private must be blocked in normal mode")
	}
	if err := AssertIPAllowed(net.ParseIP("169.254.169.254")); err == nil {
		t.Fatal("metadata must be blocked in normal mode")
	}
	if err := AssertIPAllowed(nil); err == nil {
		t.Fatal("nil IP must be rejected")
	}
}

func TestAllowPrivateTargetsLegacyEnv(t *testing.T) {
	t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "")
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "1")
	if !AllowPrivateTargets() {
		t.Fatal("legacy THREE_M_UI_CLUSTER_ALLOW_PRIVATE=1 should be honoured")
	}
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "0")
	t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "1")
	if !AllowPrivateTargets() {
		t.Fatal("THREE_M_UI_ALLOW_PRIVATE=1 should be honoured")
	}
	t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "0")
	if AllowPrivateTargets() {
		t.Fatal("0 should disable")
	}
}

func TestSafeDialContextBlocksPrivateIP(t *testing.T) {
	t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "0")
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "0")
	dial := SafeDialContext(&net.Dialer{Timeout: 100 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := dial(ctx, "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("expected private IP to be blocked before dialing")
	}
	if _, err := dial(ctx, "tcp", "169.254.169.254:80"); err == nil {
		t.Fatal("metadata must be blocked before dialing")
	}
	// Invalid address format.
	if _, err := dial(ctx, "tcp", "no-port-here"); err == nil {
		t.Fatal("expected invalid address error")
	}
}
