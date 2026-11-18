package phpruntime

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

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

func ExtListHandler(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	res := a.RunLab([]string{"ext", "list", v}, "")
	var l ExtList
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &l) {
		msg := "gagal membaca ekstensi PHP"
		if res.Code != 0 && strings.Contains(res.Out, "belum terpasang") {
			msg = "PHP " + v + " belum terpasang"
		}
		core.WriteError(w, 502, msg)
		return
	}
	for i := range l.Managed {
		if !extNames[l.Managed[i].Name] { // hanya nama yang dikenal panel yang diteruskan
			l.Managed[i].Name = ""
		}
	}
	core.WriteJSON(w, 200, l)
}

func ExtAction(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	name, act := r.PathValue("name"), r.PathValue("action")
	if !extNames[name] || !extActions[act] {
		core.WriteError(w, 400, "ekstensi atau aksi tidak didukung")
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "ext_"+act, v+" "+name, []string{"ext", act, v, name}, ""))
}

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

func IniGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	res := a.RunLab([]string{"ini", "get", v}, "")
	var l struct {
		Version string   `json:"version"`
		Keys    []IniKey `json:"keys"`
	}
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &l) {
		core.WriteError(w, 502, "gagal membaca pengaturan PHP")
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
	core.WriteJSON(w, 200, map[string]any{"version": v, "keys": keys})
}

func IniSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	var in struct{ Values map[string]string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if len(in.Values) == 0 || len(in.Values) > len(iniKeys)+len(poolKeys) {
		core.WriteError(w, 400, "tidak ada nilai untuk disimpan")
		return
	}
	args := []string{"ini", "set", v}
	for _, k := range core.SortedKeys(in.Values) {
		check, known := iniKeys[k]
		if pc, pool := poolKeys[k]; pool {
			check, known = pc, true
		}
		if !known {
			core.WriteError(w, 400, "pengaturan tidak didukung: "+k)
			return
		}
		if !check(in.Values[k]) {
			core.WriteError(w, 400, "nilai tidak valid untuk "+k)
			return
		}
		args = append(args, k+"="+in.Values[k])
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "ini_set", v, args, ""))
}

type ComposerInfo struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	PHP       string `json:"php"`
	Outdated  bool   `json:"outdated"`
	Latest    string `json:"latest"`
}

func Composer(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLabTimeout([]string{"composer", "status"}, "", a.Timeout)
	var c ComposerInfo
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &c) {
		core.WriteError(w, 502, "gagal membaca status Composer")
		return
	}
	core.WriteJSON(w, 200, c)
}

func ComposerAct(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	act := r.PathValue("action")
	if act != "install" && act != "update" {
		core.WriteError(w, 400, "aksi harus install atau update")
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "composer_"+act, "composer", []string{"composer", act}, ""))
}

var poolNum = regexp.MustCompile(`^[0-9]{1,3}$`)

func poolCount(s string) bool {
	if !poolNum.MatchString(s) {
		return false
	}
	n, _ := strconv.Atoi(s)
	return n >= 1 && n <= 200
}

var poolKeys = map[string]func(string) bool{
	"pm":                      func(s string) bool { return s == "static" || s == "dynamic" || s == "ondemand" },
	"pm.max_children":         poolCount,
	"pm.start_servers":        poolCount,
	"pm.min_spare_servers":    poolCount,
	"pm.max_spare_servers":    poolCount,
	"pm.process_idle_timeout": regexp.MustCompile(`^[0-9]{1,4}s$`).MatchString,
	"pm.max_requests": func(s string) bool {
		n, err := strconv.Atoi(s)
		return regexp.MustCompile(`^[0-9]{1,6}$`).MatchString(s) && err == nil && n <= 100000
	},
}
