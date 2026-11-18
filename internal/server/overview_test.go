package server_test

import (
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/overview"
	"github.com/roqianjas/macpanel/internal/services"
	"github.com/roqianjas/macpanel/internal/sites"
	"github.com/roqianjas/macpanel/internal/system"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestComputeAlerts(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ok := &system.SystemInfo{DiskTotal: 1000, DiskFree: 500, Mem: 60, Backup: &system.BackupInfo{At: now.Unix() - 3600}}
	svc := []services.ServiceInfo{{ID: "nginx", Label: "nginx", Installed: true, State: "running"}}
	site := []sites.SiteRow{{SiteInfo: sites.SiteInfo{Name: "a", Host: "a.lab"}, Enabled: true, HTTP: 200, SSLDays: testenv.Ip(300)}}
	if a := overview.ComputeAlerts(ok, svc, site, now, true); len(a) != 0 {
		t.Fatalf("keadaan sehat tidak boleh ada peringatan: %+v", a)
	}
	has := func(al []overview.Alert, id, lvl string) bool {
		for _, x := range al {
			if x.ID == id && x.Level == lvl {
				return true
			}
		}
		return false
	}
	d := func(free uint64) []overview.Alert {
		s := *ok
		s.DiskFree = free
		return overview.ComputeAlerts(&s, svc, site, now, true)
	}
	if has(d(160), "disk", "warn") || has(d(160), "disk", "bad") {
		t.Error("84% tidak boleh memicu")
	}
	if !has(d(150), "disk", "warn") || !has(d(81), "disk", "warn") {
		t.Error("85-91% harus warn")
	}
	if !has(d(80), "disk", "bad") {
		t.Error("92% harus bad")
	}
	m := *ok
	m.Mem = 94
	if has(overview.ComputeAlerts(&m, svc, site, now, true), "mem", "warn") {
		t.Error("94% tidak boleh memicu")
	}
	m.Mem = 95
	if !has(overview.ComputeAlerts(&m, svc, site, now, true), "mem", "warn") {
		t.Error("95% harus warn")
	}
	down := []services.ServiceInfo{{ID: "redis", Label: "Redis", Installed: true, State: "stopped"}, {ID: "memcached", Installed: true, Optional: true, State: "stopped"}, {ID: "mailpit", Installed: false, Optional: true, State: "not_installed"}, {ID: "nginx2", Label: "nginx2", Installed: false, Optional: false, State: "not_installed"}}
	a := overview.ComputeAlerts(ok, down, site, now, true)
	if !has(a, "svc-redis", "bad") || has(a, "svc-memcached", "bad") || has(a, "svc-mailpit", "bad") || has(a, "svc-nginx2", "bad") {
		t.Errorf("hanya layanan wajib yang mati yang memicu: %+v", a)
	}
	mk := func(http int, ssl *int, en bool) []sites.SiteRow {
		return []sites.SiteRow{{SiteInfo: sites.SiteInfo{Name: "x", Host: "x.lab"}, Enabled: en, HTTP: http, SSLDays: ssl}}
	}
	if !has(overview.ComputeAlerts(ok, svc, mk(0, nil, true), now, true), "site-x", "bad") || has(overview.ComputeAlerts(ok, svc, mk(0, nil, true), now, false), "site-x", "bad") {
		t.Error("HTTP 0 hanya bila pemeriksaan aktif")
	}
	if !has(overview.ComputeAlerts(ok, svc, mk(502, nil, true), now, true), "site-x", "bad") || has(overview.ComputeAlerts(ok, svc, mk(404, nil, true), now, true), "site-x", "bad") {
		t.Error("5xx memicu, 404 tidak")
	}
	if has(overview.ComputeAlerts(ok, svc, mk(502, nil, false), now, true), "site-x", "bad") {
		t.Error("situs dijeda tidak boleh memicu")
	}
	if !has(overview.ComputeAlerts(ok, svc, mk(200, testenv.Ip(13), true), now, true), "ssl-x", "warn") || has(overview.ComputeAlerts(ok, svc, mk(200, testenv.Ip(14), true), now, true), "ssl-x", "warn") {
		t.Error("SSL <14 hari warn, 14 tidak")
	}
	if !has(overview.ComputeAlerts(ok, svc, mk(200, testenv.Ip(0), true), now, true), "ssl-x", "bad") {
		t.Error("SSL 0 hari bad")
	}
	b := *ok
	b.Backup = &system.BackupInfo{At: now.Unix() - 36*3600}
	if has(overview.ComputeAlerts(&b, svc, site, now, true), "backup", "warn") {
		t.Error("36 jam pas tidak memicu")
	}
	b.Backup = &system.BackupInfo{At: now.Unix() - 37*3600}
	if !has(overview.ComputeAlerts(&b, svc, site, now, true), "backup", "warn") {
		t.Error("37 jam harus warn")
	}
	b.Backup = nil
	if !has(overview.ComputeAlerts(&b, svc, site, now, true), "backup", "warn") {
		t.Error("tanpa backup harus warn")
	}
}
