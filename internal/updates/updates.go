package updates

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type UpdateRow struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	From   string `json:"from"`
	To     string `json:"to"`
	Pinned bool   `json:"pinned"`
	Major  bool   `json:"major"`
}

var updLineRE = regexp.MustCompile(`^\s{2}(\S+)\s+(\S+)\s+->\s+(\S+)(.*)$`)

func ParseUpdates(out string) ([]UpdateRow, []string) {
	rows, pins, src := []UpdateRow{}, []string{}, ""
	for _, l := range strings.Split(out, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "Homebrew:"):
			src = "homebrew"
		case strings.HasPrefix(t, "mise:"):
			src = "mise"
		case strings.HasPrefix(t, "Pin aktif:"):
			for _, p := range strings.Split(strings.TrimPrefix(t, "Pin aktif:"), ",") {
				if p = strings.TrimSpace(p); p != "" && p != "-" {
					pins = append(pins, p)
				}
			}
			src = ""
		default:
			if m := updLineRE.FindStringSubmatch(l); m != nil && src != "" {
				rows = append(rows, UpdateRow{Source: src, Name: m[1], From: m[2], To: m[3], Pinned: strings.Contains(m[4], "PIN"), Major: strings.Contains(m[4], "MAYOR")})
			}
		}
	}
	return rows, pins
}

var formulaRE = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]{0,59}$`)

func UpdatesCheck(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLabTimeout([]string{"update", "check"}, "", a.Timeout)
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal memeriksa pembaruan")
		return
	}
	rows, pins := ParseUpdates(res.Out)
	core.WriteJSON(w, 200, map[string]any{"updates": rows, "pins": pins, "checked": a.Now().Unix()})
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
	if strings.HasPrefix(t, "mise:") {
		if !core.RuntimeTools[strings.TrimPrefix(t, "mise:")] {
			core.WriteError(w, 400, "tool mise tidak dikenal")
			return
		}
	} else if !formulaRE.MatchString(t) {
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
