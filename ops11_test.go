package main

import (
	"strings"
	"testing"
	"time"
)

func TestInstallAndCloneGuards(t *testing.T) {
	o := newOpsEnv(t, "")
	o.app.longTimeout = 20 * time.Second
	if c, _ := o.post("/api/installer", `{"kind":"laravel","name":"toko"}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	if c, _ := o.post("/api/sites/demo/clone", `{"dst":"toko2"}`); c != 403 {
		t.Fatalf("clone tanpa step-up: %d", c)
	}
	o.stepup()
	before := o.calls()
	for _, b := range []string{
		`{"kind":"drupal","name":"toko"}`, `{"kind":"laravel","name":"Toko"}`, `{"kind":"laravel","name":"a;id"}`, `{"kind":"laravel","name":"../x"}`, `{"kind":"laravel","name":"panel"}`,
		`{"kind":"laravel","name":"home"}`, `{"kind":"laravel","name":"toko","php":"7.4"}`, `{"kind":"laravel","name":"` + strings.Repeat("a", 31) + `"}`, `{"kind":"laravel","name":"-x"}`, `{"kind":"laravel","name":"toko","x":1}`, `{"name":"toko"}`,
	} {
		if c, r := o.post("/api/installer", b); c != 400 {
			t.Errorf("installer %s -> %d %s", b, c, r)
		}
	}
	for _, b := range []string{`{"dst":"demo"}`, `{"dst":"a;b"}`, `{"dst":"panel"}`, `{"dst":""}`, `{"dst":"../x"}`, `{"dst":"ok","x":1}`} {
		if c, r := o.post("/api/sites/demo/clone", b); c != 400 {
			t.Errorf("clone %s -> %d %s", b, c, r)
		}
	}
	if c, _ := o.post("/api/sites/panel/clone", `{"dst":"ok"}`); c != 400 && c != 404 {
		t.Errorf("situs inti sebagai sumber: %d", c)
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	c, b := o.post("/api/installer", `{"kind":"wordpress","name":"blog","php":"8.3"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[appinstall][wordpress][blog][--php][8.3]") {
		t.Fatalf("%s", o.calls())
	}
	c, b = o.post("/api/sites/demo/clone", `{"dst":"staging","with_db":true}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[siteclone][demo][staging][--with-db]") {
		t.Fatalf("%s", o.calls())
	}
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	if c, _ := o.post("/api/installer", `{"kind":"laravel","name":"toko"}`); c != 403 {
		t.Errorf("viewer: %d", c)
	}
}
