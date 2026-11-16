package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDBDumpsListFiltersToThisDatabase(t *testing.T) {
	o := newAppsEnv(t)
	d := o.app.dumpDir()
	os.MkdirAll(d, 0o755)
	for _, f := range []string{"tokoku-mysql-20261116-100000.sql.gz", "tokoku-mysql-20261117-100000.sql.gz", "tokoku-pgsql-20261116-100000.sql.gz", "tokoku2-mysql-20261116-100000.sql.gz", "xtokoku-mysql-20261116-100000.sql.gz", "tokoku-mysql-bad.sql.gz", "tokoku-mysql-20261116-100000.sql.gz.bak"} {
		os.WriteFile(filepath.Join(d, f), []byte("x"), 0o600)
	}
	os.Symlink("/etc/passwd", filepath.Join(d, "tokoku-mysql-20261118-100000.sql.gz"))
	b := o.do("GET", "/api/db/mysql/tokoku/dumps", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, "tokoku-mysql-20261117-100000.sql.gz") || !strings.Contains(b, "tokoku-mysql-20261116-100000.sql.gz") {
		t.Fatalf("dump milik database ini harus tampil: %s", b)
	}
	for _, bad := range []string{"pgsql", "tokoku2", "xtokoku", "-bad", ".bak", "20261118"} {
		if strings.Contains(b, bad) {
			t.Errorf("%q tidak boleh tampil (database/engine lain, nama rusak, atau symlink): %s", bad, b)
		}
	}
	if strings.Index(b, "20261117") > strings.Index(b, "20261116") {
		t.Errorf("terbaru dulu: %s", b)
	}
	for _, p := range []string{"/api/db/mysql/dev/dumps", "/api/db/oracle/x/dumps", "/api/db/mysql/A/dumps"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 400 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
}

func TestDBRestoreGuards(t *testing.T) {
	o := newAppsEnv(t)
	d := o.app.dumpDir()
	os.MkdirAll(d, 0o755)
	good := "tokoku-mysql-20261116-100000.sql.gz"
	os.WriteFile(filepath.Join(d, good), []byte("x"), 0o600)
	os.Symlink("/etc/passwd", filepath.Join(d, "tokoku-mysql-20261118-100000.sql.gz"))
	before := o.calls()
	body := func(f string) string { return `{"file":"` + f + `"}` }
	if c, _ := o.post("/api/db/mysql/tokoku/restore", body(good)); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	for _, f := range []string{"../db/" + good, "./" + good, good + "/", "../" + good, "/etc/passwd", "tokoku-pgsql-20261116-100000.sql.gz", "lain-mysql-20261116-100000.sql.gz", "tokoku-mysql-bad.sql.gz", good + ".bak", "x/" + good, "", "tokoku-mysql-20261118-100000.sql.gz"} {
		c, b := o.post("/api/db/mysql/tokoku/restore", body(f))
		if c != 400 && c != 404 {
			t.Errorf("file %q -> %d %s", f, c, b)
		}
	}
	for _, p := range []string{"/api/db/mysql/dev/restore", "/api/db/mysql/postgres/restore", "/api/db/oracle/tokoku/restore"} {
		if c, _ := o.post(p, body(good)); c != 400 {
			t.Errorf("%s harus 400: %d", p, c)
		}
	}
	if c, _ := o.post("/api/db/mysql/tokoku/restore", `{"file":"`+good+`","x":1}`); c != 400 {
		t.Errorf("field asing harus 400: %d", c)
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	c, b := o.post("/api/db/mysql/tokoku/restore", body(good))
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[db][restore][tokoku][--engine][mysql][--file]["+good+"]") {
		t.Fatalf("argv: %s", o.calls())
	}
	o.now = o.now.Add(stepupTTL + time.Second)
	if c, _ := o.post("/api/db/mysql/tokoku/restore", body(good)); c != 403 {
		t.Fatalf("step-up kedaluwarsa harus 403: %d", c)
	}
}
