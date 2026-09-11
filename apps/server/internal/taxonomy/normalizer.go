package taxonomy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

var (
	htmlTagRegex  = regexp.MustCompile(`<[^>]*>`)
	bisacCodeRegex = regexp.MustCompile(`^[A-Z]{3}\d{6}$`)
)

type TaxonomyResult struct {
	Genres []string `json:"genres"`
	Topics []string `json:"topics"`
}

type TaxonomyService struct {
	repo     repository.StorageEngine
	aiClient ai.Client
	logger   *slog.Logger
}

func NewTaxonomyService(repo repository.StorageEngine, aiClient ai.Client, logger *slog.Logger) *TaxonomyService {
	if logger == nil {
		logger = slog.Default()
	}
	return &TaxonomyService{
		repo:     repo,
		aiClient: aiClient,
		logger:   logger,
	}
}

const SystemTaxonomyPrompt = `You are an expert literary taxonomy archivist.
Your job is to analyze a book's metadata (title, author, raw subjects, synopsis/description, chapter summaries) and partition its taxonomy into:
1. "genres": 1-3 broad, canonical literary forms or genres (e.g. "Science Fiction", "Fantasy", "Mystery", "Thriller", "Horror", "Romance", "Historical Fiction", "Non-Fiction", "Biography", "Philosophy", "Science", "Technology", "Cyberpunk", "Dystopian").
2. "topics": 2-6 specific themes, conceptual topics, technologies, or subjects (e.g. "Artificial Intelligence", "Space Exploration", "Genetic Engineering", "Virtual Reality", "Surveillance", "Cold War", "Time Travel", "Climate Change").

Guidelines:
- Match topics against the provided existing library topics if applicable.
- If new topics are needed, use user-friendly, title-cased conceptual phrases.
- Analyze the book's synopsis and chapter summaries to discover themes even if they are not mentioned in the raw subjects.
- Do NOT use BISAC codes (e.g. "FIC028010" or "FICTION / Science Fiction / General").
- Respond ONLY with a valid JSON object in the exact shape: {"genres": ["..."], "topics": ["..."]}`

func (s *TaxonomyService) NormalizeBookTaxonomy(ctx context.Context, bookID string, force bool) (*TaxonomyResult, error) {
	if !force {
		existingTopics, err := s.repo.GetBookTopics(ctx, bookID)
		if err == nil && len(existingTopics) > 0 {
			existingGenres, _ := s.repo.GetBookGenres(ctx, bookID)
			res := &TaxonomyResult{
				Genres: make([]string, len(existingGenres)),
				Topics: make([]string, len(existingTopics)),
			}
			for i, g := range existingGenres {
				res.Genres[i] = g.Name
			}
			for i, t := range existingTopics {
				res.Topics[i] = t.Name
			}
			return res, nil
		}
	}

	book, err := s.repo.GetBookByID(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("getting book %s: %w", bookID, err)
	}

	authors, _ := s.repo.GetBookAuthors(ctx, bookID)
	rawGenres, _ := s.repo.GetBookGenres(ctx, bookID)
	chapters, _ := s.repo.GetChaptersByBookID(ctx, bookID)
	existingLibTopics, _ := s.repo.ListTopics(ctx)

	var result *TaxonomyResult
	if s.aiClient != nil {
		prompt := buildPrompt(book, authors, rawGenres, chapters, existingLibTopics)
		messages := []ai.ChatMessage{
			{Role: "system", Content: SystemTaxonomyPrompt},
			{Role: "user", Content: prompt},
		}

		reply, err := s.aiClient.Chat(ctx, messages)
		if err == nil {
			result = parseTaxonomyResponse(reply)
		} else {
			s.logger.Warn("AI taxonomy classification failed, falling back to heuristic", "book_id", bookID, "error", err)
		}
	}

	if result == nil || (len(result.Genres) == 0 && len(result.Topics) == 0) {
		rawGenreNames := make([]string, len(rawGenres))
		for i, g := range rawGenres {
			rawGenreNames[i] = g.Name
		}
		var desc string
		if book.Description != nil {
			desc = *book.Description
		}
		result = FallbackTaxonomy(rawGenreNames, book.Title, desc)
	}

	result.Genres = cleanList(result.Genres, 3)
	result.Topics = cleanList(result.Topics, 6)

	if len(result.Genres) == 0 {
		result.Genres = []string{"General"}
	}

	// 1. Link topics
	for _, topicName := range result.Topics {
		t, err := s.repo.UpsertTopic(ctx, topicName)
		if err != nil {
			continue
		}
		_ = s.repo.LinkBookTopic(ctx, bookID, t.ID)
	}

	// 2. Link canonical genres and track new genre IDs
	newGenreIDs := make(map[string]bool)
	for _, genreName := range result.Genres {
		g, err := s.repo.UpsertGenre(ctx, genreName)
		if err != nil {
			continue
		}
		newGenreIDs[g.ID] = true
		_ = s.repo.LinkBookGenre(ctx, bookID, g.ID)
	}

	// 3. Unlink old genres not present in new canonical genres
	for _, oldG := range rawGenres {
		if !newGenreIDs[oldG.ID] {
			_ = s.repo.UnlinkBookGenre(ctx, bookID, oldG.ID)
		}
	}

	return result, nil
}

func (s *TaxonomyService) MigrateLibraryTopics(ctx context.Context, force bool) (int, error) {
	books, err := s.repo.ListBooks(ctx, repository.BookFilter{Limit: 100000})
	if err != nil {
		return 0, fmt.Errorf("listing books for topic migration: %w", err)
	}

	migrated := 0
	for _, b := range books {
		_, err := s.NormalizeBookTaxonomy(ctx, b.ID, force)
		if err != nil {
			s.logger.Warn("Failed normalizing taxonomy for book", "book_id", b.ID, "title", b.Title, "error", err)
			continue
		}
		migrated++
	}

	pruned, err := s.repo.PruneOrphanedGenres(ctx)
	if err != nil {
		s.logger.Warn("Failed pruning orphaned genres after migration", "error", err)
	} else if pruned > 0 {
		s.logger.Info("Pruned orphaned genres", "count", pruned)
	}

	return migrated, nil
}

func buildPrompt(
	book *repository.Book,
	authors []*repository.Author,
	rawGenres []*repository.Genre,
	chapters []*repository.Chapter,
	existingTopics []*repository.Topic,
) string {
	var sb strings.Builder
	sb.WriteString("Title: ")
	sb.WriteString(book.Title)
	sb.WriteString("\n")

	if len(authors) > 0 {
		var authorNames []string
		for _, a := range authors {
			authorNames = append(authorNames, a.Name)
		}
		sb.WriteString("Authors: ")
		sb.WriteString(strings.Join(authorNames, ", "))
		sb.WriteString("\n")
	}

	if len(rawGenres) > 0 {
		var gNames []string
		for _, g := range rawGenres {
			gNames = append(gNames, g.Name)
		}
		sb.WriteString("Raw Metadata Subjects: ")
		sb.WriteString(strings.Join(gNames, ", "))
		sb.WriteString("\n")
	}

	if len(existingTopics) > 0 {
		var tNames []string
		for i, t := range existingTopics {
			if i >= 60 {
				break
			}
			tNames = append(tNames, t.Name)
		}
		sb.WriteString("Existing Library Topics (reuse if suitable): ")
		sb.WriteString(strings.Join(tNames, ", "))
		sb.WriteString("\n")
	}

	if book.Description != nil && strings.TrimSpace(*book.Description) != "" {
		cleanDesc := cleanHTML(*book.Description)
		if len(cleanDesc) > 3000 {
			cleanDesc = cleanDesc[:3000] + "..."
		}
		sb.WriteString("\nBook Synopsis:\n")
		sb.WriteString(cleanDesc)
		sb.WriteString("\n")
	}

	var summaries []string
	for _, c := range chapters {
		if strings.TrimSpace(c.Summary) != "" {
			summaries = append(summaries, c.Summary)
			if len(summaries) >= 4 {
				break
			}
		}
	}
	if len(summaries) > 0 {
		sb.WriteString("\nSample Chapter Summaries:\n")
		for _, s := range summaries {
			sb.WriteString("- ")
			sb.WriteString(s)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func cleanHTML(s string) string {
	clean := htmlTagRegex.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(clean), " ")
}

func parseTaxonomyResponse(raw string) *TaxonomyResult {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		lines := strings.Split(clean, "\n")
		if len(lines) > 2 {
			clean = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	jsonBody := clean[start : end+1]

	var res TaxonomyResult
	if err := json.Unmarshal([]byte(jsonBody), &res); err != nil {
		return nil
	}
	return &res
}

var knownGenres = map[string]string{
	"fiction":             "Fiction",
	"non-fiction":         "Non-Fiction",
	"nonfiction":          "Non-Fiction",
	"science fiction":     "Science Fiction",
	"sci-fi":              "Science Fiction",
	"fantasy":             "Fantasy",
	"mystery":             "Mystery",
	"thriller":            "Thriller",
	"horror":              "Horror",
	"romance":             "Romance",
	"historical fiction":  "Historical Fiction",
	"biography":           "Biography",
	"autobiography":       "Autobiography",
	"memoir":              "Memoir",
	"philosophy":          "Philosophy",
	"history":             "History",
	"science":             "Science",
	"technology":          "Technology",
	"poetry":              "Poetry",
	"drama":               "Drama",
	"young adult":         "Young Adult",
	"ya":                  "Young Adult",
	"cyberpunk":           "Cyberpunk",
	"dystopian":           "Dystopian",
}

func FallbackTaxonomy(rawSubjects []string, title, description string) *TaxonomyResult {
	res := &TaxonomyResult{
		Genres: []string{},
		Topics: []string{},
	}

	for _, raw := range rawSubjects {
		clean := strings.TrimSpace(raw)
		if clean == "" || bisacCodeRegex.MatchString(clean) {
			continue
		}

		if strings.Contains(clean, " / ") {
			parts := strings.Split(clean, " / ")
			for _, part := range parts {
				pClean := strings.TrimSpace(part)
				if pClean == "" || strings.EqualFold(pClean, "General") || bisacCodeRegex.MatchString(pClean) {
					continue
				}
				assignTerm(res, pClean)
			}
			continue
		}

		assignTerm(res, clean)
	}

	// If no topics yet, scan description/title for common themes
	fullText := strings.ToLower(title + " " + description)
	themeKeywords := map[string]string{
		"artificial intelligence": "Artificial Intelligence",
		"machine learning":        "Artificial Intelligence",
		"space travel":            "Space Exploration",
		"space exploration":       "Space Exploration",
		"virtual reality":         "Virtual Reality",
		"cybernetics":             "Cybernetics",
		"genetic engineering":     "Genetic Engineering",
		"climate change":          "Climate Change",
		"surveillance":            "Surveillance",
	}
	for kw, topic := range themeKeywords {
		if strings.Contains(fullText, kw) {
			res.Topics = append(res.Topics, topic)
		}
	}

	if len(res.Genres) > 1 {
		var specific []string
		for _, g := range res.Genres {
			if g != "Fiction" && g != "Non-Fiction" {
				specific = append(specific, g)
			}
		}
		if len(specific) > 0 {
			res.Genres = specific
		}
	}

	return res
}

func assignTerm(res *TaxonomyResult, term string) {
	lower := strings.ToLower(term)
	if canonical, ok := knownGenres[lower]; ok {
		res.Genres = append(res.Genres, canonical)
	} else {
		res.Topics = append(res.Topics, formatTitleCase(term))
	}
}

func formatTitleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

func cleanList(items []string, maxItems int) []string {
	seen := make(map[string]bool)
	var out []string
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		out = append(out, trimmed)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}
