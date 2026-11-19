package overview

import (
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/services"
	"github.com/roqianjas/macpanel/internal/sites"
	"github.com/roqianjas/macpanel/internal/system"
)

// Setiap peringatan inti membawa Code + Args agar UI bisa menerjemahkannya; teks Indonesia tetap ada sebagai cadangan.
func TestCoreAlertsCarryTranslationCodes(t *testing.T) {
	days := 3
	got := ComputeAlerts(
		&system.SystemInfo{DiskTotal: 100, DiskFree: 5, Mem: 97},
		[]services.ServiceInfo{{ID: "redis", Label: "Redis", Installed: true, State: "stopped"}},
		[]sites.SiteRow{{SiteInfo: sites.SiteInfo{Name: "shop", Host: "shop.lab"}, Enabled: true, HTTP: 0, SSLDays: &days}},
		time.Now(), true)
	want := map[string]bool{"disk.bad": false, "mem.high": false, "svc.down": false, "site.down": false, "ssl.soon": false, "backup.none": false}
	for _, a := range got {
		if a.Title == "" || a.Detail == "" {
			t.Errorf("teks cadangan hilang: %+v", a)
		}
		if _, ok := want[a.Code]; ok {
			want[a.Code] = true
		} else {
			t.Errorf("kode tak dikenal atau kosong: %q (%s)", a.Code, a.ID)
		}
	}
	for c, seen := range want {
		if !seen {
			t.Errorf("peringatan %s tidak muncul", c)
		}
	}
	for _, a := range got {
		if a.Code == "site.down" && a.Args["host"] != "shop.lab" {
			t.Errorf("args host salah: %v", a.Args)
		}
		if a.Code == "ssl.soon" && a.Args["n"] != 3 {
			t.Errorf("args n salah: %v", a.Args)
		}
	}
}
