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
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/shelfd/shelfd/internal/config"
)

func init() {
	sqlite_vec.Auto()
}

// OpenDB opens a Bun database connection based on the provided configuration.
func OpenDB(cfg *config.Config) (*bun.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	switch cfg.Database.Type {
	case "postgres":
		return OpenBunPostgres(cfg.Database.Postgres.DSN)
	case "sqlite":
		fallthrough
	default:
		return OpenBunSQLite(cfg.Database.SQLite.Path)
	}
}

// OpenBunSQLite opens an SQLite database wrapped in Bun with sqlite-vec registered.
func OpenBunSQLite(path string) (*bun.DB, error) {
	sqldb, err := OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	return bun.NewDB(sqldb, sqlitedialect.New()), nil
}

// OpenBunPostgres opens a PostgreSQL database wrapped in Bun using pgdriver.
func OpenBunPostgres(dsn string) (*bun.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("postgres dsn cannot be empty")
	}

	connector := pgdriver.NewConnector(pgdriver.WithDSN(dsn))
	sqldb := sql.OpenDB(connector)

	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(5)

	db := bun.NewDB(sqldb, pgdialect.New())
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	return db, nil
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

// QueryExecer abstracts query and exec operations across *sql.DB and *bun.DB.
type QueryExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// EnsureVectorDimensions dynamically ensures the vec_paragraphs virtual table matches the configured vector dimensions.
// For PostgreSQL, it verifies or provisions pgvector with HNSW index.
func EnsureVectorDimensions(ctx context.Context, db QueryExecer, dimensions int) error {
	if dimensions <= 0 {
		dimensions = 256
	}

	if bdb, ok := db.(*bun.DB); ok && bdb.Dialect().Name() == dialect.PG {
		_, err := db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector;")
		if err != nil {
			return fmt.Errorf("enabling pgvector extension: %w", err)
		}

		query := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS vec_paragraphs (
				paragraph_id TEXT PRIMARY KEY REFERENCES paragraphs(id) ON DELETE CASCADE,
				embedding vector(%d)
			);
			CREATE INDEX IF NOT EXISTS idx_vec_paragraphs_hnsw 
			ON vec_paragraphs USING hnsw (embedding vector_cosine_ops);
		`, dimensions)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("ensuring pgvector table and hnsw index with %d dimensions: %w", dimensions, err)
		}
		return nil
	}

	// SQLite
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

