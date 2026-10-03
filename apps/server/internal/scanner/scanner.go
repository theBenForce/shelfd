package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Supported format mappings
var (
	EbookExtensions = map[string]string{
		".epub": "epub",
	}
	AudioExtensions = map[string]string{
		".m4b":  "audiobook",
		".m4a":  "audiobook",
		".mp3":  "audiobook",
		".flac": "audiobook",
	}
	DocumentExtensions = map[string]string{
		".pdf": "pdf",
	}
	CoverExtensions = map[string]string{
		".jpg":  "cover",
		".jpeg": "cover",
		".png":  "cover",
		".webp": "cover",
	}
)

// DetectFileType returns the normalized file_type and whether the file is a primary catalog item.
func DetectFileType(fileName string) (fileType string, isPrimary bool) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if t, ok := EbookExtensions[ext]; ok {
		return t, true
	}
	if t, ok := AudioExtensions[ext]; ok {
		return t, true
	}
	if t, ok := DocumentExtensions[ext]; ok {
		return t, true
	}
	if t, ok := CoverExtensions[ext]; ok {
		return t, false
	}
	return "other", false
}

// DiscoveredFile represents a media file discovered during library scanning.
type DiscoveredFile struct {
	FullPath     string
	RelativePath string
	ModTime      time.Time
	SizeBytes    int64
	FileType     string
}

// Scanner crawls the library directory for media files without mutating sidecars.
type Scanner struct {
	libraryDir string
}

// NewScanner creates a new Scanner instance for the specified library directory.
func NewScanner(libraryDir string) *Scanner {
	return &Scanner{libraryDir: libraryDir}
}

// Scan walks the library directory and returns all primary media files (.epub, .m4b, .mp3, .m4a, .flac, .pdf).
// It strictly ignores hidden files (e.g. .metadata.json, .DS_Store) without mutating Audiobookshelf sidecars.
func (s *Scanner) Scan() ([]DiscoveredFile, error) {
	var discovered []DiscoveredFile

	err := filepath.WalkDir(s.libraryDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories (e.g., .git, .stversions)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != s.libraryDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files (e.g. .metadata.json, .DS_Store)
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		fileType, isPrimary := DetectFileType(d.Name())
		if !isPrimary {
			return nil
		}

		relPath, err := filepath.Rel(s.libraryDir, path)
		if err != nil {
			return fmt.Errorf("calculating relative path: %w", err)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("getting file info for %s: %w", path, err)
		}

		discovered = append(discovered, DiscoveredFile{
			FullPath:     path,
			RelativePath: relPath,
			ModTime:      info.ModTime(),
			SizeBytes:    info.Size(),
			FileType:     fileType,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scanning library directory %s: %w", s.libraryDir, err)
	}

	return discovered, nil
}

// ScanBookDirectory scans a specific book directory and returns all associated physical files.
func (s *Scanner) ScanBookDirectory(bookDirFullPath string) ([]DiscoveredFile, error) {
	entries, err := os.ReadDir(bookDirFullPath)
	if err != nil {
		return nil, fmt.Errorf("reading book directory %s: %w", bookDirFullPath, err)
	}

	var files []DiscoveredFile
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(bookDirFullPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fileType, _ := DetectFileType(entry.Name())
		relPath, err := filepath.Rel(s.libraryDir, path)
		if err != nil {
			relPath = entry.Name()
		}

		files = append(files, DiscoveredFile{
			FullPath:     path,
			RelativePath: relPath,
			ModTime:      info.ModTime(),
			SizeBytes:    info.Size(),
			FileType:     fileType,
		})
	}
	return files, nil
}

