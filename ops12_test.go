package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBackupCleanupEndpoints(t *testing.T) {
	o := newOpsEnv(t, `case "$2" in prune) printf '{"keep":7,"days":0,"remove":[],"kept":1}';; extras) printf '{"count":0,"kb":0}';; *) echo ok;; esac`)
	o.app.longTimeout = 20 * time.Second
	for _, p := range []string{"/api/backups/prune-preview?keep=5&days=30", "/api/backups/extras", "/api/backups/extras-preview?kind=db&days=30"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 200 {
			t.Errorf("%s -> %d %s", p, w.Code, w.Body.String())
		}
	}
	before := o.calls()
	for _, p := range []string{"keep=0", "keep=61", "keep=a", "days=-1", "days=3651", "keep=05", "days=1%3Bid", "keep=1%20--x"} {
		if w := o.do("GET", "/api/backups/prune-preview?"+p, "", o.cookie, "", nil); w.Code != 400 {
			t.Errorf("preview %s -> %d", p, w.Code)
		}
	}
	for _, p := range []string{"kind=x", "kind=db&days=-1", "kind=db&days=x", "kind=retired", "kind=db%3Bid", "kind="} {
		if w := o.do("GET", "/api/backups/extras-preview?"+p, "", o.cookie, "", nil); w.Code != 400 {
			t.Errorf("extras-preview %s -> %d", p, w.Code)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	for _, c := range [][2]string{{"/api/backups/prune", `{"keep":7,"days":0}`}, {"/api/backups/delete", `{"names":["20261117-023004"]}`}, {"/api/backups/extras", `{"kind":"db","days":1}`}} {
		if code, _ := o.post(c[0], c[1]); code != 403 {
			t.Errorf("%s tanpa step-up: %d", c[0], code)
		}
	}
	o.stepup()
	for _, c := range [][2]string{
		{"/api/backups/prune", `{"keep":0,"days":0}`}, {"/api/backups/prune", `{"keep":61,"days":0}`}, {"/api/backups/prune", `{"keep":7,"days":-1}`}, {"/api/backups/prune", `{"keep":7,"days":3651}`}, {"/api/backups/prune", `{"keep":7,"x":1}`},
		{"/api/backups/delete", `{"names":[]}`}, {"/api/backups/delete", `{"names":["../x"]}`}, {"/api/backups/delete", `{"names":["20261117-023004","20261117-023004"]}`}, {"/api/backups/delete", `{"names":["-rf"]}`}, {"/api/backups/delete", `{"names":["20261117-023004;id"]}`},
		{"/api/backups/extras", `{"kind":"retired","days":1}`}, {"/api/backups/extras", `{"kind":"db","days":-1}`}, {"/api/backups/extras", `{"kind":"db","days":3651}`}, {"/api/backups/extras", `{"kind":"../x","days":1}`},
	} {
		if code, r := o.post(c[0], c[1]); code != 400 {
			t.Errorf("%s %s -> %d %s", c[0], c[1], code, r)
		}
	}
	var nm []string
	for i := 0; i < 31; i++ {
		nm = append(nm, fmt.Sprintf(`"202601%02d-000000"`, i+1))
	}
	many := `{"names":[` + strings.Join(nm, ",") + `]}`
	if code, _ := o.post("/api/backups/delete", many); code != 400 {
		t.Errorf("31 nama harus ditolak: %d", code)
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	if code, _ := o.post("/api/backups/prune", `{"keep":5,"days":30}`); code != 200 || !strings.Contains(o.calls(), "CALL:[backup][prune][--keep][5][--days][30]") {
		t.Fatalf("prune: %s", o.calls())
	}
	if code, _ := o.post("/api/backups/delete", `{"names":["20261116-084328","20261116-102324"]}`); code != 200 || !strings.Contains(o.calls(), "CALL:[backup][delete][20261116-084328][20261116-102324]") {
		t.Fatalf("delete: %s", o.calls())
	}
	if code, _ := o.post("/api/backups/extras", `{"kind":"restore","days":0}`); code != 200 || !strings.Contains(o.calls(), "CALL:[backup][extras][restore][--days][0]") {
		t.Fatalf("extras: %s", o.calls())
	}
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	if code, _ := o.post("/api/backups/delete", `{"names":["20261116-084328"]}`); code != 403 {
		t.Errorf("viewer: %d", code)
	}
}
