package main

import (
	"strings"
	"testing"
	"time"
)

const fakeSys = `case "$1 $2" in
"sys procs") printf '{"procs":[{"pid":123,"name":"node","cpu":1.5,"killable":true}],"total":1}';;
"sys security") printf '{"firewall":"on","ports":[]}';;
"sys stats") printf '{"exists":true,"requests":3}';;
*) echo "ok $1 $2";;
esac`

func TestSysEndpoints(t *testing.T) {
	o := newOpsEnv(t, fakeSys)
	o.app.longTimeout = 20 * time.Second
	for _, p := range []string{"/api/sys/procs", "/api/sys/security", "/api/sites/demo/stats"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 200 {
			t.Errorf("%s -> %d %s", p, w.Code, w.Body.String())
		}
	}
	if w := o.do("GET", "/api/sites/a;b/stats", "", o.cookie, "", nil); w.Code != 400 && w.Code != 404 {
		t.Errorf("nama situs buruk: %d", w.Code)
	}
	if c, _ := o.post("/api/sys/kill", `{"pid":123}`); c != 403 {
		t.Fatalf("kill tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	before := o.calls()
	for _, b := range []string{`{"pid":0}`, `{"pid":1}`, `{"pid":-5}`, `{"pid":99999999}`, `{"pid":"12;id"}`, `{"pid":12,"x":1}`, `{}`} {
		if c, r := o.post("/api/sys/kill", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil: %s", o.calls())
	}
	if c, _ := o.post("/api/sys/kill", `{"pid":123}`); c != 200 || !strings.Contains(o.calls(), "CALL:[sys][kill][123]") {
		t.Fatalf("kill sah: %s", o.calls())
	}
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	if c, _ := o.post("/api/sys/kill", `{"pid":123}`); c != 403 {
		t.Errorf("viewer tak boleh kill: %d", c)
	}
}
