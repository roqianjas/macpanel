package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Pengaturan umum (nama panel, tema bawaan), 2FA hidup/mati per akun, dan token API untuk agen/otomasi.
//
// Token API: tiga tingkat.
//   read    : hanya GET.
//   operate : semua aksi yang tidak butuh verifikasi ulang (tambah, ubah, deploy, backup, ...).
//   full    : juga aksi yang butuh verifikasi ulang (hapus, ubah config, pasang). Penghapusan tetap
//             mewajibkan nama target diketik (kolom confirm), sama seperti di UI.
// Tidak pernah lewat token: login/2FA/kata sandi/perangkat tepercaya/token (/api/auth, /api/me/*),
// kredensial database (/creds), dan isi .env situs (/env).

const settingsSchema = `
CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY, v TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tokens(id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL, prefix TEXT NOT NULL, hash TEXT UNIQUE NOT NULL, scope TEXT NOT NULL, created INTEGER NOT NULL,
  expires INTEGER NOT NULL, last_used INTEGER NOT NULL DEFAULT 0, last_ip TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS tokens_user ON tokens(user_id);
`

const (
	maxTokens   = 10
	tokenPrefix = "mpt_"
	defaultName = "MacPanel"
)

var (
	tokenScopes = map[string]int{"read": 1, "operate": 2, "full": 3}
	nameRE19    = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} _.&'-]{0,39}$`)
	tokNameRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,39}$`)
	themes      = map[string]bool{"system": true, "light": true, "dark": true}
)

// ---------- store: pengaturan ----------

func (s *Store) Setting(k, def string) string {
	var v string
	if s.db.QueryRow("SELECT v FROM settings WHERE k=?", k).Scan(&v) != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(k, v string) {
	s.db.Exec("INSERT INTO settings(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v", k, v)
}

// ---------- store: 2FA ----------

func (s *Store) SetTOTPPending(uid int64, enc []byte) {
	s.db.Exec("UPDATE users SET totp_pending=? WHERE id=?", enc, uid)
}

func (s *Store) GetTOTPPending(uid int64) []byte {
	var b []byte
	s.db.QueryRow("SELECT totp_pending FROM users WHERE id=?", uid).Scan(&b)
	return b
}

func (s *Store) RecoveryLeft(uid int64) int {
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM recovery WHERE user_id=? AND used IS NULL", uid).Scan(&n)
	return n
}

// EnableTOTP memasang secret baru, mengganti recovery code, dan mencabut perangkat tepercaya serta sesi lain.
func (s *Store) EnableTOTP(uid int64, enc []byte, recHashes []string, step int64, keepSession string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET totp_enc=?, totp_on=1, totp_pending=NULL, totp_last=? WHERE id=?", enc, step, uid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM recovery WHERE user_id=?", uid); err != nil {
		return err
	}
	for _, h := range recHashes {
		if _, err := tx.Exec("INSERT INTO recovery(user_id,hash) VALUES(?,?)", uid, h); err != nil {
			return err
		}
	}
	tx.Exec("DELETE FROM sessions WHERE user_id=? AND id<>?", uid, keepSession)
	tx.Exec("DELETE FROM trusted WHERE user_id=?", uid)
	return tx.Commit()
}

func (s *Store) DisableTOTP(uid int64, keepSession string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET totp_on=0, totp_pending=NULL WHERE id=?", uid); err != nil {
		return err
	}
	tx.Exec("DELETE FROM recovery WHERE user_id=?", uid)
	tx.Exec("DELETE FROM sessions WHERE user_id=? AND id<>?", uid, keepSession)
	tx.Exec("DELETE FROM trusted WHERE user_id=?", uid)
	return tx.Commit()
}

// ---------- store: token ----------

type TokenRow struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Prefix   string `json:"prefix"`
	Scope    string `json:"scope"`
	Created  int64  `json:"created"`
	Expires  int64  `json:"expires"`
	LastUsed int64  `json:"last_used"`
	LastIP   string `json:"last_ip"`
	Owner    string `json:"owner,omitempty"`
}

// AddToken membuat token acak; hanya hash yang disimpan. Mengembalikan token utuh (tampil sekali) dan id.
func (s *Store) AddToken(uid int64, name, scope string, days int, now time.Time) (string, int64, bool) {
	s.db.Exec("DELETE FROM tokens WHERE expires<=?", now.Unix())
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM tokens WHERE user_id=?", uid).Scan(&n)
	if n >= maxTokens {
		return "", 0, false
	}
	tok := tokenPrefix + randToken()
	r, err := s.db.Exec("INSERT INTO tokens(user_id,name,prefix,hash,scope,created,expires) VALUES(?,?,?,?,?,?,?)",
		uid, name, tok[:len(tokenPrefix)+4], sha256hex(tok), scope, now.Unix(), now.Add(time.Duration(days)*24*time.Hour).Unix())
	if err != nil {
		return "", 0, false
	}
	id, _ := r.LastInsertId()
	return tok, id, true
}

func (s *Store) ListTokens(uid int64, now time.Time) []TokenRow {
	out := []TokenRow{}
	rows, err := s.db.Query("SELECT id,name,prefix,scope,created,expires,last_used,last_ip FROM tokens WHERE user_id=? AND expires>? ORDER BY id DESC", uid, now.Unix())
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t TokenRow
		rows.Scan(&t.ID, &t.Name, &t.Prefix, &t.Scope, &t.Created, &t.Expires, &t.LastUsed, &t.LastIP)
		out = append(out, t)
	}
	return out
}

func (s *Store) AllTokens(now time.Time) []TokenRow {
	out := []TokenRow{}
	rows, err := s.db.Query("SELECT t.id,t.name,t.prefix,t.scope,t.created,t.expires,t.last_used,t.last_ip,u.name FROM tokens t JOIN users u ON u.id=t.user_id WHERE t.expires>? ORDER BY t.id", now.Unix())
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t TokenRow
		rows.Scan(&t.ID, &t.Name, &t.Prefix, &t.Scope, &t.Created, &t.Expires, &t.LastUsed, &t.LastIP, &t.Owner)
		out = append(out, t)
	}
	return out
}

func (s *Store) RevokeToken(uid, id int64) bool {
	r, err := s.db.Exec("DELETE FROM tokens WHERE id=? AND user_id=?", id, uid)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

func (s *Store) RevokeTokenAny(id int64) bool {
	r, err := s.db.Exec("DELETE FROM tokens WHERE id=?", id)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

// TokenSession memeriksa token dan membentuk sesi sintetis (tanpa cookie, tanpa CSRF).
func (s *Store) TokenSession(tok, ip string, now time.Time) *Session {
	if len(tok) < 20 || len(tok) > 100 || !strings.HasPrefix(tok, tokenPrefix) {
		return nil
	}
	var id, uid, exp int64
	var user, role, scope, name string
	err := s.db.QueryRow(`SELECT t.id,t.user_id,u.name,u.role,t.scope,t.expires,t.name FROM tokens t JOIN users u ON u.id=t.user_id
	  WHERE t.hash=? AND u.disabled=0`, sha256hex(tok)).Scan(&id, &uid, &user, &role, &scope, &exp, &name)
	if err != nil || exp <= now.Unix() {
		return nil
	}
	if _, ok := tokenScopes[scope]; !ok {
		return nil
	}
	if role == "viewer" {
		scope = "read" // akun hanya-baca tidak pernah naik lewat token
	}
	s.db.Exec("UPDATE tokens SET last_used=?, last_ip=? WHERE id=?", now.Unix(), ip, id)
	return &Session{ID: fmt.Sprintf("token:%d", id), UserID: uid, User: user, Stage: "full", Role: role, Via: "token", Scope: scope, TokenName: name}
}

// tokenAllowed: rute yang tidak pernah boleh dipanggil dengan token.
func tokenAllowed(method, path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/auth/"), strings.HasPrefix(path, "/api/me/"):
		return false
	case strings.HasSuffix(path, "/creds"), strings.HasSuffix(path, "/env"):
		return false
	}
	return true
}

// bearer membaca "Authorization: Bearer <token>". Kembalian nil + kode bila tidak sah.
func (a *App) bearer(r *http.Request) (*Session, int) {
	ip := a.clientIP(r)
	if a.st.CountAttempts("ip:"+ip, a.now().Add(-lockWindow)) >= maxPerIP*3 {
		return nil, 429
	}
	ss := a.st.TokenSession(strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), ip, a.now())
	if ss == nil {
		a.st.AddAttempt("ip:"+ip, a.now())
		a.st.Audit("token", "token_fail", "", ip, a.now())
		return nil, 401
	}
	return ss, 0
}

func humanOnly(w http.ResponseWriter, ss *Session) bool {
	if ss.Via == "token" {
		jerr(w, 403, "tidak tersedia lewat token API")
		return false
	}
	return true
}

// ---------- handler: pengaturan ----------

func (a *App) publicSettings() map[string]any {
	return map[string]any{"name": a.st.Setting("panel_name", defaultName), "theme": a.st.Setting("theme", "system")}
}

func hPanelSettingsPublic(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	jsonOut(w, 200, a.publicSettings())
}

func hPanelSettingsGet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	m := a.publicSettings()
	m["can_edit"] = ss.Role == "admin"
	jsonOut(w, 200, m)
}

func hPanelSettingsSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Name, Theme *string }
	if !decode(w, r, &in) {
		return
	}
	if ss.Role != "admin" {
		jerr(w, 403, "hanya admin")
		return
	}
	if in.Name == nil && in.Theme == nil {
		jerr(w, 400, "tidak ada yang diubah")
		return
	}
	var ch []string
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if !nameRE19.MatchString(n) {
			jerr(w, 400, "nama panel: 1-40 karakter (huruf, angka, spasi, _ . & ' -)")
			return
		}
		a.st.SetSetting("panel_name", n)
		ch = append(ch, "nama="+n)
	}
	if in.Theme != nil {
		if !themes[*in.Theme] {
			jerr(w, 400, "tema: system, light, atau dark")
			return
		}
		a.st.SetSetting("theme", *in.Theme)
		ch = append(ch, "tema="+*in.Theme)
	}
	a.st.Audit(ss.User, "settings_change", strings.Join(ch, " "), a.clientIP(r), a.now())
	jsonOut(w, 200, a.publicSettings())
}

// ---------- handler: 2FA ----------

func hGet2FA(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	u := a.st.GetUser(ss.User)
	if u == nil {
		jerr(w, 401, "unauthenticated")
		return
	}
	jsonOut(w, 200, map[string]any{"enabled": u.TOTPOn, "recovery_left": a.st.RecoveryLeft(u.ID), "pending": len(a.st.GetTOTPPending(u.ID)) > 0})
}

func hSetup2FA(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !humanOnly(w, ss) {
		return
	}
	u := a.st.GetUser(ss.User)
	if u == nil {
		jerr(w, 401, "unauthenticated")
		return
	}
	if u.TOTPOn {
		jerr(w, 409, "2FA sudah aktif")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	secret := newTOTPSecret()
	enc, err := seal(a.key, secret)
	if err != nil {
		jerr(w, 500, "gagal menyiapkan kunci")
		return
	}
	a.st.SetTOTPPending(u.ID, enc)
	a.st.Audit(ss.User, "2fa_setup", "", a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"secret": b32.EncodeToString(secret), "uri": otpauthURI(ss.User, secret)})
}

func hEnable2FA(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !humanOnly(w, ss) {
		return
	}
	var in struct{ Code string }
	if !decode(w, r, &in) {
		return
	}
	ip := a.clientIP(r)
	if a.locked(ss.User, ip) {
		tooMany(w)
		return
	}
	u := a.st.GetUser(ss.User)
	if u == nil || u.TOTPOn {
		jerr(w, 409, "2FA sudah aktif atau akun tidak ada")
		return
	}
	pend := a.st.GetTOTPPending(u.ID)
	if len(pend) == 0 {
		jerr(w, 400, "mulai dari 'Siapkan 2FA' dulu")
		return
	}
	secret, err := open(a.key, pend)
	if err != nil {
		jerr(w, 500, "kunci tidak terbaca")
		return
	}
	step, ok := verifyTOTP(secret, strings.TrimSpace(in.Code), a.now(), 0)
	if !ok {
		a.fail(ss.User, ip)
		a.st.Audit(ss.User, "2fa_enable_fail", "", ip, a.now())
		jerr(w, 400, "kode salah")
		return
	}
	codes := newRecoveryCodes(10)
	hs := make([]string, len(codes))
	for i, c := range codes {
		hs[i] = sha256hex(c)
	}
	if err := a.st.EnableTOTP(u.ID, pend, hs, step, ss.ID); err != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "2fa_enable", "sesi lain dan perangkat tepercaya dicabut", ip, a.now())
	jsonOut(w, 200, map[string]any{"ok": true, "recovery_codes": codes})
}

func hDisable2FA(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !humanOnly(w, ss) {
		return
	}
	var in struct{ Password, Code string }
	if !decode(w, r, &in) {
		return
	}
	ip := a.clientIP(r)
	if a.locked(ss.User, ip) {
		tooMany(w)
		return
	}
	u := a.st.GetUser(ss.User)
	if u == nil || !u.TOTPOn {
		jerr(w, 409, "2FA tidak aktif")
		return
	}
	if len(in.Password) > 256 || !verifyPassword(u.PassHash, in.Password) || !a.verifyCode(ss, in.Code, ip) {
		a.fail(ss.User, ip)
		a.st.Audit(ss.User, "2fa_disable_fail", "", ip, a.now())
		jerr(w, 400, "kata sandi atau kode salah") // 400 agar UI tidak mengeluarkan sesi
		return
	}
	if err := a.st.DisableTOTP(u.ID, ss.ID); err != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "2fa_disable", "sesi lain dan perangkat tepercaya dicabut", ip, a.now())
	jsonOut(w, 200, map[string]any{"ok": true})
}

// ---------- handler: token ----------

func hTokensList(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !humanOnly(w, ss) {
		return
	}
	jsonOut(w, 200, map[string]any{"tokens": a.st.ListTokens(ss.UserID, a.now()), "max": maxTokens, "scopes": []string{"read", "operate", "full"}})
}

func hTokenCreate(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !humanOnly(w, ss) {
		return
	}
	var in struct {
		Name, Scope string
		Days        int
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !tokNameRE.MatchString(in.Name) {
		jerr(w, 400, "nama token: 1-40 karakter (huruf, angka, spasi, _ . -)")
		return
	}
	if _, ok := tokenScopes[in.Scope]; !ok {
		jerr(w, 400, "tingkat: read, operate, atau full")
		return
	}
	if in.Days == 0 {
		in.Days = 90
	}
	if in.Days < 1 || in.Days > 365 {
		jerr(w, 400, "masa berlaku 1-365 hari")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	tok, id, ok := a.st.AddToken(ss.UserID, in.Name, in.Scope, in.Days, a.now())
	if !ok {
		jerr(w, 409, fmt.Sprintf("maksimum %d token aktif per akun; cabut yang tidak dipakai", maxTokens))
		return
	}
	a.st.Audit(ss.User, "token_create", fmt.Sprintf("%s (%s, %d hari)", in.Name, in.Scope, in.Days), a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"token": tok, "id": id, "scope": in.Scope, "days": in.Days, "note": "Token hanya tampil sekali. Simpan di tempat aman."})
}

func hTokenRevoke(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !humanOnly(w, ss) {
		return
	}
	var in struct{ ID int64 }
	if !decode(w, r, &in) {
		return
	}
	if !a.st.RevokeToken(ss.UserID, in.ID) {
		jerr(w, 404, "token tidak ada")
		return
	}
	a.st.Audit(ss.User, "token_revoke", fmt.Sprint(in.ID), a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]bool{"ok": true})
}

// hRoutes: daftar rute yang bisa dipanggil lewat token (untuk agen). min_scope = tingkat minimum; aksi yang
// butuh verifikasi ulang hanya berhasil dengan token "full".
func hRoutes(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	type row struct {
		Method   string `json:"method"`
		Path     string `json:"path"`
		MinScope string `json:"min_scope"`
	}
	out := []row{}
	for _, rt := range routes() {
		if rt.Level != Full || !tokenAllowed(rt.Method, rt.Path) {
			continue
		}
		m := "operate"
		if rt.Method == "GET" || rt.Method == "HEAD" {
			m = "read"
		}
		out = append(out, row{rt.Method, rt.Path, m})
	}
	jsonOut(w, 200, map[string]any{"routes": out, "note": "Aksi bertanda step-up (hapus, config, pasang, ...) butuh token 'full'; penghapusan wajib menyertakan confirm=<nama>. Rute /api/auth, /api/me/*, */creds, */env tidak tersedia lewat token."})
}
