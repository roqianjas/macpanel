package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------- unggah dump ----------

func (o *opsEnv) upload(path string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, bytes.NewReader(body))
	r.RemoteAddr = "100.64.0.20:5555"
	r.Header.Set("Cookie", cookieName+"="+o.cookie)
	r.Header.Set("X-CSRF-Token", o.csrf)
	r.Header.Set("Origin", "https://panel.lab")
	w := httptest.NewRecorder()
	o.h.ServeHTTP(w, r)
	return w
}

func TestDBUpload(t *testing.T) {
	o := newOpsEnv(t, "")
	if w := o.upload("/api/db/mysql/tokoku/upload", []byte("SELECT 1;")); w.Code != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", w.Code)
	}
	o.stepup()
	for _, p := range []string{"/api/db/mysql/dev/upload", "/api/db/oracle/tokoku/upload", "/api/db/mysql/A..B/upload", "/api/db/mysql/postgres/upload"} {
		if w := o.upload(p, []byte("x")); w.Code != 400 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	if w := o.upload("/api/db/mysql/tokoku/upload", nil); w.Code != 400 {
		t.Errorf("kosong harus 400: %d", w.Code)
	}
	// SQL polos -> digzip; gzip -> disimpan apa adanya
	w := o.upload("/api/db/mysql/tokoku/upload", []byte("CREATE TABLE t(id INT);\n"))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "tokoku-mysql-") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	files, _ := filepath.Glob(filepath.Join(o.app.dumpDir(), "tokoku-mysql-*.sql.gz"))
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
	if w := o.upload("/api/db/pgsql/tokoku/upload", gzb.Bytes()); w.Code != 200 {
		t.Fatalf("gzip: %d %s", w.Code, w.Body.String())
	}
	pg, _ := filepath.Glob(filepath.Join(o.app.dumpDir(), "tokoku-pgsql-*.sql.gz"))
	if len(pg) != 1 {
		t.Fatalf("pgsql: %v", pg)
	}
	if b, _ := os.ReadFile(pg[0]); !bytes.Equal(b, gzb.Bytes()) {
		t.Error("gzip harus disimpan apa adanya")
	}
	// unggahan kedua dalam detik yang sama tidak menimpa
	o.upload("/api/db/mysql/tokoku/upload", []byte("a"))
	o.upload("/api/db/mysql/tokoku/upload", []byte("b"))
	if files, _ = filepath.Glob(filepath.Join(o.app.dumpDir(), "tokoku-mysql-*.sql.gz")); len(files) != 3 {
		t.Errorf("tidak boleh saling menimpa: %v", files)
	}
	if left, _ := filepath.Glob(filepath.Join(o.app.dumpDir(), ".upload-*")); len(left) != 0 {
		t.Errorf("berkas sementara tersisa: %v", left)
	}
}

func TestDBUploadTooBig(t *testing.T) {
	o := newOpsEnv(t, "")
	o.stepup()
	big := bytes.Repeat([]byte("A"), maxUpload+10)
	if w := o.upload("/api/db/mysql/tokoku/upload", big); w.Code != 413 {
		t.Fatalf("harus 413: %d", w.Code)
	}
	if files, _ := filepath.Glob(filepath.Join(o.app.dumpDir(), "*")); len(files) != 0 {
		t.Errorf("tidak boleh ada berkas tersisa: %v", files)
	}
}

// ---------- peringatan ----------

func ip(n int) *int { return &n }

func TestComputeAlerts(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ok := &SystemInfo{DiskTotal: 1000, DiskFree: 500, Mem: 60, Backup: &BackupInfo{At: now.Unix() - 3600}}
	svc := []ServiceInfo{{ID: "nginx", Label: "nginx", Installed: true, State: "running"}}
	site := []SiteRow{{SiteInfo: SiteInfo{Name: "a", Host: "a.lab"}, Enabled: true, HTTP: 200, SSLDays: ip(300)}}
	if a := computeAlerts(ok, svc, site, now, true); len(a) != 0 {
		t.Fatalf("keadaan sehat tidak boleh ada peringatan: %+v", a)
	}
	has := func(al []Alert, id, lvl string) bool {
		for _, x := range al {
			if x.ID == id && x.Level == lvl {
				return true
			}
		}
		return false
	}
	d := func(free uint64) []Alert {
		s := *ok
		s.DiskFree = free
		return computeAlerts(&s, svc, site, now, true)
	}
	if has(d(160), "disk", "warn") || has(d(160), "disk", "bad") {
		t.Error("84% tidak boleh memicu")
	}
	if !has(d(150), "disk", "warn") || !has(d(81), "disk", "warn") {
		t.Error("85-91% harus warn")
	}
	if !has(d(80), "disk", "bad") {
		t.Error("92% harus bad")
	}
	m := *ok
	m.Mem = 94
	if has(computeAlerts(&m, svc, site, now, true), "mem", "warn") {
		t.Error("94% tidak boleh memicu")
	}
	m.Mem = 95
	if !has(computeAlerts(&m, svc, site, now, true), "mem", "warn") {
		t.Error("95% harus warn")
	}
	down := []ServiceInfo{{ID: "redis", Label: "Redis", Installed: true, State: "stopped"}, {ID: "memcached", Installed: true, Optional: true, State: "stopped"}, {ID: "mailpit", Installed: false, Optional: true, State: "not_installed"}, {ID: "nginx2", Label: "nginx2", Installed: false, Optional: false, State: "not_installed"}}
	a := computeAlerts(ok, down, site, now, true)
	if !has(a, "svc-redis", "bad") || has(a, "svc-memcached", "bad") || has(a, "svc-mailpit", "bad") || has(a, "svc-nginx2", "bad") {
		t.Errorf("hanya layanan wajib yang mati yang memicu: %+v", a)
	}
	mk := func(http int, ssl *int, en bool) []SiteRow {
		return []SiteRow{{SiteInfo: SiteInfo{Name: "x", Host: "x.lab"}, Enabled: en, HTTP: http, SSLDays: ssl}}
	}
	if !has(computeAlerts(ok, svc, mk(0, nil, true), now, true), "site-x", "bad") || has(computeAlerts(ok, svc, mk(0, nil, true), now, false), "site-x", "bad") {
		t.Error("HTTP 0 hanya bila pemeriksaan aktif")
	}
	if !has(computeAlerts(ok, svc, mk(502, nil, true), now, true), "site-x", "bad") || has(computeAlerts(ok, svc, mk(404, nil, true), now, true), "site-x", "bad") {
		t.Error("5xx memicu, 404 tidak")
	}
	if has(computeAlerts(ok, svc, mk(502, nil, false), now, true), "site-x", "bad") {
		t.Error("situs dijeda tidak boleh memicu")
	}
	if !has(computeAlerts(ok, svc, mk(200, ip(13), true), now, true), "ssl-x", "warn") || has(computeAlerts(ok, svc, mk(200, ip(14), true), now, true), "ssl-x", "warn") {
		t.Error("SSL <14 hari warn, 14 tidak")
	}
	if !has(computeAlerts(ok, svc, mk(200, ip(0), true), now, true), "ssl-x", "bad") {
		t.Error("SSL 0 hari bad")
	}
	b := *ok
	b.Backup = &BackupInfo{At: now.Unix() - 36*3600}
	if has(computeAlerts(&b, svc, site, now, true), "backup", "warn") {
		t.Error("36 jam pas tidak memicu")
	}
	b.Backup = &BackupInfo{At: now.Unix() - 37*3600}
	if !has(computeAlerts(&b, svc, site, now, true), "backup", "warn") {
		t.Error("37 jam harus warn")
	}
	b.Backup = nil
	if !has(computeAlerts(&b, svc, site, now, true), "backup", "warn") {
		t.Error("tanpa backup harus warn")
	}
}

// ---------- peran ----------

func TestViewerIsReadOnly(t *testing.T) {
	o := newOpsEnv(t, fakeDev)
	if w := o.do("GET", "/api/users", "", o.cookie, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"role":"admin"`) {
		t.Fatalf("admin harus bisa melihat pengguna: %d %s", w.Code, w.Body.String())
	}
	if b := o.do("GET", "/api/me", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"role":"admin"`) {
		t.Fatalf("me: %s", b)
	}
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	if b := o.do("GET", "/api/me", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"role":"viewer"`) {
		t.Fatalf("me: %s", b)
	}
	if w := o.do("GET", "/api/sites", "", o.cookie, "", nil); w.Code != 200 {
		t.Errorf("viewer boleh membaca: %d", w.Code)
	}
	if w := o.do("GET", "/api/users", "", o.cookie, "", nil); w.Code != 403 {
		t.Errorf("viewer tidak boleh melihat daftar pengguna: %d", w.Code)
	}
	before := o.calls()
	for _, p := range []string{"/api/services/redis/start", "/api/sites/demo/disable", "/api/backups/run", "/api/composer/update", "/api/php/8.4/ini", "/api/db", "/api/php/8.4/ext/redis/install"} {
		if c, b := o.post(p, `{}`); c != 403 || !strings.Contains(b, "hanya-baca") {
			t.Errorf("viewer POST %s -> %d %s", p, c, b)
		}
	}
	if w := o.upload("/api/db/mysql/tokoku/upload", []byte("x")); w.Code != 403 {
		t.Errorf("viewer upload: %d", w.Code)
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh viewer: %s", o.calls())
	}
	if c, _ := o.post("/api/auth/logout", `{}`); c != 200 {
		t.Errorf("viewer harus bisa keluar: %d", c)
	}
}

func TestSetRoleRevokesSessions(t *testing.T) {
	o := newOpsEnv(t, "")
	if o.app.st.SetRole("tidak-ada", "viewer") {
		t.Error("user tidak ada harus false")
	}
	name := o.app.st.UserRows()[0].Name
	for _, bad := range []string{"root", "", "ADMIN", "admin;x"} {
		if o.app.st.SetRole(name, bad) {
			t.Errorf("peran %q harus ditolak", bad)
		}
	}
	if w := o.do("GET", "/api/me", "", o.cookie, "", nil); w.Code != 200 {
		t.Fatalf("sesi harus masih hidup: %d", w.Code)
	}
	if !o.app.st.SetRole(name, "viewer") {
		t.Fatal("SetRole gagal")
	}
	if w := o.do("GET", "/api/me", "", o.cookie, "", nil); w.Code != 401 {
		t.Errorf("sesi harus dicabut setelah ganti peran: %d", w.Code)
	}
}

func TestEveryNewRouteRequiresAuthAndAlertsWork(t *testing.T) {
	o := newOpsEnv(t, `case "$1 $2" in "service list") printf '[{"id":"redis","label":"Redis","state":"stopped","installed":true,"ctl":true,"optional":false}]';; *) echo ok;; esac`)
	for _, p := range []string{"/api/alerts", "/api/users"} {
		if w := o.do("GET", p, "", "", "", nil); w.Code != 401 {
			t.Errorf("%s tanpa sesi: %d", p, w.Code)
		}
	}
	if w := o.do("POST", "/api/db/mysql/tokoku/upload", "x", "", "", nil); w.Code != 401 && w.Code != 403 {
		t.Errorf("upload tanpa sesi: %d", w.Code)
	}
	b := o.do("GET", "/api/alerts", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, `"svc-redis"`) || !strings.Contains(b, `"bad"`) || !strings.Contains(b, "tidak ada pesan keluar") {
		t.Fatalf("%s", b)
	}
}

// Regresi: akun hanya-baca harus bisa menyelesaikan login (langkah TOTP adalah POST dengan sesi tahap "pwd").
func TestViewerCanLogin(t *testing.T) {
	e := newEnv(t)
	e.app.st.db.Exec("UPDATE users SET role='viewer'")
	c, _, w := e.login(e.code(0))
	if c == "" || w.Code != 200 {
		t.Fatalf("viewer gagal login: cookie=%q status=%d %s", c, w.Code, w.Body.String())
	}
	if w := e.do("GET", "/api/me", "", c, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"role":"viewer"`) {
		t.Fatalf("me: %d %s", w.Code, w.Body.String())
	}
}
