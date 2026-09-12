package api

import (
	"log/slog"
	"net/http"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

// RouterConfig contains dependencies for assembling the Shelfd REST API.
type RouterConfig struct {
	Repo            repository.StorageEngine
	Ingester        *scanner.Ingester
	Scanner         *scanner.Scanner
	Worker          *worker.Worker
	UploadWorker    *worker.UploadWorker
	AIClient        ai.Client
	Hub             *events.Hub
	DataDir         string
	LibraryDir      string
	JWTSecret       string
	Host            string
	Port            int
	Version         string
	DefaultUsername string
	Logger          *slog.Logger
}

// NewRouter constructs and returns the fully configured /api/v1 HTTP handler.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	authHandler := NewAuthHandler(cfg.Repo, cfg.JWTSecret, logger)
	bookHandler := NewBookHandler(cfg.Repo, cfg.Ingester, cfg.Worker, cfg.UploadWorker, cfg.AIClient, cfg.DataDir, cfg.LibraryDir, cfg.Hub, logger)
	taxHandler := NewTaxonomyHandler(cfg.Repo, cfg.LibraryDir, cfg.DataDir, logger)
	libHandler := NewLibraryHandler(cfg.Repo, cfg.Scanner, cfg.Ingester, cfg.Worker, cfg.Hub, logger)
	connHandler := NewConnectHandler(cfg.Host, cfg.Port, cfg.Version, cfg.DefaultUsername)
	queueHandler := NewQueueHandler(cfg.Repo, cfg.Worker, cfg.UploadWorker, cfg.Hub)

	auth := AuthMiddleware(cfg.Repo, cfg.JWTSecret)

	// Public routes
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/v1/server/connect-info", connHandler.GetConnectInfo)

	// Protected routes
	mux.Handle("POST /api/v1/auth/logout", auth(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /api/v1/auth/me", auth(http.HandlerFunc(authHandler.Me)))
	mux.Handle("POST /api/v1/auth/change-password", auth(http.HandlerFunc(authHandler.ChangePassword)))
	mux.Handle("POST /api/v1/auth/tokens", auth(http.HandlerFunc(authHandler.CreateToken)))
	mux.Handle("GET /api/v1/auth/tokens", auth(http.HandlerFunc(authHandler.ListTokens)))
	mux.Handle("DELETE /api/v1/auth/tokens/{id}", auth(http.HandlerFunc(authHandler.DeleteToken)))

	mux.Handle("GET /api/v1/books", auth(http.HandlerFunc(bookHandler.ListBooks)))
	mux.Handle("POST /api/v1/books/upload", auth(http.HandlerFunc(bookHandler.UploadBook)))
	mux.Handle("POST /api/v1/books/upload/stage", auth(http.HandlerFunc(bookHandler.StageUploadBook)))
	mux.Handle("GET /api/v1/books/upload/jobs", auth(http.HandlerFunc(bookHandler.ListUploadJobs)))
	mux.Handle("GET /api/v1/books/upload/jobs/{id}", auth(http.HandlerFunc(bookHandler.GetUploadJob)))
	mux.Handle("GET /api/v1/books/upload/jobs/{id}/cover", auth(http.HandlerFunc(bookHandler.GetUploadJobCover)))
	mux.Handle("POST /api/v1/books/upload/jobs/{id}/cover", auth(http.HandlerFunc(bookHandler.UploadJobCover)))
	mux.Handle("POST /api/v1/books/upload/jobs/{id}/commit", auth(http.HandlerFunc(bookHandler.CommitUploadJob)))
	mux.Handle("DELETE /api/v1/books/upload/jobs/{id}", auth(http.HandlerFunc(bookHandler.DeleteUploadJob)))
	mux.Handle("GET /api/v1/books/{id}", auth(http.HandlerFunc(bookHandler.GetBook)))
	mux.Handle("GET /api/v1/books/{id}/cover", auth(http.HandlerFunc(bookHandler.GetBookCover)))
	mux.Handle("GET /api/v1/books/{id}/assets/{path...}", auth(http.HandlerFunc(bookHandler.GetBookAsset)))
	mux.Handle("GET /api/v1/books/{id}/chapters/{index}/html", auth(http.HandlerFunc(bookHandler.GetChapterHTML)))
	mux.Handle("POST /api/v1/books/{id}/reparse", auth(http.HandlerFunc(bookHandler.ReparseBook)))
	mux.Handle("POST /api/v1/books/{id}/chat", auth(http.HandlerFunc(bookHandler.ChatBook)))
	mux.Handle("GET /api/v1/books/{id}/chapters/{index}", auth(http.HandlerFunc(bookHandler.GetChapter)))
	mux.Handle("GET /api/v1/chapters/{id}", auth(http.HandlerFunc(bookHandler.GetChapterDirect)))

	mux.Handle("GET /api/v1/books/{id}/bookmarks", auth(http.HandlerFunc(bookHandler.ListBookmarks)))
	mux.Handle("POST /api/v1/books/{id}/bookmarks", auth(http.HandlerFunc(bookHandler.CreateBookmark)))
	mux.Handle("DELETE /api/v1/books/{id}/bookmarks/{bookmarkId}", auth(http.HandlerFunc(bookHandler.DeleteBookmark)))
	mux.Handle("DELETE /api/v1/bookmarks/{bookmarkId}", auth(http.HandlerFunc(bookHandler.DeleteBookmark)))

	mux.Handle("GET /api/v1/books/{id}/highlights", auth(http.HandlerFunc(bookHandler.ListHighlights)))
	mux.Handle("POST /api/v1/books/{id}/highlights", auth(http.HandlerFunc(bookHandler.CreateHighlight)))
	mux.Handle("DELETE /api/v1/books/{id}/highlights/{highlightId}", auth(http.HandlerFunc(bookHandler.DeleteHighlight)))
	mux.Handle("DELETE /api/v1/highlights/{highlightId}", auth(http.HandlerFunc(bookHandler.DeleteHighlight)))

	mux.Handle("GET /api/v1/search", auth(http.HandlerFunc(bookHandler.SearchLibrary)))

	mux.Handle("GET /api/v1/authors", auth(http.HandlerFunc(taxHandler.ListAuthors)))
	mux.Handle("GET /api/v1/authors/{id}/photo", auth(http.HandlerFunc(taxHandler.GetAuthorPhoto)))
	mux.Handle("POST /api/v1/authors/{id}/photo", auth(http.HandlerFunc(taxHandler.UploadAuthorPhoto)))
	mux.Handle("GET /api/v1/genres", auth(http.HandlerFunc(taxHandler.ListGenres)))
	mux.Handle("GET /api/v1/topics", auth(http.HandlerFunc(taxHandler.ListTopics)))
	mux.Handle("GET /api/v1/series", auth(http.HandlerFunc(taxHandler.ListSeries)))

	mux.Handle("GET /api/v1/queue/status", auth(http.HandlerFunc(queueHandler.GetStatus)))
	mux.Handle("GET /api/v1/queue/events", auth(http.HandlerFunc(queueHandler.StreamEvents)))

	mux.Handle("POST /api/v1/library/scan", auth(http.HandlerFunc(libHandler.Scan)))

	return CORSMiddleware(mux)
}
