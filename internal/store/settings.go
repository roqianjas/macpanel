package store

const settingsSchema = `
CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY, v TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tokens(id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL, prefix TEXT NOT NULL, hash TEXT UNIQUE NOT NULL, scope TEXT NOT NULL, created INTEGER NOT NULL,
  expires INTEGER NOT NULL, last_used INTEGER NOT NULL DEFAULT 0, last_ip TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS tokens_user ON tokens(user_id);
`

func (s *Store) Setting(k, def string) string {
	var v string
	if s.DB.QueryRow("SELECT v FROM settings WHERE k=?", k).Scan(&v) != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(k, v string) {
	_, _ = s.DB.Exec("INSERT INTO settings(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v", k, v)
}
