package system

import (
	"net/http"
	"strconv"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func SysProcs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"sys", "procs"}, 20e9, "gagal membaca proses")
}

func SysDisk(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"sys", "disk"}, 120e9, "gagal menghitung pemakaian disk")
}

func SysSecurity(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"sys", "security"}, 30e9, "gagal membaca status keamanan")
}

func SysKill(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ PID int }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.PID < 2 || in.PID > 4194304 {
		core.WriteError(w, 400, "pid tidak valid")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	pid := strconv.Itoa(in.PID)
	a.Record(w, ss, a.ClientIP(r), "proc_kill", pid, a.Mutate([]string{"sys", "kill", pid}, ""))
}

func SysDNS(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"sys", "dns"}, 60*time.Second, "gagal membaca DNS")
}

func Health(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.WriteJSON(w, 200, map[string]string{"status": "ok"})
}
