package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func sign(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func (o *opsEnv) hook(site, body, sig, event, delivery, ip string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/hook/"+site, strings.NewReader(body))
	r.RemoteAddr = ip + ":4444"
	if sig != "" {
		r.Header.Set("X-Hub-Signature-256", sig)
	}
	if event != "" {
		r.Header.Set("X-GitHub-Event", event)
	}
	if delivery != "" {
		r.Header.Set("X-GitHub-Delivery", delivery)
	}
	w := httptest.NewRecorder()
	o.app.HookHandler().ServeHTTP(w, r)
	return w
}

func setupHook(t *testing.T, o *opsEnv) string {
	o.stepup()
	c, b := o.post("/api/sites/demo/hook", `{"enabled":true,"branch":"main","steps":["git_pull","composer_install"]}`)
	if c != 200 {
		t.Fatalf("setup: %d %s", c, b)
	}
	var out struct{ Secret string }
	json.Unmarshal([]byte(b), &out)
	if len(out.Secret) != 64 {
		t.Fatalf("secret harus 64 hex: %q", out.Secret)
	}
	return out.Secret
}

func TestWebhookManageAndSecretOnce(t *testing.T) {
	o := newOpsEnv(t, "")
	if c, _ := o.post("/api/sites/demo/hook", `{"enabled":true,"branch":"main","steps":["git_pull"]}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	sec := setupHook(t, o)
	if g := o.do("GET", "/api/sites/demo/hook", "", o.cookie, "", nil).Body.String(); strings.Contains(g, sec) || strings.Contains(g, "secret") || !strings.Contains(g, `"branch":"main"`) {
		t.Fatalf("GET tidak boleh memuat secret: %s", g)
	}
	var raw []byte
	o.app.st.db.QueryRow("SELECT secret_enc FROM hooks WHERE site='demo'").Scan(&raw)
	if strings.Contains(string(raw), sec) {
		t.Fatal("secret tersimpan polos di database")
	}
	// ubah tanpa rotate: tidak ada secret baru, secret lama tetap berlaku
	c, b := o.post("/api/sites/demo/hook", `{"enabled":true,"branch":"deploy","steps":["git_pull"]}`)
	if c != 200 || strings.Contains(b, "secret") {
		t.Fatalf("update: %d %s", c, b)
	}
	if w := o.hook("demo", `{"ref":"refs/heads/deploy","after":"abc1234"}`, sign(sec, `{"ref":"refs/heads/deploy","after":"abc1234"}`), "push", "d1", "1.2.3.4"); w.Code != 202 || !strings.Contains(w.Body.String(), "job") {
		t.Fatalf("secret lama harus tetap berlaku: %d %s", w.Code, w.Body.String())
	}
	// rotate: secret baru, lama mati
	c, b = o.post("/api/sites/demo/hook", `{"enabled":true,"branch":"deploy","steps":["git_pull"],"rotate":true}`)
	var out struct{ Secret string }
	json.Unmarshal([]byte(b), &out)
	if c != 200 || len(out.Secret) != 64 || out.Secret == sec {
		t.Fatalf("rotate: %d %s", c, b)
	}
	o.now = o.now.Add(time.Minute)
	body := `{"ref":"refs/heads/deploy"}`
	if w := o.hook("demo", body, sign(sec, body), "push", "d2", "1.2.3.4"); w.Code != 404 {
		t.Errorf("secret lama harus ditolak: %d", w.Code)
	}
	if au := o.do("GET", "/api/audit", "", o.cookie, "", nil).Body.String(); strings.Contains(au, out.Secret) || strings.Contains(au, sec) {
		t.Errorf("audit memuat secret")
	}
	for _, bad := range []string{
		`{"enabled":true,"branch":"","steps":["git_pull"]}`, `{"enabled":true,"branch":"-x","steps":["git_pull"]}`, `{"enabled":true,"branch":"a..b","steps":["git_pull"]}`, `{"enabled":true,"branch":"a b","steps":["git_pull"]}`,
		`{"enabled":true,"branch":"a;id","steps":["git_pull"]}`, `{"enabled":true,"branch":"main/","steps":["git_pull"]}`, `{"enabled":true,"branch":"a//b","steps":["git_pull"]}`, `{"enabled":true,"branch":"main","steps":["git_pull","composer_install","composer_install_nodev","composer_update","dump_autoload","node_install","node_build","artisan_migrate","artisan_optimize"]}`, `{"enabled":true,"branch":"x.lock","steps":["git_pull"]}`,
		`{"enabled":true,"branch":"main","steps":[]}`, `{"enabled":true,"branch":"main","steps":["rm -rf"]}`, `{"enabled":true,"branch":"main","steps":["git_pull","git_pull"]}`, `{"enabled":true,"branch":"main","steps":["git_pull"],"x":1}`,
	} {
		if c, r := o.post("/api/sites/demo/hook", bad); c != 400 {
			t.Errorf("%s -> %d %s", bad, c, r)
		}
	}
	if c, _ := o.post("/api/sites/panel/hook", `{"enabled":true,"branch":"main","steps":["git_pull"]}`); c != 400 && c != 404 {
		t.Errorf("situs inti: %d", c)
	}
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	if w := o.do("GET", "/api/sites/demo/hook", "", o.cookie, "", nil); w.Code != 403 {
		t.Errorf("viewer: %d", w.Code)
	}
}

func TestWebhookReceiverSecurity(t *testing.T) {
	o := newOpsEnv(t, "")
	o.app.longTimeout = 20 * time.Second
	sec := setupHook(t, o)
	push := `{"ref":"refs/heads/main","after":"0123456789abcdef"}`
	good := sign(sec, push)
	before := o.calls()

	// penolakan: tidak ada pemanggilan lab, jawaban seragam 404
	cases := []struct{ name, site, body, sig, ev, ip string }{
		{"tanpa tanda tangan", "demo", push, "", "push", "9.9.9.1"},
		{"tanda tangan salah", "demo", push, sign("salah", push), "push", "9.9.9.2"},
		{"tanda tangan untuk body lain", "demo", push, sign(sec, push+" "), "push", "9.9.9.3"},
		{"tanpa awalan sha256=", "demo", push, strings.TrimPrefix(good, "sha256="), "push", "9.9.9.4"},
		{"sha1", "demo", push, "sha1=" + strings.TrimPrefix(good, "sha256="), "push", "9.9.9.5"},
		{"situs tidak ada", "nope", push, good, "push", "9.9.9.6"},
		{"situs inti", "panel", push, good, "push", "9.9.9.7"},
		{"nama aneh", "a;b", push, good, "push", "9.9.9.8"},
	}
	for _, c := range cases {
		w := o.hook(c.site, c.body, c.sig, c.ev, "x"+c.name, c.ip)
		if w.Code != 404 && !(c.site == "a;b" && w.Code == 404) {
			t.Errorf("%s -> %d %s", c.name, w.Code, w.Body.String())
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh permintaan tak sah: %s", o.calls())
	}
	// GET / path lain / metode lain tidak ada
	for _, p := range [][2]string{{"GET", "/hook/demo"}, {"GET", "/"}, {"POST", "/api/me"}, {"GET", "/api/sites"}, {"POST", "/hook/"}} {
		r := httptest.NewRequest(p[0], p[1], nil)
		w := httptest.NewRecorder()
		o.app.HookHandler().ServeHTTP(w, r)
		if w.Code != 404 && w.Code != 405 {
			t.Errorf("%s %s -> %d (listener publik hanya boleh melayani POST /hook/{site})", p[0], p[1], w.Code)
		}
	}
	if h := o.hook("demo", push, good, "issues", "hdr", "1.1.1.1").Header().Get("Cache-Control"); h != "no-store" {
		t.Errorf("Cache-Control harus no-store: %q", h)
	}
	// ping
	if w := o.hook("demo", "{}", sign(sec, "{}"), "ping", "p1", "1.1.1.1"); w.Code != 200 || o.calls() != before {
		t.Errorf("ping: %d", w.Code)
	}
	// event lain dan branch lain diabaikan
	if w := o.hook("demo", push, good, "issues", "e1", "1.1.1.1"); w.Code != 202 || o.calls() != before {
		t.Errorf("event lain: %d", w.Code)
	}
	other := `{"ref":"refs/heads/feature","after":"abc"}`
	if w := o.hook("demo", other, sign(sec, other), "push", "e2", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), "branch") || o.calls() != before {
		t.Errorf("branch lain: %d %s", w.Code, w.Body.String())
	}
	tag := `{"ref":"refs/tags/main","after":"abc"}`
	if w := o.hook("demo", tag, sign(sec, tag), "push", "e3", "1.1.1.1"); w.Code != 202 || o.calls() != before {
		t.Errorf("tag bernama sama dengan branch: %d", w.Code)
	}
	del := `{"ref":"refs/heads/main","deleted":true}`
	if w := o.hook("demo", del, sign(sec, del), "push", "e4", "1.1.1.1"); w.Code != 202 || o.calls() != before {
		t.Errorf("hapus branch: %d", w.Code)
	}
	if w := o.hook("demo", "bukan json", sign(sec, "bukan json"), "push", "e5", "1.1.1.1"); w.Code != 400 {
		t.Errorf("payload rusak: %d", w.Code)
	}
	big := strings.Repeat("x", hookMaxBody+10)
	if w := o.hook("demo", big, sign(sec, big), "push", "e6", "1.1.1.1"); w.Code != 413 {
		t.Errorf("terlalu besar: %d", w.Code)
	}
	// sah: menjalankan deploy dengan langkah yang dikonfigurasi, bukan dari payload
	w := o.hook("demo", push, good, "push", "ok1", "1.1.1.1")
	if w.Code != 202 {
		t.Fatalf("sah: %d %s", w.Code, w.Body.String())
	}
	var j struct{ Job int64 }
	json.Unmarshal(w.Body.Bytes(), &j)
	o.waitJob(j.Job)
	if !strings.Contains(o.calls(), "CALL:[deploy][run][demo][git_pull,composer_install]") {
		t.Fatalf("argv: %s", o.calls())
	}
	// replay (delivery sama) dan terlalu cepat
	if w := o.hook("demo", push, good, "push", "ok1", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), "ganda") {
		t.Errorf("replay: %d %s", w.Code, w.Body.String())
	}
	if w := o.hook("demo", push, good, "push", "ok2", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), "cepat") {
		t.Errorf("debounce: %d %s", w.Code, w.Body.String())
	}
	o.now = o.now.Add(time.Minute)
	if w := o.hook("demo", push, good, "push", "ok3", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), `"job"`) {
		t.Errorf("setelah jeda harus jalan: %d %s", w.Code, w.Body.String())
	}
	// audit memuat aksi webhook, tanpa secret
	time.Sleep(300 * time.Millisecond)
	if au := o.do("GET", "/api/audit", "", o.cookie, "", nil).Body.String(); !strings.Contains(au, "webhook_deploy") || !strings.Contains(au, "hook_reject") || strings.Contains(au, sec) {
		t.Errorf("audit: %s", au)
	}
	// dinonaktifkan: seragam 404 walau tanda tangan benar
	o.stepup()
	o.post("/api/sites/demo/hook", `{"enabled":false,"branch":"main","steps":["git_pull"]}`)
	o.now = o.now.Add(time.Minute)
	if w := o.hook("demo", push, good, "push", "ok4", "1.1.1.9"); w.Code != 404 {
		t.Errorf("nonaktif: %d", w.Code)
	}
	if c, _ := o.post("/api/sites/demo/hook/remove", `{}`); c != 200 {
		t.Errorf("hapus: %d", c)
	}
	if w := o.hook("demo", push, good, "push", "ok5", "1.1.1.9"); w.Code != 404 {
		t.Errorf("terhapus: %d", w.Code)
	}
}

func TestWebhookBruteForceLockout(t *testing.T) {
	o := newOpsEnv(t, "")
	sec := setupHook(t, o)
	push := `{"ref":"refs/heads/main"}`
	locked := false
	for i := 0; i < 40; i++ {
		if w := o.hook("demo", push, sign("tebakan", push), "push", "", "7.7.7.7"); w.Code == 429 {
			locked = true
			break
		}
	}
	if !locked {
		t.Fatal("percobaan tanda tangan salah berulang harus dikunci (429)")
	}
	if w := o.hook("demo", push, sign(sec, push), "push", "z1", "7.7.7.7"); w.Code != 429 {
		t.Errorf("IP terkunci harus tetap ditolak walau tanda tangan benar: %d", w.Code)
	}
	if w := o.hook("demo", push, sign(sec, push), "push", "z2", "8.8.8.8"); w.Code != 202 {
		t.Errorf("IP lain tidak boleh terdampak: %d", w.Code)
	}
}

var _ = http.StatusOK
