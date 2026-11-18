package databases

import (
	"net/http"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func DBUsers(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	res := a.RunLab([]string{"db", "users", name, "--engine", engine}, "")
	var l struct {
		Users []struct {
			User string `json:"user"`
			Host string `json:"host"`
		} `json:"users"`
		Cred    bool  `json:"cred"`
		Rotated int64 `json:"rotated"`
	}
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &l) {
		core.WriteError(w, 502, "gagal membaca user database")
		return
	}
	core.WriteJSON(w, 200, l)
}

func DBPasswd(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm, Password string }
	if !core.DecodeJSON(w, r, &in) || !validDBPw(w, in.Password) {
		return
	}
	if in.Confirm != name {
		core.WriteError(w, 400, "ketik ulang nama database untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	args, stdin := []string{"db", "passwd", name, "--engine", engine}, ""
	if in.Password != "" {
		args, stdin = append(args, "--password-stdin"), in.Password+"\n"
	}
	a.Record(w, ss, a.ClientIP(r), "db_passwd", engine+"/"+name, a.Mutate(args, stdin))
}

// DBCreds menampilkan kredensial proyek (rahasia). Hanya admin, selalu step-up, dicatat di audit tanpa isi.
func DBCreds(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if ss.Role == "viewer" {
		core.WriteError(w, 403, "hanya admin")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	res := a.RunLab([]string{"db", "creds", name, "--engine", engine}, "")
	var kv map[string]string
	if res.Code != 0 || !core.ParseLabJSON(res.Out, &kv) {
		core.WriteError(w, 404, "kredensial proyek tidak ada")
		return
	}
	a.Store.Audit(ss.User, "db_creds_view", engine+"/"+name, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, kv)
}
