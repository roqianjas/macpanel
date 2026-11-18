package server_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/hardware"
	"github.com/roqianjas/macpanel/internal/testenv"
)

type thResp struct {
	Thresholds hardware.Thresholds `json:"thresholds"`
	Defaults   hardware.Thresholds `json:"defaults"`
	CanEdit    bool                `json:"can_edit"`
}

func TestHWThresholdsAPIRoundTripAndEffectOnAlerts(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	var r thResp
	if code, b := o.As(o.Cookie, o.Csrf, "GET", "/api/hw/thresholds", ""); code != 200 || json.Unmarshal([]byte(b), &r) != nil || r.Thresholds != hardware.DefaultThresholds() || !r.CanEdit {
		t.Fatalf("awal: %d %s", code, b)
	}
	// CPU 80 °C: normal pada bawaan, bad pada ambang ketat
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) { return goodSnap(80), nil }, nil)
	hardware.Sample(o.App, o.App.Now())
	alerts := func() string {
		o.Now = o.Now.Add(31 * time.Second)
		return o.Do("GET", "/api/alerts", "", o.Cookie, "", nil).Body.String()
	}
	if strings.Contains(alerts(), "hw-temp-cpu") {
		t.Fatal("80 °C tidak boleh memicu alert pada ambang bawaan")
	}
	code, b := o.Post("/api/hw/thresholds", `{"cpu_warn":70,"cpu_bad":78}`)
	if code != 200 || json.Unmarshal([]byte(b), &r) != nil || r.Thresholds.CPUBad != 78 || r.Thresholds.SSDBad != 80 {
		t.Fatalf("simpan: %d %s (field yang tidak dikirim harus tetap)", code, b)
	}
	hardware.Sample(o.App, o.App.Now())
	if a := alerts(); !strings.Contains(a, `"hw-temp-cpu"`) || !strings.Contains(a, `"bad"`) {
		t.Fatalf("ambang baru harus dipakai alert: %s", a)
	}
	// /api/hw menyertakan ambang yang sedang berlaku (UI memakainya untuk label)
	var hw struct{ Thresholds hardware.Thresholds }
	_ = json.Unmarshal(o.Do("GET", "/api/hw", "", o.Cookie, "", nil).Body.Bytes(), &hw)
	if hw.Thresholds.CPUBad != 78 {
		t.Fatalf("/api/hw tidak memuat ambang berlaku: %+v", hw.Thresholds)
	}
	// reset
	if code, b = o.Post("/api/hw/thresholds", `{"reset":true}`); code != 200 {
		t.Fatalf("reset: %d %s", code, b)
	}
	_ = json.Unmarshal([]byte(b), &r)
	if r.Thresholds != hardware.DefaultThresholds() {
		t.Fatalf("reset harus kembali ke bawaan: %+v", r.Thresholds)
	}
	var found bool
	for _, a := range o.App.Store.RecentAudit(20) {
		if a.Action == "hw_thresholds" {
			found = true
		}
	}
	if !found {
		t.Fatal("perubahan ambang harus tercatat di audit")
	}
}

func TestHWThresholdsRejectsBadInputAndKeepsOldValues(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	o.Post("/api/hw/thresholds", `{"cpu_warn":70,"cpu_bad":78}`)
	for _, body := range []string{
		`{"cpu_warn":20}`, `{"cpu_bad":300}`, `{"cpu_warn":96}`, `{"ssd_warn":90,"ssd_bad":80}`, `{"press_bad":150}`, `{"swap_mb":0}`,
		`{"cpu_warn":"panas"}`, `{"cpu_warn":null,"cpu_bad":"x"}`, `[]`, `x`,
	} {
		if code, _ := o.Post("/api/hw/thresholds", body); code != 400 {
			t.Errorf("%s harus 400, dapat %d", body, code)
		}
	}
	var r thResp
	_ = json.Unmarshal(o.Do("GET", "/api/hw/thresholds", "", o.Cookie, "", nil).Body.Bytes(), &r)
	if r.Thresholds.CPUWarn != 70 || r.Thresholds.CPUBad != 78 {
		t.Fatalf("input buruk tidak boleh mengubah nilai tersimpan: %+v", r.Thresholds)
	}
}

func TestHWThresholdsCorruptStoredValueFallsBackToDefaults(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for _, bad := range []string{`{"cpu_warn":0,"cpu_bad":0}`, `bukan json`, `{"cpu_warn":99,"cpu_bad":50}`, `{"swap_mb":-5}`} {
		o.App.Store.SetSetting("hw_thresholds", bad)
		if got := hardware.LoadThresholds(o.App); got != hardware.DefaultThresholds() {
			t.Errorf("isi rusak %q harus jatuh ke bawaan (bukan nol): %+v", bad, got)
		}
	}
	o.App.Store.SetSetting("hw_thresholds", `{"cpu_warn":75}`) // sebagian: sisanya bawaan
	if got := hardware.LoadThresholds(o.App); got.CPUWarn != 75 || got.CPUBad != 95 {
		t.Fatalf("sebagian harus digabung dengan bawaan: %+v", got)
	}
}

func TestHWThresholdsPermissions(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	if w := o.Do("GET", "/api/hw/thresholds", "", "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa login: %d", w.Code)
	}
	o.Stepup()
	tok, _ := o.MkToken("full")
	if code, _ := o.Bearer("POST", "/api/hw/thresholds", `{"cpu_warn":70,"cpu_bad":78}`, tok); code != 403 {
		t.Fatalf("token tidak boleh mengubah ambang: %d", code)
	}
	if code, _ := o.Bearer("GET", "/api/hw/thresholds", "", tok); code != 200 {
		t.Fatalf("token boleh membaca: %d", code)
	}
	if core.TokenAllowed("POST", "/api/hw/thresholds") || !core.TokenAllowed("GET", "/api/hw/thresholds") {
		t.Fatal("TokenAllowed: tulis ditutup, baca terbuka")
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	var r thResp
	if w := o.Do("GET", "/api/hw/thresholds", "", o.Cookie, "", nil); w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &r) != nil || r.CanEdit {
		t.Fatalf("viewer boleh baca tetapi can_edit=false: %d %s", w.Code, w.Body.String())
	}
	if code, _ := o.Post("/api/hw/thresholds", `{"cpu_warn":70,"cpu_bad":78}`); code != 403 {
		t.Fatalf("viewer tidak boleh mengubah: %d", code)
	}
}
