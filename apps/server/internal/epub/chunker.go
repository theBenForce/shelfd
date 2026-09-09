package epub

import (
	"strings"
)

// ParagraphChunk represents a consolidated range of paragraphs within a chapter.
type ParagraphChunk struct {
	StartParagraph int
	EndParagraph   int
	Content        string
}

const (
	DefaultMinChunkChars = 400
	DefaultMaxChunkChars = 1200
)

// ChunkChapterParagraphs groups raw chapter paragraphs into semantic chunks.
// Paragraph numbering is 1-based and matches the paragraph splitting in read_chapter_content.
func ChunkChapterParagraphs(contentPlain string, minChars, maxChars int) []ParagraphChunk {
	if minChars <= 0 {
		minChars = DefaultMinChunkChars
	}
	if maxChars <= 0 || maxChars < minChars {
		maxChars = DefaultMaxChunkChars
	}

	normalized := strings.ReplaceAll(contentPlain, "\r\n", "\n")
	rawParas := strings.Split(normalized, "\n\n")

	type indexedPara struct {
		index int
		text  string
	}

	var paras []indexedPara
	for i, p := range rawParas {
		clean := strings.TrimSpace(p)
		if clean != "" {
			paras = append(paras, indexedPara{
				index: i + 1, // 1-based index
				text:  clean,
			})
		}
	}

	if len(paras) == 0 {
		return nil
	}

	var chunks []ParagraphChunk
	var curTexts []string
	startIdx := paras[0].index
	endIdx := paras[0].index
	curLen := 0

	flush := func() {
		if len(curTexts) > 0 {
			chunks = append(chunks, ParagraphChunk{
				StartParagraph: startIdx,
				EndParagraph:   endIdx,
				Content:        strings.Join(curTexts, "\n\n"),
			})
			curTexts = nil
			curLen = 0
		}
	}

	for _, p := range paras {
		pLen := len(p.text)
		isHeading := strings.HasPrefix(p.text, "# ") || strings.HasPrefix(p.text, "## ") || strings.HasPrefix(p.text, "### ")

		// If current buffer is non-empty and adding this paragraph would exceed maxChars,
		// or if we encounter a top-level heading and already have reached minChars, flush buffer.
		if len(curTexts) > 0 {
			if (curLen+pLen > maxChars && curLen >= minChars) || (isHeading && curLen >= minChars) {
				flush()
			}
		}

		if len(curTexts) == 0 {
			startIdx = p.index
		}
		curTexts = append(curTexts, p.text)
		endIdx = p.index
		curLen += pLen + 2 // account for \n\n separator

		// If a single paragraph is already massive, flush it immediately as its own chunk
		if curLen >= maxChars {
			flush()
		}
	}

	flush()
	return chunks
}
