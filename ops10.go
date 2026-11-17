package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// ---------- Redis (hanya admin; nilai dan hapus butuh step-up) ----------

var (
	redisKeyRE = regexp.MustCompile(`^[\x21-\x7e]{1,200}$`)
	redisPatRE = regexp.MustCompile(`^[A-Za-z0-9:_.*?\-]{1,100}$`)
)

func redisDB(w http.ResponseWriter, s string) (string, bool) {
	if s == "" {
		return "0", true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 15 || strconv.Itoa(n) != s {
		jerr(w, 400, "db harus 0-15")
		return "", false
	}
	return s, true
}

func redisAdmin(w http.ResponseWriter, ss *Session) bool {
	if ss.Role == "viewer" {
		jerr(w, 403, "hanya admin")
		return false
	}
	return true
}

func hRedisInfo(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !redisAdmin(w, ss) {
		return
	}
	db, ok := redisDB(w, r.URL.Query().Get("db"))
	if !ok {
		return
	}
	labJSON(a, w, []string{"redis", "info", "--db", db}, 15e9, "Redis tidak menjawab")
}

func hRedisScan(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
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
		jerr(w, 400, "pola: huruf, angka, : _ . - * ?")
		return
	}
	labJSON(a, w, []string{"redis", "scan", "--db", db, pat}, 40e9, "gagal memindai kunci")
}

func hRedisGet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !redisAdmin(w, ss) {
		return
	}
	var in struct{ DB, Key string }
	if !decode(w, r, &in) {
		return
	}
	db, ok := redisDB(w, in.DB)
	if !ok {
		return
	}
	if !redisKeyRE.MatchString(in.Key) || strings.HasPrefix(in.Key, "-") {
		jerr(w, 400, "nama kunci tidak valid")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.st.Audit(ss.User, "redis_view", "db"+db+" "+clean(in.Key, 80), a.clientIP(r), a.now())
	labJSON(a, w, []string{"redis", "get", "--db", db, in.Key}, 15e9, "kunci tidak ada atau tidak terbaca")
}

func hRedisDel(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !redisAdmin(w, ss) {
		return
	}
	var in struct{ DB, Key string }
	if !decode(w, r, &in) {
		return
	}
	db, ok := redisDB(w, in.DB)
	if !ok {
		return
	}
	if !redisKeyRE.MatchString(in.Key) || strings.HasPrefix(in.Key, "-") {
		jerr(w, 400, "nama kunci tidak valid")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "redis_del", "db"+db+" "+clean(in.Key, 80), a.mutate([]string{"redis", "del", "--db", db, in.Key}, ""))
}

// ---------- ganti kata sandi sendiri ----------

func hMyPassword(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Current, New string }
	if !decode(w, r, &in) {
		return
	}
	ip := a.clientIP(r)
	if a.locked(ss.User, ip) {
		tooMany(w)
		return
	}
	u := a.st.GetUser(ss.User)
	if u == nil || len(in.Current) > 256 || !verifyPassword(u.PassHash, in.Current) {
		a.fail(ss.User, ip)
		a.st.Audit(ss.User, "password_change_fail", "", ip, a.now())
		jerr(w, 400, "kata sandi saat ini salah")
		return
	}
	if len(in.New) < 12 || len(in.New) > 256 || strings.EqualFold(in.New, ss.User) || in.New == in.Current {
		jerr(w, 400, "kata sandi baru minimal 12 karakter, berbeda dari yang lama dan dari nama pengguna")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	if err := a.st.SetPassword(u.ID, hashPassword(in.New), ss.ID); err != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "password_change", "sesi lain dicabut", ip, a.now())
	jsonOut(w, 200, map[string]any{"ok": true})
}
