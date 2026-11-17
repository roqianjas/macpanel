package main

import (
	"net/http"
	"regexp"
)

var newSiteRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`)

var reservedNames = map[string]bool{"home": true, "panel": true, "db": true, "nginx": true, "kuma": true, "stats": true, "dev": true, "mysql": true, "postgres": true, "root": true}

func hInstall(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Kind, Name, PHP string }
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "laravel" && in.Kind != "wordpress" {
		jerr(w, 400, "jenis: laravel atau wordpress")
		return
	}
	if !newSiteRE.MatchString(in.Name) || reservedNames[in.Name] {
		jerr(w, 400, "nama: huruf kecil, angka, tanda hubung (maks 30), bukan nama sistem")
		return
	}
	if in.PHP == "" {
		in.PHP = "8.4"
	}
	if in.PHP != "8.2" && in.PHP != "8.3" && in.PHP != "8.4" && in.PHP != "8.5" {
		jerr(w, 400, "versi PHP 8.2-8.5")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "install_"+in.Kind, in.Name, []string{"appinstall", in.Kind, in.Name, "--php", in.PHP}, ""))
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
