package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func statDir(p string) (bool, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

type SiteInfo struct {
	Name   string `json:"name"`
	Host   string `json:"host"`
	Mode   string `json:"mode"`
	Target string `json:"target"`
	PHP    string `json:"php,omitempty"`
	System bool   `json:"system"`
}

var (
	phpSockRE = regexp.MustCompile(`php-fpm-dev-(8\.[0-9])\.sock`)
	proxyRE   = regexp.MustCompile(`proxy_pass http://127\.0\.0\.1:(\d+)`)
	rootRE    = regexp.MustCompile(`(?m)^\s*root\s+([^;]+);`)
)

func (a *App) listSites() []SiteInfo {
	out := []SiteInfo{}
	files, _ := filepath.Glob(filepath.Join(a.sitesDir, "*.conf"))
	sort.Strings(files)
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".conf")
		b, _ := os.ReadFile(f)
		s := SiteInfo{Name: name, Host: name + "." + a.tld, Mode: "kustom", System: protectedSites[name]}
		txt := string(b)
		if m := phpSockRE.FindStringSubmatch(txt); m != nil {
			s.Mode, s.PHP = "php", m[1]
		} else if m := proxyRE.FindStringSubmatch(txt); m != nil {
			s.Mode, s.Target = "proxy", m[1]
		} else if rootRE.MatchString(txt) {
			s.Mode = "statis"
		}
		if m := rootRE.FindStringSubmatch(txt); m != nil && s.Mode != "proxy" {
			s.Target = strings.TrimSpace(m[1])
		}
		out = append(out, s)
	}
	return out
}

// record menyimpan hasil aksi ke tabel job dan audit, lalu menjawab klien.
func (a *App) record(w http.ResponseWriter, ss *Session, ip, action, detail string, res Result) {
	status := "ok"
	switch {
	case res.TimedOut:
		status = "timeout"
	case res.Code != 0:
		status = "gagal"
	}
	id := a.st.AddJob(ss.User, action+" "+detail, status, res.Out, res.Code, a.now())
	a.st.Audit(ss.User, action, detail+" -> "+status, ip, a.now())
	code := 200
	if status != "ok" {
		code = 422
	}
	jsonOut(w, code, map[string]any{"job": id, "status": status, "output": res.Out})
}

func (a *App) mutate(args []string, stdin string) Result {
	a.mu.Lock() // satu aksi mutasi pada satu waktu
	defer a.mu.Unlock()
	return a.runLab(args, stdin)
}

// ---------- Website ----------

func hSitesList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	jsonOut(w, 200, map[string]any{"sites": a.listSites()})
}

func hSiteAdd(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Name, Target, PHP string }
	if !decode(w, r, &in) {
		return
	}
	if !siteRE.MatchString(in.Name) || protectedSites[in.Name] {
		jerr(w, 400, "nama situs tidak valid atau dicadangkan")
		return
	}
	target, err := a.validTarget(in.Target)
	if err != nil {
		jerr(w, 400, err.Error())
		return
	}
	args := []string{"site", "add", in.Name, target}
	if in.PHP != "" {
		if !phpVersions[in.PHP] || numRE.MatchString(target) {
			jerr(w, 400, "versi PHP tidak valid, atau PHP butuh folder (bukan port)")
			return
		}
		args = append(args, "--php", in.PHP)
	}
	a.record(w, ss, a.clientIP(r), "site_add", in.Name, a.mutate(args, ""))
}

func hSiteRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name := r.PathValue("name")
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if !siteRE.MatchString(name) || protectedSites[name] {
		jerr(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	if in.Confirm != name {
		jerr(w, 400, "ketik ulang nama situs untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "site_remove", name, a.mutate([]string{"site", "remove", name}, ""))
}

// ---------- Database ----------

func parseDBList(out string) map[string][]string {
	res := map[string][]string{"mysql": {}, "pgsql": {}}
	cur := ""
	for _, ln := range strings.Split(out, "\n") {
		switch t := strings.TrimSpace(ln); {
		case t == "MySQL:":
			cur = "mysql"
		case t == "PostgreSQL:":
			cur = "pgsql"
		case t != "" && t != "(kosong)" && cur != "" && dbRE.MatchString(t):
			res[cur] = append(res[cur], t)
		}
	}
	return res
}

func hDBList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLab([]string{"db", "list"}, "")
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca daftar database")
		return
	}
	jsonOut(w, 200, map[string]any{"databases": parseDBList(res.Out), "system": []string{"dev"}})
}

// systemDBs adalah database/akun milik server sendiri; panel tidak boleh menghapus atau men-dump-nya (lab juga menolak).
var systemDBs = map[string]bool{"dev": true, "postgres": true, "template0": true, "template1": true, "mysql": true, "sys": true, "root": true, "admin": true, "information_schema": true, "performance_schema": true}

func dbParams(w http.ResponseWriter, r *http.Request) (engine, name string, ok bool) {
	engine, name = r.PathValue("engine"), r.PathValue("name")
	if (engine != "mysql" && engine != "pgsql") || !dbRE.MatchString(name) {
		jerr(w, 400, "engine atau nama tidak valid")
		return "", "", false
	}
	if systemDBs[name] {
		jerr(w, 400, "database sistem tidak boleh diubah dari panel")
		return "", "", false
	}
	return engine, name, true
}

func hDBCreate(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Name, Engine, Env string }
	if !decode(w, r, &in) {
		return
	}
	if (in.Engine != "mysql" && in.Engine != "pgsql") || !dbRE.MatchString(in.Name) {
		jerr(w, 400, "engine atau nama tidak valid")
		return
	}
	args := []string{"db", "create", in.Name, "--engine", in.Engine}
	if in.Env != "" {
		p, err := a.validEnvFile(in.Env)
		if err != nil {
			jerr(w, 400, err.Error())
			return
		}
		args = append(args, "--env", p)
	}
	a.record(w, ss, a.clientIP(r), "db_create", in.Engine+"/"+in.Name, a.mutate(args, ""))
}

func hDBDump(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	a.record(w, ss, a.clientIP(r), "db_dump", engine+"/"+name, a.mutate([]string{"db", "dump", name, "--engine", engine}, ""))
}

func hDBDrop(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if in.Confirm != name {
		jerr(w, 400, "ketik ulang nama database untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	// `lab db drop` meminta nama diketik ulang lewat stdin; konfirmasi manusia sudah dilakukan di atas.
	a.record(w, ss, a.clientIP(r), "db_drop", engine+"/"+name, a.mutate([]string{"db", "drop", name, "--engine", engine}, name+"\n"))
}

// ---------- Job ----------

func hJobs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	jsonOut(w, 200, map[string]any{"jobs": a.st.ListJobs(50)})
}

var _ = time.Second
