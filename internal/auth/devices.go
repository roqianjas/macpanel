package auth

import (
	"net/http"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func setTrust(a *core.App, w http.ResponseWriter, tok string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: store.TrustCookie, Value: tok, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

// DeviceLabel meringkas User-Agent menjadi label pendek (bukan data identitas).
func DeviceLabel(ua string) string {
	l := strings.ToLower(ua)
	br := "Browser"
	for _, b := range [][2]string{{"edg/", "Edge"}, {"firefox", "Firefox"}, {"chrome", "Chrome"}, {"safari", "Safari"}} {
		if strings.Contains(l, b[0]) {
			br = b[1]
			break
		}
	}
	os := "perangkat tidak dikenal"
	for _, o := range [][2]string{{"iphone", "iPhone"}, {"android", "Android"}, {"windows", "Windows"}, {"mac os", "macOS"}, {"linux", "Linux"}} {
		if strings.Contains(l, o[0]) {
			os = o[1]
			break
		}
	}
	return br + " di " + os
}

func Devices(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	cur := ""
	if c, err := r.Cookie(store.TrustCookie); err == nil {
		cur = c.Value
	}
	core.WriteJSON(w, 200, map[string]any{"devices": a.Store.ListTrusted(ss.UserID, cur, a.Now()), "days": int(store.TrustTTL.Hours() / 24)})
}

func DeviceRevoke(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		ID  int64 `json:"id"`
		All bool  `json:"all"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.All {
		a.Store.RevokeAllTrusted(ss.UserID)
		a.Store.Audit(ss.User, "trust_revoke_all", "", a.ClientIP(r), a.Now())
		setTrust(a, w, "", -1)
		core.WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if !a.Store.RevokeTrusted(ss.UserID, in.ID) {
		core.WriteError(w, 404, "perangkat tidak ditemukan")
		return
	}
	a.Store.Audit(ss.User, "trust_revoke", "", a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]bool{"ok": true})
}
