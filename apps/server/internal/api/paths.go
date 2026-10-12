package api

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveStoredPath resolves a stored database path (relative to libraryDir, dataDir, or absolute).
func resolveStoredPath(libraryDir, dataDir, storedPath string) string {
	if storedPath == "" {
		return ""
	}
	if filepath.IsAbs(storedPath) {
		if fi, err := os.Stat(storedPath); err == nil && !fi.IsDir() {
			return storedPath
		}
	}
	libPath := filepath.Join(libraryDir, storedPath)
	if fi, err := os.Stat(libPath); err == nil && !fi.IsDir() {
		return libPath
	}
	dataPath := filepath.Join(dataDir, storedPath)
	if fi, err := os.Stat(dataPath); err == nil && !fi.IsDir() {
		return dataPath
	}
	if fi, err := os.Stat(storedPath); err == nil && !fi.IsDir() {
		return storedPath
	}
	if !filepath.IsAbs(storedPath) && libraryDir != "" {
		return libPath
	}
	return storedPath
}

// isUnderDir checks if target is located inside baseDir.
func isUnderDir(target, baseDir string) bool {
	if target == "" || baseDir == "" {
		return false
	}
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
