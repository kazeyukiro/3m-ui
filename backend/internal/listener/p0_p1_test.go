package listener

import "testing"

func TestPortRangesOverlap(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"443", "443", true},
		{"443-445", "445-450", true},
		{"443-445", "446-450", false},
		{"443,8443", "8443", true},
		{"443,8443", "9443", false},
	}
	for _, tt := range tests {
		if got := PortRangesOverlap(tt.a, tt.b); got != tt.want {
			t.Fatalf("PortRangesOverlap(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestAddressesConflict(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.0.0.0", "127.0.0.1", true},
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.1", "127.0.0.2", false},
		{"::", "::1", true},
		{"::1", "::1", true},
		{"127.0.0.1", "::1", false},
	}
	for _, tt := range tests {
		if got := AddressesConflict(tt.a, tt.b); got != tt.want {
			t.Fatalf("AddressesConflict(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
