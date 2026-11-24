package api

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserClaims defines the JWT claims issued by Shelfd.
type UserClaims struct {
	UserID     string   `json:"uid"`
	Username   string   `json:"name"`
	PluginName string   `json:"plg,omitempty"`
	Scopes     []string `json:"scp,omitempty"`
	jwt.RegisteredClaims
}

// GenerateJWT creates and signs a new HMAC-SHA256 JWT for an authenticated user.
func GenerateJWT(userID, username, secret string, duration time.Duration) (string, time.Time, error) {
	return generateJWTWithScopes(userID, username, "", nil, secret, duration)
}

// GeneratePluginInvocationJWT creates a short-lived, scope-restricted JWT for a plugin RPC invocation on behalf of a user.
func GeneratePluginInvocationJWT(userID, username, pluginName string, scopes []string, secret string, duration time.Duration) (string, time.Time, error) {
	if pluginName == "" {
		return "", time.Time{}, errors.New("plugin name cannot be empty")
	}
	return generateJWTWithScopes(userID, username, pluginName, scopes, secret, duration)
}

// GenerateCastMediaJWT creates a read-only, book-scoped JWT for LAN Chromecast media, cover, and WebVTT fetches.
func GenerateCastMediaJWT(userID, username, bookID, secret string, duration time.Duration) (string, time.Time, error) {
	if bookID == "" {
		return "", time.Time{}, errors.New("book id cannot be empty")
	}
	return generateJWTWithScopes(userID, username, "", []string{"cast:media:" + bookID}, secret, duration)
}

func generateJWTWithScopes(userID, username, pluginName string, scopes []string, secret string, duration time.Duration) (string, time.Time, error) {
	if secret == "" {
		return "", time.Time{}, errors.New("jwt secret cannot be empty")
	}

	expiresAt := time.Now().Add(duration)
	claims := UserClaims{
		UserID:     userID,
		Username:   username,
		PluginName: pluginName,
		Scopes:     scopes,
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
