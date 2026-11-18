package main

import (
	"net/http"
	"regexp"
	"strings"
)

// Manajemen akun panel (admin), user database tambahan, dan kata sandi root database.
// Semuanya: hanya admin, sesi manusia (tidak lewat token API), kata sandi ulang (step-up).

var (
	dbUserRE = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
	dbPwRE   = regexp.MustCompile(`^[A-Za-z0-9._+=:,@%^-]{12,100}$`)
	dbPrivs  = map[string]bool{"ro": true, "rw": true, "all": true}
)

// ---------- store ----------

type AccountRow struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Disabled   bool   `json:"disabled"`
	TOTPOn     bool   `json:"totp_on"`
	MustChange bool   `json:"must_change"`
	Created    int64  `json:"created"`
	Self       bool   `json:"self"`
}

func (s *Store) AccountRows(self string) []AccountRow {
	out := []AccountRow{}
	rows, err := s.db.Query("SELECT name,role,disabled,totp_on,must_change,created FROM users ORDER BY name")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var r AccountRow
		var d, t, m int
		if rows.Scan(&r.Name, &r.Role, &d, &t, &m, &r.Created) != nil {
			continue
		}
		r.Disabled, r.TOTPOn, r.MustChange, r.Self = d == 1, t == 1, m == 1, r.Name == self
		out = append(out, r)
	}
	return out
}

func (s *Store) CreateAccount(name, passHash string, totpEnc []byte, role string, now int64) bool {
	_, err := s.db.Exec("INSERT INTO users(name,pass_hash,totp_enc,created,role,totp_on,must_change) VALUES(?,?,?,?,?,0,1)", name, passHash, totpEnc, now, role)
	return err == nil
}

// SetTempPassword: kata sandi sementara dari admin; wajib diganti saat masuk. Semua sesi, perangkat tepercaya, dan token dicabut.
func (s *Store) SetTempPassword(uid int64, hash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET pass_hash=?, must_change=1 WHERE id=?", hash, uid); err != nil {
		return err
	}
	for _, q := range []string{"DELETE FROM sessions WHERE user_id=?", "DELETE FROM trusted WHERE user_id=?", "DELETE FROM tokens WHERE user_id=?"} {
		if _, err := tx.Exec(q, uid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetDisabled(name string, off bool) bool {
	v := 0
	if off {
		v = 1
	}
	r, err := s.db.Exec("UPDATE users SET disabled=? WHERE name=?", v, name)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	if n == 1 && off {
		_, _ = s.db.Exec("DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE name=?)", name)
		_, _ = s.db.Exec("DELETE FROM trusted WHERE user_id=(SELECT id FROM users WHERE name=?)", name)
	}
	return n == 1
}

func (s *Store) AdminsExcept(name string) int {
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin' AND disabled=0 AND name<>?", name).Scan(&n) // gagal baca = n 0 = dianggap tidak ada admin lain (aman: penjaga menolak)
	return n
}

// ---------- handler: akun panel ----------

func adminOnly(w http.ResponseWriter, ss *Session) bool {
	if !humanOnly(w, ss) {
		return false
	}
	if ss.Role != "admin" {
		jerr(w, 403, "hanya admin")
		return false
	}
	return true
}

func hAccounts(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !adminOnly(w, ss) {
		return
	}
	jsonOut(w, 200, map[string]any{"accounts": a.st.AccountRows(ss.User)})
}

func hAccountCreate(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !adminOnly(w, ss) {
		return
	}
	var in struct{ Name, Role string }
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.ToLower(strings.TrimSpace(in.Name))
	if !nameRE.MatchString(in.Name) {
		jerr(w, 400, "nama: huruf kecil, angka, _ atau -, 3-32 karakter, diawali huruf")
		return
	}
	if in.Role == "" {
		in.Role = "viewer"
	}
	if in.Role != "admin" && in.Role != "viewer" {
		jerr(w, 400, "peran: admin atau viewer")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	if a.st.GetUser(in.Name) != nil {
		jerr(w, 409, "nama sudah dipakai")
		return
	}
	temp := randToken()[:20]
	enc, err := seal(a.key, newTOTPSecret()) // kolom wajib terisi; tidak dipakai sampai pemilik menyalakan 2FA sendiri
	if err != nil || !a.st.CreateAccount(in.Name, hashPassword(temp), enc, in.Role, a.now().Unix()) {
		jerr(w, 500, "gagal membuat akun")
		return
	}
	a.st.Audit(ss.User, "account_create", in.Name+" ("+in.Role+")", a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"name": in.Name, "role": in.Role, "temp_password": temp, "note": "Kata sandi sementara tampil sekali. Pemilik wajib menggantinya saat masuk pertama dan dapat menyalakan 2FA sendiri."})
}

// accountTarget memeriksa admin, nama, dan bahwa target bukan diri sendiri.
func accountTarget(a *App, w http.ResponseWriter, r *http.Request, ss *Session) (*User, bool) {
	if !adminOnly(w, ss) {
		return nil, false
	}
	name := r.PathValue("name")
	if !nameRE.MatchString(name) {
		jerr(w, 400, "nama tidak valid")
		return nil, false
	}
	if name == ss.User {
		jerr(w, 400, "tidak bisa dilakukan pada akun sendiri (kata sandi sendiri diganti di Pengaturan)")
		return nil, false
	}
	u := a.st.GetUser(name)
	if u == nil {
		jerr(w, 404, "akun tidak ada")
		return nil, false
	}
	return u, true
}

func lastAdminGuard(a *App, w http.ResponseWriter, target string) bool {
	if a.st.AdminsExcept(target) < 1 {
		jerr(w, 409, "tidak boleh menyisakan nol admin aktif")
		return false
	}
	return true
}

func hAccountRole(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	var in struct{ Role string }
	if !decode(w, r, &in) {
		return
	}
	if in.Role != "admin" && in.Role != "viewer" {
		jerr(w, 400, "peran: admin atau viewer")
		return
	}
	if in.Role == "viewer" && !lastAdminGuard(a, w, u.Name) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	if !a.st.SetRole(u.Name, in.Role) {
		jerr(w, 500, "gagal mengubah peran")
		return
	}
	if _, err := a.st.db.Exec("DELETE FROM tokens WHERE user_id=?", u.ID); err != nil {
		jerr(w, 500, "gagal mencabut token")
		return
	}
	a.st.Audit(ss.User, "account_role", u.Name+" -> "+in.Role+" (sesi dan token dicabut)", a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func hAccountDisable(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	accountSetDisabled(a, w, r, ss, true)
}
func hAccountEnable(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	accountSetDisabled(a, w, r, ss, false)
}

func accountSetDisabled(a *App, w http.ResponseWriter, r *http.Request, ss *Session, off bool) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	if off && !lastAdminGuard(a, w, u.Name) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	if !a.st.SetDisabled(u.Name, off) {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	act := map[bool]string{true: "account_disable", false: "account_enable"}[off]
	a.st.Audit(ss.User, act, u.Name, a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func hAccountReset(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	temp := randToken()[:20]
	if err := a.st.SetTempPassword(u.ID, hashPassword(temp)); err != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "account_reset", u.Name+" (sesi, perangkat tepercaya, token dicabut)", a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"name": u.Name, "temp_password": temp, "note": "Tampil sekali. Pemilik wajib menggantinya saat masuk."})
}

func hAccount2FAOff(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	if !u.TOTPOn {
		jerr(w, 409, "2FA akun itu sudah mati")
		return
	}
	if a.st.DisableTOTP(u.ID, "") != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "account_2fa_off", u.Name+" (sesi dan perangkat tepercaya dicabut)", a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func hAccountRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if in.Confirm != u.Name {
		jerr(w, 400, "ketik ulang nama akun untuk konfirmasi")
		return
	}
	if !lastAdminGuard(a, w, u.Name) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	if !a.st.DeleteUser(u.Name) {
		jerr(w, 500, "gagal menghapus")
		return
	}
	a.st.Audit(ss.User, "account_remove", u.Name, a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]bool{"ok": true})
}

// ---------- handler: user database ----------

// labSecret menjalankan lab dengan kata sandi lewat stdin dan menjaga agar tidak pernah muncul di galat.
func (a *App) labSecret(w http.ResponseWriter, args []string, pw string) (map[string]any, bool) {
	stdin := ""
	if pw != "" {
		args = append(args, "--password-stdin")
		stdin = pw + "\n"
	}
	res := a.mutate(args, stdin)
	var out map[string]any
	if res.Code != 0 || !parseLabJSON(res.Out, &out) {
		msg := strings.TrimSpace(scrub(res.Out))
		if pw != "" {
			msg = strings.ReplaceAll(msg, pw, "***")
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		jerr(w, 422, msg)
		return nil, false
	}
	return out, true
}

func validDBPw(w http.ResponseWriter, pw string) bool {
	if pw != "" && !dbPwRE.MatchString(pw) {
		jerr(w, 400, "kata sandi: 12-100 karakter dari huruf, angka, dan . _ + = : , @ % ^ -")
		return false
	}
	return true
}

func hDBUserList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	res := a.runLab([]string{"db", "user", "list", name, "--engine", engine}, "")
	var out map[string]any
	if res.Code != 0 || !parseLabJSON(res.Out, &out) {
		jerr(w, 502, "gagal membaca user database")
		return
	}
	jsonOut(w, 200, out)
}

func dbUserParams(w http.ResponseWriter, r *http.Request, ss *Session) (engine, name, user string, ok bool) {
	if !humanOnly(w, ss) {
		return
	}
	engine, name, ok = dbParams(w, r)
	if !ok {
		return
	}
	user = r.PathValue("user")
	if !dbUserRE.MatchString(user) {
		jerr(w, 400, "nama user tidak valid")
		return "", "", "", false
	}
	return
}

func hDBUserAdd(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !humanOnly(w, ss) {
		return
	}
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ User, Privs, Password string }
	if !decode(w, r, &in) {
		return
	}
	if !dbUserRE.MatchString(in.User) || !dbPrivs[in.Privs] || !validDBPw(w, in.Password) {
		if dbUserRE.MatchString(in.User) && dbPrivs[in.Privs] {
			return // validDBPw sudah menjawab
		}
		jerr(w, 400, "user: huruf kecil, angka, _; 3-32 karakter. hak: ro, rw, atau all")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	out, ok := a.labSecret(w, []string{"db", "user", "add", name, in.User, "--engine", engine, "--privs", in.Privs}, in.Password)
	if !ok {
		return
	}
	a.st.Audit(ss.User, "dbuser_add", engine+"/"+name+" "+in.User+" ("+in.Privs+")", a.clientIP(r), a.now())
	jsonOut(w, 200, out)
}

func hDBUserPasswd(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, user, ok := dbUserParams(w, r, ss)
	if !ok {
		return
	}
	var in struct{ Password string }
	if !decode(w, r, &in) || !validDBPw(w, in.Password) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	out, ok := a.labSecret(w, []string{"db", "user", "passwd", name, user, "--engine", engine}, in.Password)
	if !ok {
		return
	}
	a.st.Audit(ss.User, "dbuser_passwd", engine+"/"+name+" "+user, a.clientIP(r), a.now())
	jsonOut(w, 200, out)
}

func hDBUserPrivs(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, user, ok := dbUserParams(w, r, ss)
	if !ok {
		return
	}
	var in struct{ Privs string }
	if !decode(w, r, &in) {
		return
	}
	if !dbPrivs[in.Privs] {
		jerr(w, 400, "hak: ro, rw, atau all")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	out, ok := a.labSecret(w, []string{"db", "user", "privs", name, user, "--engine", engine, "--privs", in.Privs}, "")
	if !ok {
		return
	}
	a.st.Audit(ss.User, "dbuser_privs", engine+"/"+name+" "+user+" -> "+in.Privs, a.clientIP(r), a.now())
	jsonOut(w, 200, out)
}

func hDBUserRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, user, ok := dbUserParams(w, r, ss)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if in.Confirm != user {
		jerr(w, 400, "ketik ulang nama user untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	out, ok := a.labSecret(w, []string{"db", "user", "remove", name, user, "--engine", engine}, "")
	if !ok {
		return
	}
	a.st.Audit(ss.User, "dbuser_remove", engine+"/"+name+" "+user, a.clientIP(r), a.now())
	jsonOut(w, 200, out)
}

func hDBRootPasswd(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if !adminOnly(w, ss) {
		return
	}
	engine := r.PathValue("engine")
	if !validEngine(engine) {
		jerr(w, 400, "engine tidak valid")
		return
	}
	var in struct{ Confirm, Password string }
	if !decode(w, r, &in) || !validDBPw(w, in.Password) {
		return
	}
	if in.Confirm != engine {
		jerr(w, 400, "ketik ulang nama mesin ("+engine+") untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	out, ok := a.labSecret(w, []string{"db", "root-passwd", "--engine", engine}, in.Password)
	if !ok {
		return
	}
	a.st.Audit(ss.User, "dbroot_passwd", engine, a.clientIP(r), a.now())
	jsonOut(w, 200, out)
}
