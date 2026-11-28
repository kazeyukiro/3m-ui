package user

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/security"
)

func TestCredentialEncryptionRoundTrip(t *testing.T) {
	security.InitCredentialKey("test-secret")
	encrypted, err := encryptPassword("super-secret")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "super-secret" || encrypted == "" {
		t.Fatal("password was not encrypted")
	}
	decrypted, err := decryptPassword(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "super-secret" {
		t.Fatalf("unexpected decrypted value: %q", decrypted)
	}
}

func TestCredentialEncryptionIsRandomized(t *testing.T) {
	security.InitCredentialKey("test-secret")
	a, err := encryptPassword("same")
	if err != nil {
		t.Fatal(err)
	}
	b, err := encryptPassword("same")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected randomized ciphertext")
	}
}
