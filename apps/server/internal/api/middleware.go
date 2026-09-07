package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
)

type contextKey string

const (
	UserContextKey contextKey = "shelfd_user"
)

// CurrentUser retrieves the authenticated User from the request context.
func CurrentUser(ctx context.Context) *repository.User {
	if u, ok := ctx.Value(UserContextKey).(*repository.User); ok {
		return u
	}
	return nil
}

// AuthMiddleware creates a middleware supporting both short-lived JWTs and persistent API tokens.
func AuthMiddleware(repo repository.StorageEngine, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := ""

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					rawToken = strings.TrimSpace(parts[1])
				}
			}

			if rawToken == "" {
				rawToken = strings.TrimSpace(r.URL.Query().Get("token"))
			}

			if rawToken == "" {
				writeJSONError(w, http.StatusUnauthorized, "Missing authorization token")
				return
			}

			var user *repository.User

			// 1. Try persistent API token (e.g. shelfd_...)
			if strings.HasPrefix(rawToken, "shelfd_") {
				tokenHash := mcp.HashToken(rawToken)
				apiToken, err := repo.GetAPITokenByHash(r.Context(), tokenHash)
				if err == nil && apiToken != nil && subtle.ConstantTimeCompare([]byte(apiToken.TokenHash), []byte(tokenHash)) == 1 {
					u, err := repo.GetUserByID(r.Context(), apiToken.UserID)
					if err == nil && u != nil {
						user = u
					}
				}
			}

			// 2. Try JWT token
			if user == nil && jwtSecret != "" {
				claims, err := ValidateJWT(rawToken, jwtSecret)
				if err == nil && claims != nil {
					u, err := repo.GetUserByID(r.Context(), claims.UserID)
					if err == nil && u != nil {
						user = u
					}
				}
			}

			// 3. Fallback: try raw API token without shelfd_ prefix
			if user == nil {
				tokenHash := mcp.HashToken(rawToken)
				apiToken, err := repo.GetAPITokenByHash(r.Context(), tokenHash)
				if err == nil && apiToken != nil && subtle.ConstantTimeCompare([]byte(apiToken.TokenHash), []byte(tokenHash)) == 1 {
					u, err := repo.GetUserByID(r.Context(), apiToken.UserID)
					if err == nil && u != nil {
						user = u
					}
				}
			}

			if user == nil {
				writeJSONError(w, http.StatusUnauthorized, "Invalid or expired authorization token")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CORSMiddleware enables CORS for Flutter web and cross-origin desktop/mobile apps.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
