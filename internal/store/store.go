package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roqianjas/macpanel/internal/crypto"
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
CREATE TABLE IF NOT EXISTS jobs(id INTEGER PRIMARY KEY, at INTEGER NOT NULL, usr TEXT NOT NULL, action TEXT NOT NULL,
  status TEXT NOT NULL, code INTEGER NOT NULL, output TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS conf_versions(id INTEGER PRIMARY KEY, site TEXT NOT NULL, at INTEGER NOT NULL, usr TEXT NOT NULL, sha TEXT NOT NULL, note TEXT NOT NULL, content TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS cv_site ON conf_versions(site, id);
CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY, at INTEGER NOT NULL, usr TEXT NOT NULL, action TEXT NOT NULL,
  detail TEXT NOT NULL, ip TEXT NOT NULL, prev TEXT NOT NULL, hash TEXT NOT NULL);
` + hooksSchema + trustSchema + settingsSchema

const (
	IdleTTL   = 30 * time.Minute
	AbsTTL    = 12 * time.Hour
	PwdTTL    = 5 * time.Minute
	StepupTTL = 15 * time.Minute
)

type Store struct{ DB *sql.DB }

type Session struct {
	ID       string
	UserID   int64
	User     string
	Stage    string
	CSRF     string
	LastSeen int64
	Expires  int64
	StepUp   int64
	Role     string
	// sesi token API (tanpa cookie): Via="token", Scope read|operate|full
	MustChange bool
	Via        string
	Scope      string
	TokenName  string
}

type User struct {
	ID         int64
	Name       string
	PassHash   string
	TOTPEnc    []byte
	TOTPLast   int64
	Disabled   bool
	TOTPOn     bool
	MustChange bool
}

func OpenStore(path string) (*Store, error) {
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
	_ = f.Close()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(netqSchema); err != nil {
		return nil, err
	}
	if _, err := db.Exec(hwSchema); err != nil {
		return nil, err
	}
	if _, err := db.Exec(metricsSchema); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	for _, m := range []string{
		"ALTER TABLE users ADD COLUMN must_change INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN totp_on INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE users ADD COLUMN totp_pending BLOB",
		"ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'admin'",
	} {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return nil, err
		}
	}
	return &Store{db}, nil
}

func (s *Store) NewSession(uid int64, stage string, now time.Time) (string, *Session) {
	tok := crypto.RandToken()
	ttl := AbsTTL
	if stage == "pwd" {
		ttl = PwdTTL
	}
	ss := &Session{ID: crypto.Sha256hex(tok), UserID: uid, Stage: stage, CSRF: crypto.RandToken(), LastSeen: now.Unix(), Expires: now.Add(ttl).Unix()}
	_, _ = s.DB.Exec("INSERT INTO sessions(id,user_id,stage,csrf,created,last_seen,expires) VALUES(?,?,?,?,?,?,?)",
		ss.ID, uid, stage, ss.CSRF, now.Unix(), now.Unix(), ss.Expires)
	return tok, ss
}

func (s *Store) GetSession(tok string, now time.Time) *Session {
	if tok == "" || len(tok) > 100 {
		return nil
	}
	id := crypto.Sha256hex(tok)
	ss := &Session{ID: id}
	err := s.DB.QueryRow(`SELECT s.user_id,u.name,s.stage,s.csrf,s.last_seen,s.expires,s.stepup,u.role,u.must_change FROM sessions s JOIN users u ON u.id=s.user_id
	  WHERE s.id=? AND u.disabled=0`, id).Scan(&ss.UserID, &ss.User, &ss.Stage, &ss.CSRF, &ss.LastSeen, &ss.Expires, &ss.StepUp, &ss.Role, &ss.MustChange)
	if err != nil {
		return nil
	}
	n := now.Unix()
	if n >= ss.Expires || (ss.Stage == "full" && n-ss.LastSeen >= int64(IdleTTL.Seconds())) {
		s.DelSession(id)
		return nil
	}
	_, _ = s.DB.Exec("UPDATE sessions SET last_seen=? WHERE id=?", n, id)
	return ss
}

func (s *Store) DelSession(id string) { _, _ = s.DB.Exec("DELETE FROM sessions WHERE id=?", id) }

func (s *Store) SetStepUp(id string, until time.Time) {
	_, _ = s.DB.Exec("UPDATE sessions SET stepup=? WHERE id=?", until.Unix(), id)
}

func (s *Store) AddAttempt(k string, now time.Time) {
	_, _ = s.DB.Exec("DELETE FROM attempts WHERE at<?", now.Add(-time.Hour).Unix())
	_, _ = s.DB.Exec("INSERT INTO attempts(k,at) VALUES(?,?)", k, now.Unix())
}

func (s *Store) CountAttempts(k string, since time.Time) int {
	var n int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM attempts WHERE k=? AND at>=?", k, since.Unix()).Scan(&n)
	return n
}

func (s *Store) ClearAttempts(k string) { _, _ = s.DB.Exec("DELETE FROM attempts WHERE k=?", k) }
