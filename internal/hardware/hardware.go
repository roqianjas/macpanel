// Package hardware: suhu, kipas, daya, tekanan memori, dan kesehatan SSD.
// Data datang dari `lab sys hw` / `lab sys ssd` (macmon utama, Netdata cadangan). Paket ini memvalidasi ulang
// semuanya (jangan percaya keluaran proses luar), menyimpan riwayat 7 hari, dan menurunkan peringatan.
package hardware

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

const (
	SampleEvery = 30 * time.Second
	ssdEvery    = 10 * time.Minute
	// Sampel dianggap basi bila lebih tua dari ini; UI dan alert tidak boleh menampilkan angka lama sebagai angka sekarang.
	StaleAfter = 4 * SampleEvery
	retain     = 7 * 24 * 3600

	tempMin, tempMax = 5.0, 125.0
)

type Power struct {
	Chip   *float64 `json:"chip"`
	DCIn   *float64 `json:"dc_in"`
	System *float64 `json:"system"`
}

type Fan struct {
	RPM int  `json:"rpm"`
	Max *int `json:"max"`
}

type Mem struct {
	PressurePct *float64 `json:"pressure_pct"`
	SwapUsedMB  *float64 `json:"swap_used_mb"`
	SwapTotalMB *float64 `json:"swap_total_mb"`
}

type Thermal struct {
	SpeedLimit int  `json:"speed_limit"`
	Throttled  bool `json:"throttled"`
}

// Snapshot cermin keluaran `lab sys hw`.
type Snapshot struct {
	At      int64              `json:"at"`
	Source  string             `json:"source"` // macmon | netdata | none
	Temps   map[string]float64 `json:"temps"`
	Fan     *Fan               `json:"fan"`
	Power   *Power             `json:"power"`
	Mem     *Mem               `json:"mem"`
	Thermal *Thermal           `json:"thermal"`
	Errors  []string           `json:"errors"`
}

type SSD struct {
	Name       string `json:"name"`
	SolidState bool   `json:"solid_state"`
	Smart      string `json:"smart"` // Verified | Failing | Not Supported | ""
	Total      uint64 `json:"total"`
	Free       uint64 `json:"free"`
	Wear       string `json:"wear"`
	At         int64  `json:"at"`
}

func okTemp(v float64) bool { return !math.IsNaN(v) && v >= tempMin && v <= tempMax }

func okRange(p *float64, lo, hi float64) *float64 {
	if p == nil || math.IsNaN(*p) || *p < lo || *p > hi {
		return nil
	}
	return p
}

// Sanitize membuang nilai mustahil (bukan menggantinya): sensor yang rusak harus tampil sebagai "tidak ada".
func (s *Snapshot) Sanitize() {
	clean := map[string]float64{}
	for k, v := range s.Temps {
		if okTemp(v) {
			clean[k] = v
		}
	}
	s.Temps = clean
	if s.Fan != nil && (s.Fan.RPM < 0 || s.Fan.RPM > 20000) {
		s.Fan = nil
	}
	if s.Fan != nil && s.Fan.Max != nil && (*s.Fan.Max < 1 || *s.Fan.Max > 20000) {
		s.Fan.Max = nil
	}
	if s.Power != nil {
		s.Power.Chip, s.Power.DCIn, s.Power.System = okRange(s.Power.Chip, 0, 1000), okRange(s.Power.DCIn, 0, 1000), okRange(s.Power.System, 0, 1000)
	}
	if s.Mem != nil {
		s.Mem.PressurePct = okRange(s.Mem.PressurePct, 0, 100)
		s.Mem.SwapUsedMB = okRange(s.Mem.SwapUsedMB, 0, 1e7)
		s.Mem.SwapTotalMB = okRange(s.Mem.SwapTotalMB, 0, 1e7)
	}
	if s.Thermal != nil && (s.Thermal.SpeedLimit < 0 || s.Thermal.SpeedLimit > 100) {
		s.Thermal = nil
	}
	switch s.Source {
	case "macmon", "netdata", "none":
	default:
		s.Source = "none"
	}
	if s.Temps == nil {
		s.Temps = map[string]float64{}
	}
	if s.Errors == nil {
		s.Errors = []string{}
	}
}

func fp(v float64) *float64 { return &v }

func (s *Snapshot) point() store.HWPoint {
	p := store.HWPoint{Src: s.Source}
	for k, dst := range map[string]**float64{"cpu": &p.CPU, "gpu": &p.GPU, "ssd": &p.SSD, "memory": &p.Mem, "ambient": &p.Ambient} {
		if v, ok := s.Temps[k]; ok {
			*dst = fp(v)
		}
	}
	if s.Fan != nil {
		p.Fan = fp(float64(s.Fan.RPM))
	}
	if s.Power != nil {
		p.Power = s.Power.System
	}
	if s.Mem != nil {
		p.Press, p.Swap = s.Mem.PressurePct, s.Mem.SwapUsedMB
	}
	p.Throttled = s.Thermal != nil && s.Thermal.Throttled
	return p
}

type state struct {
	mu    sync.Mutex
	snap  *Snapshot
	ssd   *SSD
	ssdAt time.Time
	fetch func() (Snapshot, error)
	ssdFn func() (SSD, error)
	fails int
}

func st(a *core.App) *state { return core.Local[state](a, "hardware") }

// SetFetch mengganti sumber data (tes). Nil mengembalikan ke `lab`.
func SetFetch(a *core.App, f func() (Snapshot, error), s func() (SSD, error)) {
	x := st(a)
	x.mu.Lock()
	x.fetch, x.ssdFn = f, s
	x.mu.Unlock()
}

func fetchLab(a *core.App) (Snapshot, error) {
	res := a.RunLabTimeout([]string{"sys", "hw"}, "", 25*time.Second)
	var s Snapshot
	if res.Code != 0 || res.TimedOut {
		return s, errors.New("lab sys hw gagal")
	}
	if err := json.Unmarshal([]byte(res.Out), &s); err != nil {
		return s, errors.New("keluaran lab sys hw bukan JSON")
	}
	return s, nil
}

func ssdLab(a *core.App) (SSD, error) {
	res := a.RunLabTimeout([]string{"sys", "ssd"}, "", 25*time.Second)
	var s SSD
	if res.Code != 0 || res.TimedOut {
		return s, errors.New("lab sys ssd gagal")
	}
	if err := json.Unmarshal([]byte(res.Out), &s); err != nil {
		return s, errors.New("keluaran lab sys ssd bukan JSON")
	}
	return s, nil
}

// Sample mengambil satu sampel dan menyimpannya. Gagal baca tidak menulis apa pun (bukan nol palsu).
func Sample(a *core.App, now time.Time) {
	x := st(a)
	x.mu.Lock()
	f := x.fetch
	x.mu.Unlock()
	if f == nil {
		f = func() (Snapshot, error) { return fetchLab(a) }
	}
	s, err := f()
	x.mu.Lock()
	defer x.mu.Unlock()
	if err != nil {
		x.fails++
		return
	}
	x.fails = 0
	s.At = now.Unix() // waktu sampel milik panel, bukan jam proses luar
	s.Sanitize()
	x.snap = &s
	if s.Source != "none" {
		a.Store.AddHW(s.At, s.point())
	}
}

func sampleSSD(a *core.App, now time.Time) {
	x := st(a)
	x.mu.Lock()
	f := x.ssdFn
	x.mu.Unlock()
	if f == nil {
		f = func() (SSD, error) { return ssdLab(a) }
	}
	d, err := f()
	x.mu.Lock()
	defer x.mu.Unlock()
	x.ssdAt = now
	if err != nil {
		return
	}
	d.Name, d.Smart = store.Clean(d.Name, 80), store.Clean(d.Smart, 40)
	d.At = now.Unix()
	x.ssd = &d
}

// Run mengumpulkan sampel sampai ctx berhenti. Sampel pertama langsung diambil.
func Run(ctx context.Context, a *core.App) {
	sampleSSD(a, a.Now())
	Sample(a, a.Now())
	tk := time.NewTicker(SampleEvery)
	defer tk.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tk.C:
			Sample(a, now)
			if n++; n%int(ssdEvery/SampleEvery) == 0 {
				sampleSSD(a, now)
			}
			if n%120 == 0 {
				a.Store.PruneHW(now.Unix() - retain)
			}
		}
	}
}

// Current mengembalikan salinan sampel terakhir dan apakah sudah basi.
func Current(a *core.App, now time.Time) (snap *Snapshot, stale bool, ssd *SSD) {
	x := st(a)
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.snap != nil {
		c := *x.snap
		snap = &c
		stale = now.Unix()-c.At > int64(StaleAfter.Seconds())
	}
	if x.ssd != nil {
		c := *x.ssd
		ssd = &c
	}
	return
}
