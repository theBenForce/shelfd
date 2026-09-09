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

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

//go:embed migrations/*.sql
var sqliteMigrationsFS embed.FS

//go:embed migrations_pg/*.sql
var pgMigrationsFS embed.FS

// Migration represents an individual SQL schema migration.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// LoadEmbeddedMigrations reads all SQL migrations from the embedded filesystem for the specified dialect.
func LoadEmbeddedMigrations(dialectName string) ([]Migration, error) {
	var targetFS embed.FS
	var dir string
	if dialectName == "postgres" {
		targetFS = pgMigrationsFS
		dir = "migrations_pg"
	} else {
		targetFS = sqliteMigrationsFS
		dir = "migrations"
	}

	entries, err := fs.ReadDir(targetFS, dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory %s: %w", dir, err)
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

		data, err := targetFS.ReadFile(filepath.Join(dir, entry.Name()))
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

// RunMigrations applies pending SQLite migrations to the database.
func RunMigrations(ctx context.Context, db *sql.DB) error {
	return RunMigrationsWithDialect(ctx, db, "sqlite")
}

// RunBunMigrations applies pending migrations to the Bun database based on its dialect.
func RunBunMigrations(ctx context.Context, db *bun.DB) error {
	dialectName := "sqlite"
	if db.Dialect().Name() == dialect.PG {
		dialectName = "postgres"
	}
	return RunMigrationsWithDialect(ctx, db.DB, dialectName)
}

// RunMigrationsWithDialect applies migrations for the specified dialect ("sqlite" or "postgres").
func RunMigrationsWithDialect(ctx context.Context, db *sql.DB, dialectName string) error {
	// Create migration tracking table if not exists
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

	migrations, err := LoadEmbeddedMigrations(dialectName)
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

		insertSQL := "INSERT INTO schema_migrations (version, name) VALUES (?, ?)"
		if dialectName == "postgres" {
			insertSQL = "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)"
		}
		if _, err := tx.ExecContext(ctx, insertSQL, m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", m.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", m.Name, err)
		}
	}

	return nil
}
