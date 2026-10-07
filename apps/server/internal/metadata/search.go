package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SearchResult represents a normalized book metadata search result across external providers.
type SearchResult struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	Description    string   `json:"description,omitempty"`
	Publisher      string   `json:"publisher,omitempty"`
	PublishedYear  *int     `json:"published_year,omitempty"`
	ISBN           string   `json:"isbn,omitempty"`
	Language       string   `json:"language,omitempty"`
	Genres         []string `json:"genres,omitempty"`
	Series         string   `json:"series,omitempty"`
	SeriesSequence string   `json:"series_sequence,omitempty"`
	CoverURL       string   `json:"cover_url,omitempty"`
	Provider       string   `json:"provider"`
}

// SearchQuery specifies the parameters for querying metadata providers.
type SearchQuery struct {
	Query    string
	Title    string
	Author   string
	ISBN     string
	Provider string // "openlibrary", "googlebooks", "apple", or empty for all
}

// Service queries external metadata providers and retrieves remote cover images.
type Service struct {
	httpClient *http.Client
}

// NewService constructs a new metadata search Service.
func NewService(client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Service{httpClient: client}
}

// Search searches across supported providers according to the given query.
func (s *Service) Search(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	provider := strings.ToLower(strings.TrimSpace(q.Provider))

	var searchers []func(context.Context, SearchQuery) ([]SearchResult, error)
	switch provider {
	case "openlibrary":
		searchers = append(searchers, s.searchOpenLibrary)
	case "googlebooks", "google":
		searchers = append(searchers, s.searchGoogleBooks)
	case "apple", "itunes":
		searchers = append(searchers, s.searchITunes)
	default:
		// Run all three providers concurrently
		searchers = []func(context.Context, SearchQuery) ([]SearchResult, error){
			s.searchOpenLibrary,
			s.searchGoogleBooks,
			s.searchITunes,
		}
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []SearchResult
	)

	for _, searchFn := range searchers {
		wg.Add(1)
		go func(fn func(context.Context, SearchQuery) ([]SearchResult, error)) {
			defer wg.Done()
			res, err := fn(ctx, q)
			if err != nil {
				return
			}
			mu.Lock()
			results = append(results, res...)
			mu.Unlock()
		}(searchFn)
	}

	wg.Wait()

	return deduplicateResults(results), nil
}

// FetchCoverImage downloads an image from coverURL and returns the raw bytes and content type.
func (s *Service) FetchCoverImage(ctx context.Context, coverURL string) ([]byte, string, error) {
	if coverURL == "" {
		return nil, "", errors.New("empty cover URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("invalid request: %w", err)
	}
	req.Header.Set("User-Agent", "Shelfd/1.0 (Metadata Search)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("image request failed with status: %d", resp.StatusCode)
	}

	// Limit to max 10MB
	lr := io.LimitReader(resp.Body, 10<<20)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	// Normalize content type
	if strings.Contains(contentType, ";") {
		contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("unsupported content type: %s", contentType)
	}

	return data, contentType, nil
}

// ----------------- Open Library -----------------

type openLibraryDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	Publisher        []string `json:"publisher"`
	ISBN             []string `json:"isbn"`
	Language         []string `json:"language"`
	Subject          []string `json:"subject"`
	CoverI           int      `json:"cover_i"`
}

type openLibraryResponse struct {
	Docs []openLibraryDoc `json:"docs"`
}

func (s *Service) searchOpenLibrary(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	params := url.Values{}
	if q.Title != "" {
		params.Set("title", q.Title)
	}
	if q.Author != "" {
		params.Set("author", q.Author)
	}
	if q.ISBN != "" {
		params.Set("isbn", q.ISBN)
	}
	if q.Query != "" && q.Title == "" && q.Author == "" && q.ISBN == "" {
		params.Set("q", q.Query)
	} else if q.Query != "" && params.Get("q") == "" {
		params.Set("q", q.Query)
	}

	params.Set("limit", "15")

	reqURL := "https://openlibrary.org/search.json?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Shelfd/1.0 (Metadata Search)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openlibrary returned status %d", resp.StatusCode)
	}

	var olResp openLibraryResponse
	if err := json.NewDecoder(resp.Body).Decode(&olResp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(olResp.Docs))
	for _, doc := range olResp.Docs {
		if strings.TrimSpace(doc.Title) == "" {
			continue
		}

		author := ""
		if len(doc.AuthorName) > 0 {
			author = strings.Join(doc.AuthorName, ", ")
		}

		publisher := ""
		if len(doc.Publisher) > 0 {
			publisher = doc.Publisher[0]
		}

		isbn := ""
		if len(doc.ISBN) > 0 {
			isbn = doc.ISBN[0]
		}

		lang := ""
		if len(doc.Language) > 0 {
			lang = doc.Language[0]
		}

		var pubYear *int
		if doc.FirstPublishYear > 0 {
			yr := doc.FirstPublishYear
			pubYear = &yr
		}

		var coverURL string
		if doc.CoverI > 0 {
			coverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", doc.CoverI)
		}

		var genres []string
		for i, subj := range doc.Subject {
			if i >= 5 {
				break
			}
			subj = strings.TrimSpace(subj)
			if subj != "" {
				genres = append(genres, subj)
			}
		}

		title, series, seq := parseSeriesFromTitle(doc.Title)

		id := doc.Key
		if id == "" {
			id = fmt.Sprintf("ol_%s_%s", sanitizeID(title), sanitizeID(author))
		}

		results = append(results, SearchResult{
			ID:             id,
			Title:          title,
			Author:         author,
			Publisher:      publisher,
			PublishedYear:  pubYear,
			ISBN:           isbn,
			Language:       lang,
			Genres:         genres,
			Series:         series,
			SeriesSequence: seq,
			CoverURL:       coverURL,
			Provider:       "openlibrary",
		})
	}

	return results, nil
}

// ----------------- Apple Books (iTunes API) -----------------

type iTunesResult struct {
	TrackID          int64    `json:"trackId"`
	TrackName        string   `json:"trackName"`
	ArtistName       string   `json:"artistName"`
	Description      string   `json:"description"`
	ReleaseDate      string   `json:"releaseDate"`
	Genres           []string `json:"genres"`
	ArtworkUrl100    string   `json:"artworkUrl100"`
	ArtworkUrl60     string   `json:"artworkUrl60"`
	FormattedPrice   string   `json:"formattedPrice"`
}

type iTunesResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []iTunesResult `json:"results"`
}

func (s *Service) searchITunes(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	var terms []string
	if q.Title != "" {
		terms = append(terms, q.Title)
	}
	if q.Author != "" {
		terms = append(terms, q.Author)
	}
	if q.ISBN != "" {
		terms = append(terms, q.ISBN)
	}
	if len(terms) == 0 && q.Query != "" {
		terms = append(terms, q.Query)
	}
	if len(terms) == 0 {
		return nil, nil
	}

	params := url.Values{}
	params.Set("term", strings.Join(terms, " "))
	params.Set("entity", "ebook")
	params.Set("limit", "15")

	reqURL := "https://itunes.apple.com/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Shelfd/1.0 (Metadata Search)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple itunes returned status %d", resp.StatusCode)
	}

	var itResp iTunesResponse
	if err := json.NewDecoder(resp.Body).Decode(&itResp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(itResp.Results))
	for _, item := range itResp.Results {
		if strings.TrimSpace(item.TrackName) == "" {
			continue
		}

		desc := cleanHTML(item.Description)

		var pubYear *int
		if len(item.ReleaseDate) >= 4 {
			if yr, err := strconv.Atoi(item.ReleaseDate[:4]); err == nil && yr > 0 {
				pubYear = &yr
			}
		}

		coverURL := item.ArtworkUrl100
		if coverURL == "" {
			coverURL = item.ArtworkUrl60
		}
		if coverURL != "" {
			coverURL = strings.Replace(coverURL, "100x100bb", "600x600bb", 1)
			coverURL = strings.Replace(coverURL, "60x60bb", "600x600bb", 1)
		}

		title, series, seq := parseSeriesFromTitle(item.TrackName)

		results = append(results, SearchResult{
			ID:             strconv.FormatInt(item.TrackID, 10),
			Title:          title,
			Author:         item.ArtistName,
			Description:    desc,
			PublishedYear:  pubYear,
			Genres:         item.Genres,
			Series:         series,
			SeriesSequence: seq,
			CoverURL:       coverURL,
			Provider:       "apple",
		})
	}

	return results, nil
}

// ----------------- Google Books -----------------

type googleBooksItem struct {
	ID         string `json:"id"`
	VolumeInfo struct {
		Title               string   `json:"title"`
		Subtitle            string   `json:"subtitle"`
		Authors             []string `json:"authors"`
		Publisher           string   `json:"publisher"`
		PublishedDate       string   `json:"publishedDate"`
		Description         string   `json:"description"`
		Categories          []string `json:"categories"`
		Language            string   `json:"language"`
		IndustryIdentifiers []struct {
			Type       string `json:"type"`
			Identifier string `json:"identifier"`
		} `json:"industryIdentifiers"`
		ImageLinks struct {
			ExtraLarge string `json:"extraLarge"`
			Large      string `json:"large"`
			Medium     string `json:"medium"`
			Small      string `json:"small"`
			Thumbnail  string `json:"thumbnail"`
		} `json:"imageLinks"`
	} `json:"volumeInfo"`
}

type googleBooksResponse struct {
	TotalItems int               `json:"totalItems"`
	Items      []googleBooksItem `json:"items"`
}

func (s *Service) searchGoogleBooks(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	var queryParts []string
	if q.Title != "" {
		queryParts = append(queryParts, fmt.Sprintf("intitle:%s", q.Title))
	}
	if q.Author != "" {
		queryParts = append(queryParts, fmt.Sprintf("inauthor:%s", q.Author))
	}
	if q.ISBN != "" {
		queryParts = append(queryParts, fmt.Sprintf("isbn:%s", q.ISBN))
	}
	if len(queryParts) == 0 && q.Query != "" {
		queryParts = append(queryParts, q.Query)
	}
	if len(queryParts) == 0 {
		return nil, nil
	}

	params := url.Values{}
	params.Set("q", strings.Join(queryParts, " "))
	params.Set("maxResults", "15")

	reqURL := "https://www.googleapis.com/books/v1/volumes?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Shelfd/1.0 (Metadata Search)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google books returned status %d", resp.StatusCode)
	}

	var gbResp googleBooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&gbResp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(gbResp.Items))
	for _, item := range gbResp.Items {
		info := item.VolumeInfo
		if strings.TrimSpace(info.Title) == "" {
			continue
		}

		author := ""
		if len(info.Authors) > 0 {
			author = strings.Join(info.Authors, ", ")
		}

		var pubYear *int
		if len(info.PublishedDate) >= 4 {
			if yr, err := strconv.Atoi(info.PublishedDate[:4]); err == nil && yr > 0 {
				pubYear = &yr
			}
		}

		isbn := ""
		for _, id := range info.IndustryIdentifiers {
			if id.Type == "ISBN_13" {
				isbn = id.Identifier
				break
			}
			if id.Type == "ISBN_10" && isbn == "" {
				isbn = id.Identifier
			}
		}

		coverURL := info.ImageLinks.ExtraLarge
		if coverURL == "" {
			coverURL = info.ImageLinks.Large
		}
		if coverURL == "" {
			coverURL = info.ImageLinks.Medium
		}
		if coverURL == "" {
			coverURL = info.ImageLinks.Small
		}
		if coverURL == "" {
			coverURL = info.ImageLinks.Thumbnail
		}
		if coverURL != "" {
			if strings.HasPrefix(coverURL, "http://") {
				coverURL = "https://" + coverURL[7:]
			}
			coverURL = strings.Replace(coverURL, "&edge=curl", "", -1)
		}

		rawTitle := info.Title
		if info.Subtitle != "" {
			rawTitle = fmt.Sprintf("%s: %s", rawTitle, info.Subtitle)
		}
		title, series, seq := parseSeriesFromTitle(rawTitle)

		results = append(results, SearchResult{
			ID:             item.ID,
			Title:          title,
			Author:         author,
			Description:    cleanHTML(info.Description),
			Publisher:      info.Publisher,
			PublishedYear:  pubYear,
			ISBN:           isbn,
			Language:       info.Language,
			Genres:         info.Categories,
			Series:         series,
			SeriesSequence: seq,
			CoverURL:       coverURL,
			Provider:       "googlebooks",
		})
	}

	return results, nil
}

// ----------------- Helpers -----------------

var (
	htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
	seriesRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\s*\(([^)]+?)(?:,\s*|\s+)(?:Book|Vol\.?|Volume|#)\s*([0-9]+(?:\.[0-9]+)?)\)\s*$`),
		regexp.MustCompile(`(?i)\s*\(([^)]+?)\s+([0-9]+(?:\.[0-9]+)?)\)\s*$`),
		regexp.MustCompile(`(?i):\s*([^:]+?)(?:,\s*|\s+)(?:Book|Vol\.?|Volume|#)\s*([0-9]+(?:\.[0-9]+)?)\s*$`),
	}
)

func cleanHTML(s string) string {
	if s == "" {
		return ""
	}
	s = htmlTagRegex.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}

func parseSeriesFromTitle(rawTitle string) (cleanTitle, series, seq string) {
	cleanTitle = strings.TrimSpace(rawTitle)
	for _, re := range seriesRegexes {
		matches := re.FindStringSubmatch(cleanTitle)
		if len(matches) == 3 {
			series = strings.TrimSpace(matches[1])
			seq = strings.TrimSpace(matches[2])
			cleanTitle = strings.TrimSpace(cleanTitle[:len(cleanTitle)-len(matches[0])])
			return cleanTitle, series, seq
		}
	}
	return cleanTitle, "", ""
}

func sanitizeID(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	return strings.Trim(reg.ReplaceAllString(s, "_"), "_")
}

func deduplicateResults(items []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	var out []SearchResult
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Title)) + "||" + strings.ToLower(strings.TrimSpace(item.Author))
		if seen[key] && item.CoverURL == "" {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
