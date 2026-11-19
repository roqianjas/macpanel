package exposure

import (
	"strings"
	"testing"
	"time"
)

func bp(v bool) *bool { return &v }

func lis(proc string, port int, scope string, reach *bool) Listener {
	return Listener{Proc: proc, Port: port, Bind: "*", Scope: scope, LanReachable: reach}
}

func TestKeyLowercasesAndHiddenUsesQuestionMark(t *testing.T) {
	if k := Key(Listener{Proc: "Mailpit", Port: 8025}); k != "mailpit:8025" {
		t.Fatal(k)
	}
	if k := Key(Listener{Proc: "?", Port: 22, Hidden: true}); k != "?:22" {
		t.Fatal(k)
	}
	if k := Key(Listener{Proc: "sshd", Port: 22, Hidden: true}); k != "?:22" {
		t.Fatalf("hidden selalu '?': %s", k)
	}
	if k := Key(Listener{Proc: "", Port: 9}); k != "?:9" {
		t.Fatal(k)
	}
}

func TestExposedRequiresScopeNonSystemAndProvenReachability(t *testing.T) {
	cases := []struct {
		name string
		l    Listener
		want bool
	}{
		{"wildcard terjangkau", lis("a", 1, "all", bp(true)), true},
		{"alamat LAN terjangkau", lis("a", 1, "lan", bp(true)), true},
		{"wildcard diblokir firewall", lis("a", 1, "all", bp(false)), false},
		{"wildcard tidak diketahui", lis("a", 1, "all", nil), false},
		{"loopback", lis("a", 1, "loopback", bp(true)), false},
		{"tailnet", lis("a", 1, "tailnet", bp(true)), false},
		{"milik sistem", Listener{Proc: "rapportd", Port: 1, Scope: "all", System: true, LanReachable: bp(true)}, false},
	}
	for _, c := range cases {
		if got := Exposed(c.l); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestStatusPriority(t *testing.T) {
	allow := map[string]bool{"nginx:80": true}
	cases := map[string]Listener{
		"loopback": lis("x", 1, "loopback", nil),
		"tailnet":  lis("x", 1, "tailnet", nil),
		"system":   {Proc: "x", Port: 1, Scope: "all", System: true, LanReachable: bp(true)},
		"unknown":  lis("x", 1, "all", nil),
		"blocked":  lis("x", 1, "all", bp(false)),
		"allowed":  lis("nginx", 80, "all", bp(true)),
		"flagged":  lis("mailpit", 8025, "all", bp(true)),
	}
	for want, l := range cases {
		if got := Status(l, allow); got != want {
			t.Errorf("%s: dapat %s", want, got)
		}
	}
	// diizinkan tetapi tidak terjangkau: tetap 'blocked' (fakta lebih penting daripada izin)
	if Status(lis("nginx", 80, "all", bp(false)), allow) != "blocked" {
		t.Error("izin tidak boleh menutupi fakta 'diblokir'")
	}
}

func TestValidEntry(t *testing.T) {
	for _, ok := range []string{"mailpit:8025", "nginx:80", "?:22", "python3.14:9119", "a-b_c.d:65535", "x:1"} {
		if !ValidEntry(ok) {
			t.Errorf("harus sah: %s", ok)
		}
	}
	for _, bad := range []string{"", "mailpit", ":80", "mailpit:0", "mailpit:65536", "mailpit:99999", "Mailpit:80", "a b:80", "a/b:80", "x:y", "x:80;rm", "x:-1", "?:", "??:22", strings.Repeat("a", 41) + ":1", "a:1\n", "a:1:2"} {
		if ValidEntry(bad) {
			t.Errorf("harus ditolak: %q", bad)
		}
	}
}

func TestSanitizeDropsBadPortsCapsAndDefaultsScope(t *testing.T) {
	ip := "192.168.0.103\x00"
	s := Snapshot{LanIP: &ip, Listeners: []Listener{
		{Proc: "ok", Port: 80, Scope: "all"},
		{Proc: "p0", Port: 0, Scope: "all"},
		{Proc: "big", Port: 70000, Scope: "all"},
		{Proc: "", Port: 81, Scope: "aneh"},
		{Proc: strings.Repeat("x", 500), Port: 82, Scope: "loopback"},
	}}
	s.Sanitize()
	if len(s.Listeners) != 3 {
		t.Fatalf("port mustahil harus dibuang: %+v", s.Listeners)
	}
	if s.Listeners[1].Proc != "?" || s.Listeners[1].Scope != "lan" {
		t.Fatalf("proc kosong jadi ?, scope tak dikenal jadi lan (hati-hati): %+v", s.Listeners[1])
	}
	if len(s.Listeners[2].Proc) > 60 {
		t.Fatal("nama proses harus dibatasi")
	}
	if s.Errors == nil || strings.ContainsRune(*s.LanIP, 0) {
		t.Fatal("Errors tidak boleh nil; byte kontrol dibuang")
	}
	var many Snapshot
	for i := 1; i <= 500; i++ {
		many.Listeners = append(many.Listeners, Listener{Proc: "p", Port: i, Scope: "all"})
	}
	many.Sanitize()
	if len(many.Listeners) != maxListeners {
		t.Fatalf("dibatasi %d, dapat %d", maxListeners, len(many.Listeners))
	}
}

func snap(ls ...Listener) *Snapshot { return &Snapshot{At: 1000, Listeners: ls, Errors: []string{}} }

func TestFindingsNeedConfirmationAndRespectAllowlist(t *testing.T) {
	now := time.Unix(1000, 0)
	mail := lis("mailpit", 8025, "all", bp(true))
	base := Inputs{Snap: snap(mail), Allow: DefaultAllow, Now: now}

	base.Counts = map[string]int{"mailpit:8025": 1}
	if f := Findings(base); len(f) != 0 {
		t.Fatalf("satu sampel belum cukup: %+v", f)
	}
	base.Counts = map[string]int{"mailpit:8025": ConfirmSamples}
	f := Findings(base)
	if len(f) != 1 || f[0].ID != "exposure-mailpit-8025" || f[0].Level != "warn" || !strings.Contains(f[0].Title, "Mailpit") && !strings.Contains(f[0].Title, "mailpit") {
		t.Fatalf("%+v", f)
	}
	base.Allow = append([]string{"mailpit:8025"}, DefaultAllow...)
	if f := Findings(base); len(f) != 0 {
		t.Fatalf("diizinkan harus hilang segera (hitungan tidak bergantung izin): %+v", f)
	}
}

func TestFindingsNginxAndSystemAndLoopbackNeverFlagged(t *testing.T) {
	in := Inputs{
		Snap:   snap(lis("nginx", 80, "all", bp(true)), Listener{Proc: "rapportd", Port: 49152, Scope: "all", System: true, LanReachable: bp(true)}, lis("mariadbd", 3306, "loopback", bp(true)), lis("x", 9, "all", bp(false)), lis("y", 10, "all", nil)),
		Counts: map[string]int{"nginx:80": 9, "rapportd:49152": 9, "mariadbd:3306": 9, "x:9": 9, "y:10": 9},
		Allow:  DefaultAllow, Now: time.Unix(1000, 0),
	}
	if f := Findings(in); len(f) != 0 {
		t.Fatalf("%+v", f)
	}
}

func TestFindingsSensitivePortIsBadAndHiddenIsNamedByService(t *testing.T) {
	redis := lis("redis-ser", 6379, "all", bp(true))
	hidden := Listener{Proc: "?", Port: 88, Scope: "all", LanReachable: bp(true), Hidden: true, Service: "kerberos"}
	in := Inputs{Snap: snap(redis, hidden), Counts: map[string]int{"redis-ser:6379": 3, "?:88": 3}, Allow: DefaultAllow, Now: time.Unix(1000, 0)}
	f := Findings(in)
	if len(f) != 2 {
		t.Fatalf("%+v", f)
	}
	by := map[string]Finding{}
	for _, x := range f {
		by[x.ID] = x
	}
	if by["exposure-redis-ser-6379"].Level != "bad" {
		t.Errorf("port data (redis) harus bad: %+v", by)
	}
	h := by["exposure-x-88"]
	if h.Level != "warn" || !strings.Contains(h.Title, "tidak dikenal") || !strings.Contains(h.Title, "kerberos") || !strings.Contains(h.Title, "88") {
		t.Errorf("%+v", h)
	}
}

func TestFindingsDedupAndOrder(t *testing.T) {
	a := lis("b", 2, "all", bp(true))
	in := Inputs{Snap: snap(a, a, lis("a", 1, "lan", bp(true))), Counts: map[string]int{"a:1": 2, "b:2": 2}, Allow: nil, Now: time.Unix(1000, 0)}
	f := Findings(in)
	if len(f) != 2 || f[0].ID != "exposure-a-1" || f[1].ID != "exposure-b-2" {
		t.Fatalf("satu temuan per key, urut stabil: %+v", f)
	}
}

func TestFindingsStaleAndNoSnapshot(t *testing.T) {
	if f := Findings(Inputs{}); len(f) != 0 {
		t.Fatal("tanpa snapshot: tidak ada temuan")
	}
	f := Findings(Inputs{Snap: snap(), Stale: true, Now: time.Unix(1000+3600, 0)})
	if len(f) != 1 || f[0].ID != "exposure-stale" || f[0].Level != "warn" || !strings.Contains(f[0].Detail, "60 menit") {
		t.Fatalf("%+v", f)
	}
	// data basi tidak boleh menampilkan temuan lama seakan-akan terkini
	f = Findings(Inputs{Snap: snap(lis("m", 1, "all", bp(true))), Counts: map[string]int{"m:1": 5}, Stale: true, Now: time.Unix(5000, 0)})
	if len(f) != 1 || f[0].ID != "exposure-stale" {
		t.Fatalf("%+v", f)
	}
}

func TestSnapshotJSONListsAreNeverNull(t *testing.T) {
	s := Snapshot{}
	s.Sanitize()
	if s.Listeners == nil || s.Errors == nil {
		t.Fatalf("%+v", s)
	}
}
