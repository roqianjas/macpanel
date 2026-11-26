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
	c, err := updates.ParseCheck(`{"pins":["mysql@8.4","a b"],"errors":["panel"],"items":[
{"kind":"service","source":"homebrew","name":"nginx","from":"1.1","to":"1.2","pinned":false,"major":false,"upgradable":true,"target":"nginx","note":"restart"},
{"kind":"service","source":"homebrew","name":"mysql@8.4","from":"8.4.1","to":"9.0.0","pinned":true,"major":true,"upgradable":false,"target":"mysql@8.4","note":"major"},
{"kind":"runtime","source":"mise","name":"pnpm","from":"12.18.0","to":"12.21.0","pinned":false,"major":false,"upgradable":true,"target":"mise:pnpm","note":""},
{"kind":"app","source":"npm","name":"9router","from":"0.5.119","to":"0.5.123","pinned":false,"major":false,"upgradable":true,"target":"npm:9router","note":"restart"},
{"kind":"hacker","source":"homebrew","name":"x","from":"1","to":"2","target":"x"},
{"kind":"tool","source":"curl","name":"x","from":"1","to":"2","target":"x"},
{"kind":"tool","source":"homebrew","name":"x","from":"1","to":"2","target":"x;rm -rf /"},
{"kind":"tool","source":"homebrew","name":"x","from":"1 2","to":"2","target":"x"},
{"kind":"tool","source":"homebrew","name":"","from":"1","to":"2","target":"x"}]}`)
	if err != nil || len(c.Items) != 4 || c.Items[0].Kind != "service" || !c.Items[1].Pinned || !c.Items[1].Major || c.Items[2].Target != "mise:pnpm" || c.Items[3].Target != "npm:9router" {
		t.Fatalf("%+v %v", c, err)
	}
	if len(c.Pins) != 1 || c.Pins[0] != "mysql@8.4" || len(c.Errors) != 1 {
		t.Fatalf("pins/errors: %+v", c)
	}
	if c, err := updates.ParseCheck(`{"items":[],"pins":[]}`); err != nil || len(c.Items) != 0 || c.Items == nil || c.Pins == nil || c.Errors == nil {
		t.Fatalf("kosong harus slice kosong, bukan nil: %+v %v", c, err)
	}
	if _, err := updates.ParseCheck("Homebrew: semua terbaru"); err == nil {
		t.Fatal("bukan JSON harus galat")
	}
	tk := tasks.ParseTasks("NAMA   STATUS PID EXIT JADWAL/CMD\nq-1  jalan  12  -  KeepAlive  php a\nq-2  tidak dimuat  -  -  KeepAlive  php a\nj  terjadwal  -  0  tiap 60s  php t\n")
	if len(tk) != 3 || tk[1].Status != "tidak dimuat" || tk[1].PID != "-" || tk[2].Schedule != "tiap 60s php t" {
		t.Fatalf("%+v", tk)
	}
}
