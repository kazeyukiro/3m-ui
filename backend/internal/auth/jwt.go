package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID         uint      `json:"user_id"`
	Username       string    `json:"username"`
	Role           string    `json:"role"`
	SessionVersion uint      `json:"session_version"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func GenerateToken(secret string, userID uint, username, role string, sessionVersion uint, ttl time.Duration) (string, time.Time, error) {
	if strings.TrimSpace(secret) == "" {
		return "", time.Time{}, errors.New("JWT secret is not configured")
	}
	if userID == 0 || strings.TrimSpace(username) == "" || strings.TrimSpace(role) == "" {
		return "", time.Time{}, errors.New("invalid token subject")
	}
	if ttl <= 0 {
		return "", time.Time{}, errors.New("token TTL must be positive")
	}
	now := time.Now()
	exp := now.Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":         userID,
		"username":        username,
		"role":            role,
		"session_version": sessionVersion,
		"exp":             exp.Unix(),
		"iat":             now.Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

func ParseToken(secret, tokenString string) (*JWTClaims, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("JWT secret is not configured")
	}
	if strings.TrimSpace(tokenString) == "" {
		return nil, errors.New("invalid or expired token")
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, errors.New("invalid or expired token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	userID, ok := jwtUintClaim(claims, "user_id")
	if !ok || userID == 0 {
		return nil, errors.New("invalid token claims")
	}
	username, ok := claims["username"].(string)
	if !ok || strings.TrimSpace(username) == "" {
		return nil, errors.New("invalid token claims")
	}
	role, ok := claims["role"].(string)
	if !ok || strings.TrimSpace(role) == "" {
		return nil, errors.New("invalid token claims")
	}
	sessionVersion, ok := jwtUintClaim(claims, "session_version")
	if !ok {
		return nil, errors.New("invalid token claims")
	}
	exp, ok := jwtInt64Claim(claims, "exp")
	if !ok || exp <= time.Now().Unix() {
		return nil, errors.New("token expired")
	}

	return &JWTClaims{
		UserID:         userID,
		Username:       username,
		Role:           role,
		SessionVersion: sessionVersion,
		ExpiresAt:      time.Unix(exp, 0),
	}, nil
}

func jwtUintClaim(claims jwt.MapClaims, key string) (uint, bool) {
	v, ok := claims[key].(float64)
	if !ok || v < 0 || v != float64(uint64(v)) || v > float64(^uint(0)) {
		return 0, false
	}
	return uint(v), true
}

func jwtInt64Claim(claims jwt.MapClaims, key string) (int64, bool) {
	v, ok := claims[key].(float64)
	if !ok || v != float64(int64(v)) || v < 0 || v > float64(^uint64(0)>>1) {
		return 0, false
	}
	return int64(v), true
}

// TokenFromRequest extracts the Bearer token from Authorization. The HTTP
// authentication scheme is case-insensitive, so "bearer" and "BEARER" are
// accepted as well as the conventional "Bearer" spelling.
func TokenFromRequest(authHeader string) string {
	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
