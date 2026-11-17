package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeApps = `case "$1 $2" in
"service list") printf '[{"id":"nginx","label":"nginx","group":"Web server","kind":"brew","version":"1.31.8","state":"running","port":443,"installed":true,"ctl":true,"optional":false,"desc":"x","has_log":true},{"id":"redis","label":"Redis","group":"Cache","kind":"brew","version":"8","state":"HACKED<script>","installed":true,"ctl":true,"optional":false,"desc":"","has_log":true}]';;
"service logs") echo "log token=RAHASIA123";;
*) echo "ok $1 $2";;
esac`

func newAppsEnv(t *testing.T) *opsEnv {
	o := newOpsEnv(t, fakeApps)
	o.app.longTimeout = 20 * time.Second
	return o
}

func TestServicesListSanitizesState(t *testing.T) {
	o := newAppsEnv(t)
	b := o.do("GET", "/api/services", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, `"id":"nginx"`) || !strings.Contains(b, `"state":"running"`) {
		t.Fatalf("%s", b)
	}
	if strings.Contains(b, "HACKED") || !strings.Contains(b, `"state":"error"`) {
		t.Fatalf("state tak dikenal harus jadi 'error': %s", b)
	}
}

func TestServiceActionGuards(t *testing.T) {
	o := newAppsEnv(t)
	before := o.calls()
	for _, p := range []string{"/api/services/panel/stop", "/api/services/dnsmasq/restart", "/api/services/redis/kill", "/api/services/RE/start", "/api/services/-x/start", "/api/services/redis;id/start", "/api/services/a..b/start"} {
		if c, b := o.post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d %s", p, c, b)
		}
	}
	o.stepup()
	for _, p := range []string{"/api/services/panel/stop", "/api/services/dnsmasq/restart", "/api/services/redis/kill", "/api/services/-x/start"} {
		if c, _ := o.post(p, `{}`); c != 400 {
			t.Errorf("(setelah step-up) %s harus 400: %d", p, c)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	o.now = o.now.Add(stepupTTL + time.Second)
	if c, _ := o.post("/api/services/redis/stop", `{}`); c != 403 {
		t.Fatalf("stop tanpa step-up: %d", c)
	}
	if c, _ := o.post("/api/services/redis/restart", `{}`); c != 403 {
		t.Fatalf("restart tanpa step-up: %d", c)
	}
	if c, _ := o.post("/api/services/redis/start", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[service][start][redis]") {
		t.Fatalf("start tidak butuh step-up: %d %s", c, o.calls())
	}
	o.stepup()
	if c, _ := o.post("/api/services/redis/stop", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[service][stop][redis]") {
		t.Fatalf("stop: %d", c)
	}
}

func TestServiceLogsScrubbedAndBounded(t *testing.T) {
	o := newAppsEnv(t)
	b := o.do("GET", "/api/services/redis/logs?n=5", "", o.cookie, "", nil).Body.String()
	if strings.Contains(b, "RAHASIA123") || !strings.Contains(b, "log token") {
		t.Fatalf("%s", b)
	}
	for _, p := range []string{"?n=0", "?n=501", "?n=a", "?n=1;2"} {
		w := o.do("GET", "/api/services/redis/logs"+p, "", o.cookie, "", nil)
		if w.Code != 400 && !(p == "?n=1;2" && w.Code == 200) {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	if w := o.do("GET", "/api/services/Re%20dis/logs", "", o.cookie, "", nil); w.Code != 400 && w.Code != 404 {
		t.Errorf("id buruk -> %d", w.Code)
	}
}

func TestAppInstallRemoveGuards(t *testing.T) {
	o := newAppsEnv(t)
	for _, id := range []string{"nginx", "redis", "mysql", "x;y", "../x", "memcached2"} {
		if c, _ := o.post("/api/apps/"+id+"/install", `{}`); c != 400 && c != 404 && c != 307 {
			t.Errorf("install %s -> %d", id, c)
		}
	}
	o.stepup()
	for _, id := range []string{"nginx", "redis", "postgresql"} {
		if c, _ := o.post("/api/apps/"+id+"/remove", `{"confirm":"`+id+`"}`); c != 400 {
			t.Errorf("remove %s (bukan opsional) harus 400: %d", id, c)
		}
	}
	if strings.Contains(o.calls(), "[app]") {
		t.Fatalf("lab terpanggil: %s", o.calls())
	}
	c, b := o.post("/api/apps/memcached/install", `{}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[app][install][memcached]") {
		t.Fatalf("%s", o.calls())
	}
	if c, _ := o.post("/api/apps/memcached/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	o.now = o.now.Add(stepupTTL + time.Second)
	if c, _ := o.post("/api/apps/memcached/remove", `{"confirm":"memcached"}`); c != 403 {
		t.Fatalf("remove tanpa step-up: %d", c)
	}
	o.stepup()
	c, b = o.post("/api/apps/memcached/remove", `{"confirm":"memcached"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[app][remove][memcached]") {
		t.Fatalf("%s", o.calls())
	}
}

func TestSiteToggleGuards(t *testing.T) {
	o := newAppsEnv(t)
	before := o.calls()
	for _, p := range []string{"/api/sites/home/disable", "/api/sites/panel/disable", "/api/sites/db/enable", "/api/sites/demo/delete", "/api/sites/Demo/disable", "/api/sites/-x/disable", "/api/sites/de;mo/disable"} {
		if c, b := o.post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d %s", p, c, b)
		}
	}
	o.stepup()
	for _, p := range []string{"/api/sites/home/disable", "/api/sites/panel/enable", "/api/sites/demo/delete"} {
		if c, _ := o.post(p, `{}`); c != 400 {
			t.Errorf("(step-up aktif) %s harus 400: %d", p, c)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	o.now = o.now.Add(stepupTTL + time.Second)
	if c, _ := o.post("/api/sites/demo/disable", `{}`); c != 403 {
		t.Fatalf("disable tanpa step-up: %d", c)
	}
	if c, _ := o.post("/api/sites/demo/enable", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[site][enable][demo]") {
		t.Fatalf("enable tidak butuh step-up: %d %s", c, o.calls())
	}
	o.stepup()
	if c, _ := o.post("/api/sites/demo/disable", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[site][disable][demo]") {
		t.Fatalf("disable: %d", c)
	}
}

func TestSiteListRichIncludesDisabledAndSSL(t *testing.T) {
	o := newAppsEnv(t)
	os.WriteFile(filepath.Join(o.app.sitesDir, "tidur.conf.disabled"), []byte("server {\n    root "+o.home+"/Developer/proj/public;\n    fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-8.3.sock;\n}"), 0o644)
	os.WriteFile(filepath.Join(o.app.sitesDir, "x y.conf.disabled"), []byte("server {}"), 0o644) // nama tak sah: diabaikan
	b := o.do("GET", "/api/sites", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, `"name":"tidur"`) || !strings.Contains(b, `"enabled":false`) || !strings.Contains(b, `"php":"8.3"`) {
		t.Fatalf("situs dijeda harus tampil: %s", b)
	}
	if strings.Contains(b, "x y") {
		t.Fatalf("nama tak sah bocor: %s", b)
	}
	if !strings.Contains(b, `"name":"demo"`) || !strings.Contains(b, `"enabled":true`) || !strings.Contains(b, `"ssl_days":null`) {
		t.Fatalf("%s", b)
	}
}

func TestProbeOnlyLoopbackAddr(t *testing.T) {
	o := newAppsEnv(t)
	a := newApp(o.app.st, o.app.key, Config{})
	if a.probeAddr != "127.0.0.1:443" {
		t.Fatalf("alamat pemeriksaan harus loopback: %s", a.probeAddr)
	}
}

// Regresi: id katalog PHP-FPM memuat titik ("php-8.2"); dulu ditolak 400 sehingga mulai ulang dan log FPM tak pernah jalan.
func TestServiceIDsWithVersionDot(t *testing.T) {
	o := newAppsEnv(t)
	for _, id := range []string{"php-8.2", "php-8.4", "php-8.5"} {
		if w := o.do("GET", "/api/services/"+id+"/logs", "", o.cookie, "", nil); w.Code != 200 {
			t.Errorf("log %s -> %d %s", id, w.Code, w.Body.String())
		}
	}
	o.stepup()
	if c, b := o.post("/api/services/php-8.4/restart", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[service][restart][php-8.4]") {
		t.Fatalf("restart php-8.4: %d %s %s", c, b, o.calls())
	}
	before := o.calls()
	for _, id := range []string{"php-8..4", "php-.8", "php-8.", "php-8.123", "php-8.x", ".8", "php-8.4.1", "php-8.4;id", "php%2F8.4"} {
		if c, _ := o.post("/api/services/"+id+"/restart", `{}`); c != 400 && c != 404 {
			t.Errorf("id buruk %q harus ditolak: %d", id, c)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh id buruk: %s", o.calls())
	}
}
