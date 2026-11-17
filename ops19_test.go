package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func (o *opsEnv) bearer(method, path, body, tok string) (int, string) {
	w := o.do(method, path, body, "", "", map[string]string{"Authorization": "Bearer " + tok})
	return w.Code, w.Body.String()
}

func (o *opsEnv) mkToken(scope string) (string, int64) {
	o.t.Helper()
	code, body := o.post("/api/me/tokens", `{"name":"agen `+scope+`","scope":"`+scope+`","days":90}`)
	if code != 200 {
		o.t.Fatalf("buat token %s: %d %s", scope, code, body)
	}
	var r struct {
		Token string
		ID    int64
	}
	json.Unmarshal([]byte(body), &r)
	if !strings.HasPrefix(r.Token, tokenPrefix) {
		o.t.Fatalf("format token salah: %q", r.Token)
	}
	return r.Token, r.ID
}

func TestAPITokenScopes(t *testing.T) {
	o := newOpsEnv(t, "")
	// membuat token butuh kata sandi ulang
	if code, _ := o.post("/api/me/tokens", `{"name":"x","scope":"read"}`); code != 403 {
		t.Fatalf("buat token tanpa step-up harus 403, dapat %d", code)
	}
	o.stepup()
	rd, _ := o.mkToken("read")
	op, _ := o.mkToken("operate")
	fu, fuID := o.mkToken("full")

	if c, _ := o.bearer("GET", "/api/me", "", "mpt_salah-salah-salah-salah"); c != 401 {
		t.Fatalf("token palsu harus 401, dapat %d", c)
	}
	if c, _ := o.bearer("GET", "/api/me", "", "bukan-token-sama-sekali-xxxx"); c != 401 {
		t.Fatalf("format salah harus 401, dapat %d", c)
	}
	// baca-saja
	if c, b := o.bearer("GET", "/api/me", "", rd); c != 200 || !strings.Contains(b, `"via":"token"`) || !strings.Contains(b, `"scope":"read"`) {
		t.Fatalf("whoami token: %d %s", c, b)
	}
	if c, _ := o.bearer("GET", "/api/sites", "", rd); c != 200 {
		t.Fatalf("GET lewat token read harus 200, dapat %d", c)
	}
	if c, b := o.bearer("POST", "/api/settings", `{"name":"Dilarang"}`, rd); c != 403 || !strings.Contains(b, "baca-saja") {
		t.Fatalf("token read tidak boleh menulis: %d %s", c, b)
	}
	// operasi: boleh ubah pengaturan, tidak boleh aksi step-up
	if c, b := o.bearer("POST", "/api/settings", `{"name":"Lab Saya"}`, op); c != 200 {
		t.Fatalf("operate ubah pengaturan: %d %s", c, b)
	}
	if w := o.do("GET", "/api/settings/public", "", "", "", nil); !strings.Contains(w.Body.String(), "Lab Saya") {
		t.Fatalf("nama panel tidak berubah: %s", w.Body)
	}
	delBody := `{"db":"0","key":"app:cache"}`
	if c, b := o.bearer("POST", "/api/redis/del", delBody, op); c != 403 || !strings.Contains(b, "full") {
		t.Fatalf("operate tidak boleh aksi step-up: %d %s", c, b)
	}
	if strings.Contains(o.calls(), "[redis][del]") {
		t.Fatal("lab dipanggil padahal token operate")
	}
	// penuh: aksi step-up lewat
	if c, b := o.bearer("POST", "/api/redis/del", delBody, fu); c != 200 {
		t.Fatalf("token full harus lolos: %d %s", c, b)
	}
	if !strings.Contains(o.calls(), "[redis][del]") {
		t.Fatalf("lab tidak dipanggil oleh token full: %s", o.calls())
	}
	// tidak pernah lewat token, bahkan full
	for _, p := range [][2]string{
		{"POST", "/api/auth/stepup"}, {"POST", "/api/me/password"}, {"GET", "/api/me/tokens"}, {"POST", "/api/me/tokens"},
		{"POST", "/api/me/2fa/disable"}, {"GET", "/api/me/devices"}, {"POST", "/api/db/mysql/proj/creds"}, {"GET", "/api/sites/demo/env"}, {"POST", "/api/sites/demo/env"},
	} {
		if c, b := o.bearer(p[0], p[1], `{}`, fu); c != 403 || !strings.Contains(b, "tidak tersedia lewat token") {
			t.Fatalf("%s %s harus 403 untuk token full: %d %s", p[0], p[1], c, b)
		}
	}
	// daftar rute tidak memuat yang terlarang
	c, b := o.bearer("GET", "/api/routes", "", rd)
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
	for _, r := range o.app.st.RecentAudit(50) {
		if r.Action == "api_call" && strings.Contains(r.Detail, "POST /api/redis/del") {
			found = true
		}
	}
	if !found {
		t.Fatal("api_call tidak tercatat di audit")
	}
	// cabut -> langsung ditolak
	if c, _ := o.post("/api/me/tokens/revoke", `{"id":`+itoaTok(fuID)+`}`); c != 200 {
		t.Fatal("cabut gagal")
	}
	if c, _ := o.bearer("GET", "/api/me", "", fu); c != 401 {
		t.Fatalf("token yang dicabut harus 401, dapat %d", c)
	}
	// kedaluwarsa
	o.app.now = func() time.Time { return o.now.Add(91 * 24 * time.Hour) }
	if c, _ := o.bearer("GET", "/api/me", "", rd); c != 401 {
		t.Fatalf("token kedaluwarsa harus 401, dapat %d", c)
	}
}

func itoaTok(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestAPITokenLimitsAndValidation(t *testing.T) {
	o := newOpsEnv(t, "")
	o.stepup()
	for _, bad := range []string{`{"name":"","scope":"read"}`, `{"name":"<b>","scope":"read"}`, `{"name":"ok","scope":"root"}`, `{"name":"ok","scope":"read","days":400}`, `{"name":"ok","scope":"read","days":-1}`} {
		if c, _ := o.post("/api/me/tokens", bad); c != 400 {
			t.Fatalf("%s harus 400, dapat %d", bad, c)
		}
	}
	for i := 0; i < maxTokens; i++ {
		o.mkToken("read")
	}
	if c, _ := o.post("/api/me/tokens", `{"name":"lebih","scope":"read"}`); c != 409 {
		t.Fatalf("lewat batas harus 409, dapat %d", c)
	}
	// hanya hash yang disimpan
	var n int
	o.app.st.db.QueryRow("SELECT COUNT(*) FROM tokens WHERE hash LIKE 'mpt_%'").Scan(&n)
	if n != 0 {
		t.Fatal("token mentah tersimpan di database")
	}
	// akun hanya-baca: token otomatis baca-saja
	o.app.st.SetRole("admin", "viewer")
	o.app.st.db.Exec("DELETE FROM tokens")
	tok, _, _ := o.app.st.AddToken(1, "v", "full", 30, o.now)
	if c, b := o.bearer("POST", "/api/settings", `{"name":"X"}`, tok); c != 403 {
		t.Fatalf("viewer lewat token tidak boleh menulis: %d %s", c, b)
	}
	if c, b := o.bearer("GET", "/api/me", "", tok); c != 200 || !strings.Contains(b, `"scope":"read"`) {
		t.Fatalf("token full milik viewer harus turun ke read: %d %s", c, b)
	}
}

func TestTwoFactorToggle(t *testing.T) {
	e := newEnv(t)
	ck, cs, _ := e.login(e.code(0))
	e.now = e.now.Add(time.Minute)
	if w := e.do("GET", "/api/me/2fa", "", ck, "", nil); !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatalf("2FA harus aktif: %s", w.Body)
	}
	// salah kata sandi / salah kode: ditolak, 2FA tetap aktif
	bad := `{"password":"salah-salah-salah","code":"` + e.code(1) + `"}`
	if w := e.do("POST", "/api/me/2fa/disable", bad, ck, cs, nil); w.Code != 400 {
		t.Fatalf("kata sandi salah harus 400: %d", w.Code)
	}
	if w := e.do("POST", "/api/me/2fa/disable", `{"password":"kata-sandi-panjang-123","code":"000000"}`, ck, cs, nil); w.Code != 400 {
		t.Fatalf("kode salah harus 400: %d", w.Code)
	}
	if e.app.st.GetUser("admin").TOTPOn != true {
		t.Fatal("2FA mati padahal permintaan ditolak")
	}
	// trusted device + sesi lain harus tercabut
	_, _, tok := e.trustLogin(true)
	if tok == "" {
		t.Fatal("setup: perangkat tepercaya tidak terbentuk")
	}
	e.now = e.now.Add(time.Minute)
	good := `{"password":"kata-sandi-panjang-123","code":"` + e.code(0) + `"}`
	if w := e.do("POST", "/api/me/2fa/disable", good, ck, cs, nil); w.Code != 200 {
		t.Fatalf("matikan 2FA: %d %s", w.Code, w.Body)
	}
	if w := e.do("POST", "/api/me/2fa/disable", good, ck, cs, nil); w.Code != 409 {
		t.Fatalf("mematikan dua kali harus 409: %d", w.Code)
	}
	// login: kata sandi saja
	w := e.do("POST", "/api/auth/login", loginBody, "", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"no2fa":true`) || cookieOf(w) == "" {
		t.Fatalf("login tanpa 2FA: %d %s", w.Code, w.Body)
	}
	if m := e.do("GET", "/api/me", "", cookieOf(w), "", nil); m.Code != 200 {
		t.Fatalf("sesi tanpa 2FA harus penuh: %d", m.Code)
	}
	// perangkat tepercaya sudah dicabut
	if n := len(e.app.st.ListTrusted(1, "", e.now)); n != 0 {
		t.Fatalf("perangkat tepercaya masih ada: %d", n)
	}
	// nyalakan lagi: butuh step-up, kode harus benar
	ck2, cs2 := cookieOf(w), csrfOf(w)
	if w := e.do("POST", "/api/me/2fa/setup", `{}`, ck2, cs2, nil); w.Code != 403 {
		t.Fatalf("setup tanpa step-up harus 403: %d", w.Code)
	}
	e.do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck2, cs2, nil)
	sw := e.do("POST", "/api/me/2fa/setup", `{}`, ck2, cs2, nil)
	var su struct{ Secret, URI string }
	json.Unmarshal(sw.Body.Bytes(), &su)
	secret, err := b32.DecodeString(su.Secret)
	if sw.Code != 200 || err != nil || len(secret) != 20 || !strings.HasPrefix(su.URI, "otpauth://") {
		t.Fatalf("setup 2FA: %d %s", sw.Code, sw.Body)
	}
	if w := e.do("POST", "/api/me/2fa/enable", `{"code":"123456"}`, ck2, cs2, nil); w.Code != 400 {
		t.Fatalf("kode salah harus 400: %d", w.Code)
	}
	if e.app.st.GetUser("admin").TOTPOn {
		t.Fatal("2FA menyala dengan kode salah")
	}
	e.app.st.AddTrusted(1, "laptop", e.now)
	ew := e.do("POST", "/api/me/2fa/enable", `{"code":"`+totpAt(secret, e.now.Unix()/30)+`"}`, ck2, cs2, nil)
	var en struct {
		Codes []string `json:"recovery_codes"`
	}
	json.Unmarshal(ew.Body.Bytes(), &en)
	if ew.Code != 200 || len(en.Codes) != 10 {
		t.Fatalf("enable 2FA: %d %s", ew.Code, ew.Body)
	}
	if n := len(e.app.st.ListTrusted(1, "", e.now)); n != 0 {
		t.Fatalf("enable harus mencabut perangkat tepercaya: %d", n)
	}
	if w := e.do("POST", "/api/me/2fa/setup", `{}`, ck2, cs2, nil); w.Code != 409 {
		t.Fatalf("setup saat aktif harus 409: %d", w.Code)
	}
	// login kembali meminta 2FA, dengan kunci BARU; recovery code baru berlaku
	if w := e.do("POST", "/api/auth/login", loginBody, "", "", nil); !strings.Contains(w.Body.String(), "need_totp") {
		t.Fatalf("login harus minta 2FA lagi: %s", w.Body)
	}
	lw := e.do("POST", "/api/auth/login", loginBody, "", "", nil)
	tw := e.do("POST", "/api/auth/totp", `{"code":"`+en.Codes[0]+`"}`, cookieOf(lw), csrfOf(lw), nil)
	if tw.Code != 200 {
		t.Fatalf("recovery code baru ditolak: %d %s", tw.Code, tw.Body)
	}
}

func TestPanelSettings(t *testing.T) {
	o := newOpsEnv(t, "")
	if w := o.do("GET", "/api/settings/public", "", "", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), defaultName) {
		t.Fatalf("publik harus memuat nama bawaan tanpa login: %d %s", w.Code, w.Body)
	}
	for _, bad := range []string{`{"name":""}`, `{"name":"<script>"}`, `{"name":"` + strings.Repeat("a", 41) + `"}`, `{"theme":"neon"}`, `{}`} {
		if c, _ := o.post("/api/settings", bad); c != 400 {
			t.Fatalf("%s harus 400, dapat %d", bad, c)
		}
	}
	if c, b := o.post("/api/settings", `{"name":"Dapur Lab 2","theme":"dark"}`); c != 200 || !strings.Contains(b, "Dapur Lab 2") || !strings.Contains(b, "dark") {
		t.Fatalf("simpan: %d %s", c, b)
	}
	if o.app.st.Setting("panel_name", "") != "Dapur Lab 2" {
		t.Fatal("tidak tersimpan")
	}
	// tanpa sesi: tulis ditolak
	if w := o.do("POST", "/api/settings", `{"name":"X"}`, "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa sesi harus 401: %d", w.Code)
	}
}

func TestPanelSettingsAdminOnlyDirect(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest("POST", "/api/settings", strings.NewReader(`{"name":"Diretas"}`))
	w := httptest.NewRecorder()
	hPanelSettingsSet(e.app, w, r, &Session{User: "x", Role: "viewer", Stage: "full"})
	if w.Code != 403 || e.app.st.Setting("panel_name", "") == "Diretas" {
		t.Fatalf("non-admin harus ditolak di handler: %d", w.Code)
	}
}
