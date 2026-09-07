package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/shelfd/shelfd/internal/config"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
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

	if cfg.Database.Type == "sqlite" {
		log.Printf("Opening SQLite database at %s...", cfg.Database.SQLite.Path)
		db, err := database.OpenSQLite(cfg.Database.SQLite.Path)
		if err != nil {
			log.Fatalf("Failed to open SQLite database: %v", err)
		}
		defer db.Close()

		ctx := context.Background()
		log.Printf("Executing database schema migrations...")
		if err := database.RunMigrations(ctx, db); err != nil {
			log.Fatalf("Failed to execute migrations: %v", err)
		}

		repo := repository.NewSQLiteStorageEngine(db)
		defer repo.Close()
		log.Printf("Storage engine initialized successfully.")

		if *scanFlag {
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
			return
		}
	} else {
		log.Fatalf("Database type '%s' is not supported in Phase 1 MVP", cfg.Database.Type)
	}

	log.Printf("Core daemon initialized (Milestone 2 ready).")
}
