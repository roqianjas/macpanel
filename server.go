package main

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
	st          *Store
	key         []byte
	cfg         Config
	dummy       string
	now         func() time.Time
	labBin      string // biner lab (path tetap)
	home        string
	root        string // MACPANEL_HOME (bawaan ~/MacPanel)
	wwwroot     string // batas folder situs: <root>/wwwroot // HOME user; batas ~/MacPanel/wwwroot dihitung dari sini
	sitesDir    string // folder konfigurasi situs nginx (hanya dibaca panel)
	tld         string
	nginxDir    string // folder config nginx (snippets)
	certDir     string
	logDir      string // log akses per situs
	errLog      string
	timeout     time.Duration  // batas waktu tiap aksi cepat
	longTimeout time.Duration  // batas aksi lama (install, update)
	bg          sync.WaitGroup // aksi latar belakang yang masih berjalan
	mu          sync.Mutex     // satu aksi mutasi pada satu waktu
	uiDir       string         // folder hasil build UI (kosong = tanpa UI)
	probeAddr   string         // loopback:443 untuk pemeriksaan situs (kosong = tanpa pemeriksaan)
	col         *Collector     // pengumpul metrik (nil di tes)
	sysMu       sync.Mutex
	sysCache    *SystemInfo
	sysAt       time.Time
	alMu        sync.Mutex
	alCache     []Alert
	alAt        time.Time
}

type Route struct {
	Method, Path string
	Level        Level
	H            func(*App, http.ResponseWriter, *http.Request, *Session)
}

const (
	lockWindow  = 15 * time.Minute
	maxPerUser  = 5
	maxPerIP    = 10
	cookieName  = "__Host-panel"
	maxBodySize = 8 << 10
)

func newApp(st *Store, key []byte, cfg Config) *App {
	home, _ := os.UserHomeDir()
	root := os.Getenv("MACPANEL_HOME")
	if root == "" {
		root = filepath.Join(home, "MacPanel")
	}
	return &App{st: st, key: key, cfg: cfg, dummy: hashPassword("dummy-untuk-menyamakan-waktu"), now: time.Now,
		labBin: "/opt/homebrew/bin/lab", home: home, root: root, wwwroot: filepath.Join(root, "wwwroot"), sitesDir: filepath.Join(root, "server/nginx/sites"), tld: "lab", probeAddr: envOr("MACPANEL_PROBE_ADDR", "127.0.0.1:443"), timeout: 120 * time.Second, longTimeout: 30 * time.Minute,
		nginxDir: filepath.Join(root, "server/nginx"), certDir: "/opt/homebrew/etc/nginx/certs", logDir: "/opt/homebrew/var/log/nginx/sites", errLog: "/opt/homebrew/var/log/nginx/error.log"}
}

// routes adalah SATU-SATUNYA daftar rute. Tes memastikan semua yang bukan Public menolak permintaan tanpa sesi.
func routes() []Route {
	return []Route{
		{"GET", "/healthz", Public, hHealth},
		{"POST", "/api/auth/login", Public, hLogin},
		{"POST", "/api/auth/totp", Pwd, hTOTP},
		{"POST", "/api/auth/logout", Any, hLogout},
		{"POST", "/api/auth/stepup", Full, hStepUp},
		{"GET", "/api/me", Full, hMe},
		{"GET", "/api/audit", Full, hAudit},
		{"GET", "/api/sites", Full, hSitesRich},
		{"POST", "/api/sites", Full, hSiteAdd},
		{"POST", "/api/sites/{name}/remove", Full, hSiteRemove},
		{"GET", "/api/db", Full, hDBList},
		{"POST", "/api/db", Full, hDBCreate},
		{"POST", "/api/db/{engine}/{name}/dump", Full, hDBDump},
		{"POST", "/api/db/{engine}/{name}/drop", Full, hDBDrop},
		{"GET", "/api/jobs", Full, hJobs},
		{"GET", "/api/sites/{name}/settings", Full, hSettingsGet},
		{"POST", "/api/sites/{name}/settings", Full, hSettingsSet},
		{"GET", "/api/sites/{name}/config", Full, hConfigGet},
		{"POST", "/api/sites/{name}/config", Full, hConfigSet},
		{"POST", "/api/sites/{name}/restore", Full, hRestore},
		{"GET", "/api/sites/{name}/ssl", Full, hSSLGet},
		{"POST", "/api/sites/{name}/ssl/renew", Full, hSSLRenew},
		{"GET", "/api/sites/{name}/logs", Full, hLogs},
		{"GET", "/api/jobs/{id}", Full, hJob},
		{"GET", "/api/db/{engine}/{name}/dumps", Full, hDBDumps},
		{"POST", "/api/db/{engine}/{name}/restore", Full, hDBRestore},
		{"GET", "/api/php/{version}/ext", Full, hExtList},
		{"POST", "/api/php/{version}/ext/{name}/{action}", Full, hExtAction},
		{"GET", "/api/php/{version}/ini", Full, hIniGet},
		{"POST", "/api/php/{version}/ini", Full, hIniSet},
		{"GET", "/api/composer", Full, hComposer},
		{"POST", "/api/composer/{action}", Full, hComposerAct},
		{"GET", "/api/sites/{name}/deploy", Full, hDeployInfo},
		{"POST", "/api/sites/{name}/deploy", Full, hDeployRun},
		{"GET", "/api/backups", Full, hBackups},
		{"GET", "/api/alerts", Full, hAlerts},
		{"GET", "/api/sites/{name}/hook", Full, hHookGet},
		{"POST", "/api/sites/{name}/hook", Full, hHookSet},
		{"POST", "/api/sites/{name}/hook/remove", Full, hHookDel},
		{"GET", "/api/backups/prune-preview", Full, hBackupPrunePreview},
		{"POST", "/api/backups/prune", Full, hBackupPrune},
		{"POST", "/api/backups/delete", Full, hBackupDelete},
		{"GET", "/api/backups/extras", Full, hBackupExtras},
		{"GET", "/api/backups/extras-preview", Full, hBackupExtrasPreview},
		{"POST", "/api/backups/extras", Full, hBackupExtrasClean},
		{"POST", "/api/installer", Full, hInstall},
		{"POST", "/api/sites/{name}/clone", Full, hClone},
		{"GET", "/api/redis/info", Full, hRedisInfo},
		{"GET", "/api/redis/scan", Full, hRedisScan},
		{"POST", "/api/redis/get", Full, hRedisGet},
		{"POST", "/api/redis/del", Full, hRedisDel},
		{"POST", "/api/me/password", Full, hMyPassword},
		{"GET", "/api/me/devices", Full, hDevices},
		{"GET", "/api/docs", Full, hDocs},
		{"POST", "/api/me/devices/revoke", Full, hDeviceRevoke},
		{"GET", "/api/sys/procs", Full, hSysProcs},
		{"POST", "/api/sys/kill", Full, hSysKill},
		{"GET", "/api/sys/disk", Full, hSysDisk},
		{"GET", "/api/sys/security", Full, hSysSecurity},
		{"GET", "/api/sys/docker", Full, hSysDocker},
		{"GET", "/api/sys/dns", Full, hSysDNS},
		{"GET", "/api/sites/{name}/stats", Full, hSiteStats},
		{"GET", "/api/db/{engine}/{name}/users", Full, hDBUsers},
		{"POST", "/api/db/{engine}/{name}/passwd", Full, hDBPasswd},
		{"POST", "/api/db/{engine}/{name}/creds", Full, hDBCreds},
		{"GET", "/api/backups/config", Full, hBackupCfgGet},
		{"POST", "/api/backups/config", Full, hBackupCfgSet},
		{"GET", "/api/backups/ls", Full, hBackupLs},
		{"POST", "/api/backups/extract", Full, hBackupExtract},
		{"GET", "/api/sites/{name}/auth", Full, hAuthGet},
		{"POST", "/api/sites/{name}/auth/users", Full, hAuthSet},
		{"POST", "/api/sites/{name}/auth/users/{user}/remove", Full, hAuthDel},
		{"GET", "/api/sites/{name}/env", Full, hEnvGet},
		{"POST", "/api/sites/{name}/env", Full, hEnvSet},
		{"GET", "/api/dbcfg/{engine}", Full, hDBCfgGet},
		{"POST", "/api/dbcfg/{engine}", Full, hDBCfgSet},
		{"GET", "/api/dbcfg/mysql/slow", Full, hSlowLog},
		{"GET", "/api/users", Full, hUsers},
		{"POST", "/api/db/{engine}/{name}/upload", Full, hDBUpload},
		{"POST", "/api/backups/run", Full, hBackupRun},
		{"GET", "/api/services", Full, hServicesList},
		{"POST", "/api/services/{id}/{action}", Full, hServiceAction},
		{"GET", "/api/services/{id}/logs", Full, hServiceLogs},
		{"POST", "/api/apps/{id}/install", Full, hAppInstall},
		{"POST", "/api/apps/{id}/remove", Full, hAppRemove},
		{"POST", "/api/sites/{name}/{action}", Full, hSiteToggle},
		{"GET", "/api/metrics", Full, hMetrics},
		{"GET", "/api/system", Full, hSystem},
		{"GET", "/api/php", Full, hPHPList},
		{"POST", "/api/php/{version}/install", Full, hPHPInstall},
		{"POST", "/api/php/{version}/remove", Full, hPHPRemove},
		{"GET", "/api/runtime", Full, hRuntimeList},
		{"POST", "/api/runtime/{tool}/{version}/install", Full, hRuntimeInstall},
		{"POST", "/api/runtime/{tool}/{version}/use", Full, hRuntimeUse},
		{"POST", "/api/runtime/{tool}/{version}/remove", Full, hRuntimeRemove},
		{"GET", "/api/tasks/{kind}", Full, hTasksList},
		{"POST", "/api/tasks/{kind}", Full, hTaskAdd},
		{"POST", "/api/tasks/{kind}/{name}/restart", Full, hTaskRestart},
		{"POST", "/api/tasks/{kind}/{name}/remove", Full, hTaskRemove},
		{"GET", "/api/tasks/{kind}/{name}/logs", Full, hTaskLogs},
		{"GET", "/api/updates", Full, hUpdatesCheck},
		{"POST", "/api/updates/apply", Full, hUpdatesApply},
	}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, rt := range routes() {
		mux.HandleFunc(rt.Method+" "+rt.Path, a.wrap(rt))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			secHeaders(w)
			jsonOut(w, 404, map[string]string{"error": "not found"})
			return
		}
		a.serveStatic(w, r)
	})
	return mux
}

func secHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
}

func jsonOut(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func jerr(w http.ResponseWriter, code int, msg string) {
	jsonOut(w, code, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeN(w, r, v, maxBodySize)
}

// decodeN membatasi ukuran body per rute; editor config butuh lebih besar dari default.
func decodeN(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		code := 400
		if strings.Contains(err.Error(), "too large") {
			code = 413
		}
		jerr(w, code, "bad request")
		return false
	}
	return true
}

func (a *App) clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			p := strings.Split(xff, ",")
			last := strings.TrimSpace(p[len(p)-1]) // nilai terakhir ditambahkan nginx; yang lebih awal bisa dipalsukan klien
			if net.ParseIP(last) != nil {
				return last
			}
		}
	}
	return host
}

func (a *App) originOK(r *http.Request) bool {
	if sf := r.Header.Get("Sec-Fetch-Site"); sf != "" && sf != "same-origin" && sf != "none" {
		return false
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	allowed := a.cfg.Origins
	if len(allowed) == 0 {
		allowed = []string{"https://" + r.Host, "http://" + r.Host}
	}
	for _, x := range allowed {
		if o == x {
			return true
		}
	}
	return false
}

func (a *App) session(r *http.Request) *Session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	return a.st.GetSession(c.Value, a.now())
}

func (a *App) wrap(rt Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secHeaders(w)
		var ss *Session
		if rt.Level != Public {
			ss = a.session(r)
			if ss == nil || (rt.Level == Pwd && ss.Stage != "pwd") || (rt.Level == Full && ss.Stage != "full") {
				jerr(w, 401, "unauthenticated")
				return
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if ss != nil && ss.Stage == "full" && ss.Role == "viewer" && r.URL.Path != "/api/auth/logout" {
				jerr(w, 403, "akun ini hanya-baca")
				return
			}
			if !a.originOK(r) {
				jerr(w, 403, "forbidden")
				return
			}
			if ss != nil && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(ss.CSRF)) != 1 {
				jerr(w, 403, "forbidden")
				return
			}
		}
		rt.H(a, w, r, ss)
	}
}

func (a *App) setCookie(w http.ResponseWriter, tok string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: tok, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func (a *App) locked(name, ip string) bool {
	since := a.now().Add(-lockWindow)
	return a.st.CountAttempts("u:"+name, since) >= maxPerUser || a.st.CountAttempts("ip:"+ip, since) >= maxPerIP
}

func (a *App) fail(name, ip string) {
	a.st.AddAttempt("u:"+name, a.now())
	a.st.AddAttempt("ip:"+ip, a.now())
}

func tooMany(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(int(lockWindow.Seconds())))
	jerr(w, 429, "terlalu banyak percobaan, coba lagi nanti")
}

// ---------- handler ----------

func hHealth(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

func hLogin(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	var in struct{ Name, Password string }
	if !decode(w, r, &in) {
		return
	}
	name, ip := strings.ToLower(strings.TrimSpace(in.Name)), a.clientIP(r)
	if len(name) > 40 || len(in.Password) > 256 {
		jerr(w, 401, "login gagal")
		return
	}
	if a.locked(name, ip) {
		a.st.Audit(name, "login_locked", "", ip, a.now())
		tooMany(w)
		return
	}
	u := a.st.GetUser(name)
	ok := false
	if u != nil && !u.Disabled {
		ok = verifyPassword(u.PassHash, in.Password)
	} else {
		verifyPassword(a.dummy, in.Password) // samakan waktu respons
	}
	if !ok {
		a.fail(name, ip)
		a.st.Audit(name, "login_fail", "password", ip, a.now())
		jerr(w, 401, "login gagal")
		return
	}
	if c, err := r.Cookie(trustCookie); err == nil {
		if nt, ok := a.st.UseTrusted(c.Value, u.ID, a.now()); ok {
			tok, full := a.st.NewSession(u.ID, "full", a.now())
			a.setCookie(w, tok, int(absTTL.Seconds()))
			a.setTrust(w, nt, int(trustTTL.Seconds()))
			a.st.ClearAttempts("u:" + name)
			a.st.Audit(name, "login_trusted", "", ip, a.now())
			jsonOut(w, 200, map[string]any{"ok": true, "trusted": true, "csrf": full.CSRF})
			return
		}
	}
	tok, ss := a.st.NewSession(u.ID, "pwd", a.now())
	a.setCookie(w, tok, int(pwdTTL.Seconds()))
	a.st.Audit(name, "login_pwd_ok", "", ip, a.now())
	jsonOut(w, 200, map[string]any{"need_totp": true, "csrf": ss.CSRF})
}

// verifyCode memeriksa TOTP atau recovery code untuk user sesi. Gagal dihitung sebagai percobaan.
func (a *App) verifyCode(ss *Session, code, ip string) bool {
	u := a.st.GetUser(ss.User)
	if u == nil {
		return false
	}
	code = strings.TrimSpace(code)
	if len(code) == 6 {
		secret, err := open(a.key, u.TOTPEnc)
		if err != nil {
			return false
		}
		step, ok := verifyTOTP(secret, code, a.now(), u.TOTPLast)
		return ok && a.st.UseTOTPStep(u.ID, step)
	}
	return len(code) == 11 && a.st.UseRecovery(u.ID, code)
}

func hTOTP(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct {
		Code  string
		Trust bool
	}
	if !decode(w, r, &in) {
		return
	}
	ip := a.clientIP(r)
	if a.locked(ss.User, ip) {
		a.st.Audit(ss.User, "totp_locked", "", ip, a.now())
		tooMany(w)
		return
	}
	if !a.verifyCode(ss, in.Code, ip) {
		a.fail(ss.User, ip)
		a.st.Audit(ss.User, "totp_fail", "", ip, a.now())
		jerr(w, 401, "kode salah")
		return
	}
	a.st.DelSession(ss.ID)
	tok, full := a.st.NewSession(ss.UserID, "full", a.now()) // sesi baru: id berganti setelah naik tahap
	a.st.ClearAttempts("u:" + ss.User)
	a.setCookie(w, tok, int(absTTL.Seconds()))
	a.st.Audit(ss.User, "login_ok", "", ip, a.now())
	if in.Trust {
		a.setTrust(w, a.st.AddTrusted(ss.UserID, deviceLabel(r.UserAgent()), a.now()), int(trustTTL.Seconds()))
		a.st.Audit(ss.User, "trust_add", "", ip, a.now())
	}
	jsonOut(w, 200, map[string]any{"ok": true, "csrf": full.CSRF})
}

func hLogout(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	a.st.DelSession(ss.ID)
	a.setCookie(w, "", -1)
	a.st.Audit(ss.User, "logout", "", a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func hStepUp(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Password string }
	if !decode(w, r, &in) {
		return
	}
	ip := a.clientIP(r)
	if a.locked(ss.User, ip) {
		tooMany(w)
		return
	}
	u := a.st.GetUser(ss.User)
	if u == nil || u.Disabled || len(in.Password) > 256 || !verifyPassword(u.PassHash, in.Password) {
		a.fail(ss.User, ip)
		a.st.Audit(ss.User, "stepup_fail", "", ip, a.now())
		jerr(w, 400, "kata sandi salah") // 400 (bukan 401) agar UI tidak mengeluarkan sesi
		return
	}
	a.st.SetStepUp(ss.ID, a.now().Add(stepupTTL))
	a.st.Audit(ss.User, "stepup_ok", "", ip, a.now())
	jsonOut(w, 200, map[string]any{"ok": true, "valid_seconds": int(stepupTTL.Seconds())})
}

// requireStepUp dipakai aksi destruktif (fase berikutnya).
func (a *App) requireStepUp(w http.ResponseWriter, ss *Session) bool {
	if ss.StepUp > a.now().Unix() {
		return true
	}
	jerr(w, 403, "butuh verifikasi kata sandi ulang")
	return false
}

func hMe(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	jsonOut(w, 200, map[string]any{"user": ss.User, "csrf": ss.CSRF, "stepup": ss.StepUp > a.now().Unix(), "role": ss.Role})
}

func hAudit(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	jsonOut(w, 200, map[string]any{"rows": a.st.RecentAudit(100)})
}

// envOr membaca env opsional (dipakai untuk instance demo/uji); nilai bawaan dipakai bila kosong.
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
