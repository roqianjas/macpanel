package phpruntime

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

// Versi PHP untuk CLI (terminal) dan ringkasan versi web per situs. Aturan pemilihan ada di shim yang dibangkitkan `lab php shims`.

var phpVerRE = regexp.MustCompile(`^8\.[2-5]$`)

type phpSite struct {
	Site    string  `json:"site"`
	Version *string `json:"version"`
}

type phpDefault struct {
	Default    string    `json:"default"`
	Installed  []string  `json:"installed"`
	ShimsReady bool      `json:"shims_ready"`
	ShellReady bool      `json:"shell_ready"`
	BrewLinked string    `json:"brew_linked"`
	Sites      []phpSite `json:"sites"`
}

func PHPDefaultGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLabTimeout([]string{"php", "status", "--json"}, "", 30*time.Second)
	var in struct {
		Default    string    `json:"default"`
		Installed  []string  `json:"installed"`
		Shims      []string  `json:"shims"`
		ShellReady bool      `json:"shell_ready"`
		BrewLinked string    `json:"brew_linked"`
		Sites      []phpSite `json:"sites"`
	}
	if res.Code != 0 || json.Unmarshal([]byte(res.Out), &in) != nil {
		core.WriteError(w, 502, "gagal membaca pengaturan PHP")
		return
	}
	out := phpDefault{Installed: []string{}, Sites: []phpSite{}, ShellReady: in.ShellReady, ShimsReady: len(in.Shims) > 0}
	if phpVerRE.MatchString(in.Default) {
		out.Default = in.Default
	}
	if phpVerRE.MatchString(in.BrewLinked) {
		out.BrewLinked = in.BrewLinked
	}
	for _, v := range in.Installed { // nilai dari lab dibatasi lagi di sisi panel
		if phpVerRE.MatchString(v) {
			out.Installed = append(out.Installed, v)
		}
	}
	for _, s := range in.Sites {
		if !core.SiteRE.MatchString(s.Site) {
			continue
		}
		if s.Version != nil && !phpVerRE.MatchString(*s.Version) {
			s.Version = nil
		}
		out.Sites = append(out.Sites, s)
	}
	core.WriteJSON(w, 200, out)
}

func PHPDefaultSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct {
		Version string `json:"version"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !phpVerRE.MatchString(in.Version) || !core.PhpVersions[in.Version] {
		core.WriteError(w, 400, "versi PHP harus 8.2-8.5")
		return
	}
	a.Record(w, ss, a.ClientIP(r), "php_default", in.Version, a.Mutate([]string{"php", "default", in.Version}, ""))
}
