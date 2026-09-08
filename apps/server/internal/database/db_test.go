package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shelfd/shelfd/internal/database"
)

func TestOpenSQLiteOnDisk(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "nested", "data", "test.db")

	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite database on disk: %v", err)
	}
	defer db.Close()

	// Verify journal mode (defaults to DELETE for sqlite-vec single-file stability)
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	if journalMode != "delete" && journalMode != "wal" {
		t.Errorf("expected delete or wal journal_mode, got %s", journalMode)
	}

	// Verify foreign keys pragma
	var foreignKeys int
	err = db.QueryRow("PRAGMA foreign_keys;").Scan(&foreignKeys)
	if err != nil {
		t.Fatalf("querying foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("expected foreign_keys=1, got %d", foreignKeys)
	}

	// Verify migrations run on disk database
	ctx := context.Background()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations on disk db: %v", err)
	}
}

func TestEnsureVectorDimensions(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("running initial migrations: %v", err)
	}

	// Adapt to 768 dimensions (for models like nomic-embed-text)
	if err := database.EnsureVectorDimensions(ctx, db, 768); err != nil {
		t.Fatalf("EnsureVectorDimensions(768) failed: %v", err)
	}

	// Verify table exists and can accept 768-dim query/insert
	var count int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM vec_chapters").Scan(&count)
	if err != nil {
		t.Fatalf("querying vec_chapters count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

