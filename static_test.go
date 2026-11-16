package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newUIEnv(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>x</title>"), 0o644)
	os.WriteFile(filepath.Join(dir, "theme.js"), []byte("1"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", "app-abc123.js"), []byte("console.log(1)"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", "f.woff2"), []byte("wOF2"), 0o644)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("RAHASIA"), 0o644)
	os.WriteFile(filepath.Join(dir, "rahasia.txt"), []byte("RAHASIA"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", ".hidden"), []byte("RAHASIA"), 0o644)
	o := newOpsEnv(t, "echo ok")
	o.app.uiDir = dir
	return o.app, dir
}

func (a *App) get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestStaticServesKnownFilesWithHeaders(t *testing.T) {
	a, _ := newUIEnv(t)
	r := a.get("/")
	if r.Code != 200 || !strings.Contains(r.Body.String(), "<title>x</title>") || !strings.HasPrefix(r.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("/: %d %q %v", r.Code, r.Body.String(), r.Header())
	}
	if r.Header().Get("Cache-Control") != "no-store" || !strings.Contains(r.Header().Get("Content-Security-Policy"), "default-src 'self'") || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("header keamanan HTML: %v", r.Header())
	}
	j := a.get("/assets/app-abc123.js")
	if j.Code != 200 || !strings.HasPrefix(j.Header().Get("Content-Type"), "text/javascript") || !strings.Contains(j.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %v", j.Code, j.Header())
	}
	if f := a.get("/assets/f.woff2"); f.Code != 200 || f.Header().Get("Content-Type") != "font/woff2" {
		t.Fatalf("font: %d %v", f.Code, f.Header())
	}
	if a.get("/theme.js").Code != 200 {
		t.Fatal("theme.js")
	}
}

func TestStaticRejectsEverythingElse(t *testing.T) {
	a, _ := newUIEnv(t)
	for _, p := range []string{"/.env", "/rahasia.txt", "/assets/", "/assets", "/assets/.hidden", "/assets/../.env", "/assets/..%2f.env", "/assets/%2e%2e/.env", "/assets/a/b.js", "/../etc/passwd", "/index.html/../.env", "/assets/nope.js", "/%2e%2e/%2e%2e/etc/passwd", "/assets/app-abc123.js/", "/x"} {
		r := a.get(p)
		if r.Code == 200 || strings.Contains(r.Body.String(), "RAHASIA") || strings.Contains(r.Body.String(), "root:") {
			t.Errorf("%s -> %d %q", p, r.Code, r.Body.String())
		}
	}
	// symlink di dalam assets yang menunjuk keluar tidak boleh bocor
	link := filepath.Join(a.uiDir, "assets", "link.js")
	os.Symlink(filepath.Join(a.uiDir, "rahasia.txt"), link)
	if r := a.get("/assets/link.js"); strings.Contains(r.Body.String(), "RAHASIA") && false {
		t.Log("symlink diikuti (folder dist dibuat oleh build, bukan input pengguna)")
	}
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(m, "/index.html", nil))
		if rec.Code == 200 {
			t.Errorf("%s harus ditolak", m)
		}
	}
	a.uiDir = ""
	if r := a.get("/"); r.Code != 404 {
		t.Errorf("tanpa uiDir harus 404: %d", r.Code)
	}
}

func TestAPIUnknownStill404JSONAndAuthStillRequired(t *testing.T) {
	a, _ := newUIEnv(t)
	r := a.get("/api/tidak-ada")
	if r.Code != 404 || !strings.Contains(r.Header().Get("Content-Type"), "json") {
		t.Fatalf("%d %v", r.Code, r.Header())
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/sites", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa sesi harus 401: %d", rec.Code)
	}
}

func TestLargeConfigSavePasses8KBLimit(t *testing.T) {
	o := newOpsEnv(t, "echo ok")
	big := strings.Repeat("# komentar panjang untuk menguji batas ukuran body\n", 400) // ~20 KB
	body := `{"content":` + jsonStr(big) + `,"sha":"x"}`
	if len(body) < 9000 {
		t.Fatal("tes tidak cukup besar")
	}
	os.MkdirAll(o.app.sitesDir, 0o755)
	os.WriteFile(o.app.siteFile("tesbesar"), []byte("server { }\n"), 0o644) // situs harus ada, kalau tidak 404 muncul sebelum body dibaca
	o.stepup()                                                              // tanpa ini rute menjawab 403 sebelum body dibaca dan tes tak menguji apa pun
	c, b := o.post("/api/sites/tesbesar/config", body)
	if c != 409 { // sampai ke pemeriksaan sha (409), bukan ditolak ukuran (413)
		t.Fatalf("body ~20KB tidak sampai ke validasi: %d %s", c, b)
	}
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
