package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MetadataUpdate defines the user-edited fields to update within an EPUB package.
type MetadataUpdate struct {
	Title          string
	Authors        []string
	Series         *string
	SequenceNumber *float64
	Description    *string
	Publisher      *string
	Language       *string
	Genres         []string
}

// UpdateMetadata rewrites the package metadata inside an EPUB archive on disk,
// preserving the uncompressed mimetype as the first entry, maintaining Audiobookshelf
// and Calibre series tags, and leaving all other archive entries untouched.
func UpdateMetadata(epubPath string, update MetadataUpdate) error {
	// Guard against path traversal
	cleanPath := filepath.Clean(epubPath)

	zr, err := zip.OpenReader(cleanPath)
	if err != nil {
		return fmt.Errorf("opening epub archive: %w", err)
	}
	defer zr.Close()

	// Guard against Zip Slip
	for _, f := range zr.File {
		if err := validateZipEntry(f.Name); err != nil {
			return err
		}
	}

	// Locate container.xml
	var containerFile *zip.File
	for _, f := range zr.File {
		if strings.TrimPrefix(path.Clean(strings.ReplaceAll(f.Name, "\\", "/")), "/") == "META-INF/container.xml" {
			containerFile = f
			break
		}
	}
	if containerFile == nil {
		return fmt.Errorf("META-INF/container.xml not found")
	}

	containerData, err := readZipFile(containerFile)
	if err != nil {
		return fmt.Errorf("reading container.xml: %w", err)
	}

	rootPath, err := parseContainerXML(containerData)
	if err != nil {
		return fmt.Errorf("parsing container.xml: %w", err)
	}

	// Locate OPF package file
	var opfFile *zip.File
	for _, f := range zr.File {
		if f.Name == rootPath {
			opfFile = f
			break
		}
	}
	if opfFile == nil {
		return fmt.Errorf("package file not found at %s", rootPath)
	}

	opfData, err := readZipFile(opfFile)
	if err != nil {
		return fmt.Errorf("reading package file: %w", err)
	}

	parsedOPF, err := parseOPF(opfData)
	if err != nil {
		return fmt.Errorf("parsing opf package: %w", err)
	}

	newOPFData, err := buildUpdatedOPF(opfData, parsedOPF, update)
	if err != nil {
		return fmt.Errorf("building updated opf: %w", err)
	}

	// Create temp output file in the same directory for atomic rename
	tmpPath := cleanPath + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("creating temp epub file: %w", err)
	}
	defer func() {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	zw := zip.NewWriter(tmpFile)

	// 1. Entry 1: Write uncompressed mimetype
	mimetypeHeader := &zip.FileHeader{
		Name:     "mimetype",
		Method:   zip.Store,
		Modified: time.Now(),
	}
	mw, err := zw.CreateHeader(mimetypeHeader)
	if err != nil {
		return fmt.Errorf("creating mimetype entry: %w", err)
	}
	if _, err := mw.Write([]byte("application/epub+zip")); err != nil {
		return fmt.Errorf("writing mimetype entry: %w", err)
	}

	// 2. Stream all other entries from original archive
	cleanRootPath := strings.TrimPrefix(path.Clean(strings.ReplaceAll(rootPath, "\\", "/")), "/")
	for _, entry := range zr.File {
		if entry.Name == "mimetype" || entry.Name == "" {
			continue
		}

		if entry.Name == rootPath || strings.TrimPrefix(path.Clean(strings.ReplaceAll(entry.Name, "\\", "/")), "/") == cleanRootPath {
			// Write updated OPF
			h := &zip.FileHeader{
				Name:     rootPath,
				Method:   zip.Deflate,
				Modified: time.Now(),
			}
			w, err := zw.CreateHeader(h)
			if err != nil {
				return fmt.Errorf("creating opf entry in new zip: %w", err)
			}
			if _, err := w.Write(newOPFData); err != nil {
				return fmt.Errorf("writing updated opf entry: %w", err)
			}
			continue
		}

		// Verbatim copy of other entries
		h := &zip.FileHeader{
			Name:     entry.Name,
			Method:   entry.Method,
			Modified: entry.Modified,
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return fmt.Errorf("creating entry %s in new zip: %w", entry.Name, err)
		}

		rc, err := entry.Open()
		if err != nil {
			return fmt.Errorf("opening entry %s from source zip: %w", entry.Name, err)
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("copying entry %s: %w", entry.Name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("closing zip writer: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic replace original file
	if err := os.Rename(tmpPath, cleanPath); err != nil {
		return fmt.Errorf("replacing original epub file: %w", err)
	}

	return nil
}

func buildUpdatedOPF(rawOPF []byte, parsed *opfPackage, update MetadataUpdate) ([]byte, error) {
	// Sanitize corrupted preambles before <package if present
	if pkgIdx := findRootElement(rawOPF, "package"); pkgIdx > 0 {
		testXML := append(append([]byte(nil), rawOPF[:pkgIdx]...), []byte("<test/>")...)
		var dummy struct{}
		if err := xml.NewDecoder(bytes.NewReader(testXML)).Decode(&dummy); err != nil {
			rawOPF = append([]byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"), rawOPF[pkgIdx:]...)
		}
	}

	// Find <metadata and </metadata>
	lower := bytes.ToLower(rawOPF)
	startMetaIdx := bytes.Index(lower, []byte("<metadata"))
	if startMetaIdx == -1 {
		return nil, fmt.Errorf("no <metadata> tag found in opf")
	}

	endMetaTag := []byte("</metadata>")
	endMetaIdx := bytes.Index(lower, endMetaTag)
	if endMetaIdx == -1 {
		return nil, fmt.Errorf("no </metadata> tag found in opf")
	}
	endMetaIdx += len(endMetaTag)

	// Construct updated <metadata> XML block
	var buf bytes.Buffer
	buf.WriteString("  <metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\" xmlns:opf=\"http://www.idpf.org/2007/opf\">\n")

	// 1. Title
	title := strings.TrimSpace(update.Title)
	if title == "" {
		title = "Untitled"
	}
	buf.WriteString(fmt.Sprintf("    <dc:title>%s</dc:title>\n", escapeXML(title)))

	// 2. Authors (Creators)
	authors := update.Authors
	if len(authors) == 0 {
		authors = []string{"Unknown"}
	}
	for i, author := range authors {
		name := strings.TrimSpace(author)
		if name == "" {
			continue
		}
		creatorID := fmt.Sprintf("creator%02d", i+1)
		buf.WriteString(fmt.Sprintf("    <dc:creator id=\"%s\" opf:role=\"aut\">%s</dc:creator>\n", creatorID, escapeXML(name)))
		buf.WriteString(fmt.Sprintf("    <meta refines=\"#%s\" property=\"role\">aut</meta>\n", creatorID))
	}

	// 3. Description
	if update.Description != nil && strings.TrimSpace(*update.Description) != "" {
		buf.WriteString(fmt.Sprintf("    <dc:description>%s</dc:description>\n", escapeXML(strings.TrimSpace(*update.Description))))
	}

	// 4. Publisher
	if update.Publisher != nil && strings.TrimSpace(*update.Publisher) != "" {
		buf.WriteString(fmt.Sprintf("    <dc:publisher>%s</dc:publisher>\n", escapeXML(strings.TrimSpace(*update.Publisher))))
	}

	// 5. Language
	lang := "en"
	if update.Language != nil && strings.TrimSpace(*update.Language) != "" {
		lang = strings.TrimSpace(*update.Language)
	}
	buf.WriteString(fmt.Sprintf("    <dc:language>%s</dc:language>\n", escapeXML(lang)))

	// 6. Genres (dc:subject)
	for _, genre := range update.Genres {
		clean := strings.TrimSpace(genre)
		if clean != "" {
			buf.WriteString(fmt.Sprintf("    <dc:subject>%s</dc:subject>\n", escapeXML(clean)))
		}
	}

	// 7. Series (Dual-tagged for EPUB 3 and Calibre/ABS)
	if update.Series != nil && strings.TrimSpace(*update.Series) != "" {
		seriesName := strings.TrimSpace(*update.Series)
		buf.WriteString(fmt.Sprintf("    <meta property=\"belongs-to-collection\" id=\"shelfd_series_01\">%s</meta>\n", escapeXML(seriesName)))
		buf.WriteString("    <meta refines=\"#shelfd_series_01\" property=\"collection-type\">series</meta>\n")
		buf.WriteString(fmt.Sprintf("    <meta name=\"calibre:series\" content=\"%s\"/>\n", escapeAttr(seriesName)))

		if update.SequenceNumber != nil {
			seqStr := strconv.FormatFloat(*update.SequenceNumber, 'f', -1, 64)
			buf.WriteString(fmt.Sprintf("    <meta refines=\"#shelfd_series_01\" property=\"group-position\">%s</meta>\n", seqStr))
			buf.WriteString(fmt.Sprintf("    <meta name=\"calibre:series_index\" content=\"%s\"/>\n", seqStr))
		}
	}

	// 8. Preserve existing identifiers
	for _, id := range parsed.Metadata.Identifiers {
		if strings.TrimSpace(id.Value) != "" {
			if id.ID != "" {
				buf.WriteString(fmt.Sprintf("    <dc:identifier id=\"%s\">%s</dc:identifier>\n", escapeAttr(id.ID), escapeXML(id.Value)))
			} else {
				buf.WriteString(fmt.Sprintf("    <dc:identifier>%s</dc:identifier>\n", escapeXML(id.Value)))
			}
		}
	}

	// 9. Preserve existing dates
	for _, d := range parsed.Metadata.Dates {
		if strings.TrimSpace(d) != "" {
			buf.WriteString(fmt.Sprintf("    <dc:date>%s</dc:date>\n", escapeXML(strings.TrimSpace(d))))
		}
	}

	// 10. Preserve cover meta tags
	for _, m := range parsed.Metadata.Metas {
		if m.Name == "cover" && strings.TrimSpace(m.Content) != "" {
			buf.WriteString(fmt.Sprintf("    <meta name=\"cover\" content=\"%s\"/>\n", escapeAttr(m.Content)))
		}
	}

	buf.WriteString("  </metadata>")

	// Splice the new metadata block into the original OPF
	var result bytes.Buffer
	result.Write(rawOPF[:startMetaIdx])
	result.Write(buf.Bytes())
	result.Write(rawOPF[endMetaIdx:])

	return result.Bytes(), nil
}

func escapeXML(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func escapeAttr(s string) string {
	escaped := escapeXML(s)
	return strings.ReplaceAll(escaped, "\"", "&quot;")
}
