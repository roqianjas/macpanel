package deploy

import (
	"net/http"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var deploySteps = map[string]bool{"git_pull": true, "composer_install": true, "composer_install_nodev": true, "composer_update": true, "dump_autoload": true,
	"node_install": true, "node_build": true, "artisan_migrate": true, "artisan_optimize": true, "fpm_restart": true}

type DeployInfo struct {
	Site  string `json:"site"`
	Dir   string `json:"dir"`
	PHP   string `json:"php"`
	Steps []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"steps"`
	Git *struct {
		Branch string `json:"branch"`
		Commit string `json:"commit"`
		Dirty  bool   `json:"dirty"`
		Remote bool   `json:"remote"`
	} `json:"git"`
}

func DeployInfoHandler(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name := r.PathValue("name")
	if !core.SiteRE.MatchString(name) || core.ProtectedSites[name] {
		core.WriteError(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	res := a.RunLab([]string{"deploy", "info", name}, "")
	var d DeployInfo
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &d) {
		msg := strings.TrimSpace(strings.TrimPrefix(core.Scrub(res.Out), "lab:"))
		if len(msg) > 160 || msg == "" {
			msg = "gagal membaca informasi deploy"
		}
		core.WriteError(w, 502, msg)
		return
	}
	core.WriteJSON(w, 200, d)
}

func DeployRun(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name := r.PathValue("name")
	if !core.SiteRE.MatchString(name) || core.ProtectedSites[name] {
		core.WriteError(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	var in struct{ Steps []string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if len(in.Steps) == 0 || len(in.Steps) > 8 {
		core.WriteError(w, 400, "pilih 1-8 langkah")
		return
	}
	for _, s := range in.Steps {
		if !deploySteps[s] {
			core.WriteError(w, 400, "langkah tidak dikenal: "+s)
			return
		}
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "deploy", name+" ["+strings.Join(in.Steps, ",")+"]", []string{"deploy", "run", name, strings.Join(in.Steps, ",")}, ""))
}
