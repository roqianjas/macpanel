package auth

import (
	"net/http"
	"regexp"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

// Kode bahasa: dua huruf kecil, opsional wilayah (mis. id, en, pt-br). Daftar yang benar-benar tersedia ada di UI;
// server hanya menjaga bentuknya sehingga bahasa baru tidak perlu mengubah server.
var langRE = regexp.MustCompile(`^[a-z]{2,3}(-[a-z]{2,4})?$`)

func MyLang(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		Lang string `json:"lang"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Lang != "" && !langRE.MatchString(in.Lang) {
		core.WriteError(w, 400, "kode bahasa tidak valid")
		return
	}
	if !a.Store.SetLang(ss.User, in.Lang) {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"lang": in.Lang})
}
