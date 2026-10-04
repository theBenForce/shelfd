package repository

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// Matches initials like "J.R.R." or "J. R.R." or "George R.R."
	initialPattern = regexp.MustCompile(`\b([A-Z]\.)([A-Z])`)
	multipleSpaces = regexp.MustCompile(`\s+`)
)

// NormalizeAuthorName cleans and standardizes author names:
// - Standardizes initials ("J.R.R. Tolkien" -> "J. R. R. Tolkien")
// - Trims leading/trailing whitespace and surrounding quotes
// - Collapses multiple spaces
// - Handles "Last, First" format if unambiguous (e.g. "Tolkien, J. R. R." -> "J. R. R. Tolkien")
func NormalizeAuthorName(name string) string {
	s := strings.TrimSpace(name)
	s = strings.Trim(s, `"'`+"“”‘’")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Handle "Last, First Middle" format (e.g. "Tolkien, J. R. R." or "King, Stephen")
	// but skip if contains suffixes like "Jr.", "Sr.", "III", "PhD", etc.
	if parts := strings.Split(s, ","); len(parts) == 2 {
		first := strings.TrimSpace(parts[1])
		last := strings.TrimSpace(parts[0])
		lowerFirst := strings.ToLower(first)
		isSuffix := lowerFirst == "jr." || lowerFirst == "jr" || lowerFirst == "sr." || lowerFirst == "sr" ||
			lowerFirst == "ii" || lowerFirst == "iii" || lowerFirst == "iv" || lowerFirst == "phd" || lowerFirst == "md"
		if !isSuffix && first != "" && last != "" {
			s = first + " " + last
		}
	}

	// Insert space between adjacent initials (e.g., "J.R.R." -> "J. R. R.")
	for initialPattern.MatchString(s) {
		s = initialPattern.ReplaceAllString(s, "$1 $2")
	}

	// Collapse multiple spaces
	s = multipleSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// AuthorLookupKey creates a canonical key ignoring spaces, punctuation, and case
// e.g. "J. R. R. Tolkien" -> "jrrtolkien", "J.R.R. Tolkien" -> "jrrtolkien"
func AuthorLookupKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
