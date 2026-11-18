package store

import (
	"time"

	"github.com/roqianjas/macpanel/internal/crypto"
)

// Perangkat tepercaya: setelah login lengkap (kata sandi + 2FA) dengan centang "percayai perangkat",
// perangkat itu cukup kata sandi selama trustTTL. Aksi berbahaya tetap minta 2FA ulang (step-up),
// karena sesi baru selalu mulai tanpa step-up.
const (
	TrustCookie = "__Host-panel-trust"
	TrustTTL    = 30 * 24 * time.Hour
	MaxTrusted  = 10
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
	tok := crypto.RandToken()
	_, _ = s.DB.Exec("DELETE FROM trusted WHERE expires<=?", now.Unix())
	_, _ = s.DB.Exec("INSERT INTO trusted(user_id,hash,label,created,last_used,expires) VALUES(?,?,?,?,?,?)",
		uid, crypto.Sha256hex(tok), label, now.Unix(), now.Unix(), now.Add(TrustTTL).Unix())
	_, _ = s.DB.Exec("DELETE FROM trusted WHERE user_id=? AND id NOT IN (SELECT id FROM trusted WHERE user_id=? ORDER BY id DESC LIMIT ?)", uid, uid, MaxTrusted)
	return tok
}

// UseTrusted memeriksa token milik user itu dan menggantinya dengan token baru (rotasi); masa berlaku tetap.
func (s *Store) UseTrusted(tok string, uid int64, now time.Time) (string, bool) {
	if tok == "" || len(tok) > 100 {
		return "", false
	}
	nt := crypto.RandToken()
	r, err := s.DB.Exec("UPDATE trusted SET hash=?, last_used=? WHERE hash=? AND user_id=? AND expires>?", crypto.Sha256hex(nt), now.Unix(), crypto.Sha256hex(tok), uid, now.Unix())
	if err != nil {
		return "", false
	}
	n, _ := r.RowsAffected()
	return nt, n == 1
}

func (s *Store) ListTrusted(uid int64, curTok string, now time.Time) []TrustRow {
	out := []TrustRow{}
	rows, err := s.DB.Query("SELECT id,hash,label,created,last_used,expires FROM trusted WHERE user_id=? AND expires>? ORDER BY last_used DESC", uid, now.Unix())
	if err != nil {
		return out
	}
	defer rows.Close()
	cur := ""
	if curTok != "" && len(curTok) <= 100 {
		cur = crypto.Sha256hex(curTok)
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
	r, err := s.DB.Exec("DELETE FROM trusted WHERE id=? AND user_id=?", id, uid)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

func (s *Store) RevokeAllTrusted(uid int64) {
	_, _ = s.DB.Exec("DELETE FROM trusted WHERE user_id=?", uid)
}
