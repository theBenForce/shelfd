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
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".webp", "image/webp")
}

func setCacheHeaders(w http.ResponseWriter, filename string) {
	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.ToLower(filepath.Base(filename))
	if ext == ".html" || ext == "" || base == "main.dart.js" || base == "flutter.js" || base == "flutter_bootstrap.js" || base == "flutter_service_worker.js" || base == "manifest.json" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	} else {
		// Hashed and static assets: cache for 1 year
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
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
			setCacheHeaders(w, targetPath)
			http.ServeFile(w, r, targetPath)
			return
		}

		if err == nil && fi.IsDir() {
			// Directory: check for index.html inside
			indexPath := filepath.Join(targetPath, "index.html")
			if ifi, ierr := os.Stat(indexPath); ierr == nil && !ifi.IsDir() {
				setCacheHeaders(w, indexPath)
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
			setCacheHeaders(w, indexPath)
			http.ServeFile(w, r, indexPath)
			return
		}

		http.NotFound(w, r)
	}
}
