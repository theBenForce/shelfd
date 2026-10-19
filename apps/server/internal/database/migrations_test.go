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
		"topics",
		"book_topics",
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

func TestClearPrePaginatedSyntheticChapterTitlesMigration(t *testing.T) {
	ctx := context.Background()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Insert test books: one pre-paginated, one reflowable
	_, err = db.ExecContext(ctx, `
		INSERT INTO books (id, title, file_path, layout) VALUES 
		('book-fixed', 'Toy Story 4', 'Disney/Toy Story 4/Toy Story 4.epub', 'pre-paginated'),
		('book-reflowable', 'Standard Novel', 'Author/Novel/Novel.epub', 'reflowable')
	`)
	if err != nil {
		t.Fatalf("failed to insert test books: %v", err)
	}

	// Insert test chapters with synthetic and real titles
	_, err = db.ExecContext(ctx, `
		INSERT INTO chapters (id, book_id, chapter_index, title, summary, content_plain) VALUES
		('ch-fixed-1', 'book-fixed', 1, 'Page 1', '', ''),
		('ch-fixed-2', 'book-fixed', 2, 'Page 2', '', ''),
		('ch-fixed-3', 'book-fixed', 3, 'Chapter 1: Woody Moves On', '', ''),
		('ch-reflow-1', 'book-reflowable', 1, 'Page 1', '', '')
	`)
	if err != nil {
		t.Fatalf("failed to insert test chapters: %v", err)
	}

	// Apply migration 0017 logic
	_, err = db.ExecContext(ctx, `
		UPDATE chapters
		SET title = NULL
		WHERE book_id IN (SELECT id FROM books WHERE layout = 'pre-paginated')
		  AND title GLOB 'Page [0-9]*'
	`)
	if err != nil {
		t.Fatalf("failed to run migration 0017 update query: %v", err)
	}

	var fixed1Title, fixed2Title, fixed3Title, reflowTitle *string
	_ = db.QueryRowContext(ctx, "SELECT title FROM chapters WHERE id = 'ch-fixed-1'").Scan(&fixed1Title)
	_ = db.QueryRowContext(ctx, "SELECT title FROM chapters WHERE id = 'ch-fixed-2'").Scan(&fixed2Title)
	_ = db.QueryRowContext(ctx, "SELECT title FROM chapters WHERE id = 'ch-fixed-3'").Scan(&fixed3Title)
	_ = db.QueryRowContext(ctx, "SELECT title FROM chapters WHERE id = 'ch-reflow-1'").Scan(&reflowTitle)

	if fixed1Title != nil {
		t.Errorf("expected fixed-1 title to be cleared to NULL, got: %s", *fixed1Title)
	}
	if fixed2Title != nil {
		t.Errorf("expected fixed-2 title to be cleared to NULL, got: %s", *fixed2Title)
	}
	if fixed3Title == nil || *fixed3Title != "Chapter 1: Woody Moves On" {
		t.Errorf("expected fixed-3 title to be preserved, got: %v", fixed3Title)
	}
	if reflowTitle == nil || *reflowTitle != "Page 1" {
		t.Errorf("expected reflowable book chapter title to be preserved, got: %v", reflowTitle)
	}
}
