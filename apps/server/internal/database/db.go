package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	sqlite_vec.Auto()
}

// OpenSQLite opens an SQLite database with sqlite-vec registered, foreign keys enabled, and WAL mode.
func OpenSQLite(path string) (*sql.DB, error) {
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating database directory %s: %w", dir, err)
		}
	}

	dsn := path
	if !strings.Contains(dsn, "?") {
		journalMode := os.Getenv("SHELFD_SQLITE_JOURNAL_MODE")
		if journalMode == "" {
			journalMode = "DELETE"
		}
		dsn += fmt.Sprintf("?_foreign_keys=on&_busy_timeout=10000&_journal_mode=%s", journalMode)
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	// Single connection prevents multi-connection WAL lock contention and sqlite-vec shadow table race conditions
	db.SetMaxOpenConns(1)

	// Ensure foreign key constraints are strictly active
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling foreign_keys pragma: %w", err)
	}

	return db, nil
}

// EnsureVectorDimensions dynamically ensures the vec_paragraphs virtual table matches the configured vector dimensions.
// If the table is empty or uninitialized, it recreates it with the target dimension.
func EnsureVectorDimensions(ctx context.Context, db *sql.DB, dimensions int) error {
	if dimensions <= 0 {
		dimensions = 256
	}

	var count int
	err := db.QueryRowContext(ctx, "SELECT count(*) FROM vec_paragraphs").Scan(&count)
	if err == nil && count > 0 {
		// Existing vector data exists; preserve populated table
		return nil
	}

	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS vec_paragraphs;")
	query := fmt.Sprintf("CREATE VIRTUAL TABLE IF NOT EXISTS vec_paragraphs USING vec0(paragraph_id TEXT PRIMARY KEY, embedding float[%d]);", dimensions)
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("creating vec_paragraphs virtual table with %d dimensions: %w", dimensions, err)
	}

	triggerQuery := `
		CREATE TRIGGER IF NOT EXISTS trg_delete_paragraph_vec AFTER DELETE ON paragraphs
		BEGIN
			DELETE FROM vec_paragraphs WHERE paragraph_id = OLD.id;
		END;
	`
	if _, err := db.ExecContext(ctx, triggerQuery); err != nil {
		return fmt.Errorf("creating trg_delete_paragraph_vec trigger: %w", err)
	}

	return nil
}

