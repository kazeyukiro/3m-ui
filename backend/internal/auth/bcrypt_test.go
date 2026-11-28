package auth_test

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "my_secure_password"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == "" {
		t.Fatal("expected hash to be non-empty")
	}

	if hash == password {
		t.Fatal("expected hash to be different from plaintext password")
	}

	if !auth.CheckPasswordHash(password, hash) {
		t.Fatal("expected password hash check to succeed")
	}

	if auth.CheckPasswordHash("wrong_password", hash) {
		t.Fatal("expected password hash check to fail for incorrect password")
	}
}
