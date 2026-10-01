package epub

import (
	"bytes"
	"net/url"
	"path"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type ncxPackage struct {
	NavMap ncxNavMap `xml:"navMap"`
}

type ncxNavMap struct {
	NavPoints []ncxNavPoint `xml:"navPoint"`
}

type ncxNavPoint struct {
	NavLabel  ncxNavLabel   `xml:"navLabel"`
	Content   ncxContent    `xml:"content"`
	NavPoints []ncxNavPoint `xml:"navPoint"`
}

type ncxNavLabel struct {
	Text string `xml:"text"`
}

type ncxContent struct {
	Src string `xml:"src,attr"`
}

// loadTOCMap extracts navigation and TOC entries from either EPUB 3 Navigation Document or EPUB 2 NCX.
// It returns a map of normalized target href paths to their authoritative chapter titles.
func (r *Reader) loadTOCMap() map[string]string {
	tocMap := make(map[string]string)
	if r.opf == nil {
		return tocMap
	}

	// 1. Try EPUB 3 Navigation Document (item with property "nav")
	for _, item := range r.opf.Manifest.Items {
		if hasProperty(item.Properties, "nav") {
			fullPath := item.Href
			if r.rootBase != "" {
				fullPath = path.Join(r.rootBase, item.Href)
			}
			zf := r.findFile(fullPath)
			if zf == nil {
				zf = r.findFile(item.Href)
			}
			if zf != nil {
				if data, err := readZipFile(zf); err == nil {
					r.parseNavTOC(data, fullPath, tocMap)
					if len(tocMap) > 0 {
						return tocMap
					}
				}
			}
		}
	}

	// 2. Try EPUB 2 NCX file
	var ncxHref string
	if r.opf.Spine.TOC != "" {
		for _, item := range r.opf.Manifest.Items {
			if item.ID == r.opf.Spine.TOC {
				ncxHref = item.Href
				break
			}
		}
	}
	if ncxHref == "" {
		for _, item := range r.opf.Manifest.Items {
			if strings.EqualFold(item.MediaType, "application/x-dtbncx+xml") || strings.Contains(strings.ToLower(item.MediaType), "dtbncx") {
				ncxHref = item.Href
				break
			}
		}
	}
	if ncxHref == "" {
		if r.findFile("toc.ncx") != nil {
			ncxHref = "toc.ncx"
		} else if r.rootBase != "" && r.findFile(path.Join(r.rootBase, "toc.ncx")) != nil {
			ncxHref = "toc.ncx"
		}
	}

	if ncxHref != "" {
		fullPath := ncxHref
		if r.rootBase != "" && !strings.HasPrefix(fullPath, r.rootBase+"/") && fullPath != r.rootBase {
			fullPath = path.Join(r.rootBase, ncxHref)
		}
		zf := r.findFile(fullPath)
		if zf == nil {
			zf = r.findFile(ncxHref)
		}
		if zf != nil {
			if data, err := readZipFile(zf); err == nil {
				r.parseNCXTOC(data, fullPath, tocMap)
			}
		}
	}

	return tocMap
}

func hasProperty(properties, target string) bool {
	fields := strings.Fields(properties)
	for _, f := range fields {
		if strings.EqualFold(f, target) {
			return true
		}
	}
	return false
}

func normalizeTOCPath(baseDir, rawSrc string) string {
	s := strings.TrimSpace(rawSrc)
	if idx := strings.IndexAny(s, "#?"); idx != -1 {
		s = s[:idx]
	}
	if unescaped, err := url.PathUnescape(s); err == nil {
		s = unescaped
	}
	s = strings.TrimPrefix(path.Clean(strings.ReplaceAll(s, "\\", "/")), "/")
	if baseDir != "" && baseDir != "." {
		return strings.TrimPrefix(path.Clean(path.Join(baseDir, s)), "/")
	}
	return s
}

func (r *Reader) recordTOCEntry(tocMap map[string]string, baseDir, rawSrc, title string) {
	cleanTitle := cleanInlineText(title)
	if cleanTitle == "" || rawSrc == "" {
		return
	}

	target := normalizeTOCPath(baseDir, rawSrc)
	if target == "" || target == "." {
		return
	}

	// Register multiple lookup keys for resilient spine matching
	if _, exists := tocMap[target]; !exists {
		tocMap[target] = cleanTitle
	}

	if r.rootBase != "" && strings.HasPrefix(target, r.rootBase+"/") {
		rel := strings.TrimPrefix(target, r.rootBase+"/")
		if _, exists := tocMap[rel]; !exists {
			tocMap[rel] = cleanTitle
		}
	}

	base := path.Base(target)
	if _, exists := tocMap[base]; !exists {
		tocMap[base] = cleanTitle
	}
}

func (r *Reader) parseNavTOC(data []byte, navFullPath string, tocMap map[string]string) {
	doc, err := nethtml.Parse(bytes.NewReader(data))
	if err != nil {
		return
	}

	navDir := path.Dir(navFullPath)
	if navDir == "." {
		navDir = ""
	}

	var tocNav *nethtml.Node
	var firstNav *nethtml.Node

	var findNav func(*nethtml.Node)
	findNav = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.DataAtom == atom.Nav {
			if firstNav == nil {
				firstNav = n
			}
			for _, attr := range n.Attr {
				key := strings.ToLower(attr.Key)
				val := strings.ToLower(attr.Val)
				if (key == "epub:type" || key == "type") && strings.Contains(val, "toc") {
					tocNav = n
					return
				}
				if key == "role" && strings.Contains(val, "doc-toc") {
					tocNav = n
					return
				}
				if key == "id" && val == "toc" {
					tocNav = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if tocNav != nil {
				return
			}
			findNav(c)
		}
	}
	findNav(doc)

	targetNav := tocNav
	if targetNav == nil {
		targetNav = firstNav
	}
	if targetNav == nil {
		targetNav = doc
	}

	var extractLinks func(*nethtml.Node)
	extractLinks = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.DataAtom == atom.A {
			var href string
			for _, attr := range n.Attr {
				if strings.ToLower(attr.Key) == "href" {
					href = attr.Val
					break
				}
			}
			if href != "" {
				linkText := collectNodeText(n)
				r.recordTOCEntry(tocMap, navDir, href, linkText)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractLinks(c)
		}
	}
	extractLinks(targetNav)
}

func (r *Reader) parseNCXTOC(data []byte, ncxFullPath string, tocMap map[string]string) {
	var pkg ncxPackage
	if err := decodeXMLResilient(data, "ncx", &pkg); err != nil {
		return
	}

	ncxDir := path.Dir(ncxFullPath)
	if ncxDir == "." {
		ncxDir = ""
	}

	var walkNavPoints func([]ncxNavPoint)
	walkNavPoints = func(points []ncxNavPoint) {
		for _, np := range points {
			if np.Content.Src != "" && np.NavLabel.Text != "" {
				r.recordTOCEntry(tocMap, ncxDir, np.Content.Src, np.NavLabel.Text)
			}
			if len(np.NavPoints) > 0 {
				walkNavPoints(np.NavPoints)
			}
		}
	}
	walkNavPoints(pkg.NavMap.NavPoints)
}

// matchTOCTitle attempts to match a spine document against the TOC map.
func matchTOCTitle(tocMap map[string]string, paths ...string) *string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		clean := strings.TrimPrefix(path.Clean(strings.ReplaceAll(p, "\\", "/")), "/")
		if title, ok := tocMap[clean]; ok && title != "" {
			return &title
		}
		if unescaped, err := url.PathUnescape(clean); err == nil && unescaped != clean {
			if title, ok := tocMap[unescaped]; ok && title != "" {
				return &title
			}
		}
		base := path.Base(clean)
		if title, ok := tocMap[base]; ok && title != "" {
			return &title
		}
		if unescapedBase, err := url.PathUnescape(base); err == nil && unescapedBase != base {
			if title, ok := tocMap[unescapedBase]; ok && title != "" {
				return &title
			}
		}
	}
	return nil
}
