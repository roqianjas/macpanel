package web

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
)

// Hanya berkas yang dikenal yang disajikan: tidak ada listing folder, tidak ada traversal, tidak ada dotfile.
var assetRE = regexp.MustCompile(`^/assets/[A-Za-z0-9][A-Za-z0-9._-]{0,120}$`)

var staticTop = map[string]bool{"/index.html": true, "/favicon.svg": true, "/theme.js": true}

var extType = map[string]string{
	".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".svg": "image/svg+xml",
	".woff2": "font/woff2", ".woff": "font/woff", ".html": "text/html; charset=utf-8",
}

func ServeStatic(a *core.App, w http.ResponseWriter, r *http.Request) {
	core.SecHeaders(w)
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		core.WriteError(w, 405, "method not allowed")
		return
	}
	p := r.URL.Path
	if p == "/" {
		p = "/index.html"
	}
	if a.UIDir == "" || !(staticTop[p] || assetRE.MatchString(p)) || strings.Contains(p, "..") {
		core.WriteJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	f, err := os.Open(filepath.Join(a.UIDir, filepath.FromSlash(p)))
	if err != nil {
		core.WriteJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		core.WriteJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	ct := extType[strings.ToLower(filepath.Ext(p))]
	if ct == "" {
		ct = mime.TypeByExtension(filepath.Ext(p))
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	if strings.HasPrefix(p, "/assets/") { // nama berkas memuat hash isi: aman di-cache lama
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, "", st.ModTime(), f)
}
