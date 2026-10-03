package audio

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseMP4Metadata(t *testing.T) {
	// Build synthetic MP4 byte stream with moov -> mvhd, udta -> meta -> ilst (nam, ART, covr), udta -> chpl
	var buf bytes.Buffer

	// ftyp atom
	ftypData := []byte("M4A \x00\x00\x02\x00isomiso2")
	writeAtom(&buf, "ftyp", ftypData)

	// mvhd atom (version 0, timescale 1000, duration 3600000 -> 3600s = 1 hour)
	mvhdData := make([]byte, 100)
	mvhdData[0] = 0 // version
	binary.BigEndian.PutUint32(mvhdData[12:16], 1000)    // timescale
	binary.BigEndian.PutUint32(mvhdData[16:20], 3600000) // duration

	// ilst tags: ©nam, ©ART, ©nrt
	var ilstBuf bytes.Buffer
	writeIlstTextTag(&ilstBuf, "\xa9nam", "The Way of Kings")
	writeIlstTextTag(&ilstBuf, "\xa9ART", "Brandon Sanderson")
	writeIlstTextTag(&ilstBuf, "\xa9nrt", "Michael Kramer & Kate Reading")
	writeIlstTextTag(&ilstBuf, "desc", "An epic fantasy audiobook.")

	// covr tag (JPEG magic bytes)
	fakeCover := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	writeIlstDataTag(&ilstBuf, "covr", 13, fakeCover)

	// meta atom (4 bytes version/flags + ilst)
	var metaBuf bytes.Buffer
	metaBuf.Write([]byte{0, 0, 0, 0})
	writeAtom(&metaBuf, "ilst", ilstBuf.Bytes())

	// chpl atom (Nero chapters)
	// 2 chapters: Chap 1 at 0s, Chap 2 at 1800s (in 100ns units: 1800 * 10,000,000)
	var chplBuf bytes.Buffer
	chplBuf.Write([]byte{1, 0, 0, 0, 0}) // version, flags, reserved
	binary.Write(&chplBuf, binary.BigEndian, uint32(2))
	// Chap 1
	binary.Write(&chplBuf, binary.BigEndian, uint64(0))
	chplBuf.WriteByte(byte(len("Prelude to the Storm")))
	chplBuf.WriteString("Prelude to the Storm")
	// Chap 2
	binary.Write(&chplBuf, binary.BigEndian, uint64(1800*10000000))
	chplBuf.WriteByte(byte(len("Chapter 1: Stormblessed")))
	chplBuf.WriteString("Chapter 1: Stormblessed")

	// udta atom
	var udtaBuf bytes.Buffer
	writeAtom(&udtaBuf, "meta", metaBuf.Bytes())
	writeAtom(&udtaBuf, "chpl", chplBuf.Bytes())

	// moov atom
	var moovBuf bytes.Buffer
	writeAtom(&moovBuf, "mvhd", mvhdData)
	writeAtom(&moovBuf, "udta", udtaBuf.Bytes())

	writeAtom(&buf, "moov", moovBuf.Bytes())

	meta, err := ExtractMetadataFromReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), ".m4b")
	if err != nil {
		t.Fatalf("unexpected error parsing MP4: %v", err)
	}

	if meta.Title != "The Way of Kings" {
		t.Errorf("expected Title 'The Way of Kings', got '%s'", meta.Title)
	}
	if meta.Author != "Brandon Sanderson" {
		t.Errorf("expected Author 'Brandon Sanderson', got '%s'", meta.Author)
	}
	if meta.Narrator != "Michael Kramer & Kate Reading" {
		t.Errorf("expected Narrator 'Michael Kramer & Kate Reading', got '%s'", meta.Narrator)
	}
	if meta.Description != "An epic fantasy audiobook." {
		t.Errorf("expected Description 'An epic fantasy audiobook.', got '%s'", meta.Description)
	}
	if meta.DurationSeconds != 3600 {
		t.Errorf("expected DurationSeconds 3600, got %f", meta.DurationSeconds)
	}
	if len(meta.CoverData) == 0 || meta.CoverMimeType != "image/jpeg" {
		t.Errorf("expected JPEG cover data, got len=%d mime=%s", len(meta.CoverData), meta.CoverMimeType)
	}
	if len(meta.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(meta.Chapters))
	}
	if meta.Chapters[0].Title != "Prelude to the Storm" || meta.Chapters[0].DurationSec != 1800 {
		t.Errorf("unexpected chapter 0: %+v", meta.Chapters[0])
	}
	if meta.Chapters[1].Title != "Chapter 1: Stormblessed" || meta.Chapters[1].StartOffsetSec != 1800 || meta.Chapters[1].DurationSec != 1800 {
		t.Errorf("unexpected chapter 1: %+v", meta.Chapters[1])
	}
}

func TestParseMP3Metadata(t *testing.T) {
	var id3Buf bytes.Buffer

	// Frames: TIT2, TPE1, TPE2, TLEN (120000 ms = 120s), APIC
	writeID3Frame(&id3Buf, "TIT2", "\x03Dune")
	writeID3Frame(&id3Buf, "TPE1", "\x03Frank Herbert")
	writeID3Frame(&id3Buf, "TPE2", "\x03Scott Brick")
	writeID3Frame(&id3Buf, "TLEN", "\x00120000")

	// APIC frame
	var apicBuf bytes.Buffer
	apicBuf.WriteByte(0) // text encoding
	apicBuf.WriteString("image/jpeg\x00")
	apicBuf.WriteByte(3) // cover front
	apicBuf.WriteString("Cover\x00")
	apicBuf.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
	writeID3Frame(&id3Buf, "APIC", string(apicBuf.Bytes()))

	// CHAP frame
	var chapBuf bytes.Buffer
	chapBuf.WriteString("ch1\x00")
	binary.Write(&chapBuf, binary.BigEndian, uint32(0))      // start ms
	binary.Write(&chapBuf, binary.BigEndian, uint32(60000))  // end ms
	binary.Write(&chapBuf, binary.BigEndian, uint32(0))      // start offset
	binary.Write(&chapBuf, binary.BigEndian, uint32(0))      // end offset
	writeID3Frame(&chapBuf, "TIT2", "\x03Chapter 1")
	writeID3Frame(&id3Buf, "CHAP", string(chapBuf.Bytes()))

	var buf bytes.Buffer
	// ID3 header: "ID3", ver 3.0, flags 0, syncsafe size
	buf.WriteString("ID3\x03\x00\x00")
	tagSize := id3Buf.Len()
	buf.Write(encodeSyncSafe(uint32(tagSize)))
	buf.Write(id3Buf.Bytes())

	// Append dummy audio payload
	buf.Write(make([]byte, 1024))

	meta, err := ExtractMetadataFromReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), ".mp3")
	if err != nil {
		t.Fatalf("unexpected error parsing MP3: %v", err)
	}

	if meta.Title != "Dune" {
		t.Errorf("expected Title 'Dune', got '%s'", meta.Title)
	}
	if meta.Author != "Frank Herbert" {
		t.Errorf("expected Author 'Frank Herbert', got '%s'", meta.Author)
	}
	if meta.Narrator != "Scott Brick" {
		t.Errorf("expected Narrator 'Scott Brick', got '%s'", meta.Narrator)
	}
	if meta.DurationSeconds != 120 {
		t.Errorf("expected DurationSeconds 120, got %f", meta.DurationSeconds)
	}
	if len(meta.CoverData) == 0 || meta.CoverMimeType != "image/jpeg" {
		t.Errorf("expected JPEG cover data, got len=%d mime=%s", len(meta.CoverData), meta.CoverMimeType)
	}
	if len(meta.Chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(meta.Chapters))
	}
	if meta.Chapters[0].Title != "Chapter 1" || meta.Chapters[0].DurationSec != 60 {
		t.Errorf("unexpected chapter 0: %+v", meta.Chapters[0])
	}
}

// Helpers

func writeAtom(w *bytes.Buffer, atomType string, payload []byte) {
	size := uint32(8 + len(payload))
	binary.Write(w, binary.BigEndian, size)
	w.WriteString(atomType)
	w.Write(payload)
}

func writeIlstTextTag(w *bytes.Buffer, tagType string, text string) {
	var dataBuf bytes.Buffer
	dataBuf.Write([]byte{0, 0, 0, 1}) // type: UTF8
	dataBuf.Write([]byte{0, 0, 0, 0}) // locale
	dataBuf.WriteString(text)

	var itemBuf bytes.Buffer
	writeAtom(&itemBuf, "data", dataBuf.Bytes())
	writeAtom(w, tagType, itemBuf.Bytes())
}

func writeIlstDataTag(w *bytes.Buffer, tagType string, typeFlag uint32, data []byte) {
	var dataBuf bytes.Buffer
	binary.Write(&dataBuf, binary.BigEndian, typeFlag)
	dataBuf.Write([]byte{0, 0, 0, 0}) // locale
	dataBuf.Write(data)

	var itemBuf bytes.Buffer
	writeAtom(&itemBuf, "data", dataBuf.Bytes())
	writeAtom(w, tagType, itemBuf.Bytes())
}

func writeID3Frame(w *bytes.Buffer, frameID string, payload string) {
	w.WriteString(frameID)
	binary.Write(w, binary.BigEndian, uint32(len(payload)))
	w.Write([]byte{0, 0}) // flags
	w.WriteString(payload)
}

func encodeSyncSafe(val uint32) []byte {
	return []byte{
		byte((val >> 21) & 0x7F),
		byte((val >> 14) & 0x7F),
		byte((val >> 7) & 0x7F),
		byte(val & 0x7F),
	}
}
