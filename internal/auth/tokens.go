package auth

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var TokNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,39}$`)

func TokensList(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.HumanOnly(w, ss) {
		return
	}
	core.WriteJSON(w, 200, map[string]any{"tokens": a.Store.ListTokens(ss.UserID, a.Now()), "max": store.MaxTokens, "scopes": []string{"read", "operate", "full"}})
}

func TokenCreate(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.HumanOnly(w, ss) {
		return
	}
	var in struct {
		Name, Scope string
		Days        int
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !TokNameRE.MatchString(in.Name) {
		core.WriteError(w, 400, "nama token: 1-40 karakter (huruf, angka, spasi, _ . -)")
		return
	}
	if _, ok := store.TokenScopes[in.Scope]; !ok {
		core.WriteError(w, 400, "tingkat: read, operate, atau full")
		return
	}
	if in.Days == 0 {
		in.Days = 90
	}
	if in.Days < 1 || in.Days > 365 {
		core.WriteError(w, 400, "masa berlaku 1-365 hari")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	tok, id, ok := a.Store.AddToken(ss.UserID, in.Name, in.Scope, in.Days, a.Now())
	if !ok {
		core.WriteError(w, 409, fmt.Sprintf("maksimum %d token aktif per akun; cabut yang tidak dipakai", store.MaxTokens))
		return
	}
	a.Store.Audit(ss.User, "token_create", fmt.Sprintf("%s (%s, %d hari)", in.Name, in.Scope, in.Days), a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"token": tok, "id": id, "scope": in.Scope, "days": in.Days, "note": "Token hanya tampil sekali. Simpan di tempat aman."})
}

func TokenRevoke(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.HumanOnly(w, ss) {
		return
	}
	var in struct{ ID int64 }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !a.Store.RevokeToken(ss.UserID, in.ID) {
		core.WriteError(w, 404, "token tidak ada")
		return
	}
	a.Store.Audit(ss.User, "token_revoke", fmt.Sprint(in.ID), a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]bool{"ok": true})
}
