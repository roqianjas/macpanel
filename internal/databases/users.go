package databases

import (
	"net/http"
	"regexp"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var dbUserRE = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

var dbPwRE = regexp.MustCompile(`^[A-Za-z0-9._+=:,@%^-]{12,100}$`)

var dbPrivs = map[string]bool{"ro": true, "rw": true, "all": true}

func validDBPw(w http.ResponseWriter, pw string) bool {
	if pw != "" && !dbPwRE.MatchString(pw) {
		core.WriteError(w, 400, "kata sandi: 12-100 karakter dari huruf, angka, dan . _ + = : , @ % ^ -")
		return false
	}
	return true
}

func DBUserList(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	res := a.RunLab([]string{"db", "user", "list", name, "--engine", engine}, "")
	var out map[string]any
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &out) {
		core.WriteError(w, 502, "gagal membaca user database")
		return
	}
	core.WriteJSON(w, 200, out)
}

func dbUserParams(w http.ResponseWriter, r *http.Request, ss *store.Session) (engine, name, user string, ok bool) {
	if !core.HumanOnly(w, ss) {
		return
	}
	engine, name, ok = dbParams(w, r)
	if !ok {
		return
	}
	user = r.PathValue("user")
	if !dbUserRE.MatchString(user) {
		core.WriteError(w, 400, "nama user tidak valid")
		return "", "", "", false
	}
	return
}

func DBUserAdd(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.HumanOnly(w, ss) {
		return
	}
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ User, Privs, Password string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !dbUserRE.MatchString(in.User) || !dbPrivs[in.Privs] || !validDBPw(w, in.Password) {
		if dbUserRE.MatchString(in.User) && dbPrivs[in.Privs] {
			return // validDBPw sudah menjawab
		}
		core.WriteError(w, 400, "user: huruf kecil, angka, _; 3-32 karakter. hak: ro, rw, atau all")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	out, ok := a.LabSecret(w, []string{"db", "user", "add", name, in.User, "--engine", engine, "--privs", in.Privs}, in.Password)
	if !ok {
		return
	}
	a.Store.Audit(ss.User, "dbuser_add", engine+"/"+name+" "+in.User+" ("+in.Privs+")", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, out)
}

func DBUserPasswd(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, user, ok := dbUserParams(w, r, ss)
	if !ok {
		return
	}
	var in struct{ Password string }
	if !core.DecodeJSON(w, r, &in) || !validDBPw(w, in.Password) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	out, ok := a.LabSecret(w, []string{"db", "user", "passwd", name, user, "--engine", engine}, in.Password)
	if !ok {
		return
	}
	a.Store.Audit(ss.User, "dbuser_passwd", engine+"/"+name+" "+user, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, out)
}

func DBUserPrivs(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, user, ok := dbUserParams(w, r, ss)
	if !ok {
		return
	}
	var in struct{ Privs string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !dbPrivs[in.Privs] {
		core.WriteError(w, 400, "hak: ro, rw, atau all")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	out, ok := a.LabSecret(w, []string{"db", "user", "privs", name, user, "--engine", engine, "--privs", in.Privs}, "")
	if !ok {
		return
	}
	a.Store.Audit(ss.User, "dbuser_privs", engine+"/"+name+" "+user+" -> "+in.Privs, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, out)
}

func DBUserRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, user, ok := dbUserParams(w, r, ss)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Confirm != user {
		core.WriteError(w, 400, "ketik ulang nama user untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	out, ok := a.LabSecret(w, []string{"db", "user", "remove", name, user, "--engine", engine}, "")
	if !ok {
		return
	}
	a.Store.Audit(ss.User, "dbuser_remove", engine+"/"+name+" "+user, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, out)
}

func DBRootPasswd(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.AdminOnly(w, ss) {
		return
	}
	engine := r.PathValue("engine")
	if !ValidEngine(engine) {
		core.WriteError(w, 400, "engine tidak valid")
		return
	}
	var in struct{ Confirm, Password string }
	if !core.DecodeJSON(w, r, &in) || !validDBPw(w, in.Password) {
		return
	}
	if in.Confirm != engine {
		core.WriteError(w, 400, "ketik ulang nama mesin ("+engine+") untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	out, ok := a.LabSecret(w, []string{"db", "root-passwd", "--engine", engine}, in.Password)
	if !ok {
		return
	}
	a.Store.Audit(ss.User, "dbroot_passwd", engine, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, out)
}
