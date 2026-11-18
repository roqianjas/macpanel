package store

import (
	"time"

	"github.com/roqianjas/macpanel/internal/crypto"
)

func (s *Store) AddVersion(site, user, content, note string, now time.Time) {
	sha := crypto.Sha256hex(content)
	var last string
	_ = s.DB.QueryRow("SELECT sha FROM conf_versions WHERE site=? ORDER BY id DESC LIMIT 1", site).Scan(&last)
	if last == sha {
		return
	}
	_, _ = s.DB.Exec("INSERT INTO conf_versions(site,at,usr,sha,note,content) VALUES(?,?,?,?,?,?)", site, now.Unix(), user, sha, Clean(note, 120), content)
	_, _ = s.DB.Exec("DELETE FROM conf_versions WHERE site=? AND id NOT IN (SELECT id FROM conf_versions WHERE site=? ORDER BY id DESC LIMIT 20)", site, site)
}

type VersionRow struct {
	ID   int64  `json:"id"`
	At   int64  `json:"at"`
	User string `json:"user"`
	Note string `json:"note"`
}

func (s *Store) Versions(site string) []VersionRow {
	out := []VersionRow{}
	rows, err := s.DB.Query("SELECT id,at,usr,note FROM conf_versions WHERE site=? ORDER BY id DESC", site)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var v VersionRow
		if rows.Scan(&v.ID, &v.At, &v.User, &v.Note) != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (s *Store) VersionContent(site string, id int64) (string, bool) {
	var c string
	err := s.DB.QueryRow("SELECT content FROM conf_versions WHERE site=? AND id=?", site, id).Scan(&c)
	return c, err == nil
}
