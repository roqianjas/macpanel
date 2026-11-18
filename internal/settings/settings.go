package settings

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var NameRE19 = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} _.&'-]{0,39}$`)

var Themes = map[string]bool{"system": true, "light": true, "dark": true}

func publicSettings(a *core.App) map[string]any {
	return map[string]any{"name": a.Store.Setting("panel_name", store.DefaultName), "theme": a.Store.Setting("theme", "system")}
}

func PanelSettingsPublic(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.WriteJSON(w, 200, publicSettings(a))
}

func PanelSettingsGet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	m := publicSettings(a)
	m["can_edit"] = ss.Role == "admin"
	core.WriteJSON(w, 200, m)
}

func PanelSettingsSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Name, Theme *string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if ss.Role != "admin" {
		core.WriteError(w, 403, "hanya admin")
		return
	}
	if in.Name == nil && in.Theme == nil {
		core.WriteError(w, 400, "tidak ada yang diubah")
		return
	}
	var ch []string
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if !NameRE19.MatchString(n) {
			core.WriteError(w, 400, "nama panel: 1-40 karakter (huruf, angka, spasi, _ . & ' -)")
			return
		}
		a.Store.SetSetting("panel_name", n)
		ch = append(ch, "nama="+n)
	}
	if in.Theme != nil {
		if !Themes[*in.Theme] {
			core.WriteError(w, 400, "tema: system, light, atau dark")
			return
		}
		a.Store.SetSetting("theme", *in.Theme)
		ch = append(ch, "tema="+*in.Theme)
	}
	a.Store.Audit(ss.User, "settings_change", strings.Join(ch, " "), a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, publicSettings(a))
}
