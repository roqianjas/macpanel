package overview

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/store"
)

// Notifikasi ponsel lewat ntfy (https://ntfy.sh atau server sendiri). Pesan keluar dari mesin ini, jadi:
//   - topik adalah rahasia: disimpan terenkripsi (kunci Keychain panel), tidak pernah dikirim balik lewat API atau ditulis ke audit/log;
//   - server harus https, tanpa kredensial di URL, dan alamat internal (loopback, privat, link-local) ditolak saat koneksi dibuat
//     (bukan hanya saat input, supaya DNS rebinding tidak melewatinya);
//   - hanya admin lewat sesi manusia dengan step-up; token API tidak pernah boleh menyentuhnya;
//   - isi pesan hanya judul dan detail peringatan, tanpa path, IP, atau rahasia.

const (
	kEnabled = "notify_enabled"
	kServer  = "notify_server"
	kMin     = "notify_min"
	kTopic   = "notify_topic_enc"
	kState   = "notify_state"

	DefaultServer = "https://ntfy.sh"
	debounceTicks = 2             // peringatan harus terlihat 2 tick berturut-turut (sekitar 2 menit)
	remindEvery   = 6 * time.Hour // peringatan penting yang masih aktif diingatkan ulang
	maxPerHour    = 12            // batas keras pesan per jam; sisanya ditahan, tidak dibuang
	tick          = 60 * time.Second
	sendTimeout   = 6 * time.Second
)

var topicRE = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

type activeAlert struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Seen   int    `json:"seen"`
	SentAt int64  `json:"sent_at"`          // 0 = belum terkirim
	Silent bool   `json:"silent,omitempty"` // dicatat saat penyalaan pertama tanpa pesan; pulih tanpa pesan juga
}

type lastResult struct {
	At    int64  `json:"at"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type notifyState struct {
	mu         sync.Mutex
	client     *http.Client
	allowLocal bool // hanya tes: izinkan server loopback http
	active     map[string]*activeAlert
	loaded     bool
	sent       []time.Time
	last       lastResult
}

func ns(a *core.App) *notifyState { return core.Local[notifyState](a, "notify") }

// SetNotifyHTTP mengganti klien HTTP dan melonggarkan validasi alamat (hanya untuk tes).
func SetNotifyHTTP(a *core.App, c *http.Client, allowLocal bool) {
	x := ns(a)
	x.mu.Lock()
	x.client, x.allowLocal = c, allowLocal
	x.mu.Unlock()
}

func forbiddenIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// ValidServer memeriksa URL server ntfy. Pengecekan alamat sebenarnya dilakukan lagi saat koneksi dibuat.
func ValidServer(raw string, allowLocal bool) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("alamat server tidak valid")
	}
	if u.User != nil {
		return "", errors.New("alamat server tidak boleh memuat nama pengguna atau kata sandi")
	}
	if u.Scheme != "https" && !(allowLocal && u.Scheme == "http") {
		return "", errors.New("alamat server harus diawali https://")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("alamat server tidak boleh memuat query atau fragment")
	}
	if !allowLocal {
		if ip := net.ParseIP(u.Hostname()); ip != nil && forbiddenIP(ip) {
			return "", errors.New("alamat server tidak boleh alamat internal")
		}
		if h := strings.ToLower(u.Hostname()); h == "localhost" || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".lab") || strings.HasSuffix(h, ".internal") {
			return "", errors.New("alamat server tidak boleh nama internal")
		}
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/"), nil
}

func guardedClient() *http.Client {
	d := &net.Dialer{Timeout: 4 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if ip := net.ParseIP(host); ip == nil || forbiddenIP(ip) {
			return errors.New("alamat tujuan ditolak (internal)")
		}
		return nil
	}}
	return &http.Client{
		Timeout:       sendTimeout,
		Transport:     &http.Transport{DialContext: d.DialContext, Proxy: nil, MaxIdleConns: 1, TLSHandshakeTimeout: 4 * time.Second},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type config struct {
	Enabled bool
	Server  string
	Min     string
	Topic   string
}

func loadConfig(a *core.App) config {
	c := config{Enabled: a.Store.Setting(kEnabled, "") == "1", Server: a.Store.Setting(kServer, DefaultServer), Min: a.Store.Setting(kMin, "bad")}
	if enc := a.Store.Setting(kTopic, ""); enc != "" {
		if b, err := hex.DecodeString(enc); err == nil {
			if p, err := crypto.Open(a.Key, b); err == nil {
				c.Topic = string(p)
			}
		}
	}
	return c
}

func scrub(msg, topic string) string {
	if topic != "" {
		msg = strings.ReplaceAll(msg, topic, "***")
	}
	return store.Clean(msg, 160)
}

func levelOK(min, lv string) bool { return lv == "bad" || (min == "warn" && lv == "warn") }

// send mengirim satu pesan. Galat tidak pernah memuat topik. Pemanggil yang memegang x.mu meneruskan klien
// dari x (mutex tidak reentrant: membaca x di sini akan deadlock).
func send(cl *http.Client, allow bool, c config, title, body, level string) error {
	if cl == nil {
		cl = guardedClient()
	}
	srv, err := ValidServer(c.Server, allow)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", srv+"/"+url.PathEscape(c.Topic), strings.NewReader(store.Clean(body, 400)))
	if err != nil {
		return errors.New("permintaan tidak valid")
	}
	req.Header.Set("Title", store.Clean(title, 120))
	prio := "default"
	if level == "bad" {
		prio = "high"
	}
	req.Header.Set("Priority", prio)
	req.Header.Set("Tags", map[string]string{"bad": "rotating_light", "warn": "warning", "ok": "white_check_mark", "test": "bell"}[level])
	req.Header.Set("User-Agent", "macpanel")
	resp, err := cl.Do(req)
	if err != nil {
		return errors.New(scrub("gagal terhubung ke server notifikasi: "+err.Error(), c.Topic))
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("server notifikasi menjawab HTTP %d", resp.StatusCode)
	}
	return nil
}

func (x *notifyState) record(now time.Time, err error, topic string) {
	x.last = lastResult{At: now.Unix(), OK: err == nil}
	if err != nil {
		x.last.Error = scrub(err.Error(), topic)
	}
}

func (x *notifyState) rateOK(now time.Time) bool {
	cut := now.Add(-time.Hour)
	k := x.sent[:0]
	for _, t := range x.sent {
		if t.After(cut) {
			k = append(k, t)
		}
	}
	x.sent = k
	return len(x.sent) < maxPerHour
}

func saveState(a *core.App, x *notifyState) {
	b, err := json.Marshal(x.active)
	if err == nil {
		a.Store.SetSetting(kState, string(b))
	}
}

// NotifyTick mengevaluasi peringatan dan mengirim yang perlu. Dipanggil tiap menit oleh RunNotify.
func NotifyTick(a *core.App, now time.Time) {
	c := loadConfig(a)
	x := ns(a)
	x.mu.Lock()
	defer x.mu.Unlock()
	if !x.loaded {
		x.loaded = true
		x.active = map[string]*activeAlert{}
		if raw := a.Store.Setting(kState, ""); raw != "" {
			_ = json.Unmarshal([]byte(raw), &x.active)
		}
		if x.active == nil {
			x.active = map[string]*activeAlert{}
		}
	}
	if !c.Enabled || c.Topic == "" {
		return
	}
	list, _ := CurrentAlerts(a)
	cur := map[string]Alert{}
	for _, al := range list {
		if levelOK(c.Min, al.Level) {
			cur[al.ID] = al
		}
	}
	// Penyalaan pertama: peringatan yang sudah ada saat itu dicatat tanpa dikirim (hindari banjir pesan lama).
	if a.Store.Setting(kState, "") == "" && len(x.active) == 0 {
		for id, al := range cur {
			x.active[id] = &activeAlert{Level: al.Level, Title: al.Title, Seen: debounceTicks, SentAt: now.Unix(), Silent: true}
		}
		saveState(a, x)
		return
	}
	changed := false
	for id, al := range cur {
		st := x.active[id]
		if st == nil {
			st = &activeAlert{}
			x.active[id] = st
			changed = true
		}
		st.Seen++
		escalated := st.SentAt != 0 && st.Level == "warn" && al.Level == "bad"
		st.Level, st.Title = al.Level, al.Title
		due := st.SentAt == 0 && st.Seen >= debounceTicks
		remind := st.SentAt != 0 && al.Level == "bad" && now.Unix()-st.SentAt >= int64(remindEvery.Seconds())
		if !(due || escalated || remind) {
			continue
		}
		if !x.rateOK(now) {
			continue // ditahan: dicoba lagi di tick berikutnya
		}
		err := send(x.client, x.allowLocal, c, "MacPanel: "+al.Title, al.Detail, al.Level)
		x.record(now, err, c.Topic)
		x.sent = append(x.sent, now)
		if err == nil {
			st.SentAt, st.Silent = now.Unix(), false
			changed = true
		}
	}
	for id, st := range x.active {
		if _, still := cur[id]; still {
			continue
		}
		if st.SentAt != 0 && !st.Silent && x.rateOK(now) {
			err := send(x.client, x.allowLocal, c, "MacPanel pulih: "+st.Title, "Peringatan ini sudah tidak aktif.", "ok")
			x.record(now, err, c.Topic)
			x.sent = append(x.sent, now)
			if err != nil {
				continue // coba lagi tick berikutnya; tetap tercatat aktif
			}
		} else if st.SentAt != 0 && !st.Silent {
			continue // pesan "pulih" ditahan batas laju: tetap aktif, dicoba lagi
		}
		delete(x.active, id)
		changed = true
	}
	if changed {
		saveState(a, x)
	}
}

func RunNotify(ctx context.Context, a *core.App) {
	tk := time.NewTicker(tick)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tk.C:
			NotifyTick(a, now)
		}
	}
}

// notifyCode: kunci terjemahan untuk NotifyHint ("panel" = tanpa pesan keluar, "ntfy:<host>" = juga ke ponsel).
func notifyCode(a *core.App) string {
	if c := loadConfig(a); c.Enabled && c.Topic != "" {
		return "ntfy:" + hostOf(c.Server)
	}
	return "panel"
}

// NotifyHint: keterangan di kartu Peringatan, harus jujur soal apakah ada pesan keluar.
func NotifyHint(a *core.App) string {
	if c := loadConfig(a); c.Enabled && c.Topic != "" {
		return "juga dikirim ke ponsel lewat ntfy (" + hostOf(c.Server) + ")"
	}
	return "hanya ditampilkan di panel; tidak ada pesan keluar"
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return "server ntfy"
}

// ---------- API (admin, sesi manusia, step-up untuk mengubah) ----------

func NotifyGet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.AdminOnly(w, ss) {
		return
	}
	c := loadConfig(a)
	x := ns(a)
	x.mu.Lock()
	last := x.last
	x.mu.Unlock()
	core.WriteJSON(w, 200, map[string]any{"enabled": c.Enabled, "server": c.Server, "min": c.Min, "topic_set": c.Topic != "", "last": last, "default_server": DefaultServer})
}

func NotifySet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.AdminOnly(w, ss) {
		return
	}
	var in struct {
		Enabled    *bool
		Server     *string
		Min        *string
		Topic      *string
		ClearTopic bool `json:"clear_topic"`
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	x := ns(a)
	x.mu.Lock()
	allow := x.allowLocal
	x.mu.Unlock()
	if in.Server != nil {
		v, err := ValidServer(*in.Server, allow)
		if err != nil {
			core.WriteError(w, 400, err.Error())
			return
		}
		in.Server = &v
	}
	if in.Min != nil && *in.Min != "bad" && *in.Min != "warn" {
		core.WriteError(w, 400, "tingkat: bad atau warn")
		return
	}
	if in.Topic != nil && !topicRE.MatchString(*in.Topic) {
		core.WriteError(w, 400, "topik: 16-64 karakter huruf, angka, _ atau - (pilih yang sulit ditebak; topik berfungsi seperti kata sandi)")
		return
	}
	if in.Enabled != nil && *in.Enabled {
		has := loadConfig(a).Topic != "" || (in.Topic != nil && !in.ClearTopic)
		if !has {
			core.WriteError(w, 400, "isi topik dulu sebelum menyalakan notifikasi")
			return
		}
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	var ch []string
	if in.Server != nil {
		a.Store.SetSetting(kServer, *in.Server)
		ch = append(ch, "server")
	}
	if in.Min != nil {
		a.Store.SetSetting(kMin, *in.Min)
		ch = append(ch, "tingkat="+*in.Min)
	}
	if in.ClearTopic {
		a.Store.SetSetting(kTopic, "")
		a.Store.SetSetting(kEnabled, "")
		ch = append(ch, "topik dihapus", "nonaktif")
	} else if in.Topic != nil {
		enc, err := crypto.Seal(a.Key, []byte(*in.Topic))
		if err != nil {
			core.WriteError(w, 500, "gagal menyimpan topik")
			return
		}
		a.Store.SetSetting(kTopic, hex.EncodeToString(enc))
		ch = append(ch, "topik diganti")
	}
	if in.Enabled != nil && !in.ClearTopic {
		v := ""
		if *in.Enabled {
			v = "1"
		}
		a.Store.SetSetting(kEnabled, v)
		ch = append(ch, map[bool]string{true: "aktif", false: "nonaktif"}[*in.Enabled])
	}
	if len(ch) == 0 {
		core.WriteError(w, 400, "tidak ada yang diubah")
		return
	}
	a.Store.Audit(ss.User, "notify_config", strings.Join(ch, ", "), a.ClientIP(r), a.Now())
	NotifyGet(a, w, r, ss)
}

func NotifyTest(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	if !core.AdminOnly(w, ss) {
		return
	}
	c := loadConfig(a)
	if c.Topic == "" {
		core.WriteError(w, 400, "isi topik dulu")
		return
	}
	x := ns(a)
	x.mu.Lock()
	defer x.mu.Unlock()
	if !x.rateOK(a.Now()) {
		core.WriteError(w, 429, "terlalu banyak pesan dalam satu jam")
		return
	}
	err := send(x.client, x.allowLocal, c, "MacPanel: uji notifikasi", "Jika pesan ini sampai di ponsel, notifikasi berfungsi.", "test")
	x.record(a.Now(), err, c.Topic)
	x.sent = append(x.sent, a.Now())
	a.Store.Audit(ss.User, "notify_test", map[bool]string{true: "ok", false: "gagal"}[err == nil], a.ClientIP(r), a.Now())
	if err != nil {
		core.WriteJSON(w, 200, map[string]any{"ok": false, "error": scrub(err.Error(), c.Topic)})
		return
	}
	core.WriteJSON(w, 200, map[string]any{"ok": true})
}
