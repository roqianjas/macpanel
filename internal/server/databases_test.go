package server_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/databases"
	"github.com/roqianjas/macpanel/internal/sites"
	"github.com/roqianjas/macpanel/internal/store"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestMariaDBEngine(t *testing.T) {
	got := databases.ParseDBList("MariaDB:\n  shop\nMySQL:\n  (kosong)\nPostgreSQL:\n  dev\n  blog\n")
	if len(got["mariadb"]) != 1 || got["mariadb"][0] != "shop" {
		t.Fatalf("mariadb salah: %v", got)
	}
	if len(got["mysql"]) != 0 || len(got["pgsql"]) != 2 {
		t.Fatalf("mysql/pgsql salah: %v", got)
	}
	for _, e := range []string{"mariadb", "mysql", "pgsql"} {
		if !databases.ValidEngine(e) {
			t.Fatalf("%s harus sah", e)
		}
	}
	for _, e := range []string{"", "redis", "MariaDB", "mariadb;", "../mysql"} {
		if databases.ValidEngine(e) {
			t.Fatalf("%q harus ditolak", e)
		}
	}
	if _, ok := databases.DbcfgKeys["mariadb"]["max_connections"]; !ok {
		t.Fatal("dbcfg mariadb tidak punya kunci")
	}
	if !databases.DbcfgKeys["mariadb"]["max_connections"]("150") || databases.DbcfgKeys["mariadb"]["max_connections"]("5000") {
		t.Fatal("validasi max_connections salah")
	}
	if !sites.ReservedNames["mariadb"] {
		t.Fatal("nama situs 'mariadb' harus dicadangkan")
	}
}

func TestDBDumpsListFiltersToThisDatabase(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	d := databases.DumpDir(o.App)
	os.MkdirAll(d, 0o755)
	for _, f := range []string{"tokoku-mysql-20261116-100000.sql.gz", "tokoku-mysql-20261117-100000.sql.gz", "tokoku-pgsql-20261116-100000.sql.gz", "tokoku2-mysql-20261116-100000.sql.gz", "xtokoku-mysql-20261116-100000.sql.gz", "tokoku-mysql-bad.sql.gz", "tokoku-mysql-20261116-100000.sql.gz.bak"} {
		os.WriteFile(filepath.Join(d, f), []byte("x"), 0o600)
	}
	os.Symlink("/etc/passwd", filepath.Join(d, "tokoku-mysql-20261118-100000.sql.gz"))
	b := o.Do("GET", "/api/db/mysql/tokoku/dumps", "", o.Cookie, "", nil).Body.String()
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
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
}

func TestDBRestoreGuards(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	d := databases.DumpDir(o.App)
	os.MkdirAll(d, 0o755)
	good := "tokoku-mysql-20261116-100000.sql.gz"
	os.WriteFile(filepath.Join(d, good), []byte("x"), 0o600)
	os.Symlink("/etc/passwd", filepath.Join(d, "tokoku-mysql-20261118-100000.sql.gz"))
	before := o.Calls()
	body := func(f string) string { return `{"file":"` + f + `"}` }
	if c, _ := o.Post("/api/db/mysql/tokoku/restore", body(good)); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	for _, f := range []string{"../db/" + good, "./" + good, good + "/", "../" + good, "/etc/passwd", "tokoku-pgsql-20261116-100000.sql.gz", "lain-mysql-20261116-100000.sql.gz", "tokoku-mysql-bad.sql.gz", good + ".bak", "x/" + good, "", "tokoku-mysql-20261118-100000.sql.gz"} {
		c, b := o.Post("/api/db/mysql/tokoku/restore", body(f))
		if c != 400 && c != 404 {
			t.Errorf("file %q -> %d %s", f, c, b)
		}
	}
	for _, p := range []string{"/api/db/mysql/dev/restore", "/api/db/mysql/postgres/restore", "/api/db/oracle/tokoku/restore"} {
		if c, _ := o.Post(p, body(good)); c != 400 {
			t.Errorf("%s harus 400: %d", p, c)
		}
	}
	if c, _ := o.Post("/api/db/mysql/tokoku/restore", `{"file":"`+good+`","x":1}`); c != 400 {
		t.Errorf("field asing harus 400: %d", c)
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	c, b := o.Post("/api/db/mysql/tokoku/restore", body(good))
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[db][restore][tokoku][--engine][mysql][--file]["+good+"]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	if c, _ := o.Post("/api/db/mysql/tokoku/restore", body(good)); c != 403 {
		t.Fatalf("step-up kedaluwarsa harus 403: %d", c)
	}
}

func TestDBUpload(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if w := o.Upload("/api/db/mysql/tokoku/upload", []byte("SELECT 1;")); w.Code != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", w.Code)
	}
	o.Stepup()
	for _, p := range []string{"/api/db/mysql/dev/upload", "/api/db/oracle/tokoku/upload", "/api/db/mysql/A..B/upload", "/api/db/mysql/postgres/upload"} {
		if w := o.Upload(p, []byte("x")); w.Code != 400 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	if w := o.Upload("/api/db/mysql/tokoku/upload", nil); w.Code != 400 {
		t.Errorf("kosong harus 400: %d", w.Code)
	}
	// SQL polos -> digzip; gzip -> disimpan apa adanya
	w := o.Upload("/api/db/mysql/tokoku/upload", []byte("CREATE TABLE t(id INT);\n"))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "tokoku-mysql-") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	files, _ := filepath.Glob(filepath.Join(databases.DumpDir(o.App), "tokoku-mysql-*.sql.gz"))
	if len(files) != 1 {
		t.Fatalf("berkas: %v", files)
	}
	f, _ := os.Open(files[0])
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("hasil harus gzip valid: %v", err)
	}
	if b, _ := io.ReadAll(zr); string(b) != "CREATE TABLE t(id INT);\n" {
		t.Fatalf("isi: %q", b)
	}
	f.Close()
	if fi, _ := os.Stat(files[0]); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode berkas harus 0600: %v", fi.Mode())
	}
	var gzb bytes.Buffer
	zw := gzip.NewWriter(&gzb)
	zw.Write([]byte("SELECT 2;"))
	zw.Close()
	if w := o.Upload("/api/db/pgsql/tokoku/upload", gzb.Bytes()); w.Code != 200 {
		t.Fatalf("gzip: %d %s", w.Code, w.Body.String())
	}
	pg, _ := filepath.Glob(filepath.Join(databases.DumpDir(o.App), "tokoku-pgsql-*.sql.gz"))
	if len(pg) != 1 {
		t.Fatalf("pgsql: %v", pg)
	}
	if b, _ := os.ReadFile(pg[0]); !bytes.Equal(b, gzb.Bytes()) {
		t.Error("gzip harus disimpan apa adanya")
	}
	// unggahan kedua dalam detik yang sama tidak menimpa
	o.Upload("/api/db/mysql/tokoku/upload", []byte("a"))
	o.Upload("/api/db/mysql/tokoku/upload", []byte("b"))
	if files, _ = filepath.Glob(filepath.Join(databases.DumpDir(o.App), "tokoku-mysql-*.sql.gz")); len(files) != 3 {
		t.Errorf("tidak boleh saling menimpa: %v", files)
	}
	if left, _ := filepath.Glob(filepath.Join(databases.DumpDir(o.App), ".upload-*")); len(left) != 0 {
		t.Errorf("berkas sementara tersisa: %v", left)
	}
}

func TestDBUploadTooBig(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	o.Stepup()
	big := bytes.Repeat([]byte("A"), databases.MaxUpload+10)
	if w := o.Upload("/api/db/mysql/tokoku/upload", big); w.Code != 413 {
		t.Fatalf("harus 413: %d", w.Code)
	}
	if files, _ := filepath.Glob(filepath.Join(databases.DumpDir(o.App), "*")); len(files) != 0 {
		t.Errorf("tidak boleh ada berkas tersisa: %v", files)
	}
}
