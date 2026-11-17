package epub

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
)

// SMILPar represents a single <par> synchronization element in a SMIL Media Overlay document.
type SMILPar struct {
	ID        string
	EPUBType  string
	TextSrc   string
	AudioSrc  string
	ClipBegin float64
	ClipEnd   float64
}

// MediaOverlaySegment represents a resolved synchronized text/audio segment for a book spine item.
type MediaOverlaySegment struct {
	ChapterIndex int     `json:"chapter_index"`
	ChapterHref  string  `json:"chapter_href"`
	OverlayID    string  `json:"overlay_id,omitempty"`
	ParID        string  `json:"par_id,omitempty"`
	FragmentID   string  `json:"fragment_id"`
	Text         string  `json:"text,omitempty"`
	AudioSrc     string  `json:"audio_src,omitempty"`
	ClipBegin    float64 `json:"clip_begin"`
	ClipEnd      float64 `json:"clip_end"`
	GlobalBegin  float64 `json:"global_begin"`
	GlobalEnd    float64 `json:"global_end"`
}

// MediaOverlaysManifest holds the book-wide Media Overlays configuration and ordered timeline segments.
type MediaOverlaysManifest struct {
	HasMediaOverlays    bool                  `json:"has_media_overlays"`
	ActiveClass         string                `json:"active_class"`
	PlaybackActiveClass string                `json:"playback_active_class"`
	Narrator            string                `json:"narrator,omitempty"`
	DurationSeconds     *float64              `json:"duration_seconds,omitempty"`
	Segments            []MediaOverlaySegment `json:"segments"`
}

// ParseSMILClock parses a W3C EPUB Media Overlays / SMIL 3.0 clock value into seconds.
// Supported formats:
//   - Full clock: "hh:mm:ss.fff" (e.g. "01:02:03.5", "0:00:14.200")
//   - Partial clock: "mm:ss.fff" (e.g. "02:03.5", "00:05")
//   - Timecount: "12.34s", "500ms", "2.5min", "1.5h", or bare number "12.34"
func ParseSMILClock(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("empty clock value")
	}

	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "npt=") {
		s = strings.TrimSpace(s[4:])
		lower = strings.ToLower(s)
	}

	// Check timecount units (must check "ms" and "min" before "s" and "m")
	switch {
	case strings.HasSuffix(lower, "ms"):
		numStr := strings.TrimSpace(lower[:len(lower)-2])
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid ms clock value %q", raw)
		}
		return v / 1000.0, nil
	case strings.HasSuffix(lower, "min"):
		numStr := strings.TrimSpace(lower[:len(lower)-3])
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid min clock value %q", raw)
		}
		return v * 60.0, nil
	case strings.HasSuffix(lower, "h") && !strings.Contains(lower, ":"):
		numStr := strings.TrimSpace(lower[:len(lower)-1])
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid h clock value %q", raw)
		}
		return v * 3600.0, nil
	case strings.HasSuffix(lower, "s") && !strings.Contains(lower, ":"):
		numStr := strings.TrimSpace(lower[:len(lower)-1])
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid s clock value %q", raw)
		}
		return v, nil
	}

	// Check colon-separated clock values (hh:mm:ss.fff or mm:ss.fff)
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) == 3 {
			hours, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			mins, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			secs, err3 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
			if err1 != nil || err2 != nil || err3 != nil || hours < 0 || mins < 0 || secs < 0 {
				return 0, fmt.Errorf("invalid full clock value %q", raw)
			}
			return hours*3600.0 + mins*60.0 + secs, nil
		}
		if len(parts) == 2 {
			mins, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			secs, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 != nil || err2 != nil || mins < 0 || secs < 0 {
				return 0, fmt.Errorf("invalid partial clock value %q", raw)
			}
			return mins*60.0 + secs, nil
		}
		return 0, fmt.Errorf("invalid clock format %q", raw)
	}

	// Bare numeric seconds
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid numeric clock value %q", raw)
	}
	return v, nil
}

// ParseSMIL parses a SMIL 3.0 Media Overlay XML document in reading order.
func ParseSMIL(data []byte) ([]SMILPar, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	var pars []SMILPar
	var seqTypeStack []string
	var currentPar *SMILPar

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing smil xml: %w", err)
		}

		switch se := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(se.Name.Local)
			switch local {
			case "body", "seq":
				epubType := getXMLAttr(se.Attr, "type")
				if epubType == "" && len(seqTypeStack) > 0 {
					epubType = seqTypeStack[len(seqTypeStack)-1]
				}
				seqTypeStack = append(seqTypeStack, epubType)
			case "par":
				parID := getXMLAttr(se.Attr, "id")
				epubType := getXMLAttr(se.Attr, "type")
				if epubType == "" && len(seqTypeStack) > 0 {
					epubType = seqTypeStack[len(seqTypeStack)-1]
				}
				currentPar = &SMILPar{
					ID:       parID,
					EPUBType: epubType,
				}
			case "text":
				if currentPar != nil {
					src := getXMLAttr(se.Attr, "src")
					if src != "" {
						currentPar.TextSrc = src
					}
				}
			case "audio":
				if currentPar != nil {
					src := getXMLAttr(se.Attr, "src")
					if src != "" {
						currentPar.AudioSrc = src
					}
					clipBeginRaw := getXMLAttr(se.Attr, "clipBegin", "clip-begin", "clipbegin")
					if clipBeginRaw != "" {
						if v, err := ParseSMILClock(clipBeginRaw); err == nil {
							currentPar.ClipBegin = v
						}
					}
					clipEndRaw := getXMLAttr(se.Attr, "clipEnd", "clip-end", "clipend")
					if clipEndRaw != "" {
						if v, err := ParseSMILClock(clipEndRaw); err == nil {
							currentPar.ClipEnd = v
						}
					}
				}
			}
		case xml.EndElement:
			local := strings.ToLower(se.Name.Local)
			switch local {
			case "body", "seq":
				if len(seqTypeStack) > 0 {
					seqTypeStack = seqTypeStack[:len(seqTypeStack)-1]
				}
			case "par":
				if currentPar != nil {
					if currentPar.TextSrc != "" || currentPar.AudioSrc != "" {
						pars = append(pars, *currentPar)
					}
					currentPar = nil
				}
			}
		}
	}

	return pars, nil
}

func getXMLAttr(attrs []xml.Attr, names ...string) string {
	for _, want := range names {
		wantLower := strings.ToLower(want)
		for _, a := range attrs {
			if strings.ToLower(a.Name.Local) == wantLower {
				return strings.TrimSpace(a.Value)
			}
		}
	}
	return ""
}

// extractElementTextByIDMap parses an XHTML document and builds a lookup map from element ID to plain text.
func extractElementTextByIDMap(rawHTML string) map[string]string {
	result := make(map[string]string)
	normalized := normalizeXHTMLForHTML(rawHTML)
	doc, err := nethtml.Parse(strings.NewReader(normalized))
	if err != nil {
		return result
	}

	var collectText func(*nethtml.Node, *strings.Builder)
	collectText = func(n *nethtml.Node, sb *strings.Builder) {
		if n.Type == nethtml.TextNode {
			sb.WriteString(n.Data)
			return
		}
		if n.Type == nethtml.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "script" || tag == "style" || tag == "head" {
				return
			}
			if tag == "img" || tag == "image" {
				for _, a := range n.Attr {
					if strings.EqualFold(a.Key, "alt") && strings.TrimSpace(a.Val) != "" {
						sb.WriteString(" ")
						sb.WriteString(strings.TrimSpace(a.Val))
						sb.WriteString(" ")
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectText(c, sb)
		}
	}

	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode {
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "id") || strings.EqualFold(a.Key, "xml:id") {
					idVal := strings.TrimSpace(a.Val)
					if idVal != "" {
						var sb strings.Builder
						collectText(n, &sb)
						txt := strings.TrimSpace(whitespaceRegex.ReplaceAllString(sb.String(), " "))
						if txt != "" {
							result[idVal] = txt
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

// splitHrefAndFragment splits a SMIL text src like "../xhtml/page01.xhtml#word_1" into path and fragment ID.
func splitHrefAndFragment(src, smilFullPath string) (resolvedDocPath string, fragmentID string) {
	rawPath := src
	if idx := strings.Index(src, "#"); idx != -1 {
		rawPath = src[:idx]
		fragmentID = src[idx+1:]
	}
	if decodedFrag, err := url.PathUnescape(fragmentID); err == nil {
		fragmentID = decodedFrag
	}
	if decodedPath, err := url.PathUnescape(rawPath); err == nil {
		rawPath = decodedPath
	}
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", strings.TrimSpace(fragmentID)
	}
	smilDir := path.Dir(smilFullPath)
	resolved := path.Clean(path.Join(smilDir, rawPath))
	resolved = strings.TrimPrefix(resolved, "./")
	return resolved, strings.TrimSpace(fragmentID)
}

// ExtractMediaOverlays extracts the EPUB 3 Media Overlays (SMIL) configuration and timeline segments
// across all spine chapters in reading order.
func (r *Reader) ExtractMediaOverlays() (*MediaOverlaysManifest, error) {
	if r.opf == nil {
		return nil, fmt.Errorf("no opf package loaded")
	}

	parsedBook, err := r.ParseBook()
	if err != nil {
		return nil, err
	}

	activeClass := parsedBook.MediaActiveClass
	if activeClass == "" {
		activeClass = "-epub-media-overlay-active"
	}
	playbackActiveClass := parsedBook.MediaPlaybackActiveClass
	if playbackActiveClass == "" {
		playbackActiveClass = "-epub-media-overlay-playing"
	}

	manifest := &MediaOverlaysManifest{
		HasMediaOverlays:    parsedBook.HasMediaOverlays,
		ActiveClass:         activeClass,
		PlaybackActiveClass: playbackActiveClass,
		Narrator:            parsedBook.MediaNarrator,
		DurationSeconds:     parsedBook.MediaDurationSeconds,
		Segments:            []MediaOverlaySegment{},
	}

	manifestMap := make(map[string]opfItem)
	for _, item := range r.opf.Manifest.Items {
		manifestMap[item.ID] = item
	}

	// Build refined duration map: overlayID -> durationSeconds
	overlayDurations := make(map[string]float64)
	for _, m := range r.opf.Metadata.Metas {
		prop := strings.ToLower(strings.TrimSpace(m.Property))
		name := strings.ToLower(strings.TrimSpace(m.Name))
		if prop == "media:duration" || name == "media:duration" {
			ref := strings.TrimPrefix(strings.TrimSpace(m.Refines), "#")
			val := strings.TrimSpace(m.Value)
			if val == "" {
				val = strings.TrimSpace(m.Content)
			}
			if ref != "" && val != "" {
				if sec, err := ParseSMILClock(val); err == nil && sec > 0 {
					overlayDurations[ref] = sec
				}
			}
		}
	}

	type spineChapterInfo struct {
		chapterIndex   int
		fullPath       string
		relHref        string
		mediaOverlayID string
		idTextMap      map[string]string
	}

	isPrePaginated := r.IsPrePaginated()
	var spineChapters []spineChapterInfo
	chapterByPath := make(map[string]*spineChapterInfo)
	chapterIndex := 0

	for _, itemref := range r.opf.Spine.Itemrefs {
		item, exists := manifestMap[itemref.IDRef]
		if !exists {
			continue
		}
		mediaType := strings.ToLower(item.MediaType)
		if !strings.Contains(mediaType, "html") && !strings.Contains(mediaType, "xhtml") && !strings.Contains(mediaType, "xml") {
			continue
		}

		fullPath := item.Href
		if r.rootBase != "" {
			fullPath = path.Join(r.rootBase, item.Href)
		}
		fullPath = path.Clean(fullPath)

		zf := r.findFile(fullPath)
		if zf == nil {
			continue
		}
		rawHTML, err := readZipFile(zf)
		if err != nil {
			continue
		}
		rawHTMLStr := string(rawHTML)
		_, plaintext := extractTextFromHTMLWithContext(rawHTMLStr, parsedBook.Title, fullPath)
		if !isPrePaginated && strings.TrimSpace(plaintext) == "" {
			continue
		}

		chapterIndex++
		info := spineChapterInfo{
			chapterIndex:   chapterIndex,
			fullPath:       fullPath,
			relHref:        path.Clean(item.Href),
			mediaOverlayID: strings.TrimSpace(item.MediaOverlay),
			idTextMap:      extractElementTextByIDMap(rawHTMLStr),
		}
		spineChapters = append(spineChapters, info)
	}

	for i := range spineChapters {
		ch := &spineChapters[i]
		chapterByPath[strings.ToLower(ch.fullPath)] = ch
		chapterByPath[strings.ToLower(ch.relHref)] = ch
		chapterByPath[strings.ToLower(path.Base(ch.fullPath))] = ch
	}

	type rawOverlayEntry struct {
		overlayID string
		smilPath  string
		defChap   *spineChapterInfo
		pars      []SMILPar
	}

	var entries []rawOverlayEntry
	visitedOverlays := make(map[string]bool)

	// 1. Collect SMIL overlays referenced directly by spine items in spine order
	for i := range spineChapters {
		ch := &spineChapters[i]
		if ch.mediaOverlayID == "" {
			continue
		}
		smilItem, ok := manifestMap[ch.mediaOverlayID]
		if !ok {
			continue
		}
		smilFullPath := smilItem.Href
		if r.rootBase != "" {
			smilFullPath = path.Join(r.rootBase, smilItem.Href)
		}
		smilFullPath = path.Clean(smilFullPath)
		zf := r.findFile(smilFullPath)
		if zf == nil {
			continue
		}
		smilBytes, err := readZipFile(zf)
		if err != nil {
			continue
		}
		pars, err := ParseSMIL(smilBytes)
		if err != nil || len(pars) == 0 {
			continue
		}
		visitedOverlays[smilItem.ID] = true
		entries = append(entries, rawOverlayEntry{
			overlayID: smilItem.ID,
			smilPath:  smilFullPath,
			defChap:   ch,
			pars:      pars,
		})
	}

	// 2. Also collect any unreferenced application/smil+xml items in manifest order
	for _, item := range r.opf.Manifest.Items {
		if visitedOverlays[item.ID] {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.MediaType), "application/smil+xml") &&
			!strings.HasSuffix(strings.ToLower(item.Href), ".smil") {
			continue
		}
		smilFullPath := item.Href
		if r.rootBase != "" {
			smilFullPath = path.Join(r.rootBase, item.Href)
		}
		smilFullPath = path.Clean(smilFullPath)
		zf := r.findFile(smilFullPath)
		if zf == nil {
			continue
		}
		smilBytes, err := readZipFile(zf)
		if err != nil {
			continue
		}
		pars, err := ParseSMIL(smilBytes)
		if err != nil || len(pars) == 0 {
			continue
		}
		visitedOverlays[item.ID] = true
		entries = append(entries, rawOverlayEntry{
			overlayID: item.ID,
			smilPath:  smilFullPath,
			defChap:   nil,
			pars:      pars,
		})
	}

	if len(entries) == 0 {
		return manifest, nil
	}

	manifest.HasMediaOverlays = true

	// Build segments and compute cumulative global timeline across tracks
	var prevAudioPath string
	var prevOverlayID string
	var currentTrackOffset float64
	var currentTrackMaxEnd float64

	for _, entry := range entries {
		for _, par := range entry.pars {
			docPath, fragID := splitHrefAndFragment(par.TextSrc, entry.smilPath)
			var targetChap *spineChapterInfo
			if docPath != "" {
				if c, ok := chapterByPath[strings.ToLower(docPath)]; ok {
					targetChap = c
				} else if c, ok := chapterByPath[strings.ToLower(path.Base(docPath))]; ok {
					targetChap = c
				}
			}
			if targetChap == nil {
				targetChap = entry.defChap
			}
			if targetChap == nil && len(spineChapters) > 0 {
				targetChap = &spineChapters[0]
			}
			if targetChap == nil {
				continue
			}

			var resolvedAudio string
			if par.AudioSrc != "" {
				resolvedAudio, _ = splitHrefAndFragment(par.AudioSrc, entry.smilPath)
			}

			// Determine if this clip starts a new audio track whose local timestamps reset to ~0
			if resolvedAudio != "" && prevAudioPath != "" && !strings.EqualFold(resolvedAudio, prevAudioPath) {
				if par.ClipBegin < currentTrackMaxEnd-0.05 {
					trackDur := currentTrackMaxEnd
					if prevOverlayID != "" && prevOverlayID != entry.overlayID {
						if opfDur, ok := overlayDurations[prevOverlayID]; ok && opfDur >= currentTrackMaxEnd {
							trackDur = opfDur
						}
					}
					currentTrackOffset += trackDur
					currentTrackMaxEnd = 0
				}
			}

			clipBegin := par.ClipBegin
			clipEnd := par.ClipEnd
			if clipEnd < clipBegin {
				clipEnd = clipBegin
			}
			if clipEnd > currentTrackMaxEnd {
				currentTrackMaxEnd = clipEnd
			}
			if resolvedAudio != "" {
				prevAudioPath = resolvedAudio
			}
			prevOverlayID = entry.overlayID

			globalBegin := currentTrackOffset + clipBegin
			globalEnd := currentTrackOffset + clipEnd

			var fragText string
			if fragID != "" && targetChap.idTextMap != nil {
				fragText = targetChap.idTextMap[fragID]
			}

			manifest.Segments = append(manifest.Segments, MediaOverlaySegment{
				ChapterIndex: targetChap.chapterIndex,
				ChapterHref:  targetChap.fullPath,
				OverlayID:    entry.overlayID,
				ParID:        par.ID,
				FragmentID:   fragID,
				Text:         fragText,
				AudioSrc:     resolvedAudio,
				ClipBegin:    clipBegin,
				ClipEnd:      clipEnd,
				GlobalBegin:  globalBegin,
				GlobalEnd:    globalEnd,
			})
		}
	}

	if manifest.DurationSeconds == nil && len(manifest.Segments) > 0 {
		maxEnd := manifest.Segments[len(manifest.Segments)-1].GlobalEnd
		if maxEnd > 0 {
			manifest.DurationSeconds = &maxEnd
		}
	}

	return manifest, nil
}
