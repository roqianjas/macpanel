package backup

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func BackupPrunePreview(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	q := r.URL.Query()
	keep, hasKeep, ok := core.IntParam(w, q.Get("keep"), 1, 60)
	if !ok {
		return
	}
	days, hasDays, ok := core.IntParam(w, q.Get("days"), 0, 3650)
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
	core.LabJSON(a, w, args, 30e9, "gagal menghitung rencana pembersihan")
}

func BackupPrune(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Keep, Days int }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Keep < 1 || in.Keep > 60 || in.Days < 0 || in.Days > 3650 {
		core.WriteError(w, 400, "simpan 1-60 backup, umur 0-3650 hari")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "backup_prune", "keep "+strconv.Itoa(in.Keep)+" days "+strconv.Itoa(in.Days),
		a.Mutate([]string{"backup", "prune", "--keep", strconv.Itoa(in.Keep), "--days", strconv.Itoa(in.Days)}, ""))
}

func BackupDelete(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Names []string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if len(in.Names) < 1 || len(in.Names) > 30 {
		core.WriteError(w, 400, "pilih 1-30 backup")
		return
	}
	seen := map[string]bool{}
	for _, n := range in.Names {
		if !bkNameRE.MatchString(n) || seen[n] {
			core.WriteError(w, 400, "nama backup tidak valid atau ganda")
			return
		}
		seen[n] = true
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	args := append([]string{"backup", "delete"}, in.Names...)
	a.Record(w, ss, a.ClientIP(r), "backup_delete", strconv.Itoa(len(in.Names))+" backup", a.Mutate(args, ""))
}

func BackupExtras(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"backup", "extras", "info"}, 60e9, "gagal membaca pemakaian")
}

func extrasKind(w http.ResponseWriter, k string) bool {
	if k != "db" && k != "restore" {
		core.WriteError(w, 400, "jenis: db atau restore")
		return false
	}
	return true
}

func BackupExtrasPreview(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	q := r.URL.Query()
	if !extrasKind(w, q.Get("kind")) {
		return
	}
	days, _, ok := core.IntParam(w, q.Get("days"), 0, 3650)
	if !ok {
		return
	}
	core.LabJSON(a, w, []string{"backup", "extras", q.Get("kind"), "--days", strconv.Itoa(days), "--dry-run"}, 30e9, "gagal menghitung")
}

func BackupExtrasClean(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		Kind string
		Days int
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !extrasKind(w, in.Kind) || in.Days < 0 || in.Days > 3650 {
		if in.Kind == "db" || in.Kind == "restore" {
			core.WriteError(w, 400, "umur 0-3650 hari (0 = semua)")
		}
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "backup_extras_clean", in.Kind+" days "+strconv.Itoa(in.Days), a.Mutate([]string{"backup", "extras", in.Kind, "--days", strconv.Itoa(in.Days)}, ""))
}

type BackupRow struct {
	Name  string   `json:"name"`
	At    int64    `json:"at"`
	Size  int64    `json:"size"`
	Files []string `json:"files"`
}

func Backups(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLab([]string{"backup", "list"}, "")
	var l struct {
		Backups []BackupRow `json:"backups"`
		Offsite bool        `json:"offsite"`
	}
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &l) {
		core.WriteError(w, 502, "gagal membaca daftar backup")
		return
	}
	if l.Backups == nil {
		l.Backups = []BackupRow{}
	}
	core.WriteJSON(w, 200, l)
}

func BackupRun(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "backup_run", "lokal", []string{"backup", "run"}, ""))
}

var bkNameRE = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}$`)

var bkTimeRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

var bkPathRE = regexp.MustCompile(`^[A-Za-z0-9._@+ /-]{1,300}$`)

func bkPathOK(p string) bool {
	if !bkPathRE.MatchString(p) || strings.HasPrefix(p, "-") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == ".." {
			return false
		}
	}
	return true
}

func bkPart(p string) bool { return p == "developer" || p == "dotfiles" }

func BackupCfgGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLab([]string{"backup", "config", "get"}, "")
	var c struct {
		Time      string `json:"time"`
		Keep      int    `json:"keep"`
		Days      int    `json:"days"`
		ExtraDays int    `json:"extra_days"`
	}
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &c) {
		core.WriteError(w, 502, "gagal membaca jadwal backup")
		return
	}
	core.WriteJSON(w, 200, c)
}

func BackupCfgSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		Time      string
		Keep      int
		Days      int
		ExtraDays int `json:"extra_days"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !bkTimeRE.MatchString(in.Time) || in.Keep < 1 || in.Keep > 60 || in.Days < 0 || in.Days > 3650 || in.ExtraDays < 0 || in.ExtraDays > 3650 {
		core.WriteError(w, 400, "jam HH:MM dan simpan 1-60 backup")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "backup_config", in.Time+" keep "+strconv.Itoa(in.Keep), a.Mutate([]string{"backup", "config", "set", "--time", in.Time, "--keep", strconv.Itoa(in.Keep), "--days", strconv.Itoa(in.Days), "--extra-days", strconv.Itoa(in.ExtraDays)}, ""))
}

func BackupLs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	q := r.URL.Query()
	name, part, prefix := q.Get("name"), q.Get("part"), q.Get("prefix")
	if !bkNameRE.MatchString(name) || !bkPart(part) || (prefix != "" && !bkPathOK(prefix)) {
		core.WriteError(w, 400, "parameter tidak valid")
		return
	}
	args := []string{"backup", "ls", name, part}
	if prefix != "" {
		args = append(args, prefix)
	}
	res := a.RunLabTimeout(args, "", 60e9)
	var l struct {
		Prefix  string `json:"prefix"`
		Entries []struct {
			Name string `json:"name"`
			Dir  bool   `json:"dir"`
		} `json:"entries"`
	}
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &l) {
		core.WriteError(w, 502, "gagal membaca isi arsip")
		return
	}
	core.WriteJSON(w, 200, l)
}

func BackupExtract(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Name, Part, Path string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	in.Path = strings.Trim(in.Path, "/")
	if !bkNameRE.MatchString(in.Name) || !bkPart(in.Part) || in.Path == "" || !bkPathOK(in.Path) {
		core.WriteError(w, 400, "parameter tidak valid")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "backup_extract", in.Name+" "+in.Part+" "+in.Path, []string{"backup", "extract", in.Name, in.Part, in.Path}, ""))
}
