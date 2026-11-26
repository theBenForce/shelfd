package ai_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
)

func init() {
	sqlite_vec.Auto()
}

type spyRecapChatClient struct {
	chatCalls      int
	summarizeCalls int
	chatMessages   [][]ai.ChatMessage
	summarized     []string
	chatReply      string
}

func (s *spyRecapChatClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	s.summarizeCalls++
	s.summarized = append(s.summarized, title+"::"+content)
	return fmt.Sprintf("Backfilled summary for %s", title), nil
}

func (s *spyRecapChatClient) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	s.chatCalls++
	copied := make([]ai.ChatMessage, len(messages))
	copy(copied, messages)
	s.chatMessages = append(s.chatMessages, copied)
	if s.chatReply != "" {
		return s.chatReply, nil
	}
	return fmt.Sprintf("Recap generation #%d", s.chatCalls), nil
}

func (s *spyRecapChatClient) ChatWithTools(ctx context.Context, messages []ai.ChatMessage, tools []ai.ToolDefinition) (*ai.ChatResponse, error) {
	reply, err := s.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	return &ai.ChatResponse{Content: reply}, nil
}

func newRecapTestRepo(t *testing.T) repository.StorageEngine {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "recap_test.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := database.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func seedRecapBook(t *testing.T, repo repository.StorageEngine) string {
	t.Helper()
	ctx := context.Background()
	book := &repository.Book{
		ID:       "book_dune",
		Title:    "Dune",
		FilePath: "dune.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	ch1Title := "Caladan"
	ch2Title := "Arrakis"
	ch3Title := "The Betrayal"
	ch4Title := "Desert Escape"

	chapters := []*repository.Chapter{
		{
			ID:           "ch_1",
			BookID:       book.ID,
			ChapterIndex: 1,
			Title:        &ch1Title,
			Summary:      "Paul passes the Gom Jabbar test on Caladan.",
			ContentPlain: "Paul faced the Reverend Mother and endured the pain box.",
		},
		{
			ID:           "ch_2",
			BookID:       book.ID,
			ChapterIndex: 2,
			Title:        &ch2Title,
			Summary:      "",
			ContentPlain: "House Atreides arrives on Arrakis and inspects the spice harvesters.",
		},
		{
			ID:           "ch_3",
			BookID:       book.ID,
			ChapterIndex: 3,
			Title:        &ch3Title,
			Summary:      "SECRET_SPOILER_CH3: Dr. Yueh lowers the shields for the Harkonnens.",
			ContentPlain: "SECRET_SPOILER_CH3_PLAIN: Baron Harkonnen captures Duke Leto.",
		},
		{
			ID:           "ch_4",
			BookID:       book.ID,
			ChapterIndex: 4,
			Title:        &ch4Title,
			Summary:      "SECRET_SPOILER_CH4: Paul and Jessica flee into the deep desert.",
			ContentPlain: "SECRET_SPOILER_CH4_PLAIN: Sandworms pursue them across the dunes.",
		},
	}

	for _, ch := range chapters {
		if err := repo.CreateChapter(ctx, ch); err != nil {
			t.Fatalf("create chapter %d: %v", ch.ChapterIndex, err)
		}
	}
	return book.ID
}

func TestRecapService_AntiSpoilerBoundaryAndBackfill(t *testing.T) {
	repo := newRecapTestRepo(t)
	bookID := seedRecapBook(t, repo)
	spy := &spyRecapChatClient{chatReply: "Paul endured the Gom Jabbar and reached Arrakis."}

	svc := ai.NewRecapService(repo, spy, nil)
	res, err := svc.Generate(context.Background(), bookID, 2, 0, false)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if res.Cached {
		t.Fatalf("expected first recap call to be uncached")
	}
	if res.ChapterIndex != 2 || res.ChaptersIncluded != 2 {
		t.Fatalf("expected chapter ceiling 2 with 2 chapters included, got ceiling=%d included=%d", res.ChapterIndex, res.ChaptersIncluded)
	}
	if spy.chatCalls != 1 {
		t.Fatalf("expected 1 Chat call, got %d", spy.chatCalls)
	}
	if spy.summarizeCalls != 1 {
		t.Fatalf("expected 1 SummarizeChapter backfill for chapter 2, got %d", spy.summarizeCalls)
	}

	for _, item := range spy.summarized {
		if strings.Contains(item, "SECRET_SPOILER") {
			t.Fatalf("spoiler text leaked to SummarizeChapter: %s", item)
		}
	}
	for _, msg := range spy.chatMessages[0] {
		if strings.Contains(msg.Content, "SECRET_SPOILER") || strings.Contains(msg.Content, "Yueh") || strings.Contains(msg.Content, "Sandworms") {
			t.Fatalf("spoiler text from chapters > 2 leaked into LLM prompt: %s", msg.Content)
		}
	}

	ch2, err := repo.GetChapterByBookAndIndex(context.Background(), bookID, 2)
	if err != nil {
		t.Fatalf("fetch chapter 2: %v", err)
	}
	if !strings.Contains(ch2.Summary, "Backfilled summary") {
		t.Fatalf("expected chapter 2 summary to be persisted, got %q", ch2.Summary)
	}
}

func TestRecapService_CacheHitAndForceRefresh(t *testing.T) {
	repo := newRecapTestRepo(t)
	bookID := seedRecapBook(t, repo)
	spy := &spyRecapChatClient{}

	svc := ai.NewRecapService(repo, spy, nil)
	ctx := context.Background()

	first, err := svc.Generate(ctx, bookID, 2, 5, false)
	if err != nil {
		t.Fatalf("first Generate failed: %v", err)
	}
	if first.Cached || spy.chatCalls != 1 {
		t.Fatalf("expected first call uncached with 1 LLM call, got cached=%v calls=%d", first.Cached, spy.chatCalls)
	}

	second, err := svc.Generate(ctx, bookID, 2, 5, false)
	if err != nil {
		t.Fatalf("second Generate failed: %v", err)
	}
	if !second.Cached {
		t.Fatalf("expected second call to hit recap_cache")
	}
	if second.Recap != first.Recap {
		t.Fatalf("expected cached recap %q, got %q", first.Recap, second.Recap)
	}
	if spy.chatCalls != 1 {
		t.Fatalf("expected cache hit to skip LLM, got %d calls", spy.chatCalls)
	}

	refreshed, err := svc.Generate(ctx, bookID, 2, 5, true)
	if err != nil {
		t.Fatalf("forceRefresh Generate failed: %v", err)
	}
	if refreshed.Cached {
		t.Fatalf("expected forceRefresh to bypass cache")
	}
	if spy.chatCalls != 2 {
		t.Fatalf("expected forceRefresh to invoke LLM again, got %d calls", spy.chatCalls)
	}
	if refreshed.Recap == first.Recap {
		t.Fatalf("expected refreshed recap to differ from original")
	}
}

func TestRecapService_ChapterZeroAndValidation(t *testing.T) {
	repo := newRecapTestRepo(t)
	bookID := seedRecapBook(t, repo)
	spy := &spyRecapChatClient{}

	svc := ai.NewRecapService(repo, spy, nil)
	ctx := context.Background()

	if _, err := svc.Generate(ctx, bookID, -1, 0, false); !errors.Is(err, ai.ErrInvalidRecapPosition) {
		t.Fatalf("expected ErrInvalidRecapPosition for negative chapter, got %v", err)
	}
	if _, err := svc.Generate(ctx, bookID, 1, -3, false); !errors.Is(err, ai.ErrInvalidRecapPosition) {
		t.Fatalf("expected ErrInvalidRecapPosition for negative paragraph offset, got %v", err)
	}

	opening, err := svc.Generate(ctx, bookID, 0, 0, false)
	if err != nil {
		t.Fatalf("chapter 0 Generate failed: %v", err)
	}
	if spy.chatCalls != 0 {
		t.Fatalf("expected chapter 0 opening context to skip LLM, got %d calls", spy.chatCalls)
	}
	if !strings.Contains(opening.Recap, "beginning") {
		t.Fatalf("expected graceful opening-context guidance, got %q", opening.Recap)
	}
}
