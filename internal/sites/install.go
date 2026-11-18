package sites

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var newSiteRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`)

var ReservedNames = map[string]bool{"home": true, "panel": true, "db": true, "nginx": true, "kuma": true, "stats": true, "dev": true, "mysql": true, "mariadb": true, "postgres": true, "root": true}

func Install(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		Kind, Name, PHP, DB, Runtime string
		Port                         int
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	app := in.Kind == "go" || in.Kind == "python" || in.Kind == "nextjs"
	if in.Kind != "laravel" && in.Kind != "wordpress" && !app {
		core.WriteError(w, 400, "jenis: laravel, wordpress, go, python, atau nextjs")
		return
	}
	if !newSiteRE.MatchString(in.Name) || ReservedNames[in.Name] {
		core.WriteError(w, 400, "nama: huruf kecil, angka, tanda hubung (maks 30), bukan nama sistem")
		return
	}
	if app {
		if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(in.Name) {
			core.WriteError(w, 400, "nama proyek aplikasi diawali huruf")
			return
		}
		if in.Runtime != "" && !regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){0,2}$`).MatchString(in.Runtime) {
			core.WriteError(w, 400, "versi runtime: mis. 24 atau 24.22.3")
			return
		}
		if in.Port != 0 && (in.Port < 1024 || in.Port > 65535 || core.BlockedPorts[in.Port] || in.Port == 9101) {
			core.WriteError(w, 400, "port 1024-65535, bukan port sistem")
			return
		}
		if !a.RequireStepUp(w, ss) {
			return
		}
		args := []string{"appinstall", in.Kind, in.Name}
		if in.Runtime != "" {
			args = append(args, "--runtime", in.Runtime)
		}
		if in.Port != 0 {
			args = append(args, "--port", strconv.Itoa(in.Port))
		}
		core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "install_"+in.Kind, in.Name, args, ""))
		return
	}
	if in.PHP == "" {
		in.PHP = "8.4"
	}
	if in.PHP != "8.2" && in.PHP != "8.3" && in.PHP != "8.4" && in.PHP != "8.5" {
		core.WriteError(w, 400, "versi PHP 8.2-8.5")
		return
	}
	if in.DB == "" {
		in.DB = "mariadb"
	}
	if in.DB != "mariadb" && in.DB != "mysql" {
		core.WriteError(w, 400, "database: mariadb atau mysql")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "install_"+in.Kind, in.Name, []string{"appinstall", in.Kind, in.Name, "--php", in.PHP, "--db", in.DB}, ""))
}

func Clone(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	src, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Dst    string
		WithDB bool `json:"with_db"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !newSiteRE.MatchString(in.Dst) || ReservedNames[in.Dst] || in.Dst == src {
		core.WriteError(w, 400, "nama tujuan: huruf kecil, angka, tanda hubung (maks 30), bukan nama sistem atau sama dengan sumber")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	args := []string{"siteclone", src, in.Dst}
	if in.WithDB {
		args = append(args, "--with-db")
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "site_clone", src+" -> "+in.Dst, args, ""))
}
