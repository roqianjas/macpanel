package store

import (
	"strings"
)

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
	if err := s.DB.QueryRow("SELECT secret_enc,branch,steps,enabled,last_at,last_result FROM hooks WHERE site=?", site).Scan(&h.SecretEnc, &h.Branch, &steps, &en, &h.LastAt, &h.LastResult); err != nil {
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
	_, err := s.DB.Exec(`INSERT INTO hooks(site,secret_enc,branch,steps,enabled) VALUES(?,?,?,?,?)
		ON CONFLICT(site) DO UPDATE SET secret_enc=excluded.secret_enc, branch=excluded.branch, steps=excluded.steps, enabled=excluded.enabled`,
		h.Site, h.SecretEnc, h.Branch, strings.Join(h.Steps, ","), en)
	return err
}

func (s *Store) SetHookLast(site string, at int64, res string) {
	_, _ = s.DB.Exec("UPDATE hooks SET last_at=?, last_result=? WHERE site=?", at, Clean(res, 80), site)
}

func (s *Store) DeleteHook(site string) bool {
	r, err := s.DB.Exec("DELETE FROM hooks WHERE site=?", site)
	n, _ := r.RowsAffected()
	return err == nil && n > 0
}
