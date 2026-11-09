// Package web serves the compiled frontend (React/Vite build) and provides
// the SPA fallback used by the application router.
package web

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Frontend serves files from a Vite build directory. Requests for existing
// files are served directly; all other non-API requests fall back to
// index.html so client-side routing works.
type Frontend struct {
	root      string
	indexFile string
	available bool
	files     http.Handler
	logger    *log.Logger
}

// NewFrontend creates a handler for the given build directory. A missing
// directory or index.html is not fatal: the API stays available and frontend
// requests receive HTTP 503 until the build is deployed.
func NewFrontend(root string, logger *log.Logger) *Frontend {
	if logger == nil {
		logger = log.New(io.Discard, "", log.LstdFlags)
	}
	if strings.TrimSpace(root) == "" {
		root = "frontend/dist"
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	// Resolve symlinks in the root once so containment checks compare real paths.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	f := &Frontend{
		root:      abs,
		indexFile: filepath.Join(abs, "index.html"),
		files:     http.FileServer(http.Dir(abs)),
		logger:    logger,
	}

	dirInfo, err := os.Stat(abs)
	if err != nil || !dirInfo.IsDir() {
		logger.Printf("frontend build directory unavailable path=%s; SPA requests will return 503", abs)
		return f
	}
	indexInfo, err := os.Stat(f.indexFile)
	if err != nil || indexInfo.IsDir() {
		logger.Printf("frontend index.html unavailable path=%s; SPA requests will return 503", f.indexFile)
		return f
	}

	f.available = true
	logger.Printf("serving frontend path=%s", abs)
	return f
}

// ServeHTTP implements http.Handler.
func (f *Frontend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Never let backend endpoints reach the SPA fallback, even when the router
	// forwards an unknown /api path to this handler.
	if isBackendPath(r.URL.Path) {
		writeAPI404(w)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !f.available {
		http.Error(w, "frontend is not built", http.StatusServiceUnavailable)
		return
	}

	clean, ok := cleanRequestPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if clean == "/" {
		f.serveIndex(w, r)
		return
	}

	filePath, ok := f.resolve(clean)
	if !ok {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(filePath)
	if err == nil {
		if !info.Mode().IsRegular() {
			// Never expose directory listings.
			http.NotFound(w, r)
			return
		}
		f.setFileHeaders(w, clean)
		// Hand the FileServer a cleaned path so it cannot escape the root.
		req := r.Clone(r.Context())
		req.URL.Path = clean
		f.files.ServeHTTP(w, req)
		return
	}
	if !os.IsNotExist(err) {
		f.logger.Printf("frontend stat failed path=%s err=%v", clean, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Missing files with an extension are real 404s. Falling back to
	// index.html here would turn broken asset references into HTML responses.
	if path.Ext(clean) != "" {
		http.NotFound(w, r)
		return
	}

	// Client-side route (e.g. /servers/42): serve the SPA shell.
	f.serveIndex(w, r)
}

func (f *Frontend) serveIndex(w http.ResponseWriter, r *http.Request) {
	// index.html must never be cached, otherwise clients keep referencing old
	// hashed asset names after a deployment.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, f.indexFile)
}

func (f *Frontend) setFileHeaders(w http.ResponseWriter, clean string) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(clean, "/assets/") {
		// Vite emits content-hashed filenames under /assets/, so they are safe
		// to cache for a long time.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
}

// resolve maps a cleaned URL path to a file below the build root and returns
// false for anything that would escape it (including via symlinks).
func (f *Frontend) resolve(clean string) (string, bool) {
	rel := strings.TrimPrefix(clean, "/")
	candidate := filepath.Join(f.root, filepath.FromSlash(rel))
	if !within(f.root, candidate) {
		return "", false
	}

	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// Missing files are handled by the caller; symlink evaluation can only
		// fail here when the file does not exist yet.
		return candidate, true
	}
	if !within(f.root, resolved) {
		return "", false
	}
	return resolved, true
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

// cleanRequestPath normalizes a URL path and rejects traversal attempts.
func cleanRequestPath(p string) (string, bool) {
	if strings.ContainsRune(p, '\x00') {
		return "", false
	}
	// Treat backslashes as separators too; on Linux they are valid filename
	// characters, but rejecting them as traversal is safe and portable.
	p = strings.ReplaceAll(p, "\\", "/")
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", false
		}
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p), true
}

func isBackendPath(p string) bool {
	return p == "/health" || p == "/api" || strings.HasPrefix(p, "/api/")
}

func writeAPI404(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, `{"error":"not found"}`+"\n")
}
