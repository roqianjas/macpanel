package docs

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

// Dokumentasi: berkas Markdown di <root>/docs. Hanya dibaca; nama berkas dibatasi pola, ukuran dibatasi.
var docNameRE = regexp.MustCompile(`^\d\d-[a-z0-9-]{1,60}\.md$`)

const (
	DocMaxBytes = 400 << 10
	DocMaxFiles = 40
)

type DocItem struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func listDocs(a *core.App) []DocItem {
	out := []DocItem{}
	files, _ := filepath.Glob(filepath.Join(a.Root, "docs", "*.md"))
	sort.Strings(files)
	for _, f := range files {
		base := filepath.Base(f)
		if !docNameRE.MatchString(base) || len(out) >= DocMaxFiles {
			continue
		}
		fi, err := os.Lstat(f)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > DocMaxBytes {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(base, ".md")
		title := name
		for _, ln := range strings.Split(string(b), "\n") {
			if t := strings.TrimSpace(ln); strings.HasPrefix(t, "# ") {
				title = strings.TrimSpace(t[2:])
				break
			}
		}
		out = append(out, DocItem{Name: name, Title: title, Content: string(b)})
	}
	return out
}

func Docs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	w.Header().Set("Cache-Control", "no-store")
	core.WriteJSON(w, 200, map[string]any{"docs": listDocs(a)})
}
