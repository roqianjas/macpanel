package server_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/deploy"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestWebhookReceiverSecurity(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	o.App.LongTimeout = 20 * time.Second
	sec := testenv.SetupHook(t, o)
	push := `{"ref":"refs/heads/main","after":"0123456789abcdef"}`
	good := testenv.Sign(sec, push)
	before := o.Calls()

	// penolakan: tidak ada pemanggilan lab, jawaban seragam 404
	cases := []struct{ name, site, body, sig, ev, ip string }{
		{"tanpa tanda tangan", "demo", push, "", "push", "9.9.9.1"},
		{"tanda tangan salah", "demo", push, testenv.Sign("salah", push), "push", "9.9.9.2"},
		{"tanda tangan untuk body lain", "demo", push, testenv.Sign(sec, push+" "), "push", "9.9.9.3"},
		{"tanpa awalan sha256=", "demo", push, strings.TrimPrefix(good, "sha256="), "push", "9.9.9.4"},
		{"sha1", "demo", push, "sha1=" + strings.TrimPrefix(good, "sha256="), "push", "9.9.9.5"},
		{"situs tidak ada", "nope", push, good, "push", "9.9.9.6"},
		{"situs inti", "panel", push, good, "push", "9.9.9.7"},
		{"nama aneh", "a;b", push, good, "push", "9.9.9.8"},
	}
	for _, c := range cases {
		w := o.Hook(c.site, c.body, c.sig, c.ev, "x"+c.name, c.ip)
		if w.Code != 404 && !(c.site == "a;b" && w.Code == 404) {
			t.Errorf("%s -> %d %s", c.name, w.Code, w.Body.String())
		}
	}
	if o.Calls() != before {
		t.Fatalf("lab terpanggil oleh permintaan tak sah: %s", o.Calls())
	}
	// GET / path lain / metode lain tidak ada
	for _, p := range [][2]string{{"GET", "/hook/demo"}, {"GET", "/"}, {"POST", "/api/me"}, {"GET", "/api/sites"}, {"POST", "/hook/"}} {
		r := httptest.NewRequest(p[0], p[1], nil)
		w := httptest.NewRecorder()
		deploy.HookHandler(o.App).ServeHTTP(w, r)
		if w.Code != 404 && w.Code != 405 {
			t.Errorf("%s %s -> %d (listener publik hanya boleh melayani POST /hook/{site})", p[0], p[1], w.Code)
		}
	}
	if h := o.Hook("demo", push, good, "issues", "hdr", "1.1.1.1").Header().Get("Cache-Control"); h != "no-store" {
		t.Errorf("Cache-Control harus no-store: %q", h)
	}
	// ping
	if w := o.Hook("demo", "{}", testenv.Sign(sec, "{}"), "ping", "p1", "1.1.1.1"); w.Code != 200 || o.Calls() != before {
		t.Errorf("ping: %d", w.Code)
	}
	// event lain dan branch lain diabaikan
	if w := o.Hook("demo", push, good, "issues", "e1", "1.1.1.1"); w.Code != 202 || o.Calls() != before {
		t.Errorf("event lain: %d", w.Code)
	}
	other := `{"ref":"refs/heads/feature","after":"abc"}`
	if w := o.Hook("demo", other, testenv.Sign(sec, other), "push", "e2", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), "branch") || o.Calls() != before {
		t.Errorf("branch lain: %d %s", w.Code, w.Body.String())
	}
	tag := `{"ref":"refs/tags/main","after":"abc"}`
	if w := o.Hook("demo", tag, testenv.Sign(sec, tag), "push", "e3", "1.1.1.1"); w.Code != 202 || o.Calls() != before {
		t.Errorf("tag bernama sama dengan branch: %d", w.Code)
	}
	del := `{"ref":"refs/heads/main","deleted":true}`
	if w := o.Hook("demo", del, testenv.Sign(sec, del), "push", "e4", "1.1.1.1"); w.Code != 202 || o.Calls() != before {
		t.Errorf("hapus branch: %d", w.Code)
	}
	if w := o.Hook("demo", "bukan json", testenv.Sign(sec, "bukan json"), "push", "e5", "1.1.1.1"); w.Code != 400 {
		t.Errorf("payload rusak: %d", w.Code)
	}
	big := strings.Repeat("x", deploy.HookMaxBody+10)
	if w := o.Hook("demo", big, testenv.Sign(sec, big), "push", "e6", "1.1.1.1"); w.Code != 413 {
		t.Errorf("terlalu besar: %d", w.Code)
	}
	// sah: menjalankan deploy dengan langkah yang dikonfigurasi, bukan dari payload
	w := o.Hook("demo", push, good, "push", "ok1", "1.1.1.1")
	if w.Code != 202 {
		t.Fatalf("sah: %d %s", w.Code, w.Body.String())
	}
	var j struct{ Job int64 }
	json.Unmarshal(w.Body.Bytes(), &j)
	o.WaitJob(j.Job)
	if !strings.Contains(o.Calls(), "CALL:[deploy][run][demo][git_pull,composer_install]") {
		t.Fatalf("argv: %s", o.Calls())
	}
	// replay (delivery sama) dan terlalu cepat
	if w := o.Hook("demo", push, good, "push", "ok1", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), "ganda") {
		t.Errorf("replay: %d %s", w.Code, w.Body.String())
	}
	if w := o.Hook("demo", push, good, "push", "ok2", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), "cepat") {
		t.Errorf("debounce: %d %s", w.Code, w.Body.String())
	}
	o.Now = o.Now.Add(time.Minute)
	if w := o.Hook("demo", push, good, "push", "ok3", "1.1.1.1"); w.Code != 202 || !strings.Contains(w.Body.String(), `"job"`) {
		t.Errorf("setelah jeda harus jalan: %d %s", w.Code, w.Body.String())
	}
	// audit memuat aksi webhook, tanpa secret
	time.Sleep(300 * time.Millisecond)
	if au := o.Do("GET", "/api/audit", "", o.Cookie, "", nil).Body.String(); !strings.Contains(au, "webhook_deploy") || !strings.Contains(au, "hook_reject") || strings.Contains(au, sec) {
		t.Errorf("audit: %s", au)
	}
	// dinonaktifkan: seragam 404 walau tanda tangan benar
	o.Stepup()
	o.Post("/api/sites/demo/hook", `{"enabled":false,"branch":"main","steps":["git_pull"]}`)
	o.Now = o.Now.Add(time.Minute)
	if w := o.Hook("demo", push, good, "push", "ok4", "1.1.1.9"); w.Code != 404 {
		t.Errorf("nonaktif: %d", w.Code)
	}
	if c, _ := o.Post("/api/sites/demo/hook/remove", `{}`); c != 200 {
		t.Errorf("hapus: %d", c)
	}
	if w := o.Hook("demo", push, good, "push", "ok5", "1.1.1.9"); w.Code != 404 {
		t.Errorf("terhapus: %d", w.Code)
	}
}
