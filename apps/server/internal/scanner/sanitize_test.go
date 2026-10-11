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
			name:     "strips apostrophes",
			input:    "Spidey's New Costume",
			expected: "Spideys New Costume",
		},
		{
			name:     "converts ampersand to and",
			input:    "Romeo & Juliet",
			expected: "Romeo And Juliet",
		},
		{
			name:     "strips curly brace UUID",
			input:    "{766B8E90-436D-4186-B3E4-1E9F86E5BA90}",
			expected: "Unknown",
		},
		{
			name:     "strips illegal path characters and dots",
			input:    `Science: Fiction / Fantasy * "Quotes" <Tags> | ? \ Slash`,
			expected: "Science Fiction Fantasy Quotes Tags Slash",
		},
		{
			name:     "collapses multiple whitespace and title cases",
			input:    "ursula   k.   le   guin",
			expected: "Ursula K Le Guin",
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

func TestSanitizeAuthorSegment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "takes primary author from comma-separated list",
			input:    "Marvel, Marvel Press",
			expected: "Marvel",
		},
		{
			name:     "strips apostrophes and title-cases",
			input:    "o'reilly media",
			expected: "Oreilly Media",
		},
		{
			name:     "handles single author",
			input:    "Andy Weir",
			expected: "Andy Weir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.SanitizeAuthorSegment(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeAuthorSegment(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

