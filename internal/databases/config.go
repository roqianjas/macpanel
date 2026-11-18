package databases

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var mySize = regexp.MustCompile(`^([0-9]{1,5})([MG])$`)

func mbOf(s string) int {
	m := mySize.FindStringSubmatch(s)
	if m == nil {
		return -1
	}
	n, _ := strconv.Atoi(m[1])
	if m[2] == "G" {
		n *= 1024
	}
	return n
}

var myCfgKeys = map[string]func(string) bool{
	"max_connections": func(s string) bool {
		n, err := strconv.Atoi(s)
		return regexp.MustCompile(`^[0-9]{2,4}$`).MatchString(s) && err == nil && n >= 10 && n <= 2000
	},
	"innodb_buffer_pool_size": func(s string) bool { n := mbOf(s); return n >= 64 && n <= 8192 },
	"max_allowed_packet":      func(s string) bool { n := mbOf(s); return n >= 1 && n <= 1024 },
	"slow_query_log":          func(s string) bool { return s == "0" || s == "1" },
	"long_query_time":         regexp.MustCompile(`^([0-5]?[0-9](\.[0-9])?|60)$`).MatchString,
}

var DbcfgKeys = map[string]map[string]func(string) bool{
	"mariadb": myCfgKeys,
	"mysql":   myCfgKeys,
	"redis": {
		"maxmemory": func(s string) bool { return s == "0" || regexp.MustCompile(`^[0-9]{1,5}(mb|gb)$`).MatchString(s) },
		"maxmemory-policy": func(s string) bool {
			switch s {
			case "noeviction", "allkeys-lru", "volatile-lru", "allkeys-lfu", "volatile-lfu", "allkeys-random", "volatile-ttl":
				return true
			}
			return false
		},
		"appendonly": func(s string) bool { return s == "yes" || s == "no" },
		"save":       func(s string) bool { return s == "off" || s == "default" },
	},
}

type DBCfgKey struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Live  string `json:"live"`
	Hint  string `json:"hint"`
}

func dbcfgEngine(w http.ResponseWriter, r *http.Request) (string, bool) {
	e := r.PathValue("engine")
	if _, ok := DbcfgKeys[e]; !ok {
		core.WriteError(w, 400, "engine harus mariadb, mysql atau redis")
		return "", false
	}
	return e, true
}

func DBCfgGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	e, ok := dbcfgEngine(w, r)
	if !ok {
		return
	}
	res := a.RunLabTimeout([]string{"dbcfg", "get", e}, "", 40e9)
	var l struct {
		Keys []DBCfgKey `json:"keys"`
	}
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &l) {
		core.WriteError(w, 502, "gagal membaca pengaturan "+e)
		return
	}
	keys := []DBCfgKey{}
	for _, k := range l.Keys {
		if _, known := DbcfgKeys[e][k.Key]; known {
			keys = append(keys, k)
		}
	}
	core.WriteJSON(w, 200, map[string]any{"engine": e, "keys": keys})
}

func DBCfgSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	e, ok := dbcfgEngine(w, r)
	if !ok {
		return
	}
	var in struct{ Values map[string]string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if len(in.Values) == 0 || len(in.Values) > len(DbcfgKeys[e]) {
		core.WriteError(w, 400, "tidak ada nilai untuk disimpan")
		return
	}
	args := []string{"dbcfg", "set", e}
	for _, k := range core.SortedKeys(in.Values) {
		check, known := DbcfgKeys[e][k]
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
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "dbcfg_set", e, args, ""))
}

func SlowLog(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	slowEng := r.PathValue("engine")
	if slowEng != "mariadb" && slowEng != "mysql" {
		core.WriteError(w, 400, "slow log hanya untuk mariadb atau mysql")
		return
	}
	res := a.RunLabTimeout([]string{"dbcfg", "slow", slowEng}, "", 20e9)
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca slow log")
		return
	}
	core.WriteJSON(w, 200, map[string]string{"log": strings.TrimSpace(core.Scrub(res.Out))})
}
