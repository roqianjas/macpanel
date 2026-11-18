package hardware

import (
	"math"
	"testing"
	"time"
)

func TestDefaultThresholdsAreValid(t *testing.T) {
	if err := DefaultThresholds().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestThresholdsValidate(t *testing.T) {
	mod := func(f func(*Thresholds)) Thresholds { d := DefaultThresholds(); f(&d); return d }
	bad := map[string]Thresholds{
		"cpu warn terlalu rendah (alert menyala terus)": mod(func(t *Thresholds) { t.CPUWarn = 20 }),
		"cpu bad mustahil":            mod(func(t *Thresholds) { t.CPUBad = 300 }),
		"cpu warn sama dengan bad":    mod(func(t *Thresholds) { t.CPUWarn = 95 }),
		"cpu warn di atas bad":        mod(func(t *Thresholds) { t.CPUWarn = 99 }),
		"ssd terbalik":                mod(func(t *Thresholds) { t.SSDWarn, t.SSDBad = 80, 70 }),
		"ssd bad > 110":               mod(func(t *Thresholds) { t.SSDBad = 111 }),
		"tekanan > 100":               mod(func(t *Thresholds) { t.PressBad = 101 }),
		"tekanan warn terlalu rendah": mod(func(t *Thresholds) { t.PressWarn = 10 }),
		"swap nol":                    mod(func(t *Thresholds) { t.SwapMB = 0 }),
		"swap raksasa":                mod(func(t *Thresholds) { t.SwapMB = 1e9 }),
		"nan":                         mod(func(t *Thresholds) { t.CPUWarn = math.NaN() }),
		"nan di swap":                 mod(func(t *Thresholds) { t.SwapMB = math.NaN() }),
	}
	for name, th := range bad {
		if th.Validate() == nil {
			t.Errorf("%s: harus ditolak", name)
		}
	}
	ok := mod(func(t *Thresholds) { t.CPUWarn, t.CPUBad, t.SwapMB = 40, 41, 256 })
	if err := ok.Validate(); err != nil {
		t.Errorf("batas bawah yang sah ditolak: %v", err)
	}
	ok = mod(func(t *Thresholds) { t.CPUBad, t.PressBad, t.SwapMB = 120, 100, 65536 })
	if err := ok.Validate(); err != nil {
		t.Errorf("batas atas yang sah ditolak: %v", err)
	}
}

func TestFindingsUseConfiguredThresholds(t *testing.T) {
	now := time.Unix(1010, 0)
	cpu := func(v float64, th Thresholds) string {
		return ids(Findings(Inputs{Th: th, Snap: snap(map[string]float64{"cpu": v}), Now: now}))["hw-temp-cpu"]
	}
	d := DefaultThresholds()
	if cpu(80, d) != "" {
		t.Fatal("80 °C normal pada bawaan")
	}
	strict := d
	strict.CPUWarn, strict.CPUBad = 70, 78
	if cpu(80, strict) != "bad" || cpu(72, strict) != "warn" || cpu(60, strict) != "" {
		t.Fatalf("ambang ketat tidak dipakai: %q %q %q", cpu(80, strict), cpu(72, strict), cpu(60, strict))
	}
	loose := d
	loose.CPUWarn, loose.CPUBad = 100, 110
	if cpu(97, loose) != "" || cpu(101, loose) != "warn" {
		t.Fatalf("ambang longgar tidak dipakai: %q %q", cpu(97, loose), cpu(101, loose))
	}
	// swap dan tekanan juga mengikuti
	s := snap(map[string]float64{"cpu": 50})
	s.Mem = &Mem{PressurePct: fl(60), SwapUsedMB: fl(500)}
	th := d
	th.PressWarn, th.PressBad, th.SwapMB = 50, 70, 300
	got := ids(Findings(Inputs{Th: th, Snap: s, Now: now}))
	if got["hw-mempress"] != "warn" || got["hw-swap"] != "warn" {
		t.Fatalf("%v", got)
	}
}
