package overview

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
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
}

// ComputeAlerts murni (tanpa I/O) agar mudah diuji. Semua ambang ada di sini.
func ComputeAlerts(si *system.SystemInfo, svcs []services.ServiceInfo, sites []sites.SiteRow, now time.Time, probed bool) []Alert {
	out := []Alert{}
	if si != nil && si.DiskTotal > 0 {
		used := 100 * (1 - float64(si.DiskFree)/float64(si.DiskTotal))
		switch {
		case used >= 92:
			out = append(out, Alert{"disk", "bad", "Disk hampir penuh", fmt.Sprintf("%.0f%% terpakai, sisa %.1f GB", used, float64(si.DiskFree)/1e9), "#/pantau"})
		case used >= 85:
			out = append(out, Alert{"disk", "warn", "Disk mulai penuh", fmt.Sprintf("%.0f%% terpakai, sisa %.1f GB", used, float64(si.DiskFree)/1e9), "#/pantau"})
		}
	}
	if si != nil && si.Mem >= 95 {
		out = append(out, Alert{"mem", "warn", "Memori hampir habis", fmt.Sprintf("%.0f%% terpakai", si.Mem), "#/pantau"})
	}
	for _, s := range svcs {
		if s.Installed && !s.Optional && s.State != "running" {
			out = append(out, Alert{"svc-" + s.ID, "bad", s.Label + " tidak berjalan", "Status: " + s.State, "#/aplikasi"})
		}
	}
	for _, s := range sites {
		if !s.Enabled {
			continue
		}
		switch {
		case s.HTTP == 0 && probed:
			out = append(out, Alert{"site-" + s.Name, "bad", s.Host + " tidak menjawab", "Pemeriksaan HTTP gagal", "#/website"})
		case s.HTTP >= 500:
			out = append(out, Alert{"site-" + s.Name, "bad", s.Host + " error server", fmt.Sprintf("HTTP %d", s.HTTP), "#/website"})
		}
		if s.SSLDays != nil {
			switch {
			case *s.SSLDays <= 0:
				out = append(out, Alert{"ssl-" + s.Name, "bad", "Sertifikat " + s.Host + " kedaluwarsa", "Perbarui di tab SSL", "#/website"})
			case *s.SSLDays < 14:
				out = append(out, Alert{"ssl-" + s.Name, "warn", "Sertifikat " + s.Host + " segera habis", fmt.Sprintf("%d hari lagi", *s.SSLDays), "#/website"})
			}
		}
	}
	if si != nil {
		switch {
		case si.Backup == nil:
			out = append(out, Alert{"backup", "warn", "Belum ada backup", "Jalankan backup pertama", "#/backup"})
		case now.Unix()-si.Backup.At > 36*3600:
			out = append(out, Alert{"backup", "warn", "Backup terakhir sudah lama", fmt.Sprintf("%d jam lalu", (now.Unix()-si.Backup.At)/3600), "#/backup"})
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

func Alerts(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
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
	}
	core.WriteJSON(w, 200, map[string]any{"alerts": al.cache, "at": al.at.Unix(), "notify": "hanya ditampilkan di panel; tidak ada pesan keluar"})
}
