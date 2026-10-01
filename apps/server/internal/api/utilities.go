package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/ulid"
)

type UtilityHandler struct {
	repo       repository.StorageEngine
	dataDir    string
	libraryDir string
	hub        *events.Hub
	logger     *slog.Logger
}

func NewUtilityHandler(
	repo repository.StorageEngine,
	dataDir string,
	libraryDir string,
	hub *events.Hub,
	logger *slog.Logger,
) *UtilityHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &UtilityHandler{
		repo:       repo,
		dataDir:    dataDir,
		libraryDir: libraryDir,
		hub:        hub,
		logger:     logger,
	}
}

type DuplicateGroupDTO struct {
	ID          string         `json:"id"`
	MatchReason string         `json:"match_reason"`
	Confidence  float64        `json:"confidence"`
	Books       []BookListItem `json:"books"`
}

type DuplicateScanResponse struct {
	Groups              []DuplicateGroupDTO `json:"groups"`
	TotalGroups         int                 `json:"total_groups"`
	TotalDuplicateBooks int                 `json:"total_duplicate_books"`
}

type MergeBooksOptions struct {
	TransferBookmarks  *bool `json:"transfer_bookmarks"`
	TransferHighlights *bool `json:"transfer_highlights"`
	MergeMetadata      *bool `json:"merge_metadata"`
	DeleteFiles        *bool `json:"delete_files"`
}

type MergeBooksRequest struct {
	PrimaryBookID    string             `json:"primary_book_id"`
	DuplicateBookIDs []string           `json:"duplicate_book_ids"`
	Options          *MergeBooksOptions `json:"options,omitempty"`
}

// FindDuplicates scans all books in the database and identifies duplicate candidate groups.
func (h *UtilityHandler) FindDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	books, err := h.repo.ListBooks(ctx, repository.BookFilter{})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list books: %v", err))
		return
	}

	if len(books) < 2 {
		writeJSON(w, http.StatusOK, DuplicateScanResponse{
			Groups:              []DuplicateGroupDTO{},
			TotalGroups:         0,
			TotalDuplicateBooks: 0,
		})
		return
	}

	// Fetch authors for all books to aid duplicate matching
	type bookMeta struct {
		book       *repository.Book
		normTitle  string
		normISBN   string
		authorIDs  map[string]bool
		authorNorm []string
	}

	metaList := make([]bookMeta, len(books))
	for i, b := range books {
		authors, err := h.repo.GetBookAuthors(ctx, b.ID)
		if err != nil {
			authors = []*repository.Author{}
		}

		aMap := make(map[string]bool, len(authors))
		aNorm := make([]string, 0, len(authors))
		for _, a := range authors {
			aMap[a.ID] = true
			aNorm = append(aNorm, normalizeText(a.Name))
		}

		metaList[i] = bookMeta{
			book:       b,
			normTitle:  normalizeText(b.Title),
			normISBN:   normalizeIdentifier(b.Identifier),
			authorIDs:  aMap,
			authorNorm: aNorm,
		}
	}

	// Disjoint Set Union (DSU) to cluster matching duplicate books
	parent := make([]int, len(books))
	for i := range parent {
		parent[i] = i
	}
	var findRoot func(int) int
	findRoot = func(i int) int {
		if parent[i] == i {
			return i
		}
		parent[i] = findRoot(parent[i])
		return parent[i]
	}
	union := func(i, j int) {
		rootI := findRoot(i)
		rootJ := findRoot(j)
		if rootI != rootJ {
			parent[rootI] = rootJ
		}
	}

	// Store match reasons between pairs
	type pairMatch struct {
		reason     string
		confidence float64
	}
	matchMatrix := make(map[string]pairMatch)

	for i := 0; i < len(books); i++ {
		for j := i + 1; j < len(books); j++ {
			b1 := metaList[i]
			b2 := metaList[j]

			// 1. Check matching non-empty identifier (ISBN/URN)
			if b1.normISBN != "" && b2.normISBN != "" && b1.normISBN == b2.normISBN {
				union(i, j)
				pairKey := fmt.Sprintf("%d-%d", i, j)
				matchMatrix[pairKey] = pairMatch{
					reason:     "Matching Identifier (ISBN)",
					confidence: 1.0,
				}
				continue
			}

			// Check author overlap
			hasMatchingAuthor := false
			if len(b1.authorNorm) == 0 && len(b2.authorNorm) == 0 {
				hasMatchingAuthor = true
			} else {
				for _, a1 := range b1.authorNorm {
					for _, a2 := range b2.authorNorm {
						if a1 != "" && a2 != "" && (a1 == a2 || strings.Contains(a1, a2) || strings.Contains(a2, a1)) {
							hasMatchingAuthor = true
							break
						}
					}
					if hasMatchingAuthor {
						break
					}
				}
			}

			// 2. Exact normalized title match
			if b1.normTitle != "" && b1.normTitle == b2.normTitle {
				if hasMatchingAuthor || len(strings.Fields(b1.normTitle)) >= 3 {
					union(i, j)
					pairKey := fmt.Sprintf("%d-%d", i, j)
					matchMatrix[pairKey] = pairMatch{
						reason:     "Identical Title & Author",
						confidence: 0.95,
					}
					continue
				}
			}

			// 3. Subtitle / edition variations with matching author
			if hasMatchingAuthor && b1.normTitle != "" && b2.normTitle != "" {
				s1 := stripSubtitles(b1.normTitle)
				s2 := stripSubtitles(b2.normTitle)
				if s1 != "" && s1 == s2 {
					union(i, j)
					pairKey := fmt.Sprintf("%d-%d", i, j)
					matchMatrix[pairKey] = pairMatch{
						reason:     "Similar Title & Matching Author",
						confidence: 0.85,
					}
					continue
				}

				// Fuzzy similarity for minor punctuation or typos
				if len(b1.normTitle) > 5 && len(b2.normTitle) > 5 {
					sim := calculateSimilarity(b1.normTitle, b2.normTitle)
					if sim >= 0.88 {
						union(i, j)
						pairKey := fmt.Sprintf("%d-%d", i, j)
						matchMatrix[pairKey] = pairMatch{
							reason:     "Similar Title & Matching Author",
							confidence: 0.80,
						}
						continue
					}
				}
			}
		}
	}

	// Group indices by root
	groupsMap := make(map[int][]int)
	for i := range books {
		root := findRoot(i)
		groupsMap[root] = append(groupsMap[root], i)
	}

	var duplicateGroups []DuplicateGroupDTO
	totalDuplicateBooks := 0

	for _, indices := range groupsMap {
		if len(indices) < 2 {
			continue
		}

		// Determine highest confidence and most descriptive reason for the cluster
		bestReason := "Duplicate detected"
		bestConfidence := 0.80

		for _, idx1 := range indices {
			for _, idx2 := range indices {
				if idx1 < idx2 {
					pairKey := fmt.Sprintf("%d-%d", idx1, idx2)
					if pm, ok := matchMatrix[pairKey]; ok {
						if pm.confidence > bestConfidence {
							bestConfidence = pm.confidence
							bestReason = pm.reason
						}
					}
				}
			}
		}

		// Build DTOs for each book in group
		groupBooks := make([]BookListItem, 0, len(indices))
		for _, idx := range indices {
			b := books[idx]
			item := BuildBookListItem(ctx, h.repo, b)
			groupBooks = append(groupBooks, item)
		}

		// Sort books in group: prefer non-empty description, larger file size, or more authors
		sort.Slice(groupBooks, func(i, j int) bool {
			scoreI := 0
			scoreJ := 0
			if groupBooks[i].Description != nil && *groupBooks[i].Description != "" {
				scoreI += 10
			}
			if groupBooks[j].Description != nil && *groupBooks[j].Description != "" {
				scoreJ += 10
			}
			if len(groupBooks[i].Authors) > 0 {
				scoreI += 5
			}
			if len(groupBooks[j].Authors) > 0 {
				scoreJ += 5
			}
			sizeI := int64(0)
			if groupBooks[i].FileSizeBytes != nil {
				sizeI = *groupBooks[i].FileSizeBytes
			}
			sizeJ := int64(0)
			if groupBooks[j].FileSizeBytes != nil {
				sizeJ = *groupBooks[j].FileSizeBytes
			}
			if sizeI > sizeJ {
				scoreI += 2
			} else if sizeJ > sizeI {
				scoreJ += 2
			}
			return scoreI > scoreJ
		})

		groupId := fmt.Sprintf("dup_%s", ulid.New())
		duplicateGroups = append(duplicateGroups, DuplicateGroupDTO{
			ID:          groupId,
			MatchReason: bestReason,
			Confidence:  bestConfidence,
			Books:       groupBooks,
		})
		totalDuplicateBooks += len(groupBooks)
	}

	// Sort groups by confidence descending, then by book count descending
	sort.Slice(duplicateGroups, func(i, j int) bool {
		if duplicateGroups[i].Confidence != duplicateGroups[j].Confidence {
			return duplicateGroups[i].Confidence > duplicateGroups[j].Confidence
		}
		return len(duplicateGroups[i].Books) > len(duplicateGroups[j].Books)
	})

	writeJSON(w, http.StatusOK, DuplicateScanResponse{
		Groups:              duplicateGroups,
		TotalGroups:         len(duplicateGroups),
		TotalDuplicateBooks: totalDuplicateBooks,
	})
}

// MergeBooks consolidates duplicate books into a selected primary book.
func (h *UtilityHandler) MergeBooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req MergeBooksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if strings.TrimSpace(req.PrimaryBookID) == "" {
		writeJSONError(w, http.StatusBadRequest, "primary_book_id is required")
		return
	}
	if len(req.DuplicateBookIDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "duplicate_book_ids must contain at least one book id")
		return
	}

	for _, dupID := range req.DuplicateBookIDs {
		if dupID == req.PrimaryBookID {
			writeJSONError(w, http.StatusBadRequest, "primary_book_id cannot be in duplicate_book_ids")
			return
		}
	}

	ctx := r.Context()
	primaryBook, err := h.repo.GetBookByID(ctx, req.PrimaryBookID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("Primary book not found: %v", err))
		return
	}

	// Validate and fetch all duplicate books
	dupBooks := make([]*repository.Book, 0, len(req.DuplicateBookIDs))
	for _, dupID := range req.DuplicateBookIDs {
		b, err := h.repo.GetBookByID(ctx, dupID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("Duplicate book %q not found: %v", dupID, err))
			return
		}
		dupBooks = append(dupBooks, b)
	}

	// Options defaults
	transferBookmarks := true
	transferHighlights := true
	mergeMetadata := true
	deleteFiles := true

	if req.Options != nil {
		if req.Options.TransferBookmarks != nil {
			transferBookmarks = *req.Options.TransferBookmarks
		}
		if req.Options.TransferHighlights != nil {
			transferHighlights = *req.Options.TransferHighlights
		}
		if req.Options.MergeMetadata != nil {
			mergeMetadata = *req.Options.MergeMetadata
		}
		if req.Options.DeleteFiles != nil {
			deleteFiles = *req.Options.DeleteFiles
		}
	}

	// Merge metadata from duplicate books into primary book
	if mergeMetadata {
		primaryAuthors, _ := h.repo.GetBookAuthors(ctx, primaryBook.ID)
		primaryGenres, _ := h.repo.GetBookGenres(ctx, primaryBook.ID)
		primaryTopics, _ := h.repo.GetBookTopics(ctx, primaryBook.ID)
		primarySeries, _ := h.repo.GetBookSeries(ctx, primaryBook.ID)

		authorSet := make(map[string]bool, len(primaryAuthors))
		for _, a := range primaryAuthors {
			authorSet[a.ID] = true
		}
		genreSet := make(map[string]bool, len(primaryGenres))
		for _, g := range primaryGenres {
			genreSet[g.ID] = true
		}
		topicSet := make(map[string]bool, len(primaryTopics))
		for _, t := range primaryTopics {
			topicSet[t.ID] = true
		}

		for _, dup := range dupBooks {
			if (primaryBook.Description == nil || *primaryBook.Description == "") && dup.Description != nil && *dup.Description != "" {
				primaryBook.Description = dup.Description
			}
			if (primaryBook.Language == nil || *primaryBook.Language == "") && dup.Language != nil && *dup.Language != "" {
				primaryBook.Language = dup.Language
			}
			if (primaryBook.Publisher == nil || *primaryBook.Publisher == "") && dup.Publisher != nil && *dup.Publisher != "" {
				primaryBook.Publisher = dup.Publisher
			}
			if (primaryBook.Identifier == nil || *primaryBook.Identifier == "") && dup.Identifier != nil && *dup.Identifier != "" {
				primaryBook.Identifier = dup.Identifier
			}
			if primaryBook.PublishedDate == nil && dup.PublishedDate != nil {
				primaryBook.PublishedDate = dup.PublishedDate
			}
			if (primaryBook.CoverPath == nil || *primaryBook.CoverPath == "") && dup.CoverPath != nil && *dup.CoverPath != "" {
				primaryBook.CoverPath = dup.CoverPath
			}

			// Link missing authors
			dupAuthors, _ := h.repo.GetBookAuthors(ctx, dup.ID)
			for _, a := range dupAuthors {
				if !authorSet[a.ID] {
					_ = h.repo.LinkBookAuthor(ctx, primaryBook.ID, a.ID, "author")
					authorSet[a.ID] = true
				}
			}

			// Link missing genres
			dupGenres, _ := h.repo.GetBookGenres(ctx, dup.ID)
			for _, g := range dupGenres {
				if !genreSet[g.ID] {
					_ = h.repo.LinkBookGenre(ctx, primaryBook.ID, g.ID)
					genreSet[g.ID] = true
				}
			}

			// Link missing topics
			dupTopics, _ := h.repo.GetBookTopics(ctx, dup.ID)
			for _, t := range dupTopics {
				if !topicSet[t.ID] {
					_ = h.repo.LinkBookTopic(ctx, primaryBook.ID, t.ID)
					topicSet[t.ID] = true
				}
			}

			// Link missing series
			if len(primarySeries) == 0 {
				dupSeries, _ := h.repo.GetBookSeries(ctx, dup.ID)
				if len(dupSeries) > 0 {
					_ = h.repo.LinkBookSeries(ctx, primaryBook.ID, dupSeries[0].ID, dupSeries[0].SequenceNumber)
					primarySeries = dupSeries
				}
			}
		}

		if err := h.repo.UpdateBook(ctx, primaryBook); err != nil {
			h.logger.Warn("Failed to update primary book metadata during merge", "err", err)
		}
	}

	// Process each duplicate book
	for _, dup := range dupBooks {
		// Transfer bookmarks and highlights
		if transferBookmarks || transferHighlights {
			if err := h.repo.TransferBookmarksAndHighlights(ctx, dup.ID, primaryBook.ID); err != nil {
				h.logger.Warn("Failed to transfer bookmarks/highlights", "from", dup.ID, "to", primaryBook.ID, "err", err)
			}
		}

		// Delete duplicate file on disk if requested
		if deleteFiles {
			if dup.FilePath != "" && dup.FilePath != primaryBook.FilePath {
				if err := os.Remove(dup.FilePath); err != nil && !os.IsNotExist(err) {
					h.logger.Warn("Failed to delete duplicate book file", "path", dup.FilePath, "err", err)
				}
			}
			if dup.CoverPath != nil && *dup.CoverPath != "" {
				isDistinctCover := primaryBook.CoverPath == nil || *dup.CoverPath != *primaryBook.CoverPath
				if isDistinctCover {
					_ = os.Remove(*dup.CoverPath)
				}
			}
		}

		// Delete duplicate book from storage (cascades chapters, vectors, junction rows)
		if err := h.repo.DeleteBook(ctx, dup.ID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete duplicate book %q: %v", dup.ID, err))
			return
		}
	}

	_, _ = h.repo.PruneOrphanedGenres(ctx)

	if h.hub != nil {
		h.hub.Broadcast(events.Event{
			Type: events.EventBookUpdated,
			Data: map[string]any{
				"merged_into": primaryBook.ID,
			},
		})
	}

	updatedPrimary, err := h.repo.GetBookByID(ctx, primaryBook.ID)
	if err != nil {
		updatedPrimary = primaryBook
	}

	resultItem := BuildBookListItem(ctx, h.repo, updatedPrimary)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "merged",
		"book":   resultItem,
	})
}

// normalizeText strips punctuation, collapses whitespace, and lowercases text.
func normalizeText(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// normalizeIdentifier extracts alphanumeric characters and removes URN/ISBN prefixes.
func normalizeIdentifier(id *string) string {
	if id == nil {
		return ""
	}
	s := strings.ToLower(strings.TrimSpace(*id))
	s = strings.TrimPrefix(s, "urn:isbn:")
	s = strings.TrimPrefix(s, "urn:uuid:")
	s = strings.TrimPrefix(s, "isbn-13:")
	s = strings.TrimPrefix(s, "isbn-10:")
	s = strings.TrimPrefix(s, "isbn:")

	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var subtitlePatterns = regexp.MustCompile(`(?i)\b(a novel|special edition|expanded edition|collector'?s edition|deluxe edition|anniversary edition|revised edition|first edition|vol\s*\d+|volume\s*\d+|book\s*\d+|part\s*\d+)\b`)

// stripSubtitles strips common edition descriptors.
func stripSubtitles(s string) string {
	cleaned := subtitlePatterns.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(cleaned), " ")
}

// calculateSimilarity computes Jaccard word similarity between two strings.
func calculateSimilarity(s1, s2 string) float64 {
	w1 := strings.Fields(s1)
	w2 := strings.Fields(s2)
	if len(w1) == 0 || len(w2) == 0 {
		return 0
	}

	set1 := make(map[string]bool, len(w1))
	for _, w := range w1 {
		set1[w] = true
	}

	intersection := 0
	set2 := make(map[string]bool, len(w2))
	for _, w := range w2 {
		set2[w] = true
		if set1[w] {
			intersection++
		}
	}

	union := len(set1)
	for w := range set2 {
		if !set1[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
