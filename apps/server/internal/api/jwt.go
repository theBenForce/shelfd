package api

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserClaims defines the JWT claims issued by Shelfd.
type UserClaims struct {
	UserID   string `json:"uid"`
	Username string `json:"name"`
	jwt.RegisteredClaims
}

// GenerateJWT creates and signs a new HMAC-SHA256 JWT for an authenticated user.
func GenerateJWT(userID, username, secret string, duration time.Duration) (string, time.Time, error) {
	if secret == "" {
		return "", time.Time{}, errors.New("jwt secret cannot be empty")
	}

	expiresAt := time.Now().Add(duration)
	claims := UserClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "shelfd",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing jwt: %w", err)
	}

	return signed, expiresAt, nil
}

// ValidateJWT validates an incoming JWT token string against the configured secret.
func ValidateJWT(tokenStr, secret string) (*UserClaims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing jwt: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid jwt claims")
	}

	return claims, nil
}
