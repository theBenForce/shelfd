package repository_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
)

func setupTestDB(t *testing.T) (*sql.DB, repository.StorageEngine) {
	t.Helper()
	ctx := context.Background()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}

	if err := database.RunMigrations(ctx, db); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	repo := repository.NewSQLiteStorageEngine(db)
	return db, repo
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

	// 5. Insert 1536-dimensional vector embedding for chapter
	embedding := make([]float32, 1536)
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

	var count int
	db.QueryRowContext(ctx, "SELECT count(*) FROM vec_chapters WHERE chapter_id = ?", chapter.ID).Scan(&count)
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

	// Verify vector is cascade-cleaned up via trigger
	db.QueryRowContext(ctx, "SELECT count(*) FROM vec_chapters WHERE chapter_id = ?", chapter.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 vec_chapters after delete, got %d", count)
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
	book1 := &repository.Book{
		Title:         "Neuromancer",
		Description:   &desc,
		FilePath:      "William Gibson/Neuromancer/Neuromancer.epub",
		FileSizeBytes: &size,
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
	book1.Title = newTitle
	if err := repo.UpdateBook(ctx, book1); err != nil {
		t.Fatalf("update book: %v", err)
	}

	fetched, err := repo.GetBookByID(ctx, book1.ID)
	if err != nil || fetched.Title != newTitle {
		t.Fatalf("expected updated title %s, got %v", newTitle, fetched)
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

	// Pagination
	allBooks, err := repo.ListBooks(ctx, repository.BookFilter{Limit: 1, Offset: 0})
	if err != nil || len(allBooks) != 1 {
		t.Fatalf("expected 1 book with limit 1, got %v", allBooks)
	}
	offsetBooks, err := repo.ListBooks(ctx, repository.BookFilter{Limit: 1, Offset: 1})
	if err != nil || len(offsetBooks) != 1 || offsetBooks[0].ID == allBooks[0].ID {
		t.Fatalf("expected different book with offset 1, got %v", offsetBooks)
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
}
