package epub_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/shelfd/shelfd/internal/epub"
)

func TestUpdateMetadata_Success(t *testing.T) {
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opfXML := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="BookId">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>Original Title</dc:title>
    <dc:creator opf:role="aut">Unknown</dc:creator>
    <dc:identifier id="BookId">urn:uuid:12345-67890</dc:identifier>
    <dc:date>1984-07-01</dc:date>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="cover-image" href="images/cover.jpg" media-type="image/jpeg"/>
    <item id="ch1" href="text/ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`

	ch1HTML := `<!DOCTYPE html><html><head><title>Chapter 1</title></head><body><p>Hello World</p></body></html>`
	coverJPEG := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	epubBytes := createTestEPUB(map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"OEBPS/content.opf":      []byte(opfXML),
		"OEBPS/text/ch01.xhtml":  []byte(ch1HTML),
		"OEBPS/images/cover.jpg": coverJPEG,
	})

	tmpDir := t.TempDir()
	tmpEPUB := filepath.Join(tmpDir, "test.epub")
	if err := os.WriteFile(tmpEPUB, epubBytes, 0644); err != nil {
		t.Fatalf("writing temp epub: %v", err)
	}

	seriesName := "Hainish Cycle"
	seqNum := 4.0
	desc := "A pioneering work of speculative fiction."
	pub := "Ace Books"
	lang := "en"

	update := epub.MetadataUpdate{
		Title:          "The Left Hand of Darkness",
		Authors:        []string{"Ursula K. Le Guin"},
		Series:         &seriesName,
		SequenceNumber: &seqNum,
		Description:    &desc,
		Publisher:      &pub,
		Language:       &lang,
		Genres:         []string{"Science Fiction", "Speculative Fiction"},
	}

	if err := epub.UpdateMetadata(tmpEPUB, update); err != nil {
		t.Fatalf("UpdateMetadata failed: %v", err)
	}

	// 1. Verify ZIP properties (mimetype uncompressed first)
	zr, err := zip.OpenReader(tmpEPUB)
	if err != nil {
		t.Fatalf("failed to open updated epub as zip: %v", err)
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		t.Fatalf("zip has 0 entries")
	}
	firstEntry := zr.File[0]
	if firstEntry.Name != "mimetype" {
		t.Errorf("expected first entry to be 'mimetype', got %q", firstEntry.Name)
	}
	if firstEntry.Method != zip.Store {
		t.Errorf("expected mimetype to be uncompressed (Store), got method %d", firstEntry.Method)
	}

	// 2. Open via EPUB reader and parse book
	reader, err := epub.Open(tmpEPUB)
	if err != nil {
		t.Fatalf("epub.Open failed on updated epub: %v", err)
	}
	defer reader.Close()

	parsed, err := reader.ParseBook()
	if err != nil {
		t.Fatalf("ParseBook failed: %v", err)
	}

	if parsed.Title != "The Left Hand of Darkness" {
		t.Errorf("expected Title %q, got %q", "The Left Hand of Darkness", parsed.Title)
	}
	if len(parsed.Authors) != 1 || parsed.Authors[0].Name != "Ursula K. Le Guin" {
		t.Errorf("expected Author 'Ursula K. Le Guin', got %+v", parsed.Authors)
	}
	if parsed.Series == nil || parsed.Series.Name != "Hainish Cycle" {
		t.Errorf("expected Series 'Hainish Cycle', got %+v", parsed.Series)
	}
	if parsed.Series == nil || parsed.Series.SequenceNumber == nil || *parsed.Series.SequenceNumber != 4.0 {
		t.Errorf("expected Series SequenceNumber 4.0, got %+v", parsed.Series)
	}
	if parsed.Description != desc {
		t.Errorf("expected Description %q, got %q", desc, parsed.Description)
	}
	if parsed.Publisher != pub {
		t.Errorf("expected Publisher %q, got %q", pub, parsed.Publisher)
	}
	if parsed.Language != lang {
		t.Errorf("expected Language %q, got %q", lang, parsed.Language)
	}
	if len(parsed.Genres) != 2 || parsed.Genres[0] != "Science Fiction" || parsed.Genres[1] != "Speculative Fiction" {
		t.Errorf("expected 2 genres, got %+v", parsed.Genres)
	}
	if parsed.Identifier != "urn:uuid:12345-67890" {
		t.Errorf("expected Identifier preserved, got %q", parsed.Identifier)
	}

	// 3. Verify cover still extractable
	coverBytes, ext, err := reader.ExtractCoverImage()
	if err != nil {
		t.Fatalf("ExtractCoverImage failed: %v", err)
	}
	if len(coverBytes) != len(coverJPEG) || ext != ".jpg" {
		t.Errorf("cover image corrupted, got len %d, ext %s", len(coverBytes), ext)
	}

	// 4. Verify chapter content still extractable
	chapters, err := reader.ExtractChapters()
	if err != nil {
		t.Fatalf("ExtractChapters failed: %v", err)
	}
	if len(chapters) == 0 || !bytes.Contains([]byte(chapters[0].ContentPlain), []byte("Hello World")) {
		t.Errorf("chapter content missing or corrupted: %+v", chapters)
	}
}

func TestUpdateMetadata_EscapingAndNewSeries(t *testing.T) {
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opfXML := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Old</dc:title>
  </metadata>
  <manifest>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="ch1"/></spine>
</package>`

	ch1HTML := `<!DOCTYPE html><html><body><p>Text</p></body></html>`

	epubBytes := createTestEPUB(map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"content.opf":            []byte(opfXML),
		"ch1.xhtml":              []byte(ch1HTML),
	})

	tmpEPUB := filepath.Join(t.TempDir(), "bare.epub")
	_ = os.WriteFile(tmpEPUB, epubBytes, 0644)

	series := "Dune & Other Worlds"
	seq := 1.5
	update := epub.MetadataUpdate{
		Title:          "Dune: Deluxe <Special> \"Edition\"",
		Authors:        []string{"Frank Herbert & Brian Herbert"},
		Series:         &series,
		SequenceNumber: &seq,
	}

	if err := epub.UpdateMetadata(tmpEPUB, update); err != nil {
		t.Fatalf("UpdateMetadata failed: %v", err)
	}

	reader, err := epub.Open(tmpEPUB)
	if err != nil {
		t.Fatalf("epub.Open failed: %v", err)
	}
	defer reader.Close()

	parsed, err := reader.ParseBook()
	if err != nil {
		t.Fatalf("ParseBook failed: %v", err)
	}

	if parsed.Title != "Dune: Deluxe <Special> \"Edition\"" {
		t.Errorf("expected Title with special characters unescaped, got %q", parsed.Title)
	}
	if len(parsed.Authors) != 1 || parsed.Authors[0].Name != "Frank Herbert & Brian Herbert" {
		t.Errorf("expected Author with &, got %+v", parsed.Authors)
	}
	if parsed.Series == nil || parsed.Series.Name != "Dune & Other Worlds" {
		t.Errorf("expected Series with &, got %+v", parsed.Series)
	}
	if parsed.Series == nil || parsed.Series.SequenceNumber == nil || *parsed.Series.SequenceNumber != 1.5 {
		t.Errorf("expected SeqNum 1.5, got %+v", parsed.Series)
	}
}

