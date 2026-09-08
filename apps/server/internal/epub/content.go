package epub

import (
	"bytes"
	"fmt"
	"html"
	"path"
	"regexp"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	whitespaceRegex = regexp.MustCompile(`[ \t\r\n]+`)
	multiNLRegex    = regexp.MustCompile(`\n{3,}`)
)

// ExtractChapters walks spine itemrefs in order and converts XHTML content into clean plaintext/markdown.
func (r *Reader) ExtractChapters() ([]*ParsedChapter, error) {
	if r.opf == nil {
		return nil, fmt.Errorf("no opf package loaded")
	}

	manifestMap := make(map[string]opfItem)
	for _, item := range r.opf.Manifest.Items {
		manifestMap[item.ID] = item
	}

	var chapters []*ParsedChapter
	chapterIndex := 0

	for _, itemref := range r.opf.Spine.Itemrefs {
		item, exists := manifestMap[itemref.IDRef]
		if !exists {
			continue
		}

		// Only process XHTML/HTML content documents
		mediaType := strings.ToLower(item.MediaType)
		if !strings.Contains(mediaType, "html") && !strings.Contains(mediaType, "xhtml") && !strings.Contains(mediaType, "xml") {
			continue
		}

		fullPath := item.Href
		if r.rootBase != "" {
			fullPath = path.Join(r.rootBase, item.Href)
		}

		zf := r.findFile(fullPath)
		if zf == nil {
			continue
		}

		rawHTML, err := readZipFile(zf)
		if err != nil {
			return nil, fmt.Errorf("reading chapter %s: %w", fullPath, err)
		}

		title, plaintext := extractTextFromHTML(string(rawHTML))
		if strings.TrimSpace(plaintext) == "" {
			continue
		}

		chapterIndex++
		chapters = append(chapters, &ParsedChapter{
			Index:        chapterIndex,
			Title:        title,
			ContentPlain: plaintext,
		})
	}

	return chapters, nil
}

func extractTextFromHTML(raw string) (*string, string) {
	doc, err := nethtml.Parse(strings.NewReader(raw))
	if err != nil {
		// Fallback to basic string extraction on malformed HTML
		return nil, strings.TrimSpace(raw)
	}

	title := extractChapterTitle(doc)
	var blocks []string
	collectBlocks(doc, &blocks, false)

	result := strings.Join(blocks, "\n\n")
	result = multiNLRegex.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return title, result
}

func extractChapterTitle(doc *nethtml.Node) *string {
	var h1Title, h2Title, h3Title, docTitle string

	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode {
			switch n.DataAtom {
			case atom.Title:
				if docTitle == "" {
					docTitle = cleanInlineText(collectNodeText(n))
				}
			case atom.H1:
				if h1Title == "" {
					h1Title = cleanInlineText(collectNodeText(n))
				}
			case atom.H2:
				if h2Title == "" {
					h2Title = cleanInlineText(collectNodeText(n))
				}
			case atom.H3:
				if h3Title == "" {
					h3Title = cleanInlineText(collectNodeText(n))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if h1Title != "" {
		return &h1Title
	}
	if h2Title != "" {
		return &h2Title
	}
	if h3Title != "" {
		return &h3Title
	}
	if docTitle != "" {
		return &docTitle
	}
	return nil
}

func collectBlocks(n *nethtml.Node, blocks *[]string, inBlockquote bool) {
	if n == nil {
		return
	}

	if n.Type == nethtml.ElementNode {
		switch n.DataAtom {
		case atom.Head, atom.Script, atom.Style, atom.Svg, atom.Img:
			return
		case atom.Hr:
			*blocks = append(*blocks, "---")
			return
		case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
			prefix := "# "
			switch n.DataAtom {
			case atom.H2:
				prefix = "## "
			case atom.H3:
				prefix = "### "
			case atom.H4, atom.H5, atom.H6:
				prefix = "#### "
			}
			inline := extractInlineMarkdown(n)
			cleaned := cleanInlineText(inline)
			if cleaned != "" {
				*blocks = append(*blocks, prefix+cleaned)
			}
			return
		case atom.Blockquote:
			var quoteBlocks []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				collectBlocks(c, &quoteBlocks, true)
			}
			if len(quoteBlocks) == 0 {
				inline := extractInlineMarkdown(n)
				cleaned := cleanInlineText(inline)
				if cleaned != "" {
					quoteBlocks = append(quoteBlocks, cleaned)
				}
			}
			for _, qb := range quoteBlocks {
				lines := strings.Split(qb, "\n")
				for i, l := range lines {
					lines[i] = "> " + l
				}
				*blocks = append(*blocks, strings.Join(lines, "\n"))
			}
			return
		case atom.P, atom.Li:
			inline := extractInlineMarkdown(n)
			cleaned := cleanInlineText(inline)
			if cleaned != "" {
				if n.DataAtom == atom.Li {
					cleaned = "- " + cleaned
				}
				*blocks = append(*blocks, cleaned)
			}
			return
		}

		// Container elements: div, section, article, etc.
		if isContainerTag(n.DataAtom) {
			if hasBlockDescendant(n) {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					collectBlocks(c, blocks, inBlockquote)
				}
				return
			}
			// Leaf container with no block children: treat as paragraph
			inline := extractInlineMarkdown(n)
			cleaned := cleanInlineText(inline)
			if cleaned != "" {
				*blocks = append(*blocks, cleaned)
			}
			return
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectBlocks(c, blocks, inBlockquote)
	}
}

func hasBlockDescendant(n *nethtml.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == nethtml.ElementNode {
			if isBlockTag(c.DataAtom) || hasBlockDescendant(c) {
				return true
			}
		}
	}
	return false
}

func isBlockTag(a atom.Atom) bool {
	switch a {
	case atom.P, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Blockquote, atom.Hr, atom.Ul, atom.Ol, atom.Li, atom.Pre:
		return true
	}
	return false
}

func isContainerTag(a atom.Atom) bool {
	switch a {
	case atom.Div, atom.Section, atom.Article, atom.Main, atom.Header, atom.Footer, atom.Body:
		return true
	}
	return false
}

func extractInlineMarkdown(n *nethtml.Node) string {
	var buf bytes.Buffer

	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil {
			return
		}

		switch node.Type {
		case nethtml.TextNode:
			text := html.UnescapeString(node.Data)
			// Collapse internal whitespace (including newlines) to single space
			collapsed := whitespaceRegex.ReplaceAllString(text, " ")
			buf.WriteString(collapsed)
		case nethtml.ElementNode:
			switch node.DataAtom {
			case atom.Head, atom.Script, atom.Style, atom.Svg:
				return
			case atom.Br:
				buf.WriteString("\n")
				return
			case atom.Em, atom.I:
				inner := collectInlineChildren(node)
				trimmed := strings.TrimSpace(inner)
				if trimmed != "" {
					if strings.HasPrefix(inner, " ") {
						buf.WriteString(" ")
					}
					buf.WriteString("*" + trimmed + "*")
					if strings.HasSuffix(inner, " ") {
						buf.WriteString(" ")
					}
				}
				return
			case atom.Strong, atom.B:
				inner := collectInlineChildren(node)
				trimmed := strings.TrimSpace(inner)
				if trimmed != "" {
					if strings.HasPrefix(inner, " ") {
						buf.WriteString(" ")
					}
					buf.WriteString("**" + trimmed + "**")
					if strings.HasSuffix(inner, " ") {
						buf.WriteString(" ")
					}
				}
				return
			case atom.Sup:
				inner := strings.TrimSpace(collectInlineChildren(node))
				if inner != "" {
					if !strings.HasPrefix(inner, "[") {
						buf.WriteString("[" + inner + "]")
					} else {
						buf.WriteString(inner)
					}
				}
				return
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}

	return buf.String()
}

func collectInlineChildren(n *nethtml.Node) string {
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		buf.WriteString(extractInlineMarkdownNode(c))
	}
	return buf.String()
}

func extractInlineMarkdownNode(node *nethtml.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == nethtml.TextNode {
		text := html.UnescapeString(node.Data)
		return whitespaceRegex.ReplaceAllString(text, " ")
	}
	if node.Type == nethtml.ElementNode {
		switch node.DataAtom {
		case atom.Head, atom.Script, atom.Style, atom.Svg:
			return ""
		case atom.Br:
			return "\n"
		case atom.Em, atom.I:
			inner := collectInlineChildren(node)
			trimmed := strings.TrimSpace(inner)
			if trimmed == "" {
				return ""
			}
			res := "*" + trimmed + "*"
			if strings.HasPrefix(inner, " ") {
				res = " " + res
			}
			if strings.HasSuffix(inner, " ") {
				res = res + " "
			}
			return res
		case atom.Strong, atom.B:
			inner := collectInlineChildren(node)
			trimmed := strings.TrimSpace(inner)
			if trimmed == "" {
				return ""
			}
			res := "**" + trimmed + "**"
			if strings.HasPrefix(inner, " ") {
				res = " " + res
			}
			if strings.HasSuffix(inner, " ") {
				res = res + " "
			}
			return res
		case atom.Sup:
			inner := strings.TrimSpace(collectInlineChildren(node))
			if inner == "" {
				return ""
			}
			if !strings.HasPrefix(inner, "[") {
				return "[" + inner + "]"
			}
			return inner
		}
	}

	var buf bytes.Buffer
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		buf.WriteString(extractInlineMarkdownNode(c))
	}
	return buf.String()
}

func collectNodeText(n *nethtml.Node) string {
	var buf bytes.Buffer
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			buf.WriteString(html.UnescapeString(node.Data))
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}

func cleanInlineText(raw string) string {
	trimmed := strings.TrimSpace(whitespaceRegex.ReplaceAllString(raw, " "))
	return trimmed
}
