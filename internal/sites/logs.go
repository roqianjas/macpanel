package sites

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func SiteStats(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	core.LabJSON(a, w, []string{"sys", "stats", name}, 60e9, "gagal membaca log situs")
}

func tailLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, _ := f.Stat()
	const win = 1 << 20
	off := int64(0)
	if st.Size() > win {
		off = st.Size() - win
	}
	_, _ = f.Seek(off, io.SeekStart)
	b, _ := io.ReadAll(f)
	if off > 0 {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func Logs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	n := 100
	if q := r.URL.Query().Get("n"); q != "" {
		if !core.LogN.MatchString(q) {
			core.WriteError(w, 400, "n tidak valid")
			return
		}
		n, _ = strconv.Atoi(q)
		if n < 1 || n > 500 {
			core.WriteError(w, 400, "n 1-500")
			return
		}
	}
	kind := r.URL.Query().Get("kind")
	var lines []string
	var err error
	switch kind {
	case "access":
		lines, err = tailLines(a.LogDir+"/"+name+".log", n)
	case "error":
		var all []string
		all, err = tailLines(a.ErrLog, 5000)
		host := "server: " + name + "." + a.TLD
		for _, l := range all {
			if strings.Contains(l, host) {
				lines = append(lines, l)
			}
		}
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	default:
		core.WriteError(w, 400, "kind: access atau error")
		return
	}
	if err != nil || lines == nil {
		lines = []string{}
	}
	for i := range lines {
		lines[i] = core.Scrub(store.Clean(lines[i], 600))
	}
	core.WriteJSON(w, 200, map[string]any{"kind": kind, "lines": lines, "note": "log akses per situs aktif setelah pengaturan disimpan lewat panel"})
}
