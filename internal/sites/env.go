package sites

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

const MaxEnv = 64 << 10

func envPath(a *core.App, name string) (string, error) {
	st, err := readSite(a, name)
	if err != nil {
		return "", errors.New("situs tidak ada")
	}
	if st.Model == nil || (st.Model.Mode != "php" && st.Model.Mode != "statis") {
		return "", errors.New("situs ini tidak punya folder proyek yang dikenali")
	}
	root, err := a.UnderDev(st.Model.Target)
	if err != nil {
		return "", err
	}
	dir := root
	if filepath.Base(root) == "public" {
		dir = filepath.Dir(root)
	}
	if dir, err = a.UnderDev(dir); err != nil {
		return "", errors.New("folder proyek harus di bawah ~/MacPanel/wwwroot")
	}
	return filepath.Join(dir, ".env"), nil
}

func EnvGet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	if ss.Role == "viewer" {
		core.WriteError(w, 403, "hanya admin")
		return
	}
	p, err := envPath(a, name)
	if err != nil {
		core.WriteError(w, 400, err.Error())
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	fi, err := os.Lstat(p)
	if os.IsNotExist(err) {
		core.WriteJSON(w, 200, map[string]any{"exists": false, "content": "", "path": filepath.Base(filepath.Dir(p)) + "/.env"})
		return
	}
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > MaxEnv {
		core.WriteError(w, 400, ".env bukan berkas biasa atau lebih dari 64 KB")
		return
	}
	b, _ := os.ReadFile(p)
	a.Store.Audit(ss.User, "env_read", name, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"exists": true, "content": string(b), "path": filepath.Base(filepath.Dir(p)) + "/.env"})
}

func EnvSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ Content string }
	if !core.DecodeJSONLimit(w, r, &in, 2*MaxEnv+1024) {
		return
	}
	if len(in.Content) > MaxEnv || strings.ContainsRune(in.Content, 0) {
		core.WriteError(w, 400, "isi .env maksimal 64 KB tanpa karakter NUL")
		return
	}
	p, err := envPath(a, name)
	if err != nil {
		core.WriteError(w, 400, err.Error())
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Mu.Lock()
	defer a.Mu.Unlock()
	if fi, err := os.Lstat(p); err == nil {
		if !fi.Mode().IsRegular() {
			core.WriteError(w, 400, ".env bukan berkas biasa")
			return
		}
		old, _ := os.ReadFile(p)
		bak := p + ".bak." + a.Now().Format("20060102-150405")
		if err := os.WriteFile(bak, old, 0o600); err != nil {
			core.WriteError(w, 500, "gagal membuat cadangan")
			return
		}
		baks, _ := filepath.Glob(p + ".bak.*")
		sort.Strings(baks)
		for len(baks) > 3 {
			_ = os.Remove(baks[0])
			baks = baks[1:]
		}
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(in.Content), 0o600); err != nil {
		core.WriteError(w, 500, "gagal menulis")
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "env_write", name+" ("+fmt.Sprint(len(in.Content))+" byte)", a.ClientIP(r), a.Now())
	_ = time.Now
	core.WriteJSON(w, 200, map[string]any{"saved": true})
}
