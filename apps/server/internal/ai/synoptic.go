package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/shelfd/shelfd/internal/repository"
)

const (
	defaultMaxPassagesPerBook = 4
	maxAllowedPassagesPerBook = 20
	maxSynopticFetchLimit     = 100
)

var (
	ErrInvalidSynopticRequest = errors.New("invalid synoptic request")
	ErrSynopticUnavailable    = errors.New("synoptic service dependencies not configured")
)

type SynopticRequest struct {
	Query              string   `json:"query"`
	BookIDs            []string `json:"book_ids,omitempty"`
	AuthorIDs          []string `json:"author_ids,omitempty"`
	GenreID            *string  `json:"genre_id,omitempty"`
	MaxPassagesPerBook int      `json:"max_passages_per_book,omitempty"`
	Limit              int      `json:"limit,omitempty"`
}

type SynopticPoint struct {
	Topic           string `json:"topic"`
	Description     string `json:"description"`
	CitationIndices []int  `json:"citation_indices"`
}

type SynopticResult struct {
	Summary          string          `json:"summary"`
	ConsensusPoints  []SynopticPoint `json:"consensus_points"`
	DivergencePoints []SynopticPoint `json:"divergence_points"`
	Citations        []BookCitation  `json:"citations"`
	BooksCovered     int             `json:"books_covered"`
}

type SynopticRetriever interface {
	SearchVectorParagraphs(ctx context.Context, queryEmbedding []float32, filter repository.SearchFilter) ([]*repository.SearchHit, error)
	SearchFTSParagraphs(ctx context.Context, query string, filter repository.SearchFilter) ([]*repository.SearchHit, error)
}

type SynopticService struct {
	repo     SynopticRetriever
	aiClient Client
}

func NewSynopticService(repo SynopticRetriever, aiClient Client) *SynopticService {
	return &SynopticService{
		repo:     repo,
		aiClient: aiClient,
	}
}

type normalizedSynopticRequest struct {
	query              string
	bookIDs            []string
	authorIDs          []string
	genreID            *string
	maxPassagesPerBook int
	limit              int
}

func ValidateSynopticRequest(req SynopticRequest) error {
	_, err := normalizeSynopticRequest(req)
	return err
}

func normalizeSynopticRequest(req SynopticRequest) (normalizedSynopticRequest, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return normalizedSynopticRequest{}, fmt.Errorf("%w: query must not be empty", ErrInvalidSynopticRequest)
	}

	bookIDs := dedupeTrimmed(req.BookIDs)
	authorIDs := dedupeTrimmed(req.AuthorIDs)

	var genreID *string
	if req.GenreID != nil {
		if trimmed := strings.TrimSpace(*req.GenreID); trimmed != "" {
			genreID = &trimmed
		}
	}

	hasValidScope := len(bookIDs) >= 2 || len(authorIDs) >= 1 || genreID != nil
	if !hasValidScope {
		return normalizedSynopticRequest{}, fmt.Errorf("%w: provide at least two book_ids, one author_id, or a genre_id", ErrInvalidSynopticRequest)
	}

	maxPerBook := req.MaxPassagesPerBook
	if maxPerBook <= 0 {
		maxPerBook = defaultMaxPassagesPerBook
	}
	if maxPerBook > maxAllowedPassagesPerBook {
		maxPerBook = maxAllowedPassagesPerBook
	}

	limit := req.Limit
	if limit > maxSynopticFetchLimit {
		limit = maxSynopticFetchLimit
	}

	return normalizedSynopticRequest{
		query:              query,
		bookIDs:            bookIDs,
		authorIDs:          authorIDs,
		genreID:            genreID,
		maxPassagesPerBook: maxPerBook,
		limit:              limit,
	}, nil
}

func (s *SynopticService) Synthesize(ctx context.Context, req SynopticRequest) (*SynopticResult, error) {
	norm, err := normalizeSynopticRequest(req)
	if err != nil {
		return nil, err
	}
	if s.repo == nil || s.aiClient == nil {
		return nil, ErrSynopticUnavailable
	}

	hits, err := s.retrievePassages(ctx, norm)
	if err != nil {
		return nil, err
	}

	if len(hits) == 0 {
		return &SynopticResult{
			Summary:          "No matching passages were found across the selected scope for your query.",
			ConsensusPoints:  []SynopticPoint{},
			DivergencePoints: []SynopticPoint{},
			Citations:        []BookCitation{},
			BooksCovered:     0,
		}, nil
	}

	citations, booksCovered := mapHitsToCitations(hits)

	messages := buildSynopticMessages(norm.query, citations)
	rawReply, err := s.aiClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("synoptic synthesis chat failed: %w", err)
	}

	parsed := parseSynopticLLMResponse(rawReply, len(citations))
	return &SynopticResult{
		Summary:          parsed.Summary,
		ConsensusPoints:  parsed.ConsensusPoints,
		DivergencePoints: parsed.DivergencePoints,
		Citations:        citations,
		BooksCovered:     booksCovered,
	}, nil
}

func (s *SynopticService) RetrievePassages(ctx context.Context, req SynopticRequest) ([]*repository.SearchHit, error) {
	norm, err := normalizeSynopticRequest(req)
	if err != nil {
		return nil, err
	}
	if s.repo == nil || s.aiClient == nil {
		return nil, ErrSynopticUnavailable
	}
	return s.retrievePassages(ctx, norm)
}

func (s *SynopticService) retrievePassages(ctx context.Context, norm normalizedSynopticRequest) ([]*repository.SearchHit, error) {
	bookEstimate := len(norm.bookIDs)
	if bookEstimate < 4 {
		bookEstimate = 4
	}
	fetchLimit := norm.maxPassagesPerBook * bookEstimate * 2
	if norm.limit > 0 && norm.limit*2 > fetchLimit {
		fetchLimit = norm.limit * 2
	}
	if fetchLimit < 24 {
		fetchLimit = 24
	}
	if fetchLimit > maxSynopticFetchLimit {
		fetchLimit = maxSynopticFetchLimit
	}

	filter := repository.SearchFilter{
		BookIDs:   norm.bookIDs,
		AuthorIDs: norm.authorIDs,
		GenreID:   norm.genreID,
		Limit:     fetchLimit,
	}

	var vecHits []*repository.SearchHit
	var vecErr error
	if queryEmbedding, err := s.aiClient.GenerateEmbedding(ctx, norm.query); err != nil {
		vecErr = err
	} else if len(queryEmbedding) > 0 {
		vecHits, vecErr = s.repo.SearchVectorParagraphs(ctx, queryEmbedding, filter)
	}

	var ftsHits []*repository.SearchHit
	var ftsErr error
	if ftsQuery := sanitizeFTSQuery(norm.query); ftsQuery != "" {
		ftsHits, ftsErr = s.repo.SearchFTSParagraphs(ctx, ftsQuery, filter)
	}

	if vecErr != nil && ftsErr != nil {
		return nil, fmt.Errorf("synoptic retrieval failed (vector: %v; fts: %v)", vecErr, ftsErr)
	}

	return mergeAndCapHits(vecHits, ftsHits, norm.maxPassagesPerBook, norm.limit), nil
}

type passageKey struct {
	bookID         string
	chapterIndex   int
	startParagraph int
}

// mergeAndCapHits deduplicates vector and FTS hits, caps passages per book, and
// applies an optional total limit round-robin so every matched book stays represented.
func mergeAndCapHits(vecHits, ftsHits []*repository.SearchHit, maxPerBook, totalLimit int) []*repository.SearchHit {
	seen := make(map[passageKey]struct{}, len(vecHits)+len(ftsHits))
	byBook := make(map[string][]*repository.SearchHit)
	var bookOrder []string

	appendHit := func(hit *repository.SearchHit) {
		if hit == nil {
			return
		}
		key := passageKey{
			bookID:         hit.BookID,
			chapterIndex:   hit.ChapterIndex,
			startParagraph: hit.StartParagraph,
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}

		group := byBook[hit.BookID]
		if len(group) == 0 {
			bookOrder = append(bookOrder, hit.BookID)
		}
		if len(group) >= maxPerBook {
			return
		}
		byBook[hit.BookID] = append(group, hit)
	}

	for _, hit := range vecHits {
		appendHit(hit)
	}
	for _, hit := range ftsHits {
		appendHit(hit)
	}

	if totalLimit > 0 {
		selectedByBook := make(map[string][]*repository.SearchHit, len(bookOrder))
		selectedCount := 0
		for round := 0; round < maxPerBook && selectedCount < totalLimit; round++ {
			for _, bookID := range bookOrder {
				if selectedCount >= totalLimit {
					break
				}
				group := byBook[bookID]
				if round < len(group) {
					selectedByBook[bookID] = append(selectedByBook[bookID], group[round])
					selectedCount++
				}
			}
		}
		byBook = selectedByBook
	}

	result := make([]*repository.SearchHit, 0, len(seen))
	for _, bookID := range bookOrder {
		result = append(result, byBook[bookID]...)
	}
	return result
}

func mapHitsToCitations(hits []*repository.SearchHit) ([]BookCitation, int) {
	citations := make([]BookCitation, 0, len(hits))
	books := make(map[string]struct{}, len(hits))

	for _, hit := range hits {
		authorName := ""
		if hit.AuthorName != nil {
			authorName = strings.TrimSpace(*hit.AuthorName)
		}
		chapterTitle := ""
		if hit.ChapterTitle != nil {
			chapterTitle = strings.TrimSpace(*hit.ChapterTitle)
		}
		excerpt := strings.TrimSpace(hit.Content)
		if excerpt == "" {
			excerpt = strings.TrimSpace(hit.Summary)
		}

		citations = append(citations, BookCitation{
			BookID:         hit.BookID,
			BookTitle:      hit.BookTitle,
			AuthorName:     authorName,
			ChapterID:      hit.ChapterID,
			ChapterTitle:   chapterTitle,
			ChapterIndex:   hit.ChapterIndex,
			StartParagraph: hit.StartParagraph,
			EndParagraph:   hit.EndParagraph,
			Excerpt:        excerpt,
			CosineDistance: float32(hit.Distance),
		})
		books[hit.BookID] = struct{}{}
	}

	return citations, len(books)
}

const synopticSystemPrompt = `You are a comparative literary analyst performing synoptic synthesis across multiple books.
Base every claim strictly on the excerpts inside <retrieved_passages>. Do not invent facts or cite indices that do not exist.
Respond with a single valid JSON object matching this exact schema (no extra commentary):
{
  "summary": "Concise cross-book synthesis answering the query.",
  "consensus_points": [
    {"topic": "Shared theme or agreement", "description": "How the books align.", "citation_indices": [0, 2]}
  ],
  "divergence_points": [
    {"topic": "Point of contrast", "description": "How the books differ.", "citation_indices": [1, 3]}
  ]
}`

func buildSynopticMessages(query string, citations []BookCitation) []ChatMessage {
	var sb strings.Builder
	sb.WriteString("Query: ")
	sb.WriteString(query)
	sb.WriteString("\n\n<retrieved_passages>\n")
	for i, c := range citations {
		fmt.Fprintf(&sb, "[%d] Book: %s", i, c.BookTitle)
		if c.AuthorName != "" {
			fmt.Fprintf(&sb, " by %s", c.AuthorName)
		}
		fmt.Fprintf(&sb, " | Chapter %d", c.ChapterIndex)
		if c.ChapterTitle != "" {
			fmt.Fprintf(&sb, " (%s)", c.ChapterTitle)
		}
		fmt.Fprintf(&sb, " | Paragraphs %d-%d\n%s\n\n", c.StartParagraph, c.EndParagraph, c.Excerpt)
	}
	sb.WriteString("</retrieved_passages>")

	return []ChatMessage{
		{Role: "system", Content: synopticSystemPrompt},
		{Role: "user", Content: sb.String()},
	}
}

type synopticLLMPayload struct {
	Summary          string          `json:"summary"`
	ConsensusPoints  []SynopticPoint `json:"consensus_points"`
	DivergencePoints []SynopticPoint `json:"divergence_points"`
}

func parseSynopticLLMResponse(raw string, citationCount int) synopticLLMPayload {
	cleaned := stripMarkdownCodeFence(raw)

	var payload synopticLLMPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		if extracted, ok := extractJSONObject(cleaned); ok {
			err = json.Unmarshal([]byte(extracted), &payload)
		}
		if err != nil {
			return synopticLLMPayload{
				Summary:          strings.TrimSpace(raw),
				ConsensusPoints:  []SynopticPoint{},
				DivergencePoints: []SynopticPoint{},
			}
		}
	}

	payload.Summary = strings.TrimSpace(payload.Summary)
	if payload.Summary == "" {
		payload.Summary = strings.TrimSpace(raw)
	}
	payload.ConsensusPoints = sanitizeSynopticPoints(payload.ConsensusPoints, citationCount)
	payload.DivergencePoints = sanitizeSynopticPoints(payload.DivergencePoints, citationCount)
	return payload
}

func stripMarkdownCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	firstNewline := strings.IndexByte(trimmed, '\n')
	if firstNewline == -1 {
		return strings.TrimSpace(strings.Trim(trimmed, "`"))
	}

	body := trimmed[firstNewline+1:]
	if lastFence := strings.LastIndex(body, "```"); lastFence >= 0 {
		body = body[:lastFence]
	}
	return strings.TrimSpace(body)
}

func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return "", false
	}
	return strings.TrimSpace(s[start : end+1]), true
}

func sanitizeSynopticPoints(points []SynopticPoint, citationCount int) []SynopticPoint {
	if len(points) == 0 {
		return []SynopticPoint{}
	}
	out := make([]SynopticPoint, 0, len(points))
	for _, pt := range points {
		topic := strings.TrimSpace(pt.Topic)
		desc := strings.TrimSpace(pt.Description)
		if topic == "" && desc == "" {
			continue
		}
		seenIdx := make(map[int]struct{}, len(pt.CitationIndices))
		validIdx := make([]int, 0, len(pt.CitationIndices))
		for _, idx := range pt.CitationIndices {
			if idx < 0 || idx >= citationCount {
				continue
			}
			if _, ok := seenIdx[idx]; ok {
				continue
			}
			seenIdx[idx] = struct{}{}
			validIdx = append(validIdx, idx)
		}
		out = append(out, SynopticPoint{
			Topic:           topic,
			Description:     desc,
			CitationIndices: validIdx,
		})
	}
	return out
}

var ftsStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"do": {}, "does": {}, "for": {}, "from": {}, "how": {}, "in": {}, "is": {}, "it": {},
	"of": {}, "on": {}, "or": {}, "that": {}, "the": {}, "these": {}, "this": {}, "to": {},
	"what": {}, "when": {}, "where": {}, "which": {}, "who": {}, "why": {}, "with": {},
}

// sanitizeFTSQuery quotes individual terms and joins them with OR so natural-language
// questions never trigger SQLite FTS5 syntax errors and still recall relevant passages.
func sanitizeFTSQuery(raw string) string {
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(tokens) == 0 {
		return ""
	}

	filtered := make([]string, 0, len(tokens))
	fallback := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))

	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		quoted := `"` + lower + `"`
		fallback = append(fallback, quoted)
		if len(lower) >= 2 {
			if _, stop := ftsStopwords[lower]; !stop {
				filtered = append(filtered, quoted)
			}
		}
	}

	if len(filtered) > 0 {
		return strings.Join(filtered, " OR ")
	}
	return strings.Join(fallback, " OR ")
}

func dedupeTrimmed(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
