package study_test

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/study"
)

func init() {
	sqlite_vec.Auto()
}

type stubChatClient struct {
	reply string
	err   error
}

func (s *stubChatClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	return "", nil
}

func (s *stubChatClient) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.reply, nil
}

func (s *stubChatClient) ChatWithTools(ctx context.Context, messages []ai.ChatMessage, tools []ai.ToolDefinition) (*ai.ChatResponse, error) {
	return &ai.ChatResponse{Content: s.reply}, nil
}

func TestStripMarkdownCodeFence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain json",
			in:   `{"concepts":[]}`,
			want: `{"concepts":[]}`,
		},
		{
			name: "json fence",
			in:   "```json\n{\"concepts\":[{\"title\":\"A\"}]}\n```",
			want: `{"concepts":[{"title":"A"}]}`,
		},
		{
			name: "fence with surrounding prose",
			in:   "Here is the output:\n```\n[{\"front\":\"Q\",\"back\":\"A\"}]\n```\nHope this helps!",
			want: `[{"front":"Q","back":"A"}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := study.StripMarkdownCodeFence(tc.in); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildAnkiAPKG_IntegrityAndSchema(t *testing.T) {
	cards := []study.StudyCard{
		{
			Front:          "What is the Gom Jabbar?",
			Back:           "A poison needle used by the Bene Gesserit to test human awareness.",
			Tags:           []string{"dune", "bene gesserit"},
			BookID:         "book-1",
			ChapterIndex:   1,
			StartParagraph: 2,
			EndParagraph:   4,
		},
		{
			Front:          "Who administers the test to Paul Atreides?",
			Back:           "Reverend Mother Gaius Helen Mohiam.",
			Tags:           []string{"dune", "characters"},
			BookID:         "book-1",
			ChapterIndex:   1,
			StartParagraph: 5,
			EndParagraph:   6,
		},
	}

	apkg, err := study.BuildAnkiAPKG("Dune Study Deck", cards)
	if err != nil {
		t.Fatalf("BuildAnkiAPKG failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(apkg), int64(len(apkg)))
	if err != nil {
		t.Fatalf("open apkg zip: %v", err)
	}

	entries := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		entries[f.Name] = data
	}

	mediaBytes, ok := entries["media"]
	if !ok {
		t.Fatalf("missing media entry in apkg")
	}
	if string(mediaBytes) != "{}" {
		t.Fatalf("expected media '{}', got %q", string(mediaBytes))
	}

	colBytes, ok := entries["collection.anki2"]
	if !ok {
		t.Fatalf("missing collection.anki2 entry in apkg")
	}

	tmpDB := filepath.Join(t.TempDir(), "collection.anki2")
	if err := os.WriteFile(tmpDB, colBytes, 0o600); err != nil {
		t.Fatalf("write extracted collection.anki2: %v", err)
	}

	db, err := sql.Open("sqlite3", tmpDB)
	if err != nil {
		t.Fatalf("open extracted sqlite db: %v", err)
	}
	defer db.Close()

	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("PRAGMA integrity_check failed: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("expected PRAGMA integrity_check 'ok', got %q", integrity)
	}

	for _, table := range []string{"col", "notes", "cards", "revlog", "graves"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %q in collection.anki2: %v", table, err)
		}
	}

	var colCount, noteCount, cardCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM col`).Scan(&colCount); err != nil {
		t.Fatalf("count col: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&noteCount); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cardCount); err != nil {
		t.Fatalf("count cards: %v", err)
	}
	if colCount != 1 || noteCount != 2 || cardCount != 2 {
		t.Fatalf("unexpected row counts: col=%d notes=%d cards=%d", colCount, noteCount, cardCount)
	}

	var decksJSON string
	if err := db.QueryRow(`SELECT decks FROM col WHERE id = 1`).Scan(&decksJSON); err != nil {
		t.Fatalf("select decks: %v", err)
	}
	if !strings.Contains(decksJSON, "Dune Study Deck") {
		t.Fatalf("expected deck name in col.decks, got %s", decksJSON)
	}

	var flds, tags string
	if err := db.QueryRow(`SELECT flds, tags FROM notes ORDER BY id ASC LIMIT 1`).Scan(&flds, &tags); err != nil {
		t.Fatalf("select first note: %v", err)
	}
	if !strings.Contains(flds, "What is the Gom Jabbar?\x1f") {
		t.Fatalf("expected unit-separator delimited flds, got %q", flds)
	}
	if !strings.Contains(flds, "Chapter 1, ¶2–4") {
		t.Fatalf("expected paragraph attribution in note back, got %q", flds)
	}
	if !strings.Contains(tags, "bene_gesserit") {
		t.Fatalf("expected sanitized tag in notes.tags, got %q", tags)
	}
}

func TestBuildAnkiCSV_RFC4180Escaping(t *testing.T) {
	cards := []study.StudyCard{
		{
			Front:          `Explain "fear", the mind-killer`,
			Back:           "Line one, with comma\nLine two with \"quotes\"",
			Tags:           []string{"litany", "chapter 1"},
			BookID:         "book-1",
			ChapterIndex:   1,
			StartParagraph: 1,
			EndParagraph:   2,
		},
	}

	raw, err := study.BuildAnkiCSV(cards)
	if err != nil {
		t.Fatalf("BuildAnkiCSV failed: %v", err)
	}

	records, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatalf("parse generated csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected header + 1 row, got %d", len(records))
	}
	if records[0][0] != "Front" || records[0][1] != "Back" || records[0][2] != "Tags" {
		t.Fatalf("unexpected csv header: %v", records[0])
	}
	if records[1][0] != cards[0].Front || records[1][1] != cards[0].Back {
		t.Fatalf("csv round-trip mismatch: got %q / %q", records[1][0], records[1][1])
	}
	if records[1][2] != "litany chapter_1" {
		t.Fatalf("unexpected tags column: %q", records[1][2])
	}
}

func TestBuildMarkdownVaultZip_EntriesAndBacklinks(t *testing.T) {
	concepts := []study.StudyConcept{
		{
			Title:           "Gom Jabbar",
			Summary:         "The Bene Gesserit test of human awareness.",
			RelatedConcepts: []string{"Litany Against Fear"},
			BookID:          "book-1",
			ChapterIndex:    1,
			StartParagraph:  1,
			EndParagraph:    3,
		},
		{
			Title:           "Litany Against Fear",
			Summary:         "Mental discipline recited to master terror.",
			RelatedConcepts: []string{"Gom Jabbar"},
			BookID:          "book-1",
			ChapterIndex:    1,
			StartParagraph:  4,
			EndParagraph:    5,
		},
	}
	cards := []study.StudyCard{
		{
			Front:          "What is the purpose of the Gom Jabbar?",
			Back:           "To separate humans from animals by testing impulse control.",
			BookID:         "book-1",
			ChapterIndex:   1,
			StartParagraph: 1,
			EndParagraph:   3,
		},
	}

	zipBytes, err := study.BuildMarkdownVaultZip("Dune", concepts, cards)
	if err != nil {
		t.Fatalf("BuildMarkdownVaultZip failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("open markdown vault zip: %v", err)
	}

	files := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		files[f.Name] = string(b)
	}

	for _, required := range []string{
		"README.md",
		"concepts/gom-jabbar.md",
		"concepts/litany-against-fear.md",
		"chapters/chapter-01.md",
	} {
		if _, ok := files[required]; !ok {
			t.Fatalf("missing expected vault entry %q; got keys %v", required, files)
		}
	}

	if !strings.Contains(files["concepts/gom-jabbar.md"], "[[concepts/litany-against-fear|Litany Against Fear]]") {
		t.Fatalf("expected wiki backlink in concept note, got:\n%s", files["concepts/gom-jabbar.md"])
	}
	if !strings.Contains(files["README.md"], "[[chapters/chapter-01|Chapter 1]]") {
		t.Fatalf("expected chapter backlink in README, got:\n%s", files["README.md"])
	}
	if !strings.Contains(files["chapters/chapter-01.md"], "What is the purpose of the Gom Jabbar?") {
		t.Fatalf("expected flashcard in chapter index note, got:\n%s", files["chapters/chapter-01.md"])
	}
}

func TestExtractor_LLMAndFallbackAttribution(t *testing.T) {
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)
	defer repo.Close()

	book := &repository.Book{
		Title:    "Dune",
		FilePath: "Frank Herbert/Dune/Dune.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	title := "Arrival"
	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &title,
		ContentPlain: "In the week before their departure to Arrakis, Paul faced the Reverend Mother.\n\nThe Gom Jabbar waited at his neck while his hand burned inside the box.\n\nI must not fear. Fear is the mind-killer.",
	}
	if err := repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	fencedConcepts := "```json\n{\"concepts\":[{\"title\":\"Gom Jabbar\",\"summary\":\"Poison test of human awareness.\",\"start_paragraph\":2,\"end_paragraph\":2}]}\n```"
	ext := study.NewExtractor(repo, &stubChatClient{reply: fencedConcepts}, nil)
	concepts, err := ext.ExtractConcepts(ctx, study.ExtractRequest{
		BookID:       book.ID,
		ChapterIndex: 1,
	})
	if err != nil {
		t.Fatalf("ExtractConcepts failed: %v", err)
	}
	if len(concepts) != 1 || concepts[0].BookID != book.ID || concepts[0].ChapterIndex != 1 || concepts[0].StartParagraph != 2 {
		t.Fatalf("unexpected extracted concepts: %+v", concepts)
	}

	fallbackExt := study.NewExtractor(repo, &stubChatClient{reply: "not valid json"}, nil)
	cards, err := fallbackExt.GenerateCards(ctx, study.ExtractRequest{
		BookID:       book.ID,
		ChapterIndex: 1,
		MaxItems:     2,
	})
	if err != nil {
		t.Fatalf("GenerateCards fallback failed: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 fallback cards, got %d", len(cards))
	}
	for _, c := range cards {
		if c.BookID != book.ID || c.ChapterIndex != 1 || c.StartParagraph < 1 || c.EndParagraph < c.StartParagraph {
			t.Fatalf("invalid attribution on fallback card: %+v", c)
		}
	}
}

func BenchmarkBuildAnkiAPKG(b *testing.B) {
	cards := make([]study.StudyCard, 50)
	for i := range cards {
		cards[i] = study.StudyCard{
			Front:          fmt.Sprintf("Question %d: What principle governs passage %d?", i+1, i+1),
			Back:           fmt.Sprintf("Answer %d: Detailed explanation grounded in the chapter text.", i+1),
			Tags:           []string{"shelfd", fmt.Sprintf("chapter-%d", (i%5)+1)},
			BookID:         "bench-book-1",
			ChapterIndex:   (i % 5) + 1,
			StartParagraph: i + 1,
			EndParagraph:   i + 2,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := study.BuildAnkiAPKG("Benchmark Deck", cards)
		if err != nil {
			b.Fatalf("BuildAnkiAPKG failed: %v", err)
		}
		if len(out) == 0 {
			b.Fatalf("empty apkg output")
		}
	}
}
