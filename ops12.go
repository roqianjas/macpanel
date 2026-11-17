package main

import (
	"net/http"
	"strconv"
)

func intParam(w http.ResponseWriter, s string, lo, hi int) (int, bool, bool) {
	if s == "" {
		return 0, false, true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi || strconv.Itoa(n) != s {
		jerr(w, 400, "angka di luar batas")
		return 0, false, false
	}
	return n, true, true
}

func hBackupPrunePreview(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	q := r.URL.Query()
	keep, hasKeep, ok := intParam(w, q.Get("keep"), 1, 60)
	if !ok {
		return
	}
	days, hasDays, ok := intParam(w, q.Get("days"), 0, 3650)
	if !ok {
		return
	}
	args := []string{"backup", "prune", "--dry-run"}
	if hasKeep {
		args = append(args, "--keep", strconv.Itoa(keep))
	}
	if hasDays {
		args = append(args, "--days", strconv.Itoa(days))
	}
	labJSON(a, w, args, 30e9, "gagal menghitung rencana pembersihan")
}

func hBackupPrune(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Keep, Days int }
	if !decode(w, r, &in) {
		return
	}
	if in.Keep < 1 || in.Keep > 60 || in.Days < 0 || in.Days > 3650 {
		jerr(w, 400, "simpan 1-60 backup, umur 0-3650 hari")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "backup_prune", "keep "+strconv.Itoa(in.Keep)+" days "+strconv.Itoa(in.Days),
		a.mutate([]string{"backup", "prune", "--keep", strconv.Itoa(in.Keep), "--days", strconv.Itoa(in.Days)}, ""))
}

func hBackupDelete(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Names []string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.Names) < 1 || len(in.Names) > 30 {
		jerr(w, 400, "pilih 1-30 backup")
		return
	}
	seen := map[string]bool{}
	for _, n := range in.Names {
		if !bkNameRE.MatchString(n) || seen[n] {
			jerr(w, 400, "nama backup tidak valid atau ganda")
			return
		}
		seen[n] = true
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	args := append([]string{"backup", "delete"}, in.Names...)
	a.record(w, ss, a.clientIP(r), "backup_delete", strconv.Itoa(len(in.Names))+" backup", a.mutate(args, ""))
}

func hBackupExtras(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	labJSON(a, w, []string{"backup", "extras", "info"}, 60e9, "gagal membaca pemakaian")
}

func extrasKind(w http.ResponseWriter, k string) bool {
	if k != "db" && k != "restore" {
		jerr(w, 400, "jenis: db atau restore")
		return false
	}
	return true
}

func hBackupExtrasPreview(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	q := r.URL.Query()
	if !extrasKind(w, q.Get("kind")) {
		return
	}
	days, _, ok := intParam(w, q.Get("days"), 0, 3650)
	if !ok {
		return
	}
	labJSON(a, w, []string{"backup", "extras", q.Get("kind"), "--days", strconv.Itoa(days), "--dry-run"}, 30e9, "gagal menghitung")
}

func hBackupExtrasClean(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct {
		Kind string
		Days int
	}
	if !decode(w, r, &in) {
		return
	}
	if !extrasKind(w, in.Kind) || in.Days < 0 || in.Days > 3650 {
		if in.Kind == "db" || in.Kind == "restore" {
			jerr(w, 400, "umur 0-3650 hari (0 = semua)")
		}
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "backup_extras_clean", in.Kind+" days "+strconv.Itoa(in.Days), a.mutate([]string{"backup", "extras", in.Kind, "--days", strconv.Itoa(in.Days)}, ""))
}
