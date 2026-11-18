package databases

import (
	"net/http"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func ParseDBList(out string) map[string][]string {
	res := map[string][]string{"mariadb": {}, "mysql": {}, "pgsql": {}}
	cur := ""
	for _, ln := range strings.Split(out, "\n") {
		switch t := strings.TrimSpace(ln); {
		case t == "MariaDB:":
			cur = "mariadb"
		case t == "MySQL:":
			cur = "mysql"
		case t == "PostgreSQL:":
			cur = "pgsql"
		case t != "" && t != "(kosong)" && cur != "" && core.DbRE.MatchString(t):
			res[cur] = append(res[cur], t)
		}
	}
	return res
}

func DBList(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLab([]string{"db", "list"}, "")
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca daftar database")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"databases": ParseDBList(res.Out), "system": []string{"dev"}})
}

// systemDBs adalah database/akun milik server sendiri; panel tidak boleh menghapus atau men-dump-nya (lab juga menolak).
var systemDBs = map[string]bool{"dev": true, "postgres": true, "template0": true, "template1": true, "mysql": true, "sys": true, "root": true, "admin": true, "information_schema": true, "performance_schema": true}

func ValidEngine(e string) bool { return e == "mariadb" || e == "mysql" || e == "pgsql" }

func dbParams(w http.ResponseWriter, r *http.Request) (engine, name string, ok bool) {
	engine, name = r.PathValue("engine"), r.PathValue("name")
	if !ValidEngine(engine) || !core.DbRE.MatchString(name) {
		core.WriteError(w, 400, "engine atau nama tidak valid")
		return "", "", false
	}
	if systemDBs[name] {
		core.WriteError(w, 400, "database sistem tidak boleh diubah dari panel")
		return "", "", false
	}
	return engine, name, true
}

func DBCreate(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Name, Engine, Env string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !ValidEngine(in.Engine) || !core.DbRE.MatchString(in.Name) {
		core.WriteError(w, 400, "engine atau nama tidak valid")
		return
	}
	args := []string{"db", "create", in.Name, "--engine", in.Engine}
	if in.Env != "" {
		p, err := a.ValidEnvFile(in.Env)
		if err != nil {
			core.WriteError(w, 400, err.Error())
			return
		}
		args = append(args, "--env", p)
	}
	a.Record(w, ss, a.ClientIP(r), "db_create", in.Engine+"/"+in.Name, a.Mutate(args, ""))
}

func DBDump(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "db_dump", engine+"/"+name, a.Mutate([]string{"db", "dump", name, "--engine", engine}, ""))
}

func DBDrop(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Confirm != name {
		core.WriteError(w, 400, "ketik ulang nama database untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	// `lab db drop` meminta nama diketik ulang lewat stdin; konfirmasi manusia sudah dilakukan di atas.
	a.Record(w, ss, a.ClientIP(r), "db_drop", engine+"/"+name, a.Mutate([]string{"db", "drop", name, "--engine", engine}, name+"\n"))
}
