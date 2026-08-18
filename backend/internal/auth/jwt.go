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
	exp := time.Now().Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":         userID,
		"username":        username,
		"role":            role,
		"session_version": sessionVersion,
		"exp":             exp.Unix(),
		"iat":             time.Now().Unix(),
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

	exp, _ := claims["exp"].(float64)
	if time.Now().Unix() > int64(exp) {
		return nil, errors.New("token expired")
	}

	jc := &JWTClaims{}
	if v, ok := claims["user_id"].(float64); ok {
		jc.UserID = uint(v)
	}
	if v, ok := claims["username"].(string); ok {
		jc.Username = v
	}
	if v, ok := claims["role"].(string); ok {
		jc.Role = v
	}
	if v, ok := claims["session_version"].(float64); ok {
		jc.SessionVersion = uint(v)
	}
	if exp, ok := claims["exp"].(float64); ok {
		jc.ExpiresAt = time.Unix(int64(exp), 0)
	}
	return jc, nil
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
