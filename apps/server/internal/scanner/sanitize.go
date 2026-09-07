package scanner

import (
	"regexp"
	"strings"
)

var (
	illegalCharsRegex = regexp.MustCompile(`[/\\:*?"<>|]+`)
	whitespaceRegex   = regexp.MustCompile(`\s+`)
)

// SanitizePathSegment strips illegal filesystem characters, normalizes whitespace,
// and trims leading/trailing dots and spaces to safely create library directory and file names.
func SanitizePathSegment(input string) string {
	s := illegalCharsRegex.ReplaceAllString(input, " ")
	s = whitespaceRegex.ReplaceAllString(s, " ")
	s = strings.Trim(s, " .")
	if s == "" {
		return "Unknown"
	}
	return s
}
