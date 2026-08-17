package auth

import (
	"testing"
	"time"
)

func TestTokenFromRequest(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "bearer", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "bearer with surrounding whitespace", header: "Bearer   abc", want: "abc"},
		{name: "wrong scheme", header: "Basic abc", want: ""},
		{name: "empty", header: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TokenFromRequest(tt.header); got != tt.want {
				t.Fatalf("TokenFromRequest(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestJWTTokenRoundTrip(t *testing.T) {
	secret := "test-secret"
	token, expiresAt, err := GenerateToken(secret, 42, "admin", "admin", 7, time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if expiresAt.Before(time.Now()) {
		t.Fatal("generated token is already expired")
	}

	claims, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 42 || claims.Username != "admin" || claims.Role != "admin" || claims.SessionVersion != 7 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
