package accounts

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/store"
)

var NameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,31}$`)

func Accounts(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.AdminOnly(w, ss) {
		return
	}
	core.WriteJSON(w, 200, map[string]any{"accounts": a.Store.AccountRows(ss.User)})
}

func AccountCreate(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.AdminOnly(w, ss) {
		return
	}
	var in struct{ Name, Role string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.ToLower(strings.TrimSpace(in.Name))
	if !NameRE.MatchString(in.Name) {
		core.WriteError(w, 400, "nama: huruf kecil, angka, _ atau -, 3-32 karakter, diawali huruf")
		return
	}
	if in.Role == "" {
		in.Role = "viewer"
	}
	if in.Role != "admin" && in.Role != "viewer" {
		core.WriteError(w, 400, "peran: admin atau viewer")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	if a.Store.GetUser(in.Name) != nil {
		core.WriteError(w, 409, "nama sudah dipakai")
		return
	}
	temp := crypto.RandToken()[:20]
	enc, err := crypto.Seal(a.Key, crypto.NewTOTPSecret()) // kolom wajib terisi; tidak dipakai sampai pemilik menyalakan 2FA sendiri
	if err != nil || !a.Store.CreateAccount(in.Name, crypto.HashPassword(temp), enc, in.Role, a.Now().Unix()) {
		core.WriteError(w, 500, "gagal membuat akun")
		return
	}
	a.Store.Audit(ss.User, "account_create", in.Name+" ("+in.Role+")", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"name": in.Name, "role": in.Role, "temp_password": temp, "note": "Kata sandi sementara tampil sekali. Pemilik wajib menggantinya saat masuk pertama dan dapat menyalakan 2FA sendiri."})
}

// accountTarget memeriksa admin, nama, dan bahwa target bukan diri sendiri.
func accountTarget(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) (*store.User, bool) {
	if !core.AdminOnly(w, ss) {
		return nil, false
	}
	name := r.PathValue("name")
	if !NameRE.MatchString(name) {
		core.WriteError(w, 400, "nama tidak valid")
		return nil, false
	}
	if name == ss.User {
		core.WriteError(w, 400, "tidak bisa dilakukan pada akun sendiri (kata sandi sendiri diganti di Pengaturan)")
		return nil, false
	}
	u := a.Store.GetUser(name)
	if u == nil {
		core.WriteError(w, 404, "akun tidak ada")
		return nil, false
	}
	return u, true
}

func lastAdminGuard(a *core.App, w http.ResponseWriter, target string) bool {
	if a.Store.AdminsExcept(target) < 1 {
		core.WriteError(w, 409, "tidak boleh menyisakan nol admin aktif")
		return false
	}
	return true
}

func AccountRole(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	var in struct{ Role string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Role != "admin" && in.Role != "viewer" {
		core.WriteError(w, 400, "peran: admin atau viewer")
		return
	}
	if in.Role == "viewer" && !lastAdminGuard(a, w, u.Name) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	if !a.Store.SetRole(u.Name, in.Role) {
		core.WriteError(w, 500, "gagal mengubah peran")
		return
	}
	if err := a.Store.RevokeUserTokens(u.ID); err != nil {
		core.WriteError(w, 500, "gagal mencabut token")
		return
	}
	a.Store.Audit(ss.User, "account_role", u.Name+" -> "+in.Role+" (sesi dan token dicabut)", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]bool{"ok": true})
}

func AccountDisable(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	accountSetDisabled(a, w, r, ss, true)
}

func AccountEnable(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	accountSetDisabled(a, w, r, ss, false)
}

func accountSetDisabled(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session, off bool) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	if off && !lastAdminGuard(a, w, u.Name) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	if !a.Store.SetDisabled(u.Name, off) {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	act := map[bool]string{true: "account_disable", false: "account_enable"}[off]
	a.Store.Audit(ss.User, act, u.Name, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]bool{"ok": true})
}

func AccountReset(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	temp := crypto.RandToken()[:20]
	if err := a.Store.SetTempPassword(u.ID, crypto.HashPassword(temp)); err != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "account_reset", u.Name+" (sesi, perangkat tepercaya, token dicabut)", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"name": u.Name, "temp_password": temp, "note": "Tampil sekali. Pemilik wajib menggantinya saat masuk."})
}

func Account2FAOff(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	if !u.TOTPOn {
		core.WriteError(w, 409, "2FA akun itu sudah mati")
		return
	}
	if a.Store.DisableTOTP(u.ID, "") != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "account_2fa_off", u.Name+" (sesi dan perangkat tepercaya dicabut)", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]bool{"ok": true})
}

func AccountRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	u, ok := accountTarget(a, w, r, ss)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Confirm != u.Name {
		core.WriteError(w, 400, "ketik ulang nama akun untuk konfirmasi")
		return
	}
	if !lastAdminGuard(a, w, u.Name) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	if !a.Store.DeleteUser(u.Name) {
		core.WriteError(w, 500, "gagal menghapus")
		return
	}
	a.Store.Audit(ss.User, "account_remove", u.Name, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]bool{"ok": true})
}

func Users(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if ss.Role == "viewer" {
		core.WriteError(w, 403, "hanya admin")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"users": a.Store.UserRows(), "howto": "Tambah atau ubah peran lewat terminal Mac mini: panel user add <nama> --role viewer; panel user role <nama> admin|viewer"})
}
