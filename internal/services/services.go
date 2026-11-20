package services

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type ServiceInfo struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Version   string `json:"version"`
	State     string `json:"state"`
	Port      int    `json:"port,omitempty"`
	Installed bool   `json:"installed"`
	Ctl       bool   `json:"ctl"`
	Optional  bool   `json:"optional"`
	Desc      string `json:"desc"`
	HasLog    bool   `json:"has_log"`
}

var svcIDRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,18}[a-z0-9])?(\.[0-9]{1,2})?$`)

var svcActions = map[string]bool{"start": true, "stop": true, "restart": true}

var optionalApp = map[string]bool{"memcached": true, "mailpit": true, "9router": true, "docker": true, "httpd": true, "cloudflared": true}

var SvcStates = map[string]bool{"running": true, "stopped": true, "not_installed": true, "error": true}

func ServicesList(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLabTimeout([]string{"service", "list", "--json"}, "", 40*time.Second)
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca daftar layanan")
		return
	}
	rows := []ServiceInfo{}
	if err := json.Unmarshal([]byte(res.Out), &rows); err != nil {
		core.WriteError(w, 502, "keluaran layanan tidak terbaca")
		return
	}
	for i := range rows { // nilai dari lab dibatasi lagi di sisi panel
		if !SvcStates[rows[i].State] {
			rows[i].State = "error"
		}
	}
	core.WriteJSON(w, 200, map[string]any{"services": rows})
}

func svcParams(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !svcIDRE.MatchString(id) {
		core.WriteError(w, 400, "id layanan tidak valid")
		return "", false
	}
	return id, true
}

func ServiceAction(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	id, ok := svcParams(w, r)
	if !ok {
		return
	}
	act := r.PathValue("action")
	if !svcActions[act] {
		core.WriteError(w, 400, "aksi harus start, stop, atau restart")
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if id == "panel" || id == "dnsmasq" {
		core.WriteError(w, 400, "layanan ini tidak dikendalikan dari panel")
		return
	}
	if act != "start" && !a.RequireStepUp(w, ss) { // menghentikan/memulai ulang bisa memutus situs dan database
		return
	}
	a.Record(w, ss, a.ClientIP(r), "service_"+act, id, a.Mutate([]string{"service", act, id}, ""))
}

func ServiceLogs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	id, ok := svcParams(w, r)
	if !ok {
		return
	}
	n := 60
	if q := r.URL.Query().Get("n"); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || !core.LogN.MatchString(q) || v < 1 || v > 500 {
			core.WriteError(w, 400, "n 1-500")
			return
		}
		n = v
	}
	res := a.RunLab([]string{"service", "logs", id, "-n", strconv.Itoa(n)}, "")
	if res.Code != 0 {
		core.WriteError(w, 404, "log tidak tersedia untuk layanan ini")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"id": id, "log": res.Out})
}

func AppInstall(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	id := r.PathValue("id")
	if !optionalApp[id] {
		core.WriteError(w, 400, "aplikasi tidak ada di katalog")
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "app_install", id, []string{"app", "install", id}, ""))
}

func AppRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	id := r.PathValue("id")
	if !optionalApp[id] {
		core.WriteError(w, 400, "aplikasi tidak ada di katalog")
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Confirm != id {
		core.WriteError(w, 400, "ketik ulang nama aplikasi untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "app_remove", id, []string{"app", "remove", id}, ""))
}
