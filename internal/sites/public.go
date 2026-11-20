package sites

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

// Akses publik sementara per situs (Cloudflare Quick Tunnel lewat `lab public`). Semua logika keamanan inti ada di lab
// (listener loopback per situs, daftar tolak, batas waktu); sisi panel menambah konfirmasi, step-up, dan catatan Aktivitas.

var publicURLRE = regexp.MustCompile(`^https://[a-z0-9-]+\.trycloudflare\.com$`)

type publicTunnel struct {
	Site        string `json:"site"`
	Active      bool   `json:"active"`
	URL         string `json:"url"`
	Started     int64  `json:"started"`
	Expires     int64  `json:"expires"`
	SecondsLeft int64  `json:"seconds_left"`
}

type publicStatus struct {
	Installed  bool           `json:"installed"`
	TTLDefault int            `json:"ttl_default"`
	TTLMin     int            `json:"ttl_min"`
	TTLMax     int            `json:"ttl_max"`
	Tunnels    []publicTunnel `json:"tunnels"`
}

func readPublic(a *core.App) (publicStatus, bool) {
	var st publicStatus
	res := a.RunLabTimeout([]string{"public", "status", "--json"}, "", 30*time.Second)
	if res.Code != 0 || json.Unmarshal([]byte(res.Out), &st) != nil {
		return st, false
	}
	for i := range st.Tunnels { // nilai dari lab dibatasi lagi di sisi panel
		if !publicURLRE.MatchString(st.Tunnels[i].URL) {
			st.Tunnels[i].URL = ""
		}
	}
	return st, true
}

func PublicGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	st, ok := readPublic(a)
	if !ok {
		core.WriteError(w, 502, "gagal membaca status akses publik")
		return
	}
	var mine *publicTunnel
	for i := range st.Tunnels {
		if st.Tunnels[i].Site == name {
			mine = &st.Tunnels[i]
		}
	}
	core.WriteJSON(w, 200, map[string]any{"installed": st.Installed, "ttl_default": st.TTLDefault, "ttl_min": st.TTLMin,
		"ttl_max": st.TTLMax, "tunnel": mine, "active_total": len(st.Tunnels)})
}

func PublicOn(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct {
		TTL int  `json:"ttl_min"`
		Ack bool `json:"ack"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !in.Ack {
		core.WriteError(w, 400, "centang konfirmasi: situs ini akan terjangkau dari internet lewat alamat acak, tanpa batas tailnet")
		return
	}
	if in.TTL == 0 {
		in.TTL = 120
	}
	if in.TTL < 5 || in.TTL > 1440 {
		core.WriteError(w, 400, "batas waktu 5-1440 menit")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Mu.Lock()
	res := a.RunLabTimeout([]string{"public", "on", name, "--ttl", strconv.Itoa(in.TTL)}, "", 70*time.Second)
	a.Mu.Unlock()
	a.Record(w, ss, a.ClientIP(r), "public_on", name+" ttl="+strconv.Itoa(in.TTL)+"m", res)
}

func PublicOff(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "public_off", name, a.Mutate([]string{"public", "off", name}, ""))
}
