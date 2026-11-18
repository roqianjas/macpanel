package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var paramRE = regexp.MustCompile(`\{[a-z]+\}`)

var testKey = bytes.Repeat([]byte{7}, 32)

type env struct {
	t      *testing.T
	app    *App
	h      http.Handler
	secret []byte
	now    time.Time
	codes  []string
	dbp    string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dbp := filepath.Join(t.TempDir(), "p.db")
	st, err := openStore(dbp)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, now: time.Unix(1_800_000_000, 0), dbp: dbp}
	e.app = newApp(st, testKey, Config{Origins: []string{"https://panel.lab"}})
	e.app.now = func() time.Time { return e.now }
	e.h = e.app.Handler()
	e.secret = newTOTPSecret()
	enc, _ := seal(testKey, e.secret)
	e.codes = newRecoveryCodes(3)
	hs := []string{}
	for _, c := range e.codes {
		hs = append(hs, sha256hex(c))
	}
	if err := st.CreateUser("admin", hashPassword("kata-sandi-panjang-123"), enc, hs); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) do(method, path, body, cookie, csrf string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "100.64.0.20:5555"
	if cookie != "" {
		r.Header.Set("Cookie", cookieName+"="+cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	r.Header.Set("Origin", "https://panel.lab")
	for k, v := range hdr {
		if v == "" {
			r.Header.Del(k)
		} else {
			r.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func cookieOf(w *httptest.ResponseRecorder) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			return c.Value
		}
	}
	return ""
}

func csrfOf(w *httptest.ResponseRecorder) string {
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	s, _ := m["csrf"].(string)
	return s
}

func (e *env) code(offset int) string {
	return totpAt(e.secret, e.now.Unix()/30+int64(offset))
}

func (e *env) login(code string) (cookie, csrf string, w *httptest.ResponseRecorder) {
	w1 := e.do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	if w1.Code != 200 {
		e.t.Fatalf("login password: %d %s", w1.Code, w1.Body)
	}
	c1, cs1 := cookieOf(w1), csrfOf(w1)
	w2 := e.do("POST", "/api/auth/totp", `{"code":"`+code+`"}`, c1, cs1, nil)
	if w2.Code != 200 {
		return "", "", w2
	}
	return cookieOf(w2), csrfOf(w2), w2
}

// ---------- kripto ----------

func TestTOTPRFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		if got := totpAt(secret, ts/30); got != want {
			t.Errorf("T=%d: got %s want %s", ts, got, want)
		}
	}
}

func TestTOTPReplayAndWindow(t *testing.T) {
	s := []byte("12345678901234567890")
	now := time.Unix(1111111109, 0)
	step, ok := verifyTOTP(s, "081804", now, 0)
	if !ok || step != 1111111109/30 {
		t.Fatal("kode valid ditolak")
	}
	if _, ok := verifyTOTP(s, "081804", now, step); ok {
		t.Fatal("replay diterima")
	}
	if _, ok := verifyTOTP(s, "000000", now, 0); ok {
		t.Fatal("kode salah diterima")
	}
	if _, ok := verifyTOTP(s, totpAt(s, now.Unix()/30-3), now, 0); ok {
		t.Fatal("kode di luar jendela diterima")
	}
}

func TestPasswordHash(t *testing.T) {
	h := hashPassword("rahasia-panjang-1")
	if !verifyPassword(h, "rahasia-panjang-1") || verifyPassword(h, "rahasia-panjang-2") || verifyPassword("rusak", "x") {
		t.Fatal("verifikasi password salah")
	}
	h1, h2 := hashPassword("a"), hashPassword("a")
	if h1 == h2 {
		t.Fatal("salt tidak acak")
	}
}

func TestSecretEncryptedAtRest(t *testing.T) {
	e := newEnv(t)
	raw, _ := os.ReadFile(e.dbp)
	if bytes.Contains(raw, []byte(b32.EncodeToString(e.secret))) || bytes.Contains(raw, e.secret) {
		t.Fatal("secret TOTP tersimpan polos")
	}
	if _, err := open(bytes.Repeat([]byte{9}, 32), mustUserEnc(e)); err == nil {
		t.Fatal("kunci salah bisa membuka secret")
	}
}

func mustUserEnc(e *env) []byte { return e.app.st.GetUser("admin").TOTPEnc }

// ---------- akses ----------

func TestEveryRouteRequiresAuth(t *testing.T) {
	e := newEnv(t)
	public := map[string]bool{"GET /healthz": true, "POST /api/auth/login": true, "GET /api/settings/public": true} // publik sadar: hanya nama panel dan tema bawaan (tampil di halaman masuk)
	for _, rt := range routes() {
		rt.Path = paramRE.ReplaceAllString(rt.Path, "x")
		key := rt.Method + " " + rt.Path
		if rt.Level == Public {
			if !public[key] {
				t.Errorf("rute publik tak terduga: %s (tambahkan secara sadar di tes ini)", key)
			}
			continue
		}
		if w := e.do(rt.Method, rt.Path, `{}`, "", "", nil); w.Code != 401 {
			t.Errorf("%s tanpa sesi: %d (harus 401)", key, w.Code)
		}
		if w := e.do(rt.Method, rt.Path, `{}`, "tokenpalsu", "x", nil); w.Code != 401 {
			t.Errorf("%s token palsu: %d", key, w.Code)
		}
	}
	// sesi tahap password TIDAK boleh membuka rute Full
	w := e.do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	c, cs := cookieOf(w), csrfOf(w)
	for _, rt := range routes() {
		rt.Path = paramRE.ReplaceAllString(rt.Path, "x")
		if rt.Level == Full {
			if w := e.do(rt.Method, rt.Path, `{}`, c, cs, nil); w.Code != 401 {
				t.Errorf("%s dengan sesi tahap-password: %d (harus 401)", rt.Method+" "+rt.Path, w.Code)
			}
		}
	}
}

func TestUnknownPath404(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/", "/api/x", "/../etc/passwd", "/.env"} {
		w := e.do("GET", p, "", "", "", nil)
		if w.Code >= 300 && w.Code < 400 { // ServeMux merapikan path lewat redirect; tujuannya tetap harus 404
			w = e.do("GET", w.Header().Get("Location"), "", "", "", nil)
		}
		if w.Code != 404 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
}

func TestLoginFlowAndHeaders(t *testing.T) {
	e := newEnv(t)
	c, cs, w := e.login(e.code(0))
	if w.Code != 200 || c == "" || cs == "" {
		t.Fatalf("login penuh gagal: %d", w.Code)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Frame-Options", "Cache-Control"} {
		if w.Header().Get(h) == "" {
			t.Errorf("header %s hilang", h)
		}
	}
	sc := w.Header().Get("Set-Cookie")
	for _, f := range []string{"HttpOnly", "Secure", "SameSite=Strict", cookieName} {
		if !strings.Contains(sc, f) {
			t.Errorf("cookie tanpa %s: %s", f, sc)
		}
	}
	if m := e.do("GET", "/api/me", "", c, "", nil); m.Code != 200 {
		t.Fatalf("/api/me: %d", m.Code)
	}
}

func TestWrongAndReusedTOTP(t *testing.T) {
	e := newEnv(t)
	if _, _, w := e.login("000000"); w.Code != 401 {
		t.Fatal("kode salah diterima")
	}
	good := e.code(0)
	c, cs, w := e.login(good)
	if w.Code != 200 {
		t.Fatal("kode benar ditolak")
	}
	e.do("POST", "/api/auth/logout", "{}", c, cs, nil)
	if _, _, w := e.login(good); w.Code != 401 {
		t.Fatal("REPLAY: kode TOTP yang sama diterima dua kali")
	}
}

func TestSessionIDRotatesAfterTOTP(t *testing.T) {
	e := newEnv(t)
	w1 := e.do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	pre := cookieOf(w1)
	w2 := e.do("POST", "/api/auth/totp", `{"code":"`+e.code(0)+`"}`, pre, csrfOf(w1), nil)
	if cookieOf(w2) == pre || cookieOf(w2) == "" {
		t.Fatal("id sesi tidak berganti")
	}
	if m := e.do("GET", "/api/me", "", pre, "", nil); m.Code != 401 {
		t.Fatal("sesi tahap-password lama masih hidup")
	}
}

func TestRecoveryCodeSingleUse(t *testing.T) {
	e := newEnv(t)
	rc := e.codes[0]
	if _, _, w := e.login(rc); w.Code != 200 {
		t.Fatalf("recovery code ditolak: %d", w.Code)
	}
	if _, _, w := e.login(rc); w.Code != 401 {
		t.Fatal("recovery code bisa dipakai dua kali")
	}
}

func TestGenericLoginError(t *testing.T) {
	e := newEnv(t)
	a := e.do("POST", "/api/auth/login", `{"name":"tidakada","password":"kata-sandi-panjang-123"}`, "", "", nil)
	b := e.do("POST", "/api/auth/login", `{"name":"admin","password":"salah-salah-salah"}`, "", "", nil)
	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Fatalf("galat membocorkan keberadaan akun: %d %q vs %d %q", a.Code, a.Body, b.Code, b.Body)
	}
}

func TestLockout(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < maxPerUser; i++ {
		if w := e.do("POST", "/api/auth/login", `{"name":"admin","password":"salah-salah-salah"}`, "", "", nil); w.Code != 401 {
			t.Fatalf("percobaan %d: %d", i, w.Code)
		}
	}
	if w := e.do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil); w.Code != 429 {
		t.Fatalf("password BENAR saat terkunci harus 429, dapat %d", w.Code)
	}
	e.now = e.now.Add(lockWindow + time.Second)
	if w := e.do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil); w.Code != 200 {
		t.Fatalf("setelah jendela lewat harus bisa login: %d", w.Code)
	}
}

func TestLockoutCannotBeBypassedWithForwardedHeader(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < maxPerIP; i++ {
		// nama berbeda tiap kali + XFF palsu dari klien non-loopback harus diabaikan
		e.do("POST", "/api/auth/login", `{"name":"u`+string(rune('a'+i))+`","password":"x"}`, "", "", map[string]string{"X-Forwarded-For": "10.9.9." + string(rune('1'+i))})
	}
	if w := e.do("POST", "/api/auth/login", `{"name":"lain","password":"x"}`, "", "", map[string]string{"X-Forwarded-For": "1.2.3.4"}); w.Code != 429 {
		t.Fatalf("lockout per-IP bisa dilewati lewat XFF: %d", w.Code)
	}
}

func TestClientIPTrustsOnlyLoopbackProxy(t *testing.T) {
	a := newApp(newEnv(t).app.st, testKey, Config{})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 100.1.2.3")
	if ip := a.clientIP(r); ip != "100.1.2.3" {
		t.Fatalf("harus nilai terakhir (dari nginx): %s", ip)
	}
	r.RemoteAddr = "192.168.0.9:1"
	if ip := a.clientIP(r); ip != "192.168.0.9" {
		t.Fatalf("XFF dari non-loopback harus diabaikan: %s", ip)
	}
}

// ---------- CSRF / origin / body ----------

func TestCSRFAndOrigin(t *testing.T) {
	e := newEnv(t)
	c, cs, _ := e.login(e.code(0))
	if w := e.do("POST", "/api/auth/logout", "{}", c, "", nil); w.Code != 403 {
		t.Errorf("tanpa token CSRF: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/logout", "{}", c, "salah", nil); w.Code != 403 {
		t.Errorf("token CSRF salah: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/logout", "{}", c, cs, map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Errorf("origin asing: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/logout", "{}", c, cs, map[string]string{"Sec-Fetch-Site": "cross-site"}); w.Code != 403 {
		t.Errorf("sec-fetch-site cross-site: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/login", `{"name":"admin","password":"x"}`, "", "", map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Errorf("login dari origin asing: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/logout", "{}", c, cs, nil); w.Code != 200 {
		t.Errorf("logout sah: %d", w.Code)
	}
}

func TestBodyLimitAndStrictJSON(t *testing.T) {
	e := newEnv(t)
	big := `{"name":"admin","password":"` + strings.Repeat("a", 9000) + `"}`
	if w := e.do("POST", "/api/auth/login", big, "", "", nil); w.Code != 413 {
		t.Errorf("body besar: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/login", `{"name":"admin","password":"x","admin":true}`, "", "", nil); w.Code != 400 {
		t.Errorf("field tak dikenal harus 400: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/login", `bukan json`, "", "", nil); w.Code != 400 {
		t.Errorf("bukan JSON: %d", w.Code)
	}
}

// ---------- sesi ----------

func TestSessionIdleAndAbsoluteExpiry(t *testing.T) {
	e := newEnv(t)
	c, _, _ := e.login(e.code(0))
	e.now = e.now.Add(idleTTL - time.Minute)
	if w := e.do("GET", "/api/me", "", c, "", nil); w.Code != 200 {
		t.Fatal("sesi aktif ditolak")
	}
	e.now = e.now.Add(idleTTL + time.Second)
	if w := e.do("GET", "/api/me", "", c, "", nil); w.Code != 401 {
		t.Fatal("sesi idle tidak kedaluwarsa")
	}
	c2, _, _ := e.login(e.code(0))
	for i := 0; i < 30; i++ { // tetap aktif tiap 25 menit, tapi lewati batas absolut 12 jam
		e.now = e.now.Add(25 * time.Minute)
		e.do("GET", "/api/me", "", c2, "", nil)
	}
	if w := e.do("GET", "/api/me", "", c2, "", nil); w.Code != 401 {
		t.Fatal("batas absolut 12 jam tidak ditegakkan")
	}
}

func TestPwdStageExpires(t *testing.T) {
	e := newEnv(t)
	w1 := e.do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	e.now = e.now.Add(pwdTTL + time.Second)
	if w := e.do("POST", "/api/auth/totp", `{"code":"`+e.code(0)+`"}`, cookieOf(w1), csrfOf(w1), nil); w.Code != 401 {
		t.Fatalf("tahap password harus kedaluwarsa: %d", w.Code)
	}
}

func TestStepUp(t *testing.T) {
	e := newEnv(t)
	c, cs, _ := e.login(e.code(0))
	ss := e.app.session(httptest.NewRequest("GET", "/", nil))
	_ = ss
	if m := e.do("GET", "/api/me", "", c, "", nil); !strings.Contains(m.Body.String(), `"stepup":false`) {
		t.Fatal("stepup harus false di awal")
	}
	if w := e.do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, c, cs, nil); w.Code != 200 {
		t.Fatalf("stepup: %d %s", w.Code, w.Body)
	}
	if m := e.do("GET", "/api/me", "", c, "", nil); !strings.Contains(m.Body.String(), `"stepup":true`) {
		t.Fatal("stepup tidak tercatat")
	}
	e.now = e.now.Add(stepupTTL + time.Second)
	if m := e.do("GET", "/api/me", "", c, "", nil); !strings.Contains(m.Body.String(), `"stepup":false`) {
		t.Fatal("stepup tidak kedaluwarsa")
	}
}

// ---------- audit ----------

func TestAuditChainDetectsTamper(t *testing.T) {
	e := newEnv(t)
	e.login("000000")
	e.login(e.code(0))
	n, err := e.app.st.VerifyAudit()
	if err != nil || n < 3 {
		t.Fatalf("rantai harus utuh: n=%d err=%v", n, err)
	}
	db, _ := sql.Open("sqlite", "file:"+e.dbp)
	db.Exec("UPDATE audit SET action='nothing' WHERE id=2")
	db.Close()
	e.app.st.db.Close()
	st2, _ := openStore(e.dbp)
	if _, err := st2.VerifyAudit(); err == nil {
		t.Fatal("perubahan baris audit tidak terdeteksi")
	}
}

func TestAuditHasNoSecrets(t *testing.T) {
	e := newEnv(t)
	e.do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-SALAH-123"}`, "", "", nil)
	c, cs, _ := e.login(e.code(0))
	_ = cs
	body := e.do("GET", "/api/audit", "", c, "", nil).Body.String()
	for _, s := range []string{"kata-sandi", e.code(0), e.codes[0], c} {
		if strings.Contains(body, s) {
			t.Fatalf("audit memuat rahasia: %q", s)
		}
	}
	if !strings.Contains(body, "login_fail") {
		t.Fatal("percobaan gagal tidak tercatat")
	}
}

// Penjaga atomik: dua permintaan bersamaan dengan kode yang sama, hanya satu boleh menang.
func TestUseTOTPStepIsAtomic(t *testing.T) {
	e := newEnv(t)
	u := e.app.st.GetUser("admin")
	if !e.app.st.UseTOTPStep(u.ID, 100) {
		t.Fatal("langkah pertama harus diterima")
	}
	if e.app.st.UseTOTPStep(u.ID, 100) || e.app.st.UseTOTPStep(u.ID, 99) {
		t.Fatal("langkah yang sama atau lebih lama harus ditolak")
	}
	if !e.app.st.UseTOTPStep(u.ID, 101) {
		t.Fatal("langkah lebih baru harus diterima")
	}
}

func jsonQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func decodeJSONString(s string, v any) error {
	d := json.NewDecoder(strings.NewReader(s))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
