package main

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Hanya baca: memakai lab ASLI dan config nginx ASLI. Dilewati kecuali PANEL_REAL=1.
func TestRealLabReadOnly(t *testing.T) {
	if os.Getenv("PANEL_REAL") != "1" {
		t.Skip("set PANEL_REAL=1")
	}
	e := newEnv(t)
	c, _, _ := e.login(e.code(0))
	home, _ := os.UserHomeDir()
	e.app.home, e.app.sitesDir = home, home+"/MacPanel/server/nginx/sites"
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
	o.app.labBin, o.app.home, o.app.sitesDir = "/opt/homebrew/bin/lab", home, home+"/MacPanel/server/nginx/sites"
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

// ---------- P2b: nginx, lab, dan sertifikat ASLI pada situs sementara (PANEL_REAL=3) ----------

func realGet(t *testing.T, dialIP, host, path string) (int, string, string) {
	t.Helper()
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: host},
		DialContext: func(ctx context.Context, n, a string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, "tcp", dialIP+":443")
		}}
	c := &http.Client{Transport: tr, Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest("GET", "https://"+host+path, nil)
	resp, err := c.Do(req)
	if err != nil {
		return -1, "", err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header.Get("Location")
}

func TestRealSiteLifecycle(t *testing.T) {
	if os.Getenv("PANEL_REAL") != "3" {
		t.Skip("set PANEL_REAL=3")
	}
	o := newOpsEnv(t, "x")
	home, _ := os.UserHomeDir()
	a := o.app
	a.labBin, a.home, a.sitesDir = "/opt/homebrew/bin/lab", home, home+"/MacPanel/server/nginx/sites"
	a.nginxDir, a.certDir, a.logDir, a.errLog = home+"/MacPanel/server/nginx", "/opt/homebrew/etc/nginx/certs", "/opt/homebrew/var/log/nginx/sites", "/opt/homebrew/var/log/nginx/error.log"
	a.timeout = 90 * time.Second
	dir := home + "/MacPanel/wwwroot/.paneltest/public"
	os.MkdirAll(dir, 0o755)
	os.WriteFile(dir+"/index.html", []byte("PANELTEST-INDEX"), 0o644)
	os.WriteFile(dir+"/.env", []byte("RAHASIA"), 0o644)
	cleanup := func() {
		exec.Command("/opt/homebrew/bin/lab", "site", "remove", "paneltest").Run()
		os.RemoveAll(home + "/MacPanel/wwwroot/.paneltest")
		os.Remove(a.logDir + "/paneltest.log")
	}
	cleanup2 := cleanup
	defer cleanup2()
	step := func(n string, ok bool, extra ...any) {
		t.Helper()
		if !ok {
			cf, _ := os.ReadFile(home + "/MacPanel/server/nginx/sites/paneltest.conf")
			el, _ := exec.Command("tail", "-n", "4", a.errLog).Output()
			t.Logf("KONFIG:\n%s\nERROR LOG:\n%s", cf, el)
			t.Fatalf("LANGKAH GAGAL: %s %v", n, extra)
		}
		t.Log("LULUS:", n)
	}
	lan, _ := exec.Command("ipconfig", "getifaddr", "en1").Output()
	lanIP := strings.TrimSpace(string(lan))

	c, b := o.post("/api/sites", `{"name":"paneltest","target":"`+dir+`"}`)
	step("buat situs lewat panel -> lab -> addsite", c == 200, c, b)
	g := o.do("GET", "/api/sites/paneltest/settings", "", o.cookie, "", nil).Body.String()
	step("pengaturan terbaca sebagai legacy/statis", strings.Contains(g, `"kind":"legacy"`) && strings.Contains(g, `"mode":"statis"`), g)

	c, b = o.post("/api/sites/paneltest/settings", `{"mode":"statis","target":"`+dir+`","preset":"spa","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":true,"rewrites":[{"from":"^/old$","to":"/new","flag":"permanent"}]}`)
	step("simpan pengaturan (SPA + rewrite) -> nginx -t -> reload NYATA", c == 200, c, b)
	var code int
	var body string
	var waited time.Duration
	for start := time.Now(); ; {
		code, body, _ = realGet(t, "127.0.0.1", "paneltest.lab", "/dalam/rute/spa")
		waited = time.Since(start)
		if code == 200 || waited > 5*time.Second {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("config baru aktif setelah %v (reload graceful)", waited)
	step("SPA fallback bekerja di nginx nyata", code == 200 && strings.Contains(body, "PANELTEST-INDEX"), code, body)
	var loc string
	code, _, loc = realGet(t, "127.0.0.1", "paneltest.lab", "/old")
	step("rewrite permanent bekerja (301 -> /new)", code == 301 && strings.HasSuffix(loc, "/new"), code, loc)
	code, body, _ = realGet(t, "127.0.0.1", "paneltest.lab", "/.env")
	step("dotfile /.env diblokir (404) dan isinya tidak bocor", code == 404 && !strings.Contains(body, "RAHASIA"), code)
	if lanIP != "" {
		code, _, _ = realGet(t, lanIP, "paneltest.lab", "/")
		step("dari IP LAN tetap 403 (tailnet_only dipertahankan template)", code == 403, code)
	}

	o.stepup()
	cur := o.siteConf("paneltest")
	_ = cur
	path := home + "/MacPanel/server/nginx/sites/paneltest.conf"
	read := func() string { b, _ := os.ReadFile(path); return string(b) }
	before := read()
	bad := strings.Replace(before, "    client_max_body_size 10m;", "    client_max_body_size 5x;", 1)
	step("fixture edit rusak siap", bad != before)
	c, b = o.post("/api/sites/paneltest/config", `{"sha":"`+sha256hex(before)+`","content":`+jsonQuote(bad)+`}`)
	step("edit yang lolos kebijakan tetapi ditolak nginx -t => 422 + rollback", c == 422 && strings.Contains(b, "config lama dipulihkan"), c, b)
	step("file config IDENTIK byte-per-byte setelah rollback", read() == before)
	code, body, _ = realGet(t, "127.0.0.1", "paneltest.lab", "/x")
	step("situs tetap melayani setelah rollback (nginx tidak rusak)", code == 200 && strings.Contains(body, "PANELTEST-INDEX"), code)

	evil := strings.Replace(before, "    client_max_body_size 10m;", "    location /boc { allow all; }", 1)
	c, b = o.post("/api/sites/paneltest/config", `{"sha":"`+sha256hex(before)+`","content":`+jsonQuote(evil)+`}`)
	step("edit 'allow all' ditolak kebijakan (400), nginx tidak disentuh", c == 400 && read() == before, c, b)

	good := strings.Replace(before, "    client_max_body_size 10m;", "    client_max_body_size 20m;\n    add_header X-Panel-Test ok always;", 1)
	c, b = o.post("/api/sites/paneltest/config", `{"sha":"`+sha256hex(before)+`","content":`+jsonQuote(good)+`}`)
	step("edit config sah diterapkan (200)", c == 200 && read() == good, c, b)

	vs := a.st.Versions("paneltest")
	var firstID int64
	for _, v := range vs {
		if cc, _ := a.st.VersionContent("paneltest", v.ID); cc == before {
			firstID = v.ID
		}
	}
	step("versi sebelum edit tersimpan", firstID != 0, vs)
	o.now = o.now.Add(stepupTTL + time.Second)
	o.stepup()
	c, b = o.post("/api/sites/paneltest/restore", `{"id":`+itoa(firstID)+`}`)
	step("pulihkan versi lama (step-up) => file kembali", c == 200 && read() == before, c, b)

	s := o.do("GET", "/api/sites/paneltest/ssl", "", o.cookie, "", nil).Body.String()
	step("info SSL nyata (mkcert, masih berlaku)", strings.Contains(s, `"exists":true`) && strings.Contains(s, `"expired":false`), s)
	o.now = o.now.Add(stepupTTL + time.Second)
	o.stepup()
	c, b = o.post("/api/sites/paneltest/ssl/renew", `{}`)
	step("perbarui sertifikat (lab -> mkcert -> reload)", c == 200, c, b)
	code, _, _ = realGet(t, "127.0.0.1", "paneltest.lab", "/")
	step("situs tetap 200 dengan sertifikat baru", code == 200, code)

	lg := o.do("GET", "/api/sites/paneltest/logs?kind=access&n=50", "", o.cookie, "", nil).Body.String()
	step("log akses per situs berisi permintaan tadi", strings.Contains(lg, "/dalam/rute/spa"), lg)

	o.now = o.now.Add(stepupTTL + time.Second)
	o.stepup()
	c, b = o.post("/api/sites/paneltest/remove", `{"confirm":"paneltest"}`)
	step("hapus situs lewat panel (step-up + konfirmasi) => lab", c == 200, c, b)
	_, err := os.Stat(path)
	step("file config terhapus", err != nil)
	for start := time.Now(); time.Since(start) < 5*time.Second; time.Sleep(50 * time.Millisecond) {
		if code, _, _ = realGet(t, "127.0.0.1", "paneltest.lab", "/"); code != 200 {
			break
		}
	}
	step("situs tidak lagi dilayani (setelah reload graceful)", code != 200, code)
}
