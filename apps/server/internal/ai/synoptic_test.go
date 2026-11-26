package ai_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
)

type stubSynopticRetriever struct {
	vecHits    []*repository.SearchHit
	ftsHits    []*repository.SearchHit
	vecErr     error
	ftsErr     error
	lastFilter repository.SearchFilter
	lastFTSQ   string
}

func (r *stubSynopticRetriever) SearchVectorParagraphs(_ context.Context, _ []float32, filter repository.SearchFilter) ([]*repository.SearchHit, error) {
	r.lastFilter = filter
	if r.vecErr != nil {
		return nil, r.vecErr
	}
	return r.vecHits, nil
}

func (r *stubSynopticRetriever) SearchFTSParagraphs(_ context.Context, query string, filter repository.SearchFilter) ([]*repository.SearchHit, error) {
	r.lastFilter = filter
	r.lastFTSQ = query
	if r.ftsErr != nil {
		return nil, r.ftsErr
	}
	return r.ftsHits, nil
}

type stubSynopticAIClient struct {
	embedVec     []float32
	embedErr     error
	chatReply    string
	chatErr      error
	chatCalls    int
	lastMessages []ai.ChatMessage
}

func (c *stubSynopticAIClient) GenerateEmbedding(_ context.Context, _ string) ([]float32, error) {
	if c.embedErr != nil {
		return nil, c.embedErr
	}
	if len(c.embedVec) > 0 {
		return c.embedVec, nil
	}
	vec := make([]float32, 256)
	vec[0] = 0.5
	return vec, nil
}

func (c *stubSynopticAIClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := c.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		out[i] = vec
	}
	return out, nil
}

func (c *stubSynopticAIClient) SummarizeChapter(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (c *stubSynopticAIClient) Chat(_ context.Context, messages []ai.ChatMessage) (string, error) {
	c.chatCalls++
	c.lastMessages = messages
	if c.chatErr != nil {
		return "", c.chatErr
	}
	return c.chatReply, nil
}

func (c *stubSynopticAIClient) ChatWithTools(_ context.Context, _ []ai.ChatMessage, _ []ai.ToolDefinition) (*ai.ChatResponse, error) {
	return &ai.ChatResponse{}, nil
}

func strPtr(s string) *string { return &s }

func TestSynopticServiceValidation(t *testing.T) {
	svc := ai.NewSynopticService(&stubSynopticRetriever{}, &stubSynopticAIClient{})

	cases := []struct {
		name string
		req  ai.SynopticRequest
	}{
		{
			name: "empty query",
			req:  ai.SynopticRequest{Query: "   ", BookIDs: []string{"b1", "b2"}},
		},
		{
			name: "no scope",
			req:  ai.SynopticRequest{Query: "fear and courage"},
		},
		{
			name: "single book without author or genre",
			req:  ai.SynopticRequest{Query: "fear", BookIDs: []string{"b1"}},
		},
		{
			name: "duplicate single book collapses below two",
			req:  ai.SynopticRequest{Query: "fear", BookIDs: []string{"b1", " b1 "}},
		},
		{
			name: "blank genre id ignored",
			req:  ai.SynopticRequest{Query: "fear", GenreID: strPtr("   ")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Synthesize(context.Background(), tc.req)
			if !errors.Is(err, ai.ErrInvalidSynopticRequest) {
				t.Fatalf("expected ErrInvalidSynopticRequest, got %v", err)
			}
		})
	}
}

func TestSynopticServicePerBookCapAndHybridDedup(t *testing.T) {
	vecHits := []*repository.SearchHit{
		{BookID: "b1", BookTitle: "Dune", AuthorName: strPtr("Frank Herbert"), ChapterID: "c1", ChapterIndex: 1, ChapterTitle: strPtr("Gom Jabbar"), StartParagraph: 0, EndParagraph: 1, Content: "Fear is the mind-killer.", Distance: 0.11},
		{BookID: "b1", BookTitle: "Dune", AuthorName: strPtr("Frank Herbert"), ChapterID: "c1", ChapterIndex: 1, ChapterTitle: strPtr("Gom Jabbar"), StartParagraph: 2, EndParagraph: 3, Content: "I must not fear.", Distance: 0.15},
		{BookID: "b1", BookTitle: "Dune", AuthorName: strPtr("Frank Herbert"), ChapterID: "c2", ChapterIndex: 2, ChapterTitle: strPtr("Arrakis"), StartParagraph: 0, EndParagraph: 1, Content: "Desert survival demands discipline.", Distance: 0.19},
		{BookID: "b1", BookTitle: "Dune", AuthorName: strPtr("Frank Herbert"), ChapterID: "c2", ChapterIndex: 2, ChapterTitle: strPtr("Arrakis"), StartParagraph: 4, EndParagraph: 5, Content: "Water belongs to the tribe.", Distance: 0.22},
		{BookID: "b1", BookTitle: "Dune", AuthorName: strPtr("Frank Herbert"), ChapterID: "c3", ChapterIndex: 3, ChapterTitle: strPtr("Sietch"), StartParagraph: 0, EndParagraph: 1, Content: "Fifth Dune hit should be capped.", Distance: 0.25},
		{BookID: "b2", BookTitle: "Peter Rabbit", AuthorName: strPtr("Beatrix Potter"), ChapterID: "p1", ChapterIndex: 1, ChapterTitle: strPtr("Garden"), StartParagraph: 0, EndParagraph: 1, Content: "Do not go into Mr. McGregor's garden.", Distance: 0.30},
	}
	ftsHits := []*repository.SearchHit{
		// Duplicate of vector hit (b1, ch 1, start 0) must not create a second citation
		{BookID: "b1", BookTitle: "Dune", AuthorName: strPtr("Frank Herbert"), ChapterID: "c1", ChapterIndex: 1, ChapterTitle: strPtr("Gom Jabbar"), StartParagraph: 0, EndParagraph: 1, Content: "Fear is the mind-killer.", Distance: -1.5},
		// Unique FTS hit for b2 must be merged
		{BookID: "b2", BookTitle: "Peter Rabbit", AuthorName: strPtr("Beatrix Potter"), ChapterID: "p1", ChapterIndex: 1, ChapterTitle: strPtr("Garden"), StartParagraph: 3, EndParagraph: 4, Content: "Peter was most dreadfully frightened.", Distance: -2.0},
	}

	retriever := &stubSynopticRetriever{vecHits: vecHits, ftsHits: ftsHits}
	client := &stubSynopticAIClient{
		chatReply: `{
			"summary": "Both books depict fear in hostile environments.",
			"consensus_points": [
				{"topic": "Fear under threat", "description": "Both protagonists face danger.", "citation_indices": [0, 4, 99, 4]}
			],
			"divergence_points": [
				{"topic": "Response to danger", "description": "Paul uses ritual discipline while Peter flees.", "citation_indices": [1, 5]}
			]
		}`,
	}

	svc := ai.NewSynopticService(retriever, client)
	res, err := svc.Synthesize(context.Background(), ai.SynopticRequest{
		Query:   "How do these books treat fear?",
		BookIDs: []string{"b1", "b2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.BooksCovered != 2 {
		t.Fatalf("expected BooksCovered=2, got %d", res.BooksCovered)
	}
	if len(res.Citations) != 6 {
		t.Fatalf("expected 6 citations (4 capped from b1 + 2 from b2), got %d", len(res.Citations))
	}

	counts := map[string]int{}
	for _, c := range res.Citations {
		counts[c.BookID]++
		if c.Excerpt == "Fifth Dune hit should be capped." {
			t.Fatal("expected fifth passage from b1 to be dropped by default cap of 4")
		}
	}
	if counts["b1"] != 4 || counts["b2"] != 2 {
		t.Fatalf("expected per-book counts b1=4 b2=2, got %+v", counts)
	}

	first := res.Citations[0]
	if first.BookID != "b1" || first.BookTitle != "Dune" || first.AuthorName != "Frank Herbert" ||
		first.ChapterID != "c1" || first.ChapterTitle != "Gom Jabbar" || first.ChapterIndex != 1 ||
		first.StartParagraph != 0 || first.EndParagraph != 1 || first.CosineDistance <= 0 {
		t.Fatalf("unexpected first citation mapping: %+v", first)
	}

	if len(res.ConsensusPoints) != 1 {
		t.Fatalf("expected 1 consensus point, got %d", len(res.ConsensusPoints))
	}
	// Out-of-range index 99 and duplicate 4 should be stripped
	gotIdx := res.ConsensusPoints[0].CitationIndices
	if len(gotIdx) != 2 || gotIdx[0] != 0 || gotIdx[1] != 4 {
		t.Fatalf("expected sanitized citation indices [0 4], got %v", gotIdx)
	}
	if len(res.DivergencePoints) != 1 {
		t.Fatalf("expected 1 divergence point, got %d", len(res.DivergencePoints))
	}
	if client.chatCalls != 1 {
		t.Fatalf("expected 1 chat call, got %d", client.chatCalls)
	}
}

func TestSynopticServiceZeroMatchGracefulResponse(t *testing.T) {
	retriever := &stubSynopticRetriever{}
	client := &stubSynopticAIClient{}
	svc := ai.NewSynopticService(retriever, client)

	res, err := svc.Synthesize(context.Background(), ai.SynopticRequest{
		Query:   "quantum mechanics",
		BookIDs: []string{"unknown-1", "unknown-2"},
	})
	if err != nil {
		t.Fatalf("expected nil error on zero matches, got %v", err)
	}
	if client.chatCalls != 0 {
		t.Fatalf("expected zero LLM calls on zero matches, got %d", client.chatCalls)
	}
	if res.BooksCovered != 0 || len(res.Citations) != 0 {
		t.Fatalf("expected empty citations and 0 books covered, got %+v", res)
	}
	if strings.TrimSpace(res.Summary) == "" {
		t.Fatal("expected clear non-empty summary message on zero matches")
	}
	if res.ConsensusPoints == nil || res.DivergencePoints == nil || res.Citations == nil {
		t.Fatal("expected non-nil empty slices for clean JSON serialization")
	}
}

func TestSynopticServiceSingleMatchedBookFallback(t *testing.T) {
	retriever := &stubSynopticRetriever{
		vecHits: []*repository.SearchHit{
			{BookID: "b1", BookTitle: "Dune", ChapterID: "c1", ChapterIndex: 1, StartParagraph: 0, EndParagraph: 1, Content: "Spice extends life.", Distance: 0.1},
		},
	}
	client := &stubSynopticAIClient{
		chatReply: "```json\n{\"summary\":\"Only Dune matched the query.\",\"consensus_points\":[],\"divergence_points\":[]}\n```",
	}
	svc := ai.NewSynopticService(retriever, client)

	res, err := svc.Synthesize(context.Background(), ai.SynopticRequest{
		Query:   "melange spice",
		BookIDs: []string{"b1", "b2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.BooksCovered != 1 || len(res.Citations) != 1 {
		t.Fatalf("expected 1 covered book and 1 citation, got covered=%d citations=%d", res.BooksCovered, len(res.Citations))
	}
	if res.Summary != "Only Dune matched the query." {
		t.Fatalf("expected fence-stripped JSON summary, got %q", res.Summary)
	}
}

func TestSynopticServicePlainProseFallback(t *testing.T) {
	retriever := &stubSynopticRetriever{
		ftsHits: []*repository.SearchHit{
			{BookID: "b1", BookTitle: "Dune", ChapterID: "c1", ChapterIndex: 1, StartParagraph: 0, EndParagraph: 1, Content: "Fear is the mind-killer."},
		},
	}
	client := &stubSynopticAIClient{
		chatReply: "Plain prose synthesis without JSON structure.",
	}
	svc := ai.NewSynopticService(retriever, client)

	genreID := "g-scifi"
	res, err := svc.Synthesize(context.Background(), ai.SynopticRequest{
		Query:   "fear",
		GenreID: &genreID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Summary != "Plain prose synthesis without JSON structure." {
		t.Fatalf("expected plain prose fallback in summary, got %q", res.Summary)
	}
	if len(res.ConsensusPoints) != 0 || len(res.DivergencePoints) != 0 {
		t.Fatalf("expected empty structured points on prose fallback, got %+v", res)
	}
}

func BenchmarkSynopticRetrieval(b *testing.B) {
	ctx := context.Background()
	bunDB, err := database.OpenBunSQLite(":memory:")
	if err != nil {
		b.Fatalf("open sqlite: %v", err)
	}
	defer bunDB.Close()

	if err := database.RunBunMigrations(ctx, bunDB); err != nil {
		b.Fatalf("run migrations: %v", err)
	}
	repo := repository.NewBunStorageEngine(bunDB)

	bookIDs := []string{"bench-book-1", "bench-book-2", "bench-book-3"}
	titles := []string{"Dune", "Peter Rabbit", "A Wizard of Earthsea"}
	authors := []string{"Frank Herbert", "Beatrix Potter", "Ursula K. Le Guin"}
	snippets := []string{
		"Fear is the mind-killer in the desert test of the Gom Jabbar.",
		"Peter was dreadfully frightened while escaping Mr. McGregor's garden.",
		"Ged confronted his shadow with courage across the open sea.",
	}

	vec := make([]float32, 256)
	vec[0] = 1.0

	for i, bookID := range bookIDs {
		author, err := repo.UpsertAuthor(ctx, authors[i])
		if err != nil {
			b.Fatalf("upsert author: %v", err)
		}
		if err := repo.CreateBook(ctx, &repository.Book{
			ID:       bookID,
			Title:    titles[i],
			FilePath: fmt.Sprintf("%s/%s.epub", authors[i], titles[i]),
		}); err != nil {
			b.Fatalf("create book: %v", err)
		}
		if err := repo.LinkBookAuthor(ctx, bookID, author.ID, "author"); err != nil {
			b.Fatalf("link author: %v", err)
		}

		chID := fmt.Sprintf("ch-%s-1", bookID)
		chTitle := "Chapter 1"
		if err := repo.CreateChapter(ctx, &repository.Chapter{
			ID:           chID,
			BookID:       bookID,
			ChapterIndex: 1,
			Title:        &chTitle,
			Summary:      snippets[i],
			ContentPlain: snippets[i],
		}); err != nil {
			b.Fatalf("create chapter: %v", err)
		}

		var paras []*repository.Paragraph
		var vectors []repository.ParagraphVector
		for p := 0; p < 12; p++ {
			pid := fmt.Sprintf("p-%s-%d", bookID, p)
			paras = append(paras, &repository.Paragraph{
				ID:             pid,
				BookID:         bookID,
				ChapterID:      chID,
				ChapterIndex:   1,
				StartParagraph: p,
				EndParagraph:   p,
				Content:        fmt.Sprintf("%s Passage %d on fear, courage, and survival.", snippets[i], p),
				IsEmbedded:     true,
			})
			pVec := make([]float32, 256)
			pVec[0] = 0.9 - float32(p)*0.02
			pVec[1] = float32(i+1) * 0.1
			vectors = append(vectors, repository.ParagraphVector{
				ParagraphID: pid,
				Embedding:   pVec,
			})
		}
		if err := repo.CreateParagraphs(ctx, paras); err != nil {
			b.Fatalf("create paragraphs: %v", err)
		}
		if err := repo.InsertParagraphVectors(ctx, vectors); err != nil {
			b.Fatalf("insert paragraph vectors: %v", err)
		}
	}

	svc := ai.NewSynopticService(repo, &stubSynopticAIClient{embedVec: vec})
	req := ai.SynopticRequest{
		Query:              "How do these books portray fear and survival?",
		BookIDs:            bookIDs,
		MaxPassagesPerBook: 4,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hits, err := svc.RetrievePassages(ctx, req)
		if err != nil {
			b.Fatalf("retrieve passages: %v", err)
		}
		if len(hits) == 0 {
			b.Fatal("expected non-empty retrieval hits")
		}
	}
}
