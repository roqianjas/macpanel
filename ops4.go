package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

// ---------- ekstensi PHP ----------

var extNames = map[string]bool{"redis": true, "apcu": true, "xdebug": true, "imagick": true, "igbinary": true, "msgpack": true, "pcov": true, "yaml": true, "memcached": true, "mongodb": true}
var extActions = map[string]bool{"install": true, "remove": true, "enable": true, "disable": true}

type ExtInfo struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	Loaded    bool   `json:"loaded"`
	Zend      bool   `json:"zend"`
}

type ExtList struct {
	Version string    `json:"version"`
	Builtin []string  `json:"builtin"`
	Managed []ExtInfo `json:"managed"`
}

// parseLabJSON membaca JSON dari keluaran lab (stdout dan stderr tergabung): ambil dari '{' pertama.
func parseLabJSON(out string, v any) bool {
	i := strings.Index(out, "{")
	if i < 0 {
		return false
	}
	return json.NewDecoder(strings.NewReader(out[i:])).Decode(v) == nil
}

func hExtList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	res := a.runLab([]string{"ext", "list", v}, "")
	var l ExtList
	if res.Code != 0 || !parseLabJSON(res.Out, &l) {
		msg := "gagal membaca ekstensi PHP"
		if res.Code != 0 && strings.Contains(res.Out, "belum terpasang") {
			msg = "PHP " + v + " belum terpasang"
		}
		jerr(w, 502, msg)
		return
	}
	for i := range l.Managed {
		if !extNames[l.Managed[i].Name] { // hanya nama yang dikenal panel yang diteruskan
			l.Managed[i].Name = ""
		}
	}
	jsonOut(w, 200, l)
}

func hExtAction(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	name, act := r.PathValue("name"), r.PathValue("action")
	if !extNames[name] || !extActions[act] {
		jerr(w, 400, "ekstensi atau aksi tidak didukung")
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "ext_"+act, v+" "+name, []string{"ext", act, v, name}, ""))
}

// ---------- php.ini (pool FPM) ----------

var sizeRE = regexp.MustCompile(`^[0-9]{1,5}[KMG]$`)
var intRE = regexp.MustCompile(`^(-1|[0-9]{1,6})$`)
var tzRE = regexp.MustCompile(`^[A-Za-z0-9_+/-]{1,40}$`)

var iniKeys = map[string]func(string) bool{
	"memory_limit":        func(s string) bool { return s == "-1" || sizeRE.MatchString(s) },
	"upload_max_filesize": sizeRE.MatchString,
	"post_max_size":       sizeRE.MatchString,
	"max_execution_time":  intRE.MatchString,
	"max_input_time":      intRE.MatchString,
	"max_input_vars":      regexp.MustCompile(`^[0-9]{3,6}$`).MatchString,
	"display_errors":      func(s string) bool { return s == "0" || s == "1" },
	"date.timezone":       tzRE.MatchString,
}

type IniKey struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Default string `json:"default"`
	Hint    string `json:"hint"`
	Group   string `json:"group"`
}

func hIniGet(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	res := a.runLab([]string{"ini", "get", v}, "")
	var l struct {
		Version string   `json:"version"`
		Keys    []IniKey `json:"keys"`
	}
	if res.Code != 0 || !parseLabJSON(res.Out, &l) {
		jerr(w, 502, "gagal membaca pengaturan PHP")
		return
	}
	keys := []IniKey{}
	for _, k := range l.Keys {
		_, known := iniKeys[k.Key]
		if _, pool := poolKeys[k.Key]; known || pool {
			if pool {
				k.Group = "fpm"
			} else {
				k.Group = "php"
			}
			keys = append(keys, k)
		}
	}
	jsonOut(w, 200, map[string]any{"version": v, "keys": keys})
}

func hIniSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	var in struct{ Values map[string]string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.Values) == 0 || len(in.Values) > len(iniKeys)+len(poolKeys) {
		jerr(w, 400, "tidak ada nilai untuk disimpan")
		return
	}
	args := []string{"ini", "set", v}
	for _, k := range sortedKeys(in.Values) {
		check, known := iniKeys[k]
		if pc, pool := poolKeys[k]; pool {
			check, known = pc, true
		}
		if !known {
			jerr(w, 400, "pengaturan tidak didukung: "+k)
			return
		}
		if !check(in.Values[k]) {
			jerr(w, 400, "nilai tidak valid untuk "+k)
			return
		}
		args = append(args, k+"="+in.Values[k])
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "ini_set", v, args, ""))
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ---------- Composer ----------

type ComposerInfo struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	PHP       string `json:"php"`
	Outdated  bool   `json:"outdated"`
	Latest    string `json:"latest"`
}

func hComposer(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLabT([]string{"composer", "status"}, "", a.timeout)
	var c ComposerInfo
	if res.Code != 0 || !parseLabJSON(res.Out, &c) {
		jerr(w, 502, "gagal membaca status Composer")
		return
	}
	jsonOut(w, 200, c)
}

func hComposerAct(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	act := r.PathValue("action")
	if act != "install" && act != "update" {
		jerr(w, 400, "aksi harus install atau update")
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "composer_"+act, "composer", []string{"composer", act}, ""))
}

// ---------- deploy per situs ----------

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

func hDeployInfo(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	name := r.PathValue("name")
	if !siteRE.MatchString(name) || protectedSites[name] {
		jerr(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	res := a.runLab([]string{"deploy", "info", name}, "")
	var d DeployInfo
	if res.Code != 0 || !parseLabJSON(res.Out, &d) {
		msg := strings.TrimSpace(strings.TrimPrefix(scrub(res.Out), "lab:"))
		if len(msg) > 160 || msg == "" {
			msg = "gagal membaca informasi deploy"
		}
		jerr(w, 502, msg)
		return
	}
	jsonOut(w, 200, d)
}

func hDeployRun(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name := r.PathValue("name")
	if !siteRE.MatchString(name) || protectedSites[name] {
		jerr(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	var in struct{ Steps []string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.Steps) == 0 || len(in.Steps) > 8 {
		jerr(w, 400, "pilih 1-8 langkah")
		return
	}
	for _, s := range in.Steps {
		if !deploySteps[s] {
			jerr(w, 400, "langkah tidak dikenal: "+s)
			return
		}
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "deploy", name+" ["+strings.Join(in.Steps, ",")+"]", []string{"deploy", "run", name, strings.Join(in.Steps, ",")}, ""))
}

// ---------- backup ----------

type BackupRow struct {
	Name  string   `json:"name"`
	At    int64    `json:"at"`
	Size  int64    `json:"size"`
	Files []string `json:"files"`
}

func hBackups(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLab([]string{"backup", "list"}, "")
	var l struct {
		Backups []BackupRow `json:"backups"`
		Offsite bool        `json:"offsite"`
	}
	if res.Code != 0 || !parseLabJSON(res.Out, &l) {
		jerr(w, 502, "gagal membaca daftar backup")
		return
	}
	if l.Backups == nil {
		l.Backups = []BackupRow{}
	}
	jsonOut(w, 200, l)
}

func hBackupRun(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "backup_run", "lokal", []string{"backup", "run"}, ""))
}
