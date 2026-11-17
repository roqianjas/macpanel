package main

import (
	"strings"
	"testing"
	"time"
)

func TestStepUpUsesPasswordNotTOTP(t *testing.T) {
	e := newEnv(t)
	ck, cs, _ := e.trustLogin(false)
	me := func() string { return e.do("GET", "/api/me", "", ck, "", nil).Body.String() }
	if !strings.Contains(me(), `"stepup":false`) {
		t.Fatalf("awal: %s", me())
	}
	// kode TOTP tidak lagi diterima di step-up
	if w := e.do("POST", "/api/auth/stepup", `{"code":"`+e.code(1)+`"}`, ck, cs, nil); w.Code == 200 {
		t.Fatal("TOTP tidak boleh lagi membuka step-up")
	}
	if w := e.do("POST", "/api/auth/stepup", `{"password":"`+e.code(1)+`"}`, ck, cs, nil); w.Code != 400 {
		t.Fatalf("kode TOTP sebagai password harus ditolak: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/stepup", `{"password":"salah-salah-salah"}`, ck, cs, nil); w.Code != 400 || !strings.Contains(w.Body.String(), "kata sandi salah") {
		t.Fatalf("sandi salah harus 400 (bukan 401 agar sesi tidak keluar): %d %s", w.Code, w.Body)
	}
	if w := e.do("GET", "/api/me", "", ck, "", nil); w.Code != 200 {
		t.Fatalf("sesi harus tetap hidup setelah sandi salah: %d", w.Code)
	}
	if w := e.do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck, cs, nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid_seconds":900`) {
		t.Fatalf("sandi benar: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(me(), `"stepup":true`) {
		t.Fatal("step-up harus aktif")
	}
	e.now = e.now.Add(14 * time.Minute)
	if !strings.Contains(me(), `"stepup":true`) {
		t.Fatal("harus masih aktif di menit ke-14")
	}
	e.now = e.now.Add(2 * time.Minute)
	if !strings.Contains(me(), `"stepup":false`) {
		t.Fatal("harus habis setelah 15 menit")
	}
}

func TestStepUpPasswordBruteForceLocksOut(t *testing.T) {
	e := newEnv(t)
	ck, cs, _ := e.trustLogin(false)
	locked := false
	for i := 0; i < 30; i++ {
		w := e.do("POST", "/api/auth/stepup", `{"password":"tebakan-`+string(rune('a'+i%26))+`-panjang"}`, ck, cs, nil)
		if w.Code == 429 {
			locked = true
			break
		}
	}
	if !locked {
		t.Fatal("tebakan sandi di step-up harus memicu lockout")
	}
	if w := e.do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, ck, cs, nil); w.Code != 429 {
		t.Fatalf("saat terkunci, sandi benar pun harus ditolak: %d", w.Code)
	}
}

func TestStepUpRequiredMessageAndDisabledUser(t *testing.T) {
	o := newOpsEnv(t, "echo ok")
	if c, b := o.post("/api/redis/get", `{"key":"k"}`); c != 403 || !strings.Contains(b, "kata sandi ulang") {
		t.Fatalf("pesan step-up: %d %s", c, b)
	}
}

func TestNineRouterIsOptionalAppWithSafeGuards(t *testing.T) {
	o := newOpsEnv(t, "echo ok")
	c, b := o.post("/api/apps/9router/install", `{}`)
	if c != 202 {
		t.Fatalf("install 9router: %d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[app][install][9router]") {
		t.Fatalf("argv: %s", o.calls())
	}
	// hapus: konfirmasi nama + step-up
	if c, _ := o.post("/api/apps/9router/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi: %d", c)
	}
	o.now = o.now.Add(stepupTTL + time.Second)
	if c, _ := o.post("/api/apps/9router/remove", `{"confirm":"9router"}`); c != 403 {
		t.Fatalf("remove tanpa step-up: %d", c)
	}
	// aplikasi lain yang tak ada di katalog tetap ditolak
	if c, _ := o.post("/api/apps/9router-evil/install", `{}`); c != 400 && c != 404 {
		t.Fatalf("id asing: %d", c)
	}
}
