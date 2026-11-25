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
	UserContextKey            contextKey = "shelfd_user"
	authDiagnosticsContextKey contextKey = "shelfd_auth_diag"
)

type authLogDiagnostics struct {
	source      string
	errorReason string
}

func getAuthLogDiagnostics(ctx context.Context) *authLogDiagnostics {
	if d, ok := ctx.Value(authDiagnosticsContextKey).(*authLogDiagnostics); ok {
		return d
	}
	return nil
}

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
			authSource := "none"

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					rawToken = strings.TrimSpace(parts[1])
					if rawToken != "" {
						if strings.HasPrefix(rawToken, "shelfd_") {
							authSource = "bearer_api_token"
						} else {
							authSource = "bearer_jwt"
						}
					}
				}
			}

			if rawToken == "" {
				if cookie, err := r.Cookie("shelfd_token"); err == nil && cookie.Value != "" {
					rawToken = strings.TrimSpace(cookie.Value)
					if rawToken != "" {
						authSource = "cookie"
					}
				}
			}

			if rawToken == "" {
				rawToken = strings.TrimSpace(r.URL.Query().Get("token"))
				if rawToken != "" {
					authSource = "query"
				}
			}

			diag := getAuthLogDiagnostics(r.Context())
			if diag != nil {
				diag.source = authSource
			}

			if rawToken == "" {
				if diag != nil {
					diag.errorReason = "missing_token"
				}
				writeJSONError(w, http.StatusUnauthorized, "Missing authorization token")
				return
			}

			var user *repository.User

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

			var jwtClaims *UserClaims
			if user == nil && jwtSecret != "" {
				claims, err := ValidateJWT(rawToken, jwtSecret)
				if err == nil && claims != nil {
					u, err := repo.GetUserByID(r.Context(), claims.UserID)
					if err == nil && u != nil {
						user = u
						jwtClaims = claims
					}
				}
			}

			if user == nil {
				tokenHash := mcp.HashToken(rawToken)
				apiToken, err := repo.GetAPITokenByHash(r.Context(), tokenHash)
				if err == nil && apiToken != nil && subtle.ConstantTimeCompare([]byte(apiToken.TokenHash), []byte(tokenHash)) == 1 {
					u, err := repo.GetUserByID(r.Context(), apiToken.UserID)
					if err == nil && u != nil {
						user = u
						if authSource == "bearer_jwt" {
							authSource = "bearer_api_token"
							if diag != nil {
								diag.source = authSource
							}
						}
					}
				}
			}

			if user == nil {
				if diag != nil {
					diag.errorReason = "invalid_token"
				}
				writeJSONError(w, http.StatusUnauthorized, "Invalid or expired authorization token")
				return
			}

			if jwtClaims != nil && (jwtClaims.PluginName != "" || len(jwtClaims.Scopes) > 0) {
				allowed := false
				if jwtClaims.PluginName != "" {
					allowed = PluginScopeAllowsRequest(jwtClaims.Scopes, r.Method, r.URL.Path)
				} else {
					allowed = CastScopeAllowsRequest(jwtClaims.Scopes, r.Method, r.URL.Path)
				}
				if !allowed {
					if diag != nil {
						diag.errorReason = "insufficient_scope"
					}
					writeJSONError(w, http.StatusForbidden, "Insufficient token scope for requested endpoint")
					return
				}
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CastScopeAllowsRequest checks if a book-scoped cast:media:<bookID> token permits a read-only media request.
func CastScopeAllowsRequest(scopes []string, method, reqPath string) bool {
	if method != http.MethodGet && method != http.MethodHead {
		return false
	}
	for _, s := range scopes {
		if !strings.HasPrefix(s, "cast:media:") {
			continue
		}
		bookID := strings.TrimPrefix(s, "cast:media:")
		if bookID == "" {
			continue
		}
		switch reqPath {
		case "/api/v1/audiobooks/" + bookID + "/stream",
			"/api/v1/books/" + bookID + "/audio",
			"/api/v1/books/" + bookID + "/cover",
			"/api/v1/books/" + bookID + "/readalong.vtt":
			return true
		}
		if strings.HasPrefix(reqPath, "/api/v1/books/"+bookID+"/assets/") {
			return true
		}
		if strings.HasPrefix(reqPath, "/api/v1/books/"+bookID+"/files/") && strings.HasSuffix(reqPath, "/stream") {
			return true
		}
		if strings.HasPrefix(reqPath, "/api/v1/books/"+bookID+"/chapters/") && strings.HasSuffix(reqPath, "/html") {
			return true
		}
	}
	return false
}

// PluginScopeAllowsRequest checks if the declared plugin API scopes permit the HTTP method and path.
func PluginScopeAllowsRequest(scopes []string, method, path string) bool {
	hasScope := func(target string) bool {
		for _, s := range scopes {
			if strings.EqualFold(strings.TrimSpace(s), target) {
				return true
			}
		}
		return false
	}

	cleanPath := strings.TrimRight(path, "/")
	if cleanPath == "" {
		cleanPath = "/"
	}

	if method == http.MethodPost {
		if cleanPath == "/api/v1/books/upload/stage" {
			return hasScope("uploads:stage")
		}
		if strings.HasPrefix(cleanPath, "/api/v1/books/upload/jobs/") && strings.HasSuffix(cleanPath, "/cover") {
			return hasScope("uploads:stage")
		}
		if cleanPath == "/api/v1/books/upload" {
			return hasScope("uploads:write")
		}
	}

	if method == http.MethodGet || method == http.MethodHead {
		if cleanPath == "/api/v1/books" || strings.HasPrefix(cleanPath, "/api/v1/books/") ||
			strings.HasPrefix(cleanPath, "/api/v1/audiobooks/") ||
			cleanPath == "/api/v1/search" ||
			cleanPath == "/api/v1/authors" || strings.HasPrefix(cleanPath, "/api/v1/authors/") ||
			cleanPath == "/api/v1/genres" || cleanPath == "/api/v1/topics" ||
			cleanPath == "/api/v1/series" || strings.HasPrefix(cleanPath, "/api/v1/series/") {
			return hasScope("books:read")
		}
	}

	return false
}

// CORSMiddleware enables CORS for Flutter web and cross-origin desktop/mobile apps.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")

		reqHeaders := r.Header.Get("Access-Control-Request-Headers")
		if reqHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Cache-Control, X-Requested-With, MCP-Protocol-Version, Mcp-Session-Id, Mcp-Method, Mcp-Name")
		}
		w.Header().Set("Access-Control-Expose-Headers", "MCP-Protocol-Version, Mcp-Session-Id, Content-Type, Authorization, Content-Disposition")
		w.Header().Set("Access-Control-Max-Age", "86400")

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
			diag := &authLogDiagnostics{}
			reqWithDiag := r.WithContext(context.WithValue(r.Context(), authDiagnosticsContextKey, diag))

			next.ServeHTTP(sw, reqWithDiag)

			duration := time.Since(start)
			statusCode := sw.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

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

			if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) &&
				(diag.source != "" || diag.errorReason != "") {
				attrs = append(attrs, "auth_source", diag.source, "auth_error_reason", diag.errorReason)
			}

			if user := CurrentUser(reqWithDiag.Context()); user != nil {
				attrs = append(attrs, "user_id", user.ID, "username", user.Username)
			}

			logger.Log(reqWithDiag.Context(), level, "http_request", attrs...)
		})
	}
}
