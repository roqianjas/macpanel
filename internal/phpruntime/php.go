package phpruntime

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type PHPInfo struct {
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	Full      string `json:"full,omitempty"`
	FPM       bool   `json:"fpm"`
}

var phpLineRE = regexp.MustCompile(`^(8\.[0-9])\s+terpasang=(ya|tidak)\s+versi=(\S+)\s+fpm_socket=(\S+)`)

func ParsePHP(out string) []PHPInfo {
	res := []PHPInfo{}
	for _, l := range strings.Split(out, "\n") {
		if m := phpLineRE.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			p := PHPInfo{Version: m[1], Installed: m[2] == "ya", FPM: m[4] == "ada"}
			if p.Installed {
				p.Full = m[3]
			}
			res = append(res, p)
		}
	}
	return res
}

func PHPList(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLab([]string{"php", "list"}, "")
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca daftar PHP")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"php": ParsePHP(res.Out)})
}

func phpVer(w http.ResponseWriter, r *http.Request) (string, bool) {
	v := r.PathValue("version")
	if !core.PhpVersions[v] {
		core.WriteError(w, 400, "versi PHP harus 8.2-8.5")
		return "", false
	}
	return v, true
}

func PHPInstall(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "php_install", v, []string{"php", "install", v}, ""))
}

func PHPRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Confirm != v {
		core.WriteError(w, 400, "ketik ulang versi PHP untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "php_remove", v, []string{"php", "remove", v}, ""))
}
