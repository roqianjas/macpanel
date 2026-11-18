package sites

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"strconv"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/store"
)

func SettingsGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	st, err := readSite(a, name)
	if err != nil {
		core.WriteError(w, 404, err.Error())
		return
	}
	core.WriteJSON(w, 200, map[string]any{"name": name, "kind": st.Kind, "model": st.Model, "system": core.ProtectedSites[name]})
}

func SettingsSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Model
		Replace bool `json:"replace"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	st, err := readSite(a, name)
	if err != nil {
		core.WriteError(w, 404, err.Error())
		return
	}
	if st.Kind == "kustom" {
		core.WriteError(w, 400, "situs ini memakai config kustom; ubah lewat editor config")
		return
	}
	if st.Kind == "custom" && !in.Replace {
		core.WriteError(w, 409, "config sudah diedit manual; kirim replace=true untuk menimpanya (versi lama tersimpan)")
		return
	}
	m := in.Model
	if m.Mode == "" {
		core.WriteError(w, 400, "mode wajib diisi")
		return
	}
	content, err := Generate(a, name, m)
	if err != nil {
		core.WriteError(w, 400, err.Error())
		return
	}
	for _, al := range m.Aliases {
		if aliasConflict(a, name, al) {
			core.WriteError(w, 409, "domain "+al+" sudah dipakai situs lain")
			return
		}
	}
	if len(m.Aliases) > 0 { // sertifikat harus memuat semua nama sebelum nginx dimuat ulang
		if res := a.Mutate(append([]string{"site", "cert", name}, m.Aliases...), ""); res.Code != 0 {
			core.WriteError(w, 500, "gagal membuat sertifikat untuk domain tambahan: "+core.Scrub(res.Out))
			return
		}
	}
	if err := CheckConf(a, name, content); err != nil { // sabuk pengaman: hasil template harus lolos kebijakan yang sama
		core.WriteError(w, 500, "template menghasilkan config yang ditolak kebijakan: "+err.Error())
		return
	}
	a.Record(w, ss, a.ClientIP(r), "site_settings", name, applyConf(a, ss.User, name, content, "pengaturan"))
}

func ConfigGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	b, _ := os.ReadFile(a.SiteFile(name))
	core.WriteJSON(w, 200, map[string]any{"name": name, "content": string(b), "sha": crypto.Sha256hex(string(b)), "system": core.ProtectedSites[name], "versions": a.Store.Versions(name)})
}

func ConfigSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ Content, SHA string }
	if !core.DecodeJSONLimit(w, r, &in, 2*maxConfSize+1024) { // JSON memperbesar (escape); isi sebenarnya tetap dibatasi maxConfSize oleh parser
		return
	}
	cur, _ := os.ReadFile(a.SiteFile(name))
	if in.SHA != crypto.Sha256hex(string(cur)) {
		core.WriteError(w, 409, "config berubah sejak dibuka; muat ulang")
		return
	}
	if err := CheckConf(a, name, in.Content); err != nil {
		core.WriteError(w, 400, "ditolak kebijakan: "+err.Error())
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "site_config", name, applyConf(a, ss.User, name, in.Content, "editor config"))
}

func Restore(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ ID int64 }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	c, found := a.Store.VersionContent(name, in.ID)
	if !found {
		core.WriteError(w, 404, "versi tidak ada")
		return
	}
	if err := CheckConf(a, name, c); err != nil {
		core.WriteError(w, 400, "versi itu ditolak kebijakan saat ini: "+err.Error())
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "site_restore", name+"#"+strconv.FormatInt(in.ID, 10), applyConf(a, ss.User, name, c, "pulihkan versi "+strconv.FormatInt(in.ID, 10)))
}

func SSLGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	host := name + "." + a.TLD
	b, err := os.ReadFile(a.CertDir + "/" + host + ".pem")
	if err != nil {
		core.WriteJSON(w, 200, map[string]any{"host": host, "exists": false})
		return
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		core.WriteError(w, 500, "sertifikat tidak terbaca")
		return
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		core.WriteError(w, 500, "sertifikat tidak valid")
		return
	}
	days := int(c.NotAfter.Sub(a.Now()).Hours() / 24)
	core.WriteJSON(w, 200, map[string]any{"host": host, "exists": true, "issuer": c.Issuer.CommonName, "dns": c.DNSNames,
		"not_before": c.NotBefore.Unix(), "not_after": c.NotAfter.Unix(), "days_left": days, "expired": days < 0,
		"note": "Sertifikat lokal (mkcert) untuk domain .lab. Let's Encrypt tidak berlaku untuk .lab."})
}

func SSLRenew(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	args := []string{"site", "cert", name}
	if st, err := readSite(a, name); err == nil && st.Model != nil {
		args = append(args, st.Model.Aliases...)
	}
	a.Record(w, ss, a.ClientIP(r), "ssl_renew", name, a.Mutate(args, ""))
}
