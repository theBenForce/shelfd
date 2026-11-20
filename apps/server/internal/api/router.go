package api

import (
	"log/slog"
	"net/http"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/jobs"
	"github.com/shelfd/shelfd/internal/plugins"
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
	ToolInvoker     ai.ToolInvoker
	PluginHost      *plugins.Host
	Hub             *events.Hub
	JobManager      *jobs.Manager
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

	jobManager := cfg.JobManager
	if jobManager == nil {
		jobManager = jobs.NewManager(cfg.Hub)
	}

	authHandler := NewAuthHandler(cfg.Repo, cfg.JWTSecret, logger)
	bookHandler := NewBookHandler(cfg.Repo, cfg.Ingester, cfg.Worker, cfg.UploadWorker, cfg.AIClient, cfg.DataDir, cfg.LibraryDir, cfg.Hub, logger)
	if cfg.ToolInvoker != nil {
		bookHandler.SetToolInvoker(cfg.ToolInvoker)
	}
	taxHandler := NewTaxonomyHandler(cfg.Repo, cfg.LibraryDir, cfg.DataDir, logger)
	libHandler := NewLibraryHandler(cfg.Repo, cfg.Scanner, cfg.Ingester, cfg.Worker, cfg.Hub, jobManager, logger)
	connHandler := NewConnectHandler(cfg.Host, cfg.Port, cfg.Version, cfg.DefaultUsername)
	queueHandler := NewQueueHandler(cfg.Repo, cfg.Worker, cfg.UploadWorker, cfg.Hub)
	jobsHandler := NewJobsHandler(jobManager)
	utilHandler := NewUtilityHandler(cfg.Repo, cfg.DataDir, cfg.LibraryDir, cfg.Hub, logger)
	audioHandler := NewAudiobookHandler(cfg.Repo, cfg.DataDir, cfg.LibraryDir, logger)
	metadataHandler := NewMetadataHandler(cfg.Repo, cfg.DataDir, cfg.LibraryDir, cfg.Hub, logger)
	pluginHandler := NewPluginHandler(cfg.PluginHost, logger)

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

	// Plugin system routes
	mux.Handle("GET /api/v1/plugins", auth(http.HandlerFunc(pluginHandler.ListPlugins)))
	mux.Handle("POST /api/v1/plugins/install", auth(http.HandlerFunc(pluginHandler.InstallPlugin)))
	mux.Handle("POST /api/v1/plugins/discover", auth(http.HandlerFunc(pluginHandler.DiscoverPlugins)))
	mux.Handle("GET /api/v1/plugins/jobs", auth(http.HandlerFunc(pluginHandler.ListJobs)))
	mux.Handle("DELETE /api/v1/plugins/jobs/{id}", auth(http.HandlerFunc(pluginHandler.DismissJob)))
	mux.Handle("GET /api/v1/plugins/{name}", auth(http.HandlerFunc(pluginHandler.GetPlugin)))
	mux.Handle("POST /api/v1/plugins/{name}/update", auth(http.HandlerFunc(pluginHandler.UpdatePlugin)))
	mux.Handle("POST /api/v1/plugins/{name}/enable", auth(http.HandlerFunc(pluginHandler.EnablePlugin)))
	mux.Handle("POST /api/v1/plugins/{name}/disable", auth(http.HandlerFunc(pluginHandler.DisablePlugin)))
	mux.Handle("GET /api/v1/plugins/{name}/config", auth(http.HandlerFunc(pluginHandler.GetConfig)))
	mux.Handle("PUT /api/v1/plugins/{name}/config", auth(http.HandlerFunc(pluginHandler.UpdateConfig)))
	mux.Handle("GET /api/v1/plugins/{name}/auth/status", auth(http.HandlerFunc(pluginHandler.GetAuthStatus)))
	mux.Handle("POST /api/v1/plugins/{name}/auth/begin", auth(http.HandlerFunc(pluginHandler.BeginAuth)))
	mux.Handle("POST /api/v1/plugins/{name}/auth/complete", auth(http.HandlerFunc(pluginHandler.CompleteAuth)))
	mux.Handle("POST /api/v1/plugins/{name}/auth/disconnect", auth(http.HandlerFunc(pluginHandler.DisconnectAuth)))
	mux.Handle("DELETE /api/v1/plugins/{name}/auth/disconnect", auth(http.HandlerFunc(pluginHandler.DisconnectAuth)))
	mux.Handle("POST /api/v1/plugins/{name}/commands/{command}", auth(http.HandlerFunc(pluginHandler.InvokeCommand)))
	mux.Handle("POST /api/v1/plugins/{name}/actions/{action}/prepare", auth(http.HandlerFunc(pluginHandler.PrepareAction)))
	mux.Handle("GET /api/v1/plugins/{name}/actions/{action}/options", auth(http.HandlerFunc(pluginHandler.GetActionOptions)))
	mux.Handle("POST /api/v1/plugins/{name}/actions/{action}/assets", auth(http.HandlerFunc(pluginHandler.UploadActionAsset)))
	mux.Handle("POST /api/v1/plugins/{name}/actions/{action}/invoke", auth(http.HandlerFunc(pluginHandler.InvokeAction)))
	mux.Handle("GET /api/v1/plugins/{name}/catalog", auth(http.HandlerFunc(pluginHandler.GetCatalog)))
	mux.Handle("POST /api/v1/plugins/{name}/catalog/stage", auth(http.HandlerFunc(pluginHandler.StageCatalogItems)))

	// Metadata search & auto-populate
	mux.Handle("GET /api/v1/metadata/search", auth(http.HandlerFunc(metadataHandler.Search)))

	mux.Handle("GET /api/v1/books", auth(http.HandlerFunc(bookHandler.ListBooks)))
	mux.Handle("POST /api/v1/books/upload", auth(http.HandlerFunc(bookHandler.UploadBook)))
	mux.Handle("POST /api/v1/books/upload/stage", auth(http.HandlerFunc(bookHandler.StageUploadBook)))
	mux.Handle("GET /api/v1/books/upload/jobs", auth(http.HandlerFunc(bookHandler.ListUploadJobs)))
	mux.Handle("GET /api/v1/books/upload/jobs/{id}", auth(http.HandlerFunc(bookHandler.GetUploadJob)))
	mux.Handle("GET /api/v1/books/upload/jobs/{id}/cover", auth(http.HandlerFunc(bookHandler.GetUploadJobCover)))
	mux.Handle("POST /api/v1/books/upload/jobs/{id}/cover", auth(http.HandlerFunc(bookHandler.UploadJobCover)))
	mux.Handle("POST /api/v1/books/upload/jobs/{id}/cover/fetch", auth(http.HandlerFunc(metadataHandler.FetchJobCover)))
	mux.Handle("POST /api/v1/books/upload/jobs/{id}/commit", auth(http.HandlerFunc(bookHandler.CommitUploadJob)))
	mux.Handle("DELETE /api/v1/books/upload/jobs/{id}", auth(http.HandlerFunc(bookHandler.DeleteUploadJob)))
	mux.Handle("GET /api/v1/books/{id}", auth(http.HandlerFunc(bookHandler.GetBook)))
	mux.Handle("PUT /api/v1/books/{id}/metadata", auth(http.HandlerFunc(bookHandler.UpdateBookMetadata)))
	mux.Handle("POST /api/v1/books/{id}/cover", auth(http.HandlerFunc(bookHandler.UploadBookCover)))
	mux.Handle("POST /api/v1/books/{id}/cover/fetch", auth(http.HandlerFunc(metadataHandler.FetchBookCover)))
	mux.Handle("GET /api/v1/books/{id}/download", auth(http.HandlerFunc(bookHandler.DownloadBook)))
	mux.Handle("GET /api/v1/books/{id}/markdown", auth(http.HandlerFunc(bookHandler.ExportBookMarkdown)))
	mux.Handle("GET /api/v1/books/{id}/cover", auth(http.HandlerFunc(bookHandler.GetBookCover)))
	mux.Handle("GET /api/v1/books/{id}/readalong", auth(http.HandlerFunc(bookHandler.GetBookReadalong)))
	mux.Handle("GET /api/v1/books/{id}/assets/{path...}", auth(http.HandlerFunc(bookHandler.GetBookAsset)))
	mux.Handle("GET /api/v1/books/{id}/chapters/{index}/html", auth(http.HandlerFunc(bookHandler.GetChapterHTML)))
	mux.Handle("POST /api/v1/books/{id}/reparse", auth(http.HandlerFunc(bookHandler.ReparseBook)))
	mux.Handle("POST /api/v1/books/{id}/chat", auth(http.HandlerFunc(bookHandler.ChatBook)))
	mux.Handle("GET /api/v1/books/{id}/chapters/{index}", auth(http.HandlerFunc(bookHandler.GetChapter)))
	mux.Handle("GET /api/v1/chapters/{id}", auth(http.HandlerFunc(bookHandler.GetChapterDirect)))

	// Audiobook streaming & management
	mux.Handle("GET /api/v1/audiobooks/{id}/stream", auth(http.HandlerFunc(audioHandler.StreamAudio)))
	mux.Handle("HEAD /api/v1/audiobooks/{id}/stream", auth(http.HandlerFunc(audioHandler.StreamAudio)))
	mux.Handle("GET /api/v1/books/{id}/audio", auth(http.HandlerFunc(audioHandler.StreamAudio)))
	mux.Handle("HEAD /api/v1/books/{id}/audio", auth(http.HandlerFunc(audioHandler.StreamAudio)))
	mux.Handle("GET /api/v1/audiobooks/{id}/chapters", auth(http.HandlerFunc(audioHandler.GetChapters)))
	mux.Handle("GET /api/v1/audiobooks/{id}/progress", auth(http.HandlerFunc(audioHandler.GetProgress)))
	mux.Handle("POST /api/v1/audiobooks/{id}/progress", auth(http.HandlerFunc(audioHandler.UpdateProgress)))
	mux.Handle("DELETE /api/v1/audiobooks/{id}/progress", auth(http.HandlerFunc(audioHandler.DeleteProgress)))
	mux.Handle("GET /api/v1/books/{id}/files/{fileId}/stream", auth(http.HandlerFunc(audioHandler.StreamBookFile)))
	mux.Handle("HEAD /api/v1/books/{id}/files/{fileId}/stream", auth(http.HandlerFunc(audioHandler.StreamBookFile)))

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
	mux.Handle("GET /api/v1/series/{id}", auth(http.HandlerFunc(taxHandler.GetSeries)))

	mux.Handle("GET /api/v1/queue/status", auth(http.HandlerFunc(queueHandler.GetStatus)))
	mux.Handle("GET /api/v1/queue/events", auth(http.HandlerFunc(queueHandler.StreamEvents)))

	// Unified background jobs routes
	mux.Handle("GET /api/v1/jobs", auth(http.HandlerFunc(jobsHandler.ListJobs)))
	mux.Handle("DELETE /api/v1/jobs", auth(http.HandlerFunc(jobsHandler.ClearFinishedJobs)))
	mux.Handle("DELETE /api/v1/jobs/{id}", auth(http.HandlerFunc(jobsHandler.DismissJob)))

	mux.Handle("GET /api/v1/utilities/duplicates", auth(http.HandlerFunc(utilHandler.FindDuplicates)))
	mux.Handle("POST /api/v1/utilities/merge-books", auth(http.HandlerFunc(utilHandler.MergeBooks)))

	mux.Handle("POST /api/v1/library/scan", auth(http.HandlerFunc(libHandler.Scan)))

	return CORSMiddleware(mux)
}
