package server_test

import (
	"testing"

	"github.com/roqianjas/macpanel/internal/phpruntime"
	"github.com/roqianjas/macpanel/internal/tasks"
	"github.com/roqianjas/macpanel/internal/updates"
)

func TestParsersOnRealShapes(t *testing.T) {
	p := phpruntime.ParsePHP("8.2  terpasang=tidak versi=-        fpm_socket=-\n8.3  terpasang=ya    versi=8.3.36   fpm_socket=ada\nsampah\n")
	if len(p) != 2 || p[0].Installed || !p[1].Installed || p[1].Full != "8.3.36" || !p[1].FPM {
		t.Fatalf("%+v", p)
	}
	r := phpruntime.ParseRuntime("go      1.27.4                   ~/.config/mise/config.toml  latest\nnode    22.23.5\npnpm    12.18.0 (pruned in 23h)\nruby 3\n")
	if len(r) != 3 || !r[0].Default || r[1].Default || !r[2].Pruned {
		t.Fatalf("%+v", r)
	}
	u, pins := updates.ParseUpdates("Homebrew:\n  mycli              2.30.4 -> 2.31.0   \n  mysql@8.4          8.4.1 -> 9.0.0   PIN (MAYOR: x)\nmise:\n  pnpm       12.18.0 -> 12.21.0\nPin aktif: mysql@8.4, postgresql@17\nCatatan: a -> b\n")
	if len(u) != 3 || u[0].Source != "homebrew" || !u[1].Pinned || !u[1].Major || u[2].Source != "mise" || len(pins) != 2 {
		t.Fatalf("%+v %v", u, pins)
	}
	if u, pins := updates.ParseUpdates("Homebrew: semua terbaru\nmise: semua terbaru\nPin aktif: -\n"); len(u) != 0 || len(pins) != 0 {
		t.Fatalf("kosong: %+v %v", u, pins)
	}
	tk := tasks.ParseTasks("NAMA   STATUS PID EXIT JADWAL/CMD\nq-1  jalan  12  -  KeepAlive  php a\nq-2  tidak dimuat  -  -  KeepAlive  php a\nj  terjadwal  -  0  tiap 60s  php t\n")
	if len(tk) != 3 || tk[1].Status != "tidak dimuat" || tk[1].PID != "-" || tk[2].Schedule != "tiap 60s php t" {
		t.Fatalf("%+v", tk)
	}
}
