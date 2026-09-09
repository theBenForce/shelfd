package epub_test

import (
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/epub"
)

func TestChunkChapterParagraphs_Empty(t *testing.T) {
	chunks := epub.ChunkChapterParagraphs("", 200, 500)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestChunkChapterParagraphs_GroupingDialogue(t *testing.T) {
	text := `
"Are you ready?" she asked.

"As ready as I'll ever be," he replied, checking the chronometer on his wrist.

The hangar bay hissed as the pressure locks engaged. Outside, the dust of Mars blew against the reinforced viewport in reddish waves.
`
	chunks := epub.ChunkChapterParagraphs(text, 100, 300)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	c := chunks[0]
	if c.StartParagraph != 1 || c.EndParagraph != 3 {
		t.Errorf("expected paragraph range 1-3, got %d-%d", c.StartParagraph, c.EndParagraph)
	}
	if !strings.Contains(c.Content, "Are you ready?") || !strings.Contains(c.Content, "reinforced viewport") {
		t.Errorf("expected combined content in chunk, got %s", c.Content)
	}
}

func TestChunkChapterParagraphs_SplittingAtMaxChars(t *testing.T) {
	para1 := strings.Repeat("Alpha bravo charlie delta. ", 10) // ~270 chars
	para2 := strings.Repeat("Echo foxtrot golf hotel. ", 10)   // ~250 chars
	para3 := strings.Repeat("India juliet kilo lima. ", 10)    // ~240 chars

	text := strings.Join([]string{para1, para2, para3}, "\n\n")

	chunks := epub.ChunkChapterParagraphs(text, 200, 400)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	if chunks[0].StartParagraph != 1 {
		t.Errorf("expected chunk 0 to start at paragraph 1, got %d", chunks[0].StartParagraph)
	}
	lastChunk := chunks[len(chunks)-1]
	if lastChunk.EndParagraph != 3 {
		t.Errorf("expected last chunk to end at paragraph 3, got %d", lastChunk.EndParagraph)
	}
}

func TestChunkChapterParagraphs_HeadingSplitting(t *testing.T) {
	text := `
Paragraph one has enough text to satisfy the minimum character count of the chunker easily.

Paragraph two continues the discussion with even more detailed explanation.

# New Major Section

This is paragraph four under the new heading.
`
	chunks := epub.ChunkChapterParagraphs(text, 100, 800)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks split at heading, got %d", len(chunks))
	}

	if chunks[0].EndParagraph != 2 {
		t.Errorf("expected first chunk to end at paragraph 2, got %d", chunks[0].EndParagraph)
	}
	if chunks[1].StartParagraph != 3 {
		t.Errorf("expected second chunk to start at paragraph 3 (# New Major Section), got %d", chunks[1].StartParagraph)
	}
}
