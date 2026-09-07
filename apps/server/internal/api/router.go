package api

import (
	"net/http"

	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

// RouterConfig contains dependencies for assembling the Shelfd REST API.
type RouterConfig struct {
	Repo       repository.StorageEngine
	Ingester   *scanner.Ingester
	Scanner    *scanner.Scanner
	Worker     *worker.Worker
	DataDir    string
	LibraryDir string
	JWTSecret  string
	Host       string
	Port       int
	Version    string
}

// NewRouter constructs and returns the fully configured /api/v1 HTTP handler.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	authHandler := NewAuthHandler(cfg.Repo, cfg.JWTSecret)
	bookHandler := NewBookHandler(cfg.Repo, cfg.Ingester, cfg.Worker, cfg.DataDir, cfg.LibraryDir)
	taxHandler := NewTaxonomyHandler(cfg.Repo)
	libHandler := NewLibraryHandler(cfg.Repo, cfg.Scanner, cfg.Ingester, cfg.Worker, nil)
	connHandler := NewConnectHandler(cfg.Host, cfg.Port, cfg.Version)

	auth := AuthMiddleware(cfg.Repo, cfg.JWTSecret)

	// Public routes
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/v1/server/connect-info", connHandler.GetConnectInfo)
	mux.HandleFunc("GET /api/v1/books/{id}/cover", bookHandler.GetBookCover)

	// Protected routes
	mux.Handle("GET /api/v1/auth/me", auth(http.HandlerFunc(authHandler.Me)))
	mux.Handle("POST /api/v1/auth/tokens", auth(http.HandlerFunc(authHandler.CreateToken)))
	mux.Handle("GET /api/v1/auth/tokens", auth(http.HandlerFunc(authHandler.ListTokens)))
	mux.Handle("DELETE /api/v1/auth/tokens/{id}", auth(http.HandlerFunc(authHandler.DeleteToken)))

	mux.Handle("GET /api/v1/books", auth(http.HandlerFunc(bookHandler.ListBooks)))
	mux.Handle("POST /api/v1/books/upload", auth(http.HandlerFunc(bookHandler.UploadBook)))
	mux.Handle("GET /api/v1/books/{id}", auth(http.HandlerFunc(bookHandler.GetBook)))
	mux.Handle("GET /api/v1/books/{id}/chapters/{index}", auth(http.HandlerFunc(bookHandler.GetChapter)))

	mux.Handle("GET /api/v1/authors", auth(http.HandlerFunc(taxHandler.ListAuthors)))
	mux.Handle("GET /api/v1/genres", auth(http.HandlerFunc(taxHandler.ListGenres)))
	mux.Handle("GET /api/v1/series", auth(http.HandlerFunc(taxHandler.ListSeries)))

	mux.Handle("POST /api/v1/library/scan", auth(http.HandlerFunc(libHandler.Scan)))

	return CORSMiddleware(mux)
}
