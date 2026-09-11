package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/config"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/taxonomy"
	"github.com/shelfd/shelfd/internal/worker"
)

var (
	Version = "0.1.0-dev"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("shelfd %s\n", Version)
		return
	}

	configFlag := flag.String("config", "", "Path to config.yaml")
	scanFlag := flag.Bool("scan", false, "Scan library directory for books and exit")
	migrateTopicsFlag := flag.Bool("migrate-topics", false, "Migrate existing book genres to topics and canonical genres using AI and exit")
	forceFlag := flag.Bool("force", false, "Force topic migration even for books that already have topics")
	flag.Parse()

	envPath := config.LoadDotEnv()

	configPath := config.ResolveConfigPath(*configFlag)

	var cfg *config.Config
	var err error
	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Apply any SHELFD_* environment variable overrides
	config.ApplyEnvOverrides(cfg)

	// Set up structured slog logger based on configuration
	var logLevel slog.Level
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Logging.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	if envPath != "" {
		logger.Info("Loaded environment variables from file", "path", envPath)
	}

	if configPath != "" {
		logger.Info("Loaded configuration", "path", configPath)
	} else {
		logger.Info("No config file found, using defaults")
	}

	if cfg.Server.JWTSecret == "" {
		secretBytes := make([]byte, 32)
		_, _ = rand.Read(secretBytes)
		cfg.Server.JWTSecret = hex.EncodeToString(secretBytes)
		logger.Warn("server.jwt_secret not configured; generated ephemeral JWT secret")
	}

	logger.Info("shelfd starting", "version", Version, "host", cfg.Server.Host, "port", cfg.Server.Port)
	logger.Info("Storage directories initialized", "library_dir", cfg.Storage.LibraryDir, "data_dir", cfg.Storage.DataDir)

	logger.Info("Opening database", "type", cfg.Database.Type)
	bunDB, err := database.OpenDB(cfg)
	if err != nil {
		logger.Error("Failed to open database", "type", cfg.Database.Type, "error", err)
		os.Exit(1)
	}
	defer bunDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("Executing database schema migrations")
	if err := database.RunBunMigrations(ctx, bunDB); err != nil {
		logger.Error("Failed to execute migrations", "error", err)
		os.Exit(1)
	}

	if err := database.EnsureVectorDimensions(ctx, bunDB, cfg.AI.EmbeddingDimensions); err != nil {
		logger.Error("Failed to configure vector dimensions", "error", err)
		os.Exit(1)
	}

	repo := repository.NewBunStorageEngine(bunDB)
	defer repo.Close()
	logger.Info("Storage engine initialized successfully", "type", cfg.Database.Type)

	// Backfill paragraphs for any existing chapters lacking passage chunks
	if backfilled, err := repo.BackfillParagraphs(ctx); err == nil && backfilled > 0 {
		logger.Info("Backfilled paragraphs for existing chapters", "count", backfilled)
	}

	// Initialize AI client
	aiClient, err := ai.NewClient(&cfg.AI)
	if err != nil {
		logger.Error("Failed to initialize AI client", "error", err)
		os.Exit(1)
	}
	logger.Info("AI client initialized", "provider", cfg.AI.Provider, "base_url", cfg.AI.BaseURL)

	taxService := taxonomy.NewTaxonomyService(repo, aiClient, logger)

	// One-off scan mode
	if *scanFlag {
		runScan(ctx, repo, cfg, taxService, logger)
		return
	}

	// One-off topic migration mode
	if *migrateTopicsFlag {
		logger.Info("Starting library topics migration", "force", *forceFlag)
		count, err := taxService.MigrateLibraryTopics(ctx, *forceFlag)
		if err != nil {
			logger.Error("Library topics migration failed", "error", err)
			os.Exit(1)
		}
		logger.Info("Library topics migration completed successfully", "migrated_books", count)
		return
	}

	// Ensure seed admin user and MCP token exist if no tokens are configured
	ensureSeedToken(ctx, repo, cfg, logger)

	// Start background indexing worker
	chapterWorker := worker.NewWorker(repo, aiClient, worker.Config{
		BatchSize:    50,
		PollInterval: 5 * time.Second,
		Logger:       logger,
	})
	chapterWorker.Start(ctx)
	defer chapterWorker.Stop()
	logger.Info("Semantic indexing background worker started")

	// Scanner and ingester
	scannerInst := scanner.NewScanner(cfg.Storage.LibraryDir)
	ingester := scanner.NewIngester(repo, cfg.Storage.LibraryDir, cfg.Storage.DataDir)
	ingester.SetTaxonomyNormalizer(taxService)

	// Event Hub for real-time SSE broadcasts
	eventHub := events.NewHub()

	// Start background upload worker
	uploadWorker := worker.NewUploadWorker(repo, ingester, chapterWorker, worker.UploadWorkerConfig{
		PollInterval: 3 * time.Second,
		Hub:          eventHub,
		Logger:       logger,
	})
	uploadWorker.Start(ctx)
	defer uploadWorker.Stop()
	logger.Info("Asynchronous book upload queue worker started")

	// Register HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","version":"` + Version + `"}`))
	})

	if cfg.MCP.Enabled {
		mcpServer := mcp.NewServer(repo, aiClient, mcp.Config{
			BasePath: cfg.MCP.Path,
			Logger:   logger,
		})
		mcpHandler := api.CORSMiddleware(mcpServer.Routes())
		mux.Handle(cfg.MCP.Path, mcpHandler)
		mux.Handle(cfg.MCP.Path+"/", mcpHandler)
		logger.Info("MCP Server enabled", "path", cfg.MCP.Path)
	}

	// OAuth 2.0 & RFC 7591 Dynamic Client Registration
	oauthHandler := api.NewOAuthHandler(repo)
	oauthHandler.RegisterRoutes(mux)
	logger.Info("OAuth 2.0 & RFC 7591 Dynamic Registration enabled")

	// REST API Router
	apiRouter := api.NewRouter(api.RouterConfig{
		Repo:            repo,
		Ingester:        ingester,
		Scanner:         scannerInst,
		Worker:          chapterWorker,
		UploadWorker:    uploadWorker,
		AIClient:        aiClient,
		Hub:             eventHub,
		DataDir:         cfg.Storage.DataDir,
		LibraryDir:      cfg.Storage.LibraryDir,
		JWTSecret:       cfg.Server.JWTSecret,
		Host:            cfg.Server.Host,
		Port:            cfg.Server.Port,
		Version:         Version,
		DefaultUsername: cfg.Auth.AdminUsername,
		Logger:          logger,
	})
	mux.Handle("/api/v1/", apiRouter)
	logger.Info("REST API enabled at /api/v1/")

	// Static Flutter web application
	webDir := resolveWebDir(cfg.Server.WebDir)
	if webDir != "" {
		mux.HandleFunc("/", api.SPAHandler(webDir))
		logger.Info("Serving Flutter web application", "path", webDir)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	rootHandler := api.RequestLoggerMiddleware(logger)(api.CORSMiddleware(mux))
	server := &http.Server{
		Addr:    addr,
		Handler: rootHandler,
	}

	// Graceful shutdown handling
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Shelfd daemon listening", "addr", "http://"+addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stopSig
	logger.Info("Shutting down Shelfd daemon gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}
	logger.Info("Shelfd daemon stopped")
}

func runScan(ctx context.Context, repo repository.StorageEngine, cfg *config.Config, taxService *taxonomy.TaxonomyService, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("Starting library scan", "dir", cfg.Storage.LibraryDir)
	s := scanner.NewScanner(cfg.Storage.LibraryDir)
	discovered, err := s.Scan()
	if err != nil {
		logger.Error("Library scan failed", "error", err)
		os.Exit(1)
	}
	logger.Info("Discovered EPUB files", "count", len(discovered))

	ingester := scanner.NewIngester(repo, cfg.Storage.LibraryDir, cfg.Storage.DataDir)
	if taxService != nil {
		ingester.SetTaxonomyNormalizer(taxService)
	}
	newCount := 0
	modifiedCount := 0
	unchangedCount := 0
	for _, f := range discovered {
		book, status, err := ingester.SyncFile(ctx, f.FullPath, f.RelativePath)
		if err != nil {
			logger.Error("Failed to ingest file", "path", f.RelativePath, "error", err)
			continue
		}
		switch status {
		case scanner.SyncStatusNew:
			newCount++
			logger.Info("Successfully cataloged new book", "title", book.Title, "id", book.ID)
		case scanner.SyncStatusModified:
			modifiedCount++
			logger.Info("Updated modified book", "title", book.Title, "id", book.ID)
		case scanner.SyncStatusUnchanged:
			unchangedCount++
		}
	}
	logger.Info("Scan and ingestion complete", "new", newCount, "modified", modifiedCount, "unchanged", unchangedCount)
}

func ensureSeedToken(ctx context.Context, repo repository.StorageEngine, cfg *config.Config, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	adminUsername := "admin"
	if cfg != nil && strings.TrimSpace(cfg.Auth.AdminUsername) != "" {
		adminUsername = strings.TrimSpace(cfg.Auth.AdminUsername)
	}

	adminUser, err := repo.GetUserByUsername(ctx, adminUsername)
	if errors.Is(err, repository.ErrNotFound) {
		adminPass := ""
		if cfg != nil && strings.TrimSpace(cfg.Auth.AdminPassword) != "" {
			adminPass = strings.TrimSpace(cfg.Auth.AdminPassword)
		}
		if adminPass == "" {
			adminPass = os.Getenv("SHELFD_ADMIN_PASSWORD")
		}
		if adminPass == "" {
			buf := make([]byte, 12)
			_, _ = rand.Read(buf)
			adminPass = hex.EncodeToString(buf)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			logger.Warn("Failed to hash admin password", "error", err)
			return
		}

		adminUser = &repository.User{
			Username:     adminUsername,
			PasswordHash: string(hash),
		}
		if err := repo.CreateUser(ctx, adminUser); err != nil {
			logger.Warn("Failed to create admin user", "error", err)
			return
		}

		logger.Info("================================================================================")
		logger.Info(" [INITIAL SETUP] Generated default administrator account:")
		logger.Info(fmt.Sprintf(" Username: %s", adminUsername))
		logger.Info(fmt.Sprintf(" Password: %s", adminPass))
		logger.Info(" (Set SHELFD_ADMIN_USERNAME and SHELFD_ADMIN_PASSWORD environment variables to override)")
		logger.Info("================================================================================")
	} else if err != nil {
		return
	}

	// Check if any API token exists for admin
	var rawToken string
	if envTok := os.Getenv("SHELFD_MCP_TOKEN"); envTok != "" {
		rawToken = envTok
	} else {
		buf := make([]byte, 24)
		if _, err := rand.Read(buf); err != nil {
			return
		}
		rawToken = "shelfd_" + hex.EncodeToString(buf)
	}
	tokenHash := mcp.HashToken(rawToken)

	// Attempt lookup by hash - if missing, seed token
	if _, err := repo.GetAPITokenByHash(ctx, tokenHash); errors.Is(err, repository.ErrNotFound) {
		token := &repository.APIToken{
			UserID:    adminUser.ID,
			TokenHash: tokenHash,
			Name:      "Initial Setup Token",
		}
		if err := repo.CreateAPIToken(ctx, token); err == nil {
			logger.Info("================================================================================")
			logger.Info(" [INITIAL SETUP] Generated default MCP API token:")
			logger.Info(fmt.Sprintf(" Token: %s", rawToken))
			logger.Info(fmt.Sprintf(" Use in Claude Desktop / Cursor: 'Authorization: Bearer %s'", rawToken))
			logger.Info("================================================================================")
		}
	}
}

func resolveWebDir(configured string) string {
	var candidates []string
	if strings.TrimSpace(configured) != "" {
		candidates = append(candidates, configured)
	}
	candidates = append(candidates,
		"/usr/share/shelfd/web",
		"apps/app/build/web",
		"../app/build/web",
		"./web",
	)

	for _, c := range candidates {
		indexPath := filepath.Join(c, "index.html")
		if fi, err := os.Stat(indexPath); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}
