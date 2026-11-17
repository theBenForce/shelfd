package repository_test

import (
	"context"
	"testing"

	"github.com/shelfd/shelfd/internal/repository"
)

func TestNormalizeReadalongMatchText(t *testing.T) {
	cases := []struct {
		a    string
		b    string
		same bool
	}{
		{"Four Silly Skeletons!", "  four   silly skeletons ", true},
		{"Ben & Holly's Little Kingdom", "Ben and Hollys: Little Kingdom?", true},
		{"Mark &amp; Sarah", "Mark and Sarah", true},
		{"The Cat in the Hat", "The Cat on the Mat", false},
	}

	for _, tc := range cases {
		na := repository.NormalizeReadalongMatchText(tc.a)
		nb := repository.NormalizeReadalongMatchText(tc.b)
		if (na == nb) != tc.same {
			t.Errorf("NormalizeReadalongMatchText(%q)=%q vs (%q)=%q, want same=%v", tc.a, na, tc.b, nb, tc.same)
		}
	}
}

func TestMatchBookForReadalong_TitleAuthorAndFallback(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	// Seed two ebooks with the same normalized title ("The Awakening") by different authors,
	// plus one unique ebook ("Four Silly Skeletons") with "Unknown" author.
	b1 := &repository.Book{
		ID:       "b-chopin",
		Title:    "The Awakening!",
		FilePath: "Kate Chopin/The Awakening/The Awakening.epub",
		BookType: "ebook",
	}
	b2 := &repository.Book{
		ID:       "b-kelley",
		Title:    "The Awakening",
		FilePath: "Kelley Armstrong/The Awakening/The Awakening.epub",
		BookType: "ebook",
	}
	b3 := &repository.Book{
		ID:       "b-skeletons",
		Title:    "Four Silly Skeletons: A Read-Along",
		FilePath: "Unknown/Four Silly Skeletons/Four Silly Skeletons.epub",
		BookType: "ebook",
	}
	for _, b := range []*repository.Book{b1, b2, b3} {
		if err := repo.CreateBook(ctx, b); err != nil {
			t.Fatalf("CreateBook %s: %v", b.ID, err)
		}
	}

	a1, _ := repo.UpsertAuthor(ctx, "Kate Chopin")
	_ = repo.LinkBookAuthor(ctx, b1.ID, a1.ID, "Author")

	a2, _ := repo.UpsertAuthor(ctx, "Kelley & Co. Armstrong")
	_ = repo.LinkBookAuthor(ctx, b2.ID, a2.ID, "Author")

	a3, _ := repo.UpsertAuthor(ctx, "Unknown")
	_ = repo.LinkBookAuthor(ctx, b3.ID, a3.ID, "Author")

	// 1. Exact title + author disambiguates between two books with the same title (& <-> "and", punctuation-insensitive)
	m, err := repository.MatchBookForReadalong(ctx, repo, "the awakening", []string{"Kelley and Co Armstrong"})
	if err != nil {
		t.Fatalf("MatchBookForReadalong failed: %v", err)
	}
	if m == nil || m.ID != "b-kelley" {
		t.Fatalf("expected match b-kelley, got %+v", m)
	}

	// 2. Ambiguous title with no author match -> returns nil (since 2 candidates have title "The Awakening")
	mAmb, err := repository.MatchBookForReadalong(ctx, repo, "The Awakening", []string{"Unknown"})
	if err != nil {
		t.Fatalf("MatchBookForReadalong failed: %v", err)
	}
	if mAmb != nil {
		t.Fatalf("expected nil on ambiguous title-only match, got %+v", mAmb)
	}

	// 3. Unique title fallback when EPUB had "Unknown" author and M4B has @ART = "Mark Sperring"
	mFallback, err := repository.MatchBookForReadalong(ctx, repo, "Four Silly Skeletons - A Read Along!", []string{"Mark Sperring"})
	if err != nil {
		t.Fatalf("MatchBookForReadalong failed: %v", err)
	}
	if mFallback == nil || mFallback.ID != "b-skeletons" {
		t.Fatalf("expected title-only fallback to pick b-skeletons, got %+v", mFallback)
	}

	// 4. Non-existent book -> returns nil
	mNone, err := repository.MatchBookForReadalong(ctx, repo, "Completely Unrelated Book", []string{"Mark Sperring"})
	if err != nil {
		t.Fatalf("MatchBookForReadalong failed: %v", err)
	}
	if mNone != nil {
		t.Fatalf("expected nil for non-matching book, got %+v", mNone)
	}
}
