package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migration represents an individual SQL schema migration.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// LoadEmbeddedMigrations reads all SQL migrations from the embedded filesystem, sorted by version.
func LoadEmbeddedMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("parsing migration version from %s: %w", entry.Name(), err)
		}

		data, err := migrationsFS.ReadFile(filepath.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading migration file %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    entry.Name(),
			SQL:     string(data),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// RunMigrations applies all pending migrations to the database within transactions.
func RunMigrations(ctx context.Context, db *sql.DB) error {
	// Create migration tracking table if not exists
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	appliedVersions := make(map[int]bool)
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("querying applied migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("scanning migration version: %w", err)
		}
		appliedVersions[v] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating applied migrations: %w", err)
	}

	migrations, err := LoadEmbeddedMigrations()
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	for _, m := range migrations {
		if appliedVersions[m.Version] {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("beginning transaction for migration %s: %w", m.Name, err)
		}

		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", m.Name, err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", m.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", m.Name, err)
		}
	}

	return nil
}
