package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Metadata represents extracted audio file metadata and chapter bookmarks.
type Metadata struct {
	Title           string
	Author          string
	Narrator        string
	Description     string
	PublishedDate   string
	DurationSeconds float64
	BitrateKbps     int
	CoverData       []byte
	CoverMimeType   string
	Chapters        []ChapterInfo
}

// ChapterInfo represents an individual chapter in an audio track.
type ChapterInfo struct {
	Title          string
	StartOffsetSec float64
	DurationSec    float64
}

// ExtractMetadata extracts metadata and chapter information from an audio file (.m4b, .m4a, .mp3).
func ExtractMetadata(filePath string) (*Metadata, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening audio file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stating audio file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	return ExtractMetadataFromReader(f, fi.Size(), ext)
}

// ExtractMetadataFromReader parses audio metadata from a seekable reader given the file extension.
func ExtractMetadataFromReader(r io.ReadSeeker, size int64, ext string) (*Metadata, error) {
	switch ext {
	case ".m4b", ".m4a", ".mp4":
		return parseMP4(r, size)
	case ".mp3":
		return parseMP3(r, size)
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}
}

// --- MP4 / M4B / M4A Parser ---

func parseMP4(r io.ReadSeeker, size int64) (*Metadata, error) {
	meta := &Metadata{}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// Scan top-level boxes
	for {
		boxOffset, err := r.Seek(0, io.SeekCurrent)
		if err != nil || boxOffset >= size {
			break
		}

		header := make([]byte, 8)
		n, err := io.ReadFull(r, header)
		if err != nil || n < 8 {
			break
		}

		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])

		var payloadOffset int64 = 8
		if boxSize == 1 {
			largeHeader := make([]byte, 8)
			if _, err := io.ReadFull(r, largeHeader); err != nil {
				break
			}
			boxSize = int64(binary.BigEndian.Uint64(largeHeader))
			payloadOffset = 16
		} else if boxSize == 0 {
			boxSize = size - boxOffset
		}

		if boxType == "moov" {
			data := make([]byte, boxSize-payloadOffset)
			if _, err := io.ReadFull(r, data); err == nil {
				parseMoov(bytes.NewReader(data), int64(len(data)), meta)
			}
			break
		}

		nextPos := boxOffset + boxSize
		if nextPos <= boxOffset || nextPos > size {
			break
		}
		if _, err := r.Seek(nextPos, io.SeekStart); err != nil {
			break
		}
	}

	// Calculate bitrate if duration is available
	if meta.DurationSeconds > 0 && size > 0 {
		meta.BitrateKbps = int((float64(size*8) / meta.DurationSeconds) / 1000)
	}

	// Default fallback chapter if none discovered
	if len(meta.Chapters) == 0 && meta.DurationSeconds > 0 {
		meta.Chapters = []ChapterInfo{
			{
				Title:          "Chapter 1",
				StartOffsetSec: 0,
				DurationSec:    meta.DurationSeconds,
			},
		}
	}

	return meta, nil
}

func parseMoov(r *bytes.Reader, size int64, meta *Metadata) {
	for {
		boxOffset, _ := r.Seek(0, io.SeekCurrent)
		if boxOffset >= size {
			break
		}

		header := make([]byte, 8)
		if n, err := io.ReadFull(r, header); err != nil || n < 8 {
			break
		}

		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])

		if boxSize < 8 {
			break
		}

		payload := make([]byte, boxSize-8)
		if _, err := io.ReadFull(r, payload); err != nil {
			break
		}

		switch boxType {
		case "mvhd":
			if len(payload) >= 20 {
				version := payload[0]
				if version == 0 && len(payload) >= 24 {
					timescale := binary.BigEndian.Uint32(payload[12:16])
					duration := binary.BigEndian.Uint32(payload[16:20])
					if timescale > 0 {
						meta.DurationSeconds = float64(duration) / float64(timescale)
					}
				} else if version == 1 && len(payload) >= 32 {
					timescale := binary.BigEndian.Uint32(payload[20:24])
					duration := binary.BigEndian.Uint64(payload[24:32])
					if timescale > 0 {
						meta.DurationSeconds = float64(duration) / float64(timescale)
					}
				}
			}
		case "udta":
			parseUdta(bytes.NewReader(payload), int64(len(payload)), meta)
		case "trak":
			// Can contain sub-traks with text chapter references
		}
	}
}

func parseUdta(r *bytes.Reader, size int64, meta *Metadata) {
	for {
		boxOffset, _ := r.Seek(0, io.SeekCurrent)
		if boxOffset >= size {
			break
		}

		header := make([]byte, 8)
		if n, err := io.ReadFull(r, header); err != nil || n < 8 {
			break
		}

		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		if boxSize < 8 {
			break
		}

		payload := make([]byte, boxSize-8)
		if _, err := io.ReadFull(r, payload); err != nil {
			break
		}

		switch boxType {
		case "meta":
			// meta box has 4-byte version/flags before children
			if len(payload) >= 4 {
				parseMeta(bytes.NewReader(payload[4:]), int64(len(payload)-4), meta)
			}
		case "chpl":
			parseChpl(payload, meta)
		}
	}
}

func parseMeta(r *bytes.Reader, size int64, meta *Metadata) {
	for {
		boxOffset, _ := r.Seek(0, io.SeekCurrent)
		if boxOffset >= size {
			break
		}

		header := make([]byte, 8)
		if n, err := io.ReadFull(r, header); err != nil || n < 8 {
			break
		}

		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		if boxSize < 8 {
			break
		}

		payload := make([]byte, boxSize-8)
		if _, err := io.ReadFull(r, payload); err != nil {
			break
		}

		if boxType == "ilst" {
			parseIlst(bytes.NewReader(payload), int64(len(payload)), meta)
		}
	}
}

func parseIlst(r *bytes.Reader, size int64, meta *Metadata) {
	for {
		boxOffset, _ := r.Seek(0, io.SeekCurrent)
		if boxOffset >= size {
			break
		}

		header := make([]byte, 8)
		if n, err := io.ReadFull(r, header); err != nil || n < 8 {
			break
		}

		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		if boxSize < 8 {
			break
		}

		payload := make([]byte, boxSize-8)
		if _, err := io.ReadFull(r, payload); err != nil {
			break
		}

		textVal, dataVal := extractDataAtom(payload)

		switch boxType {
		case "\xa9nam", "titl":
			if meta.Title == "" {
				meta.Title = textVal
			}
		case "\xa9ART", "\xa9wrt", "\xa9aut", "auth":
			if meta.Author == "" {
				meta.Author = textVal
			}
		case "\xa9nrt":
			if meta.Narrator == "" {
				meta.Narrator = textVal
			}
		case "\xa9alb":
			if meta.Title == "" {
				meta.Title = textVal
			}
		case "\xa9day":
			meta.PublishedDate = textVal
		case "desc", "ldes", "\xa9des":
			if meta.Description == "" {
				meta.Description = textVal
			}
		case "covr":
			if len(dataVal) > 0 {
				meta.CoverData = dataVal
				if bytes.HasPrefix(dataVal, []byte{0xFF, 0xD8, 0xFF}) {
					meta.CoverMimeType = "image/jpeg"
				} else if bytes.HasPrefix(dataVal, []byte{0x89, 'P', 'N', 'G'}) {
					meta.CoverMimeType = "image/png"
				} else {
					meta.CoverMimeType = "image/jpeg"
				}
			}
		}
	}
}

func extractDataAtom(payload []byte) (string, []byte) {
	r := bytes.NewReader(payload)
	for {
		if r.Len() < 8 {
			break
		}
		header := make([]byte, 8)
		if _, err := io.ReadFull(r, header); err != nil {
			break
		}
		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		if boxSize < 8 || int(boxSize-8) > r.Len() {
			break
		}
		data := make([]byte, boxSize-8)
		if _, err := io.ReadFull(r, data); err != nil {
			break
		}
		if boxType == "data" && len(data) >= 8 {
			// bytes 0-3: type flags, bytes 4-7: locale
			val := data[8:]
			return strings.TrimSpace(string(val)), val
		}
	}
	return "", nil
}

func parseChpl(payload []byte, meta *Metadata) {
	if len(payload) < 9 {
		return
	}
	// payload[0]: version, payload[1..3]: flags, payload[4]: reserved, payload[5..8]: count
	count := int(binary.BigEndian.Uint32(payload[5:9]))
	offset := 9

	type rawChap struct {
		start100ns uint64
		title      string
	}
	var rawChapters []rawChap

	for i := 0; i < count; i++ {
		if offset+9 > len(payload) {
			break
		}
		startTime100ns := binary.BigEndian.Uint64(payload[offset : offset+8])
		titleLen := int(payload[offset+8])
		offset += 9
		if offset+titleLen > len(payload) {
			break
		}
		title := string(payload[offset : offset+titleLen])
		offset += titleLen
		rawChapters = append(rawChapters, rawChap{
			start100ns: startTime100ns,
			title:      title,
		})
	}

	for i, rc := range rawChapters {
		startSec := float64(rc.start100ns) / 10000000.0
		var durSec float64
		if i+1 < len(rawChapters) {
			nextStartSec := float64(rawChapters[i+1].start100ns) / 10000000.0
			durSec = nextStartSec - startSec
		} else if meta.DurationSeconds > startSec {
			durSec = meta.DurationSeconds - startSec
		} else {
			durSec = 0
		}
		meta.Chapters = append(meta.Chapters, ChapterInfo{
			Title:          rc.title,
			StartOffsetSec: startSec,
			DurationSec:    durSec,
		})
	}
}

// --- MP3 ID3v2 Parser ---

func parseMP3(r io.ReadSeeker, size int64) (*Metadata, error) {
	meta := &Metadata{}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	header := make([]byte, 10)
	n, err := io.ReadFull(r, header)
	if err != nil || n < 10 {
		return nil, errors.New("file too short for MP3")
	}

	var tagSize int64 = 0
	if string(header[0:3]) == "ID3" {
		version := header[3]
		// Syncsafe integer for ID3 tag size
		tagSize = int64(decodeSyncSafe(header[6:10]))
		tagData := make([]byte, tagSize)
		if _, err := io.ReadFull(r, tagData); err == nil {
			parseID3Frames(tagData, version, meta)
		}
	}

	// If duration was not found in TLEN frame, estimate from audio frames
	if meta.DurationSeconds <= 0 && size > tagSize {
		audioDataSize := size - tagSize
		// Default estimate assuming 128 kbps if unmeasured
		meta.DurationSeconds = float64(audioDataSize*8) / 128000.0
		meta.BitrateKbps = 128
	}

	if meta.DurationSeconds > 0 && size > 0 && meta.BitrateKbps == 0 {
		meta.BitrateKbps = int((float64(size*8) / meta.DurationSeconds) / 1000)
	}

	// Default fallback chapter if none found
	if len(meta.Chapters) == 0 && meta.DurationSeconds > 0 {
		meta.Chapters = []ChapterInfo{
			{
				Title:          "Track 1",
				StartOffsetSec: 0,
				DurationSec:    meta.DurationSeconds,
			},
		}
	}

	return meta, nil
}

func decodeSyncSafe(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return uint32(b[0]&0x7F)<<21 |
		uint32(b[1]&0x7F)<<14 |
		uint32(b[2]&0x7F)<<7 |
		uint32(b[3]&0x7F)
}

func parseID3Frames(data []byte, version byte, meta *Metadata) {
	offset := 0
	for offset+10 <= len(data) {
		frameID := string(data[offset : offset+4])
		// Check for null padding
		if frameID[0] == 0 {
			break
		}

		var frameSize int
		if version == 4 {
			frameSize = int(decodeSyncSafe(data[offset+4 : offset+8]))
		} else {
			frameSize = int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		}

		offset += 10
		if offset+frameSize > len(data) || frameSize <= 0 {
			break
		}

		frameData := data[offset : offset+frameSize]
		offset += frameSize

		textVal := decodeID3Text(frameData)

		switch frameID {
		case "TIT2":
			if meta.Title == "" {
				meta.Title = textVal
			}
		case "TPE1":
			if meta.Author == "" {
				meta.Author = textVal
			}
		case "TPE2", "TCOM":
			if meta.Narrator == "" {
				meta.Narrator = textVal
			}
		case "TALB":
			if meta.Title == "" {
				meta.Title = textVal
			}
		case "TDRC", "TYER":
			meta.PublishedDate = textVal
		case "COMM":
			if meta.Description == "" {
				meta.Description = textVal
			}
		case "TLEN":
			if ms, err := strconv.ParseFloat(textVal, 64); err == nil && ms > 0 {
				meta.DurationSeconds = ms / 1000.0
			}
		case "APIC":
			parseAPICFrame(frameData, meta)
		case "CHAP":
			parseCHAPFrame(frameData, meta)
		}
	}
}

func decodeID3Text(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	encoding := data[0]
	raw := data[1:]
	switch encoding {
	case 0: // ISO-8859-1
		return strings.Trim(string(raw), "\x00 \t\n\r")
	case 1: // UTF-16 with BOM
		return strings.Trim(string(raw), "\x00 \t\n\r")
	case 2: // UTF-16BE without BOM
		return strings.Trim(string(raw), "\x00 \t\n\r")
	case 3: // UTF-8
		return strings.Trim(string(raw), "\x00 \t\n\r")
	default:
		return strings.Trim(string(raw), "\x00 \t\n\r")
	}
}

func parseAPICFrame(data []byte, meta *Metadata) {
	if len(data) < 4 {
		return
	}
	// data[0]: encoding
	offset := 1
	// MIME type string terminated with 0x00
	mimeEnd := bytes.IndexByte(data[offset:], 0x00)
	if mimeEnd == -1 {
		return
	}
	mimeType := string(data[offset : offset+mimeEnd])
	offset += mimeEnd + 1

	if offset >= len(data) {
		return
	}
	// data[offset]: picture type (0x03 is cover front)
	offset++

	// Description string null-terminated
	descEnd := bytes.IndexByte(data[offset:], 0x00)
	if descEnd != -1 {
		offset += descEnd + 1
	}

	if offset < len(data) {
		meta.CoverData = data[offset:]
		if mimeType != "" {
			meta.CoverMimeType = mimeType
		} else {
			meta.CoverMimeType = "image/jpeg"
		}
	}
}

func parseCHAPFrame(data []byte, meta *Metadata) {
	// Element ID null-terminated
	idEnd := bytes.IndexByte(data, 0x00)
	if idEnd == -1 || len(data) < idEnd+17 {
		return
	}
	offset := idEnd + 1
	startMs := binary.BigEndian.Uint32(data[offset : offset+4])
	endMs := binary.BigEndian.Uint32(data[offset+4 : offset+8])
	offset += 16

	title := fmt.Sprintf("Chapter %d", len(meta.Chapters)+1)
	if offset+10 <= len(data) {
		subFrameID := string(data[offset : offset+4])
		subSize := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		offset += 10
		if subFrameID == "TIT2" && offset+subSize <= len(data) {
			title = decodeID3Text(data[offset : offset+subSize])
		}
	}

	startSec := float64(startMs) / 1000.0
	durSec := float64(endMs-startMs) / 1000.0
	if durSec < 0 {
		durSec = 0
	}

	meta.Chapters = append(meta.Chapters, ChapterInfo{
		Title:          title,
		StartOffsetSec: startSec,
		DurationSec:    durSec,
	})
}
