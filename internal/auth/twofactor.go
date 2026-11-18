package auth

import (
	"net/http"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/store"
)

func Get2FA(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	u := a.Store.GetUser(ss.User)
	if u == nil {
		core.WriteError(w, 401, "unauthenticated")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"enabled": u.TOTPOn, "recovery_left": a.Store.RecoveryLeft(u.ID), "pending": len(a.Store.GetTOTPPending(u.ID)) > 0})
}

func Setup2FA(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.HumanOnly(w, ss) {
		return
	}
	u := a.Store.GetUser(ss.User)
	if u == nil {
		core.WriteError(w, 401, "unauthenticated")
		return
	}
	if u.TOTPOn {
		core.WriteError(w, 409, "2FA sudah aktif")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	secret := crypto.NewTOTPSecret()
	enc, err := crypto.Seal(a.Key, secret)
	if err != nil {
		core.WriteError(w, 500, "gagal menyiapkan kunci")
		return
	}
	a.Store.SetTOTPPending(u.ID, enc)
	a.Store.Audit(ss.User, "2fa_setup", "", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"secret": crypto.B32.EncodeToString(secret), "uri": crypto.OtpauthURI(ss.User, secret)})
}

func Enable2FA(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.HumanOnly(w, ss) {
		return
	}
	var in struct{ Code string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	ip := a.ClientIP(r)
	if a.Locked(ss.User, ip) {
		core.TooMany(w)
		return
	}
	u := a.Store.GetUser(ss.User)
	if u == nil || u.TOTPOn {
		core.WriteError(w, 409, "2FA sudah aktif atau akun tidak ada")
		return
	}
	pend := a.Store.GetTOTPPending(u.ID)
	if len(pend) == 0 {
		core.WriteError(w, 400, "mulai dari 'Siapkan 2FA' dulu")
		return
	}
	secret, err := crypto.Open(a.Key, pend)
	if err != nil {
		core.WriteError(w, 500, "kunci tidak terbaca")
		return
	}
	step, ok := crypto.VerifyTOTP(secret, strings.TrimSpace(in.Code), a.Now(), 0)
	if !ok {
		a.Fail(ss.User, ip)
		a.Store.Audit(ss.User, "2fa_enable_fail", "", ip, a.Now())
		core.WriteError(w, 400, "kode salah")
		return
	}
	codes := crypto.NewRecoveryCodes(10)
	hs := make([]string, len(codes))
	for i, c := range codes {
		hs[i] = crypto.Sha256hex(c)
	}
	if err := a.Store.EnableTOTP(u.ID, pend, hs, step, ss.ID); err != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "2fa_enable", "sesi lain dan perangkat tepercaya dicabut", ip, a.Now())
	core.WriteJSON(w, 200, map[string]any{"ok": true, "recovery_codes": codes})
}

func Disable2FA(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.HumanOnly(w, ss) {
		return
	}
	var in struct{ Password, Code string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	ip := a.ClientIP(r)
	if a.Locked(ss.User, ip) {
		core.TooMany(w)
		return
	}
	u := a.Store.GetUser(ss.User)
	if u == nil || !u.TOTPOn {
		core.WriteError(w, 409, "2FA tidak aktif")
		return
	}
	if len(in.Password) > 256 || !crypto.VerifyPassword(u.PassHash, in.Password) || !verifyCode(a, ss, in.Code, ip) {
		a.Fail(ss.User, ip)
		a.Store.Audit(ss.User, "2fa_disable_fail", "", ip, a.Now())
		core.WriteError(w, 400, "kata sandi atau kode salah") // 400 agar UI tidak mengeluarkan sesi
		return
	}
	if err := a.Store.DisableTOTP(u.ID, ss.ID); err != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "2fa_disable", "sesi lain dan perangkat tepercaya dicabut", ip, a.Now())
	core.WriteJSON(w, 200, map[string]any{"ok": true})
}
