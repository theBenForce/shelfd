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
	whitespaceRegex       = regexp.MustCompile(`[ \t\r\n]+`)
	multiNLRegex          = regexp.MustCompile(`\n{3,}`)
	viewportRegex         = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+name=["']viewport["']|<meta[^>]+name=["']viewport["'][^>]+content=["']([^"']+)["']`)
	viewportTagRegex      = regexp.MustCompile(`(?i)<meta\b[^>]*\bname\s*=\s*["']viewport["'][^>]*>`)
	widthValRegex         = regexp.MustCompile(`(?i)width\s*=\s*(\d+)`)
	heightValRegex        = regexp.MustCompile(`(?i)height\s*=\s*(\d+)`)
	svgViewBoxRegex       = regexp.MustCompile(`(?i)<svg[^>]+viewBox=["']\s*([0-9.]+)[,\s]+([0-9.]+)[,\s]+([0-9.]+)[,\s]+([0-9.]+)["']`)
	svgWidthHeightRegex   = regexp.MustCompile(`(?i)<svg[^>]+width=["'](\d+)(?:px)?["'][^>]+height=["'](\d+)(?:px)?["']|<svg[^>]+height=["'](\d+)(?:px)?["'][^>]+width=["'](\d+)(?:px)?["']`)
	imgWidthHeightRegex   = regexp.MustCompile(`(?i)<img[^>]+width=["'](\d+)(?:px)?["'][^>]+height=["'](\d+)(?:px)?["']|<img[^>]+height=["'](\d+)(?:px)?["'][^>]+width=["'](\d+)(?:px)?["']`)
	bodyStyleAttrRegex    = regexp.MustCompile(`(?i)<body\b[^>]*\bstyle\s*=\s*["']([^"']+)["']`)
	divStyleAttrRegex     = regexp.MustCompile(`(?i)<div\b[^>]*\bstyle\s*=\s*["']([^"']+)["']`)
	styleBlockRegex       = regexp.MustCompile(`(?is)<style\b[^>]*>(.*?)</style>`)
	cssAtPageRegex        = regexp.MustCompile(`(?is)@page\b[^{]*\{([^}]*)\}`)
	cssPageSizeRegex      = regexp.MustCompile(`(?i)(?:^|[;\s])size\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*px\s+([0-9]+(?:\.[0-9]+)?)\s*px`)
	cssPageRuleRegex      = regexp.MustCompile(`(?is)(?:^|[}\s,])(?:html|body|\.[a-z0-9_-]*page[a-z0-9_-]*|#[a-z0-9_-]*page[a-z0-9_-]*|div\.[a-z0-9_-]+)\s*\{([^}]*)\}`)
	cssWidthPxRegex       = regexp.MustCompile(`(?i)(?:^|[;\s])width\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*px`)
	cssHeightPxRegex      = regexp.MustCompile(`(?i)(?:^|[;\s])height\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*px`)
	srcAttrRegex          = regexp.MustCompile(`(?i)(<img[^>]+src=["'])([^"']+)(["'])`)
	imgXlinkRegex         = regexp.MustCompile(`(?i)(<image[^>]+(?:xlink:href|href)=["'])([^"']+)(["'])`)
	linkHrefRegex         = regexp.MustCompile(`(?i)(<link[^>]+href=["'])([^"']+)(["'])`)
	sourceSrcRegex        = regexp.MustCompile(`(?i)(<source[^>]+src=["'])([^"']+)(["'])`)
	cssUrlRegex           = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]+)["']?\s*\)`)
	negativeZIndex        = regexp.MustCompile(`(?i)z-index\s*:\s*-\d+`)
	headTagRegex          = regexp.MustCompile(`(?i)<head[^>]*>`)
	closeHeadTagRegex     = regexp.MustCompile(`(?i)</head>`)
	selfClosingTagRegex   = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9:-]*)((?:\s+[^>"'/=\s]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=>/]+))?)*)\s*/>`)
	punctNewlineSpanRegex = regexp.MustCompile(`([^\p{L}\p{N}<>\s])\r?\n\s*(<span\b)`)

	htmlSelfClosingAllowedTags = map[string]bool{
		"area":     true,
		"base":     true,
		"basefont": true,
		"bgsound":  true,
		"br":       true,
		"circle":   true,
		"col":      true,
		"command":  true,
		"ellipse":  true,
		"embed":    true,
		"frame":    true,
		"hr":       true,
		"image":    true,
		"img":      true,
		"input":    true,
		"isindex":  true,
		"keygen":   true,
		"line":     true,
		"link":     true,
		"menuitem": true,
		"meta":     true,
		"nextid":   true,
		"param":    true,
		"path":     true,
		"polygon":  true,
		"polyline": true,
		"rect":     true,
		"source":   true,
		"spacer":   true,
		"stop":     true,
		"track":    true,
		"use":      true,
		"wbr":      true,
	}
)

func parseCSSPxPair(decl string, minDim int) (*int, *int) {
	wm := cssWidthPxRegex.FindStringSubmatch(decl)
	hm := cssHeightPxRegex.FindStringSubmatch(decl)
	if len(wm) < 2 || len(hm) < 2 {
		return nil, nil
	}
	wf, wErr := strconv.ParseFloat(wm[1], 64)
	hf, hErr := strconv.ParseFloat(hm[1], 64)
	if wErr != nil || hErr != nil {
		return nil, nil
	}
	w := int(wf + 0.5)
	h := int(hf + 0.5)
	if w < minDim || h < minDim {
		return nil, nil
	}
	return &w, &h
}

// ExtractCSSPageDimensions extracts fixed pixel page dimensions from CSS @page or root container rules.
func ExtractCSSPageDimensions(cssText string) (*int, *int) {
	for _, m := range cssAtPageRegex.FindAllStringSubmatch(cssText, -1) {
		if len(m) < 2 {
			continue
		}
		if sm := cssPageSizeRegex.FindStringSubmatch(m[1]); len(sm) >= 3 {
			wf, wErr := strconv.ParseFloat(sm[1], 64)
			hf, hErr := strconv.ParseFloat(sm[2], 64)
			if wErr == nil && hErr == nil && int(wf+0.5) >= 100 && int(hf+0.5) >= 100 {
				w := int(wf + 0.5)
				h := int(hf + 0.5)
				return &w, &h
			}
		}
		if w, h := parseCSSPxPair(m[1], 100); w != nil && h != nil {
			return w, h
		}
	}

	for _, m := range cssPageRuleRegex.FindAllStringSubmatch(cssText, -1) {
		if len(m) < 2 {
			continue
		}
		if w, h := parseCSSPxPair(m[1], 100); w != nil && h != nil {
			return w, h
		}
	}

	return nil, nil
}

// ExtractPageDimensions extracts the viewport width and height from an XHTML content document.
func ExtractPageDimensions(rawHTML string) (width *int, height *int) {
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

	if bm := bodyStyleAttrRegex.FindStringSubmatch(rawHTML); len(bm) >= 2 {
		if w, h := parseCSSPxPair(bm[1], 100); w != nil && h != nil {
			return w, h
		}
	}

	if sm := svgViewBoxRegex.FindStringSubmatch(rawHTML); len(sm) > 4 {
		wFloat, wErr := strconv.ParseFloat(sm[3], 64)
		hFloat, hErr := strconv.ParseFloat(sm[4], 64)
		if wErr == nil && hErr == nil && wFloat > 0 && hFloat > 0 {
			w := int(wFloat)
			h := int(hFloat)
			return &w, &h
		}
	}

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

	for _, sm := range styleBlockRegex.FindAllStringSubmatch(rawHTML, -1) {
		if len(sm) >= 2 {
			if w, h := ExtractCSSPageDimensions(sm[1]); w != nil && h != nil {
				return w, h
			}
		}
	}
	for _, dm := range divStyleAttrRegex.FindAllStringSubmatch(rawHTML, 4) {
		if len(dm) >= 2 {
			if w, h := parseCSSPxPair(dm[1], 100); w != nil && h != nil {
				return w, h
			}
		}
	}

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

// ExtractDocumentDimensions resolves fixed page dimensions from a spine XHTML document and its linked stylesheets.
func (r *Reader) ExtractDocumentDimensions(rawHTML, docHref string) (*int, *int) {
	if w, h := ExtractPageDimensions(rawHTML); w != nil && h != nil {
		return w, h
	}
	baseDir := path.Dir(docHref)
	for _, lm := range linkHrefRegex.FindAllStringSubmatch(rawHTML, -1) {
		if len(lm) < 3 {
			continue
		}
		href := strings.TrimSpace(lm[2])
		if idx := strings.IndexByte(href, '?'); idx >= 0 {
			href = href[:idx]
		}
		if idx := strings.IndexByte(href, '#'); idx >= 0 {
			href = href[:idx]
		}
		if !strings.HasSuffix(strings.ToLower(href), ".css") {
			continue
		}
		cssPath := path.Clean(path.Join(baseDir, href))
		zf := r.findFile(cssPath)
		if zf == nil && r.rootBase != "" {
			zf = r.findFile(path.Join(r.rootBase, cssPath))
		}
		if zf == nil {
			continue
		}
		cssBytes, err := readZipFile(zf)
		if err != nil {
			continue
		}
		if w, h := ExtractCSSPageDimensions(string(cssBytes)); w != nil && h != nil {
			return w, h
		}
	}
	return nil, nil
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

	tocMap := r.loadTOCMap()

	var bookTitle string
	if parsedBook, err := r.ParseBook(); err == nil && parsedBook != nil {
		bookTitle = parsedBook.Title
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
		pageWidth, pageHeight := r.ExtractDocumentDimensions(rawHTMLStr, fullPath)

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

		title, plaintext := extractTextFromHTMLWithContext(rawHTMLStr, bookTitle, fullPath)
		if tocTitle := matchTOCTitle(tocMap, fullPath, item.Href); tocTitle != nil {
			title = tocTitle
		}

		if !isPrePaginated && strings.TrimSpace(plaintext) == "" {
			continue
		}

		chapterIndex++

		chapters = append(chapters, &ParsedChapter{
			Index:          chapterIndex,
			Title:          title,
			ContentPlain:   plaintext,
			Href:           fullPath,
			PageWidth:      pageWidth,
			PageHeight:     pageHeight,
			PageSpread:     pageSpread,
			MediaOverlayID: strings.TrimSpace(item.MediaOverlay),
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
	return r.extractAssetWithLimit(assetPath, maxTextZipEntryBytes)
}

// ExtractMediaAsset returns the raw bytes and MIME type of a media asset with a higher size ceiling for embedded audio.
func (r *Reader) ExtractMediaAsset(assetPath string) ([]byte, string, error) {
	return r.extractAssetWithLimit(assetPath, maxMediaZipEntryBytes)
}

func (r *Reader) extractAssetWithLimit(assetPath string, maxBytes int64) ([]byte, string, error) {
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

	data, err := readZipFileWithLimit(zf, maxBytes)
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
	case ".smil":
		return "application/smil+xml"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".m4b", ".mp4":
		return "audio/mp4"
	case ".ogg", ".oga", ".opus":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
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

	htmlStr := normalizeXHTMLForHTML(rawHTML)

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

	readalongBridge := fmt.Sprintf(`<style>
.-epub-media-overlay-active,
[data-shelfd-readalong-active="true"] {
  background-color: rgba(255, 212, 0, 0.42);
  border-radius: 3px;
  box-shadow: 0 0 0 2px rgba(255, 212, 0, 0.32);
  transition: background-color 0.12s ease, box-shadow 0.12s ease;
}
</style>
<script>
(function() {
  var currentBookId = %q;
  var currentChapterHref = %q;
  var activeElements = [];

  function normalizeHref(h) {
    if (!h || typeof h !== 'string') return '';
    var s = h.trim().replace(/\\/g, '/');
    var hashIdx = s.indexOf('#');
    if (hashIdx >= 0) s = s.substring(0, hashIdx);
    while (s.indexOf('./') === 0) s = s.substring(2);
    while (s.indexOf('/') === 0) s = s.substring(1);
    return s.toLowerCase();
  }

  function addClasses(el, raw) {
    if (!el || !raw) return;
    var parts = String(raw).trim().split(/\s+/);
    for (var i = 0; i < parts.length; i++) {
      if (parts[i]) el.classList.add(parts[i]);
    }
  }

  function removeClasses(el, raw) {
    if (!el || !raw) return;
    var parts = String(raw).trim().split(/\s+/);
    for (var i = 0; i < parts.length; i++) {
      if (parts[i]) el.classList.remove(parts[i]);
    }
  }

  var normCurrentHref = normalizeHref(currentChapterHref);

  function clearActive(activeClass) {
    for (var i = 0; i < activeElements.length; i++) {
      var el = activeElements[i];
      if (!el) continue;
      el.removeAttribute('data-shelfd-readalong-active');
      el.classList.remove('-epub-media-overlay-active');
      removeClasses(el, activeClass);
    }
    activeElements = [];
  }

  function applyHighlight(fragmentId, activeClass, playbackActiveClass, playing) {
    var cls = activeClass || '-epub-media-overlay-active';
    var playCls = playbackActiveClass || '-epub-media-overlay-playing';
    clearActive(cls);
    if (document.documentElement) {
      if (playing) {
        addClasses(document.documentElement, playCls);
        document.documentElement.classList.add('-epub-media-overlay-playing');
      } else {
        removeClasses(document.documentElement, playCls);
        document.documentElement.classList.remove('-epub-media-overlay-playing');
      }
    }
    if (!fragmentId) return;
    var target = document.getElementById(fragmentId);
    if (!target) return;
    addClasses(target, cls);
    target.classList.add('-epub-media-overlay-active');
    target.setAttribute('data-shelfd-readalong-active', 'true');
    activeElements.push(target);
  }

  window.__shelfdHighlightReadalong = applyHighlight;

  window.addEventListener('message', function(ev) {
    var d = ev && ev.data;
    if (!d || typeof d !== 'object') return;
    if (d.type === 'shelfd:readalong-highlight') {
      var cls = d.activeClass || '';
      var playCls = d.playbackActiveClass || '';
      if (d.bookId && currentBookId && d.bookId !== currentBookId) {
        applyHighlight('', cls, playCls, false);
        return;
      }
      var incomingHref = normalizeHref(d.chapterHref || '');
      if (incomingHref && normCurrentHref && incomingHref !== normCurrentHref) {
        applyHighlight('', cls, playCls, false);
        return;
      }
      applyHighlight(d.fragmentId || '', cls, playCls, !!d.playing);
    }
  });

  function postToShelfdBridge(payload) {
    if (window.ShelfdBridge && typeof window.ShelfdBridge.postMessage === 'function') {
      try {
        window.ShelfdBridge.postMessage(JSON.stringify(payload));
      } catch (_) {}
    }
  }

  document.addEventListener('click', function(ev) {
    var sel = window.getSelection ? String(window.getSelection()).trim() : '';
    var targetEl = ev.target && ev.target.nodeType === 1 ? ev.target : (ev.target ? ev.target.parentElement : null);
    var isInteractive = targetEl && targetEl.closest && targetEl.closest('a, button, input, select, textarea, audio, video, [data-shelfd-readalong-id]');

    var node = targetEl;
    var sentReadalong = false;
    while (node && node !== document.body && node !== document.documentElement) {
      if (node.id) {
        var clickPayload = {
          type: 'shelfd:readalong-click',
          bookId: currentBookId,
          chapterHref: currentChapterHref,
          fragmentId: node.id
        };
        if (window.parent && window.parent !== window) {
          window.parent.postMessage(clickPayload, '*');
        }
        postToShelfdBridge(clickPayload);
        sentReadalong = true;
        break;
      }
      node = node.parentElement;
    }

    if (!sentReadalong && !isInteractive && sel.length === 0) {
      postToShelfdBridge({
        type: 'shelfd:toggle-controls',
        bookId: currentBookId,
        chapterHref: currentChapterHref
      });
    }
  });
})();
</script>`, bookID, chapterHref)

	dw, dh := 0, 0
	if pageWidth != nil && pageHeight != nil && *pageWidth > 0 && *pageHeight > 0 {
		dw = *pageWidth
		dh = *pageHeight
	} else if inferredW, inferredH := ExtractPageDimensions(rawHTML); inferredW != nil && inferredH != nil && *inferredW > 0 && *inferredH > 0 {
		dw = *inferredW
		dh = *inferredH
	}

	// Mobile WebViews treat fixed-pixel viewport metas as top-level layout viewports,
	// forcing window.innerWidth equal to the page width and defeating scale-to-fit.
	htmlStr = viewportTagRegex.ReplaceAllString(htmlStr, "")

	metaTag := `<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no" />`
	extraCSS := ""
	if dw > 0 && dh > 0 {
		metaTag = fmt.Sprintf(
			`<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no" data-shelfd-page-width="%d" data-shelfd-page-height="%d" />`,
			dw,
			dh,
		)
		extraCSS = fmt.Sprintf(`
body {
  width: %dpx !important;
  height: %dpx !important;
  position: absolute !important;
  left: 0 !important;
  top: 0 !important;
}`, dw, dh)
	} else {
		extraCSS = `
body {
  width: 100%;
  height: 100%;
  box-sizing: border-box;
}
img, svg {
  max-width: 100%;
  max-height: 100%;
  display: block;
}`
	}

	fitConfig := fmt.Sprintf(
		"var staticW = %d; var staticH = %d; var pageBookId = %q; var pageChapterHref = %q;",
		dw,
		dh,
		bookID,
		chapterHref,
	)

	injected := metaTag + `
<style>
html {
  margin: 0 !important;
  padding: 0 !important;
  width: 100% !important;
  height: 100% !important;
  overflow: hidden !important;
  background-color: #ffffff;
}
html[data-shelfd-fit-pending="true"] body {
  opacity: 0 !important;
}
body {
  margin: 0 !important;
  padding: 0 !important;
  overflow: hidden !important;
  background-color: #ffffff;
  transform-origin: 0 0 !important;
}` + extraCSS + `
</style>
` + readalongBridge + `
<script>
(function() {
  ` + fitConfig + `
  if (document.documentElement) {
    document.documentElement.setAttribute('data-shelfd-fit-pending', 'true');
  }
  var reportedDims = false;
  var retryCount = 0;

  function revealPage() {
    if (document.documentElement) {
      document.documentElement.removeAttribute('data-shelfd-fit-pending');
      document.documentElement.setAttribute('data-shelfd-fitted', 'true');
    }
  }

  function parsePx(val) {
    if (!val) return 0;
    var m = String(val).match(/([0-9]+(?:\.[0-9]+)?)\s*px/i);
    return m ? parseFloat(m[1]) : 0;
  }

  function detectDimensions(ww, wh) {
    if (staticW > 0 && staticH > 0) return { w: staticW, h: staticH };
    if (!document.body) return { w: 0, h: 0 };

    var bodyStyle = document.body.getAttribute('style') || '';
    var bw = parsePx((bodyStyle.match(/(?:^|[;\s])width\s*:\s*([^;]+)/i) || [])[1]);
    var bh = parsePx((bodyStyle.match(/(?:^|[;\s])height\s*:\s*([^;]+)/i) || [])[1]);
    if (bw > 0 && bh > 0) return { w: bw, h: bh };

    var svg = document.querySelector('svg[viewBox]');
    if (svg) {
      var vb = (svg.getAttribute('viewBox') || '').trim().split(/[\s,]+/);
      if (vb.length === 4) {
        var sw = parseFloat(vb[2]);
        var sh = parseFloat(vb[3]);
        if (sw > 0 && sh > 0) return { w: sw, h: sh };
      }
    }

    var children = document.body.children;
    for (var i = 0; i < children.length; i++) {
      var el = children[i];
      if (!el || el.tagName === 'SCRIPT' || el.tagName === 'STYLE') continue;
      var cs = window.getComputedStyle ? window.getComputedStyle(el) : null;
      var cw = cs ? parseFloat(cs.width) : 0;
      var ch = cs ? parseFloat(cs.height) : 0;
      if (cw >= 100 && ch >= 100 && (cw > ww + 4 || ch > wh + 4)) {
        return { w: cw, h: ch };
      }
      if (el.tagName === 'IMG' && el.naturalWidth >= 100 && el.naturalHeight >= 100 && children.length === 1) {
        return { w: el.naturalWidth, h: el.naturalHeight };
      }
    }

    var scrollW = document.body.scrollWidth || 0;
    var scrollH = document.body.scrollHeight || 0;
    if (scrollW > ww + 8 || scrollH > wh + 8) {
      return { w: Math.max(scrollW, ww), h: Math.max(scrollH, wh) };
    }
    return { w: 0, h: 0 };
  }

  function fit() {
    if (!document.body || !document.documentElement) return;
    var ww = window.__shelfdTargetWidth || document.documentElement.clientWidth || window.innerWidth;
    var wh = window.__shelfdTargetHeight || document.documentElement.clientHeight || window.innerHeight;
    if (ww <= 0 || wh <= 0) {
      if (retryCount < 60 && typeof requestAnimationFrame === 'function') {
        retryCount++;
        requestAnimationFrame(fit);
      } else {
        revealPage();
      }
      return;
    }

    var dims = detectDimensions(ww, wh);
    var pageW = dims.w;
    var pageH = dims.h;
    if (pageW <= 0 || pageH <= 0) {
      revealPage();
      return;
    }
    staticW = pageW;
    staticH = pageH;

    var scale = Math.min(ww / pageW, wh / pageH);
    var sw = pageW * scale;
    var sh = pageH * scale;
    var ox = Math.max(0, (ww - sw) / 2);
    var oy = Math.max(0, (wh - sh) / 2);
    var bs = document.body.style;
    bs.setProperty('width', pageW + 'px', 'important');
    bs.setProperty('height', pageH + 'px', 'important');
    bs.setProperty('position', 'absolute', 'important');
    bs.setProperty('left', ox + 'px', 'important');
    bs.setProperty('top', oy + 'px', 'important');
    bs.setProperty('transform-origin', '0 0', 'important');
    bs.setProperty('transform', 'scale(' + scale + ')', 'important');
    bs.setProperty('overflow', 'hidden', 'important');
    revealPage();

    if (!reportedDims) {
      reportedDims = true;
      var dimPayload = {
        type: 'shelfd:page-dimensions',
        bookId: pageBookId,
        chapterHref: pageChapterHref,
        width: Math.round(pageW),
        height: Math.round(pageH)
      };
      if (window.parent && window.parent !== window) {
        try { window.parent.postMessage(dimPayload, '*'); } catch (_) {}
      }
      if (window.ShelfdBridge && typeof window.ShelfdBridge.postMessage === 'function') {
        try { window.ShelfdBridge.postMessage(JSON.stringify(dimPayload)); } catch (_) {}
      }
    }
  }

  window.__shelfdFitPage = function(tw, th) {
    if (typeof tw === 'number' && tw > 0 && typeof th === 'number' && th > 0) {
      window.__shelfdTargetWidth = tw;
      window.__shelfdTargetHeight = th;
    }
    fit();
  };

  window.addEventListener('resize', fit);
  window.addEventListener('load', fit);
  if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', fit);
  }
  if (typeof ResizeObserver === 'function') {
    try {
      new ResizeObserver(fit).observe(document.documentElement);
    } catch (_) {}
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', fit);
  } else {
    fit();
  }
  setTimeout(revealPage, 180);
})();
</script>`

	if closeLoc := closeHeadTagRegex.FindStringIndex(htmlStr); len(closeLoc) == 2 {
		htmlStr = htmlStr[:closeLoc[0]] + injected + "\n" + htmlStr[closeLoc[0]:]
	} else if headLoc := headTagRegex.FindStringIndex(htmlStr); len(headLoc) == 2 {
		htmlStr = htmlStr[:headLoc[1]] + "\n" + injected + htmlStr[headLoc[1]:]
	} else {
		htmlStr = injected + htmlStr
	}

	return htmlStr
}

func normalizeXHTMLForHTML(rawHTML string) string {
	normalized := selfClosingTagRegex.ReplaceAllStringFunc(rawHTML, func(match string) string {
		parts := selfClosingTagRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		tagName := parts[1]
		lowerTag := strings.ToLower(tagName)
		if htmlSelfClosingAllowedTags[lowerTag] || strings.HasPrefix(lowerTag, "fe") {
			return match
		}
		return "<" + tagName + parts[2] + "></" + tagName + ">"
	})
	return punctNewlineSpanRegex.ReplaceAllString(normalized, "$1$2")
}

func extractTextFromHTML(raw string) (*string, string) {
	return extractTextFromHTMLWithContext(raw, "", "")
}

func extractTextFromHTMLWithContext(raw, bookTitle, fileName string) (*string, string) {
	normalized := normalizeXHTMLForHTML(raw)
	doc, err := nethtml.Parse(strings.NewReader(normalized))
	if err != nil {
		// Fallback to basic string extraction on malformed HTML
		return nil, strings.TrimSpace(raw)
	}

	title := extractChapterTitle(doc, bookTitle, fileName)
	var blocks []string
	collectBlocks(doc, &blocks, false)

	result := strings.Join(blocks, "\n\n")
	result = multiNLRegex.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return title, result
}

func extractChapterTitle(doc *nethtml.Node, bookTitle, fileName string) *string {
	var h1Title, h2Title, h3Title, h4Title, docTitle string

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
			case atom.H4:
				if h4Title == "" {
					h4Title = cleanInlineText(collectNodeText(n))
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
	if h4Title != "" {
		return &h4Title
	}
	if docTitle != "" && !isUnusableDocTitle(docTitle, bookTitle, fileName) {
		return &docTitle
	}
	return nil
}

func isUnusableDocTitle(docTitle, bookTitle, fileName string) bool {
	cleanDoc := strings.TrimSpace(docTitle)
	if cleanDoc == "" {
		return true
	}
	if bookTitle != "" && strings.EqualFold(cleanDoc, strings.TrimSpace(bookTitle)) {
		return true
	}
	if fileName != "" {
		cleanFile := path.Base(fileName)
		fileStem := strings.TrimSuffix(cleanFile, path.Ext(cleanFile))
		if strings.EqualFold(cleanDoc, cleanFile) || strings.EqualFold(cleanDoc, fileStem) {
			return true
		}
	}
	lower := strings.ToLower(cleanDoc)
	if strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".htm") {
		return true
	}
	return false
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
