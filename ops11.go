package main

import (
	"net/http"
	"regexp"
	"strconv"
)

var newSiteRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`)

var reservedNames = map[string]bool{"home": true, "panel": true, "db": true, "nginx": true, "kuma": true, "stats": true, "dev": true, "mysql": true, "mariadb": true, "postgres": true, "root": true}

func hInstall(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct {
		Kind, Name, PHP, DB, Runtime string
		Port                         int
	}
	if !decode(w, r, &in) {
		return
	}
	app := in.Kind == "go" || in.Kind == "python" || in.Kind == "nextjs"
	if in.Kind != "laravel" && in.Kind != "wordpress" && !app {
		jerr(w, 400, "jenis: laravel, wordpress, go, python, atau nextjs")
		return
	}
	if !newSiteRE.MatchString(in.Name) || reservedNames[in.Name] {
		jerr(w, 400, "nama: huruf kecil, angka, tanda hubung (maks 30), bukan nama sistem")
		return
	}
	if app {
		if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(in.Name) {
			jerr(w, 400, "nama proyek aplikasi diawali huruf")
			return
		}
		if in.Runtime != "" && !regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){0,2}$`).MatchString(in.Runtime) {
			jerr(w, 400, "versi runtime: mis. 24 atau 24.22.3")
			return
		}
		if in.Port != 0 && (in.Port < 1024 || in.Port > 65535 || blockedPorts[in.Port] || in.Port == 9101) {
			jerr(w, 400, "port 1024-65535, bukan port sistem")
			return
		}
		if !a.requireStepUp(w, ss) {
			return
		}
		args := []string{"appinstall", in.Kind, in.Name}
		if in.Runtime != "" {
			args = append(args, "--runtime", in.Runtime)
		}
		if in.Port != 0 {
			args = append(args, "--port", strconv.Itoa(in.Port))
		}
		respondAsync(w, a.startAsync(ss, a.clientIP(r), "install_"+in.Kind, in.Name, args, ""))
		return
	}
	if in.PHP == "" {
		in.PHP = "8.4"
	}
	if in.PHP != "8.2" && in.PHP != "8.3" && in.PHP != "8.4" && in.PHP != "8.5" {
		jerr(w, 400, "versi PHP 8.2-8.5")
		return
	}
	if in.DB == "" {
		in.DB = "mariadb"
	}
	if in.DB != "mariadb" && in.DB != "mysql" {
		jerr(w, 400, "database: mariadb atau mysql")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "install_"+in.Kind, in.Name, []string{"appinstall", in.Kind, in.Name, "--php", in.PHP, "--db", in.DB}, ""))
}

func hClone(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	src, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Dst    string
		WithDB bool `json:"with_db"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !newSiteRE.MatchString(in.Dst) || reservedNames[in.Dst] || in.Dst == src {
		jerr(w, 400, "nama tujuan: huruf kecil, angka, tanda hubung (maks 30), bukan nama sistem atau sama dengan sumber")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	args := []string{"siteclone", src, in.Dst}
	if in.WithDB {
		args = append(args, "--with-db")
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "site_clone", src+" -> "+in.Dst, args, ""))
}
