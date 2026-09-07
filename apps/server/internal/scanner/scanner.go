package scanner

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// DiscoveredFile represents an EPUB discovered during library scanning.
type DiscoveredFile struct {
	FullPath     string
	RelativePath string
}

// Scanner crawls the library directory for EPUB files without mutating sidecars.
type Scanner struct {
	libraryDir string
}

// NewScanner creates a new Scanner instance for the specified library directory.
func NewScanner(libraryDir string) *Scanner {
	return &Scanner{libraryDir: libraryDir}
}

// Scan walks the library directory and returns all found EPUB files.
// It strictly ignores hidden files and non-EPUB files (never mutating Audiobookshelf assets).
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

		// Only process .epub files
		if !strings.EqualFold(filepath.Ext(d.Name()), ".epub") {
			return nil
		}

		relPath, err := filepath.Rel(s.libraryDir, path)
		if err != nil {
			return fmt.Errorf("calculating relative path: %w", err)
		}

		discovered = append(discovered, DiscoveredFile{
			FullPath:     path,
			RelativePath: relPath,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scanning library directory %s: %w", s.libraryDir, err)
	}

	return discovered, nil
}
