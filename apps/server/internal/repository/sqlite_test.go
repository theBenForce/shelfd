package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/ulid"
)

func setupTestDB(t *testing.T) (*sql.DB, repository.StorageEngine) {
	t.Helper()
	ctx := context.Background()

	bunDB, err := database.OpenBunSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}

	if err := database.RunBunMigrations(ctx, bunDB); err != nil {
		bunDB.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	repo := repository.NewBunStorageEngine(bunDB)
	return bunDB.DB, repo
}

func TestBookCascadeDeletion(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer repo.Close()

	// 1. Create normalized author, genre, and series
	author, err := repo.UpsertAuthor(ctx, "Frank Herbert")
	if err != nil {
		t.Fatalf("failed to upsert author: %v", err)
	}

	genre, err := repo.UpsertGenre(ctx, "Science Fiction")
	if err != nil {
		t.Fatalf("failed to upsert genre: %v", err)
	}

	seriesDesc := "The epic Dune universe"
	series, err := repo.UpsertSeries(ctx, "Dune Chronicles", &seriesDesc)
	if err != nil {
		t.Fatalf("failed to upsert series: %v", err)
	}

	// 2. Create book
	book := &repository.Book{
		ID:       "book-dune-1",
		Title:    "Dune",
		FilePath: "Frank Herbert/Dune/Dune.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book: %v", err)
	}

	// 3. Link book to author, genre, and series with sequence number 1.0
	seqNum := 1.0
	if err := repo.LinkBookAuthor(ctx, book.ID, author.ID, "author"); err != nil {
		t.Fatalf("failed to link author: %v", err)
	}
	if err := repo.LinkBookGenre(ctx, book.ID, genre.ID); err != nil {
		t.Fatalf("failed to link genre: %v", err)
	}
	if err := repo.LinkBookSeries(ctx, book.ID, series.ID, &seqNum); err != nil {
		t.Fatalf("failed to link series: %v", err)
	}

	// 4. Create chapter
	chapterTitle := "Chapter 1: Arrakis"
	chapter := &repository.Chapter{
		ID:           "chapter-dune-1",
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &chapterTitle,
		Summary:      "Paul Atreides prepares to depart Caladan for Arrakis.",
		ContentPlain: "A beginning is the time for taking the most delicate care that the balances are correct.",
	}
	if err := repo.CreateChapter(ctx, chapter); err != nil {
		t.Fatalf("failed to create chapter: %v", err)
	}

	// 5. Insert 256-dimensional vector embedding for chapter
	embedding := make([]float32, 256)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}
	if err := repo.InsertChapterVector(ctx, chapter.ID, embedding); err != nil {
		t.Fatalf("failed to insert chapter vector: %v", err)
	}

	// 6. Verify associations and counts before deletion
	authors, err := repo.GetBookAuthors(ctx, book.ID)
	if err != nil || len(authors) != 1 || authors[0].Name != "Frank Herbert" {
		t.Fatalf("unexpected book authors: %v", authors)
	}

	genres, err := repo.GetBookGenres(ctx, book.ID)
	if err != nil || len(genres) != 1 || genres[0].Name != "Science Fiction" {
		t.Fatalf("unexpected book genres: %v", genres)
	}

	seriesList, err := repo.GetBookSeries(ctx, book.ID)
	if err != nil || len(seriesList) != 1 || seriesList[0].Name != "Dune Chronicles" || *seriesList[0].SequenceNumber != 1.0 {
		t.Fatalf("unexpected book series: %v", seriesList)
	}

	chapters, err := repo.GetChaptersByBookID(ctx, book.ID)
	if err != nil || len(chapters) != 1 {
		t.Fatalf("unexpected book chapters: %v", chapters)
	}

	spine, err := repo.GetBookSpine(ctx, book.ID)
	if err != nil || len(spine) != 1 {
		t.Fatalf("unexpected book spine: %v", spine)
	}
	if spine[0].ID != chapter.ID || spine[0].ChapterIndex != 1 || *spine[0].Title != chapterTitle {
		t.Fatalf("unexpected spine item data: %+v", spine[0])
	}

	var count int
	db.QueryRowContext(ctx, "SELECT count(*) FROM vec_paragraphs").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 vector row before delete, got %d", count)
	}

	// 7. Delete book and assert complete cascade
	if err := repo.DeleteBook(ctx, book.ID); err != nil {
		t.Fatalf("failed to delete book: %v", err)
	}

	// Verify book is gone
	_, err = repo.GetBookByID(ctx, book.ID)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound for deleted book, got %v", err)
	}

	// Verify junction tables are cleanly dropped
	db.QueryRowContext(ctx, "SELECT count(*) FROM book_authors WHERE book_id = ?", book.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 book_authors after delete, got %d", count)
	}

	db.QueryRowContext(ctx, "SELECT count(*) FROM book_genres WHERE book_id = ?", book.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 book_genres after delete, got %d", count)
	}

	db.QueryRowContext(ctx, "SELECT count(*) FROM book_series WHERE book_id = ?", book.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 book_series after delete, got %d", count)
	}

	// Verify chapter is cascade-deleted
	db.QueryRowContext(ctx, "SELECT count(*) FROM chapters WHERE book_id = ?", book.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 chapters after delete, got %d", count)
	}

	// Verify paragraphs and vectors are cascade-cleaned up via trigger
	db.QueryRowContext(ctx, "SELECT count(*) FROM paragraphs WHERE book_id = ?", book.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 paragraphs after delete, got %d", count)
	}
	db.QueryRowContext(ctx, "SELECT count(*) FROM vec_paragraphs").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 vec_paragraphs after delete, got %d", count)
	}

	// Verify normalized entities still exist
	db.QueryRowContext(ctx, "SELECT count(*) FROM authors WHERE id = ?", author.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected author to remain intact, got %d", count)
	}
	db.QueryRowContext(ctx, "SELECT count(*) FROM genres WHERE id = ?", genre.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected genre to remain intact, got %d", count)
	}
	db.QueryRowContext(ctx, "SELECT count(*) FROM series WHERE id = ?", series.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected series to remain intact, got %d", count)
	}
}

func TestBookCRUDAndFiltering(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	// 1. Create authors and genres
	author1, _ := repo.UpsertAuthor(ctx, "Philip K. Dick")
	author2, _ := repo.UpsertAuthor(ctx, "William Gibson")
	genre1, _ := repo.UpsertGenre(ctx, "Cyberpunk")
	genre2, _ := repo.UpsertGenre(ctx, "Dystopia")
	series1, _ := repo.UpsertSeries(ctx, "Sprawl", nil)

	// 2. Create books
	desc := "Classic cyberpunk novel"
	size := int64(450000)
	modTime := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	book1 := &repository.Book{
		Title:          "Neuromancer",
		Description:    &desc,
		FilePath:       "William Gibson/Neuromancer/Neuromancer.epub",
		FileSizeBytes:  &size,
		FileModifiedAt: &modTime,
	}
	if err := repo.CreateBook(ctx, book1); err != nil {
		t.Fatalf("create book 1: %v", err)
	}
	repo.LinkBookAuthor(ctx, book1.ID, author2.ID, "author")
	repo.LinkBookGenre(ctx, book1.ID, genre1.ID)
	seq1 := 1.0
	repo.LinkBookSeries(ctx, book1.ID, series1.ID, &seq1)

	book2 := &repository.Book{
		Title:    "Do Androids Dream of Electric Sheep?",
		FilePath: "Philip K Dick/Do Androids Dream/Do Androids Dream.epub",
	}
	if err := repo.CreateBook(ctx, book2); err != nil {
		t.Fatalf("create book 2: %v", err)
	}
	repo.LinkBookAuthor(ctx, book2.ID, author1.ID, "author")
	repo.LinkBookGenre(ctx, book2.ID, genre2.ID)

	// 3. Update book
	newTitle := "Neuromancer: 20th Anniversary Edition"
	newModTime := time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC)
	book1.Title = newTitle
	book1.FileModifiedAt = &newModTime
	if err := repo.UpdateBook(ctx, book1); err != nil {
		t.Fatalf("update book: %v", err)
	}

	fetched, err := repo.GetBookByID(ctx, book1.ID)
	if err != nil || fetched.Title != newTitle {
		t.Fatalf("expected updated title %s, got %v", newTitle, fetched)
	}
	if fetched.FileModifiedAt == nil || !fetched.FileModifiedAt.Equal(newModTime) {
		t.Fatalf("expected file modified at %v, got %v", newModTime, fetched.FileModifiedAt)
	}

	byPath, err := repo.GetBookByFilePath(ctx, book1.FilePath)
	if err != nil || byPath.FileModifiedAt == nil || !byPath.FileModifiedAt.Equal(newModTime) {
		t.Fatalf("expected GetBookByFilePath to return file modified at %v, got %v", newModTime, byPath.FileModifiedAt)
	}

	// Update non-existent book
	nonExistentBook := &repository.Book{ID: "no-such-id", Title: "Ghost"}
	if err := repo.UpdateBook(ctx, nonExistentBook); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound on updating non-existent book, got %v", err)
	}

	// Delete non-existent book
	if err := repo.DeleteBook(ctx, "no-such-id"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound on deleting non-existent book, got %v", err)
	}

	// 4. Test ListBooks filtering
	// Filter by Author
	booksByAuthor, err := repo.ListBooks(ctx, repository.BookFilter{AuthorID: &author2.ID})
	if err != nil || len(booksByAuthor) != 1 || booksByAuthor[0].ID != book1.ID {
		t.Fatalf("expected 1 book by author2, got %v", booksByAuthor)
	}

	// Filter by Genre
	booksByGenre, err := repo.ListBooks(ctx, repository.BookFilter{GenreID: &genre2.ID})
	if err != nil || len(booksByGenre) != 1 || booksByGenre[0].ID != book2.ID {
		t.Fatalf("expected 1 book by genre2, got %v", booksByGenre)
	}

	// Filter by Series
	booksBySeries, err := repo.ListBooks(ctx, repository.BookFilter{SeriesID: &series1.ID})
	if err != nil || len(booksBySeries) != 1 || booksBySeries[0].ID != book1.ID {
		t.Fatalf("expected 1 book by series1, got %v", booksBySeries)
	}

	// Filter by Search
	searchQuery := "Androids"
	booksBySearch, err := repo.ListBooks(ctx, repository.BookFilter{Search: &searchQuery})
	if err != nil || len(booksBySearch) != 1 || booksBySearch[0].ID != book2.ID {
		t.Fatalf("expected 1 book matching 'Androids', got %v", booksBySearch)
	}

	// Filter by AuthorName
	authorNameQuery := "dick"
	booksByAuthorName, err := repo.ListBooks(ctx, repository.BookFilter{AuthorName: &authorNameQuery})
	if err != nil || len(booksByAuthorName) != 1 || booksByAuthorName[0].ID != book2.ID {
		t.Fatalf("expected 1 book matching authorName 'dick', got %v (err: %v)", booksByAuthorName, err)
	}

	// Filter by GenreName
	genreNameQuery := "cyberpunk"
	booksByGenreName, err := repo.ListBooks(ctx, repository.BookFilter{GenreName: &genreNameQuery})
	if err != nil || len(booksByGenreName) != 1 || booksByGenreName[0].ID != book1.ID {
		t.Fatalf("expected 1 book matching genreName 'cyberpunk', got %v (err: %v)", booksByGenreName, err)
	}

	// Pagination
	allBooks, err := repo.ListBooks(ctx, repository.BookFilter{Limit: 1, Offset: 0})
	if err != nil || len(allBooks) != 1 {
		t.Fatalf("expected 1 book with limit 1, got %v", allBooks)
	}
	offsetBooks, err := repo.ListBooks(ctx, repository.BookFilter{Limit: 1, Offset: 1})
	if err != nil || len(offsetBooks) != 1 || offsetBooks[0].ID == allBooks[0].ID {
		t.Fatalf("expected different book with offset 1, got %v", offsetBooks)
	}

	// CountBooks verification
	totalCount, err := repo.CountBooks(ctx, repository.BookFilter{})
	if err != nil || totalCount != 2 {
		t.Fatalf("expected 2 total books, got %d (err: %v)", totalCount, err)
	}
	authorCount, err := repo.CountBooks(ctx, repository.BookFilter{AuthorID: &author2.ID})
	if err != nil || authorCount != 1 {
		t.Fatalf("expected 1 book by author2 count, got %d", authorCount)
	}
	authorNameCount, err := repo.CountBooks(ctx, repository.BookFilter{AuthorName: &authorNameQuery})
	if err != nil || authorNameCount != 1 {
		t.Fatalf("expected 1 book by authorName count, got %d", authorNameCount)
	}

	// Verify ListGenres returns BookCount
	genresList, err := repo.ListGenres(ctx)
	if err != nil || len(genresList) == 0 {
		t.Fatalf("expected genres from ListGenres, got %v (err: %v)", genresList, err)
	}
	if genresList[0].BookCount < 1 {
		t.Fatalf("expected genre BookCount >= 1, got %d", genresList[0].BookCount)
	}
}

func TestAlphabeticalAndSeriesOrdering(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	// 1. Verify Alphabetical Sorting by default
	books := []*repository.Book{
		{Title: "Zeta Book", FilePath: "test/zeta.epub"},
		{Title: "Alpha Book", FilePath: "test/alpha.epub"},
		{Title: "Beta Book", FilePath: "test/beta.epub"},
	}
	for _, b := range books {
		if err := repo.CreateBook(ctx, b); err != nil {
			t.Fatalf("failed to create book %s: %v", b.Title, err)
		}
	}

	alphaList, err := repo.ListBooks(ctx, repository.BookFilter{})
	if err != nil {
		t.Fatalf("failed to list books: %v", err)
	}
	if len(alphaList) != 3 {
		t.Fatalf("expected 3 books, got %d", len(alphaList))
	}
	if alphaList[0].Title != "Alpha Book" || alphaList[1].Title != "Beta Book" || alphaList[2].Title != "Zeta Book" {
		t.Fatalf("expected alphabetical order [Alpha, Beta, Zeta], got [%s, %s, %s]",
			alphaList[0].Title, alphaList[1].Title, alphaList[2].Title)
	}

	// 2. Verify Series Ordering (Numbered first in sequence, then unnumbered alphabetically)
	series, err := repo.UpsertSeries(ctx, "Test Series", nil)
	if err != nil {
		t.Fatalf("failed to upsert series: %v", err)
	}

	sBooks := []*repository.Book{
		{Title: "Unnumbered Zulu", FilePath: "test/s_zulu.epub"},
		{Title: "Book Two", FilePath: "test/s_two.epub"},
		{Title: "Unnumbered Alpha", FilePath: "test/s_alpha.epub"},
		{Title: "Book One", FilePath: "test/s_one.epub"},
	}
	for _, b := range sBooks {
		if err := repo.CreateBook(ctx, b); err != nil {
			t.Fatalf("create series book: %v", err)
		}
	}

	seq2 := 2.0
	seq1 := 1.0
	repo.LinkBookSeries(ctx, sBooks[0].ID, series.ID, nil)   // Zulu (nil)
	repo.LinkBookSeries(ctx, sBooks[1].ID, series.ID, &seq2)  // Two (2.0)
	repo.LinkBookSeries(ctx, sBooks[2].ID, series.ID, nil)   // Alpha (nil)
	repo.LinkBookSeries(ctx, sBooks[3].ID, series.ID, &seq1)  // One (1.0)

	seriesResults, err := repo.ListBooks(ctx, repository.BookFilter{SeriesID: &series.ID})
	if err != nil {
		t.Fatalf("failed to list series books: %v", err)
	}
	if len(seriesResults) != 4 {
		t.Fatalf("expected 4 series books, got %d", len(seriesResults))
	}

	expectedOrder := []string{"Book One", "Book Two", "Unnumbered Alpha", "Unnumbered Zulu"}
	for i, exp := range expectedOrder {
		if seriesResults[i].Title != exp {
			t.Errorf("series book [%d]: expected %q, got %q", i, exp, seriesResults[i].Title)
		}
	}

	// 3. Verify Series book_count and cover_book_id
	seriesList, err := repo.ListSeries(ctx)
	if err != nil {
		t.Fatalf("failed to list series: %v", err)
	}
	foundSeries := false
	for _, s := range seriesList {
		if s.ID == series.ID {
			foundSeries = true
			if s.BookCount != 4 {
				t.Errorf("expected series book count 4, got %d", s.BookCount)
			}
			if s.CoverBookID == nil || *s.CoverBookID == "" {
				t.Errorf("expected non-empty cover_book_id for series")
			}
		}
	}
	if !foundSeries {
		t.Errorf("expected to find test series in ListSeries")
	}
}

func TestAuthorGenreSeriesUpserts(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	// Author upsert idempotency with case insensitivity
	a1, err := repo.UpsertAuthor(ctx, "Isaac Asimov")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	a2, err := repo.UpsertAuthor(ctx, "isaac asimov")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if a1.ID != a2.ID {
		t.Errorf("expected same ID for case-insensitive author upsert: %s != %s", a1.ID, a2.ID)
	}

	// Author lookup queries
	byID, err := repo.GetAuthorByID(ctx, a1.ID)
	if err != nil || !strings.EqualFold(byID.Name, "Isaac Asimov") {
		t.Fatalf("get author by id: %v", err)
	}
	byName, err := repo.GetAuthorByName(ctx, "ISAAC ASIMOV")
	if err != nil || byName.ID != a1.ID {
		t.Fatalf("get author by name: %v", err)
	}
	allAuthors, err := repo.ListAuthors(ctx)
	if err != nil || len(allAuthors) != 1 {
		t.Fatalf("list authors: %v", err)
	}
	if _, err := repo.UpsertAuthor(ctx, "   "); err == nil {
		t.Error("expected error on empty author name")
	}
	if _, err := repo.GetAuthorByID(ctx, "ghost-id"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing author, got %v", err)
	}
	if _, err := repo.GetAuthorByName(ctx, "ghost-name"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing author, got %v", err)
	}
	authorByPartial, err := repo.GetAuthorByName(ctx, "Asimov")
	if err != nil || authorByPartial.ID != a1.ID {
		t.Fatalf("get author by partial substring 'Asimov': %v", err)
	}
	if _, err := repo.GetAuthorByName(ctx, "   "); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for whitespace author, got %v", err)
	}

	// Genre upsert
	g1, err := repo.UpsertGenre(ctx, "Cyberpunk")
	if err != nil {
		t.Fatalf("genre upsert: %v", err)
	}
	g2, err := repo.UpsertGenre(ctx, "cyberpunk")
	if err != nil {
		t.Fatalf("genre upsert 2: %v", err)
	}
	if g1.ID != g2.ID {
		t.Errorf("expected same ID for case-insensitive genre upsert: %s != %s", g1.ID, g2.ID)
	}
	genreByID, err := repo.GetGenreByID(ctx, g1.ID)
	if err != nil || !strings.EqualFold(genreByID.Name, "Cyberpunk") {
		t.Fatalf("get genre by id: %v", err)
	}
	genreByName, err := repo.GetGenreByName(ctx, "CYBERPUNK")
	if err != nil || genreByName.ID != g1.ID {
		t.Fatalf("get genre by name: %v", err)
	}
	genreByPartial, err := repo.GetGenreByName(ctx, "Cyber")
	if err != nil || genreByPartial.ID != g1.ID {
		t.Fatalf("get genre by partial substring 'Cyber': %v", err)
	}
	genreBySuffix, err := repo.GetGenreByName(ctx, "punk")
	if err != nil || genreBySuffix.ID != g1.ID {
		t.Fatalf("get genre by partial substring 'punk': %v", err)
	}
	allGenres, err := repo.ListGenres(ctx)
	if err != nil || len(allGenres) != 1 {
		t.Fatalf("list genres: %v", err)
	}
	if _, err := repo.UpsertGenre(ctx, "   "); err == nil {
		t.Error("expected error on empty genre name")
	}
	if _, err := repo.GetGenreByID(ctx, "ghost-id"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing genre, got %v", err)
	}
	if _, err := repo.GetGenreByName(ctx, "ghost-name"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing genre, got %v", err)
	}
	if _, err := repo.GetGenreByName(ctx, "   "); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for whitespace genre, got %v", err)
	}

	// Series with fractional sequence number (e.g. 2.5)
	sDesc := "Sprawl trilogy"
	s1, err := repo.UpsertSeries(ctx, "Sprawl", &sDesc)
	if err != nil {
		t.Fatalf("series upsert: %v", err)
	}
	seriesByID, err := repo.GetSeriesByID(ctx, s1.ID)
	if err != nil || !strings.EqualFold(seriesByID.Name, "Sprawl") {
		t.Fatalf("get series by id: %v", err)
	}
	seriesByName, err := repo.GetSeriesByName(ctx, "SPRAWL")
	if err != nil || seriesByName.ID != s1.ID {
		t.Fatalf("get series by name: %v", err)
	}
	allSeries, err := repo.ListSeries(ctx)
	if err != nil || len(allSeries) != 1 {
		t.Fatalf("list series: %v", err)
	}
	if _, err := repo.UpsertSeries(ctx, "   ", nil); err == nil {
		t.Error("expected error on empty series name")
	}
	if _, err := repo.GetSeriesByID(ctx, "ghost-id"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing series, got %v", err)
	}
	if _, err := repo.GetSeriesByName(ctx, "ghost-name"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing series, got %v", err)
	}
	seriesByPartial, err := repo.GetSeriesByName(ctx, "praw")
	if err != nil || seriesByPartial.ID != s1.ID {
		t.Fatalf("get series by partial substring 'praw': %v", err)
	}
	if _, err := repo.GetSeriesByName(ctx, "   "); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for whitespace series, got %v", err)
	}

	b := &repository.Book{Title: "Short Story", FilePath: "William Gibson/Sprawl/2.5.epub"}
	repo.CreateBook(ctx, b)
	seq := 2.5
	if err := repo.LinkBookSeries(ctx, b.ID, s1.ID, &seq); err != nil {
		t.Fatalf("linking book series with fractional sequence: %v", err)
	}

	links, err := repo.GetBookSeries(ctx, b.ID)
	if err != nil || len(links) != 1 || *links[0].SequenceNumber != 2.5 {
		t.Errorf("expected sequence number 2.5, got %v", links)
	}
}

func TestChapters(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	book := &repository.Book{Title: "Test Book", FilePath: "test.epub"}
	repo.CreateBook(ctx, book)

	chTitle := "Prologue"
	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 0,
		Title:        &chTitle,
		Summary:      "An introduction.",
		ContentPlain: "It began with silence.",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	fetched, err := repo.GetChapterByID(ctx, ch.ID)
	if err != nil || fetched.Summary != ch.Summary {
		t.Fatalf("get chapter by id: %v", err)
	}

	fetchedByIndex, err := repo.GetChapterByBookAndIndex(ctx, book.ID, 0)
	if err != nil || fetchedByIndex.ID != ch.ID {
		t.Fatalf("get chapter by book and index: %v", err)
	}

	if _, err := repo.GetChapterByBookAndIndex(ctx, book.ID, 999); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing index, got %v", err)
	}

	if _, err := repo.GetChapterByID(ctx, "ghost-ch"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUsersAndTokens(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	user := &repository.User{
		Username:     "homelab_admin",
		PasswordHash: "$2a$12$e8Yd8mockhash",
	}
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	fetchedUser, err := repo.GetUserByUsername(ctx, "homelab_admin")
	if err != nil || fetchedUser.ID != user.ID {
		t.Fatalf("failed to get user: %v", err)
	}

	fetchedUserByID, err := repo.GetUserByID(ctx, user.ID)
	if err != nil || fetchedUserByID.Username != user.Username {
		t.Fatalf("failed to get user by id: %v", err)
	}

	if _, err := repo.GetUserByUsername(ctx, "ghost-user"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing username, got %v", err)
	}
	if _, err := repo.GetUserByID(ctx, "ghost-id"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing user id, got %v", err)
	}

	token := &repository.APIToken{
		UserID:    user.ID,
		TokenHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Name:      "claude-desktop-mcp",
	}
	if err := repo.CreateAPIToken(ctx, token); err != nil {
		t.Fatalf("failed to create api token: %v", err)
	}

	fetchedToken, err := repo.GetAPITokenByHash(ctx, token.TokenHash)
	if err != nil || fetchedToken.ID != token.ID {
		t.Fatalf("failed to get api token: %v", err)
	}

	tokens, err := repo.ListAPITokensByUserID(ctx, user.ID)
	if err != nil || len(tokens) != 1 || tokens[0].ID != token.ID {
		t.Fatalf("expected 1 token for user, got %v", tokens)
	}

	if err := repo.DeleteAPIToken(ctx, token.ID); err != nil {
		t.Fatalf("failed to delete api token: %v", err)
	}

	_, err = repo.GetAPITokenByHash(ctx, token.TokenHash)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound after token delete, got %v", err)
	}

	if err := repo.DeleteAPIToken(ctx, "ghost-token"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound on deleting missing token, got %v", err)
	}

	// Test updating password
	newHash := "$2a$12$newMockPasswordHash"
	if err := repo.UpdateUserPassword(ctx, user.ID, newHash); err != nil {
		t.Fatalf("failed to update user password: %v", err)
	}

	updatedUser, err := repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get updated user: %v", err)
	}
	if updatedUser.PasswordHash != newHash {
		t.Errorf("expected updated password hash %s, got %s", newHash, updatedUser.PasswordHash)
	}

	if err := repo.UpdateUserPassword(ctx, "ghost-id", newHash); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing user id, got %v", err)
	}
}

func TestChapterSummaryAndVectorSearch(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	// 1. Setup metadata
	author, err := repo.UpsertAuthor(ctx, "William Gibson")
	if err != nil {
		t.Fatalf("upsert author: %v", err)
	}
	genre, err := repo.UpsertGenre(ctx, "Cyberpunk")
	if err != nil {
		t.Fatalf("upsert genre: %v", err)
	}
	series, err := repo.UpsertSeries(ctx, "Sprawl Trilogy", nil)
	if err != nil {
		t.Fatalf("upsert series: %v", err)
	}

	book1 := &repository.Book{
		Title:    "Neuromancer",
		FilePath: "William Gibson/Neuromancer/Neuromancer.epub",
	}
	if err := repo.CreateBook(ctx, book1); err != nil {
		t.Fatalf("create book 1: %v", err)
	}
	repo.LinkBookAuthor(ctx, book1.ID, author.ID, "author")
	repo.LinkBookGenre(ctx, book1.ID, genre.ID)
	seq1 := 1.0
	repo.LinkBookSeries(ctx, book1.ID, series.ID, &seq1)

	// Create chapters with empty summary
	ch1Title := "Chiba City Blues"
	ch1 := &repository.Chapter{
		BookID:       book1.ID,
		ChapterIndex: 1,
		Title:        &ch1Title,
		Summary:      "",
		ContentPlain: "The sky above the port was the color of television, tuned to a dead channel.",
	}
	if err := repo.CreateChapter(ctx, ch1); err != nil {
		t.Fatalf("create ch1: %v", err)
	}

	ch2Title := "Shopping Expedition"
	ch2 := &repository.Chapter{
		BookID:       book1.ID,
		ChapterIndex: 2,
		Title:        &ch2Title,
		Summary:      "",
		ContentPlain: "Case sat in the sushi shop, waiting for Molly.",
	}
	if err := repo.CreateChapter(ctx, ch2); err != nil {
		t.Fatalf("create ch2: %v", err)
	}

	// 2. Test GetUnindexedChapters
	unindexed, err := repo.GetUnindexedChapters(ctx, 10)
	if err != nil {
		t.Fatalf("get unindexed chapters: %v", err)
	}
	if len(unindexed) != 2 {
		t.Fatalf("expected 2 unindexed chapters, got %d", len(unindexed))
	}

	// 3. Test UpdateChapterSummary
	summary1 := "Case meets Molly in Night City and gets hired for an impossible heist."
	if err := repo.UpdateChapterSummary(ctx, ch1.ID, summary1); err != nil {
		t.Fatalf("update chapter summary: %v", err)
	}

	if err := repo.UpdateChapterSummary(ctx, "missing-chapter", "summary"); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound on missing chapter summary update, got %v", err)
	}

	// Verify unindexed chapters decreased
	unindexedAfter, err := repo.GetUnindexedChapters(ctx, 10)
	if err != nil || len(unindexedAfter) != 1 || unindexedAfter[0].ID != ch2.ID {
		t.Fatalf("expected 1 unindexed chapter (ch2), got %v", unindexedAfter)
	}

	summary2 := "Case and Molly visit the black clinics to repair his neural damage."
	if err := repo.UpdateChapterSummary(ctx, ch2.ID, summary2); err != nil {
		t.Fatalf("update ch2 summary: %v", err)
	}

	// 4. Test InsertChapterVector and SearchVectorChapters
	vec1 := make([]float32, 256)
	vec1[0] = 1.0 // Vector pointing along dimension 0
	vec2 := make([]float32, 256)
	vec2[1] = 1.0 // Vector pointing along dimension 1

	if err := repo.InsertChapterVector(ctx, ch1.ID, vec1); err != nil {
		t.Fatalf("insert vec1: %v", err)
	}
	if err := repo.InsertChapterVector(ctx, ch2.ID, vec2); err != nil {
		t.Fatalf("insert vec2: %v", err)
	}

	// Test updating/re-inserting chapter vector for existing chapter
	if err := repo.InsertChapterVector(ctx, ch1.ID, vec1); err != nil {
		t.Fatalf("re-insert/update vec1: %v", err)
	}

	// Search query matching vec1
	queryVec := make([]float32, 256)
	queryVec[0] = 0.95
	queryVec[1] = 0.05

	hits, err := repo.SearchVectorChapters(ctx, queryVec, repository.SearchFilter{Limit: 5})
	if err != nil {
		t.Fatalf("search vector chapters: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].ChapterID != ch1.ID {
		t.Fatalf("expected first hit to be ch1, got %s", hits[0].ChapterID)
	}
	if hits[0].BookTitle != "Neuromancer" {
		t.Errorf("expected book title Neuromancer, got %s", hits[0].BookTitle)
	}
	if hits[0].AuthorName == nil || *hits[0].AuthorName != "William Gibson" {
		t.Errorf("expected author William Gibson, got %v", hits[0].AuthorName)
	}
	if hits[0].SeriesName == nil || *hits[0].SeriesName != "Sprawl Trilogy" {
		t.Errorf("expected series Sprawl Trilogy, got %v", hits[0].SeriesName)
	}
	if hits[0].Distance > 0.1 {
		t.Errorf("expected cosine distance < 0.1, got %f", hits[0].Distance)
	}

	// Test SearchFilter by AuthorID
	authorHits, err := repo.SearchVectorChapters(ctx, queryVec, repository.SearchFilter{
		AuthorID: &author.ID,
		Limit:    5,
	})
	if err != nil || len(authorHits) != 2 {
		t.Fatalf("expected 2 hits for author, got %v", authorHits)
	}

	nonMatchingAuthorID := "different-author-id"
	noHits, err := repo.SearchVectorChapters(ctx, queryVec, repository.SearchFilter{
		AuthorID: &nonMatchingAuthorID,
		Limit:    5,
	})
	if err != nil || len(noHits) != 0 {
		t.Fatalf("expected 0 hits for non-matching author, got %d", len(noHits))
	}

	// Test SearchFilter by GenreID
	genreHits, err := repo.SearchVectorChapters(ctx, queryVec, repository.SearchFilter{
		GenreID: &genre.ID,
		Limit:   5,
	})
	if err != nil || len(genreHits) != 2 {
		t.Fatalf("expected 2 hits for genre, got %v", genreHits)
	}

	// Test SearchFilter by SeriesID
	seriesHits, err := repo.SearchVectorChapters(ctx, queryVec, repository.SearchFilter{
		SeriesID: &series.ID,
		Limit:    5,
	})
	if err != nil || len(seriesHits) != 2 {
		t.Fatalf("expected 2 hits for series, got %v", seriesHits)
	}

	// Test Limit = 1
	limitOneHits, err := repo.SearchVectorChapters(ctx, queryVec, repository.SearchFilter{
		Limit: 1,
	})
	if err != nil || len(limitOneHits) != 1 {
		t.Fatalf("expected 1 hit with limit 1, got %d", len(limitOneHits))
	}
}

func TestUploadJobs(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	// 1. Create upload job
	job := &repository.UploadJob{
		ID:         "job-1",
		Filename:   "dune.epub",
		StagedPath: "/data/uploads/job-1.epub",
	}
	if err := repo.CreateUploadJob(ctx, job); err != nil {
		t.Fatalf("failed to create upload job: %v", err)
	}

	// 2. Get upload job
	fetched, err := repo.GetUploadJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("failed to get upload job: %v", err)
	}
	if fetched.ID != "job-1" || fetched.Filename != "dune.epub" || fetched.Status != "queued" {
		t.Fatalf("unexpected fetched job: %+v", fetched)
	}
	if fetched.StagedPath != "/data/uploads/job-1.epub" {
		t.Fatalf("unexpected staged path: %s", fetched.StagedPath)
	}

	// 3. Update status to processing
	if err := repo.UpdateUploadJobStatus(ctx, "job-1", "processing", nil, nil); err != nil {
		t.Fatalf("failed to update status to processing: %v", err)
	}
	fetched, _ = repo.GetUploadJob(ctx, "job-1")
	if fetched.Status != "processing" {
		t.Fatalf("expected status processing, got %s", fetched.Status)
	}

	// 4. Create a book to link to completed job
	book := &repository.Book{
		ID:       "book-uploaded-1",
		Title:    "Uploaded Dune",
		FilePath: "Frank Herbert/Uploaded Dune/Uploaded Dune.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book: %v", err)
	}

	// Update status to completed with book_id
	if err := repo.UpdateUploadJobStatus(ctx, "job-1", "completed", &book.ID, nil); err != nil {
		t.Fatalf("failed to update status to completed: %v", err)
	}
	fetched, _ = repo.GetUploadJob(ctx, "job-1")
	if fetched.Status != "completed" || fetched.BookID == nil || *fetched.BookID != book.ID {
		t.Fatalf("unexpected completed job: %+v", fetched)
	}

	// 5. Test error / failed job
	job2 := &repository.UploadJob{
		ID:         "job-2",
		Filename:   "bad.epub",
		StagedPath: "/data/uploads/job-2.epub",
	}
	if err := repo.CreateUploadJob(ctx, job2); err != nil {
		t.Fatalf("failed to create job2: %v", err)
	}
	errMsg := "corrupted zip archive"
	if err := repo.UpdateUploadJobStatus(ctx, "job-2", "failed", nil, &errMsg); err != nil {
		t.Fatalf("failed to update job2 to failed: %v", err)
	}
	fetched2, err := repo.GetUploadJob(ctx, "job-2")
	if err != nil {
		t.Fatalf("failed to get job2: %v", err)
	}
	if fetched2.Status != "failed" || fetched2.ErrorMessage == nil || *fetched2.ErrorMessage != errMsg {
		t.Fatalf("unexpected failed job: %+v", fetched2)
	}

	// 6. Test GetPendingUploadJobs
	job3 := &repository.UploadJob{
		ID:         "job-3",
		Filename:   "pending.epub",
		StagedPath: "/data/uploads/job-3.epub",
	}
	if err := repo.CreateUploadJob(ctx, job3); err != nil {
		t.Fatalf("failed to create job3: %v", err)
	}

	pending, err := repo.GetPendingUploadJobs(ctx, 10)
	if err != nil {
		t.Fatalf("failed to get pending upload jobs: %v", err)
	}
	// Only job3 should be pending (job-1 is completed, job-2 is failed)
	if len(pending) != 1 || pending[0].ID != "job-3" {
		t.Fatalf("expected 1 pending job (job-3), got %d: %+v", len(pending), pending)
	}

	// 7. Test ListUploadJobs
	allJobs, err := repo.ListUploadJobs(ctx, 10)
	if err != nil {
		t.Fatalf("failed to list upload jobs: %v", err)
	}
	if len(allJobs) != 3 {
		t.Fatalf("expected 3 total jobs in list, got %d", len(allJobs))
	}

	// 7b. Test ListUploadJobs with status filter and StagedUploads in queue status
	jobStaged := &repository.UploadJob{
		ID:         "job-staged",
		Filename:   "staged.epub",
		StagedPath: "/data/uploads/job-staged.epub",
		Status:     "staged",
	}
	if err := repo.CreateUploadJob(ctx, jobStaged); err != nil {
		t.Fatalf("failed to create staged job: %v", err)
	}

	stagedJobs, err := repo.ListUploadJobs(ctx, 10, "staged")
	if err != nil {
		t.Fatalf("failed to list staged upload jobs: %v", err)
	}
	if len(stagedJobs) != 1 || stagedJobs[0].ID != "job-staged" {
		t.Fatalf("expected 1 staged job (job-staged), got %d", len(stagedJobs))
	}

	// 7c. Test GetQueueStatus includes StagedUploads
	qStatus, err := repo.GetQueueStatus(ctx)
	if err != nil {
		t.Fatalf("failed to get queue status: %v", err)
	}
	if qStatus.StagedUploads != 1 {
		t.Fatalf("expected 1 staged upload in queue status, got %d", qStatus.StagedUploads)
	}

	// 8. Test UpdateUploadJobCommit
	metaJSON := `{"title":"Updated Title","author":"Updated Author"}`
	if err := repo.UpdateUploadJobCommit(ctx, "job-3", "queued", &metaJSON); err != nil {
		t.Fatalf("failed to update upload job commit: %v", err)
	}
	fetched3, err := repo.GetUploadJob(ctx, "job-3")
	if err != nil {
		t.Fatalf("failed to get job3 after commit: %v", err)
	}
	if fetched3.Status != "queued" || fetched3.Metadata == nil || *fetched3.Metadata != metaJSON {
		t.Fatalf("unexpected committed job state: %+v", fetched3)
	}

	// 9. Test DeleteUploadJob
	if err := repo.DeleteUploadJob(ctx, "job-3"); err != nil {
		t.Fatalf("failed to delete upload job: %v", err)
	}
	_, err = repo.GetUploadJob(ctx, "job-3")
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound for deleted job, got %v", err)
	}

	// 10. Test Not Found
	_, err = repo.GetUploadJob(ctx, "nonexistent-job")
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound for nonexistent job, got %v", err)
	}

	// 9. Test Book Deletion ON DELETE SET NULL
	if err := repo.DeleteBook(ctx, book.ID); err != nil {
		t.Fatalf("failed to delete book: %v", err)
	}
	fetched, _ = repo.GetUploadJob(ctx, "job-1")
	if fetched.BookID != nil {
		t.Fatalf("expected book_id to be set to NULL on book delete, got %v", *fetched.BookID)
	}
}

func TestCreateChapter_ULIDAndSpine(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer repo.Close()
	defer db.Close()

	book := &repository.Book{
		Title:    "Neuromancer",
		FilePath: "William Gibson/Neuromancer/Neuromancer.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book: %v", err)
	}

	title1 := "Chapter 1"
	title2 := "Chapter 2"

	ch1 := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &title1,
		ContentPlain: "The sky above the port was the color of television.",
	}
	ch2 := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 2,
		Title:        &title2,
		ContentPlain: "Night City was like a deranged experiment in social Darwinism.",
	}

	if err := repo.CreateChapter(ctx, ch1); err != nil {
		t.Fatalf("failed to create chapter 1: %v", err)
	}
	if err := repo.CreateChapter(ctx, ch2); err != nil {
		t.Fatalf("failed to create chapter 2: %v", err)
	}

	if !ulid.IsValid(ch1.ID) {
		t.Fatalf("expected ch1.ID to be valid ULID, got %s", ch1.ID)
	}
	if !ulid.IsValid(ch2.ID) {
		t.Fatalf("expected ch2.ID to be valid ULID, got %s", ch2.ID)
	}
	if ch1.ID >= ch2.ID {
		t.Fatalf("expected monotonic ordering ch1.ID < ch2.ID, got ch1=%s, ch2=%s", ch1.ID, ch2.ID)
	}

	spine, err := repo.GetBookSpine(ctx, book.ID)
	if err != nil {
		t.Fatalf("failed to get book spine: %v", err)
	}
	if len(spine) != 2 {
		t.Fatalf("expected 2 spine items, got %d", len(spine))
	}
	if spine[0].ID != ch1.ID || spine[0].ChapterIndex != 1 || *spine[0].Title != title1 {
		t.Errorf("unexpected spine item 0: %+v", spine[0])
	}
	if spine[1].ID != ch2.ID || spine[1].ChapterIndex != 2 || *spine[1].Title != title2 {
		t.Errorf("unexpected spine item 1: %+v", spine[1])
	}
}

func TestGetBookByFilePath(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)

	book := &repository.Book{
		ID:       "book-path-test",
		Title:    "Test Path Book",
		FilePath: "Author/Title/Book.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	found, err := repo.GetBookByFilePath(ctx, "Author/Title/Book.epub")
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}
	if found.ID != book.ID || found.Title != book.Title {
		t.Errorf("unexpected found book: %+v", found)
	}

	_, err = repo.GetBookByFilePath(ctx, "nonexistent.epub")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent file path, got %v", err)
	}
}

func TestUpdateChapterContent(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)

	book := &repository.Book{ID: "b-1", Title: "B1", FilePath: "p1"}
	_ = repo.CreateBook(ctx, book)

	ch := &repository.Chapter{
		ID:           "ch-1",
		BookID:       book.ID,
		ChapterIndex: 1,
		ContentPlain: "Old raw content",
	}
	_ = repo.CreateChapter(ctx, ch)

	newContent := "## Markdown Heading\n\nNew formatted content"
	if err := repo.UpdateChapterContent(ctx, ch.ID, newContent); err != nil {
		t.Fatalf("update content: %v", err)
	}

	updated, err := repo.GetChapterByID(ctx, ch.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if updated.ContentPlain != newContent {
		t.Errorf("expected %s, got %s", newContent, updated.ContentPlain)
	}
}

func TestBookmarksAndHighlights(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	book := &repository.Book{ID: "book-1", Title: "Dune", FilePath: "dune.epub"}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	chapter := &repository.Chapter{
		ID:           "ch-1",
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        nil,
		Summary:      "Chapter 1 summary",
		ContentPlain: "Chapter 1 content",
	}
	if err := repo.CreateChapter(ctx, chapter); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	// 1. Bookmarks CRUD
	chID := chapter.ID
	bm := &repository.Bookmark{
		BookID:    book.ID,
		ChapterID: &chID,
		Title:     "Chapter 1 - The Gom Jabbar",
		Progress:  0.25,
	}
	if err := repo.CreateBookmark(ctx, bm); err != nil {
		t.Fatalf("create bookmark: %v", err)
	}
	if bm.ID == "" {
		t.Fatalf("expected bookmark ID to be populated")
	}

	bms, err := repo.ListBookmarksByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("list bookmarks: %v", err)
	}
	if len(bms) != 1 || bms[0].Title != bm.Title {
		t.Fatalf("unexpected bookmarks: %+v", bms)
	}

	// Delete bookmark
	if err := repo.DeleteBookmark(ctx, bm.ID); err != nil {
		t.Fatalf("delete bookmark: %v", err)
	}
	bmsAfter, _ := repo.ListBookmarksByBookID(ctx, book.ID)
	if len(bmsAfter) != 0 {
		t.Fatalf("expected 0 bookmarks after deletion, got %d", len(bmsAfter))
	}

	// 2. Highlights CRUD
	note := "A powerful philosophical insight."
	startOff := 120
	endOff := 195
	startPara := 1
	endPara := 2
	loc := `{"chapter_id":"` + chID + `","start_para":1,"end_para":2}`
	hl := &repository.Highlight{
		BookID:         book.ID,
		ChapterID:      &chID,
		SelectedText:   "The mystery of life isn't a problem to solve, but a reality to experience.",
		Note:           &note,
		Color:          "orange",
		StartOffset:    &startOff,
		EndOffset:      &endOff,
		StartParagraph: &startPara,
		EndParagraph:   &endPara,
		Location:       &loc,
	}
	if err := repo.CreateHighlight(ctx, hl); err != nil {
		t.Fatalf("create highlight: %v", err)
	}
	if hl.ID == "" {
		t.Fatalf("expected highlight ID to be populated")
	}

	hls, err := repo.ListHighlightsByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("list highlights: %v", err)
	}
	if len(hls) != 1 || hls[0].SelectedText != hl.SelectedText || *hls[0].Note != note {
		t.Fatalf("unexpected highlights: %+v", hls)
	}
	if hls[0].StartOffset == nil || *hls[0].StartOffset != 120 || hls[0].EndParagraph == nil || *hls[0].EndParagraph != 2 {
		t.Fatalf("expected location offsets to be populated, got: %+v", hls[0])
	}

	// Delete highlight
	if err := repo.DeleteHighlight(ctx, hl.ID); err != nil {
		t.Fatalf("delete highlight: %v", err)
	}
	hlsAfter, _ := repo.ListHighlightsByBookID(ctx, book.ID)
	if len(hlsAfter) != 0 {
		t.Fatalf("expected 0 highlights after deletion, got %d", len(hlsAfter))
	}
}

func TestSearchVectorChaptersWithBookID(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	b1 := &repository.Book{ID: "b-1", Title: "Book One", FilePath: "b1.epub"}
	b2 := &repository.Book{ID: "b-2", Title: "Book Two", FilePath: "b2.epub"}
	_ = repo.CreateBook(ctx, b1)
	_ = repo.CreateBook(ctx, b2)

	ch1 := &repository.Chapter{ID: "ch-b1", BookID: b1.ID, ChapterIndex: 1, Summary: "Sum 1", ContentPlain: "Text 1"}
	ch2 := &repository.Chapter{ID: "ch-b2", BookID: b2.ID, ChapterIndex: 1, Summary: "Sum 2", ContentPlain: "Text 2"}
	_ = repo.CreateChapter(ctx, ch1)
	_ = repo.CreateChapter(ctx, ch2)

	// Create dummy embeddings of 256 dims
	vec1 := make([]float32, 256)
	vec1[0] = 1.0
	vec2 := make([]float32, 256)
	vec2[0] = 0.9

	_ = repo.InsertChapterVector(ctx, ch1.ID, vec1)
	_ = repo.InsertChapterVector(ctx, ch2.ID, vec2)

	queryVec := make([]float32, 256)
	queryVec[0] = 1.0

	// Filter with BookID = b1.ID
	hits, err := repo.SearchVectorChapters(ctx, queryVec, repository.SearchFilter{
		BookID: &b1.ID,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("search vector chapters: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit for book 1, got %d", len(hits))
	}
	if hits[0].BookID != b1.ID {
		t.Errorf("expected book_id %s, got %s", b1.ID, hits[0].BookID)
	}
}

func TestParagraphStorageAndSearch(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	b := &repository.Book{
		ID:       "book-p1",
		Title:    "Neuromancer",
		FilePath: "neuromancer.epub",
	}
	if err := repo.CreateBook(ctx, b); err != nil {
		t.Fatalf("create book: %v", err)
	}

	ch := &repository.Chapter{
		ID:           "ch-p1",
		BookID:       b.ID,
		ChapterIndex: 1,
		Summary:      "Chiba City Blues",
		ContentPlain: "The sky above the port was the color of television, tuned to a dead channel.",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	// 1. Create paragraphs
	paras := []*repository.Paragraph{
		{
			BookID:         b.ID,
			ChapterID:      ch.ID,
			ChapterIndex:   1,
			StartParagraph: 1,
			EndParagraph:   1,
			Content:        "The sky above the port was the color of television, tuned to a dead channel.",
		},
		{
			BookID:         b.ID,
			ChapterID:      ch.ID,
			ChapterIndex:   1,
			StartParagraph: 2,
			EndParagraph:   2,
			Content:        "It was a cool night in Chiba City. Case was drinking at the Chatsubo bar.",
		},
	}
	if err := repo.CreateParagraphs(ctx, paras); err != nil {
		t.Fatalf("create paragraphs: %v", err)
	}

	// 2. Verify GetParagraphsByChapterID
	fetchedParas, err := repo.GetParagraphsByChapterID(ctx, ch.ID)
	if err != nil {
		t.Fatalf("get paragraphs by chapter: %v", err)
	}
	if len(fetchedParas) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(fetchedParas))
	}

	// 3. Verify GetUnindexedParagraphs
	unindexed, err := repo.GetUnindexedParagraphs(ctx, 10)
	if err != nil {
		t.Fatalf("get unindexed paragraphs: %v", err)
	}
	if len(unindexed) != 2 {
		t.Fatalf("expected 2 unindexed paragraphs, got %d", len(unindexed))
	}

	// 4. Insert vector for first paragraph
	vec := make([]float32, 256)
	vec[0] = 1.0
	if err := repo.InsertParagraphVector(ctx, fetchedParas[0].ID, vec); err != nil {
		t.Fatalf("insert paragraph vector: %v", err)
	}

	// Now only 1 unindexed paragraph should remain
	unindexedAfter, err := repo.GetUnindexedParagraphs(ctx, 10)
	if err != nil {
		t.Fatalf("get unindexed paragraphs after index: %v", err)
	}
	if len(unindexedAfter) != 1 {
		t.Fatalf("expected 1 unindexed paragraph, got %d", len(unindexedAfter))
	}
	if unindexedAfter[0].ID != fetchedParas[1].ID {
		t.Errorf("expected paragraph 2 to be unindexed, got %s", unindexedAfter[0].ID)
	}

	// 5. Test SearchVectorParagraphs
	queryVec := make([]float32, 256)
	queryVec[0] = 1.0
	vecHits, err := repo.SearchVectorParagraphs(ctx, queryVec, repository.SearchFilter{Limit: 5})
	if err != nil {
		t.Fatalf("search vector paragraphs: %v", err)
	}
	if len(vecHits) != 1 {
		t.Fatalf("expected 1 vector hit, got %d", len(vecHits))
	}
	if vecHits[0].Content != paras[0].Content {
		t.Errorf("expected hit content %q, got %q", paras[0].Content, vecHits[0].Content)
	}
	if vecHits[0].BookTitle != "Neuromancer" {
		t.Errorf("expected book title Neuromancer, got %s", vecHits[0].BookTitle)
	}

	// 6. Test SearchFTSParagraphs
	ftsHits, err := repo.SearchFTSParagraphs(ctx, "Chatsubo", repository.SearchFilter{Limit: 5})
	if err != nil {
		t.Fatalf("search FTS paragraphs: %v", err)
	}
	if len(ftsHits) != 1 {
		t.Fatalf("expected 1 FTS hit for Chatsubo, got %d", len(ftsHits))
	}
	if ftsHits[0].Content != paras[1].Content {
		t.Errorf("expected hit content %q, got %q", paras[1].Content, ftsHits[0].Content)
	}
}

func TestBackfillParagraphs(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	b := &repository.Book{
		ID:       "book-bf1",
		Title:    "Snow Crash",
		FilePath: "snowcrash.epub",
	}
	if err := repo.CreateBook(ctx, b); err != nil {
		t.Fatalf("create book: %v", err)
	}

	content := "The Deliverator belongs to an elite order.\n\nHis car has enough potential energy packed into its batteries to fire a pound of bacon into the Asteroid Belt."
	ch := &repository.Chapter{
		ID:           "ch-bf1",
		BookID:       b.ID,
		ChapterIndex: 1,
		ContentPlain: content,
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	// Run backfill
	count, err := repo.BackfillParagraphs(ctx)
	if err != nil {
		t.Fatalf("backfill paragraphs: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected at least 1 backfilled paragraph, got 0")
	}

	paras, err := repo.GetParagraphsByChapterID(ctx, ch.ID)
	if err != nil {
		t.Fatalf("get paragraphs after backfill: %v", err)
	}
	if len(paras) != count {
		t.Errorf("expected %d paragraphs, got %d", count, len(paras))
	}

	// Running backfill again should process 0 chapters
	count2, err := repo.BackfillParagraphs(ctx)
	if err != nil {
		t.Fatalf("backfill paragraphs second time: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected 0 backfilled paragraphs second time, got %d", count2)
	}
}

func TestInsertParagraphVectorsBatch(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	book := &repository.Book{
		ID:       "book-batch-vec",
		Title:    "Batch Vector Test",
		FilePath: "batch.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	ch := &repository.Chapter{
		ID:           "ch-batch-vec",
		BookID:       book.ID,
		ChapterIndex: 1,
		ContentPlain: "Batch chapter text.",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	paras := []*repository.Paragraph{
		{
			ID:             "p-batch-1",
			BookID:         book.ID,
			ChapterID:      ch.ID,
			ChapterIndex:   1,
			StartParagraph: 1,
			EndParagraph:   1,
			Content:        "Paragraph 1 text",
		},
		{
			ID:             "p-batch-2",
			BookID:         book.ID,
			ChapterID:      ch.ID,
			ChapterIndex:   1,
			StartParagraph: 2,
			EndParagraph:   2,
			Content:        "Paragraph 2 text",
		},
	}
	if err := repo.CreateParagraphs(ctx, paras); err != nil {
		t.Fatalf("create paragraphs: %v", err)
	}

	unindexed, err := repo.GetUnindexedParagraphs(ctx, 10)
	if err != nil {
		t.Fatalf("get unindexed: %v", err)
	}
	if len(unindexed) != 2 {
		t.Fatalf("expected 2 unindexed, got %d", len(unindexed))
	}

	// Insert batch vectors
	vec1 := make([]float32, 256)
	vec1[0] = 0.5
	vec2 := make([]float32, 256)
	vec2[1] = 0.8
	batch := []repository.ParagraphVector{
		{ParagraphID: paras[0].ID, Embedding: vec1},
		{ParagraphID: paras[1].ID, Embedding: vec2},
	}
	if err := repo.InsertParagraphVectors(ctx, batch); err != nil {
		t.Fatalf("insert paragraph vectors batch: %v", err)
	}

	unindexedAfter, err := repo.GetUnindexedParagraphs(ctx, 10)
	if err != nil {
		t.Fatalf("get unindexed after batch: %v", err)
	}
	if len(unindexedAfter) != 0 {
		t.Errorf("expected 0 unindexed paragraphs after batch insert, got %d", len(unindexedAfter))
	}
}

func TestTopicManagementAndFiltering(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	// 1. Upsert topics
	topicAI, err := repo.UpsertTopic(ctx, "Artificial Intelligence")
	if err != nil {
		t.Fatalf("upsert topic AI: %v", err)
	}
	if topicAI.Name != "Artificial Intelligence" {
		t.Fatalf("expected Artificial Intelligence, got %s", topicAI.Name)
	}

	topicSpace, err := repo.UpsertTopic(ctx, "Space Exploration")
	if err != nil {
		t.Fatalf("upsert topic Space: %v", err)
	}

	// Upsert duplicate returns same topic
	dupAI, err := repo.UpsertTopic(ctx, "Artificial Intelligence")
	if err != nil {
		t.Fatalf("upsert duplicate AI: %v", err)
	}
	if dupAI.ID != topicAI.ID {
		t.Errorf("expected duplicate upsert to return same ID, got %s vs %s", dupAI.ID, topicAI.ID)
	}

	// 2. GetTopicByID
	fetched, err := repo.GetTopicByID(ctx, topicAI.ID)
	if err != nil {
		t.Fatalf("get topic by ID: %v", err)
	}
	if fetched.Name != topicAI.Name {
		t.Errorf("expected %s, got %s", topicAI.Name, fetched.Name)
	}

	// 3. GetTopicByName (exact, case-insensitive, substring, empty)
	exact, err := repo.GetTopicByName(ctx, "Artificial Intelligence")
	if err != nil {
		t.Fatalf("get topic by name exact: %v", err)
	}
	if exact.ID != topicAI.ID {
		t.Errorf("expected %s, got %s", topicAI.ID, exact.ID)
	}

	caseInsensitive, err := repo.GetTopicByName(ctx, "artificial intelligence")
	if err != nil {
		t.Fatalf("get topic by name case insensitive: %v", err)
	}
	if caseInsensitive.ID != topicAI.ID {
		t.Errorf("expected %s, got %s", topicAI.ID, caseInsensitive.ID)
	}

	sub, err := repo.GetTopicByName(ctx, "Intelligence")
	if err != nil {
		t.Fatalf("get topic by name substring: %v", err)
	}
	if sub.ID != topicAI.ID {
		t.Errorf("expected %s, got %s", topicAI.ID, sub.ID)
	}

	if _, err := repo.GetTopicByName(ctx, "   "); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for whitespace name, got %v", err)
	}

	// 4. ListTopics
	topics, err := repo.ListTopics(ctx)
	if err != nil {
		t.Fatalf("list topics: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(topics))
	}
	if topics[0].Name != "Artificial Intelligence" || topics[1].Name != "Space Exploration" {
		t.Errorf("unexpected topic order: %v, %v", topics[0].Name, topics[1].Name)
	}

	// 5. Create Book and link topic
	book := &repository.Book{
		ID:       "book-neuro",
		Title:    "Neuromancer",
		FilePath: "William Gibson/Neuromancer/Neuromancer.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	if err := repo.LinkBookTopic(ctx, book.ID, topicAI.ID); err != nil {
		t.Fatalf("link book topic: %v", err)
	}

	// GetBookTopics
	bookTopics, err := repo.GetBookTopics(ctx, book.ID)
	if err != nil {
		t.Fatalf("get book topics: %v", err)
	}
	if len(bookTopics) != 1 || bookTopics[0].ID != topicAI.ID {
		t.Fatalf("expected 1 topic (%s), got %v", topicAI.ID, bookTopics)
	}

	// 6. ListBooks & CountBooks with TopicID filter
	filterAI := repository.BookFilter{TopicID: &topicAI.ID}
	booksAI, err := repo.ListBooks(ctx, filterAI)
	if err != nil {
		t.Fatalf("list books with topic filter: %v", err)
	}
	if len(booksAI) != 1 || booksAI[0].ID != book.ID {
		t.Fatalf("expected Neuromancer, got %v", booksAI)
	}

	countAI, err := repo.CountBooks(ctx, filterAI)
	if err != nil {
		t.Fatalf("count books with topic filter: %v", err)
	}
	if countAI != 1 {
		t.Fatalf("expected count 1, got %d", countAI)
	}

	filterSpace := repository.BookFilter{TopicID: &topicSpace.ID}
	booksSpace, err := repo.ListBooks(ctx, filterSpace)
	if err != nil {
		t.Fatalf("list books with space topic: %v", err)
	}
	if len(booksSpace) != 0 {
		t.Fatalf("expected 0 books for space topic, got %d", len(booksSpace))
	}

	// 7. UnlinkBookTopic
	if err := repo.UnlinkBookTopic(ctx, book.ID, topicAI.ID); err != nil {
		t.Fatalf("unlink book topic: %v", err)
	}
	bookTopicsAfter, err := repo.GetBookTopics(ctx, book.ID)
	if err != nil {
		t.Fatalf("get book topics after unlink: %v", err)
	}
	if len(bookTopicsAfter) != 0 {
		t.Fatalf("expected 0 book topics after unlink, got %d", len(bookTopicsAfter))
	}
}

func TestGenreUnlinkingAndPruning(t *testing.T) {
	ctx := context.Background()
	_, repo := setupTestDB(t)
	defer repo.Close()

	g1, err := repo.UpsertGenre(ctx, "Cyberpunk")
	if err != nil {
		t.Fatalf("upsert g1: %v", err)
	}
	g2, err := repo.UpsertGenre(ctx, "Orphaned Genre")
	if err != nil {
		t.Fatalf("upsert g2: %v", err)
	}

	book := &repository.Book{
		ID:       "book-genre-prune",
		Title:    "Count Zero",
		FilePath: "William Gibson/Count Zero/Count Zero.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	if err := repo.LinkBookGenre(ctx, book.ID, g1.ID); err != nil {
		t.Fatalf("link book genre: %v", err)
	}

	// Pruning now should only remove g2
	pruned, err := repo.PruneOrphanedGenres(ctx)
	if err != nil {
		t.Fatalf("prune genres: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned genre, got %d", pruned)
	}

	// g1 still exists
	if _, err := repo.GetGenreByID(ctx, g1.ID); err != nil {
		t.Fatalf("expected g1 to exist, got %v", err)
	}
	// g2 is gone
	if _, err := repo.GetGenreByID(ctx, g2.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected g2 to be ErrNotFound, got %v", err)
	}

	// Unlink g1 from book and prune
	if err := repo.UnlinkBookGenre(ctx, book.ID, g1.ID); err != nil {
		t.Fatalf("unlink book genre: %v", err)
	}
	pruned2, err := repo.PruneOrphanedGenres(ctx)
	if err != nil {
		t.Fatalf("prune genres 2: %v", err)
	}
	if pruned2 != 1 {
		t.Fatalf("expected 1 pruned genre, got %d", pruned2)
	}
	if _, err := repo.GetGenreByID(ctx, g1.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected g1 to be ErrNotFound after unlink and prune, got %v", err)
	}
}

