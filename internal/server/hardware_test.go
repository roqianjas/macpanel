package server_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/hardware"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func f64(v float64) *float64 { return &v }

func goodSnap(cpu float64) hardware.Snapshot {
	mx := 4900
	return hardware.Snapshot{
		Source: "macmon", Temps: map[string]float64{"cpu": cpu, "gpu": 42, "ssd": 48},
		Fan:     &hardware.Fan{RPM: 1000, Max: &mx},
		Power:   &hardware.Power{System: f64(10.8), Chip: f64(2)},
		Mem:     &hardware.Mem{PressurePct: f64(40), SwapUsedMB: f64(0.2)},
		Thermal: &hardware.Thermal{SpeedLimit: 100},
	}
}

type hwResp struct {
	Now *struct {
		At    int64              `json:"at"`
		Temps map[string]float64 `json:"temps"`
	} `json:"now"`
	Stale    bool                    `json:"stale"`
	SSD      map[string]any          `json:"ssd"`
	Forecast map[string]any          `json:"forecast"`
	Series   map[string][][2]float64 `json:"series"`
	Peak     map[string]float64      `json:"peak"`
	Samples  int                     `json:"samples"`
}

func TestHardwareSampleStoreAndEndpoint(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) { return goodSnap(50), nil },
		func() (hardware.SSD, error) {
			return hardware.SSD{Name: "APPLE SSD", Smart: "Verified", Total: 245e9}, nil
		})
	base := o.App.Now()
	for i := 0; i < 3; i++ {
		hardware.Sample(o.App, base.Add(time.Duration(i-3)*30*time.Second))
	}
	hardware.Sample(o.App, base)
	w := o.Do("GET", "/api/hw?range=1h", "", o.Cookie, "", nil)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var r hwResp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Now == nil || r.Now.Temps["cpu"] != 50 || r.Stale {
		t.Fatalf("now=%+v stale=%v", r.Now, r.Stale)
	}
	if r.Now.At != base.Unix() {
		t.Fatalf("waktu sampel harus milik panel (%d), dapat %d", base.Unix(), r.Now.At)
	}
	if r.Samples != 4 || len(r.Series["cpu"]) == 0 || r.Peak["cpu"] != 50 || r.Peak["fan"] != 1000 {
		t.Fatalf("samples=%d cpu=%v peak=%v", r.Samples, r.Series["cpu"], r.Peak)
	}
	if r.Forecast["ok"] != false {
		t.Fatalf("perkiraan harus 'belum cukup data': %v", r.Forecast)
	}
}

func TestHardwareFailedReadWritesNothing(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	fail := false
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) {
		if fail {
			return hardware.Snapshot{}, errors.New("macmon mati")
		}
		return goodSnap(55), nil
	}, nil)
	base := o.App.Now()
	hardware.Sample(o.App, base.Add(-60*time.Second))
	fail = true
	hardware.Sample(o.App, base.Add(-30*time.Second))
	hardware.Sample(o.App, base)
	var r hwResp
	_ = json.Unmarshal(o.Do("GET", "/api/hw?range=1h", "", o.Cookie, "", nil).Body.Bytes(), &r)
	if r.Samples != 1 {
		t.Fatalf("sampel gagal tidak boleh menulis nol palsu: samples=%d", r.Samples)
	}
	if r.Now == nil || r.Now.Temps["cpu"] != 55 {
		t.Fatalf("data terakhir yang valid harus tetap ada: %+v", r.Now)
	}
}

func TestHardwareNoSourceIsNotStored(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) {
		return hardware.Snapshot{Source: "none", Errors: []string{"tidak ada sumber suhu"}}, nil
	}, nil)
	hardware.Sample(o.App, o.App.Now())
	var r hwResp
	_ = json.Unmarshal(o.Do("GET", "/api/hw", "", o.Cookie, "", nil).Body.Bytes(), &r)
	if r.Samples != 0 || len(r.Series["cpu"]) != 0 {
		t.Fatalf("sumber 'none' tidak boleh menulis riwayat: %+v", r)
	}
}

func TestHardwareImpossibleValuesNeverReachClientOrDatabase(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) { return goodSnap(999), nil }, nil)
	hardware.Sample(o.App, o.App.Now())
	var r hwResp
	_ = json.Unmarshal(o.Do("GET", "/api/hw", "", o.Cookie, "", nil).Body.Bytes(), &r)
	if _, ok := r.Now.Temps["cpu"]; ok {
		t.Fatalf("suhu 999 °C lolos ke klien: %v", r.Now.Temps)
	}
	if _, ok := r.Peak["cpu"]; ok || len(r.Series["cpu"]) != 0 {
		t.Fatalf("suhu mustahil masuk database: %v %v", r.Peak, r.Series["cpu"])
	}
	if r.Now.Temps["gpu"] != 42 {
		t.Fatal("nilai lain yang sah harus tetap ada")
	}
}

func TestHardwareStaleSampleIsFlagged(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) { return goodSnap(50), nil }, nil)
	hardware.Sample(o.App, o.App.Now().Add(-10*time.Minute))
	var r hwResp
	_ = json.Unmarshal(o.Do("GET", "/api/hw", "", o.Cookie, "", nil).Body.Bytes(), &r)
	if !r.Stale {
		t.Fatal("sampel 10 menit lalu harus ditandai basi")
	}
}

func TestHardwareEndpointValidationAndAuth(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for _, q := range []string{"range=2h", "range=", "range=7D", "range=1h;x"} {
		w := o.Do("GET", "/api/hw?"+q, "", o.Cookie, "", nil)
		if q == "range=" || q == "range=1h;x" { // ';' dibuang parser query Go, jadi jatuh ke bawaan 24h (sama dengan /api/metrics)
			if w.Code != 200 {
				t.Errorf("%s: %d", q, w.Code)
			}
			continue
		}
		if w.Code != 400 {
			t.Errorf("%s: %d, harus 400", q, w.Code)
		}
	}
	if w := o.Do("GET", "/api/hw", "", "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa login: %d", w.Code)
	}
	var r hwResp // sebelum sampel pertama: now null, bukan galat
	w := o.Do("GET", "/api/hw", "", o.Cookie, "", nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &r) != nil || r.Now != nil {
		t.Fatalf("sebelum sampel pertama: %d %s", w.Code, w.Body.String())
	}
}

func TestHardwareHotCPURaisesAlert(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) { return goodSnap(97), nil }, nil)
	hardware.Sample(o.App, o.App.Now())
	w := o.Do("GET", "/api/alerts", "", o.Cookie, "", nil)
	var r struct {
		Alerts []struct{ ID, Level, Title, Detail string } `json:"alerts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil || w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	found := false
	for _, a := range r.Alerts {
		if a.ID == "hw-temp-cpu" && a.Level == "bad" && a.Detail == "97 °C" {
			found = true
		}
	}
	if !found {
		t.Fatalf("alert suhu CPU tidak muncul: %s", w.Body.String())
	}
}

func TestHardwareRetentionPrunesOldRows(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) { return goodSnap(50), nil }, nil)
	now := o.App.Now()
	hardware.Sample(o.App, now.Add(-9*24*time.Hour))
	hardware.Sample(o.App, now.Add(-1*time.Hour))
	o.App.Store.PruneMetrics(now.Unix() - 7*24*3600)
	hs := o.App.Store.HWSeries(now.Unix()-30*24*3600, 1800)
	if hs.Samples != 1 {
		t.Fatalf("baris 9 hari harus terpangkas, tersisa %d", hs.Samples)
	}
}
