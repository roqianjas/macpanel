package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func trustOf(w *httptest.ResponseRecorder) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == trustCookie {
			return c.Value
		}
	}
	return ""
}

const loginBody = `{"name":"roki","password":"kata-sandi-panjang-123"}`

func (e *env) trustLogin(trust bool) (cookie, csrf, tok string) {
	w1 := e.do("POST", "/api/auth/login", loginBody, "", "", nil)
	c1, cs1 := cookieOf(w1), csrfOf(w1)
	body := `{"code":"` + e.code(0) + `","trust":` + map[bool]string{true: "true", false: "false"}[trust] + `}`
	w2 := e.do("POST", "/api/auth/totp", body, c1, cs1, map[string]string{"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/130.0"})
	if w2.Code != 200 {
		e.t.Fatalf("totp: %d %s", w2.Code, w2.Body)
	}
	return cookieOf(w2), csrfOf(w2), trustOf(w2)
}

func TestTrustedDeviceFlow(t *testing.T) {
	e := newEnv(t)
	// tanpa centang: tidak ada cookie tepercaya
	if _, _, tok := e.trustLogin(false); tok != "" {
		t.Fatal("cookie tepercaya keluar tanpa diminta")
	}
	e.now = e.now.Add(time.Minute)
	ck, cs, tok := e.trustLogin(true)
	if tok == "" {
		t.Fatal("cookie tepercaya tidak keluar")
	}
	// step-up di sesi ini, supaya terbukti tidak 'terbawa' ke sesi tepercaya
	if w := e.do("POST", "/api/auth/stepup", `{"code":"`+e.code(1)+`"}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("stepup: %d %s", w.Code, w.Body)
	}
	// login berikutnya: kata sandi saja
	w := e.do("POST", "/api/auth/login", loginBody, "", "", map[string]string{"Cookie": trustCookie + "=" + tok})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"trusted":true`) {
		t.Fatalf("login tepercaya: %d %s", w.Code, w.Body)
	}
	c2, cs2, tok2 := cookieOf(w), csrfOf(w), trustOf(w)
	if tok2 == "" || tok2 == tok {
		t.Fatal("token tepercaya tidak dirotasi")
	}
	if m := e.do("GET", "/api/me", "", c2, "", nil); m.Code != 200 || !strings.Contains(m.Body.String(), `"stepup":false`) {
		t.Fatalf("sesi tepercaya tidak boleh punya step-up: %d %s", m.Code, m.Body)
	}
	// token lama sudah tidak berlaku; tanpa cookie wajib 2FA
	if w := e.do("POST", "/api/auth/login", loginBody, "", "", map[string]string{"Cookie": trustCookie + "=" + tok}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("token lama masih diterima setelah rotasi")
	}
	if w := e.do("POST", "/api/auth/login", loginBody, "", "", nil); !strings.Contains(w.Body.String(), "need_totp") {
		t.Fatalf("tanpa cookie harus minta 2FA: %s", w.Body)
	}
	// kata sandi salah tetap ditolak walau cookie sah
	if w := e.do("POST", "/api/auth/login", `{"name":"roki","password":"salah-salah-salah"}`, "", "", map[string]string{"Cookie": trustCookie + "=" + tok2}); w.Code != 401 {
		t.Fatalf("sandi salah + cookie sah: %d", w.Code)
	}
	// daftar + label
	d := e.do("GET", "/api/me/devices", "", c2, "", map[string]string{"Cookie": cookieName + "=" + c2 + "; " + trustCookie + "=" + tok2})
	if d.Code != 200 || !strings.Contains(d.Body.String(), "Firefox di Linux") || !strings.Contains(d.Body.String(), `"current":true`) {
		t.Fatalf("daftar perangkat: %d %s", d.Code, d.Body)
	}
	// kedaluwarsa 30 hari
	e.now = e.now.Add(trustTTL + time.Hour)
	if w := e.do("POST", "/api/auth/login", loginBody, "", "", map[string]string{"Cookie": trustCookie + "=" + tok2}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("perangkat kedaluwarsa masih diterima")
	}
	_ = cs2
}

func TestTrustedRevokeAndPasswordChange(t *testing.T) {
	e := newEnv(t)
	ck, cs, tok := e.trustLogin(true)
	both := map[string]string{"Cookie": cookieName + "=" + ck + "; " + trustCookie + "=" + tok}
	d := e.do("GET", "/api/me/devices", "", ck, "", both)
	var id string
	if i := strings.Index(d.Body.String(), `"id":`); i >= 0 {
		id = strings.SplitN(d.Body.String()[i+5:], ",", 2)[0]
	}
	if w := e.do("POST", "/api/me/devices/revoke", `{"id":99999}`, ck, cs, nil); w.Code != 404 {
		t.Fatalf("revoke id asing: %d", w.Code)
	}
	if w := e.do("POST", "/api/me/devices/revoke", `{"id":`+id+`}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("revoke: %d %s", w.Code, w.Body)
	}
	if w := e.do("POST", "/api/auth/login", loginBody, "", "", map[string]string{"Cookie": trustCookie + "=" + tok}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("perangkat yang dicabut masih diterima")
	}
	// ganti sandi mencabut semua
	e.now = e.now.Add(time.Minute)
	ck, cs, tok = e.trustLogin(true)
	if w := e.do("POST", "/api/auth/stepup", `{"code":"`+e.code(1)+`"}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("stepup: %d %s", w.Code, w.Body)
	}
	if w := e.do("POST", "/api/me/password", `{"current":"kata-sandi-panjang-123","new":"sandi-baru-yang-panjang-456"}`, ck, cs, nil); w.Code != 200 {
		t.Fatalf("ganti sandi: %d %s", w.Code, w.Body)
	}
	if w := e.do("POST", "/api/auth/login", `{"name":"roki","password":"sandi-baru-yang-panjang-456"}`, "", "", map[string]string{"Cookie": trustCookie + "=" + tok}); strings.Contains(w.Body.String(), `"trusted"`) {
		t.Fatal("perangkat tepercaya bertahan setelah ganti sandi")
	}
}

func TestTrustedCapAndIsolation(t *testing.T) {
	e := newEnv(t)
	var first string
	for i := 0; i < maxTrusted+3; i++ {
		e.now = e.now.Add(time.Minute)
		_, _, tok := e.trustLogin(true)
		if i == 0 {
			first = tok
		}
	}
	u := e.app.st.GetUser("roki")
	if n := len(e.app.st.ListTrusted(u.ID, "", e.now)); n != maxTrusted {
		t.Fatalf("batas perangkat: %d", n)
	}
	if _, ok := e.app.st.UseTrusted(first, u.ID, e.now); ok {
		t.Fatal("perangkat tertua seharusnya sudah dibuang")
	}
	// token milik user lain tidak berlaku
	if _, ok := e.app.st.UseTrusted("x", u.ID+1, e.now); ok {
		t.Fatal("token tak dikenal diterima")
	}
	if l := deviceLabel("curl/8"); l != "Browser di perangkat tidak dikenal" {
		t.Fatalf("label: %s", l)
	}
}

func TestTrustedCookieAttrsAndOtherUser(t *testing.T) {
	e := newEnv(t)
	w1 := e.do("POST", "/api/auth/login", loginBody, "", "", nil)
	w2 := e.do("POST", "/api/auth/totp", `{"code":"`+e.code(0)+`","trust":true}`, cookieOf(w1), csrfOf(w1), nil)
	var found bool
	for _, c := range w2.Result().Cookies() {
		if c.Name == trustCookie {
			found = true
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != int(trustTTL.Seconds()) {
				t.Fatalf("atribut cookie tepercaya lemah: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("cookie tepercaya tidak ada")
	}
	tok := trustOf(w2)
	// user lain tidak boleh memakai perangkat milik roki
	enc, _ := seal(testKey, newTOTPSecret())
	if err := e.app.st.CreateUser("ani", hashPassword("sandi-ani-panjang-789"), enc, nil); err != nil {
		t.Fatal(err)
	}
	w := e.do("POST", "/api/auth/login", `{"name":"ani","password":"sandi-ani-panjang-789"}`, "", "", map[string]string{"Cookie": trustCookie + "=" + tok})
	if strings.Contains(w.Body.String(), `"trusted"`) || !strings.Contains(w.Body.String(), "need_totp") {
		t.Fatalf("cookie roki dipakai user lain: %s", w.Body)
	}
	// dan tidak bisa mencabut perangkat milik roki
	roki, ani := e.app.st.GetUser("roki"), e.app.st.GetUser("ani")
	rows := e.app.st.ListTrusted(roki.ID, "", e.now)
	if len(rows) != 1 || e.app.st.RevokeTrusted(ani.ID, rows[0].ID) {
		t.Fatal("user lain bisa mencabut perangkat orang lain")
	}
}

func TestRenameUser(t *testing.T) {
	e := newEnv(t)
	_, _, tok := e.trustLogin(true)
	enc, _ := seal(testKey, newTOTPSecret())
	e.app.st.CreateUser("ani", hashPassword("sandi-ani-panjang-789"), enc, nil)
	if e.app.st.RenameUser("roki", "ani") || e.app.st.RenameUser("tidakada", "baru") {
		t.Fatal("rename ke nama terpakai / user tak ada harus gagal")
	}
	if !e.app.st.RenameUser("roki", "rokianjas") {
		t.Fatal("rename gagal")
	}
	if e.app.st.GetUser("roki") != nil || e.app.st.GetUser("rokianjas") == nil {
		t.Fatal("nama tidak berubah")
	}
	e.now = e.now.Add(time.Minute)
	w := e.do("POST", "/api/auth/login", `{"name":"rokianjas","password":"kata-sandi-panjang-123"}`, "", "", map[string]string{"Cookie": trustCookie + "=" + tok})
	if !strings.Contains(w.Body.String(), `"trusted":true`) {
		t.Fatalf("perangkat tepercaya harus tetap berlaku: %s", w.Body)
	}
	if w := e.do("POST", "/api/auth/login", loginBody, "", "", nil); w.Code != 401 {
		t.Fatalf("nama lama harus ditolak: %d", w.Code)
	}
}
