package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY, name TEXT UNIQUE NOT NULL, pass_hash TEXT NOT NULL,
  totp_enc BLOB NOT NULL, totp_last INTEGER NOT NULL DEFAULT 0, disabled INTEGER NOT NULL DEFAULT 0, created INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS recovery(id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  hash TEXT NOT NULL, used INTEGER);
CREATE INDEX IF NOT EXISTS rec_idx ON recovery(user_id, hash);
CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  stage TEXT NOT NULL, csrf TEXT NOT NULL, created INTEGER NOT NULL, last_seen INTEGER NOT NULL, expires INTEGER NOT NULL,
  stepup INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS attempts(id INTEGER PRIMARY KEY, k TEXT NOT NULL, at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS att_k ON attempts(k, at);
CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY, at INTEGER NOT NULL, usr TEXT NOT NULL, action TEXT NOT NULL,
  detail TEXT NOT NULL, ip TEXT NOT NULL, prev TEXT NOT NULL, hash TEXT NOT NULL);
`

const (
	idleTTL   = 30 * time.Minute
	absTTL    = 12 * time.Hour
	pwdTTL    = 5 * time.Minute
	stepupTTL = 5 * time.Minute
)

type Store struct{ db *sql.DB }

type Session struct {
	ID       string
	UserID   int64
	User     string
	Stage    string
	CSRF     string
	LastSeen int64
	Expires  int64
	StepUp   int64
}

type User struct {
	ID       int64
	Name     string
	PassHash string
	TOTPEnc  []byte
	TOTPLast int64
	Disabled bool
}

func openStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	f.Close()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db}, nil
}

// ---------- user ----------

func (s *Store) CreateUser(name, passHash string, totpEnc []byte, recoveryHashes []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec("INSERT INTO users(name,pass_hash,totp_enc,created) VALUES(?,?,?,?)", name, passHash, totpEnc, time.Now().Unix())
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	for _, h := range recoveryHashes {
		if _, err := tx.Exec("INSERT INTO recovery(user_id,hash) VALUES(?,?)", id, h); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetUser(name string) *User {
	u := &User{}
	var dis int
	err := s.db.QueryRow("SELECT id,name,pass_hash,totp_enc,totp_last,disabled FROM users WHERE name=?", name).
		Scan(&u.ID, &u.Name, &u.PassHash, &u.TOTPEnc, &u.TOTPLast, &dis)
	if err != nil {
		return nil
	}
	u.Disabled = dis == 1
	return u
}

func (s *Store) ListUsers() []string {
	rows, err := s.db.Query("SELECT name FROM users ORDER BY name")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		out = append(out, n)
	}
	return out
}

// UseTOTPStep: atomik, hanya berhasil bila step lebih besar dari yang tercatat (anti-replay, aman terhadap race).
func (s *Store) UseTOTPStep(uid, step int64) bool {
	r, err := s.db.Exec("UPDATE users SET totp_last=? WHERE id=? AND totp_last<?", step, uid, step)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

func (s *Store) UseRecovery(uid int64, code string) bool {
	r, err := s.db.Exec("UPDATE recovery SET used=? WHERE user_id=? AND hash=? AND used IS NULL", time.Now().Unix(), uid, sha256hex(strings.ToLower(strings.TrimSpace(code))))
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

// ---------- sesi ----------

func (s *Store) NewSession(uid int64, stage string, now time.Time) (string, *Session) {
	tok := randToken()
	ttl := absTTL
	if stage == "pwd" {
		ttl = pwdTTL
	}
	ss := &Session{ID: sha256hex(tok), UserID: uid, Stage: stage, CSRF: randToken(), LastSeen: now.Unix(), Expires: now.Add(ttl).Unix()}
	s.db.Exec("INSERT INTO sessions(id,user_id,stage,csrf,created,last_seen,expires) VALUES(?,?,?,?,?,?,?)",
		ss.ID, uid, stage, ss.CSRF, now.Unix(), now.Unix(), ss.Expires)
	return tok, ss
}

func (s *Store) GetSession(tok string, now time.Time) *Session {
	if tok == "" || len(tok) > 100 {
		return nil
	}
	id := sha256hex(tok)
	ss := &Session{ID: id}
	err := s.db.QueryRow(`SELECT s.user_id,u.name,s.stage,s.csrf,s.last_seen,s.expires,s.stepup FROM sessions s JOIN users u ON u.id=s.user_id
	  WHERE s.id=? AND u.disabled=0`, id).Scan(&ss.UserID, &ss.User, &ss.Stage, &ss.CSRF, &ss.LastSeen, &ss.Expires, &ss.StepUp)
	if err != nil {
		return nil
	}
	n := now.Unix()
	if n >= ss.Expires || (ss.Stage == "full" && n-ss.LastSeen >= int64(idleTTL.Seconds())) {
		s.DelSession(id)
		return nil
	}
	s.db.Exec("UPDATE sessions SET last_seen=? WHERE id=?", n, id)
	return ss
}

func (s *Store) DelSession(id string) { s.db.Exec("DELETE FROM sessions WHERE id=?", id) }

func (s *Store) SetStepUp(id string, until time.Time) {
	s.db.Exec("UPDATE sessions SET stepup=? WHERE id=?", until.Unix(), id)
}

// ---------- percobaan login (rate limit / lockout) ----------

func (s *Store) AddAttempt(k string, now time.Time) {
	s.db.Exec("DELETE FROM attempts WHERE at<?", now.Add(-time.Hour).Unix())
	s.db.Exec("INSERT INTO attempts(k,at) VALUES(?,?)", k, now.Unix())
}

func (s *Store) CountAttempts(k string, since time.Time) int {
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM attempts WHERE k=? AND at>=?", k, since.Unix()).Scan(&n)
	return n
}

func (s *Store) ClearAttempts(k string) { s.db.Exec("DELETE FROM attempts WHERE k=?", k) }

// ---------- audit berantai ----------

func auditHash(prev string, at int64, usr, action, detail, ip string) string {
	return sha256hex(fmt.Sprintf("%s|%d|%s|%s|%s|%s", prev, at, usr, action, detail, ip))
}

func clean(s string, n int) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r != 127 {
			b.WriteRune(r)
		}
	}
	o := b.String()
	if len(o) > n {
		o = o[:n]
	}
	return o
}

func (s *Store) Audit(usr, action, detail, ip string, now time.Time) {
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	var prev string
	if err := tx.QueryRow("SELECT hash FROM audit ORDER BY id DESC LIMIT 1").Scan(&prev); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}
	usr, action, detail, ip = clean(usr, 40), clean(action, 40), clean(detail, 200), clean(ip, 45)
	at := now.Unix()
	tx.Exec("INSERT INTO audit(at,usr,action,detail,ip,prev,hash) VALUES(?,?,?,?,?,?,?)", at, usr, action, detail, ip, prev, auditHash(prev, at, usr, action, detail, ip))
	tx.Commit()
}

// VerifyAudit memeriksa rantai; mengembalikan jumlah baris valid dan galat bila ada yang diubah/dihapus.
func (s *Store) VerifyAudit() (int, error) {
	rows, err := s.db.Query("SELECT id,at,usr,action,detail,ip,prev,hash FROM audit ORDER BY id")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	prev, n := "", 0
	for rows.Next() {
		var id, at int64
		var usr, action, detail, ip, p, h string
		rows.Scan(&id, &at, &usr, &action, &detail, &ip, &p, &h)
		if p != prev || h != auditHash(p, at, usr, action, detail, ip) {
			return n, fmt.Errorf("rantai audit rusak di baris id=%d", id)
		}
		prev = h
		n++
	}
	return n, nil
}

type AuditRow struct {
	At     int64  `json:"at"`
	User   string `json:"user"`
	Action string `json:"action"`
	Detail string `json:"detail"`
	IP     string `json:"ip"`
}

func (s *Store) RecentAudit(n int) []AuditRow {
	rows, err := s.db.Query("SELECT at,usr,action,detail,ip FROM audit ORDER BY id DESC LIMIT ?", n)
	out := []AuditRow{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var r AuditRow
		rows.Scan(&r.At, &r.User, &r.Action, &r.Detail, &r.IP)
		out = append(out, r)
	}
	return out
}

// DeleteUser menghapus user beserta sesi dan recovery code-nya (cascade). Mengembalikan false bila tidak ada.
func (s *Store) DeleteUser(name string) bool {
	r, err := s.db.Exec("DELETE FROM users WHERE name=?", name)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}
