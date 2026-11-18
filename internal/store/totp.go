package store

import (
	"strings"
	"time"

	"github.com/roqianjas/macpanel/internal/crypto"
)

func (s *Store) SetTOTPPending(uid int64, enc []byte) {
	_, _ = s.DB.Exec("UPDATE users SET totp_pending=? WHERE id=?", enc, uid)
}

func (s *Store) GetTOTPPending(uid int64) []byte {
	var b []byte
	_ = s.DB.QueryRow("SELECT totp_pending FROM users WHERE id=?", uid).Scan(&b)
	return b
}

func (s *Store) RecoveryLeft(uid int64) int {
	var n int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM recovery WHERE user_id=? AND used IS NULL", uid).Scan(&n)
	return n
}

// EnableTOTP memasang secret baru, mengganti recovery code, dan mencabut perangkat tepercaya serta sesi lain.
func (s *Store) EnableTOTP(uid int64, enc []byte, recHashes []string, step int64, keepSession string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET totp_enc=?, totp_on=1, totp_pending=NULL, totp_last=? WHERE id=?", enc, step, uid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM recovery WHERE user_id=?", uid); err != nil {
		return err
	}
	for _, h := range recHashes {
		if _, err := tx.Exec("INSERT INTO recovery(user_id,hash) VALUES(?,?)", uid, h); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=? AND id<>?", uid, keepSession); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM trusted WHERE user_id=?", uid); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DisableTOTP(uid int64, keepSession string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET totp_on=0, totp_pending=NULL WHERE id=?", uid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM recovery WHERE user_id=?", uid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=? AND id<>?", uid, keepSession); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM trusted WHERE user_id=?", uid); err != nil {
		return err
	}
	return tx.Commit()
}

// UseTOTPStep: atomik, hanya berhasil bila step lebih besar dari yang tercatat (anti-replay, aman terhadap race).
func (s *Store) UseTOTPStep(uid, step int64) bool {
	r, err := s.DB.Exec("UPDATE users SET totp_last=? WHERE id=? AND totp_last<?", step, uid, step)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

func (s *Store) UseRecovery(uid int64, code string) bool {
	r, err := s.DB.Exec("UPDATE recovery SET used=? WHERE user_id=? AND hash=? AND used IS NULL", time.Now().Unix(), uid, crypto.Sha256hex(strings.ToLower(strings.TrimSpace(code))))
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}
