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
	anchor := s.Setting(auditAnchorKey, "") // dibaca sebelum query dibuka: koneksi database hanya satu
	rows, err := s.DB.Query("SELECT id,at,usr,action,detail,ip,prev,hash FROM audit ORDER BY id")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	prev, n := anchor, 0
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
	ID     int64  `json:"id"`
	At     int64  `json:"at"`
	User   string `json:"user"`
	Action string `json:"action"`
	Detail string `json:"detail"`
	IP     string `json:"ip"`
}

// RecentAudit mengembalikan n baris terbaru (dipakai tes dan ringkasan).
func (s *Store) RecentAudit(n int) []AuditRow {
	pg, _ := s.AuditPage(PageReq{Limit: n})
	return pg.Rows
}

// AuditPage: halaman audit, terbaru dulu. Q mencari di user, aksi, detail, dan IP.
func (s *Store) AuditPage(p PageReq) (Page[AuditRow], error) {
	return paged(s, "audit", "id,at,usr,action,detail,ip", []string{"usr", "action", "detail", "ip"}, p,
		func(r interface{ Scan(...any) error }) (AuditRow, int64, error) {
			var a AuditRow
			err := r.Scan(&a.ID, &a.At, &a.User, &a.Action, &a.Detail, &a.IP)
			return a, a.ID, err
		})
}

const auditAnchorKey = "audit_anchor"

// PruneAudit membuang baris audit lebih tua dari before, tetapi selalu menyisakan keepMin baris terbaru.
// Rantai hash tetap dapat diverifikasi: hash baris terakhir yang dibuang disimpan sebagai jangkar (audit_anchor)
// dalam transaksi yang sama, dan VerifyAudit mulai dari jangkar itu. Mengembalikan jumlah baris yang dibuang.
func (s *Store) PruneAudit(before int64, keepMin int) (int64, error) {
	if keepMin < 1 {
		keepMin = 1
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var cut sql.NullInt64
	// kandidat: lebih tua dari before DAN di luar keepMin baris terbaru; cut = id kandidat terbesar (awalan yang berurutan)
	if err := tx.QueryRow(`SELECT MAX(id) FROM audit WHERE at<? AND id NOT IN (SELECT id FROM audit ORDER BY id DESC LIMIT ?)`, before, keepMin).Scan(&cut); err != nil {
		return 0, err
	}
	if !cut.Valid {
		return 0, nil
	}
	var anchor string
	if err := tx.QueryRow("SELECT hash FROM audit WHERE id=?", cut.Int64).Scan(&anchor); err != nil {
		return 0, err
	}
	if _, err := tx.Exec("INSERT INTO settings(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v", auditAnchorKey, anchor); err != nil {
		return 0, err
	}
	r, err := tx.Exec("DELETE FROM audit WHERE id<=?", cut.Int64)
	if err != nil {
		return 0, err
	}
	n, _ := r.RowsAffected()
	return n, tx.Commit()
}
