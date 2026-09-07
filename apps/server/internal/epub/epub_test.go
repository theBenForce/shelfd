package epub_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/epub"
)

// helper to create a test EPUB zip archive in memory
func createTestEPUB(files map[string][]byte) []byte {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// mimetype must be first
	mimetypeHeader := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	}
	w, _ := zw.CreateHeader(mimetypeHeader)
	w.Write([]byte("application/epub+zip"))

	for name, content := range files {
		if name == "mimetype" {
			continue
		}
		w, _ := zw.Create(name)
		w.Write(content)
	}
	zw.Close()
	return buf.Bytes()
}

func TestParseEPUB2Calibre(t *testing.T) {
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opfXML := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="BookId">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>Neuromancer</dc:title>
    <dc:creator opf:role="aut">William Gibson</dc:creator>
    <dc:creator opf:role="edt">Terry Carr</dc:creator>
    <dc:subject>Cyberpunk</dc:subject>
    <dc:subject>Science Fiction</dc:subject>
    <dc:description>The sky above the port was the color of television...</dc:description>
    <dc:publisher>Ace Books</dc:publisher>
    <dc:language>en</dc:language>
    <dc:identifier id="BookId">urn:isbn:9780441569595</dc:identifier>
    <dc:date>1984-07-01</dc:date>
    <meta name="calibre:series" content="Sprawl"/>
    <meta name="calibre:series_index" content="1.5"/>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="cover-image" href="images/cover.jpg" media-type="image/jpeg"/>
    <item id="ch1" href="text/ch01.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="text/ch02.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
  </spine>
</package>`

	ch1HTML := `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Chapter 1: Chiba City Blues</title></head>
  <body>
    <h1>Chapter 1</h1>
    <p>The sky above the port was the color of television, tuned to a dead channel.</p>
    <p>&quot;It's not like I'm using,&quot; Case heard someone say.</p>
  </body>
</html>`

	ch2HTML := `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Chapter 2: Shopping</title></head>
  <body>
    <h2>Chapter 2</h2>
    <p>He had an appointment at the Ninsei clinic at four in the morning.</p>
  </body>
</html>`

	coverJPEG := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	files := map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"content.opf":            []byte(opfXML),
		"text/ch01.xhtml":        []byte(ch1HTML),
		"text/ch02.xhtml":        []byte(ch2HTML),
		"images/cover.jpg":       coverJPEG,
	}

	epubBytes := createTestEPUB(files)
	tempDir := t.TempDir()
	epubPath := filepath.Join(tempDir, "neuromancer.epub")
	if err := os.WriteFile(epubPath, epubBytes, 0644); err != nil {
		t.Fatalf("write epub file: %v", err)
	}

	reader, err := epub.Open(epubPath)
	if err != nil {
		t.Fatalf("epub.Open failed: %v", err)
	}
	defer reader.Close()

	book, err := reader.ParseBook()
	if err != nil {
		t.Fatalf("reader.ParseBook failed: %v", err)
	}

	if book.Title != "Neuromancer" {
		t.Errorf("expected title 'Neuromancer', got '%s'", book.Title)
	}
	if len(book.Authors) != 2 {
		t.Fatalf("expected 2 authors, got %d", len(book.Authors))
	}
	if book.Authors[0].Name != "William Gibson" || book.Authors[0].Role != "aut" {
		t.Errorf("unexpected primary author: %+v", book.Authors[0])
	}
	if book.Authors[1].Name != "Terry Carr" {
		t.Errorf("unexpected secondary author: %+v", book.Authors[1])
	}

	if len(book.Genres) != 2 || book.Genres[0] != "Cyberpunk" || book.Genres[1] != "Science Fiction" {
		t.Errorf("unexpected genres: %v", book.Genres)
	}

	if book.Publisher != "Ace Books" {
		t.Errorf("expected publisher 'Ace Books', got '%s'", book.Publisher)
	}
	if book.Language != "en" {
		t.Errorf("expected language 'en', got '%s'", book.Language)
	}
	if book.Identifier != "urn:isbn:9780441569595" {
		t.Errorf("expected identifier urn:isbn:9780441569595, got '%s'", book.Identifier)
	}

	// Calibre Series
	if book.Series == nil {
		t.Fatalf("expected series to be parsed from Calibre metadata")
	}
	if book.Series.Name != "Sprawl" {
		t.Errorf("expected series name 'Sprawl', got '%s'", book.Series.Name)
	}
	if book.Series.SequenceNumber == nil || *book.Series.SequenceNumber != 1.5 {
		t.Errorf("expected sequence number 1.5, got %v", book.Series.SequenceNumber)
	}

	// Cover Image
	coverData, coverExt, err := reader.ExtractCoverImage()
	if err != nil {
		t.Fatalf("extract cover image error: %v", err)
	}
	if len(coverData) != len(coverJPEG) || coverExt != ".jpg" {
		t.Errorf("unexpected cover data: len=%d, ext=%s", len(coverData), coverExt)
	}

	// Chapters
	chapters, err := reader.ExtractChapters()
	if err != nil {
		t.Fatalf("extract chapters error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}

	if chapters[0].Title == nil || !strings.Contains(*chapters[0].Title, "Chapter 1") {
		t.Errorf("expected chapter 1 title, got %v", chapters[0].Title)
	}
	if !strings.Contains(chapters[0].ContentPlain, "The sky above the port was the color of television") {
		t.Errorf("chapter 1 text missing expected content: %s", chapters[0].ContentPlain)
	}
	if !strings.Contains(chapters[0].ContentPlain, "\n\n") {
		t.Errorf("expected paragraph breaks in chapter 1 text: %s", chapters[0].ContentPlain)
	}

	if chapters[1].Title == nil || !strings.Contains(*chapters[1].Title, "Chapter 2") {
		t.Errorf("expected chapter 2 title, got %v", chapters[1].Title)
	}
	if !strings.Contains(chapters[1].ContentPlain, "Ninsei clinic") {
		t.Errorf("chapter 2 text missing expected content: %s", chapters[1].ContentPlain)
	}
}

func TestParseEPUB3Standard(t *testing.T) {
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opfXML := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Children of Dune</dc:title>
    <dc:creator id="creator1">Frank Herbert</dc:creator>
    <meta refines="#creator1" property="role" scheme="marc:relators">aut</meta>
    <dc:subject>Space Opera</dc:subject>
    <dc:language>en-US</dc:language>
    <dc:identifier id="pub-id">urn:uuid:12345678-1234-5678-1234-567812345678</dc:identifier>
    <meta property="belongs-to-collection" id="c01">Dune Chronicles</meta>
    <meta refines="#c01" property="collection-type">series</meta>
    <meta refines="#c01" property="group-position">3.0</meta>
  </metadata>
  <manifest>
    <item id="cover-img" href="assets/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="c1" href="c01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
  </spine>
</package>`

	c1HTML := `<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Chapter One</title></head>
  <body>
    <h1>Chapter One</h1>
    <p>The desert grew out from Arrakeen like a vast tawny beast.</p>
  </body>
</html>`

	coverPNG := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	files := map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"EPUB/package.opf":       []byte(opfXML),
		"EPUB/c01.xhtml":         []byte(c1HTML),
		"EPUB/assets/cover.png":  coverPNG,
	}

	epubBytes := createTestEPUB(files)
	tempDir := t.TempDir()
	epubPath := filepath.Join(tempDir, "children_of_dune.epub")
	os.WriteFile(epubPath, epubBytes, 0644)

	reader, err := epub.Open(epubPath)
	if err != nil {
		t.Fatalf("epub.Open failed: %v", err)
	}
	defer reader.Close()

	book, err := reader.ParseBook()
	if err != nil {
		t.Fatalf("ParseBook failed: %v", err)
	}

	if book.Title != "Children of Dune" {
		t.Errorf("expected title 'Children of Dune', got '%s'", book.Title)
	}

	// EPUB 3 Series Priority
	if book.Series == nil {
		t.Fatalf("expected EPUB 3 series")
	}
	if book.Series.Name != "Dune Chronicles" {
		t.Errorf("expected 'Dune Chronicles', got '%s'", book.Series.Name)
	}
	if book.Series.SequenceNumber == nil || *book.Series.SequenceNumber != 3.0 {
		t.Errorf("expected sequence 3.0, got %v", book.Series.SequenceNumber)
	}

	coverData, coverExt, err := reader.ExtractCoverImage()
	if err != nil {
		t.Fatalf("extract cover failed: %v", err)
	}
	if len(coverData) != len(coverPNG) || coverExt != ".png" {
		t.Errorf("unexpected cover png: len=%d, ext=%s", len(coverData), coverExt)
	}
}

func TestZipSlipRejection(t *testing.T) {
	// Attempt Zip Slip path traversal
	files := map[string][]byte{
		"../../../../etc/passwd": []byte("root:x:0:0:root:/root:/bin/bash"),
		"META-INF/container.xml": []byte("<container/>"),
	}

	epubBytes := createTestEPUB(files)
	tempDir := t.TempDir()
	epubPath := filepath.Join(tempDir, "malicious.epub")
	os.WriteFile(epubPath, epubBytes, 0644)

	_, err := epub.Open(epubPath)
	if err == nil {
		t.Fatalf("expected error opening zip slip archive, got nil")
	}
	if !strings.Contains(err.Error(), "illegal path traversal") {
		t.Errorf("expected illegal path traversal error, got: %v", err)
	}
}
