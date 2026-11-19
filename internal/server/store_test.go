package server_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/auth"
	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/settings"
	"github.com/roqianjas/macpanel/internal/store"
	"github.com/roqianjas/macpanel/internal/system"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestServiceActionGuards(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	before := o.Calls()
	for _, p := range []string{"/api/services/panel/stop", "/api/services/dnsmasq/restart", "/api/services/redis/kill", "/api/services/RE/start", "/api/services/-x/start", "/api/services/redis;id/start", "/api/services/a..b/start"} {
		if c, b := o.Post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d %s", p, c, b)
		}
	}
	o.Stepup()
	for _, p := range []string{"/api/services/panel/stop", "/api/services/dnsmasq/restart", "/api/services/redis/kill", "/api/services/-x/start"} {
		if c, _ := o.Post(p, `{}`); c != 400 {
			t.Errorf("(setelah step-up) %s harus 400: %d", p, c)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	if c, _ := o.Post("/api/services/redis/stop", `{}`); c != 403 {
		t.Fatalf("stop tanpa step-up: %d", c)
	}
	if c, _ := o.Post("/api/services/redis/restart", `{}`); c != 403 {
		t.Fatalf("restart tanpa step-up: %d", c)
	}
	if c, _ := o.Post("/api/services/redis/start", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[service][start][redis]") {
		t.Fatalf("start tidak butuh step-up: %d %s", c, o.Calls())
	}
	o.Stepup()
	if c, _ := o.Post("/api/services/redis/stop", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[service][stop][redis]") {
		t.Fatalf("stop: %d", c)
	}
}

func TestAppInstallRemoveGuards(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	for _, id := range []string{"nginx", "redis", "mysql", "x;y", "../x", "memcached2"} {
		if c, _ := o.Post("/api/apps/"+id+"/install", `{}`); c != 400 && c != 404 && c != 307 {
			t.Errorf("install %s -> %d", id, c)
		}
	}
	o.Stepup()
	for _, id := range []string{"nginx", "redis", "postgresql"} {
		if c, _ := o.Post("/api/apps/"+id+"/remove", `{"confirm":"`+id+`"}`); c != 400 {
			t.Errorf("remove %s (bukan opsional) harus 400: %d", id, c)
		}
	}
	if strings.Contains(o.Calls(), "[app]") {
		t.Fatalf("lab terpanggil: %s", o.Calls())
	}
	c, b := o.Post("/api/apps/memcached/install", `{}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[app][install][memcached]") {
		t.Fatalf("%s", o.Calls())
	}
	if c, _ := o.Post("/api/apps/memcached/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	if c, _ := o.Post("/api/apps/memcached/remove", `{"confirm":"memcached"}`); c != 403 {
		t.Fatalf("remove tanpa step-up: %d", c)
	}
	o.Stepup()
	c, b = o.Post("/api/apps/memcached/remove", `{"confirm":"memcached"}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[app][remove][memcached]") {
		t.Fatalf("%s", o.Calls())
	}
}

func TestSiteToggleGuards(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	before := o.Calls()
	for _, p := range []string{"/api/sites/home/disable", "/api/sites/panel/disable", "/api/sites/db/enable", "/api/sites/demo/delete", "/api/sites/Demo/disable", "/api/sites/-x/disable", "/api/sites/de;mo/disable"} {
		if c, b := o.Post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d %s", p, c, b)
		}
	}
	o.Stepup()
	for _, p := range []string{"/api/sites/home/disable", "/api/sites/panel/enable", "/api/sites/demo/delete"} {
		if c, _ := o.Post(p, `{}`); c != 400 {
			t.Errorf("(step-up aktif) %s harus 400: %d", p, c)
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	if c, _ := o.Post("/api/sites/demo/disable", `{}`); c != 403 {
		t.Fatalf("disable tanpa step-up: %d", c)
	}
	if c, _ := o.Post("/api/sites/demo/enable", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[site][enable][demo]") {
		t.Fatalf("enable tidak butuh step-up: %d %s", c, o.Calls())
	}
	o.Stepup()
	if c, _ := o.Post("/api/sites/demo/disable", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[site][disable][demo]") {
		t.Fatalf("disable: %d", c)
	}
}

func TestCollectorRatesCounterResetAndNullOnFailure(t *testing.T) {
	o := testenv.NewOpsEnv(t, "echo ok")
	var calls []string
	net, cpuFail := testenv.NetText(1000, 500), false
	c := system.NewCollector(o.App)
	c.Runner = testenv.FakeRunner(t, &net, &cpuFail, &calls)
	c.Disk = func() (uint64, uint64) { return 1000, 250 }
	t0 := time.Unix(1_800_000_000, 0)

	c.Sample(t0) // sampel pertama: belum ada laju
	if n := len(o.App.Store.NetSeries(0, 30)); n != 0 {
		t.Fatalf("sampel pertama tidak boleh punya laju: %d", n)
	}
	net = testenv.NetText(4000, 2000) // +3000 rx, +1500 tx dalam 30 dtk
	c.Sample(t0.Add(30 * time.Second))
	ns := o.App.Store.NetSeries(0, 30)
	if len(ns) != 1 || ns[0].Iface != "en1" || ns[0].RX[0][1] != 100 || ns[0].TX[0][1] != 50 || ns[0].TotalRX != 3000 || ns[0].TotalTX != 1500 {
		t.Fatalf("laju salah (harus 100 B/s dan 50 B/s): %+v", ns)
	}
	if ns[0].Label != "LAN (en1)" {
		t.Fatal(ns[0].Label)
	}
	net = testenv.NetText(10, 10) // penghitung reset (reboot): bukan laju negatif/raksasa
	c.Sample(t0.Add(60 * time.Second))
	ns = o.App.Store.NetSeries(0, 30)
	if len(ns[0].RX) != 1 {
		t.Fatalf("reset penghitung tidak boleh menambah titik: %+v", ns[0].RX)
	}
	cpuFail = true // CPU gagal dibaca: NULL, bukan 0
	c.Sample(t0.Add(90 * time.Second))
	cpu, mem, disk, load := o.App.Store.Series(0, 30)
	if len(cpu) != 3 || len(mem) != 4 {
		t.Fatalf("cpu harus 3 titik (1 gagal), mem 4: cpu=%d mem=%d", len(cpu), len(mem))
	}
	if cpu[0][1] != 10 || mem[0][1] != 45 || disk[0][1] != 75 || load[0][1] != 1 {
		t.Fatalf("nilai: cpu=%v mem=%v disk=%v load=%v", cpu[0], mem[0], disk[0], load[0])
	}
	for _, n := range calls {
		if !strings.HasPrefix(n, "/usr/") && !strings.HasPrefix(n, "/bin/") {
			t.Fatalf("hanya path mutlak sistem yang boleh dipanggil: %s", n)
		}
	}
}

func TestSeriesBucketsAndPrune(t *testing.T) {
	o := testenv.NewOpsEnv(t, "echo ok")
	st := o.App.Store
	for i := int64(0); i < 10; i++ {
		st.AddSample(1000+i*30, float64(i*10), 50, 40, 1, true, true)
	}
	cpu, _, _, _ := st.Series(0, 300) // 10 titik x 30 dtk = 300 dtk -> 1-2 bucket
	if len(cpu) < 1 || len(cpu) > 2 {
		t.Fatalf("bucket 300: %v", cpu)
	}
	raw, _, _, _ := st.Series(0, 30)
	if len(raw) != 10 {
		t.Fatalf("mentah: %d", len(raw))
	}
	if only, _, _, _ := st.Series(1000+5*30, 30); len(only) != 5 {
		t.Fatalf("filter since: %d", len(only))
	}
	st.PruneMetrics(1000 + 5*30)
	if after, _, _, _ := st.Series(0, 30); len(after) != 5 {
		t.Fatalf("prune: %d", len(after))
	}
}

func TestMetricsAPIValidationAndShape(t *testing.T) {
	o := testenv.NewOpsEnv(t, "echo ok")
	o.App.Store.AddSample(o.Now.Unix()-60, 10, 20, 30, 1, true, true)
	for _, bad := range []string{"range=2h", "range=", "range=999d", "range=%00", "range=1H", "range=1h%20"} {
		w := o.Do("GET", "/api/metrics?"+bad, "", o.Cookie, "", nil)
		if bad == "range=" {
			if w.Code != 200 {
				t.Errorf("range kosong harus default 24h: %d", w.Code)
			}
			continue
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s -> %d", bad, w.Code)
		}
	}
	// pasangan query berisi ';' dibuang Go, jadi jatuh ke default 24h dan tidak pernah sampai ke SQL
	if w := o.Do("GET", "/api/metrics?range=1h;x", "", o.Cookie, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"range":"24h"`) {
		t.Errorf("harus default 24h: %d %s", w.Code, w.Body)
	}
	w := o.Do("GET", "/api/metrics?range=1h", "", o.Cookie, "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"cpu":[[`) || !strings.Contains(w.Body.String(), `"nets":[]`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestTrustedDeviceFlow(t *testing.T) {
	e := testenv.NewEnv(t)
	// tanpa centang: tidak ada cookie tepercaya
	if _, _, tok := e.TrustLogin(false); tok != "" {
		t.Fatal("cookie tepercaya keluar tanpa diminta")
	}
	e.Now = e.Now.Add(time.Minute)
	ck, cs, tok := e.TrustLogin(true)
	if tok == "" {
		t.Fatal("cookie tepercaya tidak keluar")
	}
	// step-up di sesi ini, supaya terbukti tidak 'terbawa' ke sesi tepercaya
	if w := e.Do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("stepup: %d %s", w.Code, w.Body)
	}
	// login berikutnya: kata sandi saja
	w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"trusted":true`) {
		t.Fatalf("login tepercaya: %d %s", w.Code, w.Body)
	}
	c2, cs2, tok2 := testenv.CookieOf(w), testenv.CsrfOf(w), testenv.TrustOf(w)
	if tok2 == "" || tok2 == tok {
		t.Fatal("token tepercaya tidak dirotasi")
	}
	if m := e.Do("GET", "/api/me", "", c2, "", nil); m.Code != 200 || !strings.Contains(m.Body.String(), `"stepup":false`) {
		t.Fatalf("sesi tepercaya tidak boleh punya step-up: %d %s", m.Code, m.Body)
	}
	// token lama sudah tidak berlaku; tanpa cookie wajib 2FA
	if w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("token lama masih diterima setelah rotasi")
	}
	if w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", nil); !strings.Contains(w.Body.String(), "need_totp") {
		t.Fatalf("tanpa cookie harus minta 2FA: %s", w.Body)
	}
	// kata sandi salah tetap ditolak walau cookie sah
	if w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"salah-salah-salah"}`, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok2}); w.Code != 401 {
		t.Fatalf("sandi salah + cookie sah: %d", w.Code)
	}
	// daftar + label
	d := e.Do("GET", "/api/me/devices", "", c2, "", map[string]string{"Cookie": core.CookieName + "=" + c2 + "; " + store.TrustCookie + "=" + tok2})
	if d.Code != 200 || !strings.Contains(d.Body.String(), "Firefox di Linux") || !strings.Contains(d.Body.String(), `"current":true`) {
		t.Fatalf("daftar perangkat: %d %s", d.Code, d.Body)
	}
	// kedaluwarsa 30 hari
	e.Now = e.Now.Add(store.TrustTTL + time.Hour)
	if w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok2}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("perangkat kedaluwarsa masih diterima")
	}
	_ = cs2
}

func TestTrustedRevokeAndPasswordChange(t *testing.T) {
	e := testenv.NewEnv(t)
	ck, cs, tok := e.TrustLogin(true)
	both := map[string]string{"Cookie": core.CookieName + "=" + ck + "; " + store.TrustCookie + "=" + tok}
	d := e.Do("GET", "/api/me/devices", "", ck, "", both)
	var id string
	if i := strings.Index(d.Body.String(), `"id":`); i >= 0 {
		id = strings.SplitN(d.Body.String()[i+5:], ",", 2)[0]
	}
	if w := e.Do("POST", "/api/me/devices/revoke", `{"id":99999}`, ck, cs, nil); w.Code != 404 {
		t.Fatalf("revoke id asing: %d", w.Code)
	}
	if w := e.Do("POST", "/api/me/devices/revoke", `{"id":`+id+`}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("revoke: %d %s", w.Code, w.Body)
	}
	if w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("perangkat yang dicabut masih diterima")
	}
	// ganti sandi mencabut semua
	e.Now = e.Now.Add(time.Minute)
	ck, cs, tok = e.TrustLogin(true)
	if w := e.Do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("stepup: %d %s", w.Code, w.Body)
	}
	if w := e.Do("POST", "/api/me/password", `{"current":"kata-sandi-panjang-123","new":"sandi-baru-yang-panjang-456"}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("ganti sandi: %d %s", w.Code, w.Body)
	}
	if w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"sandi-baru-yang-panjang-456"}`, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("perangkat tepercaya bertahan setelah ganti sandi")
	}
}

func TestTrustedCapAndIsolation(t *testing.T) {
	e := testenv.NewEnv(t)
	var first string
	for i := 0; i < store.MaxTrusted+3; i++ {
		e.Now = e.Now.Add(time.Minute)
		_, _, tok := e.TrustLogin(true)
		if i == 0 {
			first = tok
		}
	}
	u := e.App.Store.GetUser("admin")
	if n := len(e.App.Store.ListTrusted(u.ID, "", e.Now)); n != store.MaxTrusted {
		t.Fatalf("batas perangkat: %d", n)
	}
	if _, ok := e.App.Store.UseTrusted(first, u.ID, e.Now); ok {
		t.Fatal("perangkat tertua seharusnya sudah dibuang")
	}
	// token milik user lain tidak berlaku
	if _, ok := e.App.Store.UseTrusted("x", u.ID+1, e.Now); ok {
		t.Fatal("token tak dikenal diterima")
	}
	if l := auth.DeviceLabel("curl/8"); l != "Browser di perangkat tidak dikenal" {
		t.Fatalf("label: %s", l)
	}
}

func TestTrustedCookieAttrsAndOtherUser(t *testing.T) {
	e := testenv.NewEnv(t)
	w1 := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", nil)
	w2 := e.Do("POST", "/api/auth/totp", `{"code":"`+e.Code(0)+`","trust":true}`, testenv.CookieOf(w1), testenv.CsrfOf(w1), nil)
	var found bool
	for _, c := range w2.Result().Cookies() {
		if c.Name == store.TrustCookie {
			found = true
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != int(store.TrustTTL.Seconds()) {
				t.Fatalf("atribut cookie tepercaya lemah: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("cookie tepercaya tidak ada")
	}
	tok := testenv.TrustOf(w2)
	// user lain tidak boleh memakai perangkat milik admin
	enc, _ := crypto.Seal(testenv.TestKey, crypto.NewTOTPSecret())
	if err := e.App.Store.CreateUser("ani", crypto.HashPassword("sandi-ani-panjang-789"), enc, nil); err != nil {
		t.Fatal(err)
	}
	w := e.Do("POST", "/api/auth/login", `{"name":"ani","password":"sandi-ani-panjang-789"}`, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok})
	if strings.Contains(w.Body.String(), `"trusted"`) || !strings.Contains(w.Body.String(), "need_totp") {
		t.Fatalf("cookie admin dipakai user lain: %s", w.Body)
	}
	// dan tidak bisa mencabut perangkat milik admin
	admin, ani := e.App.Store.GetUser("admin"), e.App.Store.GetUser("ani")
	rows := e.App.Store.ListTrusted(admin.ID, "", e.Now)
	if len(rows) != 1 || e.App.Store.RevokeTrusted(ani.ID, rows[0].ID) {
		t.Fatal("user lain bisa mencabut perangkat orang lain")
	}
}

func TestRenameUser(t *testing.T) {
	e := testenv.NewEnv(t)
	_, _, tok := e.TrustLogin(true)
	enc, _ := crypto.Seal(testenv.TestKey, crypto.NewTOTPSecret())
	e.App.Store.CreateUser("ani", crypto.HashPassword("sandi-ani-panjang-789"), enc, nil)
	if e.App.Store.RenameUser("admin", "ani") || e.App.Store.RenameUser("tidakada", "baru") {
		t.Fatal("rename ke nama terpakai / user tak ada harus gagal")
	}
	if !e.App.Store.RenameUser("admin", "rokianjas") {
		t.Fatal("rename gagal")
	}
	if e.App.Store.GetUser("admin") != nil || e.App.Store.GetUser("rokianjas") == nil {
		t.Fatal("nama tidak berubah")
	}
	e.Now = e.Now.Add(time.Minute)
	w := e.Do("POST", "/api/auth/login", `{"name":"rokianjas","password":"kata-sandi-panjang-123"}`, "", "", map[string]string{"Cookie": store.TrustCookie + "=" + tok})
	if !strings.Contains(w.Body.String(), `"trusted":true`) {
		t.Fatalf("perangkat tepercaya harus tetap berlaku: %s", w.Body)
	}
	if w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", nil); w.Code != 401 {
		t.Fatalf("nama lama harus ditolak: %d", w.Code)
	}
}

func TestNineRouterIsOptionalAppWithSafeGuards(t *testing.T) {
	o := testenv.NewOpsEnv(t, "echo ok")
	c, b := o.Post("/api/apps/9router/install", `{}`)
	if c != 202 {
		t.Fatalf("install 9router: %d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[app][install][9router]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	// hapus: konfirmasi nama + step-up
	if c, _ := o.Post("/api/apps/9router/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	if c, _ := o.Post("/api/apps/9router/remove", `{"confirm":"9router"}`); c != 403 {
		t.Fatalf("remove tanpa step-up: %d", c)
	}
	// aplikasi lain yang tak ada di katalog tetap ditolak
	if c, _ := o.Post("/api/apps/9router-evil/install", `{}`); c != 400 && c != 404 {
		t.Fatalf("id asing: %d", c)
	}
}

func TestAPITokenScopes(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	// membuat token butuh kata sandi ulang
	if code, _ := o.Post("/api/me/tokens", `{"name":"x","scope":"read"}`); code != 403 {
		t.Fatalf("buat token tanpa step-up harus 403, dapat %d", code)
	}
	o.Stepup()
	rd, _ := o.MkToken("read")
	op, _ := o.MkToken("operate")
	fu, fuID := o.MkToken("full")

	if c, _ := o.Bearer("GET", "/api/me", "", "mpt_salah-salah-salah-salah"); c != 401 {
		t.Fatalf("token palsu harus 401, dapat %d", c)
	}
	if c, _ := o.Bearer("GET", "/api/me", "", "bukan-token-sama-sekali-xxxx"); c != 401 {
		t.Fatalf("format salah harus 401, dapat %d", c)
	}
	// baca-saja
	if c, b := o.Bearer("GET", "/api/me", "", rd); c != 200 || !strings.Contains(b, `"via":"token"`) || !strings.Contains(b, `"scope":"read"`) {
		t.Fatalf("whoami token: %d %s", c, b)
	}
	if c, _ := o.Bearer("GET", "/api/sites", "", rd); c != 200 {
		t.Fatalf("GET lewat token read harus 200, dapat %d", c)
	}
	if c, b := o.Bearer("POST", "/api/settings", `{"name":"Dilarang"}`, rd); c != 403 || !strings.Contains(b, "baca-saja") {
		t.Fatalf("token read tidak boleh menulis: %d %s", c, b)
	}
	// operasi: boleh ubah pengaturan, tidak boleh aksi step-up
	if c, b := o.Bearer("POST", "/api/settings", `{"name":"Lab Saya"}`, op); c != 200 {
		t.Fatalf("operate ubah pengaturan: %d %s", c, b)
	}
	if w := o.Do("GET", "/api/settings/public", "", "", "", nil); !strings.Contains(w.Body.String(), "Lab Saya") {
		t.Fatalf("nama panel tidak berubah: %s", w.Body)
	}
	delBody := `{"db":"0","key":"app:cache"}`
	if c, b := o.Bearer("POST", "/api/redis/del", delBody, op); c != 403 || !strings.Contains(b, "full") {
		t.Fatalf("operate tidak boleh aksi step-up: %d %s", c, b)
	}
	if strings.Contains(o.Calls(), "[redis][del]") {
		t.Fatal("lab dipanggil padahal token operate")
	}
	// penuh: aksi step-up lewat
	if c, b := o.Bearer("POST", "/api/redis/del", delBody, fu); c != 200 {
		t.Fatalf("token full harus lolos: %d %s", c, b)
	}
	if !strings.Contains(o.Calls(), "[redis][del]") {
		t.Fatalf("lab tidak dipanggil oleh token full: %s", o.Calls())
	}
	// tidak pernah lewat token, bahkan full
	for _, p := range [][2]string{
		{"POST", "/api/auth/stepup"}, {"POST", "/api/me/password"}, {"GET", "/api/me/tokens"}, {"POST", "/api/me/tokens"},
		{"POST", "/api/me/2fa/disable"}, {"GET", "/api/me/devices"}, {"POST", "/api/db/mysql/proj/creds"}, {"GET", "/api/sites/demo/env"}, {"POST", "/api/sites/demo/env"},
	} {
		if c, b := o.Bearer(p[0], p[1], `{}`, fu); c != 403 || !strings.Contains(b, "tidak tersedia lewat token") {
			t.Fatalf("%s %s harus 403 untuk token full: %d %s", p[0], p[1], c, b)
		}
	}
	// daftar rute tidak memuat yang terlarang
	c, b := o.Bearer("GET", "/api/routes", "", rd)
	var rl struct{ Routes []struct{ Path string } }
	json.Unmarshal([]byte(b), &rl)
	if c != 200 || len(rl.Routes) < 50 {
		t.Fatalf("daftar rute: %d %.200s", c, b)
	}
	for _, r := range rl.Routes {
		if strings.Contains(r.Path, "/creds") || strings.HasPrefix(r.Path, "/api/auth/") || strings.HasPrefix(r.Path, "/api/me/") || strings.HasSuffix(r.Path, "/env") {
			t.Fatalf("rute terlarang muncul di daftar: %s", r.Path)
		}
	}
	// jejak audit
	found := false
	for _, r := range o.App.Store.RecentAudit(50) {
		if r.Action == "api_call" && strings.Contains(r.Detail, "POST /api/redis/del") {
			found = true
		}
	}
	if !found {
		t.Fatal("api_call tidak tercatat di audit")
	}
	// cabut -> langsung ditolak
	if c, _ := o.Post("/api/me/tokens/revoke", `{"id":`+testenv.ItoaTok(fuID)+`}`); c != 200 {
		t.Fatal("cabut gagal")
	}
	if c, _ := o.Bearer("GET", "/api/me", "", fu); c != 401 {
		t.Fatalf("token yang dicabut harus 401, dapat %d", c)
	}
	// kedaluwarsa
	o.App.Now = func() time.Time { return o.Now.Add(91 * 24 * time.Hour) }
	if c, _ := o.Bearer("GET", "/api/me", "", rd); c != 401 {
		t.Fatalf("token kedaluwarsa harus 401, dapat %d", c)
	}
}

func TestAPITokenLimitsAndValidation(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	o.Stepup()
	for _, bad := range []string{`{"name":"","scope":"read"}`, `{"name":"<b>","scope":"read"}`, `{"name":"ok","scope":"root"}`, `{"name":"ok","scope":"read","days":400}`, `{"name":"ok","scope":"read","days":-1}`} {
		if c, _ := o.Post("/api/me/tokens", bad); c != 400 {
			t.Fatalf("%s harus 400, dapat %d", bad, c)
		}
	}
	for i := 0; i < store.MaxTokens; i++ {
		o.MkToken("read")
	}
	if c, _ := o.Post("/api/me/tokens", `{"name":"lebih","scope":"read"}`); c != 409 {
		t.Fatalf("lewat batas harus 409, dapat %d", c)
	}
	// hanya hash yang disimpan
	var n int
	o.App.Store.DB.QueryRow("SELECT COUNT(*) FROM tokens WHERE hash LIKE 'mpt_%'").Scan(&n)
	if n != 0 {
		t.Fatal("token mentah tersimpan di database")
	}
	// akun hanya-baca: token otomatis baca-saja
	o.App.Store.SetRole("admin", "viewer")
	o.App.Store.DB.Exec("DELETE FROM tokens")
	tok, _, _ := o.App.Store.AddToken(1, "v", "full", 30, o.Now)
	if c, b := o.Bearer("POST", "/api/settings", `{"name":"X"}`, tok); c != 403 {
		t.Fatalf("viewer lewat token tidak boleh menulis: %d %s", c, b)
	}
	if c, b := o.Bearer("GET", "/api/me", "", tok); c != 200 || !strings.Contains(b, `"scope":"read"`) {
		t.Fatalf("token full milik viewer harus turun ke read: %d %s", c, b)
	}
}

func TestTwoFactorToggle(t *testing.T) {
	e := testenv.NewEnv(t)
	ck, cs, _ := e.Login(e.Code(0))
	e.Now = e.Now.Add(time.Minute)
	if w := e.Do("GET", "/api/me/2fa", "", ck, "", nil); !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatalf("2FA harus aktif: %s", w.Body)
	}
	// salah kata sandi / salah kode: ditolak, 2FA tetap aktif
	bad := `{"password":"salah-salah-salah","code":"` + e.Code(1) + `"}`
	if w := e.Do("POST", "/api/me/2fa/disable", bad, ck, cs, nil); w.Code != 400 {
		t.Fatalf("kata sandi salah harus 400: %d", w.Code)
	}
	if w := e.Do("POST", "/api/me/2fa/disable", `{"password":"kata-sandi-panjang-123","code":"000000"}`, ck, cs, nil); w.Code != 400 {
		t.Fatalf("kode salah harus 400: %d", w.Code)
	}
	if e.App.Store.GetUser("admin").TOTPOn != true {
		t.Fatal("2FA mati padahal permintaan ditolak")
	}
	// trusted device + sesi lain harus tercabut
	_, _, tok := e.TrustLogin(true)
	if tok == "" {
		t.Fatal("setup: perangkat tepercaya tidak terbentuk")
	}
	e.Now = e.Now.Add(time.Minute)
	good := `{"password":"kata-sandi-panjang-123","code":"` + e.Code(0) + `"}`
	if w := e.Do("POST", "/api/me/2fa/disable", good, ck, cs, nil); w.Code != 200 {
		t.Fatalf("matikan 2FA: %d %s", w.Code, w.Body)
	}
	if w := e.Do("POST", "/api/me/2fa/disable", good, ck, cs, nil); w.Code != 409 {
		t.Fatalf("mematikan dua kali harus 409: %d", w.Code)
	}
	// login: kata sandi saja
	w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"no2fa":true`) || testenv.CookieOf(w) == "" {
		t.Fatalf("login tanpa 2FA: %d %s", w.Code, w.Body)
	}
	if m := e.Do("GET", "/api/me", "", testenv.CookieOf(w), "", nil); m.Code != 200 {
		t.Fatalf("sesi tanpa 2FA harus penuh: %d", m.Code)
	}
	// perangkat tepercaya sudah dicabut
	if n := len(e.App.Store.ListTrusted(1, "", e.Now)); n != 0 {
		t.Fatalf("perangkat tepercaya masih ada: %d", n)
	}
	// nyalakan lagi: butuh step-up, kode harus benar
	ck2, cs2 := testenv.CookieOf(w), testenv.CsrfOf(w)
	if w := e.Do("POST", "/api/me/2fa/setup", `{}`, ck2, cs2, nil); w.Code != 403 {
		t.Fatalf("setup tanpa step-up harus 403: %d", w.Code)
	}
	e.Do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck2, cs2, nil)
	sw := e.Do("POST", "/api/me/2fa/setup", `{}`, ck2, cs2, nil)
	var su struct{ Secret, URI string }
	json.Unmarshal(sw.Body.Bytes(), &su)
	secret, err := crypto.B32.DecodeString(su.Secret)
	if sw.Code != 200 || err != nil || len(secret) != 20 || !strings.HasPrefix(su.URI, "otpauth://") {
		t.Fatalf("setup 2FA: %d %s", sw.Code, sw.Body)
	}
	if w := e.Do("POST", "/api/me/2fa/enable", `{"code":"123456"}`, ck2, cs2, nil); w.Code != 400 {
		t.Fatalf("kode salah harus 400: %d", w.Code)
	}
	if e.App.Store.GetUser("admin").TOTPOn {
		t.Fatal("2FA menyala dengan kode salah")
	}
	e.App.Store.AddTrusted(1, "laptop", e.Now)
	ew := e.Do("POST", "/api/me/2fa/enable", `{"code":"`+crypto.TotpAt(secret, e.Now.Unix()/30)+`"}`, ck2, cs2, nil)
	var en struct {
		Codes []string `json:"recovery_codes"`
	}
	json.Unmarshal(ew.Body.Bytes(), &en)
	if ew.Code != 200 || len(en.Codes) != 10 {
		t.Fatalf("enable 2FA: %d %s", ew.Code, ew.Body)
	}
	if n := len(e.App.Store.ListTrusted(1, "", e.Now)); n != 0 {
		t.Fatalf("enable harus mencabut perangkat tepercaya: %d", n)
	}
	if w := e.Do("POST", "/api/me/2fa/setup", `{}`, ck2, cs2, nil); w.Code != 409 {
		t.Fatalf("setup saat aktif harus 409: %d", w.Code)
	}
	// login kembali meminta 2FA, dengan kunci BARU; recovery code baru berlaku
	if w := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", nil); !strings.Contains(w.Body.String(), "need_totp") {
		t.Fatalf("login harus minta 2FA lagi: %s", w.Body)
	}
	lw := e.Do("POST", "/api/auth/login", testenv.LoginBody, "", "", nil)
	tw := e.Do("POST", "/api/auth/totp", `{"code":"`+en.Codes[0]+`"}`, testenv.CookieOf(lw), testenv.CsrfOf(lw), nil)
	if tw.Code != 200 {
		t.Fatalf("recovery code baru ditolak: %d %s", tw.Code, tw.Body)
	}
}

func TestPanelSettings(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if w := o.Do("GET", "/api/settings/public", "", "", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), store.DefaultName) {
		t.Fatalf("publik harus memuat nama bawaan tanpa login: %d %s", w.Code, w.Body)
	}
	for _, bad := range []string{`{"name":""}`, `{"name":"<script>"}`, `{"name":"` + strings.Repeat("a", 41) + `"}`, `{"theme":"neon"}`, `{}`} {
		if c, _ := o.Post("/api/settings", bad); c != 400 {
			t.Fatalf("%s harus 400, dapat %d", bad, c)
		}
	}
	if c, b := o.Post("/api/settings", `{"name":"Dapur Lab 2","theme":"dark"}`); c != 200 || !strings.Contains(b, "Dapur Lab 2") || !strings.Contains(b, "dark") {
		t.Fatalf("simpan: %d %s", c, b)
	}
	if o.App.Store.Setting("panel_name", "") != "Dapur Lab 2" {
		t.Fatal("tidak tersimpan")
	}
	// tanpa sesi: tulis ditolak
	if w := o.Do("POST", "/api/settings", `{"name":"X"}`, "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa sesi harus 401: %d", w.Code)
	}
}

func TestPanelSettingsAdminOnlyDirect(t *testing.T) {
	e := testenv.NewEnv(t)
	r := httptest.NewRequest("POST", "/api/settings", strings.NewReader(`{"name":"Diretas"}`))
	w := httptest.NewRecorder()
	settings.PanelSettingsSet(e.App, w, r, &store.Session{User: "x", Role: "viewer", Stage: "full"})
	if w.Code != 403 || e.App.Store.Setting("panel_name", "") == "Diretas" {
		t.Fatalf("non-admin harus ditolak di handler: %d", w.Code)
	}
}

func TestAccountManagement(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.DbFake)
	if c, _ := o.Post("/api/accounts", `{"name":"budi","role":"viewer"}`); c != 403 {
		t.Fatalf("buat akun tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	for _, bad := range []string{`{"name":"B","role":"viewer"}`, `{"name":"ab","role":"viewer"}`, `{"name":"budi","role":"root"}`, `{"name":"bu di","role":"viewer"}`} {
		if c, _ := o.Post("/api/accounts", bad); c != 400 {
			t.Fatalf("%s harus 400: %d", bad, c)
		}
	}
	c, b := o.Post("/api/accounts", `{"name":"budi","role":"admin"}`)
	var mk struct {
		Temp string `json:"temp_password"`
	}
	json.Unmarshal([]byte(b), &mk)
	if c != 200 || len(mk.Temp) < 16 {
		t.Fatalf("buat akun: %d %s", c, b)
	}
	if c, _ := o.Post("/api/accounts", `{"name":"budi","role":"viewer"}`); c != 409 {
		t.Fatalf("nama kembar harus 409: %d", c)
	}
	if strings.Contains(strings.Join(func() []string {
		var s []string
		for _, r := range o.App.Store.RecentAudit(50) {
			s = append(s, r.Detail)
		}
		return s
	}(), "|"), mk.Temp) {
		t.Fatal("kata sandi sementara bocor ke audit")
	}
	// login pertama: 2FA mati, tapi dikunci sampai kata sandi diganti
	bc, bs, code := o.Login2("budi", mk.Temp)
	if code != 200 || bc == "" {
		t.Fatalf("login akun baru: %d", code)
	}
	if c, b := o.As(bc, bs, "GET", "/api/me", ""); c != 200 || !strings.Contains(b, `"must_change":true`) {
		t.Fatalf("me harus menandai must_change: %d %s", c, b)
	}
	for _, p := range []string{"/api/sites", "/api/db", "/api/accounts", "/api/system"} {
		if c, b := o.As(bc, bs, "GET", p, ""); c != 403 || !strings.Contains(b, "ganti kata sandi sementara") {
			t.Fatalf("%s harus terkunci sebelum ganti sandi: %d %s", p, c, b)
		}
	}
	o.As(bc, bs, "POST", "/api/auth/stepup", `{"password":"`+mk.Temp+`"}`)
	if c, b := o.As(bc, bs, "POST", "/api/me/password", `{"current":"`+mk.Temp+`","new":"sandi-baru-budi-12345"}`); c != 200 {
		t.Fatalf("ganti sandi sementara: %d %s", c, b)
	}
	if c, _ := o.As(bc, bs, "GET", "/api/sites", ""); c != 200 {
		t.Fatalf("setelah ganti sandi harus terbuka: %d", c)
	}
	if _, _, code := o.Login2("budi", mk.Temp); code == 200 && o.App.Store.GetUser("budi").MustChange {
		t.Fatal("sandi sementara masih berlaku")
	}

	// penjagaan: diri sendiri
	o.Stepup()
	for _, p := range []string{"role", "disable", "enable", "reset", "2fa-off", "remove"} {
		if c, _ := o.Post("/api/accounts/admin/"+p, `{"role":"viewer","confirm":"admin"}`); c != 400 {
			t.Fatalf("%s pada diri sendiri harus 400: %d", p, c)
		}
	}
	if c, _ := o.Post("/api/accounts/tidakada/disable", `{}`); c != 404 {
		t.Fatalf("akun tak ada harus 404: %d", c)
	}
	// nonaktifkan: sesi budi mati, login ditolak, aktifkan lagi
	if c, b := o.Post("/api/accounts/budi/disable", `{}`); c != 200 {
		t.Fatalf("disable: %d %s", c, b)
	}
	if c, _ := o.As(bc, bs, "GET", "/api/me", ""); c != 401 {
		t.Fatalf("sesi akun nonaktif harus mati: %d", c)
	}
	if _, _, code := o.Login2("budi", "sandi-baru-budi-12345"); code == 200 {
		t.Fatal("akun nonaktif bisa login")
	}
	o.Post("/api/accounts/budi/enable", `{}`)
	// ubah peran ke viewer: token dicabut
	o.App.Store.AddToken(o.App.Store.GetUser("budi").ID, "t", "full", 30, o.Now)
	if c, _ := o.Post("/api/accounts/budi/role", `{"role":"viewer"}`); c != 200 {
		t.Fatal("role gagal")
	}
	var n int
	o.App.Store.DB.QueryRow("SELECT COUNT(*) FROM tokens WHERE user_id=?", o.App.Store.GetUser("budi").ID).Scan(&n)
	if n != 0 {
		t.Fatal("token tidak dicabut saat peran berubah")
	}
	// reset: sandi lama mati, must_change menyala
	c, b = o.Post("/api/accounts/budi/reset", `{}`)
	json.Unmarshal([]byte(b), &mk)
	if c != 200 || len(mk.Temp) < 16 || !o.App.Store.GetUser("budi").MustChange {
		t.Fatalf("reset: %d %s", c, b)
	}
	if _, _, code := o.Login2("budi", "sandi-baru-budi-12345"); code == 200 {
		t.Fatal("sandi lama masih berlaku setelah reset")
	}
	// reset dengan sandi pilihan admin: tidak dikirim balik, pendek ditolak
	if c, _ := o.Post("/api/accounts/budi/reset", `{"password":"pendek"}`); c != 400 {
		t.Fatalf("sandi pendek harus 400: %d", c)
	}
	c, b = o.Post("/api/accounts/budi/reset", `{"password":"pilihan-admin-123"}`)
	if c != 200 || strings.Contains(b, "pilihan-admin-123") || strings.Contains(b, "temp_password") || !o.App.Store.GetUser("budi").MustChange {
		t.Fatalf("reset pilihan: %d %s", c, b)
	}
	if _, _, code := o.Login2("budi", "pilihan-admin-123"); code != 200 {
		t.Fatalf("sandi pilihan tidak berlaku: %d", code)
	}
	// 2FA off milik orang lain
	if c, _ := o.Post("/api/accounts/budi/2fa-off", `{}`); c != 409 {
		t.Fatalf("2FA budi sudah mati: %d", c)
	}
	// hapus: wajib ketik nama
	if c, _ := o.Post("/api/accounts/budi/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi salah harus 400: %d", c)
	}
	if c, _ := o.Post("/api/accounts/budi/remove", `{"confirm":"budi"}`); c != 200 || o.App.Store.GetUser("budi") != nil {
		t.Fatal("hapus gagal")
	}
	// admin terakhir tidak bisa disingkirkan oleh admin lain: dua admin, satu menurunkan yang lain lalu sebaliknya
	o.Post("/api/accounts", `{"name":"citra","role":"admin"}`)
	if c, _ := o.Post("/api/accounts/citra/role", `{"role":"viewer"}`); c != 200 {
		t.Fatal("turunkan admin kedua harus boleh (masih ada admin lain)")
	}
	if o.App.Store.AdminsExcept("citra") < 1 {
		t.Fatal("tidak ada admin tersisa")
	}
}

func TestAccountsNotForViewerOrToken(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.DbFake)
	o.Stepup()
	tok, _ := o.MkToken("full")
	for _, p := range [][2]string{{"GET", "/api/accounts"}, {"POST", "/api/accounts"}, {"POST", "/api/accounts/x/remove"}, {"POST", "/api/dbroot/mysql/passwd"}} {
		if c, b := o.Bearer(p[0], p[1], `{}`, tok); c != 403 || !strings.Contains(b, "tidak tersedia lewat token") {
			t.Fatalf("%s %s harus 403 bagi token full: %d %s", p[0], p[1], c, b)
		}
	}
	o.App.Store.SetRole("admin", "viewer")
	vc, vs, _ := o.Login(o.Code(1))
	if c, _ := o.As(vc, vs, "GET", "/api/accounts", ""); c != 403 {
		t.Fatalf("viewer tidak boleh melihat akun: %d", c)
	}
}

func TestDBUsersAndRoot(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.DbFake)
	pw := "SandiPilihanSendiri-9876"
	base := "/api/db/mariadb/proj/dbusers"
	if c, _ := o.Post(base, `{"user":"pembaca","privs":"ro"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	for _, bad := range []string{`{"user":"AB","privs":"ro"}`, `{"user":"pembaca","privs":"root"}`, `{"user":"pembaca","privs":"ro","password":"pendek"}`, `{"user":"x'; drop","privs":"ro"}`, `{"user":"pembaca","privs":"ro","password":"a'b'c'd'e'f'g'h'i'j"}`} {
		if c, _ := o.Post(base, bad); c != 400 {
			t.Fatalf("%s harus 400: %d", bad, c)
		}
	}
	if strings.Contains(o.Calls(), "[user][add]") {
		t.Fatal("lab dipanggil dengan input tak valid")
	}
	// acak: kata sandi dikembalikan sekali dari lab; tidak masuk audit/job
	c, b := o.Post(base, `{"user":"pembaca","privs":"ro"}`)
	if c != 200 || !strings.Contains(b, "PwAcakDariLab") {
		t.Fatalf("tambah user acak: %d %s", c, b)
	}
	if strings.Contains(o.Calls(), "--password-stdin") {
		t.Fatal("tanpa sandi pilihan, stdin tidak dipakai")
	}
	// pilihan sendiri: lewat stdin, tidak pernah di argumen
	if c, b := o.Post(base, `{"user":"penulis","privs":"rw","password":"`+pw+`"}`); c != 200 {
		t.Fatalf("tambah user dengan sandi sendiri: %d %s", c, b)
	}
	last := o.Calls()[strings.LastIndex(o.Calls(), "CALL:"):]
	if !strings.Contains(last, "[--password-stdin]") || !strings.Contains(last, "STDIN=["+pw+"]") {
		t.Fatalf("sandi harus lewat stdin: %s", last)
	}
	if strings.Count(last, pw) != 1 {
		t.Fatalf("sandi muncul di argumen: %s", last)
	}
	for _, r := range o.App.Store.RecentAudit(50) {
		if strings.Contains(r.Detail, pw) || strings.Contains(r.Detail, "PwAcak") {
			t.Fatalf("sandi bocor ke audit: %s", r.Detail)
		}
	}
	if c, _ := o.Post(base+"/penulis/privs", `{"privs":"all"}`); c != 200 || !strings.Contains(o.Calls(), "[privs][proj][penulis]") {
		t.Fatal("ubah hak gagal")
	}
	if c, _ := o.Post(base+"/penulis/privs", `{"privs":"super"}`); c != 400 {
		t.Fatal("hak tidak valid harus 400")
	}
	if c, _ := o.Post(base+"/penulis/passwd", `{"password":"x"}`); c != 400 {
		t.Fatal("sandi pendek harus 400")
	}
	if c, _ := o.Post(base+"/penulis/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatal("hapus tanpa konfirmasi harus 400")
	}
	if c, _ := o.Post(base+"/penulis/remove", `{"confirm":"penulis"}`); c != 200 || !strings.Contains(o.Calls(), "[user][remove][proj][penulis]") {
		t.Fatal("hapus user gagal")
	}
	// token: baca boleh, tulis tidak
	tok, _ := o.MkToken("full")
	if c, _ := o.Bearer("GET", base, "", tok); c != 200 {
		t.Fatalf("token boleh membaca daftar: %d", c)
	}
	if c, b := o.Bearer("POST", base, `{"user":"agen","privs":"all"}`, tok); c != 403 || !strings.Contains(b, "tidak tersedia lewat token") {
		t.Fatalf("token tidak boleh membuat user DB: %d %s", c, b)
	}
	// root
	if c, _ := o.Post("/api/dbroot/mysql/passwd", `{"confirm":"mariadb"}`); c != 400 {
		t.Fatal("konfirmasi root harus nama mesin")
	}
	if c, _ := o.Post("/api/dbroot/redis/passwd", `{"confirm":"redis"}`); c != 400 {
		t.Fatal("mesin tak dikenal harus 400")
	}
	if c, _ := o.Post("/api/dbroot/mysql/passwd", `{"confirm":"mysql","password":"pendek"}`); c != 400 {
		t.Fatal("sandi root pendek harus 400")
	}
	if c, b := o.Post("/api/dbroot/mysql/passwd", `{"confirm":"mysql","password":"`+pw+`"}`); c != 200 {
		t.Fatalf("root passwd: %d %s", c, b)
	}
	last = o.Calls()[strings.LastIndex(o.Calls(), "CALL:"):]
	if !strings.Contains(last, "[root-passwd]") || strings.Count(last, pw) != 1 {
		t.Fatalf("root: sandi harus hanya di stdin: %s", last)
	}
	// sandi proyek pilihan sendiri
	if c, _ := o.Post("/api/db/mariadb/proj/passwd", `{"confirm":"proj","password":"`+pw+`"}`); c != 200 || !strings.Contains(o.Calls(), "STDIN=["+pw+"]") {
		t.Fatal("sandi proyek pilihan sendiri gagal")
	}
}

func TestAccountActionsNeedStepUpAndRevoke(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.DbFake)
	o.Stepup()
	_, b := o.Post("/api/accounts", `{"name":"budi","role":"admin"}`)
	var mk struct {
		Temp string `json:"temp_password"`
	}
	json.Unmarshal([]byte(b), &mk)
	o.App.Store.SetTempPassword(o.App.Store.GetUser("budi").ID, crypto.HashPassword("sandi-budi-12345678"))
	o.App.Store.DB.Exec("UPDATE users SET must_change=0 WHERE name='budi'")
	bc, bs, code := o.Login2("budi", "sandi-budi-12345678")
	if code != 200 {
		t.Fatalf("login budi: %d", code)
	}
	budi := o.App.Store.GetUser("budi")
	o.App.Store.AddToken(budi.ID, "t", "full", 30, o.Now)
	o.App.Store.AddTrusted(budi.ID, "hp", o.Now)
	// step-up kedaluwarsa: semua aksi sensitif harus 403
	o.App.Store.DB.Exec("UPDATE sessions SET stepup=0")
	for _, p := range [][2]string{
		{"/api/accounts", `{"name":"dewi","role":"viewer"}`}, {"/api/accounts/budi/role", `{"role":"viewer"}`}, {"/api/accounts/budi/disable", `{}`},
		{"/api/accounts/budi/reset", `{}`}, {"/api/accounts/budi/remove", `{"confirm":"budi"}`}, {"/api/dbroot/mysql/passwd", `{"confirm":"mysql"}`},
		{"/api/db/mariadb/proj/dbusers", `{"user":"abc","privs":"ro"}`},
	} {
		if c, b := o.Post(p[0], p[1]); c != 403 || !strings.Contains(b, "kata sandi ulang") {
			t.Fatalf("%s tanpa step-up harus 403: %d %s", p[0], c, b)
		}
	}
	if o.App.Store.GetUser("budi") == nil || o.App.Store.GetUser("budi").MustChange {
		t.Fatal("aksi tanpa step-up tidak boleh mengubah apa pun")
	}
	o.Stepup()
	if c, _ := o.Post("/api/accounts/budi/role", `{"role":"viewer"}`); c != 200 {
		t.Fatal("role gagal")
	}
	role := ""
	for _, r := range o.App.Store.AccountRows("admin") {
		if r.Name == "budi" {
			role = r.Role
		}
	}
	if role != "viewer" {
		t.Fatalf("peran tidak tersimpan: %q", role)
	}
	if c, _ := o.As(bc, bs, "GET", "/api/me", ""); c != 401 {
		t.Fatalf("sesi budi harus dicabut saat peran berubah: %d", c)
	}
	// reset mencabut sesi, perangkat tepercaya, token
	bc, bs, _ = o.Login2("budi", "sandi-budi-12345678")
	o.App.Store.AddToken(budi.ID, "t2", "full", 30, o.Now)
	o.App.Store.AddTrusted(budi.ID, "hp2", o.Now)
	if c, _ := o.Post("/api/accounts/budi/reset", `{}`); c != 200 {
		t.Fatal("reset gagal")
	}
	if c, _ := o.As(bc, bs, "GET", "/api/me", ""); c != 401 {
		t.Fatalf("sesi harus dicabut saat reset: %d", c)
	}
	var nt, ntr int
	o.App.Store.DB.QueryRow("SELECT COUNT(*) FROM tokens WHERE user_id=?", budi.ID).Scan(&nt)
	o.App.Store.DB.QueryRow("SELECT COUNT(*) FROM trusted WHERE user_id=?", budi.ID).Scan(&ntr)
	if nt != 0 || ntr != 0 {
		t.Fatalf("token (%d) atau perangkat tepercaya (%d) tidak dicabut saat reset", nt, ntr)
	}
}

func TestAsyncJobLifecycleAndStaleMarking(t *testing.T) {
	o := testenv.NewP3Env(t)
	c, b := o.Post("/api/php/8.4/install", `{}`)
	if c != 202 {
		t.Fatalf("harus 202: %d %s", c, b)
	}
	id := testenv.JobID(t, b)
	j := o.WaitJob(id)
	if j.Status != "ok" || !strings.Contains(o.Calls(), "CALL:[php][install][8.4]") {
		t.Fatalf("job: %+v calls=%s", j, o.Calls())
	}
	g := o.Do("GET", "/api/jobs/"+testenv.Itoa(id), "", o.Cookie, "", nil)
	if g.Code != 200 || !strings.Contains(g.Body.String(), `"status":"ok"`) {
		t.Fatalf("GET job: %d %s", g.Code, g.Body)
	}
	if w := o.Do("GET", "/api/jobs/99999", "", o.Cookie, "", nil); w.Code != 404 {
		t.Fatalf("job tak ada: %d", w.Code)
	}
	for _, p := range []string{"abc", "-1", "0", "1;2"} {
		if w := o.Do("GET", "/api/jobs/"+p, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("/api/jobs/%s -> %d", p, w.Code)
		}
	}
	o.App.Store.AddJob("x", "yatim", "berjalan", "", 0, o.Now)
	o.App.Store.MarkStaleJobs()
	var st string
	o.App.Store.DB.QueryRow("SELECT status FROM jobs WHERE action='yatim'").Scan(&st)
	if st != "terputus" {
		t.Fatalf("job yatim harus 'terputus': %s", st)
	}
}

func TestRuntimeEndpoints(t *testing.T) {
	o := testenv.NewP3Env(t)
	bad := []string{"/api/runtime/ruby/3/install", "/api/runtime/node/22;rm/install", "/api/runtime/node/--help/use", "/api/runtime/node/..%2Fx/install", "/api/runtime/node/latest;x/remove", "/api/runtime/-rf/1/use"}
	o.Stepup() // supaya penolakan benar-benar berasal dari validasi, bukan dari step-up/badan permintaan
	for _, p := range bad {
		body := `{}`
		if strings.HasSuffix(p, "/remove") {
			parts := strings.Split(p, "/")
			body = `{"confirm":"` + parts[3] + "@" + parts[4] + `"}`
		}
		if c, b := o.Post(p, body); c != 400 {
			t.Errorf("%s -> %d %s (harus 400 dari validasi)", p, c, b)
		}
	}
	if o.Calls() != "" {
		t.Fatalf("lab terpanggil: %s", o.Calls())
	}
	c, b := o.Post("/api/runtime/node/24/install", `{}`)
	if c != 202 {
		t.Fatalf("%d", c)
	}
	o.WaitJob(testenv.JobID(t, b))
	if c, _ := o.Post("/api/runtime/node/24/use", `{}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[runtime][use][node][24]") {
		t.Fatalf("use: %d %s", c, o.Calls())
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second) // step-up dari tahap validasi sudah habis
	if c, _ := o.Post("/api/runtime/node/22.23.5/remove", `{"confirm":"node@22.23.5"}`); c != 403 {
		t.Fatalf("remove tanpa step-up: %d", c)
	}
	o.Stepup()
	if c, _ := o.Post("/api/runtime/node/22.23.5/remove", `{"confirm":"node"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	if c, _ := o.Post("/api/runtime/node/22.23.5/remove", `{"confirm":"node@22.23.5"}`); c != 200 || !strings.Contains(o.Calls(), "CALL:[runtime][remove][node][22.23.5]") {
		t.Fatalf("remove: %d", c)
	}
	l := o.Do("GET", "/api/runtime", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(l, `"tool":"node","version":"26.13.1","default":true`) {
		t.Fatalf("daftar: %s", l)
	}
}

func TestUpdatesCheckAndApply(t *testing.T) {
	o := testenv.NewP3Env(t)
	l := o.Do("GET", "/api/updates", "", o.Cookie, "", nil).Body.String()
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
	if c, _ := o.Post("/api/updates/apply", body("mycli", "mycli", false)); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.Stepup()
	before := o.Calls()
	for _, b := range []string{body("mycli", "lain", false), body("x;y", "x;y", false), body("-rf", "-rf", false), body("mise:ruby", "mise:ruby", false), body("../x", "../x", false), body("", "", false)} {
		if c, _ := o.Post("/api/updates/apply", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if o.Calls() != before {
		t.Fatal("lab terpanggil oleh input buruk")
	}
	c, b := o.Post("/api/updates/apply", body("postgresql@17", "postgresql@17", true))
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[update][apply][postgresql@17][--allow-pinned]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	o.Stepup()
	_, b = o.Post("/api/updates/apply", body("mise:pnpm", "mise:pnpm", false))
	o.WaitJob(testenv.JobID(t, b))
	if !strings.Contains(o.Calls(), "CALL:[update][apply][mise:pnpm] STDIN") || strings.Contains(strings.Split(o.Calls(), "mise:pnpm")[1], "--allow-pinned") {
		t.Fatalf("mise argv: %s", o.Calls())
	}
}

func TestExtActionGuards(t *testing.T) {
	o := testenv.NewDevEnv(t)
	before := o.Calls()
	if c, _ := o.Post("/api/php/8.4/ext/redis/install", `{}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	for _, p := range []string{"/api/php/8.4/ext/swoole/install", "/api/php/8.4/ext/redis/exec", "/api/php/8.4/ext/re;id/install", "/api/php/8.4/ext/-f/install", "/api/php/9.9/ext/redis/install", "/api/php/8.4/ext/REDIS/install"} {
		if c, _ := o.Post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
	if c, _ := o.Post("/api/php/8.4/ext/redis/install", `{"x":1}`); c != 400 {
		t.Errorf("field asing harus 400: %d", c)
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.Calls())
	}
	for _, act := range []string{"install", "remove", "enable", "disable"} {
		c, b := o.Post("/api/php/8.4/ext/redis/"+act, `{}`)
		if c != 202 {
			t.Fatalf("%s: %d %s", act, c, b)
		}
		o.WaitJob(testenv.JobID(t, b))
		if !strings.Contains(o.Calls(), "CALL:[ext]["+act+"][8.4][redis]") {
			t.Errorf("argv %s: %s", act, o.Calls())
		}
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	if c, _ := o.Post("/api/php/8.4/ext/redis/remove", `{}`); c != 403 {
		t.Fatalf("step-up kedaluwarsa harus 403: %d", c)
	}
}

func TestSetRoleRevokesSessions(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if o.App.Store.SetRole("tidak-ada", "viewer") {
		t.Error("user tidak ada harus false")
	}
	name := o.App.Store.UserRows()[0].Name
	for _, bad := range []string{"root", "", "ADMIN", "admin;x"} {
		if o.App.Store.SetRole(name, bad) {
			t.Errorf("peran %q harus ditolak", bad)
		}
	}
	if w := o.Do("GET", "/api/me", "", o.Cookie, "", nil); w.Code != 200 {
		t.Fatalf("sesi harus masih hidup: %d", w.Code)
	}
	if !o.App.Store.SetRole(name, "viewer") {
		t.Fatal("SetRole gagal")
	}
	if w := o.Do("GET", "/api/me", "", o.Cookie, "", nil); w.Code != 401 {
		t.Errorf("sesi harus dicabut setelah ganti peran: %d", w.Code)
	}
}

func TestDestructiveNeedsConfirmAndStepUp(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if c, _ := o.Post("/api/sites/demo/remove", `{"confirm":"demo"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	if c, _ := o.Post("/api/db/mysql/proj/drop", `{"confirm":"proj"}`); c != 403 {
		t.Fatalf("drop tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	if c, _ := o.Post("/api/db/mysql/proj/drop", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi salah harus 400: %d", c)
	}
	if o.Calls() != "" {
		t.Fatal("lab terpanggil sebelum syarat terpenuhi")
	}
	if c, b := o.Post("/api/db/mysql/proj/drop", `{"confirm":"proj"}`); c != 200 {
		t.Fatalf("drop sah: %d %s", c, b)
	}
	if !strings.Contains(o.Calls(), "CALL:[db][drop][proj][--engine][mysql] STDIN=[proj]") {
		t.Fatalf("argv/stdin drop salah: %s", o.Calls())
	}
	// step-up kedaluwarsa
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	if c, _ := o.Post("/api/sites/demo/remove", `{"confirm":"demo"}`); c != 403 {
		t.Fatalf("step-up kedaluwarsa harus 403: %d", c)
	}
}

func TestSessionIdleAndAbsoluteExpiry(t *testing.T) {
	e := testenv.NewEnv(t)
	c, _, _ := e.Login(e.Code(0))
	e.Now = e.Now.Add(store.IdleTTL - time.Minute)
	if w := e.Do("GET", "/api/me", "", c, "", nil); w.Code != 200 {
		t.Fatal("sesi aktif ditolak")
	}
	e.Now = e.Now.Add(store.IdleTTL + time.Second)
	if w := e.Do("GET", "/api/me", "", c, "", nil); w.Code != 401 {
		t.Fatal("sesi idle tidak kedaluwarsa")
	}
	c2, _, _ := e.Login(e.Code(0))
	for i := 0; i < 30; i++ { // tetap aktif tiap 25 menit, tapi lewati batas absolut 12 jam
		e.Now = e.Now.Add(25 * time.Minute)
		e.Do("GET", "/api/me", "", c2, "", nil)
	}
	if w := e.Do("GET", "/api/me", "", c2, "", nil); w.Code != 401 {
		t.Fatal("batas absolut 12 jam tidak ditegakkan")
	}
}

func TestPwdStageExpires(t *testing.T) {
	e := testenv.NewEnv(t)
	w1 := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	e.Now = e.Now.Add(store.PwdTTL + time.Second)
	if w := e.Do("POST", "/api/auth/totp", `{"code":"`+e.Code(0)+`"}`, testenv.CookieOf(w1), testenv.CsrfOf(w1), nil); w.Code != 401 {
		t.Fatalf("tahap password harus kedaluwarsa: %d", w.Code)
	}
}

func TestStepUp(t *testing.T) {
	e := testenv.NewEnv(t)
	c, cs, _ := e.Login(e.Code(0))
	ss := e.App.Session(httptest.NewRequest("GET", "/", nil))
	_ = ss
	if m := e.Do("GET", "/api/me", "", c, "", nil); !strings.Contains(m.Body.String(), `"stepup":false`) {
		t.Fatal("stepup harus false di awal")
	}
	if w := e.Do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, c, cs, nil); w.Code != 200 {
		t.Fatalf("stepup: %d %s", w.Code, w.Body)
	}
	if m := e.Do("GET", "/api/me", "", c, "", nil); !strings.Contains(m.Body.String(), `"stepup":true`) {
		t.Fatal("stepup tidak tercatat")
	}
	e.Now = e.Now.Add(store.StepupTTL + time.Second)
	if m := e.Do("GET", "/api/me", "", c, "", nil); !strings.Contains(m.Body.String(), `"stepup":false`) {
		t.Fatal("stepup tidak kedaluwarsa")
	}
}

func TestAuditChainDetectsTamper(t *testing.T) {
	e := testenv.NewEnv(t)
	e.Login("000000")
	e.Login(e.Code(0))
	n, err := e.App.Store.VerifyAudit()
	if err != nil || n < 3 {
		t.Fatalf("rantai harus utuh: n=%d err=%v", n, err)
	}
	db, _ := sql.Open("sqlite", "file:"+e.Dbp)
	db.Exec("UPDATE audit SET action='nothing' WHERE id=2")
	db.Close()
	e.App.Store.DB.Close()
	st2, _ := store.OpenStore(e.Dbp)
	if _, err := st2.VerifyAudit(); err == nil {
		t.Fatal("perubahan baris audit tidak terdeteksi")
	}
}

// Penjaga atomik: dua permintaan bersamaan dengan kode yang sama, hanya satu boleh menang.
func TestUseTOTPStepIsAtomic(t *testing.T) {
	e := testenv.NewEnv(t)
	u := e.App.Store.GetUser("admin")
	if !e.App.Store.UseTOTPStep(u.ID, 100) {
		t.Fatal("langkah pertama harus diterima")
	}
	if e.App.Store.UseTOTPStep(u.ID, 100) || e.App.Store.UseTOTPStep(u.ID, 99) {
		t.Fatal("langkah yang sama atau lebih lama harus ditolak")
	}
	if !e.App.Store.UseTOTPStep(u.ID, 101) {
		t.Fatal("langkah lebih baru harus diterima")
	}
}

func TestRealSiteLifecycle(t *testing.T) {
	if os.Getenv("PANEL_REAL") != "3" {
		t.Skip("set PANEL_REAL=3")
	}
	o := testenv.NewOpsEnv(t, "x")
	home, _ := os.UserHomeDir()
	a := o.App
	a.LabBin, a.Home, a.SitesDir = "/opt/homebrew/bin/lab", home, home+"/MacPanel/server/nginx/sites"
	a.NginxDir, a.CertDir, a.LogDir, a.ErrLog = home+"/MacPanel/server/nginx", "/opt/homebrew/etc/nginx/certs", "/opt/homebrew/var/log/nginx/sites", "/opt/homebrew/var/log/nginx/error.log"
	a.Timeout = 90 * time.Second
	dir := home + "/MacPanel/wwwroot/.paneltest/public"
	os.MkdirAll(dir, 0o755)
	os.WriteFile(dir+"/index.html", []byte("PANELTEST-INDEX"), 0o644)
	os.WriteFile(dir+"/.env", []byte("RAHASIA"), 0o644)
	cleanup := func() {
		exec.Command("/opt/homebrew/bin/lab", "site", "remove", "paneltest").Run()
		os.RemoveAll(home + "/MacPanel/wwwroot/.paneltest")
		os.Remove(a.LogDir + "/paneltest.log")
	}
	cleanup2 := cleanup
	defer cleanup2()
	step := func(n string, ok bool, extra ...any) {
		t.Helper()
		if !ok {
			cf, _ := os.ReadFile(home + "/MacPanel/server/nginx/sites/paneltest.conf")
			el, _ := exec.Command("tail", "-n", "4", a.ErrLog).Output()
			t.Logf("KONFIG:\n%s\nERROR LOG:\n%s", cf, el)
			t.Fatalf("LANGKAH GAGAL: %s %v", n, extra)
		}
		t.Log("LULUS:", n)
	}
	lan, _ := exec.Command("ipconfig", "getifaddr", "en1").Output()
	lanIP := strings.TrimSpace(string(lan))

	c, b := o.Post("/api/sites", `{"name":"paneltest","target":"`+dir+`"}`)
	step("buat situs lewat panel -> lab -> addsite", c == 200, c, b)
	g := o.Do("GET", "/api/sites/paneltest/settings", "", o.Cookie, "", nil).Body.String()
	step("pengaturan terbaca sebagai legacy/statis", strings.Contains(g, `"kind":"legacy"`) && strings.Contains(g, `"mode":"statis"`), g)

	c, b = o.Post("/api/sites/paneltest/settings", `{"mode":"statis","target":"`+dir+`","preset":"spa","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":true,"rewrites":[{"from":"^/old$","to":"/new","flag":"permanent"}]}`)
	step("simpan pengaturan (SPA + rewrite) -> nginx -t -> reload NYATA", c == 200, c, b)
	var code int
	var body string
	var waited time.Duration
	for start := time.Now(); ; {
		code, body, _ = testenv.RealGet(t, "127.0.0.1", "paneltest.lab", "/dalam/rute/spa")
		waited = time.Since(start)
		if code == 200 || waited > 5*time.Second {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("config baru aktif setelah %v (reload graceful)", waited)
	step("SPA fallback bekerja di nginx nyata", code == 200 && strings.Contains(body, "PANELTEST-INDEX"), code, body)
	var loc string
	code, _, loc = testenv.RealGet(t, "127.0.0.1", "paneltest.lab", "/old")
	step("rewrite permanent bekerja (301 -> /new)", code == 301 && strings.HasSuffix(loc, "/new"), code, loc)
	code, body, _ = testenv.RealGet(t, "127.0.0.1", "paneltest.lab", "/.env")
	step("dotfile /.env diblokir (404) dan isinya tidak bocor", code == 404 && !strings.Contains(body, "RAHASIA"), code)
	if lanIP != "" {
		code, _, _ = testenv.RealGet(t, lanIP, "paneltest.lab", "/")
		step("dari IP LAN tetap 403 (tailnet_only dipertahankan template)", code == 403, code)
	}

	o.Stepup()
	cur := o.SiteConf("paneltest")
	_ = cur
	path := home + "/MacPanel/server/nginx/sites/paneltest.conf"
	read := func() string { b, _ := os.ReadFile(path); return string(b) }
	before := read()
	bad := strings.Replace(before, "    client_max_body_size 10m;", "    client_max_body_size 5x;", 1)
	step("fixture edit rusak siap", bad != before)
	c, b = o.Post("/api/sites/paneltest/config", `{"sha":"`+crypto.Sha256hex(before)+`","content":`+testenv.JsonQuote(bad)+`}`)
	step("edit yang lolos kebijakan tetapi ditolak nginx -t => 422 + rollback", c == 422 && strings.Contains(b, "config lama dipulihkan"), c, b)
	step("file config IDENTIK byte-per-byte setelah rollback", read() == before)
	code, body, _ = testenv.RealGet(t, "127.0.0.1", "paneltest.lab", "/x")
	step("situs tetap melayani setelah rollback (nginx tidak rusak)", code == 200 && strings.Contains(body, "PANELTEST-INDEX"), code)

	evil := strings.Replace(before, "    client_max_body_size 10m;", "    location /boc { allow all; }", 1)
	c, b = o.Post("/api/sites/paneltest/config", `{"sha":"`+crypto.Sha256hex(before)+`","content":`+testenv.JsonQuote(evil)+`}`)
	step("edit 'allow all' ditolak kebijakan (400), nginx tidak disentuh", c == 400 && read() == before, c, b)

	good := strings.Replace(before, "    client_max_body_size 10m;", "    client_max_body_size 20m;\n    add_header X-Panel-Test ok always;", 1)
	c, b = o.Post("/api/sites/paneltest/config", `{"sha":"`+crypto.Sha256hex(before)+`","content":`+testenv.JsonQuote(good)+`}`)
	step("edit config sah diterapkan (200)", c == 200 && read() == good, c, b)

	vs := a.Store.Versions("paneltest")
	var firstID int64
	for _, v := range vs {
		if cc, _ := a.Store.VersionContent("paneltest", v.ID); cc == before {
			firstID = v.ID
		}
	}
	step("versi sebelum edit tersimpan", firstID != 0, vs)
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	o.Stepup()
	c, b = o.Post("/api/sites/paneltest/restore", `{"id":`+testenv.Itoa(firstID)+`}`)
	step("pulihkan versi lama (step-up) => file kembali", c == 200 && read() == before, c, b)

	s := o.Do("GET", "/api/sites/paneltest/ssl", "", o.Cookie, "", nil).Body.String()
	step("info SSL nyata (mkcert, masih berlaku)", strings.Contains(s, `"exists":true`) && strings.Contains(s, `"expired":false`), s)
	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	o.Stepup()
	c, b = o.Post("/api/sites/paneltest/ssl/renew", `{}`)
	step("perbarui sertifikat (lab -> mkcert -> reload)", c == 200, c, b)
	code, _, _ = testenv.RealGet(t, "127.0.0.1", "paneltest.lab", "/")
	step("situs tetap 200 dengan sertifikat baru", code == 200, code)

	lg := o.Do("GET", "/api/sites/paneltest/logs?kind=access&n=50", "", o.Cookie, "", nil).Body.String()
	step("log akses per situs berisi permintaan tadi", strings.Contains(lg, "/dalam/rute/spa"), lg)

	o.Now = o.Now.Add(store.StepupTTL + time.Second)
	o.Stepup()
	c, b = o.Post("/api/sites/paneltest/remove", `{"confirm":"paneltest"}`)
	step("hapus situs lewat panel (step-up + konfirmasi) => lab", c == 200, c, b)
	_, err := os.Stat(path)
	step("file config terhapus", err != nil)
	for start := time.Now(); time.Since(start) < 5*time.Second; time.Sleep(50 * time.Millisecond) {
		if code, _, _ = testenv.RealGet(t, "127.0.0.1", "paneltest.lab", "/"); code != 200 {
			break
		}
	}
	step("situs tidak lagi dilayani (setelah reload graceful)", code != 200, code)
}

func TestNginxTestFailureRollsBackByteIdentical(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	os.WriteFile(filepath.Join(o.Home, "sites", "toko.conf"), []byte(o.Base()), 0o644)
	orig := o.SiteConf("toko")
	o.Stepup()
	os.WriteFile(filepath.Join(o.Home, "fail_test"), nil, 0o644)
	edit := strings.Replace(orig, "try_files $uri =404;", "try_files $uri /index.html;", 1)
	c, b := o.RawEdit("toko", edit)
	if c != 422 || !strings.Contains(b, "config lama dipulihkan") {
		t.Fatalf("harus 422 + pesan rollback: %d %s", c, b)
	}
	if o.SiteConf("toko") != orig {
		t.Fatal("ROLLBACK GAGAL: file tidak identik dengan sebelumnya")
	}
	if strings.Contains(o.Calls(), "[reload]") {
		t.Fatal("reload tidak boleh dipanggil setelah nginx -t gagal")
	}
	if vs := o.App.Store.Versions("toko"); len(vs) != 1 || !strings.HasPrefix(vs[0].Note, "sebelum:") {
		t.Fatalf("hanya versi 'sebelum' yang boleh ada: %+v", vs)
	}
}

func TestRestoreVersion(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	os.WriteFile(filepath.Join(o.Home, "sites", "toko.conf"), []byte(o.Base()), 0o644)
	orig := o.SiteConf("toko")
	o.Stepup()
	o.RawEdit("toko", strings.Replace(orig, "try_files $uri =404;", "try_files $uri /index.html;", 1))
	vs := o.App.Store.Versions("toko")
	var origID int64
	for _, v := range vs {
		if c, _ := o.App.Store.VersionContent("toko", v.ID); c == orig {
			origID = v.ID
		}
	}
	if origID == 0 {
		t.Fatalf("versi asli tidak tersimpan: %+v", vs)
	}
	o.Now = o.Now.Add(store.StepupTTL + time.Second) // step-up habis
	if c, _ := o.Post("/api/sites/toko/restore", `{"id":`+testenv.Itoa(origID)+`}`); c != 403 {
		t.Fatalf("restore tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	if c, b := o.Post("/api/sites/toko/restore", `{"id":`+testenv.Itoa(origID)+`}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if o.SiteConf("toko") != orig {
		t.Fatal("versi tidak dipulihkan")
	}
	if c, _ := o.Post("/api/sites/toko/restore", `{"id":99999}`); c != 404 {
		t.Fatalf("id tak ada: %d", c)
	}
	// versi yang kini melanggar kebijakan tidak boleh dipulihkan
	o.App.Store.DB.Exec("INSERT INTO conf_versions(site,at,usr,sha,note,content) VALUES('toko',1,'x','h','n',?)", "server { allow all; }")
	var badID int64
	o.App.Store.DB.QueryRow("SELECT MAX(id) FROM conf_versions").Scan(&badID)
	if c, _ := o.Post("/api/sites/toko/restore", `{"id":`+testenv.Itoa(badID)+`}`); c != 400 {
		t.Fatalf("versi berbahaya harus ditolak: %d", c)
	}
}

func TestVersionsPrunedTo20AndDeduped(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	for i := 0; i < 30; i++ {
		o.App.Store.AddVersion("toko", "u", "isi-"+testenv.Itoa(int64(i)), "n", o.Now)
	}
	o.App.Store.AddVersion("toko", "u", "isi-29", "n", o.Now) // duplikat berturut-turut
	if n := len(o.App.Store.Versions("toko")); n != 20 {
		t.Fatalf("harus 20 versi: %d", n)
	}
}

func TestAccountCreateWithChosenPassword(t *testing.T) {
	o := testenv.NewP3Env(t)
	o.Stepup()
	for _, bad := range []string{`{"name":"rina","password":"pendek"}`, `{"name":"rinarinarina","password":"rinarinarina"}`} {
		if c, _ := o.Post("/api/accounts", bad); c != 400 {
			t.Errorf("harus 400: %s -> %d", bad, c)
		}
	}
	c, b := o.Post("/api/accounts", `{"name":"rina","password":"sandi-pilihan-rina1"}`)
	if c != 200 || strings.Contains(b, "sandi-pilihan-rina1") || strings.Contains(b, "temp_password") {
		t.Fatalf("buat: %d %s", c, b)
	}
	if u := o.App.Store.GetUser("rina"); u == nil || !u.MustChange {
		t.Fatal("akun harus ada dan wajib ganti sandi")
	}
	c, b = o.Post("/api/accounts", `{"name":"andi"}`)
	if c != 200 || !strings.Contains(b, "temp_password") {
		t.Fatalf("tanpa sandi harus dibuat otomatis: %d %s", c, b)
	}
}

func TestMyProfile(t *testing.T) {
	o := testenv.NewP3Env(t)
	png := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	ok := []string{`{"display_name":"Roki Anjas","avatar":"icon:rocket"}`, `{"display_name":"","avatar":""}`, `{"display_name":"R","avatar":"` + png + `"}`}
	for _, b := range ok {
		if c, r := o.Post("/api/me/profile", b); c != 200 {
			t.Errorf("harus 200: %.80s -> %d %s", b, c, r)
		}
	}
	bad := []string{
		`{"display_name":"<script>","avatar":""}`, `{"display_name":"a\nb","avatar":""}`, `{"display_name":"` + strings.Repeat("x", 41) + `","avatar":""}`,
		`{"display_name":"","avatar":"javascript:alert(1)"}`, `{"display_name":"","avatar":"https://evil/x.png"}`, `{"display_name":"","avatar":"data:image/svg+xml;base64,PHN2Zz4="}`,
		`{"display_name":"","avatar":"data:image/png;base64,AAAA\" onerror=\"x"}`, `{"display_name":"","avatar":"icon:../x"}`, `{"display_name":"","avatar":"data:image/png;base64,` + strings.Repeat("A", 60001) + `"}`, `{"display_name":"","avatar":"","x":1}`,
	}
	for _, b := range bad {
		if c, _ := o.Post("/api/me/profile", b); c != 400 && c != 413 {
			t.Errorf("harus ditolak: %.90s -> %d", b, c)
		}
	}
	o.Post("/api/me/profile", `{"display_name":"Roki Anjas","avatar":"icon:rocket"}`)
	c, b := o.As(o.Cookie, o.Csrf, "GET", "/api/me", "")
	if c != 200 || !strings.Contains(b, `"display_name":"Roki Anjas"`) || !strings.Contains(b, `"avatar":"icon:rocket"`) {
		t.Fatalf("/api/me: %d %s", c, b)
	}
}

func TestMyLang(t *testing.T) {
	o := testenv.NewP3Env(t)
	for _, ok := range []string{`{"lang":"en"}`, `{"lang":"pt-br"}`, `{"lang":""}`} {
		if c, _ := o.Post("/api/me/lang", ok); c != 200 {
			t.Errorf("harus 200: %s -> %d", ok, c)
		}
	}
	for _, bad := range []string{`{"lang":"EN"}`, `{"lang":"en;drop"}`, `{"lang":"../x"}`, `{"lang":"abcdefgh"}`, `{"x":1}`} {
		if c, _ := o.Post("/api/me/lang", bad); c != 400 {
			t.Errorf("harus 400: %s -> %d", bad, c)
		}
	}
	o.Post("/api/me/lang", `{"lang":"en"}`)
	if _, b := o.As(o.Cookie, o.Csrf, "GET", "/api/me", ""); !strings.Contains(b, `"lang":"en"`) {
		t.Fatalf("/api/me: %s", b)
	}
}
