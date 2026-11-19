package core

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/roqianjas/macpanel/internal/store"
)

type Level int

const (
	Public Level = iota // tanpa sesi
	Pwd                 // sesi tahap password (menunggu TOTP)
	Full                // sesi penuh (password + TOTP)
	Any                 // Pwd atau Full
)

type Config struct {
	Origins []string // origin yang diizinkan, mis. https://panel.lab
}

type App struct {
	Store       *store.Store
	Key         []byte
	Cfg         Config
	Dummy       string
	Now         func() time.Time
	LabBin      string // biner lab (path tetap)
	Home        string
	Root        string // MACPANEL_HOME (bawaan ~/MacPanel)
	Wwwroot     string // batas folder situs: <root>/wwwroot // HOME user; batas ~/MacPanel/wwwroot dihitung dari sini
	SitesDir    string // folder konfigurasi situs nginx (hanya dibaca panel)
	ApacheDir   string // folder vhost Apache (hanya dibaca panel; ditulis oleh `lab apache`)
	TLD         string
	NginxDir    string // folder config nginx (snippets)
	CertDir     string
	LogDir      string // log akses per situs
	ErrLog      string
	Timeout     time.Duration  // batas waktu tiap aksi cepat
	LongTimeout time.Duration  // batas aksi lama (install, update)
	BG          sync.WaitGroup // aksi latar belakang yang masih berjalan
	Mu          sync.Mutex     // satu aksi mutasi pada satu waktu
	UIDir       string         // folder hasil build UI (kosong = tanpa UI)
	ProbeAddr   string         // loopback:443 untuk pemeriksaan situs (kosong = tanpa pemeriksaan)
	ext         sync.Map       // state milik tiap domain (lihat local)
}

type Route struct {
	Method, Path string
	Level        Level
	H            func(*App, http.ResponseWriter, *http.Request, *store.Session)
}

const (
	LockWindow  = 15 * time.Minute
	MaxPerUser  = 5
	MaxPerIP    = 10
	CookieName  = "__Host-panel"
	MaxBodySize = 8 << 10
)

// EnvOr membaca env opsional (dipakai untuk instance demo/uji); nilai bawaan dipakai bila kosong.
func EnvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
