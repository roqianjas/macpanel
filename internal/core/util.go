package core

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/store"
)

func StatDir(p string) (bool, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

func IntParam(w http.ResponseWriter, s string, lo, hi int) (int, bool, bool) {
	if s == "" {
		return 0, false, true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi || strconv.Itoa(n) != s {
		WriteError(w, 400, "angka di luar batas")
		return 0, false, false
	}
	return n, true, true
}

var RuntimeTools = map[string]bool{"node": true, "go": true, "python": true, "pnpm": true}

func AdminOnly(w http.ResponseWriter, ss *store.Session) bool {
	if !HumanOnly(w, ss) {
		return false
	}
	if ss.Role != "admin" {
		WriteError(w, 403, "hanya admin")
		return false
	}
	return true
}

// ParseLabJSON membaca JSON dari keluaran lab (stdout dan stderr tergabung): ambil dari '{' pertama.
func ParseLabJSON(out string, v any) bool {
	i := strings.Index(out, "{")
	if i < 0 {
		return false
	}
	return json.NewDecoder(strings.NewReader(out[i:])).Decode(v) == nil
}

func SortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// local mengembalikan state milik satu domain pada App ini: dibuat sekali per App dan aman dipakai goroutine.
// Domain tidak menambah field ke App; mereka menyimpan state sendiri di sini.
func Local[T any](a *App, key string) *T {
	v, _ := a.ext.LoadOrStore(key, new(T))
	return v.(*T)
}

func (a *App) SiteFile(name string) string { return filepath.Join(a.SitesDir, name+".conf") }

func (a *App) SiteParam(w http.ResponseWriter, r *http.Request, write bool) (string, bool) {
	name := r.PathValue("name")
	if !SiteRE.MatchString(name) {
		WriteError(w, 400, "nama situs tidak valid")
		return "", false
	}
	if _, err := os.Stat(a.SiteFile(name)); err != nil {
		WriteError(w, 404, "situs tidak ada")
		return "", false
	}
	if write && ProtectedSites[name] {
		WriteError(w, 400, "situs sistem hanya bisa dilihat, tidak diubah dari panel")
		return "", false
	}
	return name, true
}

var LogN = regexp.MustCompile(`^[0-9]{1,3}$`)
