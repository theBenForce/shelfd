package epub

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"strconv"
	"strings"
)

type opfPackage struct {
	XMLName  xml.Name     `xml:"package"`
	Version  string       `xml:"version,attr"`
	Metadata opfMetadata  `xml:"metadata"`
	Manifest opfManifest  `xml:"manifest"`
	Spine    opfSpine     `xml:"spine"`
}

type opfMetadata struct {
	Titles       []opfElement `xml:"title"`
	Creators     []opfCreator `xml:"creator"`
	Subjects     []string     `xml:"subject"`
	Descriptions []string     `xml:"description"`
	Publishers   []string     `xml:"publisher"`
	Languages    []string     `xml:"language"`
	Identifiers  []opfElement `xml:"identifier"`
	Dates        []string     `xml:"date"`
	Metas        []opfMeta    `xml:"meta"`
}

type opfElement struct {
	ID    string `xml:"id,attr"`
	Value string `xml:",chardata"`
}

type opfCreator struct {
	ID    string `xml:"id,attr"`
	Role  string `xml:"role,attr"`
	Value string `xml:",chardata"`
}

type opfMeta struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Property string `xml:"property,attr"`
	Refines  string `xml:"refines,attr"`
	ID       string `xml:"id,attr"`
	Value    string `xml:",chardata"`
}

type opfManifest struct {
	Items []opfItem `xml:"item"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfSpine struct {
	Itemrefs []opfItemref `xml:"itemref"`
}

type opfItemref struct {
	IDRef string `xml:"idref,attr"`
}

func parseOPF(data []byte) (*opfPackage, error) {
	var pkg opfPackage
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&pkg); err != nil {
		return nil, fmt.Errorf("xml decode opf: %w", err)
	}
	return &pkg, nil
}

// ParseBook extracts the Dublin Core and series metadata into a ParsedBook struct.
func (r *Reader) ParseBook() (*ParsedBook, error) {
	if r.opf == nil {
		return nil, fmt.Errorf("no opf package loaded")
	}
	meta := r.opf.Metadata

	book := &ParsedBook{}

	// Title
	for _, t := range meta.Titles {
		clean := strings.TrimSpace(t.Value)
		if clean != "" {
			book.Title = clean
			break
		}
	}
	if book.Title == "" {
		book.Title = "Untitled"
	}

	// Roles map refined by EPUB 3 <meta refines="#id" property="role">
	refineRoles := make(map[string]string)
	for _, m := range meta.Metas {
		if strings.HasPrefix(m.Refines, "#") && m.Property == "role" {
			refineRoles[strings.TrimPrefix(m.Refines, "#")] = strings.TrimSpace(m.Value)
		}
	}

	// Authors
	for _, c := range meta.Creators {
		name := strings.TrimSpace(c.Value)
		if name == "" {
			continue
		}
		role := strings.TrimSpace(c.Role)
		if role == "" && c.ID != "" {
			role = refineRoles[c.ID]
		}
		if role == "" {
			role = "aut"
		}
		book.Authors = append(book.Authors, ParsedAuthor{
			Name: name,
			Role: role,
		})
	}

	// Genres / Subjects
	for _, s := range meta.Subjects {
		clean := strings.TrimSpace(s)
		if clean != "" {
			book.Genres = append(book.Genres, clean)
		}
	}

	// Description
	for _, d := range meta.Descriptions {
		clean := strings.TrimSpace(d)
		if clean != "" {
			book.Description = clean
			break
		}
	}

	// Publisher
	for _, p := range meta.Publishers {
		clean := strings.TrimSpace(p)
		if clean != "" {
			book.Publisher = clean
			break
		}
	}

	// Language
	for _, l := range meta.Languages {
		clean := strings.TrimSpace(l)
		if clean != "" {
			book.Language = clean
			break
		}
	}

	// Identifier
	for _, id := range meta.Identifiers {
		clean := strings.TrimSpace(id.Value)
		if clean != "" {
			book.Identifier = clean
			break
		}
	}

	// Date
	for _, d := range meta.Dates {
		clean := strings.TrimSpace(d)
		if clean != "" {
			book.PublishedDate = clean
			break
		}
	}

	// Series Resolution Priority:
	// 1. EPUB 3: belongs-to-collection + group-position
	// 2. Calibre / EPUB 2: calibre:series + calibre:series_index
	book.Series = r.resolveSeries()

	return book, nil
}

func (r *Reader) resolveSeries() *ParsedSeries {
	meta := r.opf.Metadata

	// 1. EPUB 3 Check
	for _, m := range meta.Metas {
		if m.Property == "belongs-to-collection" && strings.TrimSpace(m.Value) != "" {
			seriesName := strings.TrimSpace(m.Value)
			targetID := m.ID
			var seqNum *float64

			if targetID != "" {
				for _, sub := range meta.Metas {
					if sub.Refines == "#"+targetID && sub.Property == "group-position" {
						valStr := strings.TrimSpace(sub.Value)
						if f, err := strconv.ParseFloat(valStr, 64); err == nil {
							seqNum = &f
						}
						break
					}
				}
			}

			return &ParsedSeries{
				Name:           seriesName,
				SequenceNumber: seqNum,
			}
		}
	}

	// 2. Calibre / EPUB 2 Check
	var calibreSeries string
	var calibreIndex *float64
	for _, m := range meta.Metas {
		if m.Name == "calibre:series" && strings.TrimSpace(m.Content) != "" {
			calibreSeries = strings.TrimSpace(m.Content)
		}
		if m.Name == "calibre:series_index" && strings.TrimSpace(m.Content) != "" {
			if f, err := strconv.ParseFloat(strings.TrimSpace(m.Content), 64); err == nil {
				calibreIndex = &f
			}
		}
	}

	if calibreSeries != "" {
		return &ParsedSeries{
			Name:           calibreSeries,
			SequenceNumber: calibreIndex,
		}
	}

	return nil
}

// ExtractCoverImage locates and returns the raw bytes and extension of the cover image.
func (r *Reader) ExtractCoverImage() ([]byte, string, error) {
	if r.opf == nil {
		return nil, "", fmt.Errorf("no opf package loaded")
	}

	coverHref := ""
	coverMediaType := ""

	// 1. EPUB 3 Check: manifest item with properties="cover-image"
	for _, item := range r.opf.Manifest.Items {
		if strings.Contains(item.Properties, "cover-image") {
			coverHref = item.Href
			coverMediaType = item.MediaType
			break
		}
	}

	// 2. EPUB 2 Check: <meta name="cover" content="id">
	if coverHref == "" {
		for _, m := range r.opf.Metadata.Metas {
			if m.Name == "cover" && m.Content != "" {
				coverID := m.Content
				for _, item := range r.opf.Manifest.Items {
					if item.ID == coverID {
						coverHref = item.Href
						coverMediaType = item.MediaType
						break
					}
				}
				break
			}
		}
	}

	// 3. Heuristic Fallback
	if coverHref == "" {
		for _, item := range r.opf.Manifest.Items {
			if strings.HasPrefix(item.MediaType, "image/") {
				idLower := strings.ToLower(item.ID)
				hrefLower := strings.ToLower(item.Href)
				if strings.Contains(idLower, "cover") || strings.Contains(hrefLower, "cover") {
					coverHref = item.Href
					coverMediaType = item.MediaType
					break
				}
			}
		}
	}

	if coverHref == "" {
		return nil, "", fmt.Errorf("no cover image found in epub")
	}

	// Resolve path relative to OPF location
	fullImagePath := coverHref
	if r.rootBase != "" {
		fullImagePath = path.Join(r.rootBase, coverHref)
	}

	imgFile := r.findFile(fullImagePath)
	if imgFile == nil {
		return nil, "", fmt.Errorf("cover file %s not found in zip archive", fullImagePath)
	}

	data, err := readZipFile(imgFile)
	if err != nil {
		return nil, "", fmt.Errorf("reading cover image: %w", err)
	}

	ext := ".jpg"
	if strings.Contains(coverMediaType, "png") || strings.HasSuffix(strings.ToLower(coverHref), ".png") {
		ext = ".png"
	} else if strings.Contains(coverMediaType, "webp") || strings.HasSuffix(strings.ToLower(coverHref), ".webp") {
		ext = ".webp"
	}

	return data, ext, nil
}
