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

	// CountBooks verification
	totalCount, err := repo.CountBooks(ctx, repository.BookFilter{})
	if err != nil || totalCount != 2 {
		t.Fatalf("expected 2 total books, got %d (err: %v)", totalCount, err)
	}
	authorCount, err := repo.CountBooks(ctx, repository.BookFilter{AuthorID: &author2.ID})
	if err != nil || authorCount != 1 {
		t.Fatalf("expected 1 book by author2 count, got %d", authorCount)
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
	vec1 := make([]float32, 1536)
	vec1[0] = 1.0 // Vector pointing along dimension 0
	vec2 := make([]float32, 1536)
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
	queryVec := make([]float32, 1536)
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

	// 8. Test Not Found
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


