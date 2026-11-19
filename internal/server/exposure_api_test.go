package server_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/exposure"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func tr() *bool { b := true; return &b }

func exposedSnap(ls ...exposure.Listener) func() (exposure.Snapshot, error) {
	return func() (exposure.Snapshot, error) { return exposure.Snapshot{Listeners: ls}, nil }
}

var mailpit = exposure.Listener{Proc: "mailpit", Port: 8025, Bind: "*", Scope: "all", LanReachable: tr()}

type expView struct {
	Items []struct {
		Key    string `json:"key"`
		Status string `json:"status"`
	} `json:"items"`
	Allowed  []string `json:"allowed"`
	Defaults []string `json:"defaults"`
	Findings int      `json:"findings"`
	CanEdit  bool     `json:"can_edit"`
}

// asEnv: bagian opsEnv (tipe tak diekspor) yang dipakai pembantu ini.
type asEnv interface {
	As(cookie, csrf, method, path, body string) (int, string)
}

func getExp(t *testing.T, o asEnv, cookie, csrf string) expView {
	t.Helper()
	var v expView
	code, body := o.As(cookie, csrf, "GET", "/api/exposure", "")
	if code != 200 || json.Unmarshal([]byte(body), &v) != nil {
		t.Fatalf("GET /api/exposure: %d %s", code, body)
	}
	return v
}

func statusOf(v expView, key string) string {
	for _, it := range v.Items {
		if it.Key == key {
			return it.Status
		}
	}
	return "(tidak ada)"
}

func TestExposureSampleCountsConsecutiveAndResets(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	now := o.App.Now()
	exposure.SetFetch(o.App, exposedSnap(mailpit))
	exposure.Sample(o.App, now)
	if f := exposure.Findings(exposure.InputsNow(o.App)); len(f) != 0 {
		t.Fatalf("sampel pertama belum cukup: %+v", f)
	}
	exposure.Sample(o.App, now.Add(5*time.Minute))
	if f := exposure.Findings(exposure.InputsNow(o.App)); len(f) != 1 {
		t.Fatalf("dua sampel berturut-turut: %+v", f)
	}
	// hilang satu sampel: hitungan reset
	exposure.SetFetch(o.App, exposedSnap())
	exposure.Sample(o.App, now.Add(10*time.Minute))
	exposure.SetFetch(o.App, exposedSnap(mailpit))
	exposure.Sample(o.App, now.Add(15*time.Minute))
	if f := exposure.Findings(exposure.InputsNow(o.App)); len(f) != 0 {
		t.Fatalf("hitungan harus reset setelah menghilang: %+v", f)
	}
}

func TestExposureFetchFailureKeepsCountsAndLastSnapshot(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	now := o.App.Now()
	exposure.SetFetch(o.App, exposedSnap(mailpit))
	exposure.Sample(o.App, now)
	exposure.Sample(o.App, now.Add(5*time.Minute))
	exposure.SetFetch(o.App, func() (exposure.Snapshot, error) { return exposure.Snapshot{}, errors.New("lsof mati") })
	exposure.Sample(o.App, now.Add(10*time.Minute))
	if f := exposure.Findings(exposure.InputsNow(o.App)); len(f) != 1 {
		t.Fatalf("gagal mengukur bukan berarti sudah aman: %+v", f)
	}
}

func TestExposureAllowlistDefaultsPersistenceAndExplicitEmpty(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if got := exposure.Allowed(o.App); strings.Join(got, ",") != strings.Join(exposure.DefaultAllow, ",") {
		t.Fatalf("bawaan: %v", got)
	}
	for _, bad := range []string{"bukan json", `{"a":1}`, `"str"`} {
		o.App.Store.SetSetting("exposure_allow", bad)
		if got := exposure.Allowed(o.App); len(got) != len(exposure.DefaultAllow) {
			t.Errorf("isi rusak %q harus jatuh ke bawaan: %v", bad, got)
		}
	}
	o.App.Store.SetSetting("exposure_allow", `["x:1","BAD ENTRY","x:1","y:70000","z:2"]`)
	if got := exposure.Allowed(o.App); strings.Join(got, ",") != "x:1,z:2" {
		t.Fatalf("entri tidak sah dan ganda dibuang: %v", got)
	}
	o.App.Store.SetSetting("exposure_allow", `[]`)
	if got := exposure.Allowed(o.App); len(got) != 0 {
		t.Fatalf("daftar yang sengaja dikosongkan dihormati (bawaan tidak muncul lagi): %v", got)
	}
}

func TestExposureAPIAllowRevokeFlowAffectsAlertsImmediately(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	now := o.App.Now()
	exposure.SetFetch(o.App, exposedSnap(mailpit))
	exposure.Sample(o.App, now)
	exposure.Sample(o.App, now.Add(5*time.Minute))
	alerts := func() string {
		o.Now = o.Now.Add(31 * time.Second)
		return o.Do("GET", "/api/alerts", "", o.Cookie, "", nil).Body.String()
	}
	v := getExp(t, o, o.Cookie, o.Csrf)
	if !v.CanEdit || statusOf(v, "mailpit:8025") != "flagged" || v.Findings != 1 {
		t.Fatalf("%+v", v)
	}
	if !strings.Contains(alerts(), "exposure-mailpit-8025") {
		t.Fatal("alert harus muncul di /api/alerts (dan karenanya di notifikasi ponsel)")
	}
	if code, b := o.Post("/api/exposure/allow", `{"entry":"Mailpit:8025"}`); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	v = getExp(t, o, o.Cookie, o.Csrf)
	if statusOf(v, "mailpit:8025") != "allowed" || v.Findings != 0 {
		t.Fatalf("setelah diizinkan: %+v", v)
	}
	if strings.Contains(alerts(), "exposure-mailpit-8025") {
		t.Fatal("alert harus hilang segera setelah diizinkan")
	}
	if code, _ := o.Post("/api/exposure/allow", `{"entry":"mailpit:8025"}`); code != 200 {
		t.Fatal("mengizinkan dua kali harus idempoten")
	}
	n := 0
	for _, e := range getExp(t, o, o.Cookie, o.Csrf).Allowed {
		if e == "mailpit:8025" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("tidak boleh ganda: %d", n)
	}
	if code, _ := o.Post("/api/exposure/revoke", `{"entry":"mailpit:8025"}`); code != 200 {
		t.Fatal("revoke")
	}
	if statusOf(getExp(t, o, o.Cookie, o.Csrf), "mailpit:8025") != "flagged" || !strings.Contains(alerts(), "exposure-mailpit-8025") {
		t.Fatal("setelah dicabut harus muncul lagi")
	}
	if code, _ := o.Post("/api/exposure/revoke", `{"entry":"tidak-ada:1"}`); code != 200 {
		t.Fatal("mencabut yang tidak ada harus idempoten")
	}
	var acts []string
	for _, a := range o.App.Store.RecentAudit(30) {
		if strings.HasPrefix(a.Action, "exposure_") {
			acts = append(acts, a.Action+":"+a.Detail)
		}
	}
	if strings.Join(acts, ",") != "exposure_revoke:mailpit:8025,exposure_allow:mailpit:8025" {
		t.Fatalf("audit hanya mencatat perubahan nyata, terbaru dulu: %v", acts)
	}
}

func TestExposureAPIRejectsBadInput(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	before := getExp(t, o, o.Cookie, o.Csrf).Allowed
	for _, body := range []string{`{"entry":""}`, `{"entry":"x"}`, `{"entry":"x:0"}`, `{"entry":"x:70000"}`, `{"entry":"a b:80"}`, `{"entry":"x:80;id"}`, `{}`, `[]`, `x`, `{"entry":5}`} {
		if code, _ := o.Post("/api/exposure/allow", body); code != 400 {
			t.Errorf("%s harus 400, dapat %d", body, code)
		}
	}
	after := getExp(t, o, o.Cookie, o.Csrf).Allowed
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Fatalf("input buruk tidak boleh mengubah daftar: %v -> %v", before, after)
	}
}

func TestExposureAPIAllowlistCapped(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	var l []string
	for i := 1; i <= exposure.MaxAllow; i++ {
		l = append(l, "p"+strings.Repeat("a", 1)+":"+itoa(i))
	}
	b, _ := json.Marshal(l)
	o.App.Store.SetSetting("exposure_allow", string(b))
	if code, _ := o.Post("/api/exposure/allow", `{"entry":"baru:9"}`); code != 400 {
		t.Fatalf("daftar penuh harus ditolak: %d", code)
	}
	if code, _ := o.Post("/api/exposure/revoke", `{"entry":"pa:1"}`); code != 200 {
		t.Fatal("mencabut tetap boleh saat penuh")
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestExposureAPIPermissions(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if w := o.Do("GET", "/api/exposure", "", "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa login: %d", w.Code)
	}
	if w := o.Do("POST", "/api/exposure/allow", `{"entry":"a:1"}`, "", "", nil); w.Code != 401 {
		t.Fatalf("POST tanpa login: %d", w.Code)
	}
	o.Stepup()
	tok, _ := o.MkToken("full")
	if code, _ := o.Bearer("GET", "/api/exposure", "", tok); code != 200 {
		t.Fatalf("token boleh membaca: %d", code)
	}
	for _, p := range []string{"/api/exposure/allow", "/api/exposure/revoke"} {
		if code, _ := o.Bearer("POST", p, `{"entry":"a:1"}`, tok); code != 403 {
			t.Errorf("token tidak boleh mengubah daftar izin %s: %d", p, code)
		}
	}
	if core.TokenAllowed("POST", "/api/exposure/allow") || !core.TokenAllowed("GET", "/api/exposure") {
		t.Fatal("TokenAllowed: tulis ditutup, baca terbuka")
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if v := getExp(t, o, o.Cookie, o.Csrf); v.CanEdit {
		t.Fatal("viewer tidak boleh mengedit")
	}
	if code, _ := o.Post("/api/exposure/allow", `{"entry":"a:1"}`); code != 403 {
		t.Fatalf("viewer: %d", code)
	}
}

func TestExposureJSONListsNeverNullBeforeFirstSample(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	w := o.Do("GET", "/api/exposure", "", o.Cookie, "", nil)
	body := w.Body.String()
	if !strings.Contains(body, `"items":[]`) || !strings.Contains(body, `"allowed":[`) || strings.Contains(body, `"allowed":null`) {
		t.Fatalf("daftar harus [] bukan null (UI membaca .length): %s", body)
	}
}
