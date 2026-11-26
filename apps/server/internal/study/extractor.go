package study

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

const (
	MaxParagraphWindow = 50
	DefaultMaxItems    = 8
)

var (
	ErrBookNotFound    = errors.New("book not found")
	ErrChapterNotFound = errors.New("chapter not found")
	ErrInvalidRange    = errors.New("invalid chapter or paragraph range")
	ErrInvalidFormat   = errors.New("unsupported study deck format")
)

type StudyDeckFormat string

const (
	FormatAPKG     StudyDeckFormat = "apkg"
	FormatCSV      StudyDeckFormat = "csv"
	FormatMarkdown StudyDeckFormat = "markdown"
)

func ParseStudyDeckFormat(raw string) (StudyDeckFormat, error) {
	switch StudyDeckFormat(strings.ToLower(strings.TrimSpace(raw))) {
	case FormatAPKG:
		return FormatAPKG, nil
	case FormatCSV:
		return FormatCSV, nil
	case FormatMarkdown:
		return FormatMarkdown, nil
	default:
		return "", fmt.Errorf("%w: %q (expected apkg, csv, or markdown)", ErrInvalidFormat, raw)
	}
}

type StudyConcept struct {
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	RelatedConcepts []string `json:"related_concepts,omitempty"`
	BookID          string   `json:"book_id"`
	ChapterIndex    int      `json:"chapter_index"`
	StartParagraph  int      `json:"start_paragraph"`
	EndParagraph    int      `json:"end_paragraph"`
}

type StudyCard struct {
	Front          string   `json:"front"`
	Back           string   `json:"back"`
	Tags           []string `json:"tags,omitempty"`
	BookID         string   `json:"book_id"`
	ChapterIndex   int      `json:"chapter_index"`
	StartParagraph int      `json:"start_paragraph"`
	EndParagraph   int      `json:"end_paragraph"`
}

type ExtractRequest struct {
	BookID         string
	ChapterIndex   int
	StartParagraph int
	EndParagraph   int
	MaxItems       int
}

type Extractor struct {
	repo   repository.StorageEngine
	chat   ai.ChatClient
	logger *slog.Logger
}

func NewExtractor(repo repository.StorageEngine, chat ai.ChatClient, logger *slog.Logger) *Extractor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Extractor{
		repo:   repo,
		chat:   chat,
		logger: logger,
	}
}

func SplitParagraphs(contentPlain string) []string {
	normalized := strings.ReplaceAll(contentPlain, "\r\n", "\n")
	raw := strings.Split(normalized, "\n\n")
	paras := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			paras = append(paras, p)
		}
	}
	return paras
}

func StripMarkdownCodeFence(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if start := strings.Index(s, "```"); start != -1 {
		rest := s[start+3:]
		if nl := strings.IndexByte(rest, '\n'); nl != -1 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end != -1 {
			return strings.TrimSpace(rest[:end])
		}
		return strings.TrimSpace(rest)
	}
	return s
}

func (e *Extractor) ExtractConcepts(ctx context.Context, req ExtractRequest) ([]StudyConcept, error) {
	book, chapter, paras, startPara, endPara, err := e.loadWindow(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(paras) == 0 {
		return []StudyConcept{}, nil
	}

	maxItems := req.MaxItems
	if maxItems <= 0 {
		maxItems = DefaultMaxItems
	}

	var concepts []StudyConcept
	if e.chat != nil {
		prompt := buildConceptPrompt(book.Title, chapter, paras, startPara, endPara, maxItems)
		reply, chatErr := e.chat.Chat(ctx, []ai.ChatMessage{
			{Role: "system", Content: "Extract structured study concepts as strict JSON only. Do not include markdown prose."},
			{Role: "user", Content: prompt},
		})
		if chatErr != nil {
			e.logger.Warn("llm concept extraction failed, falling back to deterministic extraction", "book_id", book.ID, "chapter_index", chapter.ChapterIndex, "error", chatErr)
		} else {
			concepts = parseConceptsJSON(reply)
		}
	}

	if len(concepts) == 0 {
		concepts = fallbackConcepts(book.ID, chapter, paras, startPara, endPara, maxItems)
	}

	return normalizeConcepts(concepts, book.ID, chapter.ChapterIndex, startPara, endPara, maxItems), nil
}

func (e *Extractor) GenerateCards(ctx context.Context, req ExtractRequest) ([]StudyCard, error) {
	book, chapter, paras, startPara, endPara, err := e.loadWindow(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(paras) == 0 {
		return []StudyCard{}, nil
	}

	maxItems := req.MaxItems
	if maxItems <= 0 {
		maxItems = DefaultMaxItems
	}

	var cards []StudyCard
	if e.chat != nil {
		prompt := buildCardPrompt(book.Title, chapter, paras, startPara, endPara, maxItems)
		reply, chatErr := e.chat.Chat(ctx, []ai.ChatMessage{
			{Role: "system", Content: "Generate concise question-and-answer study flashcards as strict JSON only. Do not include markdown prose."},
			{Role: "user", Content: prompt},
		})
		if chatErr != nil {
			e.logger.Warn("llm study card generation failed, falling back to deterministic generation", "book_id", book.ID, "chapter_index", chapter.ChapterIndex, "error", chatErr)
		} else {
			cards = parseCardsJSON(reply)
		}
	}

	if len(cards) == 0 {
		cards = fallbackCards(book.Title, book.ID, chapter, paras, startPara, endPara, maxItems)
	}

	return normalizeCards(cards, book.ID, chapter.ChapterIndex, startPara, endPara, maxItems), nil
}

func (e *Extractor) loadWindow(ctx context.Context, req ExtractRequest) (*repository.Book, *repository.Chapter, []string, int, int, error) {
	bookID := strings.TrimSpace(req.BookID)
	if bookID == "" {
		return nil, nil, nil, 0, 0, fmt.Errorf("%w: book_id is required", ErrInvalidRange)
	}
	if req.ChapterIndex < 0 {
		return nil, nil, nil, 0, 0, fmt.Errorf("%w: chapter_index must be >= 0", ErrInvalidRange)
	}
	if req.StartParagraph < 0 || req.EndParagraph < 0 {
		return nil, nil, nil, 0, 0, fmt.Errorf("%w: paragraph bounds must be >= 0", ErrInvalidRange)
	}
	if req.EndParagraph > 0 && req.StartParagraph > 0 && req.EndParagraph < req.StartParagraph {
		return nil, nil, nil, 0, 0, fmt.Errorf("%w: end_paragraph (%d) must be >= start_paragraph (%d)", ErrInvalidRange, req.EndParagraph, req.StartParagraph)
	}

	book, err := e.repo.GetBookByID(ctx, bookID)
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		return nil, nil, nil, 0, 0, fmt.Errorf("%w: %s", ErrBookNotFound, bookID)
	}
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}

	chapter, err := e.repo.GetChapterByBookAndIndex(ctx, bookID, req.ChapterIndex)
	if errors.Is(err, repository.ErrNotFound) || chapter == nil {
		return nil, nil, nil, 0, 0, fmt.Errorf("%w: chapter %d", ErrChapterNotFound, req.ChapterIndex)
	}
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}

	paras := SplitParagraphs(chapter.ContentPlain)
	total := len(paras)
	if total == 0 {
		return book, chapter, nil, 0, 0, nil
	}

	startPara := req.StartParagraph
	if startPara == 0 {
		startPara = 1
	}
	if startPara > total {
		return nil, nil, nil, 0, 0, fmt.Errorf("%w: start_paragraph (%d) exceeds total paragraphs (%d)", ErrInvalidRange, startPara, total)
	}

	endPara := startPara + MaxParagraphWindow - 1
	if req.EndParagraph > 0 {
		if req.EndParagraph < startPara {
			return nil, nil, nil, 0, 0, fmt.Errorf("%w: end_paragraph (%d) must be >= start_paragraph (%d)", ErrInvalidRange, req.EndParagraph, startPara)
		}
		endPara = req.EndParagraph
	}
	if endPara > startPara+MaxParagraphWindow-1 {
		endPara = startPara + MaxParagraphWindow - 1
	}
	if endPara > total {
		endPara = total
	}

	return book, chapter, paras[startPara-1 : endPara], startPara, endPara, nil
}

func formatWindowedPassage(bookTitle string, chapter *repository.Chapter, paras []string, startPara, endPara int) string {
	var sb strings.Builder
	chTitle := chapterDisplayTitle(chapter)
	fmt.Fprintf(&sb, "Book: %s\nSection: %s (Paragraphs %d-%d)\n\n", bookTitle, chTitle, startPara, endPara)
	for i, p := range paras {
		fmt.Fprintf(&sb, "[¶%d] %s\n\n", startPara+i, p)
	}
	return sb.String()
}

func chapterDisplayTitle(chapter *repository.Chapter) string {
	if chapter.Title != nil && strings.TrimSpace(*chapter.Title) != "" {
		return fmt.Sprintf("Chapter %d: %s", chapter.ChapterIndex, strings.TrimSpace(*chapter.Title))
	}
	return fmt.Sprintf("Chapter %d", chapter.ChapterIndex)
}

func buildConceptPrompt(bookTitle string, chapter *repository.Chapter, paras []string, startPara, endPara, maxItems int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Extract up to %d key concepts from the passage below.\n", maxItems)
	sb.WriteString("Return a JSON object with a \"concepts\" array. Each item must have: \"title\" (string), \"summary\" (string), \"related_concepts\" (string array), \"start_paragraph\" (int), and \"end_paragraph\" (int).\n\n")
	sb.WriteString(formatWindowedPassage(bookTitle, chapter, paras, startPara, endPara))
	return sb.String()
}

func buildCardPrompt(bookTitle string, chapter *repository.Chapter, paras []string, startPara, endPara, maxItems int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Generate up to %d high-value question-and-answer study flashcards grounded in the passage below.\n", maxItems)
	sb.WriteString("Return a JSON object with a \"cards\" array. Each item must have: \"front\" (question string), \"back\" (answer string), \"tags\" (string array), \"start_paragraph\" (int), and \"end_paragraph\" (int).\n\n")
	sb.WriteString(formatWindowedPassage(bookTitle, chapter, paras, startPara, endPara))
	return sb.String()
}

func parseConceptsJSON(raw string) []StudyConcept {
	cleaned := StripMarkdownCodeFence(raw)
	if cleaned == "" {
		return nil
	}

	var wrapped struct {
		Concepts []StudyConcept `json:"concepts"`
	}
	if err := json.Unmarshal([]byte(cleaned), &wrapped); err == nil && len(wrapped.Concepts) > 0 {
		return wrapped.Concepts
	}

	var direct []StudyConcept
	if err := json.Unmarshal([]byte(cleaned), &direct); err == nil && len(direct) > 0 {
		return direct
	}

	return nil
}

func parseCardsJSON(raw string) []StudyCard {
	cleaned := StripMarkdownCodeFence(raw)
	if cleaned == "" {
		return nil
	}

	var wrapped struct {
		Cards []StudyCard `json:"cards"`
	}
	if err := json.Unmarshal([]byte(cleaned), &wrapped); err == nil && len(wrapped.Cards) > 0 {
		return wrapped.Cards
	}

	var direct []StudyCard
	if err := json.Unmarshal([]byte(cleaned), &direct); err == nil && len(direct) > 0 {
		return direct
	}

	return nil
}

func normalizeConcepts(items []StudyConcept, bookID string, chapterIndex, startPara, endPara, maxItems int) []StudyConcept {
	out := make([]StudyConcept, 0, min(len(items), maxItems))
	for _, c := range items {
		c.Title = strings.TrimSpace(c.Title)
		c.Summary = strings.TrimSpace(c.Summary)
		if c.Title == "" || c.Summary == "" {
			continue
		}
		c.BookID = bookID
		c.ChapterIndex = chapterIndex
		c.StartParagraph, c.EndParagraph = clampRange(c.StartParagraph, c.EndParagraph, startPara, endPara)
		out = append(out, c)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func normalizeCards(items []StudyCard, bookID string, chapterIndex, startPara, endPara, maxItems int) []StudyCard {
	out := make([]StudyCard, 0, min(len(items), maxItems))
	for _, c := range items {
		c.Front = strings.TrimSpace(c.Front)
		c.Back = strings.TrimSpace(c.Back)
		if c.Front == "" || c.Back == "" {
			continue
		}
		c.BookID = bookID
		c.ChapterIndex = chapterIndex
		c.StartParagraph, c.EndParagraph = clampRange(c.StartParagraph, c.EndParagraph, startPara, endPara)
		out = append(out, c)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func clampRange(start, end, minPara, maxPara int) (int, int) {
	if start < minPara || start > maxPara {
		start = minPara
	}
	if end < start || end > maxPara {
		end = start
	}
	return start, end
}

func fallbackConcepts(bookID string, chapter *repository.Chapter, paras []string, startPara, endPara, maxItems int) []StudyConcept {
	limit := min(len(paras), maxItems)
	concepts := make([]StudyConcept, 0, limit)
	chTitle := chapterDisplayTitle(chapter)

	for i := 0; i < limit; i++ {
		paraNum := startPara + i
		text := paras[i]
		title := deriveConceptTitle(chTitle, text, paraNum)
		summary := truncateText(text, 280)
		concepts = append(concepts, StudyConcept{
			Title:          title,
			Summary:        summary,
			BookID:         bookID,
			ChapterIndex:   chapter.ChapterIndex,
			StartParagraph: paraNum,
			EndParagraph:   paraNum,
		})
	}
	return concepts
}

func fallbackCards(bookTitle, bookID string, chapter *repository.Chapter, paras []string, startPara, endPara, maxItems int) []StudyCard {
	limit := min(len(paras), maxItems)
	cards := make([]StudyCard, 0, limit)
	chTitle := chapterDisplayTitle(chapter)
	tag := fmt.Sprintf("chapter-%d", chapter.ChapterIndex)

	for i := 0; i < limit; i++ {
		paraNum := startPara + i
		text := paras[i]
		topic := deriveConceptTitle(chTitle, text, paraNum)
		cards = append(cards, StudyCard{
			Front:          fmt.Sprintf("In %s (%s, ¶%d), what key point is made about %s?", bookTitle, chTitle, paraNum, topic),
			Back:           truncateText(text, 320),
			Tags:           []string{"shelfd", tag},
			BookID:         bookID,
			ChapterIndex:   chapter.ChapterIndex,
			StartParagraph: paraNum,
			EndParagraph:   paraNum,
		})
	}
	return cards
}

func deriveConceptTitle(chTitle, paragraph string, paraNum int) string {
	words := strings.Fields(paragraph)
	if len(words) == 0 {
		return fmt.Sprintf("%s (¶%d)", chTitle, paraNum)
	}
	n := min(len(words), 6)
	phrase := strings.TrimRight(strings.Join(words[:n], " "), ".,;:!?\"'")
	if phrase == "" {
		return fmt.Sprintf("%s (¶%d)", chTitle, paraNum)
	}
	return phrase
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := strings.LastIndex(s[:maxLen], " ")
	if cut > maxLen/2 {
		return s[:cut] + "..."
	}
	return s[:maxLen] + "..."
}
