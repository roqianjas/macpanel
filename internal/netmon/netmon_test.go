package netmon

import (
	"math"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/store"
)

func fp(v float64) *float64 { return &v }

func ok(ms float64) store.NetqPoint {
	return store.NetqPoint{TSState: "Running", TSSelf: 1, InetOK: true, InetMS: fp(ms), DNSLocal: 1, DNSPublicOK: true}
}

func ids(fs []Finding) map[string]string {
	m := map[string]string{}
	for _, f := range fs {
		m[f.ID] = f.Level
	}
	return m
}

func in(recent ...store.NetqPoint) Inputs {
	return Inputs{Snap: &Snapshot{At: 1000}, Recent: recent, Now: time.Unix(1010, 0)}
}

func TestHealthyProducesNothing(t *testing.T) {
	if fs := Findings(in(ok(5), ok(6), ok(5), ok(7), ok(5))); len(fs) != 0 {
		t.Fatalf("%+v", fs)
	}
}

func TestNoSnapshotYetStaysQuiet(t *testing.T) {
	if fs := Findings(Inputs{Now: time.Unix(1, 0)}); len(fs) != 0 {
		t.Fatalf("%+v", fs)
	}
}

func TestTailscaleDownNeedsTwoConsecutiveSamples(t *testing.T) {
	down := store.NetqPoint{TSState: "Stopped", TSSelf: 0, InetOK: true, InetMS: fp(5), DNSLocal: 1}
	if len(ids(Findings(in(down, ok(5), ok(5))))) != 0 {
		t.Fatal("satu sampel buruk tidak boleh memicu alert")
	}
	got := ids(Findings(in(down, down, ok(5))))
	if got["net-tailscale"] != "bad" {
		t.Fatalf("%v", got)
	}
	if len(ids(Findings(in(down, ok(5), down)))) != 0 {
		t.Fatal("harus BERTURUT-turut: sampel bagus di tengah memutus hitungan")
	}
}

func TestTailscaleRunningButOfflineIsFlagged(t *testing.T) {
	off := store.NetqPoint{TSState: "Running", TSSelf: 0, InetOK: true, InetMS: fp(5), DNSLocal: 1}
	got := Findings(in(off, off))
	if len(got) != 1 || got[0].ID != "net-tailscale" || got[0].Level != "bad" {
		t.Fatalf("%+v", got)
	}
}

func TestTailscaleNotMeasuredIsNotDown(t *testing.T) {
	// tailscale tidak ditemukan: state kosong dan self -1. Tidak mampu mengukur bukan berarti mati.
	un := store.NetqPoint{TSState: "", TSSelf: -1, InetOK: true, InetMS: fp(5), DNSLocal: 1}
	if fs := Findings(in(un, un, un)); len(fs) != 0 {
		t.Fatalf("%+v", fs)
	}
}

func TestInternetDownNeedsThreeInARow(t *testing.T) {
	d := store.NetqPoint{TSState: "Running", TSSelf: 1, InetOK: false, DNSLocal: 1}
	if _, has := ids(Findings(in(d, d, ok(5))))["net-internet"]; has {
		t.Fatal("2 gagal belum cukup")
	}
	if ids(Findings(in(d, d, d)))["net-internet"] != "warn" {
		t.Fatal("3 gagal berturut-turut harus memicu")
	}
}

func TestSlowInternet(t *testing.T) {
	slow := func() store.NetqPoint { return ok(450) }
	if ids(Findings(in(slow(), slow(), slow(), slow(), slow())))["net-slow"] != "warn" {
		t.Fatal("rata-rata 450 ms harus lambat")
	}
	if _, has := ids(Findings(in(ok(450), ok(450), ok(5), ok(5), ok(5))))["net-slow"]; has {
		t.Fatal("rata-rata 183 ms tidak lambat")
	}
	if _, has := ids(Findings(in(slow(), slow(), slow())))["net-slow"]; has {
		t.Fatal("kurang dari jendela: jangan menilai")
	}
}

func TestLocalDNS(t *testing.T) {
	bad := store.NetqPoint{TSState: "Running", TSSelf: 1, InetOK: true, InetMS: fp(5), DNSLocal: 0}
	if ids(Findings(in(bad, bad)))["net-dns"] != "warn" {
		t.Fatal("2 gagal DNS berturut-turut harus memicu")
	}
	skip := store.NetqPoint{TSState: "Running", TSSelf: 1, InetOK: true, InetMS: fp(5), DNSLocal: -1}
	if len(Findings(in(skip, skip))) != 0 {
		t.Fatal("DNS tidak dites (-1) bukan gagal")
	}
}

func TestStaleHidesEverythingElse(t *testing.T) {
	down := store.NetqPoint{TSState: "Stopped", TSSelf: 0}
	i := in(down, down, down)
	i.Stale, i.Now = true, time.Unix(1000+900, 0)
	fs := Findings(i)
	if len(fs) != 1 || fs[0].ID != "net-stale" || fs[0].Level != "warn" {
		t.Fatalf("%+v", fs)
	}
}

func TestSanitize(t *testing.T) {
	s := &Snapshot{
		Tailscale: &Tailscale{State: "Run\x00ning" + string(make([]byte, 0)), Peers: 3, PeersOnline: 9, Health: []string{string(make([]rune, 0))}},
		Internet:  Internet{OK: true, MS: fp(-5)},
		DNSLocal:  &DNS{Name: "panel.lab", OK: true, MS: fp(math.NaN())},
		DNSPublic: &DNS{Name: "x", OK: false, MS: fp(12)},
	}
	s.Sanitize()
	if s.Tailscale.PeersOnline != 0 {
		t.Fatalf("online > total harus dibuang: %+v", s.Tailscale)
	}
	if s.Internet.MS != nil || s.DNSLocal.MS != nil {
		t.Fatal("ms negatif atau NaN harus dibuang")
	}
	if s.DNSPublic.MS != nil {
		t.Fatal("DNS gagal tidak boleh punya ms")
	}
	s.Internet = Internet{OK: false, MS: fp(7)}
	s.Sanitize()
	if s.Internet.MS != nil {
		t.Fatal("internet mati tidak boleh punya latensi")
	}
	if s.Errors == nil || s.Tailscale.Health == nil {
		t.Fatal("daftar tidak boleh nil")
	}
	big := &Snapshot{Internet: Internet{OK: true, MS: fp(1e9)}}
	big.Sanitize()
	if big.Internet.MS != nil {
		t.Fatal("latensi mustahil lolos")
	}
}
