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

	// Verify WAL mode
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected wal journal_mode, got %s", journalMode)
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
