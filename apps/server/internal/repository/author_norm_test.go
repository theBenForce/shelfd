package repository

import "testing"

func TestNormalizeAuthorName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"J.R.R. Tolkien", "J. R. R. Tolkien"},
		{"J. R. R. Tolkien", "J. R. R. Tolkien"},
		{"J.  R.  R.  Tolkien", "J. R. R. Tolkien"},
		{"Tolkien, J.R.R.", "J. R. R. Tolkien"},
		{"Tolkien, J. R. R.", "J. R. R. Tolkien"},
		{"George R.R. Martin", "George R. R. Martin"},
		{"George R. R. Martin", "George R. R. Martin"},
		{"C.S. Lewis", "C. S. Lewis"},
		{"H.G. Wells", "H. G. Wells"},
		{"Philip K. Dick", "Philip K. Dick"},
		{"Martin Luther King, Jr.", "Martin Luther King, Jr."},
		{"\"Andy Serkis\"", "Andy Serkis"},
		{"  Arthur C. Clarke  ", "Arthur C. Clarke"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeAuthorName(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeAuthorName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAuthorLookupKey(t *testing.T) {
	k1 := AuthorLookupKey("J.R.R. Tolkien")
	k2 := AuthorLookupKey("J. R. R. Tolkien")
	k3 := AuthorLookupKey("  j. r. r. tolkien  ")

	if k1 != "jrrtolkien" {
		t.Errorf("expected 'jrrtolkien', got %q", k1)
	}
	if k1 != k2 || k2 != k3 {
		t.Errorf("lookup keys should match: k1=%q, k2=%q, k3=%q", k1, k2, k3)
	}
}
