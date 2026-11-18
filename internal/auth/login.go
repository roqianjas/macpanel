package auth

import (
	"net/http"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/store"
)

func MyPassword(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Current, New string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	ip := a.ClientIP(r)
	if a.Locked(ss.User, ip) {
		core.TooMany(w)
		return
	}
	u := a.Store.GetUser(ss.User)
	if u == nil || len(in.Current) > 256 || !crypto.VerifyPassword(u.PassHash, in.Current) {
		a.Fail(ss.User, ip)
		a.Store.Audit(ss.User, "password_change_fail", "", ip, a.Now())
		core.WriteError(w, 400, "kata sandi saat ini salah")
		return
	}
	if len(in.New) < 12 || len(in.New) > 256 || strings.EqualFold(in.New, ss.User) || in.New == in.Current {
		core.WriteError(w, 400, "kata sandi baru minimal 12 karakter, berbeda dari yang lama dan dari nama pengguna")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	if err := a.Store.SetPassword(u.ID, crypto.HashPassword(in.New), ss.ID); err != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "password_change", "sesi lain dicabut", ip, a.Now())
	core.WriteJSON(w, 200, map[string]any{"ok": true})
}

func Login(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	var in struct{ Name, Password string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	name, ip := strings.ToLower(strings.TrimSpace(in.Name)), a.ClientIP(r)
	if len(name) > 40 || len(in.Password) > 256 {
		core.WriteError(w, 401, "login gagal")
		return
	}
	if a.Locked(name, ip) {
		a.Store.Audit(name, "login_locked", "", ip, a.Now())
		core.TooMany(w)
		return
	}
	u := a.Store.GetUser(name)
	ok := false
	if u != nil && !u.Disabled {
		ok = crypto.VerifyPassword(u.PassHash, in.Password)
	} else {
		crypto.VerifyPassword(a.Dummy, in.Password) // samakan waktu respons
	}
	if !ok {
		a.Fail(name, ip)
		a.Store.Audit(name, "login_fail", "password", ip, a.Now())
		core.WriteError(w, 401, "login gagal")
		return
	}
	if !u.TOTPOn {
		tok, full := a.Store.NewSession(u.ID, "full", a.Now())
		a.SetCookie(w, tok, int(store.AbsTTL.Seconds()))
		a.Store.ClearAttempts("u:" + name)
		a.Store.Audit(name, "login_ok_no2fa", "", ip, a.Now())
		core.WriteJSON(w, 200, map[string]any{"ok": true, "no2fa": true, "csrf": full.CSRF})
		return
	}
	if c, err := r.Cookie(store.TrustCookie); err == nil {
		if nt, ok := a.Store.UseTrusted(c.Value, u.ID, a.Now()); ok {
			tok, full := a.Store.NewSession(u.ID, "full", a.Now())
			a.SetCookie(w, tok, int(store.AbsTTL.Seconds()))
			setTrust(a, w, nt, int(store.TrustTTL.Seconds()))
			a.Store.ClearAttempts("u:" + name)
			a.Store.Audit(name, "login_trusted", "", ip, a.Now())
			core.WriteJSON(w, 200, map[string]any{"ok": true, "trusted": true, "csrf": full.CSRF})
			return
		}
	}
	tok, ss := a.Store.NewSession(u.ID, "pwd", a.Now())
	a.SetCookie(w, tok, int(store.PwdTTL.Seconds()))
	a.Store.Audit(name, "login_pwd_ok", "", ip, a.Now())
	core.WriteJSON(w, 200, map[string]any{"need_totp": true, "csrf": ss.CSRF})
}

// verifyCode memeriksa TOTP atau recovery code untuk user sesi. Gagal dihitung sebagai percobaan.
func verifyCode(a *core.App, ss *store.Session, code, ip string) bool {
	u := a.Store.GetUser(ss.User)
	if u == nil {
		return false
	}
	code = strings.TrimSpace(code)
	if len(code) == 6 {
		secret, err := crypto.Open(a.Key, u.TOTPEnc)
		if err != nil {
			return false
		}
		step, ok := crypto.VerifyTOTP(secret, code, a.Now(), u.TOTPLast)
		return ok && a.Store.UseTOTPStep(u.ID, step)
	}
	return len(code) == 11 && a.Store.UseRecovery(u.ID, code)
}

func TOTP(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		Code  string
		Trust bool
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	ip := a.ClientIP(r)
	if a.Locked(ss.User, ip) {
		a.Store.Audit(ss.User, "totp_locked", "", ip, a.Now())
		core.TooMany(w)
		return
	}
	if !verifyCode(a, ss, in.Code, ip) {
		a.Fail(ss.User, ip)
		a.Store.Audit(ss.User, "totp_fail", "", ip, a.Now())
		core.WriteError(w, 401, "kode salah")
		return
	}
	a.Store.DelSession(ss.ID)
	tok, full := a.Store.NewSession(ss.UserID, "full", a.Now()) // sesi baru: id berganti setelah naik tahap
	a.Store.ClearAttempts("u:" + ss.User)
	a.SetCookie(w, tok, int(store.AbsTTL.Seconds()))
	a.Store.Audit(ss.User, "login_ok", "", ip, a.Now())
	if in.Trust {
		setTrust(a, w, a.Store.AddTrusted(ss.UserID, DeviceLabel(r.UserAgent()), a.Now()), int(store.TrustTTL.Seconds()))
		a.Store.Audit(ss.User, "trust_add", "", ip, a.Now())
	}
	core.WriteJSON(w, 200, map[string]any{"ok": true, "csrf": full.CSRF})
}

func Logout(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	a.Store.DelSession(ss.ID)
	a.SetCookie(w, "", -1)
	a.Store.Audit(ss.User, "logout", "", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]bool{"ok": true})
}

func StepUp(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Password string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	ip := a.ClientIP(r)
	if a.Locked(ss.User, ip) {
		core.TooMany(w)
		return
	}
	u := a.Store.GetUser(ss.User)
	if u == nil || u.Disabled || len(in.Password) > 256 || !crypto.VerifyPassword(u.PassHash, in.Password) {
		a.Fail(ss.User, ip)
		a.Store.Audit(ss.User, "stepup_fail", "", ip, a.Now())
		core.WriteError(w, 400, "kata sandi salah") // 400 (bukan 401) agar UI tidak mengeluarkan sesi
		return
	}
	a.Store.SetStepUp(ss.ID, a.Now().Add(store.StepupTTL))
	a.Store.Audit(ss.User, "stepup_ok", "", ip, a.Now())
	core.WriteJSON(w, 200, map[string]any{"ok": true, "valid_seconds": int(store.StepupTTL.Seconds())})
}

func Me(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	core.WriteJSON(w, 200, map[string]any{"user": ss.User, "csrf": ss.CSRF, "stepup": ss.StepUp > a.Now().Unix(), "role": ss.Role, "via": ss.Via, "scope": ss.Scope, "must_change": ss.MustChange})
}

func Audit(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	core.WriteJSON(w, 200, map[string]any{"rows": a.Store.RecentAudit(100)})
}
