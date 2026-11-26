package study

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func BuildMarkdownVaultZip(bookTitle string, concepts []StudyConcept, cards []StudyCard) ([]byte, error) {
	bookTitle = strings.TrimSpace(bookTitle)
	if bookTitle == "" {
		bookTitle = "Study Vault"
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	type indexedConcept struct {
		concept StudyConcept
		slug    string
	}

	usedSlugs := make(map[string]int, len(concepts))
	titleToSlug := make(map[string]string, len(concepts))
	indexed := make([]indexedConcept, 0, len(concepts))

	for i, c := range concepts {
		base := slugify(c.Title)
		if base == "" {
			base = fmt.Sprintf("concept-%d", i+1)
		}
		slug := base
		if count, exists := usedSlugs[base]; exists {
			slug = fmt.Sprintf("%s-%d", base, count+1)
		}
		usedSlugs[base]++
		titleToSlug[strings.ToLower(strings.TrimSpace(c.Title))] = slug
		indexed = append(indexed, indexedConcept{concept: c, slug: slug})
	}

	chapterSet := make(map[int]struct{})
	conceptsByChapter := make(map[int][]indexedConcept)
	cardsByChapter := make(map[int][]StudyCard)

	for _, ic := range indexed {
		chapterSet[ic.concept.ChapterIndex] = struct{}{}
		conceptsByChapter[ic.concept.ChapterIndex] = append(conceptsByChapter[ic.concept.ChapterIndex], ic)
	}
	for _, card := range cards {
		chapterSet[card.ChapterIndex] = struct{}{}
		cardsByChapter[card.ChapterIndex] = append(cardsByChapter[card.ChapterIndex], card)
	}

	chapters := make([]int, 0, len(chapterSet))
	for ch := range chapterSet {
		chapters = append(chapters, ch)
	}
	sort.Ints(chapters)

	var readme strings.Builder
	fmt.Fprintf(&readme, "# %s — Study Vault\n\n", bookTitle)
	fmt.Fprintf(&readme, "- Concepts: %d\n", len(indexed))
	fmt.Fprintf(&readme, "- Study Cards: %d\n", len(cards))
	fmt.Fprintf(&readme, "- Chapters Covered: %d\n\n", len(chapters))

	if len(chapters) > 0 {
		readme.WriteString("## Chapters\n\n")
		for _, ch := range chapters {
			fmt.Fprintf(&readme, "- [[%s|Chapter %d]]\n", chapterNoteName(ch), ch)
		}
		readme.WriteString("\n")
	}

	if len(indexed) > 0 {
		readme.WriteString("## Key Concepts\n\n")
		for _, ic := range indexed {
			fmt.Fprintf(&readme, "- [[concepts/%s|%s]] (Chapter %d, ¶%d–%d)\n",
				ic.slug, ic.concept.Title, ic.concept.ChapterIndex, ic.concept.StartParagraph, ic.concept.EndParagraph)
		}
	}

	if err := writeZipFile(zw, "README.md", readme.String()); err != nil {
		return nil, err
	}

	for _, ic := range indexed {
		c := ic.concept
		var sb strings.Builder
		sb.WriteString("---\n")
		fmt.Fprintf(&sb, "title: %q\n", c.Title)
		fmt.Fprintf(&sb, "book_id: %q\n", c.BookID)
		fmt.Fprintf(&sb, "chapter_index: %d\n", c.ChapterIndex)
		fmt.Fprintf(&sb, "start_paragraph: %d\n", c.StartParagraph)
		fmt.Fprintf(&sb, "end_paragraph: %d\n", c.EndParagraph)
		sb.WriteString("---\n\n")

		fmt.Fprintf(&sb, "# %s\n\n", c.Title)
		fmt.Fprintf(&sb, "%s\n\n", c.Summary)
		fmt.Fprintf(&sb, "Source: [[%s|Chapter %d]] (¶%d–%d)\n", chapterNoteName(c.ChapterIndex), c.ChapterIndex, c.StartParagraph, c.EndParagraph)

		if len(c.RelatedConcepts) > 0 {
			sb.WriteString("\n## Related Concepts\n\n")
			for _, rel := range c.RelatedConcepts {
				rel = strings.TrimSpace(rel)
				if rel == "" {
					continue
				}
				if targetSlug, ok := titleToSlug[strings.ToLower(rel)]; ok {
					fmt.Fprintf(&sb, "- [[concepts/%s|%s]]\n", targetSlug, rel)
				} else {
					fmt.Fprintf(&sb, "- [[%s]]\n", rel)
				}
			}
		}

		if err := writeZipFile(zw, fmt.Sprintf("concepts/%s.md", ic.slug), sb.String()); err != nil {
			return nil, err
		}
	}

	for _, ch := range chapters {
		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s — Chapter %d\n\n", bookTitle, ch)
		sb.WriteString("Back to [[README]]\n\n")

		chConcepts := conceptsByChapter[ch]
		if len(chConcepts) > 0 {
			sb.WriteString("## Concepts\n\n")
			for _, ic := range chConcepts {
				fmt.Fprintf(&sb, "- [[concepts/%s|%s]] (¶%d–%d): %s\n",
					ic.slug, ic.concept.Title, ic.concept.StartParagraph, ic.concept.EndParagraph, ic.concept.Summary)
			}
			sb.WriteString("\n")
		}

		chCards := cardsByChapter[ch]
		if len(chCards) > 0 {
			sb.WriteString("## Flashcards\n\n")
			for i, card := range chCards {
				fmt.Fprintf(&sb, "### Card %d (¶%d–%d)\n\n", i+1, card.StartParagraph, card.EndParagraph)
				fmt.Fprintf(&sb, "**Q:** %s\n\n", card.Front)
				fmt.Fprintf(&sb, "**A:** %s\n\n", card.Back)
			}
		}

		if err := writeZipFile(zw, chapterNoteName(ch)+".md", sb.String()); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close markdown vault zip: %w", err)
	}
	return buf.Bytes(), nil
}

func chapterNoteName(chapterIndex int) string {
	return fmt.Sprintf("chapters/chapter-%02d", chapterIndex)
}

func writeZipFile(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", name, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		return fmt.Errorf("write zip entry %s: %w", name, err)
	}
	return nil
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 64 {
		out = strings.TrimRight(out[:64], "-")
	}
	return out
}
