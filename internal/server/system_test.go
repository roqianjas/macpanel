package server_test

import (
	"testing"

	"github.com/roqianjas/macpanel/internal/system"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestParseNetstatRealShape(t *testing.T) {
	m := system.ParseNetstat(testenv.NetstatFixture)
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
	if got := system.IfaceLabel("utun4", "100.64.0.10"); got != "Tailscale (utun4)" {
		t.Fatal(got)
	}
	if got := system.IfaceLabel("en1", "192.168.1.20"); got != "LAN (en1)" {
		t.Fatal(got)
	}
	if len(system.ParseNetstat("sampah\n\n")) != 0 {
		t.Fatal("masukan sampah harus kosong")
	}
}
