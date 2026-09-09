package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
)

func getPostgresDSN() string {
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		return dsn
	}
	return "postgres://shelfd:shelfd_password@localhost:5432/shelfd?sslmode=disable"
}

func TestPostgresStorageEngine(t *testing.T) {
	dsn := getPostgresDSN()
	bunDB, err := database.OpenBunPostgres(dsn)
	if err != nil {
		t.Skipf("skipping postgres integration test (postgres not reachable: %v)", err)
		return
	}
	defer bunDB.Close()

	ctx := context.Background()

	// 1. Run migrations
	if err := database.RunBunMigrations(ctx, bunDB); err != nil {
		t.Fatalf("failed to run postgres migrations: %v", err)
	}

	// 2. Ensure vector dimensions
	if err := database.EnsureVectorDimensions(ctx, bunDB, 256); err != nil {
		t.Fatalf("failed to ensure pgvector dimensions: %v", err)
	}

	repo := repository.NewBunStorageEngine(bunDB)

	// Clean up any test records
	_ = repo.DeleteBook(ctx, "test-pg-book-1")

	// 3. Create Book, Author, Genre, Series
	author, err := repo.UpsertAuthor(ctx, "Robert C. Martin")
	if err != nil {
		t.Fatalf("failed to upsert author: %v", err)
	}

	genre, err := repo.UpsertGenre(ctx, "Software Engineering")
	if err != nil {
		t.Fatalf("failed to upsert genre: %v", err)
	}

	seriesDesc := "Clean Code Series"
	series, err := repo.UpsertSeries(ctx, "Clean Code Series", &seriesDesc)
	if err != nil {
		t.Fatalf("failed to upsert series: %v", err)
	}

	book := &repository.Book{
		ID:       "test-pg-book-1",
		Title:    "Clean Code: A Handbook of Agile Software Craftsmanship",
		FilePath: "Robert C Martin/Clean Code/Clean Code.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book on postgres: %v", err)
	}

	if err := repo.LinkBookAuthor(ctx, book.ID, author.ID, "author"); err != nil {
		t.Fatalf("failed to link author: %v", err)
	}
	if err := repo.LinkBookGenre(ctx, book.ID, genre.ID); err != nil {
		t.Fatalf("failed to link genre: %v", err)
	}
	seq := 1.0
	if err := repo.LinkBookSeries(ctx, book.ID, series.ID, &seq); err != nil {
		t.Fatalf("failed to link series: %v", err)
	}

	// 4. Create Chapter and Paragraphs
	title := "Meaningful Names"
	chapter := &repository.Chapter{
		ID:           "test-pg-ch-1",
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &title,
		Summary:      "Names are everywhere in software. We name variables, functions, arguments, classes, and packages.",
		ContentPlain: "Names are everywhere in software. We name variables, functions, arguments, classes, and packages. We name source files and directories and everything in them. We name and name and name. Because we do so much of it, we'd better do it well.",
	}
	if err := repo.CreateChapter(ctx, chapter); err != nil {
		t.Fatalf("failed to create chapter: %v", err)
	}

	para := &repository.Paragraph{
		ID:             "test-pg-para-1",
		BookID:         book.ID,
		ChapterID:      chapter.ID,
		ChapterIndex:   chapter.ChapterIndex,
		StartParagraph: 1,
		EndParagraph:   1,
		Content:        chapter.ContentPlain,
	}
	if err := repo.CreateParagraphs(ctx, []*repository.Paragraph{para}); err != nil {
		t.Fatalf("failed to create paragraph on postgres: %v", err)
	}

	// 5. Insert pgvector embedding (256d)
	vec := make([]float32, 256)
	vec[0] = 0.5
	vec[1] = 0.5
	if err := repo.InsertParagraphVector(ctx, para.ID, vec); err != nil {
		t.Fatalf("failed to insert pgvector: %v", err)
	}

	// 6. Test Vector KNN Search
	queryVec := make([]float32, 256)
	queryVec[0] = 0.49
	queryVec[1] = 0.51
	hits, err := repo.SearchVectorParagraphs(ctx, queryVec, repository.SearchFilter{BookID: &book.ID})
	if err != nil {
		t.Fatalf("failed to search pgvector paragraphs: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit from pgvector, got %d", len(hits))
	}
	if hits[0].BookTitle != book.Title || hits[0].ChapterID != chapter.ID {
		t.Fatalf("unexpected pgvector search hit: %+v", hits[0])
	}

	// 7. Test Postgres Full-Text Search (tsvector)
	ftsHits, err := repo.SearchFTSParagraphs(ctx, "software variables functions", repository.SearchFilter{})
	if err != nil {
		t.Fatalf("failed to execute postgres FTS search: %v", err)
	}
	if len(ftsHits) == 0 {
		t.Fatalf("expected at least 1 hit for postgres FTS, got 0")
	}
	if ftsHits[0].BookID != book.ID {
		t.Fatalf("unexpected FTS hit book ID: %s", ftsHits[0].BookID)
	}

	// 8. Test Cascade Deletion
	if err := repo.DeleteBook(ctx, book.ID); err != nil {
		t.Fatalf("failed to delete book: %v", err)
	}

	deletedBook, err := repo.GetBookByID(ctx, book.ID)
	if err != repository.ErrNotFound || deletedBook != nil {
		t.Fatalf("expected ErrNotFound after deletion, got %v", err)
	}

	// Paragraph should be cascaded
	paras, err := repo.GetParagraphsByBookID(ctx, book.ID)
	if err != nil || len(paras) != 0 {
		t.Fatalf("expected 0 paragraphs after book delete, got %d", len(paras))
	}
}
