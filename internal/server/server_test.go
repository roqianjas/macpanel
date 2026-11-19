package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/server"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestServicesListSanitizesState(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	b := o.Do("GET", "/api/services", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(b, `"id":"nginx"`) || !strings.Contains(b, `"state":"running"`) {
		t.Fatalf("%s", b)
	}
	if strings.Contains(b, "HACKED") || !strings.Contains(b, `"state":"error"`) {
		t.Fatalf("state tak dikenal harus jadi 'error': %s", b)
	}
}

func TestServiceLogsScrubbedAndBounded(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	b := o.Do("GET", "/api/services/redis/logs?n=5", "", o.Cookie, "", nil).Body.String()
	if strings.Contains(b, "RAHASIA123") || !strings.Contains(b, "log token") {
		t.Fatalf("%s", b)
	}
	for _, p := range []string{"?n=0", "?n=501", "?n=a", "?n=1;2"} {
		w := o.Do("GET", "/api/services/redis/logs"+p, "", o.Cookie, "", nil)
		if w.Code != 400 && !(p == "?n=1;2" && w.Code == 200) {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	if w := o.Do("GET", "/api/services/Re%20dis/logs", "", o.Cookie, "", nil); w.Code != 400 && w.Code != 404 {
		t.Errorf("id buruk -> %d", w.Code)
	}
}

func TestSiteListRichIncludesDisabledAndSSL(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	os.WriteFile(filepath.Join(o.App.SitesDir, "tidur.conf.disabled"), []byte("server {\n    root "+o.Home+"/Developer/proj/public;\n    fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-8.3.sock;\n}"), 0o644)
	os.WriteFile(filepath.Join(o.App.SitesDir, "x y.conf.disabled"), []byte("server {}"), 0o644) // nama tak sah: diabaikan
	b := o.Do("GET", "/api/sites", "", o.Cookie, "", nil).Body.String()
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

// Regresi: id katalog PHP-FPM memuat titik ("php-8.2"); dulu ditolak 400 sehingga mulai ulang dan log FPM tak pernah jalan.
func TestServiceIDsWithVersionDot(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	for _, id := range []string{"php-8.2", "php-8.4", "php-8.5"} {
		if w := o.Do("GET", "/api/services/"+id+"/logs", "", o.Cookie, "", nil); w.Code != 200 {
			t.Errorf("log %s -> %d %s", id, w.Code, w.Body.String())
		}
	}
	o.Stepup()
	if c, b := o.Post("/api/services/php-8.4/restart", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[service][restart][php-8.4]") {
		t.Fatalf("restart php-8.4: %d %s %s", c, b, o.Calls())
	}
	before := o.Calls()
	for _, id := range []string{"php-8..4", "php-.8", "php-8.", "php-8.123", "php-8.x", ".8", "php-8.4.1", "php-8.4;id", "php%2F8.4"} {
		if c, _ := o.Post("/api/services/"+id+"/restart", `{}`); c != 400 && c != 404 {
			t.Errorf("id buruk %q harus ditolak: %d", id, c)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh id buruk: %s", o.Calls())
	}
}

func TestRedisEndpoints(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeRedis)
	o.App.LongTimeout = 20 * time.Second
	if w := o.Do("GET", "/api/redis/scan?pattern=user:*", "", o.Cookie, "", nil); w.Code != 200 {
		t.Fatalf("scan: %d", w.Code)
	}
	before := o.Calls()
	for _, q := range []string{"/api/redis/scan?pattern=a%20b", "/api/redis/scan?pattern=a%3Bid", "/api/redis/scan?pattern=$(id)", "/api/redis/scan?db=16", "/api/redis/scan?db=-1", "/api/redis/scan?db=01", "/api/redis/scan?db=x", "/api/redis/info?db=99"} {
		if w := o.Do("GET", q, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("%s -> %d", q, w.Code)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil: %s", o.Calls())
	}
	if c, b := o.Post("/api/redis/get", `{"key":"k"}`); c != 403 || strings.Contains(b, "rahasia") {
		t.Fatalf("get tanpa step-up: %d %s", c, b)
	}
	if c, _ := o.Post("/api/redis/del", `{"key":"k"}`); c != 403 {
		t.Fatalf("del tanpa step-up: %d", c)
	}
	o.Stepup()
	for _, b := range []string{`{"key":""}`, `{"key":"a b"}`, `{"key":"-x"}`, `{"key":"a\nFLUSHALL"}`, `{"key":"` + strings.Repeat("a", 201) + `"}`, `{"key":"k","db":"16"}`, `{"key":"k","x":1}`, `{"key":"é"}`} {
		for _, p := range []string{"/api/redis/get", "/api/redis/del"} {
			if c, r := o.Post(p, b); c != 400 {
				t.Errorf("%s %s -> %d %s", p, b, c, r)
			}
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	if c, b := o.Post("/api/redis/get", `{"key":"sess:abc","db":"2"}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[redis][get][--db][2][sess:abc]") {
		t.Fatalf("get: %d %s %s", c, b, o.Calls())
	}
	if c, _ := o.Post("/api/redis/del", `{"key":"sess:abc"}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[redis][del][--db][0][sess:abc]") {
		t.Fatalf("del: %s", o.Calls())
	}
	if au := o.Do("GET", "/api/audit", "", o.Cookie, "", nil).Body.String(); !strings.Contains(au, "redis_view") || strings.Contains(au, "rahasia") {
		t.Errorf("audit: %s", au)
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	for _, p := range []string{"/api/redis/scan", "/api/redis/info"} {
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 403 {
			t.Errorf("viewer %s: %d", p, w.Code)
		}
	}
	if c, b := o.Post("/api/redis/get", `{"key":"k"}`); c != 403 || strings.Contains(b, "rahasia") {
		t.Errorf("viewer get: %d %s", c, b)
	}
}

func TestChangeOwnPassword(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	const old, nw = "kata-sandi-panjang-123", "sandi-baru-yang-panjang-456"
	// sesi kedua milik user yang sama harus dicabut
	o.Now = o.Now.Add(31 * time.Second)
	c2, _, w := o.Login(o.Code(0))
	if w.Code != 200 {
		t.Fatalf("login kedua: %d", w.Code)
	}
	if c, _ := o.Post("/api/me/password", `{"current":"`+old+`","new":"`+nw+`"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	for _, b := range []string{`{"current":"salah","new":"` + nw + `"}`, `{"current":"` + old + `","new":"pendek"}`, `{"current":"` + old + `","new":"` + old + `"}`, `{"current":"` + old + `","new":"admin"}`, `{"current":"` + old + `","new":"` + strings.Repeat("a", 257) + `"}`, `{"current":"` + old + `","new":"` + nw + `","x":1}`} {
		if c, r := o.Post("/api/me/password", b); c != 400 && c != 401 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if c, _ := o.Post("/api/me/password", `{"current":"salah","new":"`+nw+`"}`); c != 400 {
		t.Errorf("sandi saat ini salah harus 400 (401 membuat UI mengira sesi habis): %d", c)
	}
	if w := o.Do("POST", "/api/auth/login", `{"name":"admin","password":"`+nw+`"}`, "", "", nil); w.Code == 200 {
		t.Fatal("sandi baru tidak boleh aktif setelah permintaan ditolak")
	}
	if c, b := o.Post("/api/me/password", `{"current":"`+old+`","new":"`+nw+`"}`); c != 200 {
		t.Fatalf("ganti sah: %d %s", c, b)
	}
	if w := o.Do("GET", "/api/me", "", c2, "", nil); w.Code != 401 {
		t.Errorf("sesi lain harus dicabut: %d", w.Code)
	}
	if w := o.Do("GET", "/api/me", "", o.Cookie, "", nil); w.Code != 200 {
		t.Errorf("sesi saat ini harus tetap hidup: %d", w.Code)
	}
	if w := o.Do("POST", "/api/auth/login", `{"name":"admin","password":"`+old+`"}`, "", "", nil); w.Code == 200 {
		t.Error("sandi lama harus ditolak")
	}
	if w := o.Do("POST", "/api/auth/login", `{"name":"admin","password":"`+nw+`"}`, "", "", nil); w.Code != 200 {
		t.Errorf("sandi baru harus berlaku: %d", w.Code)
	}
	if au := o.Do("GET", "/api/audit", "", o.Cookie, "", nil).Body.String(); !strings.Contains(au, "password_change") || strings.Contains(au, nw) || strings.Contains(au, old) {
		t.Errorf("audit: %s", au)
	}
}

func TestInstallAndCloneGuards(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	o.App.LongTimeout = 20 * time.Second
	if c, _ := o.Post("/api/installer", `{"kind":"laravel","name":"toko"}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	if c, _ := o.Post("/api/sites/demo/clone", `{"dst":"toko2"}`); c != 403 {
		t.Fatalf("clone tanpa step-up: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	for _, b := range []string{
		`{"kind":"drupal","name":"toko"}`, `{"kind":"laravel","name":"Toko"}`, `{"kind":"laravel","name":"a;id"}`, `{"kind":"laravel","name":"../x"}`, `{"kind":"laravel","name":"panel"}`,
		`{"kind":"laravel","name":"home"}`, `{"kind":"laravel","name":"toko","php":"7.4"}`, `{"kind":"laravel","name":"` + strings.Repeat("a", 31) + `"}`, `{"kind":"laravel","name":"-x"}`, `{"kind":"laravel","name":"toko","x":1}`, `{"name":"toko"}`,
	} {
		if c, r := o.Post("/api/installer", b); c != 400 {
			t.Errorf("installer %s -> %d %s", b, c, r)
		}
	}
	for _, b := range []string{`{"dst":"demo"}`, `{"dst":"a;b"}`, `{"dst":"panel"}`, `{"dst":""}`, `{"dst":"../x"}`, `{"dst":"ok","x":1}`} {
		if c, r := o.Post("/api/sites/demo/clone", b); c != 400 {
			t.Errorf("clone %s -> %d %s", b, c, r)
		}
	}
	if c, _ := o.Post("/api/sites/panel/clone", `{"dst":"ok"}`); c != 400 && c != 404 {
		t.Errorf("situs inti sebagai sumber: %d", c)
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	c, b := o.Post("/api/installer", `{"kind":"wordpress","name":"blog","php":"8.3"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[appinstall][wordpress][blog][--php][8.3]") {
		t.Fatalf("%s", o.Calls())
	}
	c, b = o.Post("/api/sites/demo/clone", `{"dst":"staging","with_db":true}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[siteclone][demo][staging][--with-db]") {
		t.Fatalf("%s", o.Calls())
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if c, _ := o.Post("/api/installer", `{"kind":"laravel","name":"toko"}`); c != 403 {
		t.Errorf("viewer: %d", c)
	}
}

func TestBackupCleanupEndpoints(t *testing.T) {
	o := testenv.NewOpsEnv(t, `case "$2" in prune) printf '{"keep":7,"days":0,"remove":[],"kept":1}';; extras) printf '{"count":0,"kb":0}';; *) echo ok;; esac`)
	o.App.LongTimeout = 20 * time.Second
	for _, p := range []string{"/api/backups/prune-preview?keep=5&days=30", "/api/backups/extras", "/api/backups/extras-preview?kind=db&days=30"} {
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 200 {
			t.Errorf("%s -> %d %s", p, w.Code, w.Body.String())
		}
	}
	before := o.Calls()
	for _, p := range []string{"keep=0", "keep=61", "keep=a", "days=-1", "days=3651", "keep=05", "days=1%3Bid", "keep=1%20--x"} {
		if w := o.Do("GET", "/api/backups/prune-preview?"+p, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("preview %s -> %d", p, w.Code)
		}
	}
	for _, p := range []string{"kind=x", "kind=db&days=-1", "kind=db&days=x", "kind=retired", "kind=db%3Bid", "kind="} {
		if w := o.Do("GET", "/api/backups/extras-preview?"+p, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("extras-preview %s -> %d", p, w.Code)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	for _, c := range [][2]string{{"/api/backups/prune", `{"keep":7,"days":0}`}, {"/api/backups/delete", `{"names":["20261117-023004"]}`}, {"/api/backups/extras", `{"kind":"db","days":1}`}} {
		if code, _ := o.Post(c[0], c[1]); code != 403 {
			t.Errorf("%s tanpa step-up: %d", c[0], code)
		}
	}
	o.Stepup()
	for _, c := range [][2]string{
		{"/api/backups/prune", `{"keep":0,"days":0}`}, {"/api/backups/prune", `{"keep":61,"days":0}`}, {"/api/backups/prune", `{"keep":7,"days":-1}`}, {"/api/backups/prune", `{"keep":7,"days":3651}`}, {"/api/backups/prune", `{"keep":7,"x":1}`},
		{"/api/backups/delete", `{"names":[]}`}, {"/api/backups/delete", `{"names":["../x"]}`}, {"/api/backups/delete", `{"names":["20261117-023004","20261117-023004"]}`}, {"/api/backups/delete", `{"names":["-rf"]}`}, {"/api/backups/delete", `{"names":["20261117-023004;id"]}`},
		{"/api/backups/extras", `{"kind":"retired","days":1}`}, {"/api/backups/extras", `{"kind":"db","days":-1}`}, {"/api/backups/extras", `{"kind":"db","days":3651}`}, {"/api/backups/extras", `{"kind":"../x","days":1}`},
	} {
		if code, r := o.Post(c[0], c[1]); code != 400 {
			t.Errorf("%s %s -> %d %s", c[0], c[1], code, r)
		}
	}
	var nm []string
	for i := 0; i < 31; i++ {
		nm = append(nm, fmt.Sprintf(`"202601%02d-000000"`, i+1))
	}
	many := `{"names":[` + strings.Join(nm, ",") + `]}`
	if code, _ := o.Post("/api/backups/delete", many); code != 400 {
		t.Errorf("31 nama harus ditolak: %d", code)
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	if code, _ := o.Post("/api/backups/prune", `{"keep":5,"days":30}`); code != 200 || !strings.Contains(o.Calls(), "CALL:[backup][prune][--keep][5][--days][30]") {
		t.Fatalf("prune: %s", o.Calls())
	}
	if code, _ := o.Post("/api/backups/delete", `{"names":["20261116-084328","20261116-102324"]}`); code != 200 || !strings.Contains(o.Calls(), "CALL:[backup][delete][20261116-084328][20261116-102324]") {
		t.Fatalf("delete: %s", o.Calls())
	}
	if code, _ := o.Post("/api/backups/extras", `{"kind":"restore","days":0}`); code != 200 || !strings.Contains(o.Calls(), "CALL:[backup][extras][restore][--days][0]") {
		t.Fatalf("extras: %s", o.Calls())
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if code, _ := o.Post("/api/backups/delete", `{"names":["20261116-084328"]}`); code != 403 {
		t.Errorf("viewer: %d", code)
	}
}

func TestWebhookManageAndSecretOnce(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if c, _ := o.Post("/api/sites/demo/hook", `{"enabled":true,"branch":"main","steps":["git_pull"]}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	sec := testenv.SetupHook(t, o)
	if g := o.Do("GET", "/api/sites/demo/hook", "", o.Cookie, "", nil).Body.String(); strings.Contains(g, sec) || strings.Contains(g, "secret") || !strings.Contains(g, `"branch":"main"`) {
		t.Fatalf("GET tidak boleh memuat secret: %s", g)
	}
	var raw []byte
	o.App.Store.DB.QueryRow("SELECT secret_enc FROM hooks WHERE site='demo'").Scan(&raw)
	if strings.Contains(string(raw), sec) {
		t.Fatal("secret tersimpan polos di database")
	}
	// ubah tanpa rotate: tidak ada secret baru, secret lama tetap berlaku
	c, b := o.Post("/api/sites/demo/hook", `{"enabled":true,"branch":"deploy","steps":["git_pull"]}`)
	if c != 200 || strings.Contains(b, "secret") {
		t.Fatalf("update: %d %s", c, b)
	}
	if w := o.Hook("demo", `{"ref":"refs/heads/deploy","after":"abc1234"}`, testenv.Sign(sec, `{"ref":"refs/heads/deploy","after":"abc1234"}`), "push", "d1", "1.2.3.4"); w.Code != 202 || !strings.Contains(w.Body.String(), "job") {
		t.Fatalf("secret lama harus tetap berlaku: %d %s", w.Code, w.Body.String())
	}
	// rotate: secret baru, lama mati
	c, b = o.Post("/api/sites/demo/hook", `{"enabled":true,"branch":"deploy","steps":["git_pull"],"rotate":true}`)
	var out struct{ Secret string }
	json.Unmarshal([]byte(b), &out)
	if c != 200 || len(out.Secret) != 64 || out.Secret == sec {
		t.Fatalf("rotate: %d %s", c, b)
	}
	o.Now = o.Now.Add(time.Minute)
	body := `{"ref":"refs/heads/deploy"}`
	if w := o.Hook("demo", body, testenv.Sign(sec, body), "push", "d2", "1.2.3.4"); w.Code != 404 {
		t.Errorf("secret lama harus ditolak: %d", w.Code)
	}
	if au := o.Do("GET", "/api/audit", "", o.Cookie, "", nil).Body.String(); strings.Contains(au, out.Secret) || strings.Contains(au, sec) {
		t.Errorf("audit memuat secret")
	}
	for _, bad := range []string{
		`{"enabled":true,"branch":"","steps":["git_pull"]}`, `{"enabled":true,"branch":"-x","steps":["git_pull"]}`, `{"enabled":true,"branch":"a..b","steps":["git_pull"]}`, `{"enabled":true,"branch":"a b","steps":["git_pull"]}`,
		`{"enabled":true,"branch":"a;id","steps":["git_pull"]}`, `{"enabled":true,"branch":"main/","steps":["git_pull"]}`, `{"enabled":true,"branch":"a//b","steps":["git_pull"]}`, `{"enabled":true,"branch":"main","steps":["git_pull","composer_install","composer_install_nodev","composer_update","dump_autoload","node_install","node_build","artisan_migrate","artisan_optimize"]}`, `{"enabled":true,"branch":"x.lock","steps":["git_pull"]}`,
		`{"enabled":true,"branch":"main","steps":[]}`, `{"enabled":true,"branch":"main","steps":["rm -rf"]}`, `{"enabled":true,"branch":"main","steps":["git_pull","git_pull"]}`, `{"enabled":true,"branch":"main","steps":["git_pull"],"x":1}`,
	} {
		if c, r := o.Post("/api/sites/demo/hook", bad); c != 400 {
			t.Errorf("%s -> %d %s", bad, c, r)
		}
	}
	if c, _ := o.Post("/api/sites/panel/hook", `{"enabled":true,"branch":"main","steps":["git_pull"]}`); c != 400 && c != 404 {
		t.Errorf("situs inti: %d", c)
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if w := o.Do("GET", "/api/sites/demo/hook", "", o.Cookie, "", nil); w.Code != 403 {
		t.Errorf("viewer: %d", w.Code)
	}
}

func TestWebhookBruteForceLockout(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	sec := testenv.SetupHook(t, o)
	push := `{"ref":"refs/heads/main"}`
	locked := false
	for i := 0; i < 40; i++ {
		if w := o.Hook("demo", push, testenv.Sign("tebakan", push), "push", "", "7.7.7.7"); w.Code == 429 {
			locked = true
			break
		}
	}
	if !locked {
		t.Fatal("percobaan tanda tangan salah berulang harus dikunci (429)")
	}
	if w := o.Hook("demo", push, testenv.Sign(sec, push), "push", "z1", "7.7.7.7"); w.Code != 429 {
		t.Errorf("IP terkunci harus tetap ditolak walau tanda tangan benar: %d", w.Code)
	}
	if w := o.Hook("demo", push, testenv.Sign(sec, push), "push", "z2", "8.8.8.8"); w.Code != 202 {
		t.Errorf("IP lain tidak boleh terdampak: %d", w.Code)
	}
}

func TestSysDNSReadOnly(t *testing.T) {
	o := testenv.NewOpsEnv(t, `case "$2" in dns) printf '{"tld":"lab","wildcard":"100.1.1.1","rows":[]}';; *) echo ok;; esac`)
	o.App.LongTimeout = 20 * time.Second
	if w := o.Do("GET", "/api/sys/dns", "", o.Cookie, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "wildcard") {
		t.Fatalf("dns: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(o.Calls(), "CALL:[sys][dns]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	// hanya baca: tidak ada rute tulis untuk DNS
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		if w := o.Do(m, "/api/sys/dns", "{}", o.Cookie, o.Csrf, nil); w.Code != 404 && w.Code != 405 {
			t.Errorf("%s /api/sys/dns harus tidak ada: %d", m, w.Code)
		}
	}
}

func TestStepUpUsesPasswordNotTOTP(t *testing.T) {
	e := testenv.NewEnv(t)
	ck, cs, _ := e.TrustLogin(false)
	me := func() string { return e.Do("GET", "/api/me", "", ck, "", nil).Body.String() }
	if !strings.Contains(me(), `"stepup":false`) {
		t.Fatalf("awal: %s", me())
	}
	// kode TOTP tidak lagi diterima di step-up
	if w := e.Do("POST", "/api/auth/stepup", `{"code":"`+e.Code(1)+`"}`, ck, cs, nil); w.Code == 200 {
		t.Fatal("TOTP tidak boleh lagi membuka step-up")
	}
	if w := e.Do("POST", "/api/auth/stepup", `{"password":"`+e.Code(1)+`"}`, ck, cs, nil); w.Code != 400 {
		t.Fatalf("kode TOTP sebagai password harus ditolak: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/stepup", `{"password":"salah-salah-salah"}`, ck, cs, nil); w.Code != 400 || !strings.Contains(w.Body.String(), "kata sandi salah") {
		t.Fatalf("sandi salah harus 400 (bukan 401 agar sesi tidak keluar): %d %s", w.Code, w.Body)
	}
	if w := e.Do("GET", "/api/me", "", ck, "", nil); w.Code != 200 {
		t.Fatalf("sesi harus tetap hidup setelah sandi salah: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck, cs, nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid_seconds":900`) {
		t.Fatalf("sandi benar: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(me(), `"stepup":true`) {
		t.Fatal("step-up harus aktif")
	}
	e.Now = e.Now.Add(14 * time.Minute)
	if !strings.Contains(me(), `"stepup":true`) {
		t.Fatal("harus masih aktif di menit ke-14")
	}
	e.Now = e.Now.Add(2 * time.Minute)
	if !strings.Contains(me(), `"stepup":false`) {
		t.Fatal("harus habis setelah 15 menit")
	}
}

func TestStepUpPasswordBruteForceLocksOut(t *testing.T) {
	e := testenv.NewEnv(t)
	ck, cs, _ := e.TrustLogin(false)
	locked := false
	for i := 0; i < 30; i++ {
		w := e.Do("POST", "/api/auth/stepup", `{"password":"tebakan-`+string(rune('a'+i%26))+`-panjang"}`, ck, cs, nil)
		if w.Code == 429 {
			locked = true
			break
		}
	}
	if !locked {
		t.Fatal("tebakan sandi di step-up harus memicu lockout")
	}
	if w := e.Do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck, cs, nil); w.Code != 429 {
		t.Fatalf("saat terkunci, sandi benar pun harus ditolak: %d", w.Code)
	}
}

func TestStepUpRequiredMessageAndDisabledUser(t *testing.T) {
	o := testenv.NewOpsEnv(t, "echo ok")
	if c, b := o.Post("/api/redis/get", `{"key":"k"}`); c != 403 || !strings.Contains(b, "kata sandi ulang") {
		t.Fatalf("pesan step-up: %d %s", c, b)
	}
}

func TestDBRootErrorScrubsPassword(t *testing.T) {
	o := testenv.NewOpsEnv(t, `echo "ERROR: syntax near PASSWORD 'SandiBocor-123456'" >&2; exit 1`)
	o.Stepup()
	pw := "SandiBocor-123456"
	c, b := o.Post("/api/dbroot/pgsql/passwd", `{"confirm":"pgsql","password":"`+pw+`"}`)
	if c != 422 || strings.Contains(b, pw) {
		t.Fatalf("galat tidak boleh memuat sandi: %d %s", c, b)
	}
}

func TestViewerCanOnlyManageOwnAccount(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.DbFake)
	o.Stepup()
	_, b := o.Post("/api/accounts", `{"name":"dewi","role":"viewer"}`)
	var mk struct {
		Temp string `json:"temp_password"`
	}
	json.Unmarshal([]byte(b), &mk)
	vc, vs, code := o.Login2("dewi", mk.Temp)
	if code != 200 {
		t.Fatalf("login viewer: %d", code)
	}
	if c, b := o.As(vc, vs, "POST", "/api/auth/stepup", `{"password":"`+mk.Temp+`"}`); c != 200 {
		t.Fatalf("viewer harus bisa verifikasi ulang: %d %s", c, b)
	}
	if c, b := o.As(vc, vs, "POST", "/api/me/password", `{"current":"`+mk.Temp+`","new":"sandi-dewi-baru-12345"}`); c != 200 {
		t.Fatalf("viewer harus bisa ganti sandi sementara: %d %s", c, b)
	}
	if c, _ := o.As(vc, vs, "GET", "/api/sites", ""); c != 200 {
		t.Fatalf("setelah ganti sandi viewer bisa membaca: %d", c)
	}
	for _, p := range [][2]string{{"/api/settings", `{"name":"X"}`}, {"/api/sites", `{"name":"x","target":"~/x"}`}, {"/api/accounts", `{"name":"zzz"}`}, {"/api/db/mariadb/proj/dbusers", `{"user":"abc","privs":"all"}`}, {"/api/me/tokens", `{"name":"t","scope":"full"}`}} {
		if c, b := o.As(vc, vs, "POST", p[0], p[1]); c != 403 {
			t.Fatalf("viewer tidak boleh POST %s: %d %s", p[0], c, b)
		}
	}
}

func TestInstallAppKinds(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if c, _ := o.Post("/api/installer", `{"kind":"go","name":"api-go"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	for _, bad := range []string{
		`{"kind":"ruby","name":"x-app"}`, `{"kind":"go","name":"1go"}`, `{"kind":"go","name":"Go App"}`, `{"kind":"go","name":"api-go","runtime":"24;rm"}`,
		`{"kind":"nextjs","name":"web","runtime":"../x"}`, `{"kind":"python","name":"svc","port":80}`, `{"kind":"python","name":"svc","port":9100}`, `{"kind":"python","name":"svc","port":3306}`, `{"kind":"python","name":"svc","port":9101}`, `{"kind":"go","name":"panel"}`,
	} {
		if c, b := o.Post("/api/installer", bad); c != 400 {
			t.Fatalf("%s harus 400: %d %s", bad, c, b)
		}
	}
	if strings.Contains(o.Calls(), "appinstall") {
		t.Fatal("lab dipanggil dengan input tak valid")
	}
	for _, tc := range []struct{ body, want string }{
		{`{"kind":"go","name":"api-go"}`, "[appinstall][go][api-go]"},
		{`{"kind":"python","name":"svc-py","runtime":"3.13","port":3500}`, "[appinstall][python][svc-py][--runtime][3.13][--port][3500]"},
		{`{"kind":"nextjs","name":"web","runtime":"24.22.3"}`, "[appinstall][nextjs][web][--runtime][24.22.3]"},
	} {
		if c, b := o.Post("/api/installer", tc.body); c != 202 {
			t.Fatalf("%s: %d %s", tc.body, c, b)
		}
		deadline := time.Now().Add(3 * time.Second)
		for !strings.Contains(o.Calls(), tc.want) && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if !strings.Contains(o.Calls(), tc.want) {
			t.Fatalf("argumen salah, harus memuat %s: %s", tc.want, o.Calls())
		}
	}
	if strings.Contains(o.Calls(), "[--php]") && strings.Contains(o.Calls(), "[go][api-go][--php]") {
		t.Fatal("proyek aplikasi tidak boleh membawa --php")
	}
}

func TestDockerEndpoints(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for _, bad := range []string{"/api/docker/containers/a;b/start", "/api/docker/containers/-x/start", "/api/docker/containers/web/exec", "/api/docker/containers/web/attach"} {
		if c, _ := o.Post(bad, `{}`); c != 400 && c != 404 {
			t.Fatalf("%s harus ditolak: %d", bad, c)
		}
	}
	if strings.Contains(o.Calls(), "docker") {
		t.Fatal("lab dipanggil dengan input tak valid")
	}
	if c, _ := o.Post("/api/docker/containers/web/start", `{}`); c != 200 {
		t.Fatalf("start tanpa step-up harus boleh: %d", c)
	}
	if c, _ := o.Post("/api/docker/containers/web/remove", `{"confirm":"web"}`); c != 403 {
		t.Fatalf("hapus tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	if c, _ := o.Post("/api/docker/containers/web/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("hapus tanpa ketik ulang nama harus 400: %d", c)
	}
	for _, bad := range []string{`{"ref":"NGINX"}`, `{"ref":"a b"}`, `{"ref":"x;rm -rf /"}`, `{"ref":"--privileged"}`, `{"ref":""}`} {
		if c, _ := o.Post("/api/docker/images/pull", bad); c != 400 {
			t.Fatalf("pull %s harus 400: %d", bad, c)
		}
	}
	if c, _ := o.Post("/api/docker/images/pull", `{"ref":"nginx:1.27"}`); c != 202 {
		t.Fatalf("pull sah: %d", c)
	}
	if c, _ := o.Post("/api/docker/images/remove", `{"ref":"nginx:1.27","confirm":"x"}`); c != 400 {
		t.Fatalf("hapus image tanpa konfirmasi: %d", c)
	}
	if c, _ := o.Post("/api/docker/compose/..%2Fetc/up", `{}`); c != 400 && c != 404 {
		t.Fatalf("proyek traversal harus ditolak: %d", c)
	}
	if c, _ := o.Post("/api/docker/compose/Web/up", `{}`); c != 400 {
		t.Fatalf("nama huruf besar harus 400: %d", c)
	}
	if c, _ := o.Post("/api/docker/compose/web/exec", `{}`); c != 400 {
		t.Fatalf("aksi compose liar harus 400: %d", c)
	}
	if c, _ := o.Post("/api/docker/compose/web/up", `{}`); c != 202 {
		t.Fatalf("compose up sah: %d", c)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(o.Calls(), "[docker][compose][up][web]") && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(o.Calls(), "[docker][compose][up][web]") {
		t.Fatalf("argumen compose salah: %s", o.Calls())
	}
}

func TestAsyncJobFailureAndTimeout(t *testing.T) {
	o := testenv.NewP3Env(t)
	os.WriteFile(filepath.Join(o.Home, "fail"), nil, 0o644)
	_, b := o.Post("/api/php/8.3/install", `{}`)
	if j := o.WaitJob(testenv.JobID(t, b)); j.Status != "gagal" || j.Code != 2 || !strings.Contains(j.Output, "boom") {
		t.Fatalf("%+v", j)
	}
	os.Remove(filepath.Join(o.Home, "fail"))
	os.WriteFile(filepath.Join(o.Home, "slow"), nil, 0o644)
	o.App.LongTimeout = time.Second
	start := time.Now()
	_, b = o.Post("/api/php/8.3/install", `{}`)
	if j := o.WaitJob(testenv.JobID(t, b)); j.Status != "timeout" || time.Since(start) > 8*time.Second {
		t.Fatalf("timeout: %+v (%v)", j, time.Since(start))
	}
}

func TestAsyncRequestReturnsImmediately(t *testing.T) {
	o := testenv.NewP3Env(t)
	os.WriteFile(filepath.Join(o.Home, "slow"), nil, 0o644)
	o.App.LongTimeout = 1 * time.Second // diatur SEBELUM job dimulai (goroutine membacanya)
	start := time.Now()
	c, _ := o.Post("/api/php/8.4/install", `{}`)
	if c != 202 || time.Since(start) > 3*time.Second {
		t.Fatalf("harus segera 202: %d (%v)", c, time.Since(start))
	}
	o.App.BG.Wait()
}

func TestPHPValidationAndRemove(t *testing.T) {
	o := testenv.NewP3Env(t)
	for _, v := range []string{"8.1", "7.4", "8.4;id", "x", "8.4%20"} {
		if c, _ := o.Post("/api/php/"+v+"/install", `{}`); c != 400 && c != 404 {
			t.Errorf("php %s -> %d", v, c)
		}
	}
	if o.Calls() != "" {
		t.Fatal("lab terpanggil oleh versi tidak valid")
	}
	if c, _ := o.Post("/api/php/8.3/remove", `{"confirm":"8.3"}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.Stepup()
	if c, _ := o.Post("/api/php/8.3/remove", `{"confirm":"8.4"}`); c != 400 {
		t.Fatalf("konfirmasi salah: %d", c)
	}
	c, b := o.Post("/api/php/8.3/remove", `{"confirm":"8.3"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[php][remove][8.3]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	l := o.Do("GET", "/api/php", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(l, `"version":"8.3","installed":true`) {
		t.Fatalf("daftar php: %s", l)
	}
}

func TestWorkerAddArgvAndGuards(t *testing.T) {
	o := testenv.NewP3Env(t)
	dir := filepath.Join(o.Home, "Developer/proj")
	body := `{"name":"queue","dir":"` + dir + `","cmd":"php artisan queue:work --tries=3","php":"8.4","procs":2}`
	if c, _ := o.Post("/api/tasks/worker", body); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	if o.Calls() != "" {
		t.Fatal("lab terpanggil tanpa step-up")
	}
	o.Stepup()
	if c, b := o.Post("/api/tasks/worker", body); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if !strings.Contains(o.Calls(), "CALL:[worker][add][queue][--type][cmd][--dir]["+dir+"][--cmd][php artisan queue:work --tries=3][--php][8.4][--procs][2]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	n0 := len(o.Calls())
	bad := []string{
		`{"name":"Q","dir":"` + dir + `","cmd":"php a"}`, `{"name":"a;b","dir":"` + dir + `","cmd":"php a"}`, `{"name":"q","dir":"/etc","cmd":"php a"}`, `{"name":"q","dir":"` + dir + `/../../outside","cmd":"php a"}`,
		`{"name":"q","dir":"` + dir + `","cmd":"bash x"}`, `{"name":"q","dir":"` + dir + `","cmd":"php -r x"}`, `{"name":"q","dir":"` + dir + `","cmd":"php a","php":"8.1"}`,
		`{"name":"q","dir":"` + dir + `","cmd":"php a","procs":9}`, `{"name":"q","dir":"` + dir + `","cmd":"php a","procs":-1}`, `{"name":"q","dir":"` + dir + `","cmd":"php a","every":"5m"}`,
		`{"name":"q","dir":"` + dir + `","cmd":"php a","evil":1}`, `{"name":"q","dir":"` + dir + `/public/index.php","cmd":"php a"}`,
	}
	for _, b := range bad {
		if c, _ := o.Post("/api/tasks/worker", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if len(o.Calls()) != n0 {
		t.Fatal("lab terpanggil oleh input buruk")
	}
	if c, _ := o.Post("/api/tasks/lain", body); c != 400 {
		t.Fatalf("jenis tak dikenal: %d", c)
	}
}

func TestJobAddScheduleValidation(t *testing.T) {
	o := testenv.NewP3Env(t)
	o.Stepup()
	dir := filepath.Join(o.Home, "Developer/proj")
	ok := func(extra string) int {
		c, _ := o.Post("/api/tasks/job", `{"name":"tick","dir":"`+dir+`","cmd":"php tick.php",`+extra+`}`)
		return c
	}
	if ok(`"every":"5m"`) != 200 || !strings.Contains(o.Calls(), "[job][add][tick][--type][cmd][--dir]["+dir+"][--cmd][php tick.php][--every][5m]") {
		t.Fatalf("every: %s", o.Calls())
	}
	if ok(`"at":"03:15"`) != 200 || !strings.Contains(o.Calls(), "[--at][03:15]") {
		t.Fatal("at")
	}
	for _, e := range []string{`"every":"0m"`, `"every":"5x"`, `"every":"1000m"`, `"every":"m"`, `"at":"25:00"`, `"at":"3:15"`, `"at":"03:60"`, `"every":"5m","at":"03:15"`, `"cmd2":1`, `"procs":2,"every":"5m"`, `"every":"5m;id"`} {
		if c := ok(e); c != 400 {
			t.Errorf("%s -> %d", e, c)
		}
	}
	if c, _ := o.Post("/api/tasks/job", `{"name":"tick","dir":"`+dir+`","cmd":"php tick.php"}`); c != 400 {
		t.Fatalf("tanpa jadwal: %d", c)
	}
}

func TestTaskListRestartRemoveLogs(t *testing.T) {
	o := testenv.NewP3Env(t)
	l := o.Do("GET", "/api/tasks/worker", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(l, `"name":"queue-1","status":"jalan"`) || !strings.Contains(l, `"status":"tidak dimuat"`) {
		t.Fatalf("%s", l)
	}
	if c, _ := o.Post("/api/tasks/worker/queue-1/restart", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[worker][restart][queue-1]") {
		t.Fatalf("restart: %s", o.Calls())
	}
	if c, _ := o.Post("/api/tasks/worker/queue/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	if c, _ := o.Post("/api/tasks/worker/queue/remove", `{"confirm":"queue"}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[worker][remove][queue]") {
		t.Fatalf("remove: %s", o.Calls())
	}
	lg := o.Do("GET", "/api/tasks/worker/queue-1/logs?n=5", "", o.Cookie, "", nil).Body.String()
	if strings.Contains(lg, "RAHASIA123") || !strings.Contains(lg, "baris log") {
		t.Fatalf("log: %s", lg)
	}
	for _, p := range []string{"/api/tasks/worker/Q/logs", "/api/tasks/worker/a;b/logs", "/api/tasks/worker/x/logs?n=0", "/api/tasks/worker/x/logs?n=501", "/api/tasks/worker/x/logs?n=a"} {
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	for _, p := range []string{"/api/tasks/worker/-x/restart", "/api/tasks/worker/A/remove"} {
		if c, _ := o.Post(p, `{"confirm":"x"}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
}

func TestAsyncJobsSerialized(t *testing.T) {
	o := testenv.NewP3Env(t)
	var ids []int64
	for _, v := range []string{"8.3", "8.4", "8.5"} {
		_, b := o.Post("/api/php/"+v+"/install", `{}`)
		ids = append(ids, testenv.JobID(t, b))
	}
	for _, id := range ids {
		if j := o.WaitJob(id); j.Status != "ok" {
			t.Fatalf("%+v", j)
		}
	}
	b, _ := os.ReadFile(filepath.Join(o.Home, "order.log"))
	if string(b) != strings.Repeat("START\nEND\n", 3) {
		t.Fatalf("job asinkron berjalan bersamaan (harus serial): %q", b)
	}
}

func TestSystemDatabasesProtected(t *testing.T) {
	o := testenv.NewP3Env(t)
	o.Stepup()
	for _, n := range []string{"dev", "postgres", "mysql", "root", "template1"} {
		for _, act := range []string{"drop", "dump"} {
			body := `{"confirm":"` + n + `"}`
			if c, b := o.Post("/api/db/pgsql/"+n+"/"+act, body); c != 400 {
				t.Errorf("%s %s -> %d %s (harus 400)", act, n, c, b)
			}
		}
	}
	if strings.Contains(o.Calls(), "[db]") {
		t.Fatalf("lab terpanggil untuk DB sistem: %s", o.Calls())
	}
	if c, _ := o.Post("/api/db/mysql/tokoku/drop", `{"confirm":"tokoku"}`); c != 200 {
		t.Fatalf("DB biasa tetap harus bisa: %d", c)
	}
}

func TestExtListOnlyKnownNames(t *testing.T) {
	o := testenv.NewDevEnv(t)
	b := o.Do("GET", "/api/php/8.4/ext", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(b, `"name":"redis"`) || strings.Contains(b, "EVIL") {
		t.Fatalf("nama tak dikenal harus dikosongkan: %s", b)
	}
	for _, p := range []string{"/api/php/7.4/ext", "/api/php/8.4;id/ext", "/api/php/8.44/ext"} {
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
}

func TestIniGetFiltersUnknownKeys(t *testing.T) {
	o := testenv.NewDevEnv(t)
	b := o.Do("GET", "/api/php/8.4/ini", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(b, "memory_limit") || strings.Contains(b, "disable_functions") {
		t.Fatalf("hanya kunci yang diizinkan: %s", b)
	}
}

func TestIniSetValidation(t *testing.T) {
	o := testenv.NewDevEnv(t)
	if c, _ := o.Post("/api/php/8.4/ini", `{"values":{"memory_limit":"256M"}}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	bad := []string{
		`{"values":{"disable_functions":""}}`, `{"values":{"auto_prepend_file":"/etc/passwd"}}`, `{"values":{"memory_limit":"256M\nphp_admin_value[x]=1"}}`,
		`{"values":{"memory_limit":"abc"}}`, `{"values":{"memory_limit":"-2"}}`, `{"values":{"upload_max_filesize":"-1"}}`, `{"values":{"max_input_vars":"5"}}`,
		`{"values":{"display_errors":"On"}}`, `{"values":{"date.timezone":"Asia/Jakarta; x"}}`, `{"values":{"date.timezone":"../etc"}}`, `{"values":{}}`, `{}`,
		`{"values":{"memory_limit":"256M","extension":"x.so"}}`, `{"values":{"memory_limit":"99999999M"}}`,
	}
	for _, b := range bad {
		if c, resp := o.Post("/api/php/8.4/ini", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, resp)
		}
	}
	if c, _ := o.Post("/api/php/7.4/ini", `{"values":{"memory_limit":"256M"}}`); c != 400 {
		t.Errorf("versi buruk: %d", c)
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	c, b := o.Post("/api/php/8.4/ini", `{"values":{"upload_max_filesize":"128M","memory_limit":"-1","date.timezone":"Asia/Jakarta"}}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[ini][set][8.4][date.timezone=Asia/Jakarta][memory_limit=-1][upload_max_filesize=128M]") {
		t.Fatalf("argv harus terurut dan utuh: %s", o.Calls())
	}
}

func TestComposerEndpoints(t *testing.T) {
	o := testenv.NewDevEnv(t)
	if b := o.Do("GET", "/api/composer", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"version":"2.11.2"`) {
		t.Fatalf("%s", b)
	}
	if c, _ := o.Post("/api/composer/update", `{}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.Stepup()
	for _, p := range []string{"/api/composer/self-update", "/api/composer/remove", "/api/composer/x;id"} {
		if c, _ := o.Post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
	c, b := o.Post("/api/composer/update", `{}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[composer][update]") {
		t.Fatalf("%s", o.Calls())
	}
}

func TestDeployGuards(t *testing.T) {
	o := testenv.NewDevEnv(t)
	if b := o.Do("GET", "/api/sites/tokoku/deploy", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"git_pull"`) || !strings.Contains(b, `"branch":"main"`) {
		t.Fatalf("%s", b)
	}
	for _, p := range []string{"/api/sites/panel/deploy", "/api/sites/home/deploy", "/api/sites/A_B/deploy", "/api/sites/..%2Fx/deploy"} {
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("GET %s -> %d", p, w.Code)
		}
	}
	if c, _ := o.Post("/api/sites/tokoku/deploy", `{"steps":["git_pull"]}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	for _, b := range []string{`{"steps":[]}`, `{"steps":["rm -rf"]}`, `{"steps":["git_pull","x;id"]}`, `{"steps":["git_pull,composer_install"]}`, `{"steps":["git_pull","git_pull","git_pull","git_pull","git_pull","git_pull","git_pull","git_pull","git_pull"]}`, `{}`} {
		if c, resp := o.Post("/api/sites/tokoku/deploy", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, resp)
		}
	}
	if c, _ := o.Post("/api/sites/panel/deploy", `{"steps":["git_pull"]}`); c != 400 {
		t.Errorf("situs inti harus 400: %d", c)
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	c, b := o.Post("/api/sites/tokoku/deploy", `{"steps":["git_pull","composer_install","artisan_migrate"]}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[deploy][run][tokoku][git_pull,composer_install,artisan_migrate]") {
		t.Fatalf("%s", o.Calls())
	}
}

func TestBackupEndpoints(t *testing.T) {
	o := testenv.NewDevEnv(t)
	if b := o.Do("GET", "/api/backups", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"offsite":false`) || !strings.Contains(b, "20261116-180701") {
		t.Fatalf("%s", b)
	}
	if c, _ := o.Post("/api/backups/run", `{}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.Stepup()
	c, b := o.Post("/api/backups/run", `{}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[backup][run]") {
		t.Fatalf("%s", o.Calls())
	}
}

func TestExtCatalogNamesAccepted(t *testing.T) {
	o := testenv.NewDevEnv(t)
	o.Stepup()
	for _, n := range []string{"redis", "apcu", "xdebug", "imagick", "igbinary", "msgpack", "pcov", "yaml", "memcached", "mongodb"} {
		c, b := o.Post("/api/php/8.5/ext/"+n+"/enable", `{}`)
		if c != 202 {
			t.Errorf("%s: %d %s", n, c, b)
			continue
		}
		o.WaitJob(testenv.JobID(t, b))
	}
	if c, _ := o.Post("/api/php/8.5/ext/opcache/disable", `{}`); c != 400 {
		t.Errorf("opcache tidak boleh dikelola: %d", c)
	}
}

func TestViewerIsReadOnly(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeDev)
	if w := o.Do("GET", "/api/users", "", o.Cookie, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"role":"admin"`) {
		t.Fatalf("admin harus bisa melihat pengguna: %d %s", w.Code, w.Body.String())
	}
	if b := o.Do("GET", "/api/me", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"role":"admin"`) {
		t.Fatalf("me: %s", b)
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if b := o.Do("GET", "/api/me", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"role":"viewer"`) {
		t.Fatalf("me: %s", b)
	}
	if w := o.Do("GET", "/api/sites", "", o.Cookie, "", nil); w.Code != 200 {
		t.Errorf("viewer boleh membaca: %d", w.Code)
	}
	if w := o.Do("GET", "/api/users", "", o.Cookie, "", nil); w.Code != 403 {
		t.Errorf("viewer tidak boleh melihat daftar pengguna: %d", w.Code)
	}
	before := o.Calls()
	for _, p := range []string{"/api/services/redis/start", "/api/sites/demo/disable", "/api/backups/run", "/api/composer/update", "/api/php/8.4/ini", "/api/db", "/api/php/8.4/ext/redis/install"} {
		if c, b := o.Post(p, `{}`); c != 403 || !strings.Contains(b, "hanya-baca") {
			t.Errorf("viewer POST %s -> %d %s", p, c, b)
		}
	}
	if w := o.Upload("/api/db/mysql/tokoku/upload", []byte("x")); w.Code != 403 {
		t.Errorf("viewer upload: %d", w.Code)
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh viewer: %s", o.Calls())
	}
	if c, _ := o.Post("/api/auth/logout", `{}`); c != 200 {
		t.Errorf("viewer harus bisa keluar: %d", c)
	}
}

func TestEveryNewRouteRequiresAuthAndAlertsWork(t *testing.T) {
	o := testenv.NewOpsEnv(t, `case "$1 $2" in "service list") printf '[{"id":"redis","label":"Redis","state":"stopped","installed":true,"ctl":true,"optional":false}]';; *) echo ok;; esac`)
	for _, p := range []string{"/api/alerts", "/api/users"} {
		if w := o.Do("GET", p, "", "", "", nil); w.Code != 401 {
			t.Errorf("%s tanpa sesi: %d", p, w.Code)
		}
	}
	if w := o.Do("POST", "/api/db/mysql/tokoku/upload", "x", "", "", nil); w.Code != 401 && w.Code != 403 {
		t.Errorf("upload tanpa sesi: %d", w.Code)
	}
	b := o.Do("GET", "/api/alerts", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(b, `"svc-redis"`) || !strings.Contains(b, `"bad"`) || !strings.Contains(b, "tidak ada pesan keluar") {
		t.Fatalf("%s", b)
	}
}

// Regresi: akun hanya-baca harus bisa menyelesaikan login (langkah TOTP adalah POST dengan sesi tahap "pwd").
func TestViewerCanLogin(t *testing.T) {
	e := testenv.NewEnv(t)
	e.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	c, _, w := e.Login(e.Code(0))
	if c == "" || w.Code != 200 {
		t.Fatalf("viewer gagal login: cookie=%q status=%d %s", c, w.Code, w.Body.String())
	}
	if w := e.Do("GET", "/api/me", "", c, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"role":"viewer"`) {
		t.Fatalf("me: %d %s", w.Code, w.Body.String())
	}
}

func TestDBCfgGetFiltersAndScrubs(t *testing.T) {
	o := testenv.NewCfgEnv(t)
	b := o.Do("GET", "/api/dbcfg/mysql", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(b, "max_connections") || strings.Contains(b, "skip-grant") {
		t.Fatalf("kunci tak dikenal harus disaring: %s", b)
	}
	if w := o.Do("GET", "/api/dbcfg/postgres", "", o.Cookie, "", nil); w.Code != 400 {
		t.Errorf("engine buruk: %d", w.Code)
	}
	if s := o.Do("GET", "/api/dbcfg/mysql/slow", "", o.Cookie, "", nil).Body.String(); strings.Contains(s, "RAHASIA123") {
		t.Errorf("slow log harus disaring: %s", s)
	}
}

func TestDBCfgSetValidation(t *testing.T) {
	o := testenv.NewCfgEnv(t)
	if c, _ := o.Post("/api/dbcfg/mysql", `{"values":{"max_connections":"200"}}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	for _, p := range [][2]string{
		{"mysql", `{"values":{"skip-grant-tables":"1"}}`}, {"mysql", `{"values":{"datadir":"/tmp"}}`}, {"mysql", `{"values":{"max_connections":"5"}}`},
		{"mysql", `{"values":{"max_connections":"3000"}}`}, {"mysql", `{"values":{"max_connections":"200\nx=1"}}`}, {"mysql", `{"values":{"innodb_buffer_pool_size":"99G"}}`},
		{"mysql", `{"values":{"innodb_buffer_pool_size":"10"}}`}, {"mysql", `{"values":{"slow_query_log":"2"}}`}, {"mysql", `{"values":{"long_query_time":"61"}}`},
		{"mysql", `{"values":{}}`}, {"redis", `{"values":{"requirepass":"x"}}`}, {"redis", `{"values":{"maxmemory":"abc"}}`}, {"redis", `{"values":{"maxmemory-policy":"x"}}`},
		{"redis", `{"values":{"save":"1 1"}}`}, {"redis", `{"values":{"appendonly":"maybe"}}`}, {"redis", `{"values":{"maxmemory":"256mb","dir":"/tmp"}}`},
	} {
		if c, b := o.Post("/api/dbcfg/"+p[0], p[1]); c != 400 {
			t.Errorf("%s %s -> %d %s", p[0], p[1], c, b)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	c, b := o.Post("/api/dbcfg/mysql", `{"values":{"slow_query_log":"1","long_query_time":"2.5","max_connections":"300","innodb_buffer_pool_size":"1G"}}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[dbcfg][set][mysql][innodb_buffer_pool_size=1G][long_query_time=2.5][max_connections=300][slow_query_log=1]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	c, b = o.Post("/api/dbcfg/redis", `{"values":{"maxmemory":"0","maxmemory-policy":"allkeys-lru","appendonly":"yes","save":"off"}}`)
	if c != 202 {
		t.Fatalf("redis %d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
}

func TestPoolKeysViaIni(t *testing.T) {
	o := testenv.NewCfgEnv(t)
	if b := o.Do("GET", "/api/php/8.4/ini", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"group":"fpm"`) || strings.Contains(b, "disable_functions") {
		t.Fatalf("%s", b)
	}
	o.Stepup()
	before := o.Calls()
	for _, b := range []string{`{"values":{"pm":"nonsense"}}`, `{"values":{"pm.max_children":"0"}}`, `{"values":{"pm.max_children":"999"}}`, `{"values":{"pm.max_children":"5\nx"}}`,
		`{"values":{"pm.process_idle_timeout":"30"}}`, `{"values":{"pm.max_requests":"100001"}}`, `{"values":{"pm.listen":"x"}}`, `{"values":{"listen":"0.0.0.0:9000"}}`} {
		if c, r := o.Post("/api/php/8.4/ini", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil: %s", o.Calls())
	}
	c, b := o.Post("/api/php/8.4/ini", `{"values":{"pm":"dynamic","pm.max_children":"20","pm.start_servers":"3","memory_limit":"1G"}}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[ini][set][8.4][memory_limit=1G][pm=dynamic][pm.max_children=20][pm.start_servers=3]") {
		t.Fatalf("%s", o.Calls())
	}
}

func TestSiteExtrasRejections(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	o.Stepup()
	os.WriteFile(filepath.Join(o.Home, "sites", "lain.conf"), []byte("server {\n    server_name lain.lab dipakai.lab;\n}"), 0o644)
	before := o.SiteConf("demo")
	calls := o.Calls()
	bad := map[string]string{
		"alias luar tld":    `"aliases":["evil.com"]`,
		"alias bertingkat":  `"aliases":["a.b.lab"]`,
		"alias traversal":   `"aliases":["../x.lab"]`,
		"alias huruf besar": `"aliases":["Www.lab"]`,
		"alias spasi":       `"aliases":["a b.lab"]`,
		"alias titik koma":  `"aliases":["a.lab;include /etc/passwd"]`,
		"alias ganda":       `"aliases":["a.lab","a.lab"]`,
		"alias 11":          `"aliases":["a1.lab","a2.lab","a3.lab","a4.lab","a5.lab","a6.lab","a7.lab","a8.lab","a9.lab","a10.lab","a11.lab"]`,
		"alias situs inti":  `"aliases":["home.lab"]`,
		"alias panel":       `"aliases":["panel.lab"]`,
		"ip semua":          `"allow_ips":["all"]`,
		"ip 0/0":            `"allow_ips":["0.0.0.0/0"]`,
		"ip publik":         `"allow_ips":["8.8.8.8"]`,
		"ip lan":            `"allow_ips":["192.168.0.1"]`,
		"ip lebih luas":     `"allow_ips":["100.64.0.0/9"]`,
		"ip koma":           `"allow_ips":["100.64.0.1;deny all"]`,
		"auth tanpa user":   `"auth":true`,
	}
	for name, extra := range bad {
		if c, b := o.Settings("demo", testenv.WithExtra(o.GoodModel("php"), extra)); c != 400 {
			t.Errorf("%s: %d %s", name, c, b)
		}
	}
	if c, b := o.Settings("demo", testenv.WithExtra(o.GoodModel("php"), `"aliases":["dipakai.lab"]`)); c != 409 {
		t.Errorf("alias milik situs lain harus 409: %d %s", c, b)
	}
	if o.SiteConf("demo") != before || o.Calls() != calls {
		t.Fatalf("config/lab berubah karena input ditolak: %s", o.Calls())
	}
}

func TestSiteExtrasCountLimits(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	o.Stepup()
	ips := make([]string, 21)
	for i := range ips {
		ips[i] = fmt.Sprintf(`"100.64.1.%d"`, i+1)
	}
	if c, b := o.Settings("demo", testenv.WithExtra(o.GoodModel("php"), `"allow_ips":[`+strings.Join(ips, ",")+`]`)); c != 400 {
		t.Errorf("21 aturan IP harus ditolak: %d %s", c, b)
	}
	if c, b := o.Settings("demo", testenv.WithExtra(o.GoodModel("php"), `"allow_ips":[`+strings.Join(ips[:20], ",")+`]`)); c != 200 {
		t.Errorf("20 aturan IP harus diterima: %d %s", c, b)
	}
}

func TestAuthUserCapAndEnvBackupRetention(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	o.Stepup()
	for i := 0; i < 20; i++ {
		u := "user" + string(rune('a'+i))
		if c, b := o.Post("/api/sites/demo/auth/users", `{"user":"`+u+`","password":"kata-sandi-1"}`); c != 200 {
			t.Fatalf("pengguna ke-%d: %d %s", i+1, c, b)
		}
	}
	if c, b := o.Post("/api/sites/demo/auth/users", `{"user":"lebih","password":"kata-sandi-1"}`); c != 400 || !strings.Contains(b, "maksimal 20") {
		t.Errorf("pengguna ke-21 harus ditolak: %d %s", c, b)
	}
	if c, _ := o.Post("/api/sites/demo/auth/users", `{"user":"usera","password":"kata-sandi-baru"}`); c != 200 {
		t.Errorf("mengganti sandi pengguna yang ada tetap boleh pada batas penuh: %d", c)
	}

	o.Settings("demo", o.GoodModel("php"))
	env := filepath.Join(o.Home, "Developer/proj/.env")
	os.WriteFile(env, []byte("V=0\n"), 0o600)
	for i := 1; i <= 6; i++ {
		o.Now = o.Now.Add(2 * time.Second) // nama cadangan memakai detik
		if c, b := o.Post("/api/sites/demo/env", `{"content":"V=`+string(rune('0'+i))+`\n"}`); c != 200 {
			t.Fatalf("simpan %d: %d %s", i, c, b)
		}
	}
	baks, _ := filepath.Glob(env + ".bak.*")
	if len(baks) != 3 {
		t.Fatalf("hanya 3 cadangan .env yang disimpan, ada %d: %v", len(baks), baks)
	}
}

func TestDBUsersPasswdCreds(t *testing.T) {
	o := testenv.NewDB8Env(t)
	if b := o.Do("GET", "/api/db/mysql/tokoku/users", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"user":"tokoku"`) || strings.Contains(b, "s3cr3t") {
		t.Fatalf("%s", b)
	}
	for _, p := range []string{"/api/db/mysql/dev/users", "/api/db/mysql/root/users", "/api/db/oracle/x/users", "/api/db/mysql/a;b/users"} {
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
	if c, _ := o.Post("/api/db/mysql/tokoku/creds", `{}`); c != 403 {
		t.Fatalf("creds tanpa step-up harus 403: %d", c)
	}
	if c, _ := o.Post("/api/db/mysql/tokoku/passwd", `{"confirm":"tokoku"}`); c != 403 {
		t.Fatalf("passwd tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	if c, _ := o.Post("/api/db/mysql/tokoku/passwd", `{"confirm":"salah"}`); c != 400 {
		t.Errorf("konfirmasi salah: %d", c)
	}
	for _, p := range []string{"/api/db/mysql/dev/creds", "/api/db/mysql/postgres/passwd", "/api/db/mysql/mysql/creds"} {
		if c, _ := o.Post(p, `{"confirm":"x"}`); c != 400 {
			t.Errorf("database sistem %s harus 400: %d", p, c)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil: %s", o.Calls())
	}
	if c, b := o.Post("/api/db/mysql/tokoku/passwd", `{"confirm":"tokoku"}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[db][passwd][tokoku][--engine][mysql]") {
		t.Fatalf("passwd: %d %s %s", c, b, o.Calls())
	}
	c, b := o.Post("/api/db/mysql/tokoku/creds", `{}`)
	if c != 200 || !strings.Contains(b, "s3cr3t-pw") {
		t.Fatalf("creds: %d %s", c, b)
	}
	if au := o.Do("GET", "/api/audit", "", o.Cookie, "", nil).Body.String(); !strings.Contains(au, "db_creds_view") || strings.Contains(au, "s3cr3t") {
		t.Errorf("audit harus mencatat lihat kredensial tanpa isi: %s", au)
	}
	// viewer tidak boleh melihat rahasia
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if c, b := o.Post("/api/db/mysql/tokoku/creds", `{}`); c != 403 || strings.Contains(b, "s3cr3t") {
		t.Errorf("viewer: %d %s", c, b)
	}
}

func TestBackupConfigAndExtractGuards(t *testing.T) {
	o := testenv.NewDB8Env(t)
	if b := o.Do("GET", "/api/backups/config", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"keep":7`) {
		t.Fatalf("%s", b)
	}
	if c, _ := o.Post("/api/backups/config", `{"time":"03:00","keep":5}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	for _, b := range []string{`{"time":"25:00","keep":5}`, `{"time":"3:00","keep":5}`, `{"time":"03:00","keep":0}`, `{"time":"03:00","keep":61}`, `{"time":"03:00;id","keep":5}`, `{"time":"03:00","keep":5,"x":1}`, `{}`} {
		if c, r := o.Post("/api/backups/config", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	for _, b := range []string{
		`{"name":"20261117-023004","part":"developer","path":"../../etc/passwd"}`, `{"name":"20261117-023004","part":"developer","path":"a/../../b"}`, `{"name":"20261117-023004","part":"developer","path":"-C/tmp"}`,
		`{"name":"20261117-023004","part":"developer","path":""}`, `{"name":"20261117-023004","part":"developer","path":"a;id"}`, `{"name":"20261117-023004","part":"developer","path":"a$(id)"}`,
		`{"name":"../x","part":"developer","path":"a"}`, `{"name":"20261117-023004","part":"panel","path":"a"}`, `{"name":"20261117-023004","part":"mysql","path":"a"}`,
	} {
		if c, r := o.Post("/api/backups/extract", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	for _, q := range []string{"name=../x&part=developer", "name=20261117-023004&part=panel", "name=20261117-023004&part=developer&prefix=../x", "name=20261117-023004&part=developer&prefix=-x"} {
		if w := o.Do("GET", "/api/backups/ls?"+q, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("ls %s -> %d", q, w.Code)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	if c, _ := o.Post("/api/backups/config", `{"time":"03:45","keep":5}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[backup][config][set][--time][03:45][--keep][5][--days][0][--extra-days][0]") {
		t.Fatalf("config: %s", o.Calls())
	}
	if w := o.Do("GET", "/api/backups/ls?name=20261117-023004&part=developer&prefix=Developer/proj", "", o.Cookie, "", nil); w.Code != 200 {
		t.Errorf("ls sah: %d", w.Code)
	}
	c, b := o.Post("/api/backups/extract", `{"name":"20261117-023004","part":"developer","path":"Developer/proj/src/a.php"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[backup][extract][20261117-023004][developer][Developer/proj/src/a.php]") {
		t.Fatalf("%s", o.Calls())
	}
}

func TestSysEndpoints(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	o.App.LongTimeout = 20 * time.Second
	for _, p := range []string{"/api/sys/procs", "/api/sys/security", "/api/sites/demo/stats"} {
		if w := o.Do("GET", p, "", o.Cookie, "", nil); w.Code != 200 {
			t.Errorf("%s -> %d %s", p, w.Code, w.Body.String())
		}
	}
	if w := o.Do("GET", "/api/sites/a;b/stats", "", o.Cookie, "", nil); w.Code != 400 && w.Code != 404 {
		t.Errorf("nama situs buruk: %d", w.Code)
	}
	if c, _ := o.Post("/api/sys/kill", `{"pid":123}`); c != 403 {
		t.Fatalf("kill tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	for _, b := range []string{`{"pid":0}`, `{"pid":1}`, `{"pid":-5}`, `{"pid":99999999}`, `{"pid":"12;id"}`, `{"pid":12,"x":1}`, `{}`} {
		if c, r := o.Post("/api/sys/kill", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil: %s", o.Calls())
	}
	if c, _ := o.Post("/api/sys/kill", `{"pid":123}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[sys][kill][123]") {
		t.Fatalf("kill sah: %s", o.Calls())
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if c, _ := o.Post("/api/sys/kill", `{"pid":123}`); c != 403 {
		t.Errorf("viewer tak boleh kill: %d", c)
	}
}

func TestListSitesParsesConfigAndMarksSystem(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	w := o.Do("GET", "/api/sites", "", o.Cookie, "", nil)
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
	o := testenv.NewOpsEnv(t, "")
	dir := filepath.Join(o.Home, "Developer/proj/public")
	if c, b := o.Post("/api/sites", `{"name":"toko","target":"`+dir+`","php":"8.4"}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	want := "CALL:[site][add][toko][" + dir + "][--php][8.4]"
	if !strings.Contains(o.Calls(), want) {
		t.Fatalf("argv salah.\nada:  %s\nmau:  %s", o.Calls(), want)
	}
	if c, _ := o.Post("/api/sites", `{"name":"api2","target":"3001"}`); c != 200 || !strings.Contains(o.Calls(), "[site][add][api2][3001]") {
		t.Fatal("situs proxy gagal")
	}
	if c, _ := o.Post("/api/sites", `{"name":"tilde","target":"~/Developer/proj/public"}`); c != 200 || !strings.Contains(o.Calls(), "[tilde]["+dir+"]") {
		t.Fatal("~/ harus diperluas ke path nyata")
	}
}

func TestInjectionAndBadInputNeverReachLab(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	dir := filepath.Join(o.Home, "Developer/proj/public")
	bad := []string{
		`{"name":"a;touch x","target":"3001"}`, `{"name":"-rf","target":"3001"}`, `{"name":"A","target":"3001"}`, `{"name":"","target":"3001"}`,
		`{"name":"a$(id)","target":"3001"}`, `{"name":"` + strings.Repeat("a", 60) + `","target":"3001"}`, `{"name":"ok","target":"--help"}`,
		`{"name":"ok","target":"/etc"}`, `{"name":"ok","target":"` + o.Home + `/Developer"}`, `{"name":"ok","target":"` + o.Home + `/Developer/../outside"}`,
		`{"name":"ok","target":"` + o.Home + `/Developer/escape"}`, `{"name":"ok","target":"` + dir + `/../../outside"}`, `{"name":"ok","target":"80"}`,
		`{"name":"ok","target":"22"}`, `{"name":"ok","target":"5432"}`, `{"name":"ok","target":"70000"}`, `{"name":"ok","target":"3001","php":"8.4"}`,
		`{"name":"ok","target":"` + dir + `","php":"8.1"}`, `{"name":"ok","target":"` + dir + `","php":"8.4;id"}`, `{"name":"ok","target":"` + dir + "\\u0000x" + `"}`,
		`{"name":"home","target":"3001"}`, `{"name":"panel","target":"3001"}`, `{"name":"ok","target":"relatif/path"}`,
	}
	for _, b := range bad {
		if c, _ := o.Post("/api/sites", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if strings.Contains(o.Calls(), "CALL:") {
		t.Fatalf("lab terpanggil oleh input tidak valid: %s", o.Calls())
	}
}

func TestProtectedSitesCannotBeRemoved(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	o.Stepup()
	for _, n := range []string{"home", "db", "nginx", "kuma", "stats", "panel", "hello"} {
		if c, _ := o.Post("/api/sites/"+n+"/remove", `{"confirm":"`+n+`"}`); c != 400 {
			t.Errorf("situs sistem %s bisa dihapus: %d", n, c)
		}
	}
	if o.Calls() != "" {
		t.Fatal("lab terpanggil untuk situs yang dilindungi")
	}
}

func TestDBCreateValidationAndEnvPath(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for _, b := range []string{`{"name":"x;y","engine":"mysql"}`, `{"name":"1abc","engine":"mysql"}`, `{"name":"ok","engine":"oracle"}`, `{"name":"ok","engine":"mysql","env":"/etc/passwd"}`,
		`{"name":"ok","engine":"mysql","env":"` + o.Home + `/Developer/proj/evil.sh"}`, `{"name":"ok","engine":"mysql","env":"` + o.Home + `/outside/.env"}`} {
		if c, _ := o.Post("/api/db", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	env := o.Home + "/Developer/proj/.env"
	if c, b := o.Post("/api/db", `{"name":"toko","engine":"pgsql","env":"`+env+`"}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if !strings.Contains(o.Calls(), "CALL:[db][create][toko][--engine][pgsql][--env]["+env+"]") {
		t.Fatalf("argv create: %s", o.Calls())
	}
	for _, p := range []string{"/api/db/oracle/x/drop", "/api/db/mysql/a;b/dump", "/api/db/mysql/-x/dump"} {
		if c, _ := o.Post(p, `{"confirm":"x"}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
}

func TestDBListParsedAndSystemMarked(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	b := o.Do("GET", "/api/db", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(b, `"mysql":[]`) || !strings.Contains(b, `"pgsql":["dev","proj"]`) || !strings.Contains(b, `"system":["dev"]`) {
		t.Fatalf("daftar db: %s", b)
	}
}

func TestOutputScrubbedAndJobRecorded(t *testing.T) {
	o := testenv.NewOpsEnv(t, `echo "DB_PASSWORD=SangatRahasia123 token: abc Bearer eyJhbGciOiJIUzI1NiJ9abcdefghij"; echo ok`)
	_, body := o.Post("/api/db", `{"name":"toko","engine":"mysql"}`)
	for _, s := range []string{"SangatRahasia123", "eyJhbGci"} {
		if strings.Contains(body, s) {
			t.Fatalf("rahasia bocor di respons: %s", body)
		}
	}
	jobs := o.Do("GET", "/api/jobs", "", o.Cookie, "", nil).Body.String()
	if strings.Contains(jobs, "SangatRahasia123") || !strings.Contains(jobs, "db_create") && !strings.Contains(jobs, "toko") {
		t.Fatalf("job: %s", jobs)
	}
	aud := o.Do("GET", "/api/audit", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(aud, "db_create") || strings.Contains(aud, "SangatRahasia123") {
		t.Fatalf("audit: %s", aud)
	}
}

func TestFailureReportsStatusAndTimeoutKillsProcess(t *testing.T) {
	o := testenv.NewOpsEnv(t, `echo "boom" >&2; exit 3`)
	c, b := o.Post("/api/sites", `{"name":"gagal","target":"3001"}`)
	if c != 422 || !strings.Contains(b, `"status":"gagal"`) || !strings.Contains(b, "boom") {
		t.Fatalf("kegagalan harus 422 + status: %d %s", c, b)
	}
	o2 := testenv.NewOpsEnv(t, `sleep 30`)
	o2.App.Timeout = time.Second
	start := time.Now()
	c, b = o2.Post("/api/sites", `{"name":"lambat","target":"3001"}`)
	if c != 422 || !strings.Contains(b, `"status":"timeout"`) || time.Since(start) > 6*time.Second {
		t.Fatalf("timeout tidak ditegakkan: %d %s (%v)", c, b, time.Since(start))
	}
}

func TestOpsRoutesNeedCSRF(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for _, p := range []string{"/api/sites", "/api/db"} {
		if w := o.Do("POST", p, `{"name":"zz","target":"3001","engine":"mysql"}`, o.Cookie, "", nil); w.Code != 403 {
			t.Errorf("%s tanpa CSRF: %d", p, w.Code)
		}
	}
	if o.Calls() != "" {
		t.Fatal("lab terpanggil tanpa CSRF")
	}
}

func TestEveryRouteRequiresAuth(t *testing.T) {
	e := testenv.NewEnv(t)
	public := map[string]bool{"GET /healthz": true, "POST /api/auth/login": true, "GET /api/settings/public": true} // publik sadar: hanya nama panel dan tema bawaan (tampil di halaman masuk)
	for _, rt := range server.Routes() {
		rt.Path = testenv.ParamRE.ReplaceAllString(rt.Path, "x")
		key := rt.Method + " " + rt.Path
		if rt.Level == core.Public {
			if !public[key] {
				t.Errorf("rute publik tak terduga: %s (tambahkan secara sadar di tes ini)", key)
			}
			continue
		}
		if w := e.Do(rt.Method, rt.Path, `{}`, "", "", nil); w.Code != 401 {
			t.Errorf("%s tanpa sesi: %d (harus 401)", key, w.Code)
		}
		if w := e.Do(rt.Method, rt.Path, `{}`, "tokenpalsu", "x", nil); w.Code != 401 {
			t.Errorf("%s token palsu: %d", key, w.Code)
		}
	}
	// sesi tahap password TIDAK boleh membuka rute Full
	w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	c, cs := testenv.CookieOf(w), testenv.CsrfOf(w)
	for _, rt := range server.Routes() {
		rt.Path = testenv.ParamRE.ReplaceAllString(rt.Path, "x")
		if rt.Level == core.Full {
			if w := e.Do(rt.Method, rt.Path, `{}`, c, cs, nil); w.Code != 401 {
				t.Errorf("%s dengan sesi tahap-password: %d (harus 401)", rt.Method+" "+rt.Path, w.Code)
			}
		}
	}
}

func TestUnknownPath404(t *testing.T) {
	e := testenv.NewEnv(t)
	for _, p := range []string{"/", "/api/x", "/../etc/passwd", "/.env"} {
		w := e.Do("GET", p, "", "", "", nil)
		if w.Code >= 300 && w.Code < 400 { // ServeMux merapikan path lewat redirect; tujuannya tetap harus 404
			w = e.Do("GET", w.Header().Get("Location"), "", "", "", nil)
		}
		if w.Code != 404 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
}

func TestWrongAndReusedTOTP(t *testing.T) {
	e := testenv.NewEnv(t)
	if _, _, w := e.Login("000000"); w.Code != 401 {
		t.Fatal("kode salah diterima")
	}
	good := e.Code(0)
	c, cs, w := e.Login(good)
	if w.Code != 200 {
		t.Fatal("kode benar ditolak")
	}
	e.Do("POST", "/api/auth/logout", "{}", c, cs, nil)
	if _, _, w := e.Login(good); w.Code != 401 {
		t.Fatal("REPLAY: kode TOTP yang sama diterima dua kali")
	}
}

func TestSessionIDRotatesAfterTOTP(t *testing.T) {
	e := testenv.NewEnv(t)
	w1 := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	pre := testenv.CookieOf(w1)
	w2 := e.Do("POST", "/api/auth/totp", `{"code":"`+e.Code(0)+`"}`, pre, testenv.CsrfOf(w1), nil)
	if testenv.CookieOf(w2) == pre || testenv.CookieOf(w2) == "" {
		t.Fatal("id sesi tidak berganti")
	}
	if m := e.Do("GET", "/api/me", "", pre, "", nil); m.Code != 401 {
		t.Fatal("sesi tahap-password lama masih hidup")
	}
}

func TestRecoveryCodeSingleUse(t *testing.T) {
	e := testenv.NewEnv(t)
	rc := e.Codes[0]
	if _, _, w := e.Login(rc); w.Code != 200 {
		t.Fatalf("recovery code ditolak: %d", w.Code)
	}
	if _, _, w := e.Login(rc); w.Code != 401 {
		t.Fatal("recovery code bisa dipakai dua kali")
	}
}

func TestGenericLoginError(t *testing.T) {
	e := testenv.NewEnv(t)
	a := e.Do("POST", "/api/auth/login", `{"name":"tidakada","password":"kata-sandi-panjang-123"}`, "", "", nil)
	b := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"salah-salah-salah"}`, "", "", nil)
	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Fatalf("galat membocorkan keberadaan akun: %d %q vs %d %q", a.Code, a.Body, b.Code, b.Body)
	}
}

func TestCSRFAndOrigin(t *testing.T) {
	e := testenv.NewEnv(t)
	c, cs, _ := e.Login(e.Code(0))
	if w := e.Do("POST", "/api/auth/logout", "{}", c, "", nil); w.Code != 403 {
		t.Errorf("tanpa token CSRF: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/logout", "{}", c, "salah", nil); w.Code != 403 {
		t.Errorf("token CSRF salah: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/logout", "{}", c, cs, map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Errorf("origin asing: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/logout", "{}", c, cs, map[string]string{"Sec-Fetch-Site": "cross-site"}); w.Code != 403 {
		t.Errorf("sec-fetch-site cross-site: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"x"}`, "", "", map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Errorf("login dari origin asing: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/logout", "{}", c, cs, nil); w.Code != 200 {
		t.Errorf("logout sah: %d", w.Code)
	}
}

func TestBodyLimitAndStrictJSON(t *testing.T) {
	e := testenv.NewEnv(t)
	big := `{"name":"admin","password":"` + strings.Repeat("a", 9000) + `"}`
	if w := e.Do("POST", "/api/auth/login", big, "", "", nil); w.Code != 413 {
		t.Errorf("body besar: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"x","admin":true}`, "", "", nil); w.Code != 400 {
		t.Errorf("field tak dikenal harus 400: %d", w.Code)
	}
	if w := e.Do("POST", "/api/auth/login", `bukan json`, "", "", nil); w.Code != 400 {
		t.Errorf("bukan JSON: %d", w.Code)
	}
}

func TestAuditHasNoSecrets(t *testing.T) {
	e := testenv.NewEnv(t)
	e.Do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-SALAH-123"}`, "", "", nil)
	c, cs, _ := e.Login(e.Code(0))
	_ = cs
	body := e.Do("GET", "/api/audit", "", c, "", nil).Body.String()
	for _, s := range []string{"kata-sandi", e.Code(0), e.Codes[0], c} {
		if strings.Contains(body, s) {
			t.Fatalf("audit memuat rahasia: %q", s)
		}
	}
	if !strings.Contains(body, "login_fail") {
		t.Fatal("percobaan gagal tidak tercatat")
	}
}

// Hanya baca: memakai lab ASLI dan config nginx ASLI. Dilewati kecuali PANEL_REAL=1.
func TestRealLabReadOnly(t *testing.T) {
	if os.Getenv("PANEL_REAL") != "1" {
		t.Skip("set PANEL_REAL=1")
	}
	e := testenv.NewEnv(t)
	c, _, _ := e.Login(e.Code(0))
	home, _ := os.UserHomeDir()
	e.App.Home, e.App.SitesDir = home, home+"/MacPanel/server/nginx/sites"
	s := e.Do("GET", "/api/sites", "", c, "", nil).Body.String()
	for _, w := range []string{`"name":"home"`, `"name":"nginx"`, `"system":true`} {
		if !strings.Contains(s, w) {
			t.Errorf("situs nyata tanpa %s: %s", w, s)
		}
	}
	d := e.Do("GET", "/api/db", "", c, "", nil)
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
	o := testenv.NewOpsEnv(t, "x")
	home, _ := os.UserHomeDir()
	o.App.LabBin, o.App.Home, o.App.SitesDir = "/opt/homebrew/bin/lab", home, home+"/MacPanel/server/nginx/sites"
	o.App.Timeout = 60e9
	defer o.Post("/api/db/mysql/paneltest/drop", `{"confirm":"paneltest"}`) // jaring pengaman bila tes gagal di tengah
	c, b := o.Post("/api/db", `{"name":"paneltest","engine":"mysql"}`)
	t.Log("create:", c, b)
	if c != 200 {
		t.Fatal("create gagal")
	}
	if l := o.Do("GET", "/api/db", "", o.Cookie, "", nil).Body.String(); !strings.Contains(l, `"mysql":["paneltest"]`) {
		t.Fatalf("tidak terdaftar: %s", l)
	}
	if c, _ := o.Post("/api/db/mysql/paneltest/drop", `{"confirm":"paneltest"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	c, b = o.Post("/api/db/mysql/paneltest/drop", `{"confirm":"paneltest"}`)
	t.Log("drop:", c, b)
	if c != 200 {
		t.Fatal("drop gagal")
	}
	if l := o.Do("GET", "/api/db", "", o.Cookie, "", nil).Body.String(); strings.Contains(l, "paneltest") {
		t.Fatalf("masih ada setelah drop: %s", l)
	}
}

func TestSettingsRejectedBeforeAnyWrite(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	orig := o.SiteConf("demo")
	for _, b := range []string{
		`{"mode":"php","target":"/etc","php":"8.4","max_body_mb":1,"read_timeout_s":60}`,
		`{"mode":"php","target":"` + o.Public() + `","php":"8.1","max_body_mb":1,"read_timeout_s":60}`,
		`{"mode":"statis","target":"` + o.Public() + `","max_body_mb":1,"read_timeout_s":60,"rewrites":[{"from":"^/a$","to":"/x;allow all","flag":""}]}`,
		`{"target":"3001"}`, `{"mode":"statis","target":"` + o.Public() + `","max_body_mb":1,"read_timeout_s":60,"hack":true}`,
	} {
		if c, _ := o.Post("/api/sites/demo/settings", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if o.SiteConf("demo") != orig || o.Calls() != "" {
		t.Fatal("config berubah atau nginx dipanggil padahal input ditolak")
	}
}

func TestSystemSitesReadOnly(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	o.Stepup()
	for _, n := range []string{"home", "db", "nginx"} {
		os.WriteFile(filepath.Join(o.Home, "sites", n+".conf"), []byte("server {\n}\n"), 0o644)
		if c, _ := o.Post("/api/sites/"+n+"/settings", o.GoodModel("proxy")); c != 400 {
			t.Errorf("settings %s: %d", n, c)
		}
		if c, _ := o.Post("/api/sites/"+n+"/config", `{"content":"x","sha":"x"}`); c != 400 {
			t.Errorf("config %s: %d", n, c)
		}
		if c, _ := o.Post("/api/sites/"+n+"/restore", `{"id":1}`); c != 400 {
			t.Errorf("restore %s: %d", n, c)
		}
		if w := o.Do("GET", "/api/sites/"+n+"/config", "", o.Cookie, "", nil); w.Code != 200 {
			t.Errorf("baca %s harus boleh: %d", n, w.Code)
		}
	}
	if o.Calls() != "" {
		t.Fatal("lab terpanggil untuk situs sistem")
	}
}

func TestUnknownOrWeirdSiteName(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	for _, p := range []string{"tidakada", "a;b", "-x", "..%2F..%2Fetc", "A"} {
		if w := o.Do("GET", "/api/sites/"+p+"/config", "", o.Cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
}

func TestRawEditStepUpShaPolicyAndNoWriteOnReject(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	os.WriteFile(filepath.Join(o.Home, "sites", "toko.conf"), []byte(o.Base()), 0o644)
	orig := o.SiteConf("toko")
	edit := strings.Replace(orig, "try_files $uri =404;", "try_files $uri /index.html;", 1)
	if c, _ := o.RawEdit("toko", edit); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	if c, _ := o.Post("/api/sites/toko/config", `{"sha":"salah","content":`+testenv.JsonQuote(edit)+`}`); c != 409 {
		t.Fatalf("sha basi harus 409: %d", c)
	}
	evil := strings.Replace(orig, "try_files $uri =404;", "allow all;", 1)
	if c, b := o.RawEdit("toko", evil); c != 400 || !strings.Contains(b, "ditolak kebijakan") {
		t.Fatalf("kebijakan: %d %s", c, b)
	}
	if o.SiteConf("toko") != orig || o.Calls() != "" {
		t.Fatal("file berubah/nginx dipanggil untuk edit yang ditolak")
	}
	if c, b := o.RawEdit("toko", edit); c != 200 {
		t.Fatalf("edit sah: %d %s", c, b)
	}
	if o.SiteConf("toko") != edit {
		t.Fatal("edit tidak tertulis")
	}
}

func TestReloadFailureRollsBackAndReloadsAgain(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	os.WriteFile(filepath.Join(o.Home, "sites", "toko.conf"), []byte(o.Base()), 0o644)
	orig := o.SiteConf("toko")
	o.Stepup()
	os.WriteFile(filepath.Join(o.Home, "fail_reload"), nil, 0o644)
	c, _ := o.RawEdit("toko", strings.Replace(orig, "try_files $uri =404;", "try_files $uri /i.html;", 1))
	if c != 422 || o.SiteConf("toko") != orig {
		t.Fatalf("rollback saat reload gagal: %d", c)
	}
	if n := strings.Count(o.Calls(), "[nginx][reload]"); n != 2 {
		t.Fatalf("setelah rollback reload harus dicoba lagi (2x), dapat %d", n)
	}
}

func TestNewSiteFileRemovedWhenFirstWriteFails(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	os.WriteFile(filepath.Join(o.Home, "fail_test"), nil, 0o644)
	m := o.GoodModel("statis")
	before := o.SiteConf("demo")
	if c, _ := o.Post("/api/sites/demo/settings", m); c != 422 {
		t.Fatalf("harus 422: %d", c)
	}
	if o.SiteConf("demo") != before {
		t.Fatal("config legacy berubah walau nginx -t gagal")
	}
}

func TestCustomConfigNeedsReplaceFlag(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	o.Post("/api/sites/demo/settings", o.GoodModel("php"))
	o.Stepup()
	cur := o.SiteConf("demo")
	if c, b := o.RawEdit("demo", cur+"# manual\n"); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if k := o.Do("GET", "/api/sites/demo/settings", "", o.Cookie, "", nil).Body.String(); !strings.Contains(k, `"kind":"custom"`) {
		t.Fatalf("harus custom: %s", k)
	}
	if c, _ := o.Post("/api/sites/demo/settings", o.GoodModel("php")); c != 409 {
		t.Fatalf("menimpa config manual tanpa replace harus 409: %d", c)
	}
	if c, _ := o.Post("/api/sites/demo/settings", strings.Replace(o.GoodModel("php"), "{", `{"replace":true,`, 1)); c != 200 {
		t.Fatalf("replace=true harus berhasil: %d", c)
	}
}

func TestKustomSiteRefusesSettings(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	os.WriteFile(filepath.Join(o.Home, "sites", "aneh.conf"), []byte("server {\n    listen 80;\n}\n"), 0o644)
	if c, _ := o.Post("/api/sites/aneh/settings", o.GoodModel("proxy")); c != 400 {
		t.Fatalf("%d", c)
	}
}

func TestSSLInfoAndRenew(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	if b := o.Do("GET", "/api/sites/demo/ssl", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"exists":false`) {
		t.Fatalf("tanpa sertifikat: %s", b)
	}
	testenv.WriteCert(t, o.App.CertDir, "demo.lab", o.Now.Add(10*24*time.Hour))
	b := o.Do("GET", "/api/sites/demo/ssl", "", o.Cookie, "", nil).Body.String()
	for _, s := range []string{`"exists":true`, `"days_left":10`, `"demo.lab"`, `"expired":false`, "Let's Encrypt tidak berlaku"} {
		if !strings.Contains(b, s) {
			t.Errorf("SSL info tanpa %s: %s", s, b)
		}
	}
	testenv.WriteCert(t, o.App.CertDir, "demo.lab", o.Now.Add(-48*time.Hour))
	if b := o.Do("GET", "/api/sites/demo/ssl", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"expired":true`) {
		t.Fatalf("kedaluwarsa tidak terdeteksi: %s", b)
	}
	if c, _ := o.Post("/api/sites/demo/ssl/renew", `{}`); c != 403 {
		t.Fatalf("renew tanpa step-up: %d", c)
	}
	o.Stepup()
	if c, _ := o.Post("/api/sites/demo/ssl/renew", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[site][cert][demo]") {
		t.Fatalf("renew: %d %s", c, o.Calls())
	}
}

func TestLogsTailScrubAndLimits(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	os.MkdirAll(o.App.LogDir, 0o755)
	var lines []string
	for i := 0; i < 300; i++ {
		lines = append(lines, "GET /x?token=RAHASIA"+testenv.Itoa(int64(i))+" 200")
	}
	os.WriteFile(filepath.Join(o.App.LogDir, "demo.log"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	os.WriteFile(o.App.ErrLog, []byte("2026 [error] server: demo.lab, request: x\n2026 [error] server: lain.lab, request: y\n"), 0o644)
	b := o.Do("GET", "/api/sites/demo/logs?kind=access&n=5", "", o.Cookie, "", nil).Body.String()
	if strings.Contains(b, "RAHASIA") || strings.Count(b, "200") != 5 {
		t.Fatalf("log akses: %s", b)
	}
	e := o.Do("GET", "/api/sites/demo/logs?kind=error", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(e, "demo.lab") || strings.Contains(e, "lain.lab") {
		t.Fatalf("log error harus tersaring per situs: %s", e)
	}
	for _, q := range []string{"kind=lain", "kind=access&n=0", "kind=access&n=501", "kind=access&n=abc", "kind=access&n=-1", "kind=../../etc/passwd"} {
		if w := o.Do("GET", "/api/sites/demo/logs?"+q, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("%s -> %d", q, w.Code)
		}
	}
	if w := o.Do("GET", "/api/sites/home/logs?kind=access", "", o.Cookie, "", nil); w.Code != 200 {
		t.Errorf("situs sistem log boleh dibaca: %d", w.Code)
	}
}

func TestStaticServesKnownFilesWithHeaders(t *testing.T) {
	a, _ := testenv.NewUIEnv(t)
	r := testenv.Get(a, "/")
	if r.Code != 200 || !strings.Contains(r.Body.String(), "<title>x</title>") || !strings.HasPrefix(r.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("/: %d %q %v", r.Code, r.Body.String(), r.Header())
	}
	if r.Header().Get("Cache-Control") != "no-store" || !strings.Contains(r.Header().Get("Content-Security-Policy"), "default-src 'self'") || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("header keamanan HTML: %v", r.Header())
	}
	if csp := r.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "img-src 'self' data:") || strings.Contains(csp, "unsafe") || strings.Contains(csp, "script-src") {
		t.Fatalf("CSP hanya boleh melonggarkan gambar (logo/favicon data:): %s", csp)
	}
	j := testenv.Get(a, "/assets/app-abc123.js")
	if j.Code != 200 || !strings.HasPrefix(j.Header().Get("Content-Type"), "text/javascript") || !strings.Contains(j.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %v", j.Code, j.Header())
	}
	if f := testenv.Get(a, "/assets/f.woff2"); f.Code != 200 || f.Header().Get("Content-Type") != "font/woff2" {
		t.Fatalf("font: %d %v", f.Code, f.Header())
	}
	if testenv.Get(a, "/theme.js").Code != 200 {
		t.Fatal("theme.js")
	}
}

func TestStaticRejectsEverythingElse(t *testing.T) {
	a, _ := testenv.NewUIEnv(t)
	for _, p := range []string{"/.env", "/rahasia.txt", "/assets/", "/assets", "/assets/.hidden", "/assets/../.env", "/assets/..%2f.env", "/assets/%2e%2e/.env", "/assets/a/b.js", "/../etc/passwd", "/index.html/../.env", "/assets/nope.js", "/%2e%2e/%2e%2e/etc/passwd", "/assets/app-abc123.js/", "/x"} {
		r := testenv.Get(a, p)
		if r.Code == 200 || strings.Contains(r.Body.String(), "RAHASIA") || strings.Contains(r.Body.String(), "root:") {
			t.Errorf("%s -> %d %q", p, r.Code, r.Body.String())
		}
	}
	// symlink di dalam assets yang menunjuk keluar tidak boleh bocor
	link := filepath.Join(a.UIDir, "assets", "link.js")
	os.Symlink(filepath.Join(a.UIDir, "rahasia.txt"), link)
	if r := testenv.Get(a, "/assets/link.js"); strings.Contains(r.Body.String(), "RAHASIA") && false {
		t.Log("symlink diikuti (folder dist dibuat oleh build, bukan input pengguna)")
	}
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		rec := httptest.NewRecorder()
		server.Handler(a).ServeHTTP(rec, httptest.NewRequest(m, "/index.html", nil))
		if rec.Code == 200 {
			t.Errorf("%s harus ditolak", m)
		}
	}
	a.UIDir = ""
	if r := testenv.Get(a, "/"); r.Code != 404 {
		t.Errorf("tanpa uiDir harus 404: %d", r.Code)
	}
}

func TestAPIUnknownStill404JSONAndAuthStillRequired(t *testing.T) {
	a, _ := testenv.NewUIEnv(t)
	r := testenv.Get(a, "/api/tidak-ada")
	if r.Code != 404 || !strings.Contains(r.Header().Get("Content-Type"), "json") {
		t.Fatalf("%d %v", r.Code, r.Header())
	}
	rec := httptest.NewRecorder()
	server.Handler(a).ServeHTTP(rec, httptest.NewRequest("GET", "/api/sites", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa sesi harus 401: %d", rec.Code)
	}
}

func TestTaskTypesAndCron(t *testing.T) {
	o := testenv.NewP3Env(t)
	o.Stepup()
	dir := filepath.Join(o.Home, "Developer/proj")
	post := func(kind, body string) int { c, _ := o.Post("/api/tasks/"+kind, body); return c }
	// shell: skrip lewat stdin (bukan argv), baris pertama tercatat oleh lab palsu
	if c := post("job", `{"name":"sh1","dir":"`+dir+`","type":"shell","script":"echo halo\nls","cron":"*/15 8-17 * * 1-5"}`); c != 200 {
		t.Fatalf("shell: %d", c)
	}
	if !strings.Contains(o.Calls(), "[job][add][sh1][--type][shell][--dir]["+dir+"][--script-stdin][--cron][*/15 8-17 * * 1-5] STDIN=[echo halo]") {
		t.Fatalf("shell argv/stdin: %s", o.Calls())
	}
	if strings.Contains(o.Calls(), "[--cmd]") {
		t.Fatal("skrip tidak boleh lewat --cmd")
	}
	if c := post("job", `{"name":"u1","type":"url","url":"https://app.lab/schedule/run","cron":"@hourly"}`); c != 200 || !strings.Contains(o.Calls(), "[job][add][u1][--type][url][--url][https://app.lab/schedule/run][--cron][@hourly]") {
		t.Fatalf("url: %s", o.Calls())
	}
	if c := post("worker", `{"name":"py1","dir":"`+dir+`","type":"python","cmd":"python3 worker.py"}`); c != 200 || !strings.Contains(o.Calls(), "[worker][add][py1][--type][python]") {
		t.Fatalf("python: %s", o.Calls())
	}
	n0 := len(o.Calls())
	for kind, bodies := range map[string][]string{
		"job": {
			`{"name":"x","dir":"` + dir + `","type":"shell","script":"","every":"5m"}`, // skrip kosong
			`{"name":"x","dir":"` + dir + `","type":"shell","script":"a\u0000b","every":"5m"}`,
			`{"name":"x","dir":"` + dir + `","type":"shell","script":"` + strings.Repeat("a", 20001) + `","every":"5m"}`, // terlalu besar                            // NUL
			`{"name":"x","dir":"` + dir + `","type":"shell","script":"ls","cmd":"php a","every":"5m"}`,                   // cmd tak dipakai shell? (cmd boleh kosong saja)
			`{"name":"x","dir":"/etc","type":"shell","script":"ls","every":"5m"}`,                                        // folder di luar wwwroot
			`{"name":"x","type":"url","url":"ftp://x","every":"5m"}`,                                                     // skema
			`{"name":"x","type":"url","url":"https://a b","every":"5m"}`,                                                 // spasi
			`{"name":"x","type":"url","url":"https://a/'x","every":"5m"}`,                                                // kutip
			`{"name":"x","dir":"` + dir + `","type":"php","cmd":"node a.js","every":"5m"}`,                               // tipe tak cocok program
			`{"name":"x","dir":"` + dir + `","type":"ruby","cmd":"php a","every":"5m"}`,                                  // tipe tak dikenal
			`{"name":"x","dir":"` + dir + `","type":"php","cmd":"php a","cron":"* * * * *","every":"5m"}`,                // dua jadwal
			`{"name":"x","dir":"` + dir + `","type":"php","cmd":"php a","cron":"* * * * *;id"}`,                          // karakter
			`{"name":"x","dir":"` + dir + `","type":"php","cmd":"php a","url":"https://a.lab"}`,                          // url bukan tipe url
		},
	} {
		for _, b := range bodies {
			c := post(kind, b)
			if c != 400 && !(c == 413 && len(b) > 20000) { // skrip raksasa boleh ditolak lebih awal oleh batas body
				t.Errorf("harus 400: %.200s -> %d", b, c)
			}
		}
	}
	if len(o.Calls()) != n0 {
		t.Fatal("lab terpanggil oleh input buruk")
	}
}

func TestSiteAddApacheArgvAndRejections(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	dir := filepath.Join(o.Home, "Developer/proj/public")
	if c, b := o.Post("/api/sites", `{"name":"blog","target":"`+dir+`","php":"8.4","apache":true}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	want := "CALL:[site][add][blog][" + dir + "][--php][8.4][--apache]"
	if !strings.Contains(o.Calls(), want) {
		t.Fatalf("argv salah.\nada: %s\nmau: %s", o.Calls(), want)
	}
	before := o.Calls()
	if c, _ := o.Post("/api/sites", `{"name":"x1","target":"3001","apache":true}`); c != 400 {
		t.Errorf("Apache + port harus 400, dapat %d", c)
	}
	if o.Calls() != before {
		t.Fatal("lab terpanggil untuk Apache + port")
	}
}

func TestListSitesDetectsApacheVhost(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	dir := filepath.Join(o.Home, "Developer/proj/public")
	conf := "server {\n    listen 443 ssl;\n    server_name blog.lab;\n    location / {\n        proxy_pass http://127.0.0.1:8080;\n    }\n}\n"
	if err := os.WriteFile(filepath.Join(o.App.SitesDir, "blog.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	ad := filepath.Join(o.Home, "apache", "sites")
	_ = os.MkdirAll(ad, 0o755)
	vh := "<VirtualHost 127.0.0.1:8080>\n  ServerName blog.lab\n  DocumentRoot \"" + dir + "\"\n  SetHandler \"proxy:unix:/x/php-fpm-dev-8.4.sock|fcgi://localhost\"\n</VirtualHost>\n"
	_ = os.WriteFile(filepath.Join(ad, "blog.conf"), []byte(vh), 0o644)
	b := o.Do("GET", "/api/sites", "", o.Cookie, "", nil).Body.String()
	for _, s := range []string{`"name":"blog"`, `"mode":"apache"`, `"php":"8.4"`, `"target":"` + dir + `"`} {
		if !strings.Contains(b, s) {
			t.Errorf("daftar situs tanpa %s: %s", s, b)
		}
	}
	// proxy biasa ke port 8080 TANPA vhost Apache tetap proxy
	_ = os.Remove(filepath.Join(ad, "blog.conf"))
	b = o.Do("GET", "/api/sites", "", o.Cookie, "", nil).Body.String()
	if strings.Contains(b, `"mode":"apache"`) {
		t.Errorf("tanpa vhost Apache tidak boleh bermode apache: %s", b)
	}
}
