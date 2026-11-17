package main

import (
	"net/http"
	"strconv"
	"time"
)

func labJSON(a *App, w http.ResponseWriter, args []string, timeout time.Duration, msg string) {
	res := a.runLabT(args, "", timeout)
	var raw map[string]any
	if res.Code != 0 || !parseLabJSON(res.Out, &raw) {
		jerr(w, 502, msg)
		return
	}
	jsonOut(w, 200, raw)
}

func hSysProcs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	labJSON(a, w, []string{"sys", "procs"}, 20e9, "gagal membaca proses")
}

func hSysDisk(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	labJSON(a, w, []string{"sys", "disk"}, 120e9, "gagal menghitung pemakaian disk")
}

func hSysSecurity(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	labJSON(a, w, []string{"sys", "security"}, 30e9, "gagal membaca status keamanan")
}

func hSysDocker(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	labJSON(a, w, []string{"sys", "docker"}, 30e9, "gagal membaca Docker")
}

func hSiteStats(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	labJSON(a, w, []string{"sys", "stats", name}, 60e9, "gagal membaca log situs")
}

func hSysKill(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ PID int }
	if !decode(w, r, &in) {
		return
	}
	if in.PID < 2 || in.PID > 4194304 {
		jerr(w, 400, "pid tidak valid")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	pid := strconv.Itoa(in.PID)
	a.record(w, ss, a.clientIP(r), "proc_kill", pid, a.mutate([]string{"sys", "kill", pid}, ""))
}
