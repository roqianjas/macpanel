package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// opsEnv: env login + `lab` palsu yang mencatat argv dan stdin, HOME dan folder situs sementara.
type opsEnv struct {
	*env
	log    string
	cookie string
	csrf   string
	home   string
}

func newOpsEnv(t *testing.T, fakeBody string) *opsEnv {
	t.Helper()
	e := newEnv(t)
	home, _ := filepath.EvalSymlinks(t.TempDir()) // macOS: /var -> /private/var
	os.MkdirAll(filepath.Join(home, "Developer/proj/public"), 0o755)
	os.MkdirAll(filepath.Join(home, "sites"), 0o755)
	os.MkdirAll(filepath.Join(home, "outside"), 0o755)
	os.Symlink(filepath.Join(home, "outside"), filepath.Join(home, "Developer/escape"))
	log := filepath.Join(home, "calls.log")
	if fakeBody == "" {
		fakeBody = `case "$1 $2" in
"site list") printf 'db.lab\nhome.lab\n';;
"db list") printf 'MySQL:\n  (kosong)\nPostgreSQL:\n  dev\n  proj\n';;
*) echo "ok $1 $2";; esac`
	}
	script := "#!/bin/sh\nprintf 'CALL:' >> " + log + "\nfor a in \"$@\"; do printf '[%s]' \"$a\" >> " + log + "; done\nread -r line; printf ' STDIN=[%s]\\n' \"$line\" >> " + log + "\n" + fakeBody + "\n"
	bin := filepath.Join(home, "lab")
	os.WriteFile(bin, []byte(script), 0o755)
	e.app.labBin, e.app.home, e.app.sitesDir = bin, home, filepath.Join(home, "sites")
	e.app.timeout = 5 * time.Second
	os.WriteFile(filepath.Join(home, "sites/demo.conf"), []byte("server {\n    root "+home+"/Developer/proj/public;\n    fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-8.4.sock;\n}"), 0o644)
	os.WriteFile(filepath.Join(home, "sites/api.conf"), []byte("server { location / { proxy_pass http://127.0.0.1:3000; } }"), 0o644)
	os.WriteFile(filepath.Join(home, "sites/home.conf"), []byte("server {\n    root /x;\n}"), 0o644)
	c, cs, _ := e.login(e.code(0))
	return &opsEnv{env: e, log: log, cookie: c, csrf: cs, home: home}
}

func (o *opsEnv) post(path, body string) (int, string) {
	w := o.do("POST", path, body, o.cookie, o.csrf, nil)
	return w.Code, w.Body.String()
}

func (o *opsEnv) calls() string { b, _ := os.ReadFile(o.log); return string(b) }

func (o *opsEnv) stepup() {
	o.now = o.now.Add(31 * time.Second)
	if w := o.do("POST", "/api/auth/stepup", `{"code":"`+o.code(0)+`"}`, o.cookie, o.csrf, nil); w.Code != 200 {
		o.t.Fatalf("stepup: %d", w.Code)
	}
}

func TestListSitesParsesConfigAndMarksSystem(t *testing.T) {
	o := newOpsEnv(t, "")
	w := o.do("GET", "/api/sites", "", o.cookie, "", nil)
	b := w.Body.String()
	for _, s := range []string{`"name":"demo"`, `"mode":"php"`, `"php":"8.4"`, `"mode":"proxy"`, `"target":"3000"`} {
		if !strings.Contains(b, s) {
			t.Errorf("daftar situs tanpa %s: %s", s, b)
		}
	}
	if !strings.Contains(b, `"name":"home","host":"home.lab","mode":"statis","target":"/x","system":true`) {
		t.Errorf("situs sistem tidak ditandai: %s", b)
	}
}

func TestSiteAddBuildsExactArgv(t *testing.T) {
	o := newOpsEnv(t, "")
	dir := filepath.Join(o.home, "Developer/proj/public")
	if c, b := o.post("/api/sites", `{"name":"toko","target":"`+dir+`","php":"8.4"}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	want := "CALL:[site][add][toko][" + dir + "][--php][8.4]"
	if !strings.Contains(o.calls(), want) {
		t.Fatalf("argv salah.\nada:  %s\nmau:  %s", o.calls(), want)
	}
	if c, _ := o.post("/api/sites", `{"name":"api2","target":"3001"}`); c != 200 || !strings.Contains(o.calls(), "[site][add][api2][3001]") {
		t.Fatal("situs proxy gagal")
	}
	if c, _ := o.post("/api/sites", `{"name":"tilde","target":"~/Developer/proj/public"}`); c != 200 || !strings.Contains(o.calls(), "[tilde]["+dir+"]") {
		t.Fatal("~/ harus diperluas ke path nyata")
	}
}

func TestInjectionAndBadInputNeverReachLab(t *testing.T) {
	o := newOpsEnv(t, "")
	dir := filepath.Join(o.home, "Developer/proj/public")
	bad := []string{
		`{"name":"a;touch x","target":"3001"}`, `{"name":"-rf","target":"3001"}`, `{"name":"A","target":"3001"}`, `{"name":"","target":"3001"}`,
		`{"name":"a$(id)","target":"3001"}`, `{"name":"` + strings.Repeat("a", 60) + `","target":"3001"}`, `{"name":"ok","target":"--help"}`,
		`{"name":"ok","target":"/etc"}`, `{"name":"ok","target":"` + o.home + `/Developer"}`, `{"name":"ok","target":"` + o.home + `/Developer/../outside"}`,
		`{"name":"ok","target":"` + o.home + `/Developer/escape"}`, `{"name":"ok","target":"` + dir + `/../../outside"}`, `{"name":"ok","target":"80"}`,
		`{"name":"ok","target":"22"}`, `{"name":"ok","target":"5432"}`, `{"name":"ok","target":"70000"}`, `{"name":"ok","target":"3001","php":"8.4"}`,
		`{"name":"ok","target":"` + dir + `","php":"8.1"}`, `{"name":"ok","target":"` + dir + `","php":"8.4;id"}`, `{"name":"ok","target":"` + dir + "\\u0000x" + `"}`,
		`{"name":"home","target":"3001"}`, `{"name":"panel","target":"3001"}`, `{"name":"ok","target":"relatif/path"}`,
	}
	for _, b := range bad {
		if c, _ := o.post("/api/sites", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if strings.Contains(o.calls(), "CALL:") {
		t.Fatalf("lab terpanggil oleh input tidak valid: %s", o.calls())
	}
}

func TestProtectedSitesCannotBeRemoved(t *testing.T) {
	o := newOpsEnv(t, "")
	o.stepup()
	for _, n := range []string{"home", "db", "nginx", "kuma", "stats", "panel", "hello"} {
		if c, _ := o.post("/api/sites/"+n+"/remove", `{"confirm":"`+n+`"}`); c != 400 {
			t.Errorf("situs sistem %s bisa dihapus: %d", n, c)
		}
	}
	if o.calls() != "" {
		t.Fatal("lab terpanggil untuk situs yang dilindungi")
	}
}

func TestDestructiveNeedsConfirmAndStepUp(t *testing.T) {
	o := newOpsEnv(t, "")
	if c, _ := o.post("/api/sites/demo/remove", `{"confirm":"demo"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	if c, _ := o.post("/api/db/mysql/proj/drop", `{"confirm":"proj"}`); c != 403 {
		t.Fatalf("drop tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	if c, _ := o.post("/api/db/mysql/proj/drop", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi salah harus 400: %d", c)
	}
	if o.calls() != "" {
		t.Fatal("lab terpanggil sebelum syarat terpenuhi")
	}
	if c, b := o.post("/api/db/mysql/proj/drop", `{"confirm":"proj"}`); c != 200 {
		t.Fatalf("drop sah: %d %s", c, b)
	}
	if !strings.Contains(o.calls(), "CALL:[db][drop][proj][--engine][mysql] STDIN=[proj]") {
		t.Fatalf("argv/stdin drop salah: %s", o.calls())
	}
	// step-up kedaluwarsa
	o.now = o.now.Add(stepupTTL + time.Second)
	if c, _ := o.post("/api/sites/demo/remove", `{"confirm":"demo"}`); c != 403 {
		t.Fatalf("step-up kedaluwarsa harus 403: %d", c)
	}
}

func TestDBCreateValidationAndEnvPath(t *testing.T) {
	o := newOpsEnv(t, "")
	for _, b := range []string{`{"name":"x;y","engine":"mysql"}`, `{"name":"1abc","engine":"mysql"}`, `{"name":"ok","engine":"oracle"}`, `{"name":"ok","engine":"mysql","env":"/etc/passwd"}`,
		`{"name":"ok","engine":"mysql","env":"` + o.home + `/Developer/proj/evil.sh"}`, `{"name":"ok","engine":"mysql","env":"` + o.home + `/outside/.env"}`} {
		if c, _ := o.post("/api/db", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	env := o.home + "/Developer/proj/.env"
	if c, b := o.post("/api/db", `{"name":"toko","engine":"pgsql","env":"`+env+`"}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if !strings.Contains(o.calls(), "CALL:[db][create][toko][--engine][pgsql][--env]["+env+"]") {
		t.Fatalf("argv create: %s", o.calls())
	}
	for _, p := range []string{"/api/db/oracle/x/drop", "/api/db/mysql/a;b/dump", "/api/db/mysql/-x/dump"} {
		if c, _ := o.post(p, `{"confirm":"x"}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
}

func TestDBListParsedAndSystemMarked(t *testing.T) {
	o := newOpsEnv(t, "")
	b := o.do("GET", "/api/db", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, `"mysql":[]`) || !strings.Contains(b, `"pgsql":["dev","proj"]`) || !strings.Contains(b, `"system":["dev"]`) {
		t.Fatalf("daftar db: %s", b)
	}
}

func TestOutputScrubbedAndJobRecorded(t *testing.T) {
	o := newOpsEnv(t, `echo "DB_PASSWORD=SangatRahasia123 token: abc Bearer eyJhbGciOiJIUzI1NiJ9abcdefghij"; echo ok`)
	_, body := o.post("/api/db", `{"name":"toko","engine":"mysql"}`)
	for _, s := range []string{"SangatRahasia123", "eyJhbGci"} {
		if strings.Contains(body, s) {
			t.Fatalf("rahasia bocor di respons: %s", body)
		}
	}
	jobs := o.do("GET", "/api/jobs", "", o.cookie, "", nil).Body.String()
	if strings.Contains(jobs, "SangatRahasia123") || !strings.Contains(jobs, "db_create") && !strings.Contains(jobs, "toko") {
		t.Fatalf("job: %s", jobs)
	}
	aud := o.do("GET", "/api/audit", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(aud, "db_create") || strings.Contains(aud, "SangatRahasia123") {
		t.Fatalf("audit: %s", aud)
	}
}

func TestFailureReportsStatusAndTimeoutKillsProcess(t *testing.T) {
	o := newOpsEnv(t, `echo "boom" >&2; exit 3`)
	c, b := o.post("/api/sites", `{"name":"gagal","target":"3001"}`)
	if c != 422 || !strings.Contains(b, `"status":"gagal"`) || !strings.Contains(b, "boom") {
		t.Fatalf("kegagalan harus 422 + status: %d %s", c, b)
	}
	o2 := newOpsEnv(t, `sleep 30`)
	o2.app.timeout = time.Second
	start := time.Now()
	c, b = o2.post("/api/sites", `{"name":"lambat","target":"3001"}`)
	if c != 422 || !strings.Contains(b, `"status":"timeout"`) || time.Since(start) > 6*time.Second {
		t.Fatalf("timeout tidak ditegakkan: %d %s (%v)", c, b, time.Since(start))
	}
}

func TestOutputCap(t *testing.T) {
	o := newOpsEnv(t, `head -c 600000 /dev/zero | tr '\0' 'a'`)
	r := o.app.runLab([]string{"x", "y"}, "")
	if len(r.Out) > maxOutput+10 {
		t.Fatalf("output tidak dibatasi: %d", len(r.Out))
	}
}

func TestEnvIsCleared(t *testing.T) {
	o := newOpsEnv(t, `echo "SECRET_TEST=${SECRET_TEST:-kosong} HOME=$HOME"`)
	os.Setenv("SECRET_TEST", "bocor")
	defer os.Unsetenv("SECRET_TEST")
	r := o.app.runLab([]string{"x", "y"}, "")
	if strings.Contains(r.Out, "bocor") || !strings.Contains(r.Out, "kosong") {
		t.Fatalf("env induk bocor ke anak: %s", r.Out)
	}
}

func TestOpsRoutesNeedCSRF(t *testing.T) {
	o := newOpsEnv(t, "")
	for _, p := range []string{"/api/sites", "/api/db"} {
		if w := o.do("POST", p, `{"name":"zz","target":"3001","engine":"mysql"}`, o.cookie, "", nil); w.Code != 403 {
			t.Errorf("%s tanpa CSRF: %d", p, w.Code)
		}
	}
	if o.calls() != "" {
		t.Fatal("lab terpanggil tanpa CSRF")
	}
}
