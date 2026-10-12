package scanner

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var (
	curlyUUIDRegex   = regexp.MustCompile(`(?i)\{[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\}`)
	nonPathSafeRegex = regexp.MustCompile(`[^a-zA-Z0-9\-\s]+`)
)

// SanitizePathSegment normalizes strings for safe library paths matching audiobook-downloader:
// URL decoding, stripping curly brace UUIDs, converting & to and, stripping punctuation (including apostrophes),
// and title casing each word.
func SanitizePathSegment(input string) string {
	if unescaped, err := url.QueryUnescape(input); err == nil {
		input = unescaped
	}
	s := curlyUUIDRegex.ReplaceAllString(input, "")
	s = strings.ReplaceAll(s, "&", "and")
	s = nonPathSafeRegex.ReplaceAllString(s, "")
	words := strings.Fields(s)
	if len(words) == 0 {
		return "Unknown"
	}
	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	res := strings.Join(words, " ")
	if res == "" {
		return "Unknown"
	}
	return res
}

// SanitizeAuthorSegment extracts the primary author from comma-separated lists and sanitizes it.
func SanitizeAuthorSegment(input string) string {
	parts := strings.Split(input, ",")
	first := strings.TrimSpace(parts[0])
	first = strings.TrimRight(first, ".,")
	if first == "" {
		first = input
	}
	return SanitizePathSegment(first)
}

