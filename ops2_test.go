package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeP3 = `D=$(dirname "$0")
case "$1 $2" in
"php list") printf '8.2  terpasang=tidak versi=-        fpm_socket=-\n8.3  terpasang=ya    versi=8.3.36   fpm_socket=ada\n8.5  terpasang=ya    versi=8.5.12   fpm_socket=ada\n';;
"runtime list") printf 'go      1.27.4                   ~/.config/mise/config.toml  latest\nnode    22.23.5\nnode    26.13.1                  ~/.config/mise/config.toml  lts\npnpm    12.18.0 (pruned in 23h)\n';;
"update check") printf 'Homebrew:\n  mycli              2.30.4 -> 2.31.0   \n  postgresql@17      17.1 -> 17.2   PIN \n  mysql@8.4          8.4.1 -> 9.0.0   PIN (MAYOR: butuh dump/restore)\nmise:\n  pnpm       12.18.0 -> 12.21.0\nPin aktif: mysql@8.4, postgresql@17\n';;
"worker list") printf 'NAMA                    STATUS        PID     EXIT  JADWAL/CMD\nqueue-1                 jalan         79674   -     KeepAlive  /opt/homebrew/bin/php artisan queue:work\nqueue-2                 tidak dimuat  -       -     KeepAlive  /opt/homebrew/bin/php artisan queue:work\n';;
"job list") printf 'NAMA                    STATUS        PID     EXIT  JADWAL/CMD\ntick                    terjadwal     -       0     tiap 60s  /opt/homebrew/bin/php tick.php\nharian                  terjadwal     -       -     harian 03:15  /opt/homebrew/bin/php x.php\n';;
"php install") [ -f "$D/fail" ] && { echo "boom"; exit 2; }; [ -f "$D/slow" ] && sleep 30; printf 'START\n' >> "$D/order.log"; sleep 0.3; printf 'END\n' >> "$D/order.log"; echo "ok php install";;
"worker logs"|"job logs") echo "baris log token=RAHASIA123";;
*) [ -f "$D/slow" ] && sleep 30; [ -f "$D/fail" ] && { echo "boom"; exit 2; }; echo "ok $1 $2";;
esac`

func newP3Env(t *testing.T) *opsEnv {
	t.Helper()
	o := newOpsEnv(t, fakeP3)
	o.app.longTimeout = 20 * time.Second
	return o
}

func (o *opsEnv) waitJob(id int64) JobRow {
	o.t.Helper()
	for i := 0; i < 200; i++ {
		if j := o.app.st.GetJob(id); j != nil && j.Status != "berjalan" {
			return *j
		}
		time.Sleep(50 * time.Millisecond)
	}
	o.t.Fatalf("job %d tidak selesai", id)
	return JobRow{}
}

func jobID(t *testing.T, body string) int64 {
	t.Helper()
	var m struct{ Job int64 }
	if err := json.Unmarshal([]byte(body), &m); err != nil || m.Job == 0 {
		t.Fatalf("tidak ada id job: %s", body)
	}
	return m.Job
}

// ---------- parser ----------

func TestParsersOnRealShapes(t *testing.T) {
	p := parsePHP("8.2  terpasang=tidak versi=-        fpm_socket=-\n8.3  terpasang=ya    versi=8.3.36   fpm_socket=ada\nsampah\n")
	if len(p) != 2 || p[0].Installed || !p[1].Installed || p[1].Full != "8.3.36" || !p[1].FPM {
		t.Fatalf("%+v", p)
	}
	r := parseRuntime("go      1.27.4                   ~/.config/mise/config.toml  latest\nnode    22.23.5\npnpm    12.18.0 (pruned in 23h)\nruby 3\n")
	if len(r) != 3 || !r[0].Default || r[1].Default || !r[2].Pruned {
		t.Fatalf("%+v", r)
	}
	u, pins := parseUpdates("Homebrew:\n  mycli              2.30.4 -> 2.31.0   \n  mysql@8.4          8.4.1 -> 9.0.0   PIN (MAYOR: x)\nmise:\n  pnpm       12.18.0 -> 12.21.0\nPin aktif: mysql@8.4, postgresql@17\nCatatan: a -> b\n")
	if len(u) != 3 || u[0].Source != "homebrew" || !u[1].Pinned || !u[1].Major || u[2].Source != "mise" || len(pins) != 2 {
		t.Fatalf("%+v %v", u, pins)
	}
	if u, pins := parseUpdates("Homebrew: semua terbaru\nmise: semua terbaru\nPin aktif: -\n"); len(u) != 0 || len(pins) != 0 {
		t.Fatalf("kosong: %+v %v", u, pins)
	}
	tk := parseTasks("NAMA   STATUS PID EXIT JADWAL/CMD\nq-1  jalan  12  -  KeepAlive  php a\nq-2  tidak dimuat  -  -  KeepAlive  php a\nj  terjadwal  -  0  tiap 60s  php t\n")
	if len(tk) != 3 || tk[1].Status != "tidak dimuat" || tk[1].PID != "-" || tk[2].Schedule != "tiap 60s php t" {
		t.Fatalf("%+v", tk)
	}
}

// ---------- job asinkron ----------

func TestAsyncJobLifecycleAndStaleMarking(t *testing.T) {
	o := newP3Env(t)
	c, b := o.post("/api/php/8.4/install", `{}`)
	if c != 202 {
		t.Fatalf("harus 202: %d %s", c, b)
	}
	id := jobID(t, b)
	j := o.waitJob(id)
	if j.Status != "ok" || !strings.Contains(o.calls(), "CALL:[php][install][8.4]") {
		t.Fatalf("job: %+v calls=%s", j, o.calls())
	}
	g := o.do("GET", "/api/jobs/"+itoa(id), "", o.cookie, "", nil)
	if g.Code != 200 || !strings.Contains(g.Body.String(), `"status":"ok"`) {
		t.Fatalf("GET job: %d %s", g.Code, g.Body)
	}
	if w := o.do("GET", "/api/jobs/99999", "", o.cookie, "", nil); w.Code != 404 {
		t.Fatalf("job tak ada: %d", w.Code)
	}
	for _, p := range []string{"abc", "-1", "0", "1;2"} {
		if w := o.do("GET", "/api/jobs/"+p, "", o.cookie, "", nil); w.Code != 400 {
			t.Errorf("/api/jobs/%s -> %d", p, w.Code)
		}
	}
	o.app.st.AddJob("x", "yatim", "berjalan", "", 0, o.now)
	o.app.st.MarkStaleJobs()
	var st string
	o.app.st.db.QueryRow("SELECT status FROM jobs WHERE action='yatim'").Scan(&st)
	if st != "terputus" {
		t.Fatalf("job yatim harus 'terputus': %s", st)
	}
}

func TestAsyncJobFailureAndTimeout(t *testing.T) {
	o := newP3Env(t)
	os.WriteFile(filepath.Join(o.home, "fail"), nil, 0o644)
	_, b := o.post("/api/php/8.3/install", `{}`)
	if j := o.waitJob(jobID(t, b)); j.Status != "gagal" || j.Code != 2 || !strings.Contains(j.Output, "boom") {
		t.Fatalf("%+v", j)
	}
	os.Remove(filepath.Join(o.home, "fail"))
	os.WriteFile(filepath.Join(o.home, "slow"), nil, 0o644)
	o.app.longTimeout = time.Second
	start := time.Now()
	_, b = o.post("/api/php/8.3/install", `{}`)
	if j := o.waitJob(jobID(t, b)); j.Status != "timeout" || time.Since(start) > 8*time.Second {
		t.Fatalf("timeout: %+v (%v)", j, time.Since(start))
	}
}

func TestAsyncRequestReturnsImmediately(t *testing.T) {
	o := newP3Env(t)
	os.WriteFile(filepath.Join(o.home, "slow"), nil, 0o644)
	o.app.longTimeout = 1 * time.Second // diatur SEBELUM job dimulai (goroutine membacanya)
	start := time.Now()
	c, _ := o.post("/api/php/8.4/install", `{}`)
	if c != 202 || time.Since(start) > 3*time.Second {
		t.Fatalf("harus segera 202: %d (%v)", c, time.Since(start))
	}
	o.app.bg.Wait()
}

// ---------- PHP dan runtime ----------

func TestPHPValidationAndRemove(t *testing.T) {
	o := newP3Env(t)
	for _, v := range []string{"8.1", "7.4", "8.4;id", "x", "8.4%20"} {
		if c, _ := o.post("/api/php/"+v+"/install", `{}`); c != 400 && c != 404 {
			t.Errorf("php %s -> %d", v, c)
		}
	}
	if o.calls() != "" {
		t.Fatal("lab terpanggil oleh versi tidak valid")
	}
	if c, _ := o.post("/api/php/8.3/remove", `{"confirm":"8.3"}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.stepup()
	if c, _ := o.post("/api/php/8.3/remove", `{"confirm":"8.4"}`); c != 400 {
		t.Fatalf("konfirmasi salah: %d", c)
	}
	c, b := o.post("/api/php/8.3/remove", `{"confirm":"8.3"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[php][remove][8.3]") {
		t.Fatalf("argv: %s", o.calls())
	}
	l := o.do("GET", "/api/php", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(l, `"version":"8.3","installed":true`) {
		t.Fatalf("daftar php: %s", l)
	}
}

func TestRuntimeEndpoints(t *testing.T) {
	o := newP3Env(t)
	bad := []string{"/api/runtime/ruby/3/install", "/api/runtime/node/22;rm/install", "/api/runtime/node/--help/use", "/api/runtime/node/..%2Fx/install", "/api/runtime/node/latest;x/remove", "/api/runtime/-rf/1/use"}
	o.stepup() // supaya penolakan benar-benar berasal dari validasi, bukan dari step-up/badan permintaan
	for _, p := range bad {
		body := `{}`
		if strings.HasSuffix(p, "/remove") {
			parts := strings.Split(p, "/")
			body = `{"confirm":"` + parts[3] + "@" + parts[4] + `"}`
		}
		if c, b := o.post(p, body); c != 400 {
			t.Errorf("%s -> %d %s (harus 400 dari validasi)", p, c, b)
		}
	}
	if o.calls() != "" {
		t.Fatalf("lab terpanggil: %s", o.calls())
	}
	c, b := o.post("/api/runtime/node/24/install", `{}`)
	if c != 202 {
		t.Fatalf("%d", c)
	}
	o.waitJob(jobID(t, b))
	if c, _ := o.post("/api/runtime/node/24/use", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[runtime][use][node][24]") {
		t.Fatalf("use: %d %s", c, o.calls())
	}
	o.now = o.now.Add(stepupTTL + time.Second) // step-up dari tahap validasi sudah habis
	if c, _ := o.post("/api/runtime/node/22.23.5/remove", `{"confirm":"node@22.23.5"}`); c != 403 {
		t.Fatalf("remove tanpa step-up: %d", c)
	}
	o.stepup()
	if c, _ := o.post("/api/runtime/node/22.23.5/remove", `{"confirm":"node"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	if c, _ := o.post("/api/runtime/node/22.23.5/remove", `{"confirm":"node@22.23.5"}`); c != 200 || !strings.Contains(o.calls(), "CALL:[runtime][remove][node][22.23.5]") {
		t.Fatalf("remove: %d", c)
	}
	l := o.do("GET", "/api/runtime", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(l, `"tool":"node","version":"26.13.1","default":true`) {
		t.Fatalf("daftar: %s", l)
	}
}

// ---------- worker dan cron ----------

func TestValidCmdTable(t *testing.T) {
	o := newP3Env(t)
	os.WriteFile(filepath.Join(o.home, "Developer/proj/artisan"), []byte("<?php"), 0o644)
	good := []string{"php artisan queue:work --tries=3 --sleep=3", "node worker.js", "python3 -m celery worker", "npm run worker", "pnpm start", "composer run-script x", "php " + o.home + "/Developer/proj/artisan schedule:run"}
	for _, c := range good {
		if err := o.app.validCmd(c); err != nil {
			t.Errorf("harus sah: %q: %v", c, err)
		}
	}
	bad := []string{
		"", "bash -c x", "sh script.sh", "curl http://x", "rm -rf x", "/bin/sh x", "./run.sh", "php;id", "php a && id", "php a | cat", "php $(id)", "php `id`", "php 'a b'", `php "a"`, "php a\\b", "php a\nid", "php a > x", "php a & b",
		"php -r phpinfo();", "php -R x", "php -B x", "php -d auto_prepend_file=/x a.php", "php -S 0.0.0.0:8000", "php -a", "php -f x.php", "node -e 1", "node --eval=1", "node -p 1", "node --print 1", "node --require /x", "node --inspect=0.0.0.0 x.js", "node --inspect-brk x.js",
		"python3 -c 1", "python -i", "npm exec x", "npm x", "pnpm dlx x", "pnpm exec x", "npm install x", "npx something", "composer exec x", "composer install", "uv run x",
		"php ../../etc/x.php", "php /etc/passwd", "php ~/x.php", "php /Users/rokianjas/.ssh/id_x", "php " + o.home + "/outside/x.php", strings.Repeat("a", 301),
	}
	for _, c := range bad {
		if err := o.app.validCmd(c); err == nil {
			t.Errorf("harus DITOLAK: %q", c)
		}
	}
}

func TestWorkerAddArgvAndGuards(t *testing.T) {
	o := newP3Env(t)
	dir := filepath.Join(o.home, "Developer/proj")
	body := `{"name":"queue","dir":"` + dir + `","cmd":"php artisan queue:work --tries=3","php":"8.4","procs":2}`
	if c, _ := o.post("/api/tasks/worker", body); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	if o.calls() != "" {
		t.Fatal("lab terpanggil tanpa step-up")
	}
	o.stepup()
	if c, b := o.post("/api/tasks/worker", body); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if !strings.Contains(o.calls(), "CALL:[worker][add][queue][--dir]["+dir+"][--cmd][php artisan queue:work --tries=3][--php][8.4][--procs][2]") {
		t.Fatalf("argv: %s", o.calls())
	}
	n0 := len(o.calls())
	bad := []string{
		`{"name":"Q","dir":"` + dir + `","cmd":"php a"}`, `{"name":"a;b","dir":"` + dir + `","cmd":"php a"}`, `{"name":"q","dir":"/etc","cmd":"php a"}`, `{"name":"q","dir":"` + dir + `/../../outside","cmd":"php a"}`,
		`{"name":"q","dir":"` + dir + `","cmd":"bash x"}`, `{"name":"q","dir":"` + dir + `","cmd":"php -r x"}`, `{"name":"q","dir":"` + dir + `","cmd":"php a","php":"8.1"}`,
		`{"name":"q","dir":"` + dir + `","cmd":"php a","procs":9}`, `{"name":"q","dir":"` + dir + `","cmd":"php a","procs":-1}`, `{"name":"q","dir":"` + dir + `","cmd":"php a","every":"5m"}`,
		`{"name":"q","dir":"` + dir + `","cmd":"php a","evil":1}`, `{"name":"q","dir":"` + dir + `/public/index.php","cmd":"php a"}`,
	}
	for _, b := range bad {
		if c, _ := o.post("/api/tasks/worker", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if len(o.calls()) != n0 {
		t.Fatal("lab terpanggil oleh input buruk")
	}
	if c, _ := o.post("/api/tasks/lain", body); c != 400 {
		t.Fatalf("jenis tak dikenal: %d", c)
	}
}

func TestJobAddScheduleValidation(t *testing.T) {
	o := newP3Env(t)
	o.stepup()
	dir := filepath.Join(o.home, "Developer/proj")
	ok := func(extra string) int {
		c, _ := o.post("/api/tasks/job", `{"name":"tick","dir":"`+dir+`","cmd":"php tick.php",`+extra+`}`)
		return c
	}
	if ok(`"every":"5m"`) != 200 || !strings.Contains(o.calls(), "[job][add][tick][--dir]["+dir+"][--cmd][php tick.php][--every][5m]") {
		t.Fatalf("every: %s", o.calls())
	}
	if ok(`"at":"03:15"`) != 200 || !strings.Contains(o.calls(), "[--at][03:15]") {
		t.Fatal("at")
	}
	for _, e := range []string{`"every":"0m"`, `"every":"5x"`, `"every":"1000m"`, `"every":"m"`, `"at":"25:00"`, `"at":"3:15"`, `"at":"03:60"`, `"every":"5m","at":"03:15"`, `"cmd2":1`, `"procs":2,"every":"5m"`, `"every":"5m;id"`} {
		if c := ok(e); c != 400 {
			t.Errorf("%s -> %d", e, c)
		}
	}
	if c, _ := o.post("/api/tasks/job", `{"name":"tick","dir":"`+dir+`","cmd":"php tick.php"}`); c != 400 {
		t.Fatalf("tanpa jadwal: %d", c)
	}
}

func TestTaskListRestartRemoveLogs(t *testing.T) {
	o := newP3Env(t)
	l := o.do("GET", "/api/tasks/worker", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(l, `"name":"queue-1","status":"jalan"`) || !strings.Contains(l, `"status":"tidak dimuat"`) {
		t.Fatalf("%s", l)
	}
	if c, _ := o.post("/api/tasks/worker/queue-1/restart", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[worker][restart][queue-1]") {
		t.Fatalf("restart: %s", o.calls())
	}
	if c, _ := o.post("/api/tasks/worker/queue/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	if c, _ := o.post("/api/tasks/worker/queue/remove", `{"confirm":"queue"}`); c != 200 || !strings.Contains(o.calls(), "CALL:[worker][remove][queue]") {
		t.Fatalf("remove: %s", o.calls())
	}
	lg := o.do("GET", "/api/tasks/worker/queue-1/logs?n=5", "", o.cookie, "", nil).Body.String()
	if strings.Contains(lg, "RAHASIA123") || !strings.Contains(lg, "baris log") {
		t.Fatalf("log: %s", lg)
	}
	for _, p := range []string{"/api/tasks/worker/Q/logs", "/api/tasks/worker/a;b/logs", "/api/tasks/worker/x/logs?n=0", "/api/tasks/worker/x/logs?n=501", "/api/tasks/worker/x/logs?n=a"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	for _, p := range []string{"/api/tasks/worker/-x/restart", "/api/tasks/worker/A/remove"} {
		if c, _ := o.post(p, `{"confirm":"x"}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
}

// ---------- update ----------

func TestUpdatesCheckAndApply(t *testing.T) {
	o := newP3Env(t)
	l := o.do("GET", "/api/updates", "", o.cookie, "", nil).Body.String()
	for _, s := range []string{`"name":"mycli"`, `"major":true`, `"source":"mise"`, `"pins":["mysql@8.4","postgresql@17"]`} {
		if !strings.Contains(l, s) {
			t.Errorf("update tanpa %s: %s", s, l)
		}
	}
	body := func(target, confirm string, pinned bool) string {
		p := "false"
		if pinned {
			p = "true"
		}
		return `{"target":"` + target + `","confirm":"` + confirm + `","allow_pinned":` + p + `}`
	}
	if c, _ := o.post("/api/updates/apply", body("mycli", "mycli", false)); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.stepup()
	before := o.calls()
	for _, b := range []string{body("mycli", "lain", false), body("x;y", "x;y", false), body("-rf", "-rf", false), body("mise:ruby", "mise:ruby", false), body("../x", "../x", false), body("", "", false)} {
		if c, _ := o.post("/api/updates/apply", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if o.calls() != before {
		t.Fatal("lab terpanggil oleh input buruk")
	}
	c, b := o.post("/api/updates/apply", body("postgresql@17", "postgresql@17", true))
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[update][apply][postgresql@17][--allow-pinned]") {
		t.Fatalf("argv: %s", o.calls())
	}
	o.now = o.now.Add(stepupTTL + time.Second)
	o.stepup()
	_, b = o.post("/api/updates/apply", body("mise:pnpm", "mise:pnpm", false))
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[update][apply][mise:pnpm] STDIN") || strings.Contains(strings.Split(o.calls(), "mise:pnpm")[1], "--allow-pinned") {
		t.Fatalf("mise argv: %s", o.calls())
	}
}

func TestAsyncJobsSerialized(t *testing.T) {
	o := newP3Env(t)
	var ids []int64
	for _, v := range []string{"8.3", "8.4", "8.5"} {
		_, b := o.post("/api/php/"+v+"/install", `{}`)
		ids = append(ids, jobID(t, b))
	}
	for _, id := range ids {
		if j := o.waitJob(id); j.Status != "ok" {
			t.Fatalf("%+v", j)
		}
	}
	b, _ := os.ReadFile(filepath.Join(o.home, "order.log"))
	if string(b) != strings.Repeat("START\nEND\n", 3) {
		t.Fatalf("job asinkron berjalan bersamaan (harus serial): %q", b)
	}
}

func TestSystemDatabasesProtected(t *testing.T) {
	o := newP3Env(t)
	o.stepup()
	for _, n := range []string{"dev", "postgres", "mysql", "root", "template1"} {
		for _, act := range []string{"drop", "dump"} {
			body := `{"confirm":"` + n + `"}`
			if c, b := o.post("/api/db/pgsql/"+n+"/"+act, body); c != 400 {
				t.Errorf("%s %s -> %d %s (harus 400)", act, n, c, b)
			}
		}
	}
	if strings.Contains(o.calls(), "[db]") {
		t.Fatalf("lab terpanggil untuk DB sistem: %s", o.calls())
	}
	if c, _ := o.post("/api/db/mysql/tokoku/drop", `{"confirm":"tokoku"}`); c != 200 {
		t.Fatalf("DB biasa tetap harus bisa: %d", c)
	}
}
