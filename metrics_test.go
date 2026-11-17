package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

const netstatFixture = `Name       Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll
lo0        16384 <Link#1>                         25768     0    6588285    25768     0    6588285     0
lo0        16384 127           127.0.0.1          25768     -    6588285    25768     -    6588285     -
gif0*      1280  <Link#2>                             0     0          0        0     0          0     0
en0        1500  <Link#7>    02:00:00:00:00:00        0     0          0        0     0          0     0
en1        1500  <Link#16>   02:00:00:00:00:01   115628     0  127070191    55025     0   20695228     0
en1        1500  192.168.1     192.168.1.20      115628     -  127070191    55025     -   20695228     -
utun0      1380  <Link#19>                            0     0          0        4     0        320     0
utun4      1380  <Link#21>                          500     0       9000      700     0       12000     0
utun4      1380  100.64.0.10/ 100.64.0.10       500     -       9000      700     -       12000     -
bridge0    1500  <Link#14>   02:00:00:00:00:00        0     0          0        0     0          0     0
awdl0      1500  <Link#17>   02:00:00:00:00:00       21     0      12303       24     0       3687     0
`

func TestParseNetstatRealShape(t *testing.T) {
	m := parseNetstat(netstatFixture)
	if len(m) != 2 {
		t.Fatalf("hanya interface ber-IPv4 non-loopback (LAN + Tailscale): %+v", m)
	}
	if e := m["en1"]; e.RX != 127070191 || e.TX != 20695228 || e.IP != "192.168.1.20" {
		t.Fatalf("en1 salah: %+v", e)
	}
	if u := m["utun4"]; u.RX != 9000 || u.TX != 12000 || u.IP != "100.64.0.10" {
		t.Fatalf("utun4 (kolom Address kosong, dihitung dari kanan) salah: %+v", u)
	}
	if _, ok := m["lo0"]; ok {
		t.Fatal("loopback tidak boleh dilaporkan")
	}
	if got := ifaceLabel("utun4", "100.64.0.10"); got != "Tailscale (utun4)" {
		t.Fatal(got)
	}
	if got := ifaceLabel("en1", "192.168.1.20"); got != "LAN (en1)" {
		t.Fatal(got)
	}
	if len(parseNetstat("sampah\n\n")) != 0 {
		t.Fatal("masukan sampah harus kosong")
	}
}

func TestParseTopTakesLastSample(t *testing.T) {
	out := "Processes: 1\nCPU usage: 11.23% user, 10.11% sys, 78.65% idle \nPhysMem: x\nCPU usage: 3.11% user, 2.24% sys, 94.64% idle \n"
	v, ok := parseTop(out)
	if !ok || v < 5.35 || v > 5.37 {
		t.Fatalf("harus sampel TERAKHIR (100-94.64=5.36), dapat %v %v", v, ok)
	}
	if _, ok := parseTop("tidak ada baris cpu"); ok {
		t.Fatal("tanpa baris CPU harus gagal, bukan 0")
	}
	if _, ok := parseTop("CPU usage: 1% user, 1% sys, 140% idle"); ok {
		t.Fatal("idle di luar 0-100 harus ditolak")
	}
}

func TestParseMemLoadBootPS(t *testing.T) {
	vm := "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages free: 70972.\nPages wired down: 100.\nPages purgeable: 50.\nAnonymous pages: 350.\nPages occupied by compressor: 50.\n"
	// (350-50+100+50) halaman * 16384 / (1000 halaman * 16384) = 45%
	if v, ok := parseVMStat(vm, 1000*16384); !ok || v != 45 {
		t.Fatalf("vm_stat: %v %v", v, ok)
	}
	if _, ok := parseVMStat(vm, 0); ok {
		t.Fatal("total 0 harus gagal")
	}
	if _, ok := parseVMStat("kosong", 1000); ok {
		t.Fatal("teks tak dikenal harus gagal")
	}
	if _, ok := parseVMStat(vm, 100*16384); ok {
		t.Fatal("hasil >100% harus ditolak")
	}
	if l, ok := parseLoad("{ 0.97 1.53 2.14 }\n"); !ok || l != [3]float64{0.97, 1.53, 2.14} {
		t.Fatalf("%v", l)
	}
	if b, ok := parseBoot("{ sec = 1763260000, usec = 0 } Sun Nov 16 02:26:40 2026"); !ok || b != 1763260000 {
		t.Fatalf("%v", b)
	}
	ps := parsePS("  PID  %CPU    RSS COMM\n  101  42.5 204800 Google Chrome Helper\n  102   1.0   1024 nginx\nbaris rusak\n", 5)
	if len(ps) != 2 || ps[0].Name != "Google Chrome Helper" || ps[0].CPU != 42.5 || ps[0].RSSMB != 200 {
		t.Fatalf("%+v", ps)
	}
	if got := parsePS("h\n1 1 1 a\n2 2 2 b\n3 3 3 c\n", 2); len(got) != 2 {
		t.Fatalf("batas n: %d", len(got))
	}
}

// fakeRun meniru perintah sistem; mencatat nama program dan menolak yang tidak diharapkan.
func fakeRunner(t *testing.T, net *string, cpuFail *bool, calls *[]string) runner {
	return func(_ time.Duration, name string, args ...string) (string, error) {
		*calls = append(*calls, name)
		switch name {
		case "/usr/bin/top":
			if *cpuFail {
				return "", errors.New("gagal")
			}
			return "CPU usage: 1.0% user, 1.0% sys, 90.0% idle\nCPU usage: 5.0% user, 5.0% sys, 90.0% idle\n", nil
		case "/usr/bin/vm_stat":
			return "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages wired down: 100.\nPages purgeable: 50.\nAnonymous pages: 350.\nPages occupied by compressor: 50.\n", nil
		case "/usr/sbin/sysctl":
			if len(args) > 1 && args[1] == "hw.memsize" {
				return "16384000\n", nil
			}
			return "{ 1.00 2.00 3.00 }\n", nil
		case "/usr/sbin/netstat":
			return *net, nil
		}
		t.Fatalf("program tak terduga dipanggil: %s", name)
		return "", nil
	}
}

func netText(rx, tx int) string {
	r, x := itoa(int64(rx)), itoa(int64(tx))
	return "Name Mtu Network Address Ipkts Ierrs Ibytes Opkts Oerrs Obytes Coll\n" +
		"en1 1500 <Link#16> 02:00:00:00:00:01 1 0 " + r + " 1 0 " + x + " 0\n" +
		"en1 1500 192.168.1 192.168.1.20 1 - " + r + " 1 - " + x + " -\n"
}

func TestCollectorRatesCounterResetAndNullOnFailure(t *testing.T) {
	o := newOpsEnv(t, "echo ok")
	var calls []string
	net, cpuFail := netText(1000, 500), false
	c := newCollector(o.app)
	c.run = fakeRunner(t, &net, &cpuFail, &calls)
	c.disk = func() (uint64, uint64) { return 1000, 250 }
	t0 := time.Unix(1_800_000_000, 0)

	c.Sample(t0) // sampel pertama: belum ada laju
	if n := len(o.app.st.NetSeries(0, 30)); n != 0 {
		t.Fatalf("sampel pertama tidak boleh punya laju: %d", n)
	}
	net = netText(4000, 2000) // +3000 rx, +1500 tx dalam 30 dtk
	c.Sample(t0.Add(30 * time.Second))
	ns := o.app.st.NetSeries(0, 30)
	if len(ns) != 1 || ns[0].Iface != "en1" || ns[0].RX[0][1] != 100 || ns[0].TX[0][1] != 50 || ns[0].TotalRX != 3000 || ns[0].TotalTX != 1500 {
		t.Fatalf("laju salah (harus 100 B/s dan 50 B/s): %+v", ns)
	}
	if ns[0].Label != "LAN (en1)" {
		t.Fatal(ns[0].Label)
	}
	net = netText(10, 10) // penghitung reset (reboot): bukan laju negatif/raksasa
	c.Sample(t0.Add(60 * time.Second))
	ns = o.app.st.NetSeries(0, 30)
	if len(ns[0].RX) != 1 {
		t.Fatalf("reset penghitung tidak boleh menambah titik: %+v", ns[0].RX)
	}
	cpuFail = true // CPU gagal dibaca: NULL, bukan 0
	c.Sample(t0.Add(90 * time.Second))
	cpu, mem, disk, load := o.app.st.Series(0, 30)
	if len(cpu) != 3 || len(mem) != 4 {
		t.Fatalf("cpu harus 3 titik (1 gagal), mem 4: cpu=%d mem=%d", len(cpu), len(mem))
	}
	if cpu[0][1] != 10 || mem[0][1] != 45 || disk[0][1] != 75 || load[0][1] != 1 {
		t.Fatalf("nilai: cpu=%v mem=%v disk=%v load=%v", cpu[0], mem[0], disk[0], load[0])
	}
	for _, n := range calls {
		if !strings.HasPrefix(n, "/usr/") && !strings.HasPrefix(n, "/bin/") {
			t.Fatalf("hanya path mutlak sistem yang boleh dipanggil: %s", n)
		}
	}
}

func TestSeriesBucketsAndPrune(t *testing.T) {
	o := newOpsEnv(t, "echo ok")
	st := o.app.st
	for i := int64(0); i < 10; i++ {
		st.AddSample(1000+i*30, float64(i*10), 50, 40, 1, true, true)
	}
	cpu, _, _, _ := st.Series(0, 300) // 10 titik x 30 dtk = 300 dtk -> 1-2 bucket
	if len(cpu) < 1 || len(cpu) > 2 {
		t.Fatalf("bucket 300: %v", cpu)
	}
	raw, _, _, _ := st.Series(0, 30)
	if len(raw) != 10 {
		t.Fatalf("mentah: %d", len(raw))
	}
	if only, _, _, _ := st.Series(1000+5*30, 30); len(only) != 5 {
		t.Fatalf("filter since: %d", len(only))
	}
	st.PruneMetrics(1000 + 5*30)
	if after, _, _, _ := st.Series(0, 30); len(after) != 5 {
		t.Fatalf("prune: %d", len(after))
	}
}

func TestMetricsAPIValidationAndShape(t *testing.T) {
	o := newOpsEnv(t, "echo ok")
	o.app.st.AddSample(o.now.Unix()-60, 10, 20, 30, 1, true, true)
	for _, bad := range []string{"range=2h", "range=", "range=999d", "range=%00", "range=1H", "range=1h%20"} {
		w := o.do("GET", "/api/metrics?"+bad, "", o.cookie, "", nil)
		if bad == "range=" {
			if w.Code != 200 {
				t.Errorf("range kosong harus default 24h: %d", w.Code)
			}
			continue
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s -> %d", bad, w.Code)
		}
	}
	// pasangan query berisi ';' dibuang Go, jadi jatuh ke default 24h dan tidak pernah sampai ke SQL
	if w := o.do("GET", "/api/metrics?range=1h;x", "", o.cookie, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"range":"24h"`) {
		t.Errorf("harus default 24h: %d %s", w.Code, w.Body)
	}
	w := o.do("GET", "/api/metrics?range=1h", "", o.cookie, "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"cpu":[[`) || !strings.Contains(w.Body.String(), `"nets":[]`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
