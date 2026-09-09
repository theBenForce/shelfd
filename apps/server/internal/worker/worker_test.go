package worker_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/worker"
)

func init() {
	sqlite_vec.Auto()
}

type mockAIClient struct {
	mu            sync.Mutex
	summaryErr    error
	embeddingErr  error
	summaryCalled int
	embedCalled   int
}

func (m *mockAIClient) SummarizeChapter(ctx context.Context, title, chapterText string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summaryCalled++
	if m.summaryErr != nil {
		return "", m.summaryErr
	}
	return "Summary: " + chapterText[:min(20, len(chapterText))], nil
}

func (m *mockAIClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embedCalled++
	if m.embeddingErr != nil {
		return nil, m.embeddingErr
	}
	vec := make([]float32, 256)
	vec[0] = 1.0
	return vec, nil
}

func (m *mockAIClient) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	return "Mock chat response", nil
}

func setupTestDB(t *testing.T) (*sql.DB, repository.StorageEngine) {
	t.Helper()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := database.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)
	return db, repo
}

func TestWorker_ProcessBatch_Success(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	book := &repository.Book{
		Title:    "Dune",
		FilePath: "Frank Herbert/Dune/Dune.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	for i := 1; i <= 3; i++ {
		ch := &repository.Chapter{
			BookID:       book.ID,
			ChapterIndex: i,
			ContentPlain: "A beginning is a very delicate time in the universe.",
		}
		if err := repo.CreateChapter(ctx, ch); err != nil {
			t.Fatalf("create chapter %d: %v", i, err)
		}
	}

	aiMock := &mockAIClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewWorker(repo, aiMock, worker.Config{
		BatchSize: 10,
		Logger:    logger,
	})

	count, err := w.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 chapters processed, got %d", count)
	}

	unindexed, err := repo.GetUnindexedChapters(ctx, 10)
	if err != nil {
		t.Fatalf("get unindexed: %v", err)
	}
	if len(unindexed) != 0 {
		t.Fatalf("expected 0 unindexed chapters, got %d", len(unindexed))
	}

	// Verify second run processes 0
	count2, err := w.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("process batch 2: %v", err)
	}
	if count2 != 0 {
		t.Fatalf("expected 0 chapters processed on second run, got %d", count2)
	}
}

func TestWorker_ProcessBatch_EmptyContent(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	book := &repository.Book{
		Title:    "Empty Notes",
		FilePath: "Author/Book/Book.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	title := "Frontispiece"
	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &title,
		ContentPlain: "   \n\t  ",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	aiMock := &mockAIClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewWorker(repo, aiMock, worker.Config{
		BatchSize: 10,
		Logger:    logger,
	})

	count, err := w.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("process batch with empty content: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 chapter processed, got %d", count)
	}

	fetched, err := repo.GetChapterByID(ctx, ch.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if fetched.Summary == "" {
		t.Errorf("expected non-empty summary for empty content chapter")
	}
}

func TestWorker_ProcessBatch_AiErrors(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	book := &repository.Book{
		Title:    "Faulty Book",
		FilePath: "Author/Book/Book.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		ContentPlain: "Testing error handling.",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	// 1. Summarization error
	aiMock := &mockAIClient{summaryErr: errors.New("upstream LLM timeout")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewWorker(repo, aiMock, worker.Config{
		BatchSize: 10,
		Logger:    logger,
	})

	count, err := w.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("expected no fatal error on summarization failure, got %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 processed due to error, got %d", count)
	}

	// 2. Embedding error on paragraph
	para := &repository.Paragraph{
		BookID:         book.ID,
		ChapterID:      ch.ID,
		ChapterIndex:   1,
		StartParagraph: 1,
		EndParagraph:   1,
		Content:        "Testing paragraph embedding error handling.",
	}
	if err := repo.CreateParagraphs(ctx, []*repository.Paragraph{para}); err != nil {
		t.Fatalf("create paragraph: %v", err)
	}

	aiMock.summaryErr = nil
	aiMock.embeddingErr = errors.New("upstream embedding failed")
	count, err = w.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("expected no fatal error on embedding failure, got %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 processed due to embedding error, got %d", count)
	}
}

func TestWorker_BackgroundTriggerAndStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	book := &repository.Book{
		Title:    "Trigger Test",
		FilePath: "Author/Book/Book.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		ContentPlain: "Immediate background processing.",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	aiMock := &mockAIClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewWorker(repo, aiMock, worker.Config{
		BatchSize:    10,
		PollInterval: 500 * time.Millisecond,
		Logger:       logger,
	})

	w.Start(ctx)
	w.Trigger()

	// Wait up to 2 seconds for worker to process
	deadline := time.Now().Add(2 * time.Second)
	success := false
	for time.Now().Before(deadline) {
		unindexed, err := repo.GetUnindexedChapters(ctx, 10)
		if err == nil && len(unindexed) == 0 {
			success = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !success {
		t.Fatalf("expected chapter to be processed by background worker via trigger")
	}

	w.Stop()
}

func TestWorker_ProcessParagraphs(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	book := &repository.Book{
		Title:    "Neuromancer",
		FilePath: "William Gibson/Neuromancer/Neuromancer.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		ContentPlain: "The sky above the port was the color of television, tuned to a dead channel.",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	para := &repository.Paragraph{
		BookID:         book.ID,
		ChapterID:      ch.ID,
		ChapterIndex:   1,
		StartParagraph: 1,
		EndParagraph:   1,
		Content:        ch.ContentPlain,
	}
	if err := repo.CreateParagraphs(ctx, []*repository.Paragraph{para}); err != nil {
		t.Fatalf("create paragraph: %v", err)
	}

	aiMock := &mockAIClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewWorker(repo, aiMock, worker.Config{
		BatchSize: 10,
		Logger:    logger,
	})

	n, err := w.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("ProcessBatch error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 processed paragraph, got %d", n)
	}

	// Verify paragraph is indexed in vec_paragraphs
	unindexed, err := repo.GetUnindexedParagraphs(ctx, 10)
	if err != nil {
		t.Fatalf("GetUnindexedParagraphs error: %v", err)
	}
	if len(unindexed) != 0 {
		t.Errorf("expected 0 unindexed paragraphs, got %d", len(unindexed))
	}
}
