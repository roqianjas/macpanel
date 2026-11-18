package redis

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var redisKeyRE = regexp.MustCompile(`^[\x21-\x7e]{1,200}$`)

var redisPatRE = regexp.MustCompile(`^[A-Za-z0-9:_.*?\-]{1,100}$`)

func redisDB(w http.ResponseWriter, s string) (string, bool) {
	if s == "" {
		return "0", true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 15 || strconv.Itoa(n) != s {
		core.WriteError(w, 400, "db harus 0-15")
		return "", false
	}
	return s, true
}

func redisAdmin(w http.ResponseWriter, ss *store.Session) bool {
	if ss.Role == "viewer" {
		core.WriteError(w, 403, "hanya admin")
		return false
	}
	return true
}

func RedisInfo(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !redisAdmin(w, ss) {
		return
	}
	db, ok := redisDB(w, r.URL.Query().Get("db"))
	if !ok {
		return
	}
	core.LabJSON(a, w, []string{"redis", "info", "--db", db}, 15e9, "Redis tidak menjawab")
}

func RedisScan(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !redisAdmin(w, ss) {
		return
	}
	q := r.URL.Query()
	db, ok := redisDB(w, q.Get("db"))
	if !ok {
		return
	}
	pat := q.Get("pattern")
	if pat == "" {
		pat = "*"
	}
	if !redisPatRE.MatchString(pat) {
		core.WriteError(w, 400, "pola: huruf, angka, : _ . - * ?")
		return
	}
	core.LabJSON(a, w, []string{"redis", "scan", "--db", db, pat}, 40e9, "gagal memindai kunci")
}

func RedisGet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !redisAdmin(w, ss) {
		return
	}
	var in struct{ DB, Key string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	db, ok := redisDB(w, in.DB)
	if !ok {
		return
	}
	if !redisKeyRE.MatchString(in.Key) || strings.HasPrefix(in.Key, "-") {
		core.WriteError(w, 400, "nama kunci tidak valid")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Store.Audit(ss.User, "redis_view", "db"+db+" "+store.Clean(in.Key, 80), a.ClientIP(r), a.Now())
	core.LabJSON(a, w, []string{"redis", "get", "--db", db, in.Key}, 15e9, "kunci tidak ada atau tidak terbaca")
}

func RedisDel(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !redisAdmin(w, ss) {
		return
	}
	var in struct{ DB, Key string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	db, ok := redisDB(w, in.DB)
	if !ok {
		return
	}
	if !redisKeyRE.MatchString(in.Key) || strings.HasPrefix(in.Key, "-") {
		core.WriteError(w, 400, "nama kunci tidak valid")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "redis_del", "db"+db+" "+store.Clean(in.Key, 80), a.Mutate([]string{"redis", "del", "--db", db, in.Key}, ""))
}
