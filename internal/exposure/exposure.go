// Package exposure: layanan yang mendengar di luar loopback dan terjangkau dari jaringan rumah.
// Data dari `lab sys exposure` (lsof + sambungan TCP ke IP LAN), divalidasi ulang di sini. Satu temuan hanya muncul bila
// layanan yang sama terlihat terbuka pada beberapa sampel berturut-turut: layanan yang baru naik tidak boleh langsung
// membuat panik, tetapi yang terbuka berjam-jam harus terlihat (kasus Mailpit yang terbuka ke LAN tanpa terlihat).
//
// Batas yang jujur: lsof hanya melihat proses milik user ini. Layanan milik root (SSH, Screen Sharing) ditemukan dari
// sambungan ke IP LAN dan dilaporkan sebagai "proses tidak terlihat" dengan tebakan nama dari nomor port.
package exposure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

const (
	SampleEvery    = 5 * time.Minute
	StaleAfter     = 3 * SampleEvery
	ConfirmSamples = 2
	MaxAllow       = 100
	maxListeners   = 200
	allowKey       = "exposure_allow"
)

// DefaultAllow: pengecualian bawaan. nginx membuka 80/443 untuk semua antarmuka tetapi menolak (403) yang bukan dari
// tailnet. "?:22" dan "?:5900" adalah SSH dan Screen Sharing (milik root, nama prosesnya tidak terlihat); pemilik
// memutuskan membiarkannya (hanya login kunci / login macOS, dan berguna lewat Tailscale).
var DefaultAllow = []string{"nginx:80", "nginx:443", "?:22", "?:5900"}

// Port yang menyimpan data: terbuka ke LAN dianggap parah, bukan sekadar peringatan.
var sensitive = map[int]bool{3306: true, 3307: true, 5432: true, 6379: true, 11211: true, 27017: true, 9100: true, 9101: true, 19999: true}

var entryRE = regexp.MustCompile(`^([a-z0-9._-]{1,40}|\?):([0-9]{1,5})$`)

type Listener struct {
	Proc         string `json:"proc"`
	PID          *int   `json:"pid"`
	Port         int    `json:"port"`
	Bind         string `json:"bind"`
	Scope        string `json:"scope"` // loopback | tailnet | all | lan
	System       bool   `json:"system"`
	LanReachable *bool  `json:"lan_reachable"`
	Service      string `json:"service"`
	Hidden       bool   `json:"hidden"`
	// PFGuarded: ada aturan pf (berkas anchor milik kita) yang memblokir port ini. Uji sambung dari mesin sendiri lewat lo0 dan
	// tidak bisa membuktikan blokir itu aktif, jadi bukan temuan; status "guarded".
	PFGuarded bool `json:"pf_guarded"`
}

type Snapshot struct {
	At        int64      `json:"at"`
	LanIP     *string    `json:"lan_ip"`
	Listeners []Listener `json:"listeners"`
	Errors    []string   `json:"errors"`
}

var scopes = map[string]bool{"loopback": true, "tailnet": true, "all": true, "lan": true}

// Sanitize membuang nilai mustahil dan membatasi teks dari proses luar.
func (s *Snapshot) Sanitize() {
	if s.LanIP != nil {
		v := store.Clean(*s.LanIP, 45)
		s.LanIP = &v
	}
	out := make([]Listener, 0, len(s.Listeners))
	for _, l := range s.Listeners {
		if l.Port < 1 || l.Port > 65535 {
			continue
		}
		l.Proc = store.Clean(l.Proc, 60)
		if l.Proc == "" {
			l.Proc = "?"
		}
		l.Bind = store.Clean(l.Bind, 60)
		l.Service = store.Clean(l.Service, 30)
		if !scopes[l.Scope] {
			l.Scope = "lan" // tidak dikenal: perlakukan hati-hati, bukan loopback
		}
		out = append(out, l)
		if len(out) >= maxListeners {
			break
		}
	}
	s.Listeners = out
	if s.Errors == nil {
		s.Errors = []string{}
	}
	for i := range s.Errors {
		s.Errors[i] = store.Clean(s.Errors[i], 150)
	}
}

// Key: "proses:port" ("?" untuk yang prosesnya tidak terlihat). Huruf kecil agar cocok dengan daftar izin.
func Key(l Listener) string {
	p := strings.ToLower(l.Proc)
	if l.Hidden || p == "" {
		p = "?"
	}
	return p + ":" + strconv.Itoa(l.Port)
}

// Exposed: mendengar di luar loopback/tailnet, bukan milik sistem Apple, dan sambungan dari LAN terbukti berhasil.
func Exposed(l Listener) bool {
	return (l.Scope == "all" || l.Scope == "lan") && !l.System && !l.PFGuarded && l.LanReachable != nil && *l.LanReachable
}

// Status satu pendengar: loopback | tailnet | system | guarded | blocked | unknown | allowed | flagged.
func Status(l Listener, allow map[string]bool) string {
	switch {
	case l.Scope == "loopback":
		return "loopback"
	case l.Scope == "tailnet":
		return "tailnet"
	case l.System:
		return "system"
	case l.PFGuarded:
		return "guarded"
	case l.LanReachable == nil:
		return "unknown"
	case !*l.LanReachable:
		return "blocked"
	case allow[Key(l)]:
		return "allowed"
	}
	return "flagged"
}

// ValidEntry memeriksa "proses:port".
func ValidEntry(s string) bool {
	m := entryRE.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	n, _ := strconv.Atoi(m[2])
	return n >= 1 && n <= 65535
}

// Allowed membaca daftar izin tersimpan. Isi rusak atau kosong-tidak-sah jatuh ke bawaan; daftar yang SENGAJA dikosongkan
// pemilik ("[]") dihormati, jadi bawaan tidak muncul kembali tanpa diminta.
func Allowed(a *core.App) []string {
	raw := a.Store.Setting(allowKey, "")
	if raw == "" {
		return append([]string{}, DefaultAllow...)
	}
	var l []string
	if json.Unmarshal([]byte(raw), &l) != nil {
		return append([]string{}, DefaultAllow...)
	}
	out := []string{}
	seen := map[string]bool{}
	for _, e := range l {
		if ValidEntry(e) && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
}

func allowSet(l []string) map[string]bool {
	m := map[string]bool{}
	for _, e := range l {
		m[e] = true
	}
	return m
}

type state struct {
	mu     sync.Mutex
	snap   *Snapshot
	counts map[string]int // sampel berturut-turut tempat key terlihat Exposed (sebelum daftar izin)
	fetch  func() (Snapshot, error)
}

func st(a *core.App) *state { return core.Local[state](a, "exposure") }

// SetFetch mengganti sumber data (tes).
func SetFetch(a *core.App, f func() (Snapshot, error)) {
	x := st(a)
	x.mu.Lock()
	x.fetch = f
	x.mu.Unlock()
}

func fetchLab(a *core.App) (Snapshot, error) {
	var s Snapshot
	res := a.RunLabTimeout([]string{"sys", "exposure"}, "", 40*time.Second)
	if res.Code != 0 || res.TimedOut {
		return s, errors.New("lab sys exposure gagal")
	}
	if err := json.Unmarshal([]byte(res.Out), &s); err != nil {
		return s, errors.New("keluaran lab sys exposure bukan JSON")
	}
	return s, nil
}

// Sample mengambil satu sampel. Gagal mengambil TIDAK mengubah hitungan: tidak mampu mengukur bukan berarti sudah aman.
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
	cur := map[string]bool{}
	for _, l := range s.Listeners {
		if Exposed(l) {
			cur[Key(l)] = true
		}
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.snap = &s
	next := map[string]int{}
	for k := range cur {
		next[k] = x.counts[k] + 1
	}
	x.counts = next
}

func Run(ctx context.Context, a *core.App) {
	Sample(a, a.Now())
	tk := time.NewTicker(SampleEvery)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tk.C:
			Sample(a, now)
		}
	}
}

// Finding: peringatan turunan (overview mengubahnya menjadi Alert).
type Finding struct{ ID, Level, Title, Detail string }

type Inputs struct {
	Snap   *Snapshot
	Stale  bool
	Counts map[string]int
	Allow  []string
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
	counts := map[string]int{}
	for k, v := range x.counts {
		counts[k] = v
	}
	x.mu.Unlock()
	in := Inputs{Snap: snap, Counts: counts, Allow: Allowed(a), Now: now}
	in.Stale = snap != nil && now.Unix()-snap.At > int64(StaleAfter.Seconds())
	return in
}

// Findings murni (tanpa I/O).
func Findings(in Inputs) []Finding {
	out := []Finding{}
	if in.Snap == nil {
		return out
	}
	if in.Stale {
		return append(out, Finding{"exposure-stale", "warn", "Pemeriksaan paparan layanan berhenti", fmt.Sprintf("sampel terakhir %d menit lalu", (in.Now.Unix()-in.Snap.At)/60)})
	}
	allow := allowSet(in.Allow)
	seen := map[string]bool{}
	for _, l := range in.Snap.Listeners {
		k := Key(l)
		if seen[k] || !Exposed(l) || allow[k] || in.Counts[k] < ConfirmSamples {
			continue
		}
		seen[k] = true
		level := "warn"
		if sensitive[l.Port] {
			level = "bad"
		}
		name := l.Proc
		if l.Hidden {
			name = "Layanan tidak dikenal"
			if l.Service != "" {
				name += " (" + l.Service + ")"
			}
		}
		out = append(out, Finding{
			ID:     "exposure-" + strings.NewReplacer(":", "-", "?", "x").Replace(k),
			Level:  level,
			Title:  fmt.Sprintf("%s terbuka ke jaringan rumah (port %d)", name, l.Port),
			Detail: "Mendengar di semua antarmuka dan sambungan dari LAN berhasil. Layanan lokal sebaiknya hanya di 127.0.0.1 atau tailnet. Bila memang disengaja, tandai sebagai diizinkan di Pantau.",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type Item struct {
	Listener
	Key    string `json:"key"`
	Status string `json:"status"`
}

func view(a *core.App, ss *store.Session) map[string]any {
	in := InputsNow(a)
	allow := allowSet(in.Allow)
	items := []Item{}
	if in.Snap != nil {
		for _, l := range in.Snap.Listeners {
			items = append(items, Item{Listener: l, Key: Key(l), Status: Status(l, allow)})
		}
	}
	fs := Findings(in)
	return map[string]any{
		"now": in.Snap, "stale": in.Stale, "items": items, "allowed": in.Allow, "defaults": DefaultAllow,
		"findings": len(fs), "confirm_samples": ConfirmSamples, "interval": int(SampleEvery.Seconds()),
		"can_edit": ss.Role == "admin" && ss.Via != "token",
	}
}

// Handler: GET /api/exposure
func Handler(a *core.App, w http.ResponseWriter, _ *http.Request, ss *store.Session) {
	core.WriteJSON(w, 200, view(a, ss))
}

func save(a *core.App, list []string) {
	b, _ := json.Marshal(list)
	a.Store.SetSetting(allowKey, string(b))
}

// Change: POST /api/exposure/allow dan /api/exposure/revoke. Hanya admin; mengubah daftar izin memengaruhi peringatan saja.
func Change(add bool) func(*core.App, http.ResponseWriter, *http.Request, *store.Session) {
	return func(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
		if !core.AdminOnly(w, ss) {
			return
		}
		var in struct {
			Entry string `json:"entry"`
		}
		if !core.DecodeJSON(w, r, &in) {
			return
		}
		e := strings.ToLower(strings.TrimSpace(in.Entry))
		if !ValidEntry(e) {
			core.WriteError(w, 400, "entri harus berbentuk proses:port (mis. mailpit:8025), port 1-65535")
			return
		}
		cur := Allowed(a)
		has := false
		for _, x := range cur {
			if x == e {
				has = true
			}
		}
		act := "exposure_revoke"
		switch {
		case add && has, !add && !has:
			// idempoten: tidak ada yang berubah
		case add:
			if len(cur) >= MaxAllow {
				core.WriteError(w, 400, fmt.Sprintf("daftar izin penuh (maks %d)", MaxAllow))
				return
			}
			act = "exposure_allow"
			save(a, append(cur, e))
			a.Store.Audit(ss.User, act, e, a.ClientIP(r), a.Now())
		default:
			next := []string{}
			for _, x := range cur {
				if x != e {
					next = append(next, x)
				}
			}
			save(a, next)
			a.Store.Audit(ss.User, act, e, a.ClientIP(r), a.Now())
		}
		core.WriteJSON(w, 200, view(a, ss))
	}
}
