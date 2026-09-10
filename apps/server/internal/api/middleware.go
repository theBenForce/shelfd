package api

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

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
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Cache-Control")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

func (w *statusResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// RequestLoggerMiddleware logs HTTP requests with method, path, status, latency, and client metadata.
func RequestLoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusResponseWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)
			statusCode := sw.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

			// Health checks logged at debug level to prevent log clutter
			level := slog.LevelInfo
			if r.URL.Path == "/health" {
				level = slog.LevelDebug
			} else if statusCode >= 500 {
				level = slog.LevelError
			} else if statusCode >= 400 {
				level = slog.LevelWarn
			}

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", statusCode,
				"duration_ms", float64(duration.Microseconds()) / 1000.0,
				"bytes", sw.bytesWritten,
				"remote_ip", clientIP(r),
			}

			if user := CurrentUser(r.Context()); user != nil {
				attrs = append(attrs, "user_id", user.ID, "username", user.Username)
			}

			logger.Log(r.Context(), level, "http_request", attrs...)
		})
	}
}
