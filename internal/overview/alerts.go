package overview

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/exposure"
	"github.com/roqianjas/macpanel/internal/hardware"
	"github.com/roqianjas/macpanel/internal/netmon"
	"github.com/roqianjas/macpanel/internal/services"
	"github.com/roqianjas/macpanel/internal/sites"
	"github.com/roqianjas/macpanel/internal/store"
	"github.com/roqianjas/macpanel/internal/system"
)

type Alert struct {
	ID     string `json:"id"`
	Level  string `json:"level"` // bad | warn
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Link   string `json:"link"`
	// Code + Args: kunci terjemahan stabil untuk UI (Title/Detail tetap berbahasa Indonesia sebagai cadangan dan untuk notifikasi).
	Code string         `json:"code,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

func coded(a Alert, code string, args map[string]any) Alert {
	a.Code, a.Args = code, args
	return a
}

// ComputeAlerts murni (tanpa I/O) agar mudah diuji. Semua ambang ada di sini.
func ComputeAlerts(si *system.SystemInfo, svcs []services.ServiceInfo, sites []sites.SiteRow, now time.Time, probed bool) []Alert {
	out := []Alert{}
	if si != nil && si.DiskTotal > 0 {
		used := 100 * (1 - float64(si.DiskFree)/float64(si.DiskTotal))
		switch {
		case used >= 92:
			out = append(out, coded(Alert{ID: "disk", Level: "bad", Title: "Disk hampir penuh", Detail: fmt.Sprintf("%.0f%% terpakai, sisa %.1f GB", used, float64(si.DiskFree)/1e9), Link: "#/pantau"}, "disk.bad", map[string]any{"used": fmt.Sprintf("%.0f", used), "free": fmt.Sprintf("%.1f", float64(si.DiskFree)/1e9)}))
		case used >= 85:
			out = append(out, coded(Alert{ID: "disk", Level: "warn", Title: "Disk mulai penuh", Detail: fmt.Sprintf("%.0f%% terpakai, sisa %.1f GB", used, float64(si.DiskFree)/1e9), Link: "#/pantau"}, "disk.warn", map[string]any{"used": fmt.Sprintf("%.0f", used), "free": fmt.Sprintf("%.1f", float64(si.DiskFree)/1e9)}))
		}
	}
	if si != nil && si.Mem >= 95 {
		out = append(out, coded(Alert{ID: "mem", Level: "warn", Title: "Memori hampir habis", Detail: fmt.Sprintf("%.0f%% terpakai", si.Mem), Link: "#/pantau"}, "mem.high", map[string]any{"used": fmt.Sprintf("%.0f", si.Mem)}))
	}
	for _, s := range svcs {
		if s.Installed && !s.Optional && s.State != "running" {
			out = append(out, coded(Alert{ID: "svc-" + s.ID, Level: "bad", Title: s.Label + " tidak berjalan", Detail: "Status: " + s.State, Link: "#/aplikasi"}, "svc.down", map[string]any{"name": s.Label, "state": s.State}))
		}
	}
	for _, s := range sites {
		if !s.Enabled {
			continue
		}
		switch {
		case s.HTTP == 0 && probed:
			out = append(out, coded(Alert{ID: "site-" + s.Name, Level: "bad", Title: s.Host + " tidak menjawab", Detail: "Pemeriksaan HTTP gagal", Link: "#/website"}, "site.down", map[string]any{"host": s.Host}))
		case s.HTTP >= 500:
			out = append(out, coded(Alert{ID: "site-" + s.Name, Level: "bad", Title: s.Host + " error server", Detail: fmt.Sprintf("HTTP %d", s.HTTP), Link: "#/website"}, "site.error", map[string]any{"host": s.Host, "code": s.HTTP}))
		}
		if s.SSLDays != nil {
			switch {
			case *s.SSLDays <= 0:
				out = append(out, coded(Alert{ID: "ssl-" + s.Name, Level: "bad", Title: "Sertifikat " + s.Host + " kedaluwarsa", Detail: "Perbarui di tab SSL", Link: "#/website"}, "ssl.expired", map[string]any{"host": s.Host}))
			case *s.SSLDays < 14:
				out = append(out, coded(Alert{ID: "ssl-" + s.Name, Level: "warn", Title: "Sertifikat " + s.Host + " segera habis", Detail: fmt.Sprintf("%d hari lagi", *s.SSLDays), Link: "#/website"}, "ssl.soon", map[string]any{"host": s.Host, "n": *s.SSLDays}))
			}
		}
	}
	if si != nil {
		switch {
		case si.Backup == nil:
			out = append(out, coded(Alert{ID: "backup", Level: "warn", Title: "Belum ada backup", Detail: "Jalankan backup pertama", Link: "#/backup"}, "backup.none", nil))
		case now.Unix()-si.Backup.At > 36*3600:
			out = append(out, coded(Alert{ID: "backup", Level: "warn", Title: "Backup terakhir sudah lama", Detail: fmt.Sprintf("%d jam lalu", (now.Unix()-si.Backup.At)/3600), Link: "#/backup"}, "backup.old", map[string]any{"n": (now.Unix() - si.Backup.At) / 3600}))
		}
	}
	return out
}

// alState: cache peringatan (domain overview).
type alState struct {
	mu    sync.Mutex
	cache []Alert
	at    time.Time
}

// CurrentAlerts mengembalikan peringatan aktif (dihitung ulang paling cepat tiap 30 detik) dan waktu hitungnya.
// Dipakai handler /api/alerts dan pengirim notifikasi, supaya keduanya selalu melihat hal yang sama.
func CurrentAlerts(a *core.App) ([]Alert, time.Time) {
	al := core.Local[alState](a, "alerts")
	al.mu.Lock()
	defer al.mu.Unlock()
	if al.cache == nil || a.Now().Sub(al.at) > 30*time.Second {
		si := system.CachedSystemInfo(a)
		var svcs []services.ServiceInfo
		if res := a.RunLabTimeout([]string{"service", "list", "--json"}, "", 40*time.Second); res.Code == 0 {
			_ = json.Unmarshal([]byte(res.Out), &svcs)
		}
		for i := range svcs {
			if !services.SvcStates[svcs[i].State] {
				svcs[i].State = "error"
			}
		}
		al.cache, al.at = ComputeAlerts(&si, svcs, sites.ListSiteRows(a), a.Now(), a.ProbeAddr != ""), a.Now()
		for _, f := range hardware.Findings(hardware.InputsNow(a)) {
			al.cache = append(al.cache, Alert{ID: f.ID, Level: f.Level, Title: f.Title, Detail: f.Detail, Link: "#/pantau"})
		}
		for _, f := range netmon.Findings(netmon.InputsNow(a)) {
			al.cache = append(al.cache, Alert{ID: f.ID, Level: f.Level, Title: f.Title, Detail: f.Detail, Link: "#/pantau"})
		}
		for _, f := range exposure.Findings(exposure.InputsNow(a)) {
			al.cache = append(al.cache, Alert{ID: f.ID, Level: f.Level, Title: f.Title, Detail: f.Detail, Link: "#/pantau"})
		}
	}
	return nonNil(al.cache), al.at
}

func Alerts(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	list, at := CurrentAlerts(a)
	core.WriteJSON(w, 200, map[string]any{"alerts": list, "at": at.Unix(), "notify": NotifyHint(a), "notify_code": notifyCode(a)})
}

// nonNil menyalin daftar dan menjamin hasilnya tidak nil: JSON-nya harus "[]", bukan "null"
// (UI membaca .length langsung; null membuat seluruh aplikasi kosong).
func nonNil(l []Alert) []Alert {
	out := make([]Alert, len(l))
	copy(out, l)
	return out
}
