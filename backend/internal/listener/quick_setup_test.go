package listener

import "testing"

func TestResolveRealityKeys(t *testing.T) {
	priv, pub, err := resolveRealityKeys("", "")
	if err != nil {
		t.Fatal(err)
	}
	if priv == "" || pub == "" {
		t.Fatal("empty keys")
	}
	pub2, err := derivePublicKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	if pub2 != pub {
		t.Fatalf("derived public key mismatch")
	}
}

func TestRandomHelpers(t *testing.T) {
	if len(randomHex(8)) != 16 {
		t.Fatal("hex length")
	}
	if len(randomPassword(12)) != 12 {
		t.Fatal("password length")
	}
}
