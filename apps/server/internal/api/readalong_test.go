package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/shelfd/shelfd/internal/api"
)

func buildReadalongEPUB(t *testing.T, title, author string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	mh := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	mw, _ := zw.CreateHeader(mh)
	_, _ = mw.Write([]byte("application/epub+zip"))

	cw, _ := zw.Create("META-INF/container.xml")
	_, _ = cw.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))

	creatorTag := ""
	if author != "" {
		creatorTag = `<dc:creator>` + author + `</dc:creator>`
	}

	opfw, _ := zw.Create("OEBPS/content.opf")
	_, _ = opfw.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:isbn:9780001112223</dc:identifier>
    <dc:title>` + title + `</dc:title>
    ` + creatorTag + `
    <meta property="rendition:layout">pre-paginated</meta>
    <meta property="media:active-class">-epub-media-overlay-active</meta>
    <meta property="media:playback-active-class">-epub-media-overlay-playing</meta>
    <meta property="media:duration" refines="#ov1">0:00:04.000</meta>
    <meta property="media:duration" refines="#ov2">0:00:05.000</meta>
    <meta property="media:duration">0:00:09.000</meta>
  </metadata>
  <manifest>
    <item id="p1" href="xhtml/page01.xhtml" media-type="application/xhtml+xml" media-overlay="ov1"/>
    <item id="p2" href="xhtml/page02.xhtml" media-type="application/xhtml+xml" media-overlay="ov2"/>
    <item id="ov1" href="smil/page01.smil" media-type="application/smil+xml"/>
    <item id="ov2" href="smil/page02.smil" media-type="application/smil+xml"/>
  </manifest>
  <spine>
    <itemref idref="p1"/>
    <itemref idref="p2"/>
  </spine>
</package>`))

	p1, _ := zw.Create("OEBPS/xhtml/page01.xhtml")
	_, _ = p1.Write([]byte(`<!DOCTYPE html><html><head><meta name="viewport" content="width=600, height=800"/></head><body><p><span id="w1">Four</span> <span id="w2">silly</span> <span id="w3">skeletons</span></p></body></html>`))

	p2, _ := zw.Create("OEBPS/xhtml/page02.xhtml")
	_, _ = p2.Write([]byte(`<!DOCTYPE html><html><head><meta name="viewport" content="width=600, height=800"/></head><body><p><span id="w4">dancing</span> <span id="w5">in</span> <span id="w6">the</span> <span id="w7">night!</span></p></body></html>`))

	s1, _ := zw.Create("OEBPS/smil/page01.smil")
	_, _ = s1.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" version="3.0">
  <body>
    <par id="par1"><text src="../xhtml/page01.xhtml#w1"/><audio src="../audio/p1.mp3" clipBegin="0:00:00.000" clipEnd="0:00:01.200"/></par>
    <par id="par2"><text src="../xhtml/page01.xhtml#w2"/><audio src="../audio/p1.mp3" clipBegin="0:00:01.200" clipEnd="0:00:02.400"/></par>
    <par id="par3"><text src="../xhtml/page01.xhtml#w3"/><audio src="../audio/p1.mp3" clipBegin="0:00:02.400" clipEnd="0:00:04.000"/></par>
  </body>
</smil>`))

	s2, _ := zw.Create("OEBPS/smil/page02.smil")
	_, _ = s2.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" version="3.0">
  <body>
    <par id="par4"><text src="../xhtml/page02.xhtml#w4"/><audio src="../audio/p2.mp3" clipBegin="0:00:00.000" clipEnd="0:00:02.000"/></par>
    <par id="par5"><text src="../xhtml/page02.xhtml#w5"/><audio src="../audio/p2.mp3" clipBegin="0:00:02.000" clipEnd="0:00:03.000"/></par>
    <par id="par6"><text src="../xhtml/page02.xhtml#w6"/><audio src="../audio/p2.mp3" clipBegin="0:00:03.000" clipEnd="0:00:04.000"/></par>
    <par id="par7"><text src="../xhtml/page02.xhtml#w7"/><audio src="../audio/p2.mp3" clipBegin="0:00:04.000" clipEnd="0:00:05.000"/></par>
  </body>
</smil>`))

	_ = zw.Close()
	return buf.Bytes()
}

func TestAPI_ReadalongStageUpload_MatchLinkDedupeAndTimeline(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)

	// Step 1: POST the epub via /api/v1/books/upload (even with missing author in EPUB to test @ART author update)
	epubBytes := buildReadalongEPUB(t, "Four Silly Skeletons!", "")
	var epubBody bytes.Buffer
	epubMpw := multipart.NewWriter(&epubBody)
	epubPart, _ := epubMpw.CreateFormFile("file", "four_silly_skeletons.epub")
	_, _ = epubPart.Write(epubBytes)
	_ = epubMpw.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload", &epubBody)
	uploadReq.Header.Set("Authorization", "Bearer "+token)
	uploadReq.Header.Set("Content-Type", epubMpw.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	f.handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on EPUB upload, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}

	// Step 2: POST the read-along audio with cover via /api/v1/books/upload/stage
	audioBytes := createSyntheticMP3("Four Silly Skeletons", "Mark Sperring & Sue Hendra", "Jane Narrator", 9000, nil)
	coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x01, 0x02}

	var stageBody bytes.Buffer
	stageMpw := multipart.NewWriter(&stageBody)
	audioPart, _ := stageMpw.CreateFormFile("file", "four_silly_skeletons.m4b")
	_, _ = audioPart.Write(audioBytes)
	coverPart, _ := stageMpw.CreateFormFile("cover", "cover.jpg")
	_, _ = coverPart.Write(coverBytes)
	_ = stageMpw.Close()

	stageReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &stageBody)
	stageReq.Header.Set("Authorization", "Bearer "+token)
	stageReq.Header.Set("Content-Type", stageMpw.FormDataContentType())
	stageRec := httptest.NewRecorder()
	f.handler.ServeHTTP(stageRec, stageReq)
	if stageRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on stage upload, got %d: %s", stageRec.Code, stageRec.Body.String())
	}

	var stageResp1 struct {
		ID          string `json:"id"`
		JobID       string `json:"job_id"`
		Status      string `json:"status"`
		IsDuplicate bool   `json:"is_duplicate"`
		HasCover    bool   `json:"has_cover"`
	}
	if err := json.Unmarshal(stageRec.Body.Bytes(), &stageResp1); err != nil {
		t.Fatalf("unmarshal stageResp1: %v", err)
	}
	if stageResp1.IsDuplicate {
		t.Errorf("expected first read-along upload to have is_duplicate = false")
	}
	if !stageResp1.HasCover {
		t.Errorf("expected has_cover = true")
	}

	// Acceptance Criterion 1: GET /api/v1/books?title=<t>&author=<a> ->
	// exactly ONE book entry for the title, with the audio linked (has_readalong true, duration > 0), author = the @ART value.
	lookupURL := "/api/v1/books?title=" + url.QueryEscape("Four Silly Skeletons") + "&author=" + url.QueryEscape("Mark Sperring & Sue Hendra")
	listReq := httptest.NewRequest(http.MethodGet, lookupURL, nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	f.handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/v1/books, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp struct {
		Books []api.BookListItem `json:"books"`
		Total int                `json:"total"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal listResp: %v", err)
	}
	if len(listResp.Books) != 1 || listResp.Total != 1 {
		t.Fatalf("expected exactly 1 book entry, got %d (total=%d): %+v", len(listResp.Books), listResp.Total, listResp.Books)
	}

	bookItem := listResp.Books[0]
	if !bookItem.HasReadalong {
		t.Errorf("expected has_readalong = true on linked book")
	}
	if bookItem.BookType != "ebook" && bookItem.BookType != "" {
		t.Errorf("expected primary book_type to remain ebook, got %q", bookItem.BookType)
	}
	if bookItem.DurationSeconds == nil || *bookItem.DurationSeconds <= 0 {
		t.Errorf("expected duration_seconds > 0, got %v", bookItem.DurationSeconds)
	}
	if len(bookItem.Authors) == 0 || bookItem.Authors[0].Name != "Mark Sperring & Sue Hendra" {
		t.Errorf("expected author 'Mark Sperring & Sue Hendra', got %+v", bookItem.Authors)
	}

	// Acceptance Criterion 2: Re-POST the same m4b -> is_duplicate: true, no second entry.
	var stageBody2 bytes.Buffer
	stageMpw2 := multipart.NewWriter(&stageBody2)
	audioPart2, _ := stageMpw2.CreateFormFile("file", "four_silly_skeletons.m4b")
	_, _ = audioPart2.Write(audioBytes)
	_ = stageMpw2.Close()

	stageReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &stageBody2)
	stageReq2.Header.Set("Authorization", "Bearer "+token)
	stageReq2.Header.Set("Content-Type", stageMpw2.FormDataContentType())
	stageRec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(stageRec2, stageReq2)
	if stageRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on second stage upload, got %d: %s", stageRec2.Code, stageRec2.Body.String())
	}

	var stageResp2 struct {
		IsDuplicate bool `json:"is_duplicate"`
	}
	if err := json.Unmarshal(stageRec2.Body.Bytes(), &stageResp2); err != nil {
		t.Fatalf("unmarshal stageResp2: %v", err)
	}
	if !stageResp2.IsDuplicate {
		t.Errorf("expected re-POST of read-along audio to return is_duplicate = true")
	}

	// Verify still only 1 book in library
	allBooksReq := httptest.NewRequest(http.MethodGet, "/api/v1/books", nil)
	allBooksReq.Header.Set("Authorization", "Bearer "+token)
	allBooksRec := httptest.NewRecorder()
	f.handler.ServeHTTP(allBooksRec, allBooksReq)
	var allBooksResp struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(allBooksRec.Body.Bytes(), &allBooksResp)
	if allBooksResp.Total != 1 {
		t.Fatalf("expected total books = 1 after duplicate re-POST, got %d", allBooksResp.Total)
	}

	// Verify GET /api/v1/books/{id}/readalong returns the SMIL Media Overlays timeline
	raReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+bookItem.ID+"/readalong", nil)
	raReq.Header.Set("Authorization", "Bearer "+token)
	raRec := httptest.NewRecorder()
	f.handler.ServeHTTP(raRec, raReq)
	if raRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /readalong, got %d: %s", raRec.Code, raRec.Body.String())
	}

	var raResp api.ReadalongResponse
	if err := json.Unmarshal(raRec.Body.Bytes(), &raResp); err != nil {
		t.Fatalf("unmarshal ReadalongResponse: %v", err)
	}
	if !raResp.HasReadalong || !raResp.HasMediaOverlays {
		t.Errorf("expected HasReadalong=true and HasMediaOverlays=true, got %+v", raResp)
	}
	if len(raResp.Segments) != 7 {
		t.Fatalf("expected 7 SMIL segments, got %d", len(raResp.Segments))
	}
	if raResp.Segments[0].FragmentID != "w1" || raResp.Segments[0].Text != "Four" || raResp.Segments[0].ChapterIndex != 1 {
		t.Errorf("unexpected first segment: %+v", raResp.Segments[0])
	}
	if raResp.Segments[3].FragmentID != "w4" || raResp.Segments[3].Text != "dancing" || raResp.Segments[3].ChapterIndex != 2 || raResp.Segments[3].GlobalBegin != 4.0 {
		t.Errorf("unexpected page 2 segment: %+v", raResp.Segments[3])
	}

	// Verify GET /api/v1/books/{id}/audio streams the linked audio file
	streamReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+bookItem.ID+"/audio", nil)
	streamReq.Header.Set("Authorization", "Bearer "+token)
	streamRec := httptest.NewRecorder()
	f.handler.ServeHTTP(streamRec, streamReq)
	if streamRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK streaming linked read-along audio, got %d: %s", streamRec.Code, streamRec.Body.String())
	}
}
