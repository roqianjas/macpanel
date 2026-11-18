package server_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqianjas/macpanel/internal/docs"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestDocsListSafe(t *testing.T) {
	o := testenv.NewOpsEnv(t, "echo ok")
	d := filepath.Join(o.Home, "docs")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, "01-satu.md"), []byte("# Judul Satu\nisi"), 0o644)
	os.WriteFile(filepath.Join(d, "02-dua.md"), []byte("tanpa judul"), 0o644)
	os.WriteFile(filepath.Join(d, "02-dua.md.bak.20261117"), []byte("cadangan"), 0o644)
	os.WriteFile(filepath.Join(d, "..evil.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(d, "Besar.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(d, "03-besar.md"), []byte(strings.Repeat("a", docs.DocMaxBytes+1)), 0o644)
	os.WriteFile(filepath.Join(o.Home, "rahasia.md"), []byte("RAHASIA"), 0o644)
	os.Symlink(filepath.Join(o.Home, "rahasia.md"), filepath.Join(d, "04-link.md"))
	w := o.Do("GET", "/api/docs", "", o.Cookie, "", nil)
	b := w.Body.String()
	if w.Code != 200 || !strings.Contains(b, "Judul Satu") || !strings.Contains(b, `"title":"02-dua"`) {
		t.Fatalf("daftar: %d %s", w.Code, b)
	}
	for _, bad := range []string{"cadangan", "evil", "Besar", "03-besar", "RAHASIA", "04-link"} {
		if strings.Contains(b, bad) {
			t.Fatalf("%q tidak boleh ikut: %s", bad, b)
		}
	}
	if c := w.Header().Get("Cache-Control"); c != "no-store" {
		t.Fatalf("cache: %q", c)
	}
	if w := o.Do("GET", "/api/docs", "", "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa sesi: %d", w.Code)
	}
}
