package server_test

import (
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/testenv"
)

func waitCall(t *testing.T, o interface{ Calls() string }, want string) bool {
	t.Helper()
	for i := 0; i < 40; i++ {
		if strings.Contains(o.Calls(), want) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func TestCleanReadEndpoints(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	if code, body := o.As(o.Cookie, o.Csrf, "GET", "/api/clean", ""); code != 200 || !strings.Contains(body, `"go-build"`) {
		t.Fatalf("GET /api/clean: %d %s", code, body)
	}
	if code, body := o.As(o.Cookie, o.Csrf, "GET", "/api/mem", ""); code != 200 || !strings.Contains(body, "total_mb") {
		t.Fatalf("GET /api/mem: %d %s", code, body)
	}
}

func TestCleanPreviewDoesNotApply(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	if code, body := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/preview", `{"ids":["go-build"]}`); code != 200 {
		t.Fatalf("preview: %d %s", code, body)
	}
	c := o.Calls()
	if !strings.Contains(c, "[clean][run][go-build]") || strings.Contains(c, "[--apply]") {
		t.Fatalf("pratinjau tidak boleh --apply: %q", c)
	}
}

func TestCleanRunBuildsFixedArgs(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	o.Stepup()
	code, body := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", `{"ids":["go-build","npm-cache"]}`)
	if code != 202 {
		t.Fatalf("run: %d %s", code, body)
	}
	if !waitCall(t, o, "[clean][run][go-build][npm-cache][--apply]") {
		t.Fatalf("argumen salah: %q", o.Calls())
	}
	if strings.Contains(o.Calls(), "[--confirm-careful]") {
		t.Fatal("tingkat Aman tidak boleh membawa --confirm-careful")
	}
}

func TestCleanCarefulRules(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	bad := []string{
		`{"ids":["bk:migrasi-x"]}`,
		`{"ids":["bk:migrasi-x"],"typed":"salah"}`,
		`{"ids":["bk:migrasi-x"],"typed":"bk:migrasi-x"}`,
		`{"ids":["bk:migrasi-x","go-build"],"typed":"migrasi-x"}`,
		`{"ids":["bk:migrasi-x","bk:unduh-mac"],"typed":"migrasi-x"}`,
	}
	for _, b := range bad {
		if code, _ := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", b); code != 400 {
			t.Errorf("harus 400: %s -> %d", b, code)
		}
	}
	if strings.Contains(o.Calls(), "[clean][run]") {
		t.Fatalf("tidak boleh ada panggilan: %q", o.Calls())
	}
	o.Stepup()
	if code, body := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", `{"ids":["bk:migrasi-x"],"typed":"migrasi-x"}`); code != 202 {
		t.Fatalf("careful valid: %d %s", code, body)
	}
	if !waitCall(t, o, "[clean][run][bk:migrasi-x][--confirm-careful][--apply]") {
		t.Fatalf("argumen salah: %q", o.Calls())
	}
}

func TestCleanRejectsBadIDs(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	for _, id := range []string{"--apply", "-x", "../etc", "a/b", "/etc/passwd", "a b", "", "x;rm", "bk:../a", "$(id)", strings.Repeat("a", 91), "bk:", "a..b"} {
		for _, p := range []string{"/api/clean/run", "/api/clean/preview"} {
			body := `{"ids":[` + jsonQ(id) + `],"typed":""}`
			if code, _ := o.As(o.Cookie, o.Csrf, "POST", p, body); code != 400 {
				t.Errorf("%s id %q -> %d, harus 400", p, id, code)
			}
		}
	}
	if code, _ := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", `{"ids":[]}`); code != 400 {
		t.Error("daftar kosong harus 400")
	}
	if code, _ := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", `{"ids":["a","a"]}`); code != 400 {
		t.Error("id ganda harus 400")
	}
	if strings.Contains(o.Calls(), "[clean][run]") {
		t.Fatalf("masukan buruk sampai ke lab: %q", o.Calls())
	}
}

func jsonQ(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}

func TestCleanViewerAndTokenBlocked(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	for _, p := range []string{"/api/clean/scan", "/api/clean/preview", "/api/clean/run"} {
		if code, _ := o.As(o.Cookie, o.Csrf, "POST", p, `{"ids":["go-build"]}`); code != 403 {
			t.Errorf("viewer %s -> %d, harus 403", p, code)
		}
	}
	if strings.Contains(o.Calls(), "[clean][run]") || strings.Contains(o.Calls(), "[clean][scan]") {
		t.Fatalf("viewer memicu lab: %q", o.Calls())
	}
}

func TestCleanRunNeedsStepUp(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	for _, b := range []string{`{"ids":["go-build"]}`, `{"ids":["bk:migrasi-x"],"typed":"migrasi-x"}`} {
		if code, _ := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", b); code != 403 {
			t.Errorf("tanpa step-up harus 403: %s -> %d", b, code)
		}
	}
	time.Sleep(150 * time.Millisecond)
	if strings.Contains(o.Calls(), "[--apply]") {
		t.Fatalf("lab dipanggil tanpa step-up: %q", o.Calls())
	}
}

func TestCleanExtraCarefulPrefixNeedsTyping(t *testing.T) {
	o := testenv.NewOpsEnv(t, testenv.FakeSys)
	o.Stepup()
	if code, _ := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", `{"ids":["cx:hermes-backups"],"typed":"salah"}`); code != 400 {
		t.Fatalf("harus 400, dapat %d", code)
	}
	if code, _ := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", `{"ids":["cx:hermes-backups","go-build"],"typed":"hermes-backups"}`); code != 400 {
		t.Fatal("careful tidak boleh digabung")
	}
	if code, _ := o.As(o.Cookie, o.Csrf, "POST", "/api/clean/run", `{"ids":["cx:hermes-backups"],"typed":"hermes-backups"}`); code != 202 {
		t.Fatal("valid harus 202")
	}
	if !waitCall(t, o, "[cx:hermes-backups][--confirm-careful][--apply]") {
		t.Fatalf("argumen salah: %q", o.Calls())
	}
}
