package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/shelfd/shelfd/internal/repository"
)

var (
	ErrInvalidRecapPosition     = errors.New("chapter_index and paragraph_offset must be non-negative")
	ErrRecapAIClientUnavailable = errors.New("ai chat client is not configured")
)

const (
	maxRecapSummaryBackfills = 5
	maxRecapExcerptChars     = 1200
)

type RecapResult struct {
	BookID           string    `json:"book_id"`
	BookTitle        string    `json:"book_title"`
	ChapterIndex     int       `json:"chapter_index"`
	ParagraphOffset  int       `json:"paragraph_offset"`
	Recap            string    `json:"recap"`
	Cached           bool      `json:"cached"`
	ChaptersIncluded int       `json:"chapters_included"`
	CreatedAt        time.Time `json:"created_at"`
}

type RecapService struct {
	repo     repository.StorageEngine
	aiClient ChatClient
	logger   *slog.Logger
}

func NewRecapService(repo repository.StorageEngine, aiClient ChatClient, logger *slog.Logger) *RecapService {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecapService{
		repo:     repo,
		aiClient: aiClient,
		logger:   logger,
	}
}

func (s *RecapService) Generate(
	ctx context.Context,
	bookID string,
	chapterIndex int,
	paragraphOffset int,
	forceRefresh bool,
) (*RecapResult, error) {
	if chapterIndex < 0 || paragraphOffset < 0 {
		return nil, ErrInvalidRecapPosition
	}

	book, err := s.repo.GetBookByID(ctx, bookID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("loading book for recap: %w", err)
	}
	if book == nil {
		return nil, repository.ErrNotFound
	}

	allChapters, err := s.repo.GetChaptersByBookID(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("loading chapters for recap: %w", err)
	}

	// Strictly enforce spoiler boundary: never include chapters beyond chapterIndex.
	eligible := make([]*repository.Chapter, 0, len(allChapters))
	for _, ch := range allChapters {
		if ch != nil && ch.ChapterIndex <= chapterIndex {
			eligible = append(eligible, ch)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].ChapterIndex < eligible[j].ChapterIndex
	})

	if !forceRefresh {
		cached, cacheErr := s.repo.GetRecapCache(ctx, bookID, chapterIndex, paragraphOffset)
		if cacheErr == nil && cached != nil && strings.TrimSpace(cached.Content) != "" {
			return &RecapResult{
				BookID:           book.ID,
				BookTitle:        book.Title,
				ChapterIndex:     chapterIndex,
				ParagraphOffset:  paragraphOffset,
				Recap:            cached.Content,
				Cached:           true,
				ChaptersIncluded: len(eligible),
				CreatedAt:        cached.CreatedAt,
			}, nil
		}
		if cacheErr != nil && !errors.Is(cacheErr, repository.ErrNotFound) {
			s.logger.Warn("recap cache lookup failed, continuing with generation",
				"book_id", bookID,
				"chapter_index", chapterIndex,
				"error", cacheErr,
			)
		}
	}

	if chapterIndex == 0 && !hasRecapContent(eligible) {
		opening := fmt.Sprintf(
			"You are at the beginning of \"%s\". Start reading Chapter 1 to build your spoiler-free \"Previously On\" recap.",
			book.Title,
		)
		now := time.Now().UTC()
		_ = s.repo.UpsertRecapCache(ctx, &repository.RecapCacheEntry{
			BookID:          book.ID,
			ChapterIndex:    chapterIndex,
			ParagraphOffset: paragraphOffset,
			Content:         opening,
			CreatedAt:       now,
		})
		return &RecapResult{
			BookID:           book.ID,
			BookTitle:        book.Title,
			ChapterIndex:     chapterIndex,
			ParagraphOffset:  paragraphOffset,
			Recap:            opening,
			Cached:           false,
			ChaptersIncluded: len(eligible),
			CreatedAt:        now,
		}, nil
	}

	if s.aiClient == nil {
		return nil, ErrRecapAIClientUnavailable
	}

	if !hasRecapContent(eligible) {
		emptyMsg := fmt.Sprintf(
			"No chapter text is available yet for \"%s\" up to Chapter %d.",
			book.Title,
			chapterIndex,
		)
		now := time.Now().UTC()
		return &RecapResult{
			BookID:           book.ID,
			BookTitle:        book.Title,
			ChapterIndex:     chapterIndex,
			ParagraphOffset:  paragraphOffset,
			Recap:            emptyMsg,
			Cached:           false,
			ChaptersIncluded: len(eligible),
			CreatedAt:        now,
		}, nil
	}

	sections := make([]string, 0, len(eligible))
	backfilled := 0
	for _, ch := range eligible {
		summary := strings.TrimSpace(ch.Summary)
		if summary == "" {
			plain := strings.TrimSpace(ch.ContentPlain)
			if plain == "" {
				continue
			}
			if backfilled < maxRecapSummaryBackfills {
				generated, sumErr := s.aiClient.SummarizeChapter(ctx, chapterHeading(ch), plain)
				if sumErr == nil && strings.TrimSpace(generated) != "" {
					summary = strings.TrimSpace(generated)
					backfilled++
					if upErr := s.repo.UpdateChapterSummary(ctx, ch.ID, summary); upErr != nil {
						s.logger.Warn("failed to persist backfilled chapter summary",
							"book_id", bookID,
							"chapter_id", ch.ID,
							"error", upErr,
						)
					}
				} else {
					if sumErr != nil {
						s.logger.Warn("chapter summary backfill failed, falling back to excerpt",
							"book_id", bookID,
							"chapter_id", ch.ID,
							"error", sumErr,
						)
					}
					summary = truncateRecapExcerpt(plain, maxRecapExcerptChars)
				}
			} else {
				summary = truncateRecapExcerpt(plain, maxRecapExcerptChars)
			}
		}

		sections = append(sections, fmt.Sprintf("### %s\n%s", chapterHeading(ch), summary))
	}

	systemPrompt := fmt.Sprintf(
		"You are a spoiler-safe literary companion creating a \"Previously On\" recap for \"%s\".\n"+
			"The reader has ONLY read up to Chapter %d.\n"+
			"STRICT ANTI-SPOILER RULE: Base your recap EXCLUSIVELY on the chapter summaries and excerpts provided below (Chapters <= %d). "+
			"NEVER mention, hint at, foreshadow, or reference any events, character fates, twists, or plot developments occurring after Chapter %d, even if you know them from outside knowledge.\n"+
			"Write a clear, engaging narrative recap in 2 to 4 short paragraphs covering key events so far, active characters, and unresolved threads.",
		book.Title, chapterIndex, chapterIndex, chapterIndex,
	)

	var userPrompt strings.Builder
	fmt.Fprintf(&userPrompt, "Book: %s\n", book.Title)
	fmt.Fprintf(&userPrompt, "Current progression ceiling: Chapter %d", chapterIndex)
	if paragraphOffset > 0 {
		fmt.Fprintf(&userPrompt, " (paragraph %d)", paragraphOffset)
	}
	userPrompt.WriteString("\n\nProvided chapter context:\n\n")
	userPrompt.WriteString(strings.Join(sections, "\n\n"))

	reply, err := s.aiClient.Chat(ctx, []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt.String()},
	})
	if err != nil {
		return nil, fmt.Errorf("generating recap: %w", err)
	}

	recapText := strings.TrimSpace(reply)
	if recapText == "" {
		return nil, errors.New("ai returned empty recap")
	}

	now := time.Now().UTC()
	if err := s.repo.UpsertRecapCache(ctx, &repository.RecapCacheEntry{
		BookID:          book.ID,
		ChapterIndex:    chapterIndex,
		ParagraphOffset: paragraphOffset,
		Content:         recapText,
		CreatedAt:       now,
	}); err != nil {
		s.logger.Warn("failed to cache generated recap",
			"book_id", bookID,
			"chapter_index", chapterIndex,
			"error", err,
		)
	}

	return &RecapResult{
		BookID:           book.ID,
		BookTitle:        book.Title,
		ChapterIndex:     chapterIndex,
		ParagraphOffset:  paragraphOffset,
		Recap:            recapText,
		Cached:           false,
		ChaptersIncluded: len(eligible),
		CreatedAt:        now,
	}, nil
}

func hasRecapContent(chapters []*repository.Chapter) bool {
	for _, ch := range chapters {
		if strings.TrimSpace(ch.Summary) != "" || strings.TrimSpace(ch.ContentPlain) != "" {
			return true
		}
	}
	return false
}

func chapterHeading(ch *repository.Chapter) string {
	if ch.Title != nil && strings.TrimSpace(*ch.Title) != "" {
		return fmt.Sprintf("Chapter %d: %s", ch.ChapterIndex, strings.TrimSpace(*ch.Title))
	}
	return fmt.Sprintf("Chapter %d", ch.ChapterIndex)
}

func truncateRecapExcerpt(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "..."
}
