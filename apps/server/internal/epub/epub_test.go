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
	testCases := []struct {
		name      string
		entryName string
	}{
		{"parent traversal", "../../../../etc/passwd"},
		{"leading slash with traversal", "/../../../../etc/passwd"},
		{"root parent traversal", "/.."},
		{"nested parent traversal", "folder/../../../etc/passwd"},
		{"windows backslash traversal", `..\..\windows\win.ini`},
		{"nested backslash traversal", `folder\..\..\secret.txt`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string][]byte{
				tc.entryName:             []byte("malicious content"),
				"META-INF/container.xml": []byte("<container/>"),
			}

			epubBytes := createTestEPUB(files)
			tempDir := t.TempDir()
			epubPath := filepath.Join(tempDir, "malicious.epub")
			if err := os.WriteFile(epubPath, epubBytes, 0644); err != nil {
				t.Fatalf("writing malicious epub: %v", err)
			}

			_, err := epub.Open(epubPath)
			if err == nil {
				t.Fatalf("expected error opening zip slip archive for %s, got nil", tc.entryName)
			}
			if !strings.Contains(err.Error(), "illegal path traversal") {
				t.Errorf("expected illegal path traversal error for %s, got: %v", tc.entryName, err)
			}
		})
	}
}

func TestZipRootSlashEntryAllowed(t *testing.T) {
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opfXML := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="BookId">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Root Slash Book</dc:title>
    <dc:creator>Author Name</dc:creator>
  </metadata>
  <manifest>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`

	files := map[string][]byte{
		"/":                      []byte{},
		"":                       []byte{},
		"META-INF/container.xml": []byte(containerXML),
		"content.opf":            []byte(opfXML),
		"ch1.xhtml":              []byte("<html><body>Chapter 1</body></html>"),
	}

	epubBytes := createTestEPUB(files)
	tempDir := t.TempDir()
	epubPath := filepath.Join(tempDir, "rootslash.epub")
	if err := os.WriteFile(epubPath, epubBytes, 0644); err != nil {
		t.Fatalf("writing test epub: %v", err)
	}

	reader, err := epub.Open(epubPath)
	if err != nil {
		t.Fatalf("expected epub.Open to succeed for archive with '/' and empty entries, got: %v", err)
	}
	defer reader.Close()

	book, err := reader.ParseBook()
	if err != nil {
		t.Fatalf("expected ParseBook to succeed, got: %v", err)
	}
	if book.Title != "Root Slash Book" {
		t.Errorf("expected title 'Root Slash Book', got: %q", book.Title)
	}
}

func TestExtractChapters_PreservesMarkdownFormatting(t *testing.T) {
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opfXML := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="BookId">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Formatting Test</dc:title>
  </metadata>
  <manifest>
    <item id="ch1" href="ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`

	ch1HTML := `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Chapter 2: The Dixiecrat Dilemma</title></head>
  <body>
    <h1>Chapter 2: The Dixiecrat Dilemma</h1>
    <p>Demythologizing our past is necessary if we are to understand our present.</p>
    <h2 class="h2">
      <span class="line">The reign of the Dixiecrats</span>
    </h2>
    <p class="noindent">
      During much of the twentieth century, the Democratic Party’s rule was hegemonic.
      Democratic senator Theodore Bilbo was <em>chillingly</em> blunt: <strong>Red-blooded men know what I mean.</strong>
      <a id="ch02endnote_5" href="endnotes.xhtml#ch02endnote-5">
        <sup class="sup">5</sup>
      </a>
    </p>
    <blockquote>
      <p>A house divided against itself cannot stand.</p>
    </blockquote>
    <hr/>
    <p>The post-war era transformed politics.</p>
  </body>
</html>`

	files := map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"content.opf":            []byte(opfXML),
		"ch01.xhtml":             []byte(ch1HTML),
	}

	epubBytes := createTestEPUB(files)
	tempDir := t.TempDir()
	epubPath := filepath.Join(tempDir, "formatting_test.epub")
	if err := os.WriteFile(epubPath, epubBytes, 0644); err != nil {
		t.Fatalf("write epub file: %v", err)
	}

	reader, err := epub.Open(epubPath)
	if err != nil {
		t.Fatalf("epub.Open failed: %v", err)
	}
	defer reader.Close()

	chapters, err := reader.ExtractChapters()
	if err != nil {
		t.Fatalf("extract chapters error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}

	content := chapters[0].ContentPlain

	// 1. Heading preserved with Markdown ##
	if !strings.Contains(content, "## The reign of the Dixiecrats") {
		t.Errorf("expected ## The reign of the Dixiecrats, got:\n%s", content)
	}

	// 2. Inline footnote [5] attached to sentence, NOT split into its own paragraph
	if strings.Contains(content, "\n\n5\n\n") || strings.Contains(content, "\n5\n") {
		t.Errorf("footnote 5 was split into separate line:\n%s", content)
	}
	if !strings.Contains(content, "[5]") {
		t.Errorf("expected inline [5] footnote marker in content, got:\n%s", content)
	}

	// 3. Emphasis and bold preserved
	if !strings.Contains(content, "*chillingly*") {
		t.Errorf("expected *chillingly* italic formatting, got:\n%s", content)
	}
	if !strings.Contains(content, "**Red-blooded men know what I mean.**") {
		t.Errorf("expected **bold** formatting, got:\n%s", content)
	}

	// 4. Blockquote preserved with >
	if !strings.Contains(content, "> A house divided against itself cannot stand.") {
		t.Errorf("expected blockquote with > prefix, got:\n%s", content)
	}

	// 5. Divider preserved
	if !strings.Contains(content, "---") {
		t.Errorf("expected divider ---, got:\n%s", content)
	}
}

func TestFixedLayoutDetection(t *testing.T) {
	// EPUB 3 pre-paginated
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opfEPUB3 := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="BookId">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Coco Read-Along Storybook</dc:title>
    <meta property="rendition:layout">pre-paginated</meta>
    <meta property="rendition:spread">auto</meta>
    <meta property="rendition:orientation">auto</meta>
  </metadata>
  <manifest>
    <item id="cover" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="p1" href="text/p01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine page-progression-direction="ltr">
    <itemref idref="cover" properties="page-spread-right"/>
    <itemref idref="p1" properties="page-spread-left"/>
  </spine>
</package>`

	coverHTML := `<!DOCTYPE html><html><head><meta name="viewport" content="width=1024, height=768"/></head><body><img src="../images/cover.jpg"/></body></html>`
	p1HTML := `<!DOCTYPE html><html><head><meta name="viewport" content="width=1024, height=768"/></head><body><p>Story text</p></body></html>`

	epubBytes := createTestEPUB(map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"OEBPS/content.opf":      []byte(opfEPUB3),
		"OEBPS/text/cover.xhtml": []byte(coverHTML),
		"OEBPS/text/p01.xhtml":   []byte(p1HTML),
	})

	zr, err := zip.NewReader(bytes.NewReader(epubBytes), int64(len(epubBytes)))
	if err != nil {
		t.Fatalf("zip new reader error: %v", err)
	}

	reader, err := epub.NewReader(zr)
	if err != nil {
		t.Fatalf("epub new reader error: %v", err)
	}

	if !reader.IsPrePaginated() {
		t.Errorf("expected IsPrePaginated to return true for EPUB 3 pre-paginated")
	}

	book, err := reader.ParseBook()
	if err != nil {
		t.Fatalf("parse book error: %v", err)
	}

	if book.Layout != "pre-paginated" {
		t.Errorf("expected book.Layout == 'pre-paginated', got %s", book.Layout)
	}
	if book.RenditionSpread != "auto" {
		t.Errorf("expected rendition_spread == 'auto', got %s", book.RenditionSpread)
	}
	if book.PageProgressionDirection != "ltr" {
		t.Errorf("expected page_progression_direction == 'ltr', got %s", book.PageProgressionDirection)
	}

	chapters, err := reader.ExtractChapters()
	if err != nil {
		t.Fatalf("extract chapters error: %v", err)
	}

	// Cover has no text, but since it's pre-paginated, it MUST NOT be discarded!
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters (including image-only cover), got %d", len(chapters))
	}

	if chapters[0].PageWidth == nil || *chapters[0].PageWidth != 1024 {
		t.Errorf("expected cover width 1024, got %v", chapters[0].PageWidth)
	}
	if chapters[0].PageHeight == nil || *chapters[0].PageHeight != 768 {
		t.Errorf("expected cover height 768, got %v", chapters[0].PageHeight)
	}
	if chapters[0].PageSpread == nil || *chapters[0].PageSpread != "right" {
		t.Errorf("expected cover spread 'right', got %v", chapters[0].PageSpread)
	}
	if chapters[1].PageSpread == nil || *chapters[1].PageSpread != "left" {
		t.Errorf("expected p1 spread 'left', got %v", chapters[1].PageSpread)
	}
}

func TestExtractPageDimensions_SVGViewBox(t *testing.T) {
	svgHTML := `<!DOCTYPE html><html><head><title>Page</title></head><body>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 800" width="100%" height="100%">
  <image href="../images/p1.jpg" width="1200" height="800"/>
</svg></body></html>`

	w, h := epub.ExtractPageDimensions(svgHTML)
	if w == nil || *w != 1200 {
		t.Errorf("expected width 1200, got %v", w)
	}
	if h == nil || *h != 800 {
		t.Errorf("expected height 800, got %v", h)
	}
}

func TestRewritePageHTML(t *testing.T) {
	rawHTML := `<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="../css/styles.css" type="text/css"/>
</head>
<body>
  <div style="position: relative; width: 1024px; height: 768px;">
    <img src="../images/artwork.jpg" style="position: absolute; left: 0; top: 0; z-index: -1;" />
    <div style="position: absolute; left: 50px; top: 100px; z-index: 2;">
      <p>Hello Story</p>
    </div>
  </div>
</body>
</html>`

	bookID := "test-book-123"
	chapterHref := "OEBPS/text/page01.xhtml"
	pageWidth := 1024
	pageHeight := 768

	rewritten := epub.RewritePageHTML(rawHTML, bookID, chapterHref, &pageWidth, &pageHeight)

	// Verify relative asset URLs are rewritten to Shelfd API endpoints
	expectedCSS := `/api/v1/books/test-book-123/assets/OEBPS/css/styles.css`
	if !strings.Contains(rewritten, expectedCSS) {
		t.Errorf("expected rewritten CSS url %s, got:\n%s", expectedCSS, rewritten)
	}

	expectedImg := `/api/v1/books/test-book-123/assets/OEBPS/images/artwork.jpg`
	if !strings.Contains(rewritten, expectedImg) {
		t.Errorf("expected rewritten image url %s, got:\n%s", expectedImg, rewritten)
	}

	// Verify negative z-index is normalized to 1
	if strings.Contains(rewritten, "z-index: -1") {
		t.Errorf("expected negative z-index to be normalized to positive")
	}
	if !strings.Contains(rewritten, "z-index: 1") {
		t.Errorf("expected z-index: 1 for background image")
	}

	// Verify viewport meta tag is injected
	if !strings.Contains(rewritten, `width=1024, height=768`) {
		t.Errorf("expected injected viewport tag")
	}
}

