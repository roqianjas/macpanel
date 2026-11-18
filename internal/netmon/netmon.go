// Package netmon: konektivitas mesin (Tailscale, internet, DNS). Data dari `lab sys net`, divalidasi ulang di sini,
// disimpan 7 hari, dan diturunkan menjadi peringatan. Alert baru muncul setelah beberapa sampel gagal berturut-turut:
// satu paket hilang tidak boleh membuat pemilik panik, tetapi putus 3 menit harus terlihat.
package netmon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

const (
	SampleEvery = 60 * time.Second
	StaleAfter  = 4 * SampleEvery
	retain      = 7 * 24 * 3600

	// Ambang alert (jumlah sampel berturut-turut = menit, karena sampel tiap 60 detik).
	TSDownSamples   = 2
	InetDownSamples = 3
	DNSDownSamples  = 2
	SlowMS          = 300.0
	SlowWindow      = 5
	maxMS           = 60000.0
)

type Tailscale struct {
	State       string   `json:"state"`
	SelfOnline  bool     `json:"self_online"`
	Peers       int      `json:"peers"`
	PeersOnline int      `json:"peers_online"`
	Health      []string `json:"health"`
}

type Internet struct {
	OK bool     `json:"ok"`
	MS *float64 `json:"ms"`
}

type DNS struct {
	Name string   `json:"name"`
	OK   bool     `json:"ok"`
	MS   *float64 `json:"ms"`
}

// Snapshot cermin keluaran `lab sys net`.
type Snapshot struct {
	At        int64      `json:"at"`
	Tailscale *Tailscale `json:"tailscale"`
	Internet  Internet   `json:"internet"`
	DNSLocal  *DNS       `json:"dns_local"`
	DNSPublic *DNS       `json:"dns_public"`
	Errors    []string   `json:"errors"`
}

func okMS(p *float64) *float64 {
	if p == nil || math.IsNaN(*p) || *p < 0 || *p > maxMS {
		return nil
	}
	return p
}

// Sanitize membuang nilai mustahil dan membatasi teks dari proses luar.
func (s *Snapshot) Sanitize() {
	if t := s.Tailscale; t != nil {
		t.State = store.Clean(t.State, 20)
		if t.Peers < 0 || t.Peers > 10000 {
			t.Peers = 0
		}
		if t.PeersOnline < 0 || t.PeersOnline > t.Peers {
			t.PeersOnline = 0
		}
		if t.Health == nil {
			t.Health = []string{}
		}
		for i := range t.Health {
			t.Health[i] = store.Clean(t.Health[i], 120)
		}
	}
	s.Internet.MS = okMS(s.Internet.MS)
	if !s.Internet.OK {
		s.Internet.MS = nil
	}
	for _, d := range []*DNS{s.DNSLocal, s.DNSPublic} {
		if d != nil {
			d.Name = store.Clean(d.Name, 100)
			d.MS = okMS(d.MS)
			if !d.OK {
				d.MS = nil
			}
		}
	}
	if s.Errors == nil {
		s.Errors = []string{}
	}
}

func (s *Snapshot) point() store.NetqPoint {
	p := store.NetqPoint{T: s.At, TSSelf: -1, DNSLocal: -1, InetOK: s.Internet.OK, InetMS: s.Internet.MS}
	if t := s.Tailscale; t != nil {
		p.TSState, p.TSPeers, p.TSOnline = t.State, t.Peers, t.PeersOnline
		p.TSSelf = 0
		if t.SelfOnline {
			p.TSSelf = 1
		}
	}
	if d := s.DNSLocal; d != nil {
		p.DNSLocal = 0
		if d.OK {
			p.DNSLocal = 1
		}
		p.DNSLocalMS = d.MS
	}
	if d := s.DNSPublic; d != nil {
		p.DNSPublicOK, p.DNSPublicMS = d.OK, d.MS
	}
	return p
}

type state struct {
	mu    sync.Mutex
	snap  *Snapshot
	fetch func() (Snapshot, error)
}

func st(a *core.App) *state { return core.Local[state](a, "netmon") }

// SetFetch mengganti sumber data (tes).
func SetFetch(a *core.App, f func() (Snapshot, error)) {
	x := st(a)
	x.mu.Lock()
	x.fetch = f
	x.mu.Unlock()
}

func fetchLab(a *core.App) (Snapshot, error) {
	var s Snapshot
	res := a.RunLabTimeout([]string{"sys", "net", "--dns-name", "panel." + a.TLD}, "", 30*time.Second)
	if res.Code != 0 || res.TimedOut {
		return s, errors.New("lab sys net gagal")
	}
	if err := json.Unmarshal([]byte(res.Out), &s); err != nil {
		return s, errors.New("keluaran lab sys net bukan JSON")
	}
	return s, nil
}

// Sample mengambil dan menyimpan satu sampel. Gagal mengambil TIDAK ditulis sebagai "internet mati":
// tidak mampu mengukur berbeda dari hasil ukur yang buruk.
func Sample(a *core.App, now time.Time) {
	x := st(a)
	x.mu.Lock()
	f := x.fetch
	x.mu.Unlock()
	if f == nil {
		f = func() (Snapshot, error) { return fetchLab(a) }
	}
	s, err := f()
	if err != nil {
		return
	}
	s.At = now.Unix()
	s.Sanitize()
	x.mu.Lock()
	x.snap = &s
	x.mu.Unlock()
	a.Store.AddNetq(s.At, s.point())
}

func Run(ctx context.Context, a *core.App) {
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
			if n++; n%120 == 0 {
				a.Store.PruneNetq(now.Unix() - retain)
			}
		}
	}
}

// Finding: peringatan turunan (overview mengubahnya menjadi Alert).
type Finding struct{ ID, Level, Title, Detail string }

type Inputs struct {
	Snap   *Snapshot
	Stale  bool
	Recent []store.NetqPoint // terbaru dulu
	Now    time.Time
}

func InputsNow(a *core.App) Inputs {
	now := a.Now()
	x := st(a)
	x.mu.Lock()
	var snap *Snapshot
	if x.snap != nil {
		c := *x.snap
		snap = &c
	}
	x.mu.Unlock()
	in := Inputs{Snap: snap, Recent: a.Store.NetqRecent(SlowWindow), Now: now}
	in.Stale = snap != nil && now.Unix()-snap.At > int64(StaleAfter.Seconds())
	return in
}

// lastN: n sampel terbaru semuanya memenuhi bad, dan jumlahnya cukup.
func lastN(r []store.NetqPoint, n int, bad func(store.NetqPoint) bool) bool {
	if len(r) < n {
		return false
	}
	for _, p := range r[:n] {
		if !bad(p) {
			return false
		}
	}
	return true
}

// Findings murni (tanpa I/O).
func Findings(in Inputs) []Finding {
	out := []Finding{}
	if in.Snap == nil {
		return out
	}
	if in.Stale {
		return append(out, Finding{"net-stale", "warn", "Pemantauan konektivitas berhenti", fmt.Sprintf("sampel terakhir %d menit lalu", (in.Now.Unix()-in.Snap.At)/60)})
	}
	r := in.Recent
	if lastN(r, TSDownSamples, func(p store.NetqPoint) bool { return p.TSState != "" && p.TSState != "Running" }) {
		out = append(out, Finding{"net-tailscale", "bad", "Tailscale tidak berjalan", "status: " + r[0].TSState + ". Akses jarak jauh ke mesin ini terputus."})
	} else if lastN(r, TSDownSamples, func(p store.NetqPoint) bool { return p.TSState == "Running" && p.TSSelf == 0 }) {
		out = append(out, Finding{"net-tailscale", "bad", "Tailscale terputus dari jaringan", "berjalan tetapi mesin ini tidak online di tailnet"})
	}
	if lastN(r, InetDownSamples, func(p store.NetqPoint) bool { return !p.InetOK }) {
		out = append(out, Finding{"net-internet", "warn", "Internet tidak terjangkau", fmt.Sprintf("gagal %d pengukuran berturut-turut", InetDownSamples)})
	} else if len(r) >= SlowWindow {
		var sum float64
		n := 0
		for _, p := range r {
			if p.InetOK && p.InetMS != nil {
				sum += *p.InetMS
				n++
			}
		}
		if n >= SlowWindow-1 && sum/float64(n) > SlowMS {
			out = append(out, Finding{"net-slow", "warn", "Internet lambat", fmt.Sprintf("rata-rata %.0f ms (sambung TCP)", sum/float64(n))})
		}
	}
	if lastN(r, DNSDownSamples, func(p store.NetqPoint) bool { return p.DNSLocal == 0 }) {
		out = append(out, Finding{"net-dns", "warn", "Nama lokal tidak bisa di-resolve", "domain .lab tidak terjawab; akses lewat nama gagal, lewat IP masih bisa"})
	}
	return out
}

var ranges = map[string]struct{ Sec, Bucket int64 }{"1h": {3600, 60}, "24h": {86400, 300}, "7d": {7 * 86400, 1800}}

// Handler: GET /api/netmon?range=1h|24h|7d
func Handler(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	rk := r.URL.Query().Get("range")
	if rk == "" {
		rk = "24h"
	}
	rs, ok := ranges[rk]
	if !ok {
		core.WriteError(w, 400, "range harus 1h, 24h, atau 7d")
		return
	}
	in := InputsNow(a)
	stats := a.Store.NetqStats(in.Now.Unix()-rs.Sec, rs.Bucket)
	core.WriteJSON(w, 200, map[string]any{
		"now": in.Snap, "stale": in.Stale, "range": rk, "bucket": rs.Bucket, "interval": int(SampleEvery.Seconds()),
		"series": stats.Series, "samples": stats.Samples, "ts_down": stats.TSDown, "inet_down": stats.InetDown,
		"dns_local_down": stats.DNSLocalDown, "dns_local_n": stats.DNSLocalN, "inet_max_ms": stats.InetMaxMS,
		"thresholds": map[string]float64{"slow_ms": SlowMS},
	})
}
