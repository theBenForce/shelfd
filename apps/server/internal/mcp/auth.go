package mcp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/shelfd/shelfd/internal/repository"
)

type contextKey string

const (
	TokenContextKey contextKey = "mcp_token"
)

// HashToken computes the lowercase hexadecimal SHA-256 hash of a raw API token.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// AuthMiddleware creates an HTTP middleware that validates incoming Bearer API tokens
// against SHA-256 hashed records in the database using constant-time comparison.
func AuthMiddleware(repo repository.StorageEngine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Bypass auth check for CORS preflight OPTIONS requests
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			rawToken := ""

			// 1. Try Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					rawToken = strings.TrimSpace(parts[1])
				}
			}

			// 2. Fallback to query parameter
			if rawToken == "" {
				rawToken = strings.TrimSpace(r.URL.Query().Get("token"))
			}

			if rawToken == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"missing authorization token"}`))
				return
			}

			tokenHash := HashToken(rawToken)
			storedToken, err := repo.GetAPITokenByHash(r.Context(), tokenHash)
			if err != nil || storedToken == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid or expired token"}`))
				return
			}

			// Perform constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(storedToken.TokenHash), []byte(tokenHash)) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid token"}`))
				return
			}

			ctx := context.WithValue(r.Context(), TokenContextKey, storedToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
