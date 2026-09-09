package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

// ToolExecutor executes MCP tools against repository and AI services.
type ToolExecutor struct {
	repo     repository.StorageEngine
	aiClient ai.Client
}

// NewToolExecutor creates a new ToolExecutor.
func NewToolExecutor(repo repository.StorageEngine, aiClient ai.Client) *ToolExecutor {
	return &ToolExecutor{
		repo:     repo,
		aiClient: aiClient,
	}
}

// Execute dispatches a tool call to the appropriate tool implementation.
func (te *ToolExecutor) Execute(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	switch name {
	case "search_library":
		return te.searchLibrary(ctx, args)
	case "get_book_metadata":
		return te.getBookMetadata(ctx, args)
	case "read_chapter_content":
		return te.readChapterContent(ctx, args)
	default:
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("unknown tool: %s", name),
				},
			},
		}, nil
	}
}

// searchLibrary performs hybrid semantic vector search across chapter summaries.
func (te *ToolExecutor) searchLibrary(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	query, _ := args["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "argument 'query' is required and cannot be empty"}},
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

	if authorName, ok := args["author"].(string); ok && strings.TrimSpace(authorName) != "" {
		author, err := te.repo.GetAuthorByName(ctx, strings.TrimSpace(authorName))
		if err == nil && author != nil {
			filter.AuthorID = &author.ID
		} else {
			// Specified author does not exist
			return &CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("No results found: author '%s' not found in library.", authorName)}},
			}, nil
		}
	}

	if genreName, ok := args["genre"].(string); ok && strings.TrimSpace(genreName) != "" {
		genre, err := te.repo.GetGenreByName(ctx, strings.TrimSpace(genreName))
		if err == nil && genre != nil {
			filter.GenreID = &genre.ID
		} else {
			return &CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("No results found: genre '%s' not found in library.", genreName)}},
			}, nil
		}
	}

	if seriesName, ok := args["series"].(string); ok && strings.TrimSpace(seriesName) != "" {
		series, err := te.repo.GetSeriesByName(ctx, strings.TrimSpace(seriesName))
		if err == nil && series != nil {
			filter.SeriesID = &series.ID
		} else {
			return &CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("No results found: series '%s' not found in library.", seriesName)}},
			}, nil
		}
	}

	queryEmbedding, err := te.aiClient.GenerateEmbedding(ctx, query)
	if err != nil {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("failed to generate search embedding: %v", err)}},
		}, nil
	}

	hits, err := te.repo.SearchVectorParagraphs(ctx, queryEmbedding, filter)
	if err != nil {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("vector search failed: %v", err)}},
		}, nil
	}

	if len(hits) == 0 {
		return &CallToolResult{
			Content: []ContentItem{{Type: "text", Text: "No matching passages found."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d relevant passage(s):\n\n", len(hits)))

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
	sb.WriteString("(Tip: Call read_chapter_content with book_id, chapter_index, start_paragraph, and end_paragraph to read full context.)")

	return &CallToolResult{
		Content: []ContentItem{{Type: "text", Text: strings.TrimSpace(sb.String())}},
	}, nil
}

// getBookMetadata retrieves complete book details, TOC, and chapter summaries.
func (te *ToolExecutor) getBookMetadata(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	bookID, _ := args["book_id"].(string)
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "argument 'book_id' is required"}},
		}, nil
	}

	book, err := te.repo.GetBookByID(ctx, bookID)
	if err != nil {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("book with ID '%s' not found", bookID)}},
		}, nil
	}

	authors, _ := te.repo.GetBookAuthors(ctx, bookID)
	genres, _ := te.repo.GetBookGenres(ctx, bookID)
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

	return &CallToolResult{
		Content: []ContentItem{{Type: "text", Text: strings.TrimSpace(sb.String())}},
	}, nil
}

// readChapterContent retrieves plain text for a specified chapter with paragraph windowing.
func (te *ToolExecutor) readChapterContent(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	bookID, _ := args["book_id"].(string)
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "argument 'book_id' is required"}},
		}, nil
	}

	chapterID, _ := args["chapter_id"].(string)
	chapterID = strings.TrimSpace(chapterID)

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
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: "either 'chapter_id' or 'chapter_index' is required"}},
		}, nil
	}

	book, err := te.repo.GetBookByID(ctx, bookID)
	if err != nil {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("book with ID '%s' not found", bookID)}},
		}, nil
	}

	var chapter *repository.Chapter
	if chapterID != "" {
		chapter, err = te.repo.GetChapterByID(ctx, chapterID)
		if err != nil || (chapter != nil && chapter.BookID != bookID) {
			return &CallToolResult{
				IsError: true,
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("chapter '%s' not found in book '%s'", chapterID, book.Title)}},
			}, nil
		}
	} else {
		chapter, err = te.repo.GetChapterByBookAndIndex(ctx, bookID, chapterIndex)
		if err != nil {
			return &CallToolResult{
				IsError: true,
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("chapter %d not found in book '%s'", chapterIndex, book.Title)}},
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
		return &CallToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("[Book: %s | Chapter %d]\n\n(This chapter contains no text content.)", book.Title, chapterIndex)}},
		}, nil
	}

	startPara := 1
	if rawStart, ok := args["start_paragraph"].(float64); ok && rawStart > 0 {
		startPara = int(rawStart)
	} else if rawStartInt, ok := args["start_paragraph"].(int); ok && rawStartInt > 0 {
		startPara = rawStartInt
	}

	if startPara > totalParas {
		return &CallToolResult{
			IsError: true,
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("start_paragraph (%d) exceeds total paragraphs (%d)", startPara, totalParas)}},
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

	return &CallToolResult{
		Content: []ContentItem{{Type: "text", Text: strings.TrimSpace(sb.String())}},
	}, nil
}
