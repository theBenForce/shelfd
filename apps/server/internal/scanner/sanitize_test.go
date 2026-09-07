package scanner_test

import (
	"testing"

	"github.com/shelfd/shelfd/internal/scanner"
)

func TestSanitizePathSegment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean string untouched",
			input:    "Frank Herbert",
			expected: "Frank Herbert",
		},
		{
			name:     "strips illegal path characters",
			input:    `Science: Fiction / Fantasy * "Quotes" <Tags> | ? \ Slash`,
			expected: "Science Fiction Fantasy Quotes Tags Slash",
		},
		{
			name:     "collapses multiple whitespace",
			input:    "Ursula   K.   Le   Guin",
			expected: "Ursula K. Le Guin",
		},
		{
			name:     "trims leading trailing spaces and dots",
			input:    "  .Dune Chronicles.  ",
			expected: "Dune Chronicles",
		},
		{
			name:     "fallback for all illegal chars",
			input:    `/*?"<>|`,
			expected: "Unknown",
		},
		{
			name:     "fallback for empty string",
			input:    "   ",
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.SanitizePathSegment(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizePathSegment(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
