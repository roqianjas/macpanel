package main

import (
	"os"
	"strings"
	"testing"
)

// Hanya baca: memakai lab ASLI dan config nginx ASLI. Dilewati kecuali PANEL_REAL=1.
func TestRealLabReadOnly(t *testing.T) {
	if os.Getenv("PANEL_REAL") != "1" {
		t.Skip("set PANEL_REAL=1")
	}
	e := newEnv(t)
	c, _, _ := e.login(e.code(0))
	home, _ := os.UserHomeDir()
	e.app.home, e.app.sitesDir = home, home+"/Developer/_ops/web/nginx/sites"
	s := e.do("GET", "/api/sites", "", c, "", nil).Body.String()
	for _, w := range []string{`"name":"home"`, `"name":"nginx"`, `"system":true`} {
		if !strings.Contains(s, w) {
			t.Errorf("situs nyata tanpa %s: %s", w, s)
		}
	}
	d := e.do("GET", "/api/db", "", c, "", nil)
	if d.Code != 200 || !strings.Contains(d.Body.String(), `"pgsql"`) {
		t.Errorf("db nyata: %d %s", d.Code, d.Body)
	}
	t.Log("sites:", s)
	t.Log("db:", d.Body.String())
}

// Mutasi ASLI terukur: buat lalu hapus database uji lewat panel -> lab -> MySQL. Hanya jalan bila PANEL_REAL=2.
func TestRealDBCycle(t *testing.T) {
	if os.Getenv("PANEL_REAL") != "2" {
		t.Skip("set PANEL_REAL=2")
	}
	o := newOpsEnv(t, "x")
	home, _ := os.UserHomeDir()
	o.app.labBin, o.app.home, o.app.sitesDir = "/opt/homebrew/bin/lab", home, home+"/Developer/_ops/web/nginx/sites"
	o.app.timeout = 60e9
	defer o.post("/api/db/mysql/paneltest/drop", `{"confirm":"paneltest"}`) // jaring pengaman bila tes gagal di tengah
	c, b := o.post("/api/db", `{"name":"paneltest","engine":"mysql"}`)
	t.Log("create:", c, b)
	if c != 200 {
		t.Fatal("create gagal")
	}
	if l := o.do("GET", "/api/db", "", o.cookie, "", nil).Body.String(); !strings.Contains(l, `"mysql":["paneltest"]`) {
		t.Fatalf("tidak terdaftar: %s", l)
	}
	if c, _ := o.post("/api/db/mysql/paneltest/drop", `{"confirm":"paneltest"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	c, b = o.post("/api/db/mysql/paneltest/drop", `{"confirm":"paneltest"}`)
	t.Log("drop:", c, b)
	if c != 200 {
		t.Fatal("drop gagal")
	}
	if l := o.do("GET", "/api/db", "", o.cookie, "", nil).Body.String(); strings.Contains(l, "paneltest") {
		t.Fatalf("masih ada setelah drop: %s", l)
	}
}
