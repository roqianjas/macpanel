package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// ---------- kunci pool FPM (dipakai bersama /api/php/{v}/ini) ----------

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

// ---------- config server database ----------

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

var dbcfgKeys = map[string]map[string]func(string) bool{
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
	if _, ok := dbcfgKeys[e]; !ok {
		jerr(w, 400, "engine harus mariadb, mysql atau redis")
		return "", false
	}
	return e, true
}

func hDBCfgGet(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	e, ok := dbcfgEngine(w, r)
	if !ok {
		return
	}
	res := a.runLabT([]string{"dbcfg", "get", e}, "", 40e9)
	var l struct {
		Keys []DBCfgKey `json:"keys"`
	}
	if res.Code != 0 || !parseLabJSON(res.Out, &l) {
		jerr(w, 502, "gagal membaca pengaturan "+e)
		return
	}
	keys := []DBCfgKey{}
	for _, k := range l.Keys {
		if _, known := dbcfgKeys[e][k.Key]; known {
			keys = append(keys, k)
		}
	}
	jsonOut(w, 200, map[string]any{"engine": e, "keys": keys})
}

func hDBCfgSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	e, ok := dbcfgEngine(w, r)
	if !ok {
		return
	}
	var in struct{ Values map[string]string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.Values) == 0 || len(in.Values) > len(dbcfgKeys[e]) {
		jerr(w, 400, "tidak ada nilai untuk disimpan")
		return
	}
	args := []string{"dbcfg", "set", e}
	for _, k := range sortedKeys(in.Values) {
		check, known := dbcfgKeys[e][k]
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
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "dbcfg_set", e, args, ""))
}

func hSlowLog(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	slowEng := r.PathValue("engine")
	if slowEng != "mariadb" && slowEng != "mysql" {
		jerr(w, 400, "slow log hanya untuk mariadb atau mysql")
		return
	}
	res := a.runLabT([]string{"dbcfg", "slow", slowEng}, "", 20e9)
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca slow log")
		return
	}
	jsonOut(w, 200, map[string]string{"log": strings.TrimSpace(scrub(res.Out))})
}
