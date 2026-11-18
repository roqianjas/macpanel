package server_test

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/hardware"
	"github.com/roqianjas/macpanel/internal/overview"
	"github.com/roqianjas/macpanel/internal/testenv"
)

const topic = "TopikRahasiaPanjang12345"

type ntfyMsg struct {
	Path, Title, Priority, Tags, Body string
}

type fakeNtfy struct {
	mu   sync.Mutex
	msgs []ntfyMsg
	code int
	srv  *httptest.Server
}

func newFakeNtfy(t *testing.T) *fakeNtfy {
	f := &fakeNtfy{code: 200}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.msgs = append(f.msgs, ntfyMsg{r.URL.Path, r.Header.Get("Title"), r.Header.Get("Priority"), r.Header.Get("Tags"), string(b)})
		code := f.code
		f.mu.Unlock()
		w.WriteHeader(code)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeNtfy) n() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.msgs) }
func (f *fakeNtfy) last() ntfyMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.msgs) == 0 {
		return ntfyMsg{}
	}
	return f.msgs[len(f.msgs)-1]
}
func (f *fakeNtfy) setCode(c int) { f.mu.Lock(); f.code = c; f.mu.Unlock() }

type nenv struct {
	*testing.T
	f *fakeNtfy
}

// setup membuat lingkungan: ntfy palsu, notifikasi aktif (min=bad), dan suhu CPU yang bisa diatur.
func setupNotify(t *testing.T, min string) (*nenv, func(cpu float64) time.Time, func(string, string) (int, string), func() map[string]any, interface{}) {
	t.Helper()
	o := testenv.NewOpsEnv(t, "")
	f := newFakeNtfy(t)
	overview.SetNotifyHTTP(o.App, f.srv.Client(), true)
	temp := 50.0
	hardware.SetFetch(o.App, func() (hardware.Snapshot, error) { return goodSnap(temp), nil }, nil)
	o.Stepup()
	body := `{"server":"` + f.srv.URL + `","topic":"` + topic + `","min":"` + min + `","enabled":true}`
	if code, b := o.Post("/api/notify", body); code != 200 {
		t.Fatalf("konfigurasi: %d %s", code, b)
	}
	tick := func(cpu float64) time.Time {
		temp = cpu
		o.Now = o.Now.Add(61 * time.Second)
		hardware.Sample(o.App, o.Now)
		overview.NotifyTick(o.App, o.Now)
		return o.Now
	}
	get := func() map[string]any {
		var m map[string]any
		_ = json.Unmarshal(o.Do("GET", "/api/notify", "", o.Cookie, "", nil).Body.Bytes(), &m)
		return m
	}
	return &nenv{T: t, f: f}, tick, o.Post, get, o
}

func TestNotifyFirstRunBaselineIsSilentThenSendsNewAlert(t *testing.T) {
	n, tick, _, _, _ := setupNotify(t, "bad")
	tick(97) // alert sudah ada saat notifikasi pertama kali berjalan: dicatat tanpa kirim
	if n.f.n() != 0 {
		t.Fatalf("penyalaan pertama tidak boleh membanjiri pesan lama: %d pesan", n.f.n())
	}
	tick(50) // pulih tanpa pernah dikirim: tidak ada pesan "pulih" yatim
	if n.f.n() != 0 {
		t.Fatalf("pulih dari alert yang tidak pernah dikirim tidak boleh mengirim: %d", n.f.n())
	}
	tick(97) // alert baru, tick pertama: debounce
	if n.f.n() != 0 {
		t.Fatal("alert baru harus melewati debounce 2 tick")
	}
	tick(97)
	if n.f.n() != 1 {
		t.Fatalf("setelah 2 tick harus terkirim satu pesan, dapat %d", n.f.n())
	}
	m := n.f.last()
	if m.Path != "/"+topic || m.Priority != "high" || !strings.Contains(m.Title, "Suhu CPU tinggi") || m.Body != "97 °C" || m.Tags != "rotating_light" {
		t.Fatalf("%+v", m)
	}
	tick(97)
	tick(97)
	if n.f.n() != 1 {
		t.Fatalf("alert yang sama tidak boleh dikirim ulang: %d", n.f.n())
	}
	tick(50)
	if n.f.n() != 2 || !strings.HasPrefix(n.f.last().Title, "MacPanel pulih:") || n.f.last().Tags != "white_check_mark" {
		t.Fatalf("harus ada pesan pulih: %d %+v", n.f.n(), n.f.last())
	}
	tick(50)
	tick(50)
	if n.f.n() != 2 {
		t.Fatal("setelah pulih tidak boleh ada pesan lagi")
	}
}

func TestNotifyFlappingBelowDebounceSendsNothing(t *testing.T) {
	n, tick, _, _, _ := setupNotify(t, "bad")
	tick(50)
	for i := 0; i < 6; i++ { // naik-turun tiap tick: tidak pernah 2 berturut-turut
		tick(97)
		tick(50)
	}
	if n.f.n() != 0 {
		t.Fatalf("alert yang berkedip tiap menit tidak boleh mengirim: %d", n.f.n())
	}
}

func TestNotifyMinimumLevel(t *testing.T) {
	n, tick, _, _, _ := setupNotify(t, "bad")
	tick(50)
	tick(88) // warn (85..95)
	tick(88)
	tick(88)
	if n.f.n() != 0 {
		t.Fatalf("min=bad tidak boleh mengirim peringatan 'warn': %d", n.f.n())
	}
	n2, tick2, _, _, _ := setupNotify(t, "warn")
	tick2(50)
	tick2(88)
	tick2(88)
	if n2.f.n() != 1 || n2.f.last().Priority != "default" || n2.f.last().Tags != "warning" {
		t.Fatalf("min=warn harus mengirim 'warn' berprioritas biasa: %d %+v", n2.f.n(), n2.f.last())
	}
}

func TestNotifyEscalationAndReminder(t *testing.T) {
	n, tick, _, _, _ := setupNotify(t, "warn")
	tick(50)
	tick(88)
	tick(88) // warn terkirim
	if n.f.n() != 1 {
		t.Fatalf("warn awal: %d", n.f.n())
	}
	tick(97) // naik menjadi bad: kirim lagi
	if n.f.n() != 2 || n.f.last().Priority != "high" {
		t.Fatalf("eskalasi harus mengirim ulang berprioritas tinggi: %d %+v", n.f.n(), n.f.last())
	}
	for i := 0; i < 5; i++ { // 5 menit lagi: belum waktunya mengingatkan
		tick(97)
	}
	if n.f.n() != 2 {
		t.Fatalf("belum 6 jam, tidak boleh mengingatkan: %d", n.f.n())
	}
}

func TestNotifyDeliveryFailureIsRetriedAndNeverLeaksTopic(t *testing.T) {
	n, tick, _, get, _ := setupNotify(t, "bad")
	tick(50)
	n.f.setCode(500)
	tick(97)
	tick(97) // gagal terkirim
	last, _ := get()["last"].(map[string]any)
	if last["ok"] != false || !strings.Contains(last["error"].(string), "HTTP 500") {
		t.Fatalf("kegagalan harus tercatat: %v", last)
	}
	n.f.setCode(200)
	before := n.f.n()
	tick(97) // dicoba lagi
	if n.f.n() != before+1 {
		t.Fatalf("pengiriman yang gagal harus dicoba lagi tick berikutnya (%d -> %d)", before, n.f.n())
	}
	if l, _ := get()["last"].(map[string]any); l["ok"] != true {
		t.Fatalf("setelah berhasil, status terakhir harus ok: %v", l)
	}
}

func TestNotifyErrorMessageNeverContainsTopic(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	f := newFakeNtfy(t)
	overview.SetNotifyHTTP(o.App, f.srv.Client(), true)
	o.Stepup()
	o.Post("/api/notify", `{"server":"`+f.srv.URL+`","topic":"`+topic+`"}`)
	f.srv.Close() // server mati: galat jaringan memuat URL lengkap termasuk topik bila tidak dibersihkan
	code, body := o.Post("/api/notify/test", `{}`)
	if code != 200 || strings.Contains(body, topic) {
		t.Fatalf("galat tidak boleh memuat topik: %d %s", code, body)
	}
	var r struct {
		OK    bool
		Error string
	}
	_ = json.Unmarshal([]byte(body), &r)
	if r.OK || r.Error == "" {
		t.Fatalf("harus gagal dengan pesan: %s", body)
	}
}

func TestNotifyRateLimitHoldsInsteadOfDropping(t *testing.T) {
	n, tick, _, _, o := setupNotify(t, "bad")
	_ = o
	tick(50)
	for i := 0; i < 15; i++ { // 45 tick = 45 menit (kurang dari satu jam); tanpa batas ini akan terkirim 30 pesan
		tick(97)
		tick(97)
		tick(50)
	}
	if got := n.f.n(); got > 12 {
		t.Fatalf("batas 12 pesan per jam dilanggar: %d", got)
	}
	if n.f.n() == 0 {
		t.Fatal("seharusnya ada yang terkirim")
	}
}

func TestNotifyDisabledOrClearedSendsNothing(t *testing.T) {
	n, tick, post, get, _ := setupNotify(t, "bad")
	tick(50)
	if code, _ := post("/api/notify", `{"enabled":false}`); code != 200 {
		t.Fatal("matikan gagal")
	}
	tick(97)
	tick(97)
	tick(97)
	if n.f.n() != 0 {
		t.Fatalf("dinonaktifkan tidak boleh mengirim: %d", n.f.n())
	}
	if code, _ := post("/api/notify", `{"enabled":true}`); code != 200 {
		t.Fatal("nyalakan lagi gagal")
	}
	if code, _ := post("/api/notify", `{"clear_topic":true}`); code != 200 {
		t.Fatal("hapus topik gagal")
	}
	g := get()
	if g["topic_set"] != false || g["enabled"] != false {
		t.Fatalf("hapus topik harus mematikan notifikasi: %v", g)
	}
}

func TestNotifyConfigSecurity(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	f := newFakeNtfy(t)
	overview.SetNotifyHTTP(o.App, f.srv.Client(), true)
	body := `{"server":"` + f.srv.URL + `","topic":"` + topic + `","enabled":true}`

	if code, _ := o.Post("/api/notify", body); code != 403 {
		t.Fatalf("tanpa step-up harus 403, dapat %d", code)
	}
	o.Stepup() // membuat token pun butuh step-up; token yang dibuat tetap tidak boleh menyentuh /api/notify
	tok, _ := o.MkToken("full")
	if code, _ := o.Bearer("GET", "/api/notify", "", tok); code != 403 {
		t.Fatalf("token API (bahkan 'full') harus ditolak, dapat %d", code)
	}
	if code, _ := o.Bearer("POST", "/api/notify/test", "{}", tok); code != 403 {
		t.Fatalf("token tidak boleh mengirim uji: %d", code)
	}
	if w := o.Do("GET", "/api/notify", "", "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa login: %d", w.Code)
	}
	o.Stepup()
	if code, b := o.Post("/api/notify", body); code != 200 || strings.Contains(b, topic) {
		t.Fatalf("simpan: %d (respons tidak boleh memuat topik) %s", code, b)
	}
	if b := o.Do("GET", "/api/notify", "", o.Cookie, "", nil).Body.String(); strings.Contains(b, topic) || !strings.Contains(b, `"topic_set":true`) {
		t.Fatalf("GET tidak boleh membocorkan topik: %s", b)
	}
	var stored string
	_ = o.App.Store.DB.QueryRow("SELECT v FROM settings WHERE k='notify_topic_enc'").Scan(&stored)
	raw, err := hex.DecodeString(stored)
	if stored == "" || err != nil || strings.Contains(stored, topic) || strings.Contains(string(raw), topic) || strings.Contains(stored, hex.EncodeToString([]byte(topic))) {
		t.Fatalf("topik harus tersimpan terenkripsi, bukan teks biasa atau hex dari teks biasa: %q", stored)
	}
	if len(raw) < len(topic)+12 {
		t.Fatalf("nilai tersimpan terlalu pendek untuk ciphertext berautentikasi (nonce+tag): %d byte", len(raw))
	}
	for _, a := range o.App.Store.RecentAudit(50) {
		if strings.Contains(a.Detail, topic) || strings.Contains(a.Action, topic) {
			t.Fatalf("topik bocor ke audit: %+v", a)
		}
	}
	found := false
	for _, a := range o.App.Store.RecentAudit(50) {
		if a.Action == "notify_config" {
			found = true
		}
	}
	if !found {
		t.Fatal("perubahan konfigurasi harus tercatat di audit")
	}
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	if code, _ := o.Post("/api/notify", `{"enabled":false}`); code != 403 {
		t.Fatalf("viewer tidak boleh mengubah: %d", code)
	}
	if w := o.Do("GET", "/api/notify", "", o.Cookie, "", nil); w.Code != 403 {
		t.Fatalf("viewer tidak boleh membaca konfigurasi: %d", w.Code)
	}
}

func TestNotifyInputValidation(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	overview.SetNotifyHTTP(o.App, nil, false) // validasi produksi: tanpa pelonggaran
	o.Stepup()
	bad := []string{
		`{"server":"http://ntfy.sh"}`, `{"server":"https://user:pw@ntfy.sh"}`, `{"server":"https://127.0.0.1"}`,
		`{"server":"https://169.254.169.254"}`, `{"server":"https://localhost"}`, `{"server":"https://panel.lab"}`,
		`{"topic":"pendek"}`, `{"topic":"../../etc/passwd/aaaaaaaa"}`, `{"topic":"spasi spasi spasi spasi"}`,
		`{"min":"semua"}`, `{"enabled":true}`, `{}`,
	}
	for _, b := range bad {
		if code, _ := o.Post("/api/notify", b); code != 400 {
			t.Errorf("%s harus 400, dapat %d", b, code)
		}
	}
	if code, b := o.Post("/api/notify", `{"server":"https://ntfy.example.com","topic":"`+topic+`","min":"warn","enabled":true}`); code != 200 {
		t.Fatalf("konfigurasi sah ditolak: %d %s", code, b)
	}
}

func TestNotifyTestEndpointSendsAndNeedsTopic(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	f := newFakeNtfy(t)
	overview.SetNotifyHTTP(o.App, f.srv.Client(), true)
	if code, _ := o.Post("/api/notify/test", `{}`); code != 400 {
		t.Fatalf("tanpa topik harus 400: %d", code)
	}
	o.Stepup()
	o.Post("/api/notify", `{"server":"`+f.srv.URL+`","topic":"`+topic+`"}`)
	code, body := o.Post("/api/notify/test", `{}`)
	if code != 200 || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("%d %s", code, body)
	}
	m := f.last()
	if m.Path != "/"+topic || m.Priority != "default" || m.Tags != "bell" || !strings.Contains(m.Title, "uji notifikasi") {
		t.Fatalf("%+v", m)
	}
}

func TestAlertsHintIsHonestAboutOutgoingMessages(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	hint := func() string {
		var r struct{ Notify string }
		_ = json.Unmarshal(o.Do("GET", "/api/alerts", "", o.Cookie, "", nil).Body.Bytes(), &r)
		return r.Notify
	}
	if h := hint(); !strings.Contains(h, "tidak ada pesan keluar") {
		t.Fatalf("sebelum diaktifkan: %q", h)
	}
	f := newFakeNtfy(t)
	overview.SetNotifyHTTP(o.App, f.srv.Client(), true)
	o.Stepup()
	o.Post("/api/notify", `{"server":"`+f.srv.URL+`","topic":"`+topic+`","enabled":true}`)
	if h := hint(); strings.Contains(h, "tidak ada pesan keluar") || !strings.Contains(h, "ponsel") {
		t.Fatalf("setelah diaktifkan keterangan tidak boleh lagi bilang tanpa pesan keluar: %q", h)
	}
}

// Dua pagar independen: daftar rute terlarang token (TokenAllowed) dan AdminOnly/HumanOnly di handler.
// Tes lewat HTTP hanya melihat salah satunya yang menolak, jadi pagar pertama diuji langsung.
func TestTokenAllowedBlocksNotifyRoutes(t *testing.T) {
	for _, c := range [][2]string{{"GET", "/api/notify"}, {"POST", "/api/notify"}, {"POST", "/api/notify/test"}} {
		if core.TokenAllowed(c[0], c[1]) {
			t.Errorf("%s %s tidak boleh bisa dipanggil dengan token API", c[0], c[1])
		}
	}
	if !core.TokenAllowed("GET", "/api/hw") || !core.TokenAllowed("GET", "/api/netmon") {
		t.Error("data pemantauan hanya-baca tetap boleh untuk token")
	}
}
