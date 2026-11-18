package phpruntime

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type RuntimeInfo struct {
	Tool    string `json:"tool"`
	Version string `json:"version"`
	Default bool   `json:"default"`
	Pruned  bool   `json:"pruned"`
}

var runtimeLineRE = regexp.MustCompile(`^(node|go|python|pnpm)\s+(\S+)(.*)$`)

func ParseRuntime(out string) []RuntimeInfo {
	res := []RuntimeInfo{}
	for _, l := range strings.Split(out, "\n") {
		if m := runtimeLineRE.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			res = append(res, RuntimeInfo{Tool: m[1], Version: m[2], Default: strings.Contains(m[3], "config.toml"), Pruned: strings.Contains(m[3], "pruned")})
		}
	}
	return res
}

var versionRE = regexp.MustCompile(`^(latest|lts|[0-9]+(\.[0-9]+){0,2})$`)

func RuntimeList(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLab([]string{"runtime", "list"}, "")
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca daftar runtime")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"runtimes": ParseRuntime(res.Out), "tools": []string{"node", "go", "python", "pnpm"}})
}

func rtParams(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	t, v := r.PathValue("tool"), r.PathValue("version")
	if !core.RuntimeTools[t] || !versionRE.MatchString(v) {
		core.WriteError(w, 400, "tool atau versi tidak valid")
		return "", "", false
	}
	return t, v, true
}

func RuntimeInstall(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	t, v, ok := rtParams(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "runtime_install", t+"@"+v, []string{"runtime", "install", t, v}, ""))
}

func RuntimeUse(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	t, v, ok := rtParams(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "runtime_use", t+"@"+v, a.Mutate([]string{"runtime", "use", t, v}, ""))
}

func RuntimeRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	t, v, ok := rtParams(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Confirm != t+"@"+v {
		core.WriteError(w, 400, "ketik ulang "+t+"@"+v+" untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "runtime_remove", t+"@"+v, a.Mutate([]string{"runtime", "remove", t, v}, ""))
}
