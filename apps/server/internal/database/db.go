package database

import (
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
		dsn += "?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	// For in-memory databases, retain a single connection to preserve tables across queries
	if path == ":memory:" || strings.Contains(path, "mode=memory") {
		db.SetMaxOpenConns(1)
	}

	// Ensure foreign key constraints are strictly active
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling foreign_keys pragma: %w", err)
	}

	return db, nil
}
