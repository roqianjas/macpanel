package server_test

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/server"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestProbeOnlyLoopbackAddr(t *testing.T) {
	o := testenv.NewAppsEnv(t)
	a := server.NewApp(o.App.Store, o.App.Key, core.Config{})
	if a.ProbeAddr != "127.0.0.1:443" {
		t.Fatalf("alamat pemeriksaan harus loopback: %s", a.ProbeAddr)
	}
}

func TestOutputCap(t *testing.T) {
	o := testenv.NewOpsEnv(t, `head -c 600000 /dev/zero | tr '\0' 'a'`)
	r := o.App.RunLab([]string{"x", "y"}, "")
	if len(r.Out) > core.MaxOutput+10 {
		t.Fatalf("output tidak dibatasi: %d", len(r.Out))
	}
}

func TestEnvIsCleared(t *testing.T) {
	o := testenv.NewOpsEnv(t, `echo "SECRET_TEST=${SECRET_TEST:-kosong} HOME=$HOME"`)
	os.Setenv("SECRET_TEST", "bocor")
	defer os.Unsetenv("SECRET_TEST")
	r := o.App.RunLab([]string{"x", "y"}, "")
	if strings.Contains(r.Out, "bocor") || !strings.Contains(r.Out, "kosong") {
		t.Fatalf("env induk bocor ke anak: %s", r.Out)
	}
}

func TestLoginFlowAndHeaders(t *testing.T) {
	e := testenv.NewEnv(t)
	c, cs, w := e.Login(e.Code(0))
	if w.Code != 200 || c == "" || cs == "" {
		t.Fatalf("login penuh gagal: %d", w.Code)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Frame-Options", "Cache-Control"} {
		if w.Header().Get(h) == "" {
			t.Errorf("header %s hilang", h)
		}
	}
	sc := w.Header().Get("Set-Cookie")
	for _, f := range []string{"HttpOnly", "Secure", "SameSite=Strict", core.CookieName} {
		if !strings.Contains(sc, f) {
			t.Errorf("cookie tanpa %s: %s", f, sc)
		}
	}
	if m := e.Do("GET", "/api/me", "", c, "", nil); m.Code != 200 {
		t.Fatalf("/api/me: %d", m.Code)
	}
}

func TestLockout(t *testing.T) {
	e := testenv.NewEnv(t)
	for i := 0; i < core.MaxPerUser; i++ {
		if w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"salah-salah-salah"}`, "", "", nil); w.Code != 401 {
			t.Fatalf("percobaan %d: %d", i, w.Code)
		}
	}
	if w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil); w.Code != 429 {
		t.Fatalf("password BENAR saat terkunci harus 429, dapat %d", w.Code)
	}
	e.Now = e.Now.Add(core.LockWindow + time.Second)
	if w := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil); w.Code != 200 {
		t.Fatalf("setelah jendela lewat harus bisa login: %d", w.Code)
	}
}

func TestLockoutCannotBeBypassedWithForwardedHeader(t *testing.T) {
	e := testenv.NewEnv(t)
	for i := 0; i < core.MaxPerIP; i++ {
		// nama berbeda tiap kali + XFF palsu dari klien non-loopback harus diabaikan
		e.Do("POST", "/api/auth/login", `{"name":"u`+string(rune('a'+i))+`","password":"x"}`, "", "", map[string]string{"X-Forwarded-For": "10.9.9." + string(rune('1'+i))})
	}
	if w := e.Do("POST", "/api/auth/login", `{"name":"lain","password":"x"}`, "", "", map[string]string{"X-Forwarded-For": "1.2.3.4"}); w.Code != 429 {
		t.Fatalf("lockout per-IP bisa dilewati lewat XFF: %d", w.Code)
	}
}

func TestClientIPTrustsOnlyLoopbackProxy(t *testing.T) {
	a := server.NewApp(testenv.NewEnv(t).App.Store, testenv.TestKey, core.Config{})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 100.1.2.3")
	if ip := a.ClientIP(r); ip != "100.1.2.3" {
		t.Fatalf("harus nilai terakhir (dari nginx): %s", ip)
	}
	r.RemoteAddr = "192.168.0.9:1"
	if ip := a.ClientIP(r); ip != "192.168.0.9" {
		t.Fatalf("XFF dari non-loopback harus diabaikan: %s", ip)
	}
}

func TestLargeConfigSavePasses8KBLimit(t *testing.T) {
	o := testenv.NewOpsEnv(t, "echo ok")
	big := strings.Repeat("# komentar panjang untuk menguji batas ukuran body\n", 400) // ~20 KB
	body := `{"content":` + testenv.JsonStr(big) + `,"sha":"x"}`
	if len(body) < 9000 {
		t.Fatal("tes tidak cukup besar")
	}
	os.MkdirAll(o.App.SitesDir, 0o755)
	os.WriteFile(o.App.SiteFile("tesbesar"), []byte("server { }\n"), 0o644) // situs harus ada, kalau tidak 404 muncul sebelum body dibaca
	o.Stepup()                                                              // tanpa ini rute menjawab 403 sebelum body dibaca dan tes tak menguji apa pun
	c, b := o.Post("/api/sites/tesbesar/config", body)
	if c != 409 { // sampai ke pemeriksaan sha (409), bukan ditolak ukuran (413)
		t.Fatalf("body ~20KB tidak sampai ke validasi: %d %s", c, b)
	}
}
