package system

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

// Tailnet: ringkasan dari `lab tailscale status`. Hanya baca; perubahan (start/stop/uninstall) butuh sudo dan
// dijalankan pemilik di terminal, jadi tidak ada endpoint tulis (panel tidak memegang root).
type Tailnet struct {
	State       string   `json:"state"`
	SelfOnline  bool     `json:"self_online"`
	Peers       int      `json:"peers"`
	PeersOnline int      `json:"peers_online"`
	Health      []string `json:"health"`
	IP          *string  `json:"ip"`
	Name        *string  `json:"name"`
	Version     *string  `json:"version"`
}

type TailscaleInfo struct {
	Formula     bool     `json:"formula"`
	DaemonPlist bool     `json:"daemon_plist"`
	Tailnet     *Tailnet `json:"tailnet"`
	Error       *string  `json:"error"`
}

var tailStates = map[string]bool{"Running": true, "Stopped": true, "NeedsLogin": true, "NeedsMachineAuth": true, "Starting": true, "NoState": true}

// TailscaleStatus membaca ulang bentuk keluaran lab dan membatasi nilainya sebelum sampai ke browser.
func TailscaleStatus(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	res := a.RunLabTimeout([]string{"tailscale", "status"}, "", 20*time.Second)
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca status Tailscale")
		return
	}
	var in TailscaleInfo
	if err := json.Unmarshal([]byte(res.Out), &in); err != nil {
		core.WriteError(w, 502, "keluaran Tailscale tidak terbaca")
		return
	}
	if t := in.Tailnet; t != nil {
		if !tailStates[t.State] {
			t.State = "NoState"
		}
		if len(t.Health) > 3 {
			t.Health = t.Health[:3]
		}
	}
	core.WriteJSON(w, 200, in)
}
