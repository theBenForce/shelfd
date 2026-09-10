package epub

import (
	"bytes"
	"fmt"
	"html"
	"path"
	"regexp"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	whitespaceRegex     = regexp.MustCompile(`[ \t\r\n]+`)
	multiNLRegex        = regexp.MustCompile(`\n{3,}`)
	viewportRegex       = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+name=["']viewport["']|<meta[^>]+name=["']viewport["'][^>]+content=["']([^"']+)["']`)
	widthValRegex       = regexp.MustCompile(`(?i)width\s*=\s*(\d+)`)
	heightValRegex      = regexp.MustCompile(`(?i)height\s*=\s*(\d+)`)
	svgViewBoxRegex     = regexp.MustCompile(`(?i)<svg[^>]+viewBox=["']\s*([0-9.]+)[,\s]+([0-9.]+)[,\s]+([0-9.]+)[,\s]+([0-9.]+)["']`)
	svgWidthHeightRegex = regexp.MustCompile(`(?i)<svg[^>]+width=["'](\d+)(?:px)?["'][^>]+height=["'](\d+)(?:px)?["']|<svg[^>]+height=["'](\d+)(?:px)?["'][^>]+width=["'](\d+)(?:px)?["']`)
	imgWidthHeightRegex = regexp.MustCompile(`(?i)<img[^>]+width=["'](\d+)(?:px)?["'][^>]+height=["'](\d+)(?:px)?["']|<img[^>]+height=["'](\d+)(?:px)?["'][^>]+width=["'](\d+)(?:px)?["']`)
	srcAttrRegex        = regexp.MustCompile(`(?i)(<img[^>]+src=["'])([^"']+)(["'])`)
	imgXlinkRegex       = regexp.MustCompile(`(?i)(<image[^>]+(?:xlink:href|href)=["'])([^"']+)(["'])`)
	linkHrefRegex       = regexp.MustCompile(`(?i)(<link[^>]+href=["'])([^"']+)(["'])`)
	sourceSrcRegex      = regexp.MustCompile(`(?i)(<source[^>]+src=["'])([^"']+)(["'])`)
	cssUrlRegex         = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]+)["']?\s*\)`)
	negativeZIndex      = regexp.MustCompile(`(?i)z-index\s*:\s*-\d+`)
	headTagRegex        = regexp.MustCompile(`(?i)<head[^>]*>`)
)

// ExtractPageDimensions extracts the viewport width and height from an XHTML content document.
func ExtractPageDimensions(rawHTML string) (width *int, height *int) {
	// 1. Meta viewport tag
	if m := viewportRegex.FindStringSubmatch(rawHTML); len(m) > 0 {
		contentStr := m[1]
		if contentStr == "" {
			contentStr = m[2]
		}
		if wm := widthValRegex.FindStringSubmatch(contentStr); len(wm) > 1 {
			if w, err := strconv.Atoi(wm[1]); err == nil && w > 0 {
				width = &w
			}
		}
		if hm := heightValRegex.FindStringSubmatch(contentStr); len(hm) > 1 {
			if h, err := strconv.Atoi(hm[1]); err == nil && h > 0 {
				height = &h
			}
		}
		if width != nil && height != nil {
			return width, height
		}
	}

	// 2. SVG viewBox fallback
	if sm := svgViewBoxRegex.FindStringSubmatch(rawHTML); len(sm) > 4 {
		wFloat, wErr := strconv.ParseFloat(sm[3], 64)
		hFloat, hErr := strconv.ParseFloat(sm[4], 64)
		if wErr == nil && hErr == nil && wFloat > 0 && hFloat > 0 {
			w := int(wFloat)
			h := int(hFloat)
			return &w, &h
		}
	}

	// 3. SVG width/height attribute fallback
	if swm := svgWidthHeightRegex.FindStringSubmatch(rawHTML); len(swm) > 0 {
		var wStr, hStr string
		if swm[1] != "" {
			wStr, hStr = swm[1], swm[2]
		} else {
			hStr, wStr = swm[3], swm[4]
		}
		if w, err := strconv.Atoi(wStr); err == nil && w > 0 {
			if h, err := strconv.Atoi(hStr); err == nil && h > 0 {
				return &w, &h
			}
		}
	}

	// 4. Primary img width/height fallback
	if im := imgWidthHeightRegex.FindStringSubmatch(rawHTML); len(im) > 0 {
		var wStr, hStr string
		if im[1] != "" {
			wStr, hStr = im[1], im[2]
		} else {
			hStr, wStr = im[3], im[4]
		}
		if w, err := strconv.Atoi(wStr); err == nil && w > 0 {
			if h, err := strconv.Atoi(hStr); err == nil && h > 0 {
				return &w, &h
			}
		}
	}

	return width, height
}

// ExtractChapters walks spine itemrefs in order and converts XHTML content into clean plaintext/markdown.
func (r *Reader) ExtractChapters() ([]*ParsedChapter, error) {
	if r.opf == nil {
		return nil, fmt.Errorf("no opf package loaded")
	}

	manifestMap := make(map[string]opfItem)
	for _, item := range r.opf.Manifest.Items {
		manifestMap[item.ID] = item
	}

	isPrePaginated := r.IsPrePaginated()
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

		rawHTMLStr := string(rawHTML)
		pageWidth, pageHeight := ExtractPageDimensions(rawHTMLStr)

		// Resolve page spread
		var pageSpread *string
		props := strings.ToLower(itemref.Properties)
		if strings.Contains(props, "page-spread-left") {
			s := "left"
			pageSpread = &s
		} else if strings.Contains(props, "page-spread-right") {
			s := "right"
			pageSpread = &s
		} else if strings.Contains(props, "page-spread-center") || strings.Contains(props, "rendition:page-spread-center") {
			s := "center"
			pageSpread = &s
		} else if pageWidth != nil && pageHeight != nil && float64(*pageWidth) > float64(*pageHeight)*1.2 {
			s := "center"
			pageSpread = &s
		} else if isPrePaginated {
			if chapterIndex == 0 {
				s := "right" // Cover standalone Recto
				pageSpread = &s
			} else if chapterIndex%2 == 1 {
				s := "left" // Verso
				pageSpread = &s
			} else {
				s := "right" // Recto
				pageSpread = &s
			}
		}

		title, plaintext := extractTextFromHTML(rawHTMLStr)
		if !isPrePaginated && strings.TrimSpace(plaintext) == "" {
			continue
		}

		chapterIndex++
		if isPrePaginated && title == nil {
			t := fmt.Sprintf("Page %d", chapterIndex)
			title = &t
		}

		chapters = append(chapters, &ParsedChapter{
			Index:        chapterIndex,
			Title:        title,
			ContentPlain: plaintext,
			Href:         fullPath,
			PageWidth:    pageWidth,
			PageHeight:   pageHeight,
			PageSpread:   pageSpread,
		})
	}

	return chapters, nil
}

// ExtractRawDocument returns the raw bytes of an XHTML document from the EPUB archive.
func (r *Reader) ExtractRawDocument(href string) ([]byte, error) {
	cleanHref := path.Clean(href)
	zf := r.findFile(cleanHref)
	if zf == nil && r.rootBase != "" {
		zf = r.findFile(path.Join(r.rootBase, cleanHref))
	}
	if zf == nil {
		return nil, fmt.Errorf("document not found: %s", href)
	}
	return readZipFile(zf)
}

// ExtractAsset returns the raw bytes and MIME type of an asset from the EPUB archive.
func (r *Reader) ExtractAsset(assetPath string) ([]byte, string, error) {
	cleanPath := path.Clean(strings.TrimPrefix(assetPath, "/"))
	if strings.HasPrefix(cleanPath, "..") {
		return nil, "", fmt.Errorf("illegal path traversal in asset: %s", assetPath)
	}

	zf := r.findFile(cleanPath)
	if zf == nil && r.rootBase != "" {
		zf = r.findFile(path.Join(r.rootBase, cleanPath))
	}
	if zf == nil {
		return nil, "", fmt.Errorf("asset not found: %s", assetPath)
	}

	data, err := readZipFile(zf)
	if err != nil {
		return nil, "", fmt.Errorf("reading asset: %w", err)
	}

	return data, DetectMimeType(cleanPath), nil
}

// DetectMimeType infers MIME type from file extension.
func DetectMimeType(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".html", ".xhtml":
		return "text/html; charset=utf-8"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	default:
		return "application/octet-stream"
	}
}

// RewritePageHTML rewrites relative asset paths in XHTML to Shelfd asset endpoints and normalizes styling.
func RewritePageHTML(rawHTML, bookID, chapterHref string, pageWidth, pageHeight *int) string {
	baseDir := path.Dir(chapterHref)
	resolveURL := func(relURL string) string {
		trimmed := strings.TrimSpace(relURL)
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") ||
			strings.HasPrefix(trimmed, "data:") || strings.HasPrefix(trimmed, "/") {
			return trimmed
		}
		target := path.Clean(path.Join(baseDir, trimmed))
		return fmt.Sprintf("/api/v1/books/%s/assets/%s", bookID, target)
	}

	htmlStr := rawHTML

	// Rewrite img src
	htmlStr = srcAttrRegex.ReplaceAllStringFunc(htmlStr, func(m string) string {
		parts := srcAttrRegex.FindStringSubmatch(m)
		if len(parts) == 4 {
			return parts[1] + resolveURL(parts[2]) + parts[3]
		}
		return m
	})

	// Rewrite image xlink:href/href (SVG)
	htmlStr = imgXlinkRegex.ReplaceAllStringFunc(htmlStr, func(m string) string {
		parts := imgXlinkRegex.FindStringSubmatch(m)
		if len(parts) == 4 {
			return parts[1] + resolveURL(parts[2]) + parts[3]
		}
		return m
	})

	// Rewrite link href (CSS)
	htmlStr = linkHrefRegex.ReplaceAllStringFunc(htmlStr, func(m string) string {
		parts := linkHrefRegex.FindStringSubmatch(m)
		if len(parts) == 4 {
			return parts[1] + resolveURL(parts[2]) + parts[3]
		}
		return m
	})

	// Rewrite source src
	htmlStr = sourceSrcRegex.ReplaceAllStringFunc(htmlStr, func(m string) string {
		parts := sourceSrcRegex.FindStringSubmatch(m)
		if len(parts) == 4 {
			return parts[1] + resolveURL(parts[2]) + parts[3]
		}
		return m
	})

	// Rewrite css url(...)
	htmlStr = cssUrlRegex.ReplaceAllStringFunc(htmlStr, func(m string) string {
		parts := cssUrlRegex.FindStringSubmatch(m)
		if len(parts) == 2 {
			return fmt.Sprintf("url('%s')", resolveURL(parts[1]))
		}
		return m
	})

	// Fix negative z-indices
	htmlStr = negativeZIndex.ReplaceAllString(htmlStr, "z-index: 1")

	// Inject defensive containment CSS, viewport meta, and auto-fit scaling for fixed-layout pages
	var injected string
	if pageWidth != nil && pageHeight != nil && *pageWidth > 0 && *pageHeight > 0 {
		dw := *pageWidth
		dh := *pageHeight
		injected = fmt.Sprintf(`<meta name="viewport" content="width=%d, height=%d" />
<style>
html {
  margin: 0 !important;
  padding: 0 !important;
  width: 100%% !important;
  height: 100%% !important;
  overflow: hidden !important;
  background-color: transparent !important;
}
body {
  margin: 0 !important;
  padding: 0 !important;
  overflow: hidden !important;
  background-color: transparent !important;
  transform-origin: 0 0 !important;
}
</style>
<script>
(function() {
  var dw = %d;
  var dh = %d;
  function fit() {
    if (!document.body) return;
    var ww = window.innerWidth;
    var wh = window.innerHeight;
    if (ww <= 0 || wh <= 0) return;
    var scale = Math.min(ww / dw, wh / dh);
    var sw = dw * scale;
    var sh = dh * scale;
    var ox = Math.max(0, (ww - sw) / 2);
    var oy = Math.max(0, (wh - sh) / 2);
    document.body.style.width = dw + 'px';
    document.body.style.height = dh + 'px';
    document.body.style.position = 'absolute';
    document.body.style.left = ox + 'px';
    document.body.style.top = oy + 'px';
    document.body.style.transformOrigin = '0 0';
    document.body.style.transform = 'scale(' + scale + ')';
    document.body.style.overflow = 'hidden';
  }
  window.addEventListener('resize', fit);
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', fit);
  } else {
    fit();
  }
})();
</script>`, dw, dh, dw, dh)
	} else {
		injected = `<style>
html, body {
  margin: 0 !important;
  padding: 0 !important;
  width: 100% !important;
  height: 100% !important;
  overflow: hidden !important;
  box-sizing: border-box !important;
  background-color: transparent !important;
}
img, svg {
  max-width: 100%;
  max-height: 100%;
  display: block;
}
</style>`
	}

	headLoc := headTagRegex.FindStringIndex(htmlStr)
	if len(headLoc) == 2 {
		htmlStr = htmlStr[:headLoc[1]] + "\n" + injected + htmlStr[headLoc[1]:]
	} else {
		htmlStr = injected + htmlStr
	}

	return htmlStr
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
