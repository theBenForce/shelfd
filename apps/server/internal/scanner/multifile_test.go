package scanner_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
)

func createSampleM4A(albumTitle, trackTitle string, trackNum uint32, durationSec uint32) []byte {
	var moovBuf bytes.Buffer

	// mvhd box (duration in timescale 1000)
	var mvhdBuf bytes.Buffer
	mvhdBuf.WriteByte(0) // version 0
	mvhdBuf.Write([]byte{0, 0, 0}) // flags
	binary.Write(&mvhdBuf, binary.BigEndian, uint32(0)) // creation
	binary.Write(&mvhdBuf, binary.BigEndian, uint32(0)) // mod
	binary.Write(&mvhdBuf, binary.BigEndian, uint32(1000)) // timescale = 1000
	binary.Write(&mvhdBuf, binary.BigEndian, durationSec*1000) // duration
	binary.Write(&mvhdBuf, binary.BigEndian, uint32(0x00010000)) // rate 1.0
	binary.Write(&mvhdBuf, binary.BigEndian, uint16(0x0100))     // volume 1.0
	mvhdBuf.Write(make([]byte, 10))                              // reserved
	mvhdBuf.Write(make([]byte, 36))                              // matrix
	mvhdBuf.Write(make([]byte, 24))                              // pre-defined
	binary.Write(&mvhdBuf, binary.BigEndian, uint32(2))          // next track id
	writeBox(&moovBuf, "mvhd", mvhdBuf.Bytes())

	// udta -> meta -> ilst
	var ilstBuf bytes.Buffer
	if albumTitle != "" {
		writeIlstTextAtom(&ilstBuf, "\xa9alb", albumTitle)
	}
	if trackTitle != "" {
		writeIlstTextAtom(&ilstBuf, "\xa9nam", trackTitle)
	}
	if trackNum > 0 {
		var trknData bytes.Buffer
		binary.Write(&trknData, binary.BigEndian, uint16(0)) // reserved
		binary.Write(&trknData, binary.BigEndian, uint16(trackNum))
		binary.Write(&trknData, binary.BigEndian, uint16(21)) // total tracks
		binary.Write(&trknData, binary.BigEndian, uint16(0))
		writeIlstDataAtom(&ilstBuf, "trkn", 0, trknData.Bytes())
	}

	var metaBuf bytes.Buffer
	metaBuf.Write([]byte{0, 0, 0, 0}) // version & flags
	writeBox(&metaBuf, "hdlr", []byte{0, 0, 0, 0, 0, 0, 0, 0, 'm', 'd', 'i', 'r', 'a', 'p', 'p', 'l', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	writeBox(&metaBuf, "ilst", ilstBuf.Bytes())

	var udtaBuf bytes.Buffer
	writeBox(&udtaBuf, "meta", metaBuf.Bytes())
	writeBox(&moovBuf, "udta", udtaBuf.Bytes())

	var buf bytes.Buffer
	// ftyp box
	writeBox(&buf, "ftyp", []byte("M4A \x00\x00\x02\x00M4A mp42isom"))
	// moov box
	writeBox(&buf, "moov", moovBuf.Bytes())

	return buf.Bytes()
}

func writeBox(w *bytes.Buffer, boxType string, payload []byte) {
	size := uint32(len(payload) + 8)
	binary.Write(w, binary.BigEndian, size)
	w.WriteString(boxType)
	w.Write(payload)
}

func writeIlstTextAtom(w *bytes.Buffer, atomName, textVal string) {
	writeIlstDataAtom(w, atomName, 1, []byte(textVal))
}

func writeIlstDataAtom(w *bytes.Buffer, atomName string, dataType uint32, rawData []byte) {
	var dataAtom bytes.Buffer
	dataSize := uint32(len(rawData) + 16)
	binary.Write(&dataAtom, binary.BigEndian, dataSize)
	dataAtom.WriteString("data")
	binary.Write(&dataAtom, binary.BigEndian, dataType)
	binary.Write(&dataAtom, binary.BigEndian, uint32(0))
	dataAtom.Write(rawData)

	writeBox(w, atomName, dataAtom.Bytes())
}

func TestMultiFileAudiobookIngestion(t *testing.T) {
	tempLib := t.TempDir()
	tempData := t.TempDir()
	ctx := context.Background()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("creating db: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	repo := repository.NewSQLiteStorageEngine(db)
	defer repo.Close()

	// Setup directory: /library/J.R.R. Tolkien/The Hobbit/
	bookDir := filepath.Join(tempLib, "J.R.R. Tolkien", "The Hobbit")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create 3 track files:
	// Track 1: 100 seconds (e.g. 1. An Unexpected Party)
	// Track 2: 200 seconds (e.g. 2. Roast Mutton)
	// Track 3: 300 seconds (e.g. 3. A Short Rest)
	track1 := createSampleM4A("The Hobbit", "1. An Unexpected Party", 1, 100)
	track2 := createSampleM4A("The Hobbit", "2. Roast Mutton", 2, 200)
	track3 := createSampleM4A("The Hobbit", "3. A Short Rest", 3, 300)

	os.WriteFile(filepath.Join(bookDir, "01 - An Unexpected Party.m4a"), track1, 0644)
	os.WriteFile(filepath.Join(bookDir, "02 - Roast Mutton.m4a"), track2, 0644)
	os.WriteFile(filepath.Join(bookDir, "03 - A Short Rest.m4a"), track3, 0644)

	ingester := scanner.NewIngester(repo, tempLib, tempData)

	// Scan library
	s := scanner.NewScanner(tempLib)
	discovered, err := s.Scan()
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(discovered) != 3 {
		t.Fatalf("expected 3 discovered files, got %d", len(discovered))
	}

	// Ingest all 3 files
	for _, f := range discovered {
		_, _, err := ingester.SyncFile(ctx, f.FullPath, f.RelativePath)
		if err != nil {
			t.Fatalf("ingest error on %s: %v", f.RelativePath, err)
		}
	}

	// Verify that ONLY ONE book record was created in the database
	books, err := repo.ListBooks(ctx, repository.BookFilter{})
	if err != nil {
		t.Fatalf("listing books: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("expected exactly 1 Book record for multi-file audiobook, got %d", len(books))
	}

	b := books[0]
	if b.Title != "The Hobbit" {
		t.Errorf("expected book Title 'The Hobbit', got '%s'", b.Title)
	}
	if b.DurationSeconds == nil || *b.DurationSeconds != 600.0 {
		t.Errorf("expected total DurationSeconds 600 (100+200+300), got %v", b.DurationSeconds)
	}

	// Verify Audio Chapters created
	chapters, err := repo.GetAudioChaptersByBookID(ctx, b.ID)
	if err != nil {
		t.Fatalf("getting audio chapters: %v", err)
	}

	if len(chapters) != 3 {
		t.Fatalf("expected 3 audio chapters, got %d", len(chapters))
	}

	if chapters[0].Title != "1. An Unexpected Party" || chapters[0].StartOffsetSec != 0 || chapters[0].DurationSec != 100 {
		t.Errorf("unexpected chapter 1: %+v", chapters[0])
	}
	if chapters[1].Title != "2. Roast Mutton" || chapters[1].StartOffsetSec != 100 || chapters[1].DurationSec != 200 {
		t.Errorf("unexpected chapter 2: %+v", chapters[1])
	}
	if chapters[2].Title != "3. A Short Rest" || chapters[2].StartOffsetSec != 300 || chapters[2].DurationSec != 300 {
		t.Errorf("unexpected chapter 3: %+v", chapters[2])
	}

	// Verify BookFiles created
	files, err := repo.GetBookFilesByBookID(ctx, b.ID)
	if err != nil {
		t.Fatalf("getting book files: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 book files, got %d", len(files))
	}
}

func TestMultiFileAudiobook_Track17TitleFixAndAuthorNormalization(t *testing.T) {
	tempLib := t.TempDir()
	tempData := t.TempDir()
	ctx := context.Background()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("creating db: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	repo := repository.NewSQLiteStorageEngine(db)
	defer repo.Close()

	// 1. Create directory structure: J.R.R. Tolkien / The Hobbit
	bookDir := filepath.Join(tempLib, "J.R.R. Tolkien", "The Hobbit")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create Track 17 first with empty album and track title "17. The Clouds Burst"
	track17 := createSampleM4A("", "17. The Clouds Burst", 17, 120)
	os.WriteFile(filepath.Join(bookDir, "17. The Clouds Burst.m4a"), track17, 0644)

	ingester := scanner.NewIngester(repo, tempLib, tempData)

	// Ingest track 17 first
	s := scanner.NewScanner(tempLib)
	files, err := s.Scan()
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	for _, f := range files {
		_, _, err := ingester.SyncFile(ctx, f.FullPath, f.RelativePath)
		if err != nil {
			t.Fatalf("ingest track 17 error: %v", err)
		}
	}

	// Verify that title was NOT set to "17. The Clouds Burst" but resolved to folder "The Hobbit"
	books, err := repo.ListBooks(ctx, repository.BookFilter{})
	if err != nil {
		t.Fatalf("list books: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
	if books[0].Title != "The Hobbit" {
		t.Errorf("expected book Title 'The Hobbit', got '%s'", books[0].Title)
	}

	// Now add Track 18
	track18 := createSampleM4A("The Hobbit", "18. The Return Journey", 18, 150)
	os.WriteFile(filepath.Join(bookDir, "18. The Return Journey.m4a"), track18, 0644)

	// Rescan
	files2, _ := s.Scan()
	for _, f := range files2 {
		_, _, _ = ingester.SyncFile(ctx, f.FullPath, f.RelativePath)
	}

	// Verify author is normalized
	authors, err := repo.ListAuthors(ctx)
	if err != nil {
		t.Fatalf("list authors: %v", err)
	}
	if len(authors) != 1 {
		t.Fatalf("expected 1 author, got %d", len(authors))
	}
	if authors[0].Name != "J. R. R. Tolkien" {
		t.Errorf("expected normalized author name 'J. R. R. Tolkien', got %q", authors[0].Name)
	}
}
