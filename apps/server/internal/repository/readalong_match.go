package repository

import (
	"context"
	"html"
	"strings"
	"unicode"
)

// NormalizeReadalongMatchText normalizes a title or author string for read-along shelf matching:
// - Unescapes HTML entities (e.g. &amp; -> &)
// - Case-insensitive (lowercased)
// - Converts "&" <-> "and"
// - Strips punctuation (retains letters and digits)
// - Collapses whitespace to single spaces
func NormalizeReadalongMatchText(s string) string {
	s = html.UnescapeString(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "&", " and ")

	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else if r == '\'' || r == '\u2019' || r == '\u2018' || r == '`' {
			// Omit apostrophes so "Holly's" and "Hollys" normalize identically
			continue
		} else {
			sb.WriteByte(' ')
		}
	}

	return strings.Join(strings.Fields(sb.String()), " ")
}

func normalizeAuthorCandidates(authors []string) []string {
	var out []string
	seen := make(map[string]bool)
	add := func(norm string) {
		if norm == "" || norm == "unknown" || seen[norm] {
			return
		}
		seen[norm] = true
		out = append(out, norm)
	}

	var nonUnknownRaw []string
	for _, a := range authors {
		for _, part := range strings.FieldsFunc(a, func(r rune) bool {
			return r == ',' || r == ';'
		}) {
			clean := strings.TrimSpace(part)
			if clean == "" || strings.EqualFold(clean, "unknown") {
				continue
			}
			nonUnknownRaw = append(nonUnknownRaw, clean)
			add(NormalizeReadalongMatchText(clean))
			add(AuthorLookupKey(clean))
		}
	}
	if len(nonUnknownRaw) > 1 {
		add(NormalizeReadalongMatchText(strings.Join(nonUnknownRaw, " and ")))
	}
	return out
}

// MatchBookForReadalong searches existing shelf items (ebooks) for a read-along match:
// 1. Tries normalized title + author match first.
// 2. Falls back to normalized title-only match when exactly one ebook candidate normalizes to an exact title match.
func MatchBookForReadalong(ctx context.Context, repo StorageEngine, title string, authors []string) (*Book, error) {
	normTitle := NormalizeReadalongMatchText(title)
	if normTitle == "" || normTitle == "untitled" {
		return nil, nil
	}

	books, err := repo.ListBooks(ctx, BookFilter{Limit: 1000})
	if err != nil {
		return nil, err
	}

	var titleMatches []*Book
	for _, b := range books {
		if b == nil {
			continue
		}
		// Match against primary ebook shelf items (keep ebook primary, ignore standalone audiobooks)
		if b.BookType != "" && b.BookType != "ebook" {
			continue
		}
		if NormalizeReadalongMatchText(b.Title) == normTitle {
			titleMatches = append(titleMatches, b)
		}
	}

	if len(titleMatches) == 0 {
		return nil, nil
	}

	inputAuthorKeys := normalizeAuthorCandidates(authors)
	if len(inputAuthorKeys) > 0 {
		inputSet := make(map[string]bool, len(inputAuthorKeys))
		for _, k := range inputAuthorKeys {
			inputSet[k] = true
		}

		for _, candidate := range titleMatches {
			bookAuthors, err := repo.GetBookAuthors(ctx, candidate.ID)
			if err != nil || len(bookAuthors) == 0 {
				continue
			}
			rawNames := make([]string, 0, len(bookAuthors))
			for _, ba := range bookAuthors {
				if ba != nil && strings.TrimSpace(ba.Name) != "" {
					rawNames = append(rawNames, ba.Name)
				}
			}
			candKeys := normalizeAuthorCandidates(rawNames)
			for _, ck := range candKeys {
				if inputSet[ck] {
					return candidate, nil
				}
			}
		}
	}

	// Fallback: title-only match when exactly one ebook candidate normalizes to an exact title match
	if len(titleMatches) == 1 {
		return titleMatches[0], nil
	}

	return nil, nil
}
