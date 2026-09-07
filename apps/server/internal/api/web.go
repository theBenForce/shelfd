package api

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".json", "application/json")
	_ = mime.AddExtensionType(".html", "text/html; charset=utf-8")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
}

// SPAHandler returns an http.HandlerFunc that serves static assets from webDir,
// falling back to index.html for non-asset paths to support SPA client-side routing.
func SPAHandler(webDir string) http.HandlerFunc {
	absWebDir, err := filepath.Abs(webDir)
	if err != nil {
		absWebDir = webDir
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		cleanPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		targetPath := filepath.Join(absWebDir, cleanPath)

		// Guard against directory traversal attacks
		if !strings.HasPrefix(targetPath, absWebDir) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		fi, err := os.Stat(targetPath)
		if err == nil && !fi.IsDir() {
			// Specific file exists
			http.ServeFile(w, r, targetPath)
			return
		}

		if err == nil && fi.IsDir() {
			// Directory: check for index.html inside
			indexPath := filepath.Join(targetPath, "index.html")
			if ifi, ierr := os.Stat(indexPath); ierr == nil && !ifi.IsDir() {
				http.ServeFile(w, r, indexPath)
				return
			}
		}

		// If path has an asset file extension (e.g. .png, .js, .wasm, .css) and wasn't found, 404
		if filepath.Ext(cleanPath) != "" {
			http.NotFound(w, r)
			return
		}

		// Fallback to root index.html for SPA client-side routing (e.g. /library, /connect)
		indexPath := filepath.Join(absWebDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}

		http.NotFound(w, r)
	}
}
