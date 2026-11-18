package hardware

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

// Thresholds: ambang peringatan perangkat keras. Bawaan = tebakan beralasan; pemilik dapat mengubahnya setelah
// melihat riwayat nyata mesinnya. Disimpan sebagai JSON di setting "hw_thresholds".
type Thresholds struct {
	CPUWarn   float64 `json:"cpu_warn"` // berlaku juga untuk GPU
	CPUBad    float64 `json:"cpu_bad"`
	SSDWarn   float64 `json:"ssd_warn"`
	SSDBad    float64 `json:"ssd_bad"`
	PressWarn float64 `json:"press_warn"`
	PressBad  float64 `json:"press_bad"`
	SwapMB    float64 `json:"swap_mb"`
}

const thresholdKey = "hw_thresholds"

func DefaultThresholds() Thresholds {
	return Thresholds{CPUWarn: 85, CPUBad: 95, SSDWarn: 70, SSDBad: 80, PressWarn: 80, PressBad: 90, SwapMB: 2048}
}

// Validate menolak nilai di luar akal sehat. Batas bawah mencegah alert yang menyala terus (mis. 20 °C),
// batas atas mencegah alert yang tidak mungkin menyala (mis. 300 °C).
func (t Thresholds) Validate() error {
	pair := func(name string, w, b, lo, hi float64) error {
		if w < lo || b > hi || w != w || b != b {
			return errors.New(name + ": nilai harus di antara " + ftoa(lo) + " dan " + ftoa(hi))
		}
		if w >= b {
			return errors.New(name + ": batas 'perhatian' harus lebih rendah dari batas 'penting'")
		}
		return nil
	}
	if err := pair("suhu CPU/GPU", t.CPUWarn, t.CPUBad, 40, 120); err != nil {
		return err
	}
	if err := pair("suhu SSD", t.SSDWarn, t.SSDBad, 40, 110); err != nil {
		return err
	}
	if err := pair("tekanan memori", t.PressWarn, t.PressBad, 30, 100); err != nil {
		return err
	}
	if t.SwapMB < 256 || t.SwapMB > 65536 || t.SwapMB != t.SwapMB {
		return errors.New("swap: antara 256 dan 65536 MB")
	}
	return nil
}

func ftoa(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// LoadThresholds membaca ambang tersimpan. Isi rusak atau tidak sah jatuh ke bawaan, bukan ke nol
// (nol akan membuat setiap pembacaan memicu alert).
func LoadThresholds(a *core.App) Thresholds {
	d := DefaultThresholds()
	raw := a.Store.Setting(thresholdKey, "")
	if raw == "" {
		return d
	}
	t := d // field yang tidak ada di JSON tetap bawaan
	if json.Unmarshal([]byte(raw), &t) != nil || t.Validate() != nil {
		return d
	}
	return t
}

// ThresholdsGet: GET /api/hw/thresholds (semua peran boleh membaca; hanya admin mengubah).
func ThresholdsGet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	core.WriteJSON(w, 200, map[string]any{"thresholds": LoadThresholds(a), "defaults": DefaultThresholds(), "can_edit": ss.Role == "admin" && ss.Via != "token"})
}

// ThresholdsSet: POST /api/hw/thresholds. Isi {"reset":true} mengembalikan bawaan.
func ThresholdsSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.AdminOnly(w, ss) {
		return
	}
	var in struct {
		Reset bool `json:"reset"`
		Thresholds
	}
	cur := LoadThresholds(a)
	in.Thresholds = cur
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Reset {
		a.Store.SetSetting(thresholdKey, "")
		a.Store.Audit(ss.User, "hw_thresholds", "reset ke bawaan", a.ClientIP(r), a.Now())
		ThresholdsGet(a, w, r, ss)
		return
	}
	if err := in.Thresholds.Validate(); err != nil {
		core.WriteError(w, 400, err.Error())
		return
	}
	b, _ := json.Marshal(in.Thresholds)
	a.Store.SetSetting(thresholdKey, string(b))
	a.Store.Audit(ss.User, "hw_thresholds", string(b), a.ClientIP(r), a.Now())
	ThresholdsGet(a, w, r, ss)
}
