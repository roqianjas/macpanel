package hardware

import (
	"math"
	"testing"
	"time"
)

func series(days float64, n int, start, perDay float64) [][2]float64 {
	pts := make([][2]float64, n)
	for i := range pts {
		x := days * float64(i) / float64(n-1)
		pts[i] = [2]float64{1_700_000_000 + x*86400, start + perDay*x}
	}
	return pts
}

func TestForecastLinearGrowth(t *testing.T) {
	f := ForecastDisk(series(5, 200, 50, 2), 245_054_767_104) // 50% -> 60% dalam 5 hari = 2 %/hari
	if !f.OK || f.DaysToFull == nil {
		t.Fatalf("%+v", f)
	}
	if math.Abs(f.PctPerDay-2) > 0.01 || math.Abs(*f.DaysToFull-20) > 0.2 || f.R2 < 0.99 {
		t.Fatalf("pct/hari=%v hari=%v r2=%v", f.PctPerDay, *f.DaysToFull, f.R2)
	}
	if f.GBPerDay == nil || math.Abs(*f.GBPerDay-4.9) > 0.1 {
		t.Fatalf("GB/hari=%v, mau ~4.9 (2%% dari 245 GB)", f.GBPerDay)
	}
}

func TestForecastNeedsEnoughData(t *testing.T) {
	for name, pts := range map[string][][2]float64{
		"kosong": nil, "satu titik": {{1, 50}}, "terlalu sedikit titik": series(3, 10, 50, 1),
		"kurang dari sehari": series(0.5, 100, 50, 5),
	} {
		f := ForecastDisk(pts, 1e12)
		if f.OK || f.Reason == "" || f.DaysToFull != nil {
			t.Errorf("%s: %+v", name, f)
		}
	}
}

func TestForecastFlatAndShrinking(t *testing.T) {
	for _, slope := range []float64{0, -1, 0.01} {
		f := ForecastDisk(series(3, 100, 50, slope), 1e12)
		if !f.OK || f.DaysToFull != nil || f.Reason == "" {
			t.Errorf("slope %v: %+v", slope, f)
		}
	}
}

func TestForecastAlreadyFullAndFarFuture(t *testing.T) {
	f := ForecastDisk(series(3, 100, 99, 0.2), 1e12) // 99 -> 99.6, tersisa 0.4% pada 0.2%/hari
	if f.DaysToFull == nil || *f.DaysToFull > 3 {
		t.Fatalf("%+v", f)
	}
	f = ForecastDisk(series(3, 100, 10, 0.03), 1e12) // 0.03 %/hari = 3000 hari
	if f.DaysToFull == nil || *f.DaysToFull < 2900 {
		t.Fatalf("%+v", f)
	}
	f = ForecastDisk(series(3, 100, 10, 0.025), 1e12) // 3600 hari: masih di bawah batas
	if f.DaysToFull == nil {
		t.Fatalf("%+v", f)
	}
	f = ForecastDisk(series(3, 100, 10, 0.021), 1e12) // 4285 hari > 10 tahun
	if f.DaysToFull != nil || f.Reason == "" {
		t.Fatalf("lebih dari 10 tahun harus tanpa angka: %+v", f)
	}
}

func TestForecastNoisyDataHasLowR2(t *testing.T) {
	pts := series(5, 200, 50, 0.5)
	for i := range pts {
		if i%2 == 0 {
			pts[i][1] += 8
		} else {
			pts[i][1] -= 8
		}
	}
	if f := ForecastDisk(pts, 1e12); f.R2 >= 0.5 {
		t.Fatalf("data berisik harus ber-R2 rendah, dapat %v", f.R2)
	}
}

func fl(v float64) *float64 { return &v }

func snap(temps map[string]float64) *Snapshot {
	return &Snapshot{At: 1000, Source: "macmon", Temps: temps, Errors: []string{}}
}

func ids(fs []Finding) map[string]string {
	m := map[string]string{}
	for _, f := range fs {
		m[f.ID] = f.Level
	}
	return m
}

func TestFindingsTemperatureThresholds(t *testing.T) {
	now := time.Unix(1010, 0)
	cases := []struct {
		cpu  float64
		want string
	}{{40, ""}, {84.9, ""}, {85, "warn"}, {94.9, "warn"}, {95, "bad"}, {120, "bad"}}
	for _, c := range cases {
		got := ids(Findings(Inputs{Th: DefaultThresholds(), Snap: snap(map[string]float64{"cpu": c.cpu}), Now: now}))["hw-temp-cpu"]
		if got != c.want {
			t.Errorf("cpu %v: %q, mau %q", c.cpu, got, c.want)
		}
	}
	got := ids(Findings(Inputs{Th: DefaultThresholds(), Snap: snap(map[string]float64{"ssd": 72, "gpu": 96}), Now: now}))
	if got["hw-temp-ssd"] != "warn" || got["hw-temp-gpu"] != "bad" {
		t.Fatalf("%v", got)
	}
}

func TestFindingsThrottleMemoryAndSwap(t *testing.T) {
	now := time.Unix(1010, 0)
	s := snap(map[string]float64{"cpu": 50})
	s.Thermal = &Thermal{SpeedLimit: 70, Throttled: true}
	s.Mem = &Mem{PressurePct: fl(85), SwapUsedMB: fl(3000)}
	got := ids(Findings(Inputs{Th: DefaultThresholds(), Snap: s, Now: now}))
	if got["hw-throttle"] != "bad" || got["hw-mempress"] != "warn" || got["hw-swap"] != "warn" {
		t.Fatalf("%v", got)
	}
	s.Mem.PressurePct = fl(95)
	if ids(Findings(Inputs{Th: DefaultThresholds(), Snap: s, Now: now}))["hw-mempress"] != "bad" {
		t.Fatal("tekanan 95% harus bad")
	}
	s.Mem, s.Thermal = &Mem{PressurePct: fl(40), SwapUsedMB: fl(0.2)}, &Thermal{SpeedLimit: 100}
	if len(Findings(Inputs{Th: DefaultThresholds(), Snap: s, Now: now})) != 0 {
		t.Fatal("kondisi normal tidak boleh memunculkan peringatan")
	}
}

func TestFindingsStaleHidesStaleReadings(t *testing.T) {
	// angka panas yang sudah basi tidak boleh memicu peringatan suhu; yang muncul peringatan "berhenti"
	got := ids(Findings(Inputs{Th: DefaultThresholds(), Snap: snap(map[string]float64{"cpu": 99}), Stale: true, Now: time.Unix(1000+600, 0)}))
	if len(got) != 1 || got["hw-stale"] != "warn" {
		t.Fatalf("%v", got)
	}
}

func TestFindingsNoSensor(t *testing.T) {
	s := &Snapshot{At: 1000, Source: "none", Temps: map[string]float64{}, Errors: []string{"macmon tidak terpasang", "tidak ada sumber suhu (pasang macmon)"}}
	fs := Findings(Inputs{Th: DefaultThresholds(), Snap: s, Now: time.Unix(1010, 0)})
	if len(fs) != 1 || fs[0].ID != "hw-nosensor" || fs[0].Detail != s.Errors[1] {
		t.Fatalf("%+v", fs)
	}
}

func TestFindingsBeforeFirstSampleStaysQuiet(t *testing.T) {
	if fs := Findings(Inputs{Th: DefaultThresholds(), Now: time.Unix(1, 0)}); len(fs) != 0 {
		t.Fatalf("sebelum sampel pertama tidak boleh ada alert: %+v", fs)
	}
}

func TestFindingsSMART(t *testing.T) {
	now := time.Unix(1010, 0)
	for smart, want := range map[string]string{"Verified": "", "Not Supported": "", "": "", "Failing": "bad", "About to fail": "bad"} {
		got := ids(Findings(Inputs{Th: DefaultThresholds(), SSD: &SSD{Smart: smart}, Now: now}))["hw-smart"]
		if got != want {
			t.Errorf("smart %q: %q, mau %q", smart, got, want)
		}
	}
}

func TestFindingsDiskForecast(t *testing.T) {
	d := func(days, r2 float64) Forecast { return Forecast{OK: true, DaysToFull: &days, R2: r2, PctPerDay: 1} }
	now := time.Unix(1010, 0)
	for _, c := range []struct {
		f    Forecast
		want string
	}{{d(2, 0.9), "bad"}, {d(10, 0.9), "warn"}, {d(14, 0.9), ""}, {d(30, 0.99), ""}, {d(2, 0.3), ""}} {
		if got := ids(Findings(Inputs{Th: DefaultThresholds(), Forecast: c.f, Now: now}))["hw-diskforecast"]; got != c.want {
			t.Errorf("%v hari r2 %v: %q, mau %q", *c.f.DaysToFull, c.f.R2, got, c.want)
		}
	}
}

func TestSanitizeDropsImpossibleValuesInsteadOfInventingOnes(t *testing.T) {
	mx := 0
	s := &Snapshot{
		Source: "aneh", Temps: map[string]float64{"cpu": 0, "gpu": 200, "ssd": 48, "memory": math.NaN()},
		Fan:     &Fan{RPM: -1, Max: &mx},
		Power:   &Power{Chip: fl(-2), DCIn: fl(7), System: fl(5000)},
		Mem:     &Mem{PressurePct: fl(140), SwapUsedMB: fl(12), SwapTotalMB: fl(-1)},
		Thermal: &Thermal{SpeedLimit: 400},
	}
	s.Sanitize()
	if len(s.Temps) != 1 || s.Temps["ssd"] != 48 {
		t.Fatalf("suhu: %v", s.Temps)
	}
	if s.Fan != nil || s.Thermal != nil || s.Source != "none" {
		t.Fatalf("fan=%v thermal=%v source=%q", s.Fan, s.Thermal, s.Source)
	}
	if s.Power.Chip != nil || s.Power.System != nil || s.Power.DCIn == nil {
		t.Fatalf("daya: %+v", s.Power)
	}
	if s.Mem.PressurePct != nil || s.Mem.SwapTotalMB != nil || s.Mem.SwapUsedMB == nil {
		t.Fatalf("mem: %+v", s.Mem)
	}
}

func TestSanitizeKeepsZeroRPMFan(t *testing.T) {
	s := &Snapshot{Source: "macmon", Fan: &Fan{RPM: 0}}
	s.Sanitize()
	if s.Fan == nil || s.Fan.RPM != 0 {
		t.Fatal("kipas 0 rpm adalah data nyata, bukan hilang")
	}
	if s.Temps == nil || s.Errors == nil {
		t.Fatal("peta dan daftar tidak boleh nil (JSON null merusak UI)")
	}
}
