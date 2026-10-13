package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/ulid"
)

// isValidID checks if an identifier is a valid ULID or UUID.
func isValidID(s string) bool {
	if s == "" {
		return false
	}
	if ulid.IsValid(s) {
		return true
	}
	_, err := uuid.Parse(s)
	return err == nil
}

// ToolExecutor executes MCP tools against repository and AI services.
type ToolExecutor struct {
	repo     repository.StorageEngine
	aiClient ai.Client
	logger   *slog.Logger
}

// NewToolExecutor creates a new ToolExecutor with structured logger.
func NewToolExecutor(repo repository.StorageEngine, aiClient ai.Client, logger *slog.Logger) *ToolExecutor {
	if logger == nil {
		logger = slog.Default()
	}
	return &ToolExecutor{
		repo:     repo,
		aiClient: aiClient,
		logger:   logger,
	}
}

// AvailableToolDefinitions returns the LLM-compatible tool definitions.
func (te *ToolExecutor) AvailableToolDefinitions() []ai.ToolDefinition {
	return AvailableToolDefinitions()
}

// ExecuteInProcess dispatches a tool call for internal consumers (such as chat agents)
// returning both text content and structured citations.
func (te *ToolExecutor) ExecuteInProcess(ctx context.Context, name string, args map[string]any) (*ai.ToolExecutionResult, error) {
	switch name {
	case "search_library":
		return te.searchLibrary(ctx, args)
	case "get_book_metadata":
		return te.getBookMetadata(ctx, args)
	case "read_chapter_content":
		return te.readChapterContent(ctx, args)
	default:
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("unknown tool: %s", name),
		}, nil
	}
}

// Execute dispatches a tool call to the appropriate tool implementation for JSON-RPC MCP clients.
func (te *ToolExecutor) Execute(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	res, err := te.ExecuteInProcess(ctx, name, args)
	if err != nil {
		return nil, err
	}
	return &CallToolResult{
		IsError: res.IsError,
		Content: []ContentItem{{Type: "text", Text: res.Content}},
	}, nil
}

// searchLibrary performs hybrid semantic vector search across chapter summaries.
func (te *ToolExecutor) searchLibrary(ctx context.Context, args map[string]any) (*ai.ToolExecutionResult, error) {
	query, _ := args["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: "argument 'query' is required and cannot be empty",
		}, nil
	}

	limit := 5
	if rawLimit, ok := args["limit"].(float64); ok && rawLimit > 0 {
		limit = int(rawLimit)
	} else if rawLimitInt, ok := args["limit"].(int); ok && rawLimitInt > 0 {
		limit = rawLimitInt
	}
	if limit > 20 {
		limit = 20
	}

	filter := repository.SearchFilter{
		Limit: limit,
	}

	if rawBookID, ok := args["book_id"].(string); ok && strings.TrimSpace(rawBookID) != "" {
		bookID := strings.TrimSpace(rawBookID)
		if !isValidID(bookID) {
			return &ai.ToolExecutionResult{
				IsError: true,
				Content: fmt.Sprintf("invalid book_id format: %s", bookID),
			}, nil
		}
		filter.BookID = &bookID
	}

	var fallbackNotes []string

	if authorName, ok := args["author"].(string); ok && strings.TrimSpace(authorName) != "" {
		author, err := te.repo.GetAuthorByName(ctx, strings.TrimSpace(authorName))
		if err == nil && author != nil {
			filter.AuthorID = &author.ID
		} else {
			te.logger.Warn("filter author not found, proceeding without author filter", "author", authorName)
			fallbackNotes = append(fallbackNotes, fmt.Sprintf("Author '%s' not found in library; searched across all authors.", authorName))
		}
	}

	if genreName, ok := args["genre"].(string); ok && strings.TrimSpace(genreName) != "" {
		genre, err := te.repo.GetGenreByName(ctx, strings.TrimSpace(genreName))
		if err == nil && genre != nil {
			filter.GenreID = &genre.ID
		} else {
			te.logger.Warn("filter genre not found, proceeding without genre filter", "genre", genreName)
			fallbackNotes = append(fallbackNotes, fmt.Sprintf("Genre '%s' not found in library; searched across all genres.", genreName))
		}
	}

	if topicName, ok := args["topic"].(string); ok && strings.TrimSpace(topicName) != "" {
		topic, err := te.repo.GetTopicByName(ctx, strings.TrimSpace(topicName))
		if err == nil && topic != nil {
			filter.TopicID = &topic.ID
		} else {
			te.logger.Warn("filter topic not found, proceeding without topic filter", "topic", topicName)
			fallbackNotes = append(fallbackNotes, fmt.Sprintf("Topic '%s' not found in library; searched across all topics.", topicName))
		}
	}

	if seriesName, ok := args["series"].(string); ok && strings.TrimSpace(seriesName) != "" {
		series, err := te.repo.GetSeriesByName(ctx, strings.TrimSpace(seriesName))
		if err == nil && series != nil {
			filter.SeriesID = &series.ID
		} else {
			te.logger.Warn("filter series not found, proceeding without series filter", "series", seriesName)
			fallbackNotes = append(fallbackNotes, fmt.Sprintf("Series '%s' not found in library; searched across all series.", seriesName))
		}
	}

	queryEmbedding, err := te.aiClient.GenerateEmbedding(ctx, query)
	if err != nil {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("failed to generate search embedding: %v", err),
		}, nil
	}

	hits, err := te.repo.SearchVectorParagraphs(ctx, queryEmbedding, filter)
	if err != nil {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("vector search failed: %v", err),
		}, nil
	}

	var sb strings.Builder
	for _, note := range fallbackNotes {
		sb.WriteString(fmt.Sprintf("(Note: %s)\n\n", note))
	}

	if len(hits) == 0 {
		sb.WriteString("No matching passages found.")
		te.logger.Info("mcp search_library executed",
			"query", query,
			"hits", 0,
			"book_filter", filter.BookID != nil,
			"author_filter", filter.AuthorID != nil,
			"genre_filter", filter.GenreID != nil,
			"topic_filter", filter.TopicID != nil,
			"series_filter", filter.SeriesID != nil,
		)
		return &ai.ToolExecutionResult{
			Content: strings.TrimSpace(sb.String()),
		}, nil
	}

	sb.WriteString(fmt.Sprintf("Found %d relevant passage(s):\n\n", len(hits)))

	var citations []ai.BookCitation
	for i, hit := range hits {
		author := "Unknown"
		if hit.AuthorName != nil && *hit.AuthorName != "" {
			author = *hit.AuthorName
		}
		chTitle := fmt.Sprintf("Chapter %d", hit.ChapterIndex)
		if hit.ChapterTitle != nil && *hit.ChapterTitle != "" {
			chTitle = fmt.Sprintf("Chapter %d: %s", hit.ChapterIndex, *hit.ChapterTitle)
		}

		// Enforce Level-of-Detail limit (< 100 tokens / ~400 characters)
		excerpt := hit.Content
		if excerpt == "" {
			excerpt = hit.Summary
		}
		if len(excerpt) > 400 {
			cutoff := strings.LastIndex(excerpt[:400], " ")
			if cutoff > 250 {
				excerpt = excerpt[:cutoff] + "..."
			} else {
				excerpt = excerpt[:400] + "..."
			}
		}

		citations = append(citations, ai.BookCitation{
			BookID:         hit.BookID,
			BookTitle:      hit.BookTitle,
			AuthorName:     author,
			ChapterID:      hit.ChapterID,
			ChapterTitle:   chTitle,
			ChapterIndex:   hit.ChapterIndex,
			StartParagraph: hit.StartParagraph,
			EndParagraph:   hit.EndParagraph,
			Excerpt:        excerpt,
			CosineDistance: float32(hit.Distance),
		})

		sb.WriteString(fmt.Sprintf("%d. **%s** by %s\n", i+1, hit.BookTitle, author))
		sb.WriteString(fmt.Sprintf("   Book ID: `%s`\n", hit.BookID))
		if hit.SeriesName != nil && *hit.SeriesName != "" {
			if hit.SeriesIndex != nil {
				sb.WriteString(fmt.Sprintf("   Series: %s (#%g)\n", *hit.SeriesName, *hit.SeriesIndex))
			} else {
				sb.WriteString(fmt.Sprintf("   Series: %s\n", *hit.SeriesName))
			}
		}
		sb.WriteString(fmt.Sprintf("   Section: %s (Chapter ID: `%s`)\n", chTitle, hit.ChapterID))
		if hit.StartParagraph > 0 {
			sb.WriteString(fmt.Sprintf("   Passage: Paragraphs %d–%d\n", hit.StartParagraph, hit.EndParagraph))
		}
		sb.WriteString(fmt.Sprintf("   Cosine Distance: %.4f\n", hit.Distance))
		sb.WriteString(fmt.Sprintf("   Excerpt: %s\n\n", excerpt))
	}

	te.logger.Info("mcp search_library executed",
		"query", query,
		"hits", len(hits),
		"book_filter", filter.BookID != nil,
		"author_filter", filter.AuthorID != nil,
		"genre_filter", filter.GenreID != nil,
		"topic_filter", filter.TopicID != nil,
		"series_filter", filter.SeriesID != nil,
	)

	return &ai.ToolExecutionResult{
		Content:   strings.TrimSpace(sb.String()),
		Citations: citations,
	}, nil
}

// getBookMetadata retrieves complete book details, TOC, and chapter summaries.
func (te *ToolExecutor) getBookMetadata(ctx context.Context, args map[string]any) (*ai.ToolExecutionResult, error) {
	bookID, _ := args["book_id"].(string)
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: "argument 'book_id' is required",
		}, nil
	}
	if !isValidID(bookID) {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("invalid book_id format: %s", bookID),
		}, nil
	}

	book, err := te.repo.GetBookByID(ctx, bookID)
	if err != nil {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("book with ID '%s' not found", bookID),
		}, nil
	}

	authors, _ := te.repo.GetBookAuthors(ctx, bookID)
	genres, _ := te.repo.GetBookGenres(ctx, bookID)
	topics, _ := te.repo.GetBookTopics(ctx, bookID)
	seriesList, _ := te.repo.GetBookSeries(ctx, bookID)
	chapters, _ := te.repo.GetChaptersByBookID(ctx, bookID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", book.Title))
	sb.WriteString(fmt.Sprintf("- **Book ID**: `%s`\n", book.ID))

	if len(authors) > 0 {
		var authorNames []string
		for _, a := range authors {
			authorNames = append(authorNames, a.Name)
		}
		sb.WriteString(fmt.Sprintf("- **Author(s)**: %s\n", strings.Join(authorNames, ", ")))
	}

	if len(seriesList) > 0 {
		var seriesParts []string
		for _, s := range seriesList {
			if s.SequenceNumber != nil {
				seriesParts = append(seriesParts, fmt.Sprintf("%s (#%g)", s.Name, *s.SequenceNumber))
			} else {
				seriesParts = append(seriesParts, s.Name)
			}
		}
		sb.WriteString(fmt.Sprintf("- **Series**: %s\n", strings.Join(seriesParts, ", ")))
	}

	if len(genres) > 0 {
		var genreNames []string
		for _, g := range genres {
			genreNames = append(genreNames, g.Name)
		}
		sb.WriteString(fmt.Sprintf("- **Genres**: %s\n", strings.Join(genreNames, ", ")))
	}

	if len(topics) > 0 {
		var topicNames []string
		for _, t := range topics {
			topicNames = append(topicNames, t.Name)
		}
		sb.WriteString(fmt.Sprintf("- **Topics**: %s\n", strings.Join(topicNames, ", ")))
	}

	if book.Publisher != nil && *book.Publisher != "" {
		sb.WriteString(fmt.Sprintf("- **Publisher**: %s\n", *book.Publisher))
	}
	if book.Language != nil && *book.Language != "" {
		sb.WriteString(fmt.Sprintf("- **Language**: %s\n", *book.Language))
	}
	if book.Description != nil && *book.Description != "" {
		sb.WriteString(fmt.Sprintf("\n### Description\n%s\n", *book.Description))
	}

	sb.WriteString(fmt.Sprintf("\n### Table of Contents (%d chapters)\n", len(chapters)))
	if len(chapters) == 0 {
		sb.WriteString("No chapters indexed.\n")
	} else {
		for _, ch := range chapters {
			title := fmt.Sprintf("Chapter %d", ch.ChapterIndex)
			if ch.Title != nil && *ch.Title != "" {
				title = fmt.Sprintf("Chapter %d: %s", ch.ChapterIndex, *ch.Title)
			}
			sb.WriteString(fmt.Sprintf("\n- **%s** (ID: `%s`)\n", title, ch.ID))
			if ch.Summary != "" {
				sb.WriteString(fmt.Sprintf("  Summary: %s\n", ch.Summary))
			}
		}
	}

	te.logger.Debug("mcp get_book_metadata executed", "book_id", bookID, "title", book.Title, "chapters", len(chapters))

	return &ai.ToolExecutionResult{
		Content: strings.TrimSpace(sb.String()),
	}, nil
}

// readChapterContent retrieves plain text for a specified chapter with paragraph windowing.
func (te *ToolExecutor) readChapterContent(ctx context.Context, args map[string]any) (*ai.ToolExecutionResult, error) {
	bookID, _ := args["book_id"].(string)
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: "argument 'book_id' is required",
		}, nil
	}
	if !isValidID(bookID) {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("invalid book_id format: %s", bookID),
		}, nil
	}

	chapterID, _ := args["chapter_id"].(string)
	chapterID = strings.TrimSpace(chapterID)
	if chapterID != "" && !isValidID(chapterID) {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("invalid chapter_id format: %s", chapterID),
		}, nil
	}

	var chapterIndex int
	hasIndex := false
	if rawIdx, ok := args["chapter_index"].(float64); ok {
		chapterIndex = int(rawIdx)
		hasIndex = true
	} else if rawIdxInt, ok := args["chapter_index"].(int); ok {
		chapterIndex = rawIdxInt
		hasIndex = true
	}

	if chapterID == "" && !hasIndex {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: "either 'chapter_id' or 'chapter_index' is required",
		}, nil
	}

	book, err := te.repo.GetBookByID(ctx, bookID)
	if err != nil {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("book with ID '%s' not found", bookID),
		}, nil
	}

	var chapter *repository.Chapter
	if chapterID != "" {
		chapter, err = te.repo.GetChapterByID(ctx, chapterID)
		if err != nil || (chapter != nil && chapter.BookID != bookID) {
			return &ai.ToolExecutionResult{
				IsError: true,
				Content: fmt.Sprintf("chapter '%s' not found in book '%s'", chapterID, book.Title),
			}, nil
		}
	} else {
		chapter, err = te.repo.GetChapterByBookAndIndex(ctx, bookID, chapterIndex)
		if err != nil {
			return &ai.ToolExecutionResult{
				IsError: true,
				Content: fmt.Sprintf("chapter %d not found in book '%s'", chapterIndex, book.Title),
			}, nil
		}
	}

	// Split into paragraphs
	normalized := strings.ReplaceAll(chapter.ContentPlain, "\r\n", "\n")
	rawParas := strings.Split(normalized, "\n\n")
	var paras []string
	for _, p := range rawParas {
		p = strings.TrimSpace(p)
		if p != "" {
			paras = append(paras, p)
		}
	}

	totalParas := len(paras)
	if totalParas == 0 {
		return &ai.ToolExecutionResult{
			Content: fmt.Sprintf("[Book: %s | Chapter %d]\n\n(This chapter contains no text content.)", book.Title, chapterIndex),
		}, nil
	}

	startPara := 1
	if rawStart, ok := args["start_paragraph"].(float64); ok && rawStart > 0 {
		startPara = int(rawStart)
	} else if rawStartInt, ok := args["start_paragraph"].(int); ok && rawStartInt > 0 {
		startPara = rawStartInt
	}

	if startPara > totalParas {
		return &ai.ToolExecutionResult{
			IsError: true,
			Content: fmt.Sprintf("start_paragraph (%d) exceeds total paragraphs (%d)", startPara, totalParas),
		}, nil
	}

	const maxWindow = 50
	endPara := startPara + maxWindow - 1
	if rawEnd, ok := args["end_paragraph"].(float64); ok && rawEnd >= float64(startPara) {
		endPara = int(rawEnd)
	} else if rawEndInt, ok := args["end_paragraph"].(int); ok && rawEndInt >= startPara {
		endPara = rawEndInt
	}

	// Enforce LOD max window limit
	if endPara > startPara+maxWindow-1 {
		endPara = startPara + maxWindow - 1
	}
	if endPara > totalParas {
		endPara = totalParas
	}

	chTitle := fmt.Sprintf("Chapter %d", chapter.ChapterIndex)
	if chapter.Title != nil && *chapter.Title != "" {
		chTitle = fmt.Sprintf("Chapter %d: %s", chapter.ChapterIndex, *chapter.Title)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Book: %s | %s]\n", book.Title, chTitle))
	sb.WriteString(fmt.Sprintf("[Showing paragraphs %d through %d of %d total]\n\n", startPara, endPara, totalParas))

	for i := startPara - 1; i < endPara; i++ {
		sb.WriteString(fmt.Sprintf("[¶%d] %s\n\n", i+1, paras[i]))
	}

	te.logger.Debug("mcp read_chapter_content executed",
		"book_id", bookID,
		"chapter_index", chapter.ChapterIndex,
		"start_paragraph", startPara,
		"end_paragraph", endPara,
		"total_paragraphs", totalParas,
	)

	author := "Unknown"
	if authors, _ := te.repo.GetBookAuthors(ctx, bookID); len(authors) > 0 && authors[0].Name != "" {
		author = authors[0].Name
	}
	sampleExcerpt := ""
	if len(paras) >= startPara {
		sampleExcerpt = paras[startPara-1]
		if len(sampleExcerpt) > 200 {
			sampleExcerpt = sampleExcerpt[:200] + "..."
		}
	}

	citation := ai.BookCitation{
		BookID:         book.ID,
		BookTitle:      book.Title,
		AuthorName:     author,
		ChapterID:      chapter.ID,
		ChapterTitle:   chTitle,
		ChapterIndex:   chapter.ChapterIndex,
		StartParagraph: startPara,
		EndParagraph:   endPara,
		Excerpt:        sampleExcerpt,
	}

	return &ai.ToolExecutionResult{
		Content:   strings.TrimSpace(sb.String()),
		Citations: []ai.BookCitation{citation},
	}, nil
}
