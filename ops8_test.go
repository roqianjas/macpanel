package main

import (
	"strings"
	"testing"
	"time"
)

const fakeDB8 = `case "$1 $2" in
"db users") printf '{"users":[{"user":"tokoku","host":"127.0.0.1"}],"cred":true,"rotated":1}';;
"db creds") printf '{"DB_HOST":"127.0.0.1","DB_PASSWORD":"s3cr3t-pw","DB_USERNAME":"tokoku"}';;
"backup config") case "$3" in get) printf '{"time":"02:30","keep":7}';; *) echo "OK set";; esac;;
"backup ls") printf '{"prefix":"","entries":[{"name":"Developer","dir":true}]}';;
*) echo "ok $1 $2";;
esac`

func newDB8Env(t *testing.T) *opsEnv {
	o := newOpsEnv(t, fakeDB8)
	o.app.longTimeout = 20 * time.Second
	return o
}

func TestDBUsersPasswdCreds(t *testing.T) {
	o := newDB8Env(t)
	if b := o.do("GET", "/api/db/mysql/tokoku/users", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"user":"tokoku"`) || strings.Contains(b, "s3cr3t") {
		t.Fatalf("%s", b)
	}
	for _, p := range []string{"/api/db/mysql/dev/users", "/api/db/mysql/root/users", "/api/db/oracle/x/users", "/api/db/mysql/a;b/users"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	if c, _ := o.post("/api/db/mysql/tokoku/creds", `{}`); c != 403 {
		t.Fatalf("creds tanpa step-up harus 403: %d", c)
	}
	if c, _ := o.post("/api/db/mysql/tokoku/passwd", `{"confirm":"tokoku"}`); c != 403 {
		t.Fatalf("passwd tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	before := o.calls()
	if c, _ := o.post("/api/db/mysql/tokoku/passwd", `{"confirm":"salah"}`); c != 400 {
		t.Errorf("konfirmasi salah: %d", c)
	}
	for _, p := range []string{"/api/db/mysql/dev/creds", "/api/db/mysql/postgres/passwd", "/api/db/mysql/mysql/creds"} {
		if c, _ := o.post(p, `{"confirm":"x"}`); c != 400 {
			t.Errorf("database sistem %s harus 400: %d", p, c)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil: %s", o.calls())
	}
	if c, b := o.post("/api/db/mysql/tokoku/passwd", `{"confirm":"tokoku"}`); c != 200 || !strings.Contains(o.calls(), "CALL:[db][passwd][tokoku][--engine][mysql]") {
		t.Fatalf("passwd: %d %s %s", c, b, o.calls())
	}
	c, b := o.post("/api/db/mysql/tokoku/creds", `{}`)
	if c != 200 || !strings.Contains(b, "s3cr3t-pw") {
		t.Fatalf("creds: %d %s", c, b)
	}
	if au := o.do("GET", "/api/audit", "", o.cookie, "", nil).Body.String(); !strings.Contains(au, "db_creds_view") || strings.Contains(au, "s3cr3t") {
		t.Errorf("audit harus mencatat lihat kredensial tanpa isi: %s", au)
	}
	// viewer tidak boleh melihat rahasia
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	if c, b := o.post("/api/db/mysql/tokoku/creds", `{}`); c != 403 || strings.Contains(b, "s3cr3t") {
		t.Errorf("viewer: %d %s", c, b)
	}
}

func TestBackupConfigAndExtractGuards(t *testing.T) {
	o := newDB8Env(t)
	if b := o.do("GET", "/api/backups/config", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"keep":7`) {
		t.Fatalf("%s", b)
	}
	if c, _ := o.post("/api/backups/config", `{"time":"03:00","keep":5}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.stepup()
	before := o.calls()
	for _, b := range []string{`{"time":"25:00","keep":5}`, `{"time":"3:00","keep":5}`, `{"time":"03:00","keep":0}`, `{"time":"03:00","keep":61}`, `{"time":"03:00;id","keep":5}`, `{"time":"03:00","keep":5,"x":1}`, `{}`} {
		if c, r := o.post("/api/backups/config", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	for _, b := range []string{
		`{"name":"20261117-023004","part":"developer","path":"../../etc/passwd"}`, `{"name":"20261117-023004","part":"developer","path":"a/../../b"}`, `{"name":"20261117-023004","part":"developer","path":"-C/tmp"}`,
		`{"name":"20261117-023004","part":"developer","path":""}`, `{"name":"20261117-023004","part":"developer","path":"a;id"}`, `{"name":"20261117-023004","part":"developer","path":"a$(id)"}`,
		`{"name":"../x","part":"developer","path":"a"}`, `{"name":"20261117-023004","part":"panel","path":"a"}`, `{"name":"20261117-023004","part":"mysql","path":"a"}`,
	} {
		if c, r := o.post("/api/backups/extract", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	for _, q := range []string{"name=../x&part=developer", "name=20261117-023004&part=panel", "name=20261117-023004&part=developer&prefix=../x", "name=20261117-023004&part=developer&prefix=-x"} {
		if w := o.do("GET", "/api/backups/ls?"+q, "", o.cookie, "", nil); w.Code != 400 {
			t.Errorf("ls %s -> %d", q, w.Code)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	if c, _ := o.post("/api/backups/config", `{"time":"03:45","keep":5}`); c != 200 || !strings.Contains(o.calls(), "CALL:[backup][config][set][--time][03:45][--keep][5]") {
		t.Fatalf("config: %s", o.calls())
	}
	if w := o.do("GET", "/api/backups/ls?name=20261117-023004&part=developer&prefix=Developer/proj", "", o.cookie, "", nil); w.Code != 200 {
		t.Errorf("ls sah: %d", w.Code)
	}
	c, b := o.post("/api/backups/extract", `{"name":"20261117-023004","part":"developer","path":"Developer/proj/src/a.php"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[backup][extract][20261117-023004][developer][Developer/proj/src/a.php]") {
		t.Fatalf("%s", o.calls())
	}
}
