package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// ---------- user database ----------

func hDBUsers(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	res := a.runLab([]string{"db", "users", name, "--engine", engine}, "")
	var l struct {
		Users []struct {
			User string `json:"user"`
			Host string `json:"host"`
		} `json:"users"`
		Cred    bool  `json:"cred"`
		Rotated int64 `json:"rotated"`
	}
	if res.Code != 0 || !parseLabJSON(res.Out, &l) {
		jerr(w, 502, "gagal membaca user database")
		return
	}
	jsonOut(w, 200, l)
}

func hDBPasswd(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm, Password string }
	if !decode(w, r, &in) || !validDBPw(w, in.Password) {
		return
	}
	if in.Confirm != name {
		jerr(w, 400, "ketik ulang nama database untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	args, stdin := []string{"db", "passwd", name, "--engine", engine}, ""
	if in.Password != "" {
		args, stdin = append(args, "--password-stdin"), in.Password+"\n"
	}
	a.record(w, ss, a.clientIP(r), "db_passwd", engine+"/"+name, a.mutate(args, stdin))
}

// hDBCreds menampilkan kredensial proyek (rahasia). Hanya admin, selalu step-up, dicatat di audit tanpa isi.
func hDBCreds(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if ss.Role == "viewer" {
		jerr(w, 403, "hanya admin")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	res := a.runLab([]string{"db", "creds", name, "--engine", engine}, "")
	var kv map[string]string
	if res.Code != 0 || !parseLabJSON(res.Out, &kv) {
		jerr(w, 404, "kredensial proyek tidak ada")
		return
	}
	a.st.Audit(ss.User, "db_creds_view", engine+"/"+name, a.clientIP(r), a.now())
	jsonOut(w, 200, kv)
}

// ---------- backup: jadwal, jelajah, ekstrak ----------

var (
	bkNameRE = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}$`)
	bkTimeRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	bkPathRE = regexp.MustCompile(`^[A-Za-z0-9._@+ /-]{1,300}$`)
)

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

func hBackupCfgGet(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLab([]string{"backup", "config", "get"}, "")
	var c struct {
		Time      string `json:"time"`
		Keep      int    `json:"keep"`
		Days      int    `json:"days"`
		ExtraDays int    `json:"extra_days"`
	}
	if res.Code != 0 || !parseLabJSON(res.Out, &c) {
		jerr(w, 502, "gagal membaca jadwal backup")
		return
	}
	jsonOut(w, 200, c)
}

func hBackupCfgSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct {
		Time      string
		Keep      int
		Days      int
		ExtraDays int `json:"extra_days"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !bkTimeRE.MatchString(in.Time) || in.Keep < 1 || in.Keep > 60 || in.Days < 0 || in.Days > 3650 || in.ExtraDays < 0 || in.ExtraDays > 3650 {
		jerr(w, 400, "jam HH:MM dan simpan 1-60 backup")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "backup_config", in.Time+" keep "+strconv.Itoa(in.Keep), a.mutate([]string{"backup", "config", "set", "--time", in.Time, "--keep", strconv.Itoa(in.Keep), "--days", strconv.Itoa(in.Days), "--extra-days", strconv.Itoa(in.ExtraDays)}, ""))
}

func hBackupLs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	q := r.URL.Query()
	name, part, prefix := q.Get("name"), q.Get("part"), q.Get("prefix")
	if !bkNameRE.MatchString(name) || !bkPart(part) || (prefix != "" && !bkPathOK(prefix)) {
		jerr(w, 400, "parameter tidak valid")
		return
	}
	args := []string{"backup", "ls", name, part}
	if prefix != "" {
		args = append(args, prefix)
	}
	res := a.runLabT(args, "", 60e9)
	var l struct {
		Prefix  string `json:"prefix"`
		Entries []struct {
			Name string `json:"name"`
			Dir  bool   `json:"dir"`
		} `json:"entries"`
	}
	if res.Code != 0 || !parseLabJSON(res.Out, &l) {
		jerr(w, 502, "gagal membaca isi arsip")
		return
	}
	jsonOut(w, 200, l)
}

func hBackupExtract(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Name, Part, Path string }
	if !decode(w, r, &in) {
		return
	}
	in.Path = strings.Trim(in.Path, "/")
	if !bkNameRE.MatchString(in.Name) || !bkPart(in.Part) || in.Path == "" || !bkPathOK(in.Path) {
		jerr(w, 400, "parameter tidak valid")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "backup_extract", in.Name+" "+in.Part+" "+in.Path, []string{"backup", "extract", in.Name, in.Part, in.Path}, ""))
}
