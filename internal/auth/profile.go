package auth

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

const maxAvatar = 60000

var (
	iconAvatarRE = regexp.MustCompile(`^icon:[a-z0-9]{2,16}$`)
	pngAvatarRE  = regexp.MustCompile(`^data:image/png;base64,[A-Za-z0-9+/]+={0,2}$`)
)

// ValidProfile: nama tampilan teks biasa (maks 40 karakter, tanpa karakter kontrol); avatar kosong, ikon bawaan, atau PNG data kecil.
func ValidProfile(display, avatar string) string {
	d := strings.TrimSpace(display)
	if utf8.RuneCountInString(d) > 40 {
		return "nama tampilan maksimal 40 karakter"
	}
	for _, r := range d {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return "nama tampilan tidak boleh berisi karakter kontrol atau < >"
		}
	}
	switch {
	case avatar == "", iconAvatarRE.MatchString(avatar):
	case len(avatar) <= maxAvatar && pngAvatarRE.MatchString(avatar):
	default:
		return "avatar harus kosong, ikon bawaan, atau gambar PNG kecil (maks 60 KB)"
	}
	return ""
}

func MyProfile(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		DisplayName string `json:"display_name"`
		Avatar      string `json:"avatar"`
	}
	if !core.DecodeJSONLimit(w, r, &in, 100<<10) {
		return
	}
	if msg := ValidProfile(in.DisplayName, in.Avatar); msg != "" {
		core.WriteError(w, 400, msg)
		return
	}
	d := strings.TrimSpace(in.DisplayName)
	if !a.Store.SetProfile(ss.User, d, in.Avatar) {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "profile_update", "nama tampilan dan avatar", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"display_name": d, "avatar": in.Avatar})
}
