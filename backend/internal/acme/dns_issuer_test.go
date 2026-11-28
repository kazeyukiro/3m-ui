package acme

import "testing"

func TestCertNamesWildcard(t *testing.T) {
	names := certNames("*.example.com")
	if len(names) != 2 || names[0] != "*.example.com" || names[1] != "example.com" {
		t.Fatalf("got %v", names)
	}
}

func TestNeedsDNS01(t *testing.T) {
	if !NeedsDNS01(Settings{Domain: "*.example.com"}) {
		t.Fatal("wildcard should need DNS-01")
	}
	if !NeedsDNS01(Settings{Domain: "a.example.com", Challenge: "dns-01"}) {
		t.Fatal("explicit dns-01")
	}
	if NeedsDNS01(Settings{Domain: "a.example.com"}) {
		t.Fatal("plain domain should stay HTTP-01")
	}
}

func TestDNS01TXTValueLength(t *testing.T) {
	v := dns01TXTValue("token.thumbprint")
	if len(v) < 40 {
		t.Fatalf("unexpected value %q", v)
	}
}
