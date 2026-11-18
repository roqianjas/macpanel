package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/roqianjas/macpanel/internal/crypto"
)

const (
	MaxTokens   = 10
	TokenPrefix = "mpt_"
	DefaultName = "MacPanel"
)

var TokenScopes = map[string]int{"read": 1, "operate": 2, "full": 3}

type TokenRow struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Prefix   string `json:"prefix"`
	Scope    string `json:"scope"`
	Created  int64  `json:"created"`
	Expires  int64  `json:"expires"`
	LastUsed int64  `json:"last_used"`
	LastIP   string `json:"last_ip"`
	Owner    string `json:"owner,omitempty"`
}

// AddToken membuat token acak; hanya hash yang disimpan. Mengembalikan token utuh (tampil sekali) dan id.
func (s *Store) AddToken(uid int64, name, scope string, days int, now time.Time) (string, int64, bool) {
	_, _ = s.DB.Exec("DELETE FROM tokens WHERE expires<=?", now.Unix())
	var n int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM tokens WHERE user_id=?", uid).Scan(&n)
	if n >= MaxTokens {
		return "", 0, false
	}
	tok := TokenPrefix + crypto.RandToken()
	r, err := s.DB.Exec("INSERT INTO tokens(user_id,name,prefix,hash,scope,created,expires) VALUES(?,?,?,?,?,?,?)",
		uid, name, tok[:len(TokenPrefix)+4], crypto.Sha256hex(tok), scope, now.Unix(), now.Add(time.Duration(days)*24*time.Hour).Unix())
	if err != nil {
		return "", 0, false
	}
	id, _ := r.LastInsertId()
	return tok, id, true
}

func (s *Store) ListTokens(uid int64, now time.Time) []TokenRow {
	out := []TokenRow{}
	rows, err := s.DB.Query("SELECT id,name,prefix,scope,created,expires,last_used,last_ip FROM tokens WHERE user_id=? AND expires>? ORDER BY id DESC", uid, now.Unix())
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t TokenRow
		if rows.Scan(&t.ID, &t.Name, &t.Prefix, &t.Scope, &t.Created, &t.Expires, &t.LastUsed, &t.LastIP) != nil {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (s *Store) AllTokens(now time.Time) []TokenRow {
	out := []TokenRow{}
	rows, err := s.DB.Query("SELECT t.id,t.name,t.prefix,t.scope,t.created,t.expires,t.last_used,t.last_ip,u.name FROM tokens t JOIN users u ON u.id=t.user_id WHERE t.expires>? ORDER BY t.id", now.Unix())
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t TokenRow
		if rows.Scan(&t.ID, &t.Name, &t.Prefix, &t.Scope, &t.Created, &t.Expires, &t.LastUsed, &t.LastIP, &t.Owner) != nil {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (s *Store) RevokeToken(uid, id int64) bool {
	r, err := s.DB.Exec("DELETE FROM tokens WHERE id=? AND user_id=?", id, uid)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

func (s *Store) RevokeTokenAny(id int64) bool {
	r, err := s.DB.Exec("DELETE FROM tokens WHERE id=?", id)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

// TokenSession memeriksa token dan membentuk sesi sintetis (tanpa cookie, tanpa CSRF).
func (s *Store) TokenSession(tok, ip string, now time.Time) *Session {
	if len(tok) < 20 || len(tok) > 100 || !strings.HasPrefix(tok, TokenPrefix) {
		return nil
	}
	var id, uid, exp int64
	var user, role, scope, name string
	err := s.DB.QueryRow(`SELECT t.id,t.user_id,u.name,u.role,t.scope,t.expires,t.name FROM tokens t JOIN users u ON u.id=t.user_id
	  WHERE t.hash=? AND u.disabled=0`, crypto.Sha256hex(tok)).Scan(&id, &uid, &user, &role, &scope, &exp, &name)
	if err != nil || exp <= now.Unix() {
		return nil
	}
	if _, ok := TokenScopes[scope]; !ok {
		return nil
	}
	if role == "viewer" {
		scope = "read" // akun hanya-baca tidak pernah naik lewat token
	}
	_, _ = s.DB.Exec("UPDATE tokens SET last_used=?, last_ip=? WHERE id=?", now.Unix(), ip, id)
	return &Session{ID: fmt.Sprintf("token:%d", id), UserID: uid, User: user, Stage: "full", Role: role, Via: "token", Scope: scope, TokenName: name}
}

// RevokeUserTokens mencabut semua token API milik satu akun (dipakai saat peran akun diubah).
func (s *Store) RevokeUserTokens(uid int64) error {
	_, err := s.DB.Exec("DELETE FROM tokens WHERE user_id=?", uid)
	return err
}
