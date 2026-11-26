package testenv

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/deploy"
	"github.com/roqianjas/macpanel/internal/server"
	"github.com/roqianjas/macpanel/internal/store"
	"github.com/roqianjas/macpanel/internal/system"
)

const fakeApps = `case "$1 $2" in
"service list") printf '[{"id":"nginx","label":"nginx","group":"Web server","kind":"brew","version":"1.31.8","state":"running","port":443,"installed":true,"ctl":true,"optional":false,"desc":"x","has_log":true},{"id":"redis","label":"Redis","group":"Cache","kind":"brew","version":"8","state":"HACKED<script>","installed":true,"ctl":true,"optional":false,"desc":"","has_log":true}]';;
"service logs") echo "log token=RAHASIA123";;
*) echo "ok $1 $2";;
esac`

func NewAppsEnv(t *testing.T) *opsEnv {
	o := NewOpsEnv(t, fakeApps)
	o.App.LongTimeout = 20 * time.Second
	return o
}

const NetstatFixture = `Name       Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll
lo0        16384 <Link#1>                         25768     0    6588285    25768     0    6588285     0
lo0        16384 127           127.0.0.1          25768     -    6588285    25768     -    6588285     -
gif0*      1280  <Link#2>                             0     0          0        0     0          0     0
en0        1500  <Link#7>    02:00:00:00:00:00        0     0          0        0     0          0     0
en1        1500  <Link#16>   02:00:00:00:00:01   115628     0  127070191    55025     0   20695228     0
en1        1500  192.168.1     192.168.1.20      115628     -  127070191    55025     -   20695228     -
utun0      1380  <Link#19>                            0     0          0        4     0        320     0
utun4      1380  <Link#21>                          500     0       9000      700     0       12000     0
utun4      1380  100.64.0.10/ 100.64.0.10       500     -       9000      700     -       12000     -
bridge0    1500  <Link#14>   02:00:00:00:00:00        0     0          0        0     0          0     0
awdl0      1500  <Link#17>   02:00:00:00:00:00       21     0      12303       24     0       3687     0
`

// fakeRun meniru perintah sistem; mencatat nama program dan menolak yang tidak diharapkan.
func FakeRunner(t *testing.T, net *string, cpuFail *bool, calls *[]string) system.Runner {
	return func(_ time.Duration, name string, args ...string) (string, error) {
		*calls = append(*calls, name)
		switch name {
		case "/usr/bin/top":
			if *cpuFail {
				return "", errors.New("gagal")
			}
			return "CPU usage: 1.0% user, 1.0% sys, 90.0% idle\nCPU usage: 5.0% user, 5.0% sys, 90.0% idle\n", nil
		case "/usr/bin/vm_stat":
			return "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages wired down: 100.\nPages purgeable: 50.\nAnonymous pages: 350.\nPages occupied by compressor: 50.\n", nil
		case "/usr/sbin/sysctl":
			if len(args) > 1 && args[1] == "hw.memsize" {
				return "16384000\n", nil
			}
			return "{ 1.00 2.00 3.00 }\n", nil
		case "/usr/sbin/netstat":
			return *net, nil
		}
		t.Fatalf("program tak terduga dipanggil: %s", name)
		return "", nil
	}
}

func NetText(rx, tx int) string {
	r, x := Itoa(int64(rx)), Itoa(int64(tx))
	return "Name Mtu Network Address Ipkts Ierrs Ibytes Opkts Oerrs Obytes Coll\n" +
		"en1 1500 <Link#16> 02:00:00:00:00:01 1 0 " + r + " 1 0 " + x + " 0\n" +
		"en1 1500 192.168.1 192.168.1.20 1 - " + r + " 1 - " + x + " -\n"
}

const FakeRedis = `case "$1 $2" in
"redis info") printf '{"dbsize":2,"used_memory":"1M","maxmemory":"0B","keyspace":{}}';;
"redis scan") printf '{"keys":[{"key":"a","type":"string","ttl":-1}],"more":false}';;
"redis get") printf '{"key":"k","type":"string","ttl":-1,"size":3,"value":"nilai-rahasia","truncated":false}';;
*) echo "OK";;
esac`

func Sign(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func (o *opsEnv) Hook(site, body, sig, event, delivery, ip string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/hook/"+site, strings.NewReader(body))
	r.RemoteAddr = ip + ":4444"
	if sig != "" {
		r.Header.Set("X-Hub-Signature-256", sig)
	}
	if event != "" {
		r.Header.Set("X-GitHub-Event", event)
	}
	if delivery != "" {
		r.Header.Set("X-GitHub-Delivery", delivery)
	}
	w := httptest.NewRecorder()
	deploy.HookHandler(o.App).ServeHTTP(w, r)
	return w
}

func SetupHook(t *testing.T, o *opsEnv) string {
	o.Stepup()
	c, b := o.Post("/api/sites/demo/hook", `{"enabled":true,"branch":"main","steps":["git_pull","composer_install"]}`)
	if c != 200 {
		t.Fatalf("setup: %d %s", c, b)
	}
	var out struct{ Secret string }
	json.Unmarshal([]byte(b), &out)
	if len(out.Secret) != 64 {
		t.Fatalf("secret harus 64 hex: %q", out.Secret)
	}
	return out.Secret
}

func TrustOf(w *httptest.ResponseRecorder) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == store.TrustCookie {
			return c.Value
		}
	}
	return ""
}

const LoginBody = `{"name":"admin","password":"kata-sandi-panjang-123"}`

func (e *env) TrustLogin(trust bool) (cookie, csrf, tok string) {
	w1 := e.Do("POST", "/api/auth/login", LoginBody, "", "", nil)
	c1, cs1 := CookieOf(w1), CsrfOf(w1)
	body := `{"code":"` + e.Code(0) + `","trust":` + map[bool]string{true: "true", false: "false"}[trust] + `}`
	w2 := e.Do("POST", "/api/auth/totp", body, c1, cs1, map[string]string{"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/130.0"})
	if w2.Code != 200 {
		e.t.Fatalf("totp: %d %s", w2.Code, w2.Body)
	}
	return CookieOf(w2), CsrfOf(w2), TrustOf(w2)
}

func (o *opsEnv) Bearer(method, path, body, tok string) (int, string) {
	w := o.Do(method, path, body, "", "", map[string]string{"Authorization": "Bearer " + tok})
	return w.Code, w.Body.String()
}

func (o *opsEnv) MkToken(scope string) (string, int64) {
	o.t.Helper()
	code, body := o.Post("/api/me/tokens", `{"name":"agen `+scope+`","scope":"`+scope+`","days":90}`)
	if code != 200 {
		o.t.Fatalf("buat token %s: %d %s", scope, code, body)
	}
	var r struct {
		Token string
		ID    int64
	}
	json.Unmarshal([]byte(body), &r)
	if !strings.HasPrefix(r.Token, store.TokenPrefix) {
		o.t.Fatalf("format token salah: %q", r.Token)
	}
	return r.Token, r.ID
}

func ItoaTok(n int64) string { b, _ := json.Marshal(n); return string(b) }

const DbFake = `case "$1 $2" in
"db user") printf '{"ok":true,"user":"x","password":"PwAcakDariLab1234567890"}\n';;
"db root-passwd") printf '{"ok":true,"engine":"x"}\n';;
"db passwd") echo "OK: diganti";;
*) echo "ok $1 $2";; esac`

func (o *opsEnv) Login2(name, pw string) (cookie, csrf string, code int) {
	w := o.Do("POST", "/api/auth/login", `{"name":"`+name+`","password":"`+pw+`"}`, "", "", nil)
	return CookieOf(w), CsrfOf(w), w.Code
}

func (o *opsEnv) As(cookie, csrf, method, path, body string) (int, string) {
	w := o.Do(method, path, body, cookie, csrf, nil)
	return w.Code, w.Body.String()
}

const fakeP3 = `D=$(dirname "$0")
case "$1 $2" in
"php list") printf '8.2  terpasang=tidak versi=-        fpm_socket=-\n8.3  terpasang=ya    versi=8.3.36   fpm_socket=ada\n8.5  terpasang=ya    versi=8.5.12   fpm_socket=ada\n';;
"runtime list") printf 'go      1.27.4                   ~/.config/mise/config.toml  latest\nnode    22.23.5\nnode    26.13.1                  ~/.config/mise/config.toml  lts\npnpm    12.18.0 (pruned in 23h)\n';;
"update check") printf '{"checked":1,"pins":["mysql@8.4","postgresql@17"],"errors":[],"counts":{},"items":[{"kind":"tool","source":"homebrew","name":"mycli","from":"2.30.4","to":"2.31.0","pinned":false,"major":false,"upgradable":true,"target":"mycli","note":""},{"kind":"service","source":"homebrew","name":"postgresql@17","from":"17.1","to":"17.2","pinned":true,"major":false,"upgradable":true,"target":"postgresql@17","note":"pinned"},{"kind":"service","source":"homebrew","name":"mysql@8.4","from":"8.4.1","to":"9.0.0","pinned":true,"major":true,"upgradable":false,"target":"mysql@8.4","note":"major"},{"kind":"runtime","source":"mise","name":"pnpm","from":"12.18.0","to":"12.21.0","pinned":false,"major":false,"upgradable":true,"target":"mise:pnpm","note":""},{"kind":"lib","source":"homebrew","name":"nss","from":"3.133","to":"3.134","pinned":false,"major":false,"upgradable":false,"target":"nss","note":"bulk"},{"kind":"app","source":"npm","name":"9router","from":"0.5.119","to":"0.5.123","pinned":false,"major":false,"upgradable":true,"target":"npm:9router","note":"restart"}]}\n';;
"worker list") printf 'NAMA                    STATUS        PID     EXIT  JADWAL/CMD\nqueue-1                 jalan         79674   -     KeepAlive  /opt/homebrew/bin/php artisan queue:work\nqueue-2                 tidak dimuat  -       -     KeepAlive  /opt/homebrew/bin/php artisan queue:work\n';;
"job list") printf 'NAMA                    STATUS        PID     EXIT  JADWAL/CMD\ntick                    terjadwal     -       0     tiap 60s  /opt/homebrew/bin/php tick.php\nharian                  terjadwal     -       -     harian 03:15  /opt/homebrew/bin/php x.php\n';;
"php install") [ -f "$D/fail" ] && { echo "boom"; exit 2; }; [ -f "$D/slow" ] && sleep 30; printf 'START\n' >> "$D/order.log"; sleep 0.3; printf 'END\n' >> "$D/order.log"; echo "ok php install";;
"worker logs"|"job logs") echo "baris log token=RAHASIA123";;
*) [ -f "$D/slow" ] && sleep 30; [ -f "$D/fail" ] && { echo "boom"; exit 2; }; echo "ok $1 $2";;
esac`

func NewP3Env(t *testing.T) *opsEnv {
	t.Helper()
	o := NewOpsEnv(t, fakeP3)
	o.App.LongTimeout = 20 * time.Second
	return o
}

func (o *opsEnv) WaitJob(id int64) store.JobRow {
	o.t.Helper()
	for i := 0; i < 200; i++ {
		if j := o.App.Store.GetJob(id); j != nil && j.Status != "berjalan" {
			return *j
		}
		time.Sleep(50 * time.Millisecond)
	}
	o.t.Fatalf("job %d tidak selesai", id)
	return store.JobRow{}
}

func JobID(t *testing.T, body string) int64 {
	t.Helper()
	var m struct{ Job int64 }
	if err := json.Unmarshal([]byte(body), &m); err != nil || m.Job == 0 {
		t.Fatalf("tidak ada id job: %s", body)
	}
	return m.Job
}

const FakeDev = `case "$1 $2" in
"ext list") printf '{"version":"8.4","builtin":["curl","mbstring"],"managed":[{"name":"redis","label":"Redis","installed":true,"enabled":true,"loaded":true,"zend":false},{"name":"EVIL<x>","label":"x","installed":false,"enabled":false,"loaded":false,"zend":false}]}';;
"ini get") printf '{"version":"8.4","keys":[{"key":"memory_limit","value":"512M","default":"128M","hint":"h"},{"key":"disable_functions","value":"x","default":"","hint":""}]}';;
"composer status") printf '{"installed":true,"version":"2.11.2","php":"8.5.12","outdated":false,"latest":""}';;
"deploy info") printf '{"site":"tokoku","dir":"/x","php":"8.4","steps":[{"id":"git_pull","label":"Tarik"}],"git":{"branch":"main","commit":"abc msg","dirty":false,"remote":true}}';;
"backup list") printf '{"backups":[{"name":"20261116-180701","at":1,"size":10,"files":["a"]}],"offsite":false}';;
*) echo "ok $1 $2";;
esac`

func NewDevEnv(t *testing.T) *opsEnv {
	o := NewOpsEnv(t, FakeDev)
	o.App.LongTimeout = 20 * time.Second
	return o
}

func (o *opsEnv) Upload(path string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, bytes.NewReader(body))
	r.RemoteAddr = "100.64.0.20:5555"
	r.Header.Set("Cookie", core.CookieName+"="+o.Cookie)
	r.Header.Set("X-CSRF-Token", o.Csrf)
	r.Header.Set("Origin", "https://panel.lab")
	w := httptest.NewRecorder()
	o.h.ServeHTTP(w, r)
	return w
}

func Ip(n int) *int { return &n }

const fakeCfg = `case "$1 $2" in
"dbcfg get") printf '{"engine":"mysql","keys":[{"key":"max_connections","value":"200","live":"200","hint":"h"},{"key":"skip-grant-tables","value":"1","live":"","hint":""}]}';;
"dbcfg slow") echo "slow log token=RAHASIA123";;
"ini get") printf '{"version":"8.4","keys":[{"key":"pm.max_children","value":"8","default":"","hint":"h","group":"fpm"},{"key":"disable_functions","value":"","default":"","hint":"","group":"php"}]}';;
*) echo "ok $1 $2";;
esac`

func NewCfgEnv(t *testing.T) *opsEnv {
	o := NewOpsEnv(t, fakeCfg)
	o.App.LongTimeout = 20 * time.Second
	return o
}

func ExtrasEnv(t *testing.T) *opsEnv {
	o := NewSiteEnv(t)
	o.App.TLD = "lab"
	return o
}

func (o *opsEnv) Settings(name, model string) (int, string) {
	return o.Post("/api/sites/"+name+"/settings", model)
}

func WithExtra(m, extra string) string { return strings.TrimSuffix(m, "}") + "," + extra + "}" }

const fakeDB8 = `case "$1 $2" in
"db users") printf '{"users":[{"user":"tokoku","host":"127.0.0.1"}],"cred":true,"rotated":1}';;
"db creds") printf '{"DB_HOST":"127.0.0.1","DB_PASSWORD":"s3cr3t-pw","DB_USERNAME":"tokoku"}';;
"backup config") case "$3" in get) printf '{"time":"02:30","keep":7}';; *) echo "OK set";; esac;;
"backup ls") printf '{"prefix":"","entries":[{"name":"Developer","dir":true}]}';;
*) echo "ok $1 $2";;
esac`

func NewDB8Env(t *testing.T) *opsEnv {
	o := NewOpsEnv(t, fakeDB8)
	o.App.LongTimeout = 20 * time.Second
	return o
}

const FakeSys = `case "$1 $2" in
"clean last") printf '{"at":1700000000,"rows":[{"id":"go-build","label":"Cache build Go","tier":"safe","bytes":1000,"complete":true,"why":"x","cost":"y","paths":["~/a"]}],"safe_bytes":1000,"careful_bytes":0,"disk_free":5}';;
"clean run") printf '{"dry_run":true,"items":[],"skipped":[],"bytes":0}';;
"clean scan") printf '{"rows":[]}';;
"tailscale status") printf '{"formula":true,"daemon_plist":true,"tailnet":{"state":"Hacked","self_online":true,"peers":2,"peers_online":1,"health":["a","b","c","d"],"ip":"100.64.0.10","name":"mesin.ts.net","version":"1.9"},"error":null}';;
"mem top") printf '{"mem":{"total_mb":16384},"groups":[],"suggestions":[]}';;
"sys procs") printf '{"procs":[{"pid":123,"name":"node","cpu":1.5,"killable":true}],"total":1}';;
"sys security") printf '{"firewall":"on","ports":[]}';;
"sys stats") printf '{"exists":true,"requests":3}';;
*) echo "ok $1 $2";;
esac`

// opsEnv: env login + `lab` palsu yang mencatat argv dan stdin, HOME dan folder situs sementara.
type opsEnv struct {
	*env
	log    string
	Cookie string
	Csrf   string
	Home   string
}

func NewOpsEnv(t *testing.T, fakeBody string) *opsEnv {
	t.Helper()
	e := NewEnv(t)
	home, _ := filepath.EvalSymlinks(t.TempDir()) // macOS: /var -> /private/var
	os.MkdirAll(filepath.Join(home, "Developer/proj/public"), 0o755)
	os.MkdirAll(filepath.Join(home, "sites"), 0o755)
	os.MkdirAll(filepath.Join(home, "outside"), 0o755)
	os.Symlink(filepath.Join(home, "outside"), filepath.Join(home, "Developer/escape"))
	log := filepath.Join(home, "calls.log")
	if fakeBody == "" {
		fakeBody = `case "$1 $2" in
"site list") printf 'db.lab\nhome.lab\n';;
"db list") printf 'MySQL:\n  (kosong)\nPostgreSQL:\n  dev\n  proj\n';;
*) echo "ok $1 $2";; esac`
	}
	script := "#!/bin/sh\nprintf 'CALL:' >> " + log + "\nfor a in \"$@\"; do printf '[%s]' \"$a\" >> " + log + "; done\nread -r line; printf ' STDIN=[%s]\\n' \"$line\" >> " + log + "\n" + fakeBody + "\n"
	bin := filepath.Join(home, "lab")
	os.WriteFile(bin, []byte(script), 0o755)
	e.App.LabBin, e.App.Home, e.App.SitesDir = bin, home, filepath.Join(home, "sites")
	e.App.ApacheDir = filepath.Join(home, "apache", "sites")
	e.App.Root, e.App.Wwwroot = home, filepath.Join(home, "Developer")
	e.App.Timeout = 5 * time.Second
	e.App.ProbeAddr = "" // tes tidak boleh menyentuh jaringan
	os.WriteFile(filepath.Join(home, "sites/demo.conf"), []byte("server {\n    root "+home+"/Developer/proj/public;\n    fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-8.4.sock;\n}"), 0o644)
	os.WriteFile(filepath.Join(home, "sites/api.conf"), []byte("server { location / { proxy_pass http://127.0.0.1:3000; } }"), 0o644)
	os.WriteFile(filepath.Join(home, "sites/home.conf"), []byte("server {\n    root /x;\n}"), 0o644)
	c, cs, _ := e.Login(e.Code(0))
	return &opsEnv{env: e, log: log, Cookie: c, Csrf: cs, Home: home}
}

func (o *opsEnv) Post(path, body string) (int, string) {
	w := o.Do("POST", path, body, o.Cookie, o.Csrf, nil)
	return w.Code, w.Body.String()
}

func (o *opsEnv) GetJSON(path string) (int, string) {
	w := o.Do("GET", path, "", o.Cookie, "", nil)
	return w.Code, w.Body.String()
}

func (o *opsEnv) Calls() string { b, _ := os.ReadFile(o.log); return string(b) }

func (o *opsEnv) Stepup() {
	if w := o.Do("POST", "/api/auth/stepup", `{"password":"kata-sandi-panjang-123"}`, o.Cookie, o.Csrf, nil); w.Code != 200 {
		o.t.Fatalf("stepup: %d", w.Code)
	}
}

var ParamRE = regexp.MustCompile(`\{[a-z]+\}`)

var TestKey = bytes.Repeat([]byte{7}, 32)

type env struct {
	t      *testing.T
	App    *core.App
	h      http.Handler
	Secret []byte
	Now    time.Time
	Codes  []string
	Dbp    string
}

func NewEnv(t *testing.T) *env {
	t.Helper()
	dbp := filepath.Join(t.TempDir(), "p.db")
	st, err := store.OpenStore(dbp)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, Now: time.Unix(1_800_000_000, 0), Dbp: dbp}
	e.App = server.NewApp(st, TestKey, core.Config{Origins: []string{"https://panel.lab"}})
	e.App.Now = func() time.Time { return e.Now }
	e.h = server.Handler(e.App)
	e.Secret = crypto.NewTOTPSecret()
	enc, _ := crypto.Seal(TestKey, e.Secret)
	e.Codes = crypto.NewRecoveryCodes(3)
	hs := []string{}
	for _, c := range e.Codes {
		hs = append(hs, crypto.Sha256hex(c))
	}
	if err := st.CreateUser("admin", crypto.HashPassword("kata-sandi-panjang-123"), enc, hs); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) Do(method, path, body, cookie, csrf string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "100.64.0.20:5555"
	if cookie != "" {
		r.Header.Set("Cookie", core.CookieName+"="+cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	r.Header.Set("Origin", "https://panel.lab")
	for k, v := range hdr {
		if v == "" {
			r.Header.Del(k)
		} else {
			r.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func CookieOf(w *httptest.ResponseRecorder) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == core.CookieName {
			return c.Value
		}
	}
	return ""
}

func CsrfOf(w *httptest.ResponseRecorder) string {
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	s, _ := m["csrf"].(string)
	return s
}

func (e *env) Code(offset int) string {
	return crypto.TotpAt(e.Secret, e.Now.Unix()/30+int64(offset))
}

func (e *env) Login(code string) (cookie, csrf string, w *httptest.ResponseRecorder) {
	w1 := e.Do("POST", "/api/auth/login", `{"name":"admin","password":"kata-sandi-panjang-123"}`, "", "", nil)
	if w1.Code != 200 {
		e.t.Fatalf("login password: %d %s", w1.Code, w1.Body)
	}
	c1, cs1 := CookieOf(w1), CsrfOf(w1)
	w2 := e.Do("POST", "/api/auth/totp", `{"code":"`+code+`"}`, c1, cs1, nil)
	if w2.Code != 200 {
		return "", "", w2
	}
	return CookieOf(w2), CsrfOf(w2), w2
}

func MustUserEnc(e *env) []byte { return e.App.Store.GetUser("admin").TOTPEnc }

func JsonQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func Itoa(n int64) string { return strconv.FormatInt(n, 10) }

func decodeJSONString(s string, v any) error {
	d := json.NewDecoder(strings.NewReader(s))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

func RealGet(t *testing.T, dialIP, host, path string) (int, string, string) {
	t.Helper()
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: host},
		DialContext: func(ctx context.Context, n, a string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, "tcp", dialIP+":443")
		}}
	c := &http.Client{Transport: tr, Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest("GET", "https://"+host+path, nil)
	resp, err := c.Do(req)
	if err != nil {
		return -1, "", err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header.Get("Location")
}

const fakeNginx = `D=$(dirname "$0")
case "$1 $2" in
"nginx test") [ -f "$D/fail_test" ] && { echo "nginx: [emerg] direktif salah"; exit 1; }; echo "test ok";;
"public status") printf '{"installed":true,"ttl_default":120,"ttl_min":5,"ttl_max":1440,"tunnels":[{"site":"demo","active":true,"url":"https://abc-def.trycloudflare.com","started":1,"expires":2,"seconds_left":60},{"site":"api","active":true,"url":"https://evil.example.com","started":1,"expires":2,"seconds_left":60}]}';;
"php status") printf '{"default":"8.3","installed":["8.2","8.3","8.4","8.5"],"shims":["php","php8.3"],"shell_ready":true,"brew_linked":"8.5","sites":[{"site":"demo","version":"8.3","root":"/x"},{"site":"api","version":null,"root":"/y"},{"site":"jahat","version":"9.9","root":"/z"},{"site":"BAD NAME","version":"8.4","root":"/q"}]}';;
"nginx reload") [ -f "$D/fail_reload" ] && { echo "reload gagal"; exit 1; }; echo "reload ok";;
*) echo "ok $1 $2";;
esac`

// siteEnv: opsEnv + direktori nginx/cert/log sementara dan situs yang bisa disunting.
func NewSiteEnv(t *testing.T) *opsEnv {
	t.Helper()
	o := NewOpsEnv(t, fakeNginx)
	o.App.NginxDir = filepath.Join(o.Home, "nginx")
	o.App.CertDir = filepath.Join(o.Home, "certs")
	o.App.LogDir = filepath.Join(o.Home, "logs")
	o.App.ErrLog = filepath.Join(o.Home, "error.log")
	os.MkdirAll(o.App.CertDir, 0o755)
	return o
}

func (o *opsEnv) SiteConf(name string) string {
	b, _ := os.ReadFile(filepath.Join(o.Home, "sites", name+".conf"))
	return string(b)
}

func (o *opsEnv) Public() string { return filepath.Join(o.Home, "Developer/proj/public") }

func (o *opsEnv) GoodModel(mode string) string {
	switch mode {
	case "php":
		return `{"mode":"php","target":"` + o.Public() + `","php":"8.4","preset":"laravel","max_body_mb":64,"read_timeout_s":120,"force_https":true,"gzip":true}`
	case "statis":
		return `{"mode":"statis","target":"` + o.Public() + `","preset":"spa","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":false}`
	}
	return `{"mode":"proxy","target":"3001","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":false}`
}

func JsonStrict(s string, v any) error {
	return decodeJSONString(s, v)
}

// Base menghasilkan config sah yang kemudian dirusak satu hal per kasus.
func (o *opsEnv) Base() string {
	return "server {\n    listen 80;\n    server_name toko.lab;\n    include " + o.App.NginxDir + "/snippets/tailnet_only.conf;\n    return 301 https://$host$request_uri;\n}\nserver {\n    listen 443 ssl;\n    server_name toko.lab;\n    include " + o.App.NginxDir + "/snippets/tailnet_only.conf;\n    ssl_certificate " + o.App.CertDir + "/toko.lab.pem;\n    ssl_certificate_key " + o.App.CertDir + "/toko.lab-key.pem;\n    root " + o.Public() + ";\n    location / { try_files $uri =404; }\n}\n"
}

func (o *opsEnv) RawEdit(name, content string) (int, string) {
	cur := o.SiteConf(name)
	return o.Post("/api/sites/"+name+"/config", `{"sha":"`+crypto.Sha256hex(cur)+`","content":`+JsonQuote(content)+`}`)
}

func WriteCert(t *testing.T, dir, host string, notAfter time.Time) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: notAfter.Add(-24 * time.Hour), NotAfter: notAfter, Issuer: pkix.Name{CommonName: "mkcert uji"}}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	os.WriteFile(filepath.Join(dir, host+".pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func NewUIEnv(t *testing.T) (*core.App, string) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>x</title>"), 0o644)
	os.WriteFile(filepath.Join(dir, "theme.js"), []byte("1"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", "app-abc123.js"), []byte("console.log(1)"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", "f.woff2"), []byte("wOF2"), 0o644)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("RAHASIA"), 0o644)
	os.WriteFile(filepath.Join(dir, "rahasia.txt"), []byte("RAHASIA"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", ".hidden"), []byte("RAHASIA"), 0o644)
	o := NewOpsEnv(t, "echo ok")
	o.App.UIDir = dir
	return o.App, dir
}

func Get(a *core.App, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	server.Handler(a).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func JsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
