package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/roqianjas/macpanel/internal/crypto"
)

func auditHash(prev string, at int64, usr, action, detail, ip string) string {
	return crypto.Sha256hex(fmt.Sprintf("%s|%d|%s|%s|%s|%s", prev, at, usr, action, detail, ip))
}

func Clean(s string, n int) string {
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
	tx, err := s.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	var prev string
	if err := tx.QueryRow("SELECT hash FROM audit ORDER BY id DESC LIMIT 1").Scan(&prev); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}
	usr, action, detail, ip = Clean(usr, 40), Clean(action, 40), Clean(detail, 200), Clean(ip, 45)
	at := now.Unix()
	if _, err := tx.Exec("INSERT INTO audit(at,usr,action,detail,ip,prev,hash) VALUES(?,?,?,?,?,?,?)", at, usr, action, detail, ip, prev, auditHash(prev, at, usr, action, detail, ip)); err != nil {
		log.Printf("audit gagal ditulis: %v", err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("audit gagal di-commit: %v", err)
	}
}

// VerifyAudit memeriksa rantai; mengembalikan jumlah baris valid dan galat bila ada yang diubah/dihapus.
func (s *Store) VerifyAudit() (int, error) {
	rows, err := s.DB.Query("SELECT id,at,usr,action,detail,ip,prev,hash FROM audit ORDER BY id")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	prev, n := "", 0
	for rows.Next() {
		var id, at int64
		var usr, action, detail, ip, p, h string
		if rows.Scan(&id, &at, &usr, &action, &detail, &ip, &p, &h) != nil {
			continue
		}
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
	rows, err := s.DB.Query("SELECT at,usr,action,detail,ip FROM audit ORDER BY id DESC LIMIT ?", n)
	out := []AuditRow{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var r AuditRow
		if rows.Scan(&r.At, &r.User, &r.Action, &r.Detail, &r.IP) != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}
