package epub_test

import (
	"archive/zip"
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/epub"
)

func TestParseSMILClock(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{"0:00:00", 0.0, false},
		{"0:00:14.200", 14.2, false},
		{"01:02:03.5", 3723.5, false},
		{"02:03.25", 123.25, false},
		{"00:05", 5.0, false},
		{"12.345s", 12.345, false},
		{"500ms", 0.5, false},
		{"2.5min", 150.0, false},
		{"1.5h", 5400.0, false},
		{"npt=8.75s", 8.75, false},
		{"42.125", 42.125, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"-5s", 0, true},
	}

	for _, tc := range tests {
		got, err := epub.ParseSMILClock(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseSMILClock(%q) expected error, got %v", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSMILClock(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if math.Abs(got-tc.want) > 1e-4 {
			t.Errorf("ParseSMILClock(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseSMIL_DocumentOrderAndNestedSeq(t *testing.T) {
	smilXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" xmlns:epub="http://www.idpf.org/2007/ops" version="3.0">
  <body epub:textref="../xhtml/page01.xhtml" epub:type="bodymatter chapter">
    <par id="par1">
      <text src="../xhtml/page01.xhtml#w1"/>
      <audio src="../audio/part01.mp3" clipBegin="0:00:00.000" clipEnd="0:00:01.250"/>
    </par>
    <seq id="seq1" epub:textref="../xhtml/page01.xhtml#sec1" epub:type="section">
      <par id="par2">
        <text src="../xhtml/page01.xhtml#w2"/>
        <audio src="../audio/part01.mp3" clipBegin="1.25s" clipEnd="2.75s"/>
      </par>
    </seq>
    <par id="par3" epub:type="pagebreak">
      <text src="../xhtml/page01.xhtml#w3"/>
      <audio src="../audio/part01.mp3" clipBegin="2750ms" clipEnd="00:04.500"/>
    </par>
  </body>
</smil>`)

	pars, err := epub.ParseSMIL(smilXML)
	if err != nil {
		t.Fatalf("ParseSMIL failed: %v", err)
	}
	if len(pars) != 3 {
		t.Fatalf("expected 3 pars, got %d", len(pars))
	}

	if pars[0].ID != "par1" || pars[0].TextSrc != "../xhtml/page01.xhtml#w1" || pars[0].ClipBegin != 0 || pars[0].ClipEnd != 1.25 {
		t.Errorf("unexpected par[0]: %+v", pars[0])
	}
	if pars[1].ID != "par2" || pars[1].EPUBType != "section" || pars[1].ClipBegin != 1.25 || pars[1].ClipEnd != 2.75 {
		t.Errorf("unexpected par[1]: %+v", pars[1])
	}
	if pars[2].ID != "par3" || pars[2].EPUBType != "pagebreak" || pars[2].ClipBegin != 2.75 || pars[2].ClipEnd != 4.5 {
		t.Errorf("unexpected par[2]: %+v", pars[2])
	}
}

func TestExtractMediaOverlays_EPUB3ReadAlong(t *testing.T) {
	files := map[string][]byte{
		"META-INF/container.xml": []byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`),
		"OEBPS/content.opf": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:isbn:9780000000001</dc:identifier>
    <dc:title>Four Silly Skeletons</dc:title>
    <dc:creator>Mark Sperring</dc:creator>
    <meta property="rendition:layout">pre-paginated</meta>
    <meta property="media:active-class">readaloud-word-active</meta>
    <meta property="media:playback-active-class">readaloud-playing</meta>
    <meta property="media:narrator">Jane Reader</meta>
    <meta property="media:duration" refines="#ov1">0:00:05.000</meta>
    <meta property="media:duration" refines="#ov2">0:00:06.000</meta>
    <meta property="media:duration">0:00:11.000</meta>
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
</package>`),
		"OEBPS/xhtml/page01.xhtml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><meta name="viewport" content="width=600, height=800"/></head>
<body>
  <p><span id="w1">Four</span> <span id="w2">silly</span> <span id="w3">skeletons</span></p>
</body>
</html>`),
		"OEBPS/xhtml/page02.xhtml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><meta name="viewport" content="width=600, height=800"/></head>
<body>
  <p><span id="p2_w1">dancing</span> <span id="p2_w2">in</span> <span id="p2_w3">the</span> <span id="p2_w4">dark!</span></p>
</body>
</html>`),
		"OEBPS/smil/page01.smil": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" version="3.0">
  <body>
    <par id="s1"><text src="../xhtml/page01.xhtml#w1"/><audio src="../audio/track1.mp3" clipBegin="0:00:00.000" clipEnd="0:00:01.200"/></par>
    <par id="s2"><text src="../xhtml/page01.xhtml#w2"/><audio src="../audio/track1.mp3" clipBegin="0:00:01.200" clipEnd="0:00:02.500"/></par>
    <par id="s3"><text src="../xhtml/page01.xhtml#w3"/><audio src="../audio/track1.mp3" clipBegin="0:00:02.500" clipEnd="0:00:04.800"/></par>
  </body>
</smil>`),
		"OEBPS/smil/page02.smil": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" version="3.0">
  <body>
    <par id="s4"><text src="../xhtml/page02.xhtml#p2_w1"/><audio src="../audio/track2.mp3" clipBegin="0:00:00.000" clipEnd="0:00:02.000"/></par>
    <par id="s5"><text src="../xhtml/page02.xhtml#p2_w2"/><audio src="../audio/track2.mp3" clipBegin="0:00:02.000" clipEnd="0:00:03.000"/></par>
    <par id="s6"><text src="../xhtml/page02.xhtml#p2_w3"/><audio src="../audio/track2.mp3" clipBegin="0:00:03.000" clipEnd="0:00:04.000"/></par>
    <par id="s7"><text src="../xhtml/page02.xhtml#p2_w4"/><audio src="../audio/track2.mp3" clipBegin="0:00:04.000" clipEnd="0:00:06.000"/></par>
  </body>
</smil>`),
	}

	epubBytes := createTestEPUB(files)
	zr, err := zip.NewReader(bytes.NewReader(epubBytes), int64(len(epubBytes)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	reader, err := epub.NewReader(zr)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	parsedBook, err := reader.ParseBook()
	if err != nil {
		t.Fatalf("ParseBook failed: %v", err)
	}
	if !parsedBook.HasMediaOverlays {
		t.Errorf("expected HasMediaOverlays = true")
	}
	if parsedBook.MediaActiveClass != "readaloud-word-active" {
		t.Errorf("expected MediaActiveClass 'readaloud-word-active', got %q", parsedBook.MediaActiveClass)
	}
	if parsedBook.MediaPlaybackActiveClass != "readaloud-playing" {
		t.Errorf("expected MediaPlaybackActiveClass 'readaloud-playing', got %q", parsedBook.MediaPlaybackActiveClass)
	}
	if parsedBook.MediaNarrator != "Jane Reader" {
		t.Errorf("expected MediaNarrator 'Jane Reader', got %q", parsedBook.MediaNarrator)
	}
	if parsedBook.MediaDurationSeconds == nil || math.Abs(*parsedBook.MediaDurationSeconds-11.0) > 1e-4 {
		t.Errorf("expected MediaDurationSeconds = 11.0, got %v", parsedBook.MediaDurationSeconds)
	}

	manifest, err := reader.ExtractMediaOverlays()
	if err != nil {
		t.Fatalf("ExtractMediaOverlays failed: %v", err)
	}
	if !manifest.HasMediaOverlays {
		t.Fatalf("expected manifest.HasMediaOverlays = true")
	}
	if len(manifest.Segments) != 7 {
		t.Fatalf("expected 7 overlay segments, got %d", len(manifest.Segments))
	}

	// Verify first segment on Page 1
	seg0 := manifest.Segments[0]
	if seg0.ChapterIndex != 1 || seg0.FragmentID != "w1" || seg0.Text != "Four" || seg0.GlobalBegin != 0 || seg0.GlobalEnd != 1.2 {
		t.Errorf("unexpected seg[0]: %+v", seg0)
	}

	// Verify fourth segment (first word on Page 2): since track1 had refined duration 5.0s,
	// track2's global offset starts at 5.0s!
	seg3 := manifest.Segments[3]
	if seg3.ChapterIndex != 2 || seg3.FragmentID != "p2_w1" || seg3.Text != "dancing" {
		t.Errorf("unexpected seg[3]: %+v", seg3)
	}
	if math.Abs(seg3.GlobalBegin-5.0) > 1e-4 || math.Abs(seg3.GlobalEnd-7.0) > 1e-4 {
		t.Errorf("expected seg[3] global [5.0, 7.0], got [%v, %v]", seg3.GlobalBegin, seg3.GlobalEnd)
	}

	// Verify RewritePageHTML injects readalong highlight bridge with chapterHref scoping
	w := 600
	h := 800
	rewritten := epub.RewritePageHTML(string(files["OEBPS/xhtml/page01.xhtml"]), "book-123", "OEBPS/xhtml/page01.xhtml", &w, &h)
	if !strings.Contains(rewritten, "shelfd:readalong-highlight") || !strings.Contains(rewritten, "__shelfdHighlightReadalong") {
		t.Errorf("expected RewritePageHTML to inject readalong highlight bridge")
	}
	if !strings.Contains(rewritten, "d.chapterHref") || !strings.Contains(rewritten, "incomingHref !== normCurrentHref") {
		t.Errorf("expected RewritePageHTML bridge to scope highlights by chapterHref")
	}
}
