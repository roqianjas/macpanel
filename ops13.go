package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---------- penyimpanan ----------

const hooksSchema = `CREATE TABLE IF NOT EXISTS hooks(site TEXT PRIMARY KEY, secret_enc BLOB NOT NULL, branch TEXT NOT NULL, steps TEXT NOT NULL,
  enabled INTEGER NOT NULL, last_at INTEGER NOT NULL DEFAULT 0, last_result TEXT NOT NULL DEFAULT '');`

type Hook struct {
	Site       string
	SecretEnc  []byte
	Branch     string
	Steps      []string
	Enabled    bool
	LastAt     int64
	LastResult string
}

func (s *Store) GetHook(site string) *Hook {
	h := &Hook{Site: site}
	var steps string
	var en int
	if err := s.db.QueryRow("SELECT secret_enc,branch,steps,enabled,last_at,last_result FROM hooks WHERE site=?", site).Scan(&h.SecretEnc, &h.Branch, &steps, &en, &h.LastAt, &h.LastResult); err != nil {
		return nil
	}
	h.Steps, h.Enabled = strings.Split(steps, ","), en == 1
	return h
}

func (s *Store) PutHook(h *Hook) error {
	en := 0
	if h.Enabled {
		en = 1
	}
	_, err := s.db.Exec(`INSERT INTO hooks(site,secret_enc,branch,steps,enabled) VALUES(?,?,?,?,?)
		ON CONFLICT(site) DO UPDATE SET secret_enc=excluded.secret_enc, branch=excluded.branch, steps=excluded.steps, enabled=excluded.enabled`,
		h.Site, h.SecretEnc, h.Branch, strings.Join(h.Steps, ","), en)
	return err
}

func (s *Store) SetHookLast(site string, at int64, res string) {
	_, _ = s.db.Exec("UPDATE hooks SET last_at=?, last_result=? WHERE site=?", at, clean(res, 80), site)
}

func (s *Store) DeleteHook(site string) bool {
	r, err := s.db.Exec("DELETE FROM hooks WHERE site=?", site)
	n, _ := r.RowsAffected()
	return err == nil && n > 0
}

// ---------- pengelolaan (butuh sesi penuh + step-up) ----------

var branchRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,99}$`)

func branchOK(b string) bool {
	return branchRE.MatchString(b) && !strings.Contains(b, "..") && !strings.HasSuffix(b, "/") && !strings.HasSuffix(b, ".lock") && !strings.Contains(b, "//")
}

func hHookGet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	if ss.Role == "viewer" {
		jerr(w, 403, "hanya admin")
		return
	}
	h := a.st.GetHook(name)
	if h == nil {
		jsonOut(w, 200, map[string]any{"configured": false})
		return
	}
	jsonOut(w, 200, map[string]any{"configured": true, "enabled": h.Enabled, "branch": h.Branch, "steps": h.Steps, "last_at": h.LastAt, "last_result": h.LastResult})
}

func hHookSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Enabled bool
		Branch  string
		Steps   []string
		Rotate  bool
	}
	if !decode(w, r, &in) {
		return
	}
	if !branchOK(in.Branch) {
		jerr(w, 400, "nama branch tidak valid")
		return
	}
	if len(in.Steps) < 1 || len(in.Steps) > 8 {
		jerr(w, 400, "pilih 1-8 langkah")
		return
	}
	seen := map[string]bool{}
	for _, s := range in.Steps {
		if !deploySteps[s] || seen[s] {
			jerr(w, 400, "langkah tidak dikenal atau ganda: "+s)
			return
		}
		seen[s] = true
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	h := a.st.GetHook(name)
	newSecret := ""
	if h == nil || in.Rotate {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			jerr(w, 500, "gagal membuat secret")
			return
		}
		newSecret = hex.EncodeToString(b)
		enc, err := seal(a.key, []byte(newSecret))
		if err != nil {
			jerr(w, 500, "gagal menyimpan secret")
			return
		}
		if h == nil {
			h = &Hook{Site: name}
		}
		h.SecretEnc = enc
	}
	h.Branch, h.Steps, h.Enabled = in.Branch, in.Steps, in.Enabled
	if err := a.st.PutHook(h); err != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	act := "hook_update"
	if newSecret != "" {
		act = "hook_secret_new"
	}
	a.st.Audit(ss.User, act, name+" branch "+in.Branch, a.clientIP(r), a.now())
	out := map[string]any{"saved": true}
	if newSecret != "" {
		out["secret"] = newSecret // satu-satunya kali secret ditampilkan
	}
	jsonOut(w, 200, out)
}

func hHookDel(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) || !a.requireStepUp(w, ss) {
		return
	}
	if !a.st.DeleteHook(name) {
		jerr(w, 404, "webhook belum diatur")
		return
	}
	a.st.Audit(ss.User, "hook_delete", name, a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"deleted": true})
}

// ---------- penerima publik (listener terpisah; satu-satunya rute di sana) ----------

var (
	hookBusy   sync.Map // site -> struct{}
	hookSeenMu sync.Mutex
	hookSeen   = map[string]time.Time{}
)

const hookMinGap = 15 * time.Second
const hookMaxBody = 1 << 20

func hookReplay(id string, now time.Time) bool {
	if id == "" {
		return false
	}
	hookSeenMu.Lock()
	defer hookSeenMu.Unlock()
	for k, t := range hookSeen {
		if now.Sub(t) > time.Hour {
			delete(hookSeen, k)
		}
	}
	if _, dup := hookSeen[id]; dup {
		return true
	}
	if len(hookSeen) < 2000 {
		hookSeen[id] = now
	}
	return false
}

// HookHandler hanya melayani POST /hook/{site}. Tidak ada sesi; keabsahan dibuktikan tanda tangan HMAC-SHA256 (format GitHub).
func (a *App) HookHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /hook/{site}", a.hookRecv)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	return mux
}

func hookReply(w http.ResponseWriter, code int, v map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (a *App) hookRecv(w http.ResponseWriter, r *http.Request) {
	site, ip := r.PathValue("site"), a.clientIP(r)
	lockKey := "hook:" + ip
	if a.locked(lockKey, ip) {
		hookReply(w, 429, map[string]any{"error": "terlalu banyak percobaan"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, hookMaxBody))
	if err != nil {
		hookReply(w, 413, map[string]any{"error": "terlalu besar"})
		return
	}
	notFound := func(why string) {
		a.fail(lockKey, ip)
		a.st.Audit("webhook", "hook_reject", clean(site, 40)+" "+why, ip, a.now())
		hookReply(w, 404, map[string]any{"error": "tidak ditemukan"})
	}
	var h *Hook
	if siteRE.MatchString(site) && !protectedSites[site] {
		h = a.st.GetHook(site)
	}
	secret := []byte("tidak-ada-secret-dummy-untuk-waktu-sama")
	if h != nil && h.Enabled {
		if s, err := open(a.key, h.SecretEnc); err == nil {
			secret = s
		} else {
			h = nil
		}
	} else {
		h = nil
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	got := r.Header.Get("X-Hub-Signature-256")
	if h == nil {
		notFound("tidak ada/nonaktif")
		return
	}
	if !hmac.Equal([]byte(got), []byte(want)) {
		notFound("tanda tangan salah")
		return
	}
	switch r.Header.Get("X-GitHub-Event") {
	case "ping":
		hookReply(w, 200, map[string]any{"ok": "pong"})
		return
	case "push":
	default:
		hookReply(w, 202, map[string]any{"ignored": "event"})
		return
	}
	var p struct {
		Ref     string `json:"ref"`
		Deleted bool   `json:"deleted"`
		After   string `json:"after"`
	}
	if json.Unmarshal(body, &p) != nil {
		hookReply(w, 400, map[string]any{"error": "payload bukan JSON"})
		return
	}
	if p.Deleted || p.Ref != "refs/heads/"+h.Branch {
		hookReply(w, 202, map[string]any{"ignored": "branch"})
		return
	}
	now := a.now()
	if hookReplay(r.Header.Get("X-GitHub-Delivery"), now) {
		hookReply(w, 202, map[string]any{"ignored": "pengiriman ganda"})
		return
	}
	if h.LastAt != 0 && now.Sub(time.Unix(h.LastAt, 0)) < hookMinGap {
		hookReply(w, 202, map[string]any{"ignored": "terlalu cepat"})
		return
	}
	if _, busy := hookBusy.LoadOrStore(site, struct{}{}); busy {
		hookReply(w, 202, map[string]any{"ignored": "deploy sedang berjalan"})
		return
	}
	a.st.SetHookLast(site, now.Unix(), "berjalan")
	short := p.After
	if len(short) > 7 {
		short = short[:7]
	}
	short = regexp.MustCompile(`[^0-9a-f]`).ReplaceAllString(short, "")
	steps := strings.Join(h.Steps, ",")
	detail := site + " [" + steps + "] " + short
	id := a.st.AddJob("webhook", "webhook_deploy "+detail, "berjalan", "", 0, now)
	a.st.Audit("webhook", "webhook_deploy", detail+" -> mulai", ip, now)
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		defer hookBusy.Delete(site)
		a.mu.Lock()
		res := a.runLabT([]string{"deploy", "run", site, steps}, "", a.longTimeout)
		a.mu.Unlock()
		st := statusOf(res)
		a.st.FinishJob(id, st, res.Code, res.Out)
		a.st.SetHookLast(site, a.now().Unix(), st)
		a.st.Audit("webhook", "webhook_deploy", detail+" -> "+st, ip, a.now())
	}()
	hookReply(w, 202, map[string]any{"job": id, "status": "berjalan"})
}
