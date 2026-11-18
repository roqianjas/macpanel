package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const mod = "github.com/roqianjas/macpanel/"

// Aturan ketergantungan (lihat docs/architecture): lapisan bawah tidak boleh mengimpor lapisan atas,
// dan paket domain tidak saling mengimpor. Tes ini gagal bila aturan dilanggar.
var layer = map[string]int{
	"internal/crypto": 0,
	"internal/store":  1,
	"internal/core":   2,
	// domain: lapisan 3 (semua yang tidak disebut di sini)
	"internal/overview": 4, // agregator: boleh membaca beberapa domain
	"internal/server":   5, // akar komposisi: merangkai semua domain
	"cmd/panel":         6,
	"internal/testenv":  6, // utilitas tes
}

func layerOf(dir string) int {
	if l, ok := layer[dir]; ok {
		return l
	}
	return 3
}

func imports(t *testing.T, dir string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range f.Imports {
			if path := strings.Trim(im.Path.Value, `"`); strings.HasPrefix(path, mod) {
				out[strings.TrimPrefix(path, mod)] = true
			}
		}
	}
	return out
}

func TestDependencyRules(t *testing.T) {
	root := filepath.Join("..", "..")
	var dirs []string
	for _, base := range []string{"internal", "cmd"} {
		filepath.WalkDir(filepath.Join(root, base), func(p string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				if m, _ := filepath.Glob(filepath.Join(p, "*.go")); len(m) > 0 {
					rel, _ := filepath.Rel(root, p)
					dirs = append(dirs, filepath.ToSlash(rel))
				}
			}
			return nil
		})
	}
	sort.Strings(dirs)
	if len(dirs) < 10 {
		t.Fatalf("terlalu sedikit paket ditemukan (%d); path salah?", len(dirs))
	}
	for _, dir := range dirs {
		if dir == "internal/archtest" || dir == "internal/testenv" {
			continue
		}
		from := layerOf(dir)
		for imp := range imports(t, filepath.Join(root, dir)) {
			to := layerOf(imp)
			switch {
			case to > from:
				t.Errorf("%s (lapisan %d) mengimpor %s (lapisan %d): arah ketergantungan terbalik", dir, from, imp, to)
			case from == 3 && to == 3:
				t.Errorf("%s mengimpor %s: paket domain tidak boleh saling mengimpor (pindahkan yang dipakai bersama ke internal/core)", dir, imp)
			case from == to && dir != imp && from != 4:
				t.Errorf("%s mengimpor %s pada lapisan yang sama", dir, imp)
			}
		}
	}
}

func TestSQLOnlyInStore(t *testing.T) {
	// satu-satunya tempat yang boleh mengimpor database/sql dan driver SQLite adalah internal/store
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if strings.HasPrefix(filepath.ToSlash(rel), "internal/store/") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			t.Error(err)
			return nil
		}
		for _, im := range f.Imports {
			if path := strings.Trim(im.Path.Value, `"`); path == "database/sql" || strings.HasPrefix(path, "modernc.org/sqlite") {
				t.Errorf("%s mengimpor %s: SQL hanya boleh di internal/store", rel, path)
			}
		}
		return nil
	})
}
