package deploy

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

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/store"
)

var branchRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,99}$`)

func branchOK(b string) bool {
	return branchRE.MatchString(b) && !strings.Contains(b, "..") && !strings.HasSuffix(b, "/") && !strings.HasSuffix(b, ".lock") && !strings.Contains(b, "//")
}

func HookGet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	if ss.Role == "viewer" {
		core.WriteError(w, 403, "hanya admin")
		return
	}
	h := a.Store.GetHook(name)
	if h == nil {
		core.WriteJSON(w, 200, map[string]any{"configured": false})
		return
	}
	core.WriteJSON(w, 200, map[string]any{"configured": true, "enabled": h.Enabled, "branch": h.Branch, "steps": h.Steps, "last_at": h.LastAt, "last_result": h.LastResult})
}

func HookSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Enabled bool
		Branch  string
		Steps   []string
		Rotate  bool
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !branchOK(in.Branch) {
		core.WriteError(w, 400, "nama branch tidak valid")
		return
	}
	if len(in.Steps) < 1 || len(in.Steps) > 8 {
		core.WriteError(w, 400, "pilih 1-8 langkah")
		return
	}
	seen := map[string]bool{}
	for _, s := range in.Steps {
		if !deploySteps[s] || seen[s] {
			core.WriteError(w, 400, "langkah tidak dikenal atau ganda: "+s)
			return
		}
		seen[s] = true
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	h := a.Store.GetHook(name)
	newSecret := ""
	if h == nil || in.Rotate {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			core.WriteError(w, 500, "gagal membuat secret")
			return
		}
		newSecret = hex.EncodeToString(b)
		enc, err := crypto.Seal(a.Key, []byte(newSecret))
		if err != nil {
			core.WriteError(w, 500, "gagal menyimpan secret")
			return
		}
		if h == nil {
			h = &store.Hook{Site: name}
		}
		h.SecretEnc = enc
	}
	h.Branch, h.Steps, h.Enabled = in.Branch, in.Steps, in.Enabled
	if err := a.Store.PutHook(h); err != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	act := "hook_update"
	if newSecret != "" {
		act = "hook_secret_new"
	}
	a.Store.Audit(ss.User, act, name+" branch "+in.Branch, a.ClientIP(r), a.Now())
	out := map[string]any{"saved": true}
	if newSecret != "" {
		out["secret"] = newSecret // satu-satunya kali secret ditampilkan
	}
	core.WriteJSON(w, 200, out)
}

func HookDel(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) || !a.RequireStepUp(w, ss) {
		return
	}
	if !a.Store.DeleteHook(name) {
		core.WriteError(w, 404, "webhook belum diatur")
		return
	}
	a.Store.Audit(ss.User, "hook_delete", name, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"deleted": true})
}

var hookBusy sync.Map

var hookSeenMu sync.Mutex

var hookSeen = map[string]time.Time{}

const hookMinGap = 15 * time.Second

const HookMaxBody = 1 << 20

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
func HookHandler(a *core.App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /hook/{site}", func(w http.ResponseWriter, r *http.Request) { hookRecv(a, w, r) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	return mux
}

func hookReply(w http.ResponseWriter, code int, v map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func hookRecv(a *core.App, w http.ResponseWriter, r *http.Request) {
	site, ip := r.PathValue("site"), a.ClientIP(r)
	lockKey := "hook:" + ip
	if a.Locked(lockKey, ip) {
		hookReply(w, 429, map[string]any{"error": "terlalu banyak percobaan"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, HookMaxBody))
	if err != nil {
		hookReply(w, 413, map[string]any{"error": "terlalu besar"})
		return
	}
	notFound := func(why string) {
		a.Fail(lockKey, ip)
		a.Store.Audit("webhook", "hook_reject", store.Clean(site, 40)+" "+why, ip, a.Now())
		hookReply(w, 404, map[string]any{"error": "tidak ditemukan"})
	}
	var h *store.Hook
	if core.SiteRE.MatchString(site) && !core.ProtectedSites[site] {
		h = a.Store.GetHook(site)
	}
	secret := []byte("tidak-ada-secret-dummy-untuk-waktu-sama")
	if h != nil && h.Enabled {
		if s, err := crypto.Open(a.Key, h.SecretEnc); err == nil {
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
	now := a.Now()
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
	a.Store.SetHookLast(site, now.Unix(), "berjalan")
	short := p.After
	if len(short) > 7 {
		short = short[:7]
	}
	short = regexp.MustCompile(`[^0-9a-f]`).ReplaceAllString(short, "")
	steps := strings.Join(h.Steps, ",")
	detail := site + " [" + steps + "] " + short
	id := a.Store.AddJob("webhook", "webhook_deploy "+detail, "berjalan", "", 0, now)
	a.Store.Audit("webhook", "webhook_deploy", detail+" -> mulai", ip, now)
	a.BG.Add(1)
	go func() {
		defer a.BG.Done()
		defer hookBusy.Delete(site)
		a.Mu.Lock()
		res := a.RunLabTimeout([]string{"deploy", "run", site, steps}, "", a.LongTimeout)
		a.Mu.Unlock()
		st := core.StatusOf(res)
		a.Store.FinishJob(id, st, res.Code, res.Out)
		a.Store.SetHookLast(site, a.Now().Unix(), st)
		a.Store.Audit("webhook", "webhook_deploy", detail+" -> "+st, ip, a.Now())
	}()
	hookReply(w, 202, map[string]any{"job": id, "status": "berjalan"})
}
