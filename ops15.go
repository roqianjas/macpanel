package main

import (
	"net/http"
	"strings"
	"time"
)

// Perangkat tepercaya: setelah login lengkap (kata sandi + 2FA) dengan centang "percayai perangkat",
// perangkat itu cukup kata sandi selama trustTTL. Aksi berbahaya tetap minta 2FA ulang (step-up),
// karena sesi baru selalu mulai tanpa step-up.
const (
	trustCookie = "__Host-panel-trust"
	trustTTL    = 30 * 24 * time.Hour
	maxTrusted  = 10
)

const trustSchema = `
CREATE TABLE IF NOT EXISTS trusted(id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  hash TEXT UNIQUE NOT NULL, label TEXT NOT NULL, created INTEGER NOT NULL, last_used INTEGER NOT NULL, expires INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS trusted_user ON trusted(user_id);
`

type TrustRow struct {
	ID       int64  `json:"id"`
	Label    string `json:"label"`
	Created  int64  `json:"created"`
	LastUsed int64  `json:"last_used"`
	Expires  int64  `json:"expires"`
	Current  bool   `json:"current"`
}

// AddTrusted membuat token perangkat baru (hanya hash yang disimpan). Perangkat terlama dibuang bila melebihi batas.
func (s *Store) AddTrusted(uid int64, label string, now time.Time) string {
	tok := randToken()
	_, _ = s.db.Exec("DELETE FROM trusted WHERE expires<=?", now.Unix())
	_, _ = s.db.Exec("INSERT INTO trusted(user_id,hash,label,created,last_used,expires) VALUES(?,?,?,?,?,?)",
		uid, sha256hex(tok), label, now.Unix(), now.Unix(), now.Add(trustTTL).Unix())
	_, _ = s.db.Exec("DELETE FROM trusted WHERE user_id=? AND id NOT IN (SELECT id FROM trusted WHERE user_id=? ORDER BY id DESC LIMIT ?)", uid, uid, maxTrusted)
	return tok
}

// UseTrusted memeriksa token milik user itu dan menggantinya dengan token baru (rotasi); masa berlaku tetap.
func (s *Store) UseTrusted(tok string, uid int64, now time.Time) (string, bool) {
	if tok == "" || len(tok) > 100 {
		return "", false
	}
	nt := randToken()
	r, err := s.db.Exec("UPDATE trusted SET hash=?, last_used=? WHERE hash=? AND user_id=? AND expires>?", sha256hex(nt), now.Unix(), sha256hex(tok), uid, now.Unix())
	if err != nil {
		return "", false
	}
	n, _ := r.RowsAffected()
	return nt, n == 1
}

func (s *Store) ListTrusted(uid int64, curTok string, now time.Time) []TrustRow {
	out := []TrustRow{}
	rows, err := s.db.Query("SELECT id,hash,label,created,last_used,expires FROM trusted WHERE user_id=? AND expires>? ORDER BY last_used DESC", uid, now.Unix())
	if err != nil {
		return out
	}
	defer rows.Close()
	cur := ""
	if curTok != "" && len(curTok) <= 100 {
		cur = sha256hex(curTok)
	}
	for rows.Next() {
		var t TrustRow
		var h string
		if rows.Scan(&t.ID, &h, &t.Label, &t.Created, &t.LastUsed, &t.Expires) == nil {
			t.Current = cur != "" && h == cur
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) RevokeTrusted(uid, id int64) bool {
	r, err := s.db.Exec("DELETE FROM trusted WHERE id=? AND user_id=?", id, uid)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

func (s *Store) RevokeAllTrusted(uid int64) {
	_, _ = s.db.Exec("DELETE FROM trusted WHERE user_id=?", uid)
}

func (a *App) setTrust(w http.ResponseWriter, tok string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: trustCookie, Value: tok, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

// deviceLabel meringkas User-Agent menjadi label pendek (bukan data identitas).
func deviceLabel(ua string) string {
	l := strings.ToLower(ua)
	br := "Browser"
	for _, b := range [][2]string{{"edg/", "Edge"}, {"firefox", "Firefox"}, {"chrome", "Chrome"}, {"safari", "Safari"}} {
		if strings.Contains(l, b[0]) {
			br = b[1]
			break
		}
	}
	os := "perangkat tidak dikenal"
	for _, o := range [][2]string{{"iphone", "iPhone"}, {"android", "Android"}, {"windows", "Windows"}, {"mac os", "macOS"}, {"linux", "Linux"}} {
		if strings.Contains(l, o[0]) {
			os = o[1]
			break
		}
	}
	return br + " di " + os
}

func hDevices(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	cur := ""
	if c, err := r.Cookie(trustCookie); err == nil {
		cur = c.Value
	}
	jsonOut(w, 200, map[string]any{"devices": a.st.ListTrusted(ss.UserID, cur, a.now()), "days": int(trustTTL.Hours() / 24)})
}

func hDeviceRevoke(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct {
		ID  int64 `json:"id"`
		All bool  `json:"all"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.All {
		a.st.RevokeAllTrusted(ss.UserID)
		a.st.Audit(ss.User, "trust_revoke_all", "", a.clientIP(r), a.now())
		a.setTrust(w, "", -1)
		jsonOut(w, 200, map[string]bool{"ok": true})
		return
	}
	if !a.st.RevokeTrusted(ss.UserID, in.ID) {
		jerr(w, 404, "perangkat tidak ditemukan")
		return
	}
	a.st.Audit(ss.User, "trust_revoke", "", a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]bool{"ok": true})
}
