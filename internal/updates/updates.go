package updates

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

// UpdateRow: satu pembaruan tersedia. Kind memisahkan tab di panel (runtime, service, app, tool, lib, panel).
type UpdateRow struct {
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	Name       string `json:"name"`
	From       string `json:"from"`
	To         string `json:"to"`
	Pinned     bool   `json:"pinned"`
	Major      bool   `json:"major"`
	Upgradable bool   `json:"upgradable"`
	Target     string `json:"target"`
	Note       string `json:"note"`
	Behind     int    `json:"behind,omitempty"`
}

// Check: hasil `lab update check --json`, sudah disaring.
type Check struct {
	Items  []UpdateRow
	Pins   []string
	Errors []string
}

var (
	kindOK   = map[string]bool{"runtime": true, "service": true, "app": true, "tool": true, "lib": true, "panel": true}
	sourceOK = map[string]bool{"homebrew": true, "mise": true, "npm": true, "git": true}
	targetRE = regexp.MustCompile(`^(mise:[a-z]{1,20}|npm:[a-z0-9][a-z0-9._-]{0,59}|panel|[a-z0-9][a-z0-9@._+-]{0,59})$`)
	npmRE    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,59}$`)
	verRE    = regexp.MustCompile(`^[A-Za-z0-9._+~-]{1,40}$`)
)

// ParseCheck membaca keluaran `lab update check --json`. Baris yang bentuknya tidak dikenal dibuang, bukan dipercaya:
// panel meneruskan Target ke `lab update apply`, jadi hanya bentuk yang sah yang boleh lewat.
func ParseCheck(out string) (Check, error) {
	var raw struct {
		Items  []UpdateRow `json:"items"`
		Pins   []string    `json:"pins"`
		Errors []string    `json:"errors"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return Check{}, err
	}
	c := Check{Items: []UpdateRow{}, Pins: []string{}, Errors: []string{}}
	for _, r := range raw.Items {
		if !kindOK[r.Kind] || !sourceOK[r.Source] || !targetRE.MatchString(r.Target) || !verRE.MatchString(r.From) || !verRE.MatchString(r.To) || r.Name == "" || len(r.Name) > 60 {
			continue
		}
		if len(r.Note) > 12 {
			r.Note = ""
		}
		c.Items = append(c.Items, r)
	}
	for _, p := range raw.Pins {
		if formulaRE.MatchString(p) {
			c.Pins = append(c.Pins, p)
		}
	}
	for _, e := range raw.Errors {
		if len(e) <= 60 {
			c.Errors = append(c.Errors, e)
		}
	}
	return c, nil
}

var formulaRE = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]{0,59}$`)

func UpdatesCheck(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLabTimeout([]string{"update", "check", "--json"}, "", a.Timeout)
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal memeriksa pembaruan")
		return
	}
	c, err := ParseCheck(res.Out)
	if err != nil {
		core.WriteError(w, 502, "keluaran lab update check bukan JSON")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"updates": c.Items, "pins": c.Pins, "errors": c.Errors, "checked": a.Now().Unix()})
}

func UpdatesApply(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		Target, Confirm string
		AllowPinned     bool `json:"allow_pinned"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	t := in.Target
	switch {
	case t == "panel":
		core.WriteError(w, 400, "MacPanel diperbarui lewat make deploy, bukan dari panel")
		return
	case t == "lib:all":
	case strings.HasPrefix(t, "npm:"):
		if !npmRE.MatchString(strings.TrimPrefix(t, "npm:")) {
			core.WriteError(w, 400, "nama aplikasi npm tidak valid")
			return
		}
	case strings.HasPrefix(t, "mise:"):
		if !core.RuntimeTools[strings.TrimPrefix(t, "mise:")] {
			core.WriteError(w, 400, "tool mise tidak dikenal")
			return
		}
	case !formulaRE.MatchString(t):
		core.WriteError(w, 400, "nama formula tidak valid")
		return
	}
	if in.Confirm != t {
		core.WriteError(w, 400, "ketik ulang nama target untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	args := []string{"update", "apply", t}
	if in.AllowPinned {
		args = append(args, "--allow-pinned")
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "update_apply", t, args, ""))
}
