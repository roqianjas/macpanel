package system

import (
	"testing"
)

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
