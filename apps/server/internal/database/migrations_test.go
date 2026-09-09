package database_test

import (
	"context"
	"testing"

	"github.com/shelfd/shelfd/internal/database"
)

func TestRunMigrations(t *testing.T) {
	ctx := context.Background()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify idempotency
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations second time (idempotency check): %v", err)
	}

	// Verify tables exist
	expectedTables := []string{
		"users",
		"api_tokens",
		"authors",
		"genres",
		"series",
		"books",
		"book_authors",
		"book_genres",
		"book_series",
		"chapters",
		"paragraphs",
		"vec_paragraphs",
		"bookmarks",
		"highlights",
		"schema_migrations",
	}

	for _, table := range expectedTables {
		var name string
		err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type IN ('table', 'shadow') AND name = ?", table).Scan(&name)
		if err != nil {
			// Also check virtual table naming in sqlite_master
			err = db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE name = ?", table).Scan(&name)
			if err != nil {
				t.Errorf("expected table %s to exist in sqlite_master, error: %v", table, err)
			}
		}
	}
}
