package epub

import (
	"fmt"
	"html"
	"path"
	"regexp"
	"strings"
)

var (
	scriptStyleRegex = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	titleTagRegex    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	headerTagRegex   = regexp.MustCompile(`(?is)<h[1-3][^>]*>(.*?)</h[1-3]>`)
	blockTagsRegex   = regexp.MustCompile(`(?i)<(?:p|div|h[1-6]|li|tr|br|blockquote)[^>]*>`)
	allTagsRegex     = regexp.MustCompile(`(?s)<[^>]+>`)
	multiSpaceRegex  = regexp.MustCompile(`[ \t\r]+`)
	multiNLRegex     = regexp.MustCompile(`\n{3,}`)
)

// ExtractChapters walks spine itemrefs in order and converts XHTML content into clean plaintext.
func (r *Reader) ExtractChapters() ([]*ParsedChapter, error) {
	if r.opf == nil {
		return nil, fmt.Errorf("no opf package loaded")
	}

	manifestMap := make(map[string]opfItem)
	for _, item := range r.opf.Manifest.Items {
		manifestMap[item.ID] = item
	}

	var chapters []*ParsedChapter
	chapterIndex := 0

	for _, itemref := range r.opf.Spine.Itemrefs {
		item, exists := manifestMap[itemref.IDRef]
		if !exists {
			continue
		}

		// Only process XHTML/HTML content documents
		mediaType := strings.ToLower(item.MediaType)
		if !strings.Contains(mediaType, "html") && !strings.Contains(mediaType, "xhtml") && !strings.Contains(mediaType, "xml") {
			continue
		}

		fullPath := item.Href
		if r.rootBase != "" {
			fullPath = path.Join(r.rootBase, item.Href)
		}

		zf := r.findFile(fullPath)
		if zf == nil {
			continue
		}

		rawHTML, err := readZipFile(zf)
		if err != nil {
			return nil, fmt.Errorf("reading chapter %s: %w", fullPath, err)
		}

		title, plaintext := extractTextFromHTML(string(rawHTML))
		if strings.TrimSpace(plaintext) == "" {
			continue
		}

		chapterIndex++
		chapters = append(chapters, &ParsedChapter{
			Index:        chapterIndex,
			Title:        title,
			ContentPlain: plaintext,
		})
	}

	return chapters, nil
}

func extractTextFromHTML(raw string) (*string, string) {
	// Try extracting title from <title> or <h1>-<h3>
	var title *string
	if m := titleTagRegex.FindStringSubmatch(raw); len(m) > 1 {
		clean := cleanSnippet(m[1])
		if clean != "" {
			title = &clean
		}
	}
	if title == nil {
		if m := headerTagRegex.FindStringSubmatch(raw); len(m) > 1 {
			clean := cleanSnippet(m[1])
			if clean != "" {
				title = &clean
			}
		}
	}

	// 1. Remove <script> and <style>
	s := scriptStyleRegex.ReplaceAllString(raw, "")

	// 2. Insert paragraph/block newlines
	s = blockTagsRegex.ReplaceAllString(s, "\n\n")

	// 3. Strip all remaining HTML tags
	s = allTagsRegex.ReplaceAllString(s, "")

	// 4. Decode HTML entities
	s = html.UnescapeString(s)

	// 5. Clean whitespace and linebreaks
	lines := strings.Split(s, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(multiSpaceRegex.ReplaceAllString(line, " "))
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		} else if len(cleanedLines) > 0 && cleanedLines[len(cleanedLines)-1] != "" {
			cleanedLines = append(cleanedLines, "")
		}
	}

	result := strings.Join(cleanedLines, "\n\n")
	result = multiNLRegex.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return title, result
}

func cleanSnippet(raw string) string {
	s := allTagsRegex.ReplaceAllString(raw, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(multiSpaceRegex.ReplaceAllString(s, " "))
}
