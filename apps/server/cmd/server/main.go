package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/config"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
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
	flag.Parse()

	configPath := *configFlag
	if configPath == "" {
		configPath = os.Getenv("SHELFD_CONFIG")
	}
	if configPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		} else if _, err := os.Stat("/config.yaml"); err == nil {
			configPath = "/config.yaml"
		}
	}

	var cfg *config.Config
	var err error
	if configPath != "" {
		log.Printf("Loading configuration from %s...", configPath)
		cfg, err = config.Load(configPath)
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}
	} else {
		log.Printf("No config.yaml found, using defaults...")
		cfg = config.DefaultConfig()
	}

	log.Printf("shelfd %s starting on %s:%d", Version, cfg.Server.Host, cfg.Server.Port)
	log.Printf("Library directory: %s | Data directory: %s", cfg.Storage.LibraryDir, cfg.Storage.DataDir)

	if cfg.Database.Type != "sqlite" {
		log.Fatalf("Database type '%s' is not supported in Phase 1 MVP", cfg.Database.Type)
	}

	log.Printf("Opening SQLite database at %s...", cfg.Database.SQLite.Path)
	db, err := database.OpenSQLite(cfg.Database.SQLite.Path)
	if err != nil {
		log.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Executing database schema migrations...")
	if err := database.RunMigrations(ctx, db); err != nil {
		log.Fatalf("Failed to execute migrations: %v", err)
	}

	repo := repository.NewSQLiteStorageEngine(db)
	defer repo.Close()
	log.Printf("Storage engine initialized successfully.")

	// One-off scan mode
	if *scanFlag {
		runScan(ctx, repo, cfg)
		return
	}

	// Initialize AI client
	aiClient, err := ai.NewClient(&cfg.AI)
	if err != nil {
		log.Fatalf("Failed to initialize AI client: %v", err)
	}
	log.Printf("AI client initialized (Provider: %s, BaseURL: %s)", cfg.AI.Provider, cfg.AI.BaseURL)

	// Ensure seed admin user and MCP token exist if no tokens are configured
	ensureSeedToken(ctx, repo)

	// Start background indexing worker
	chapterWorker := worker.NewWorker(repo, aiClient, worker.Config{
		BatchSize:    10,
		PollInterval: 5 * time.Second,
	})
	chapterWorker.Start(ctx)
	defer chapterWorker.Stop()
	log.Printf("Semantic indexing background worker started.")

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
		})
		mux.Handle(cfg.MCP.Path+"/", mcpServer.Routes())
		log.Printf("MCP Server enabled at %s/sse and %s/messages", cfg.MCP.Path, cfg.MCP.Path)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown handling
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Shelfd daemon listening on http://%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-stopSig
	log.Printf("Shutting down Shelfd daemon gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Printf("Shelfd daemon stopped.")
}

func runScan(ctx context.Context, repo repository.StorageEngine, cfg *config.Config) {
	log.Printf("Starting library scan in %s...", cfg.Storage.LibraryDir)
	s := scanner.NewScanner(cfg.Storage.LibraryDir)
	discovered, err := s.Scan()
	if err != nil {
		log.Fatalf("Library scan failed: %v", err)
	}
	log.Printf("Discovered %d EPUB files.", len(discovered))

	ingester := scanner.NewIngester(repo, cfg.Storage.LibraryDir, cfg.Storage.DataDir)
	for _, f := range discovered {
		book, err := ingester.IngestFile(ctx, f.FullPath, f.RelativePath)
		if err != nil {
			log.Printf("Failed to ingest %s: %v", f.RelativePath, err)
			continue
		}
		log.Printf("Successfully cataloged: %s (ID: %s)", book.Title, book.ID)
	}
	log.Printf("Scan and ingestion complete.")
}

func ensureSeedToken(ctx context.Context, repo repository.StorageEngine) {
	adminUser, err := repo.GetUserByUsername(ctx, "admin")
	if errors.Is(err, repository.ErrNotFound) {
		adminUser = &repository.User{
			Username:     "admin",
			PasswordHash: "$2a$12$e8Yd8mockhashnotforlogin",
		}
		if err := repo.CreateUser(ctx, adminUser); err != nil {
			log.Printf("Warning: failed to create admin user: %v", err)
			return
		}
	} else if err != nil {
		return
	}

	// Check if any API token exists for admin
	// Generate random 32-byte secret token
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return
	}
	rawToken := "shelfd_" + hex.EncodeToString(buf)
	tokenHash := mcp.HashToken(rawToken)

	// Attempt lookup by hash - if missing, seed token
	if _, err := repo.GetAPITokenByHash(ctx, tokenHash); errors.Is(err, repository.ErrNotFound) {
		token := &repository.APIToken{
			UserID:    adminUser.ID,
			TokenHash: tokenHash,
			Name:      "Initial Setup Token",
		}
		if err := repo.CreateAPIToken(ctx, token); err == nil {
			log.Printf("================================================================================")
			log.Printf(" [INITIAL SETUP] Generated default MCP API token:")
			log.Printf(" Token: %s", rawToken)
			log.Printf(" Use in Claude Desktop / Cursor: 'Authorization: Bearer %s'", rawToken)
			log.Printf("================================================================================")
		}
	}
}
