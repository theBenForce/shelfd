package taxonomy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/taxonomy"
)

type mockAIClient struct {
	chatFunc func(ctx context.Context, messages []ai.ChatMessage) (string, error)
}

func (m *mockAIClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	return "", nil
}

func (m *mockAIClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 256), nil
}

func (m *mockAIClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range res {
		res[i] = make([]float32, 256)
	}
	return res, nil
}

func (m *mockAIClient) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, messages)
	}
	return "", errors.New("chat not implemented")
}

func setupTestDB(t *testing.T) repository.StorageEngine {
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

	return repository.NewBunStorageEngine(bunDB)
}

func TestNormalizeBookTaxonomy_AI(t *testing.T) {
	ctx := context.Background()
	repo := setupTestDB(t)
	defer repo.Close()

	// 1. Create book and initial raw genres
	desc := "A computer hacker is hired by a mysterious employer to pull off the ultimate hack against an AI."
	book := &repository.Book{
		ID:          "book-neuro",
		Title:       "Neuromancer",
		Description: &desc,
		FilePath:    "William Gibson/Neuromancer/Neuromancer.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	rawGenre, err := repo.UpsertGenre(ctx, "FICTION / Science Fiction / General")
	if err != nil {
		t.Fatalf("upsert raw genre: %v", err)
	}
	if err := repo.LinkBookGenre(ctx, book.ID, rawGenre.ID); err != nil {
		t.Fatalf("link raw genre: %v", err)
	}

	mockAI := &mockAIClient{
		chatFunc: func(ctx context.Context, messages []ai.ChatMessage) (string, error) {
			return `{"genres": ["Science Fiction", "Cyberpunk"], "topics": ["Artificial Intelligence", "Virtual Reality"]}`, nil
		},
	}

	service := taxonomy.NewTaxonomyService(repo, mockAI, nil)

	res, err := service.NormalizeBookTaxonomy(ctx, book.ID, false)
	if err != nil {
		t.Fatalf("normalize book taxonomy: %v", err)
	}

	if len(res.Genres) != 2 || res.Genres[0] != "Science Fiction" || res.Genres[1] != "Cyberpunk" {
		t.Errorf("unexpected genres: %v", res.Genres)
	}
	if len(res.Topics) != 2 || res.Topics[0] != "Artificial Intelligence" || res.Topics[1] != "Virtual Reality" {
		t.Errorf("unexpected topics: %v", res.Topics)
	}

	// Verify old raw genre was unlinked
	genres, err := repo.GetBookGenres(ctx, book.ID)
	if err != nil {
		t.Fatalf("get book genres: %v", err)
	}
	for _, g := range genres {
		if g.ID == rawGenre.ID {
			t.Errorf("expected raw genre to be unlinked, but still found %s", g.Name)
		}
	}

	// Verify topics were stored and linked
	topics, err := repo.GetBookTopics(ctx, book.ID)
	if err != nil {
		t.Fatalf("get book topics: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(topics))
	}
}

func TestNormalizeBookTaxonomy_Fallback(t *testing.T) {
	ctx := context.Background()
	repo := setupTestDB(t)
	defer repo.Close()

	desc := "A novel exploring artificial intelligence and cybernetics in the future."
	book := &repository.Book{
		ID:          "book-fall",
		Title:       "Future Shock",
		Description: &desc,
		FilePath:    "Author/Future/Future.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	rawGenre, err := repo.UpsertGenre(ctx, "FICTION / Science Fiction / Space Exploration")
	if err != nil {
		t.Fatalf("upsert raw genre: %v", err)
	}
	if err := repo.LinkBookGenre(ctx, book.ID, rawGenre.ID); err != nil {
		t.Fatalf("link raw genre: %v", err)
	}

	// No AI client -> deterministic fallback
	service := taxonomy.NewTaxonomyService(repo, nil, nil)

	res, err := service.NormalizeBookTaxonomy(ctx, book.ID, false)
	if err != nil {
		t.Fatalf("normalize book taxonomy: %v", err)
	}

	// Fallback should have extracted "Science Fiction" as genre and "Space Exploration" as topic, plus "Artificial Intelligence" from description
	if len(res.Genres) == 0 || res.Genres[0] != "Science Fiction" {
		t.Errorf("expected Science Fiction genre, got %v", res.Genres)
	}

	topics, err := repo.GetBookTopics(ctx, book.ID)
	if err != nil {
		t.Fatalf("get book topics: %v", err)
	}
	if len(topics) == 0 {
		t.Fatalf("expected topics, got 0")
	}
}

func TestMigrateLibraryTopics(t *testing.T) {
	ctx := context.Background()
	repo := setupTestDB(t)
	defer repo.Close()

	b1 := &repository.Book{ID: "b1", Title: "Book One", FilePath: "a/b1.epub"}
	b2 := &repository.Book{ID: "b2", Title: "Book Two", FilePath: "a/b2.epub"}
	_ = repo.CreateBook(ctx, b1)
	_ = repo.CreateBook(ctx, b2)

	gOrphan, _ := repo.UpsertGenre(ctx, "Orphan To Prune")
	g1, _ := repo.UpsertGenre(ctx, "FICTION / Fantasy / General")
	_ = repo.LinkBookGenre(ctx, b1.ID, g1.ID)

	service := taxonomy.NewTaxonomyService(repo, nil, nil)
	migrated, err := service.MigrateLibraryTopics(ctx, false)
	if err != nil {
		t.Fatalf("migrate library topics: %v", err)
	}
	if migrated != 2 {
		t.Errorf("expected 2 migrated books, got %d", migrated)
	}

	// Orphaned genre should have been pruned
	if _, err := repo.GetGenreByID(ctx, gOrphan.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected orphan genre to be pruned, got err: %v", err)
	}
}
