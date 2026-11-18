package store

import (
	"time"
)

type AccountRow struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Disabled   bool   `json:"disabled"`
	TOTPOn     bool   `json:"totp_on"`
	MustChange bool   `json:"must_change"`
	Created    int64  `json:"created"`
	Self       bool   `json:"self"`
}

func (s *Store) AccountRows(self string) []AccountRow {
	out := []AccountRow{}
	rows, err := s.DB.Query("SELECT name,role,disabled,totp_on,must_change,created FROM users ORDER BY name")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var r AccountRow
		var d, t, m int
		if rows.Scan(&r.Name, &r.Role, &d, &t, &m, &r.Created) != nil {
			continue
		}
		r.Disabled, r.TOTPOn, r.MustChange, r.Self = d == 1, t == 1, m == 1, r.Name == self
		out = append(out, r)
	}
	return out
}

func (s *Store) CreateAccount(name, passHash string, totpEnc []byte, role string, now int64) bool {
	_, err := s.DB.Exec("INSERT INTO users(name,pass_hash,totp_enc,created,role,totp_on,must_change) VALUES(?,?,?,?,?,0,1)", name, passHash, totpEnc, now, role)
	return err == nil
}

// SetTempPassword: kata sandi sementara dari admin; wajib diganti saat masuk. Semua sesi, perangkat tepercaya, dan token dicabut.
func (s *Store) SetTempPassword(uid int64, hash string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET pass_hash=?, must_change=1 WHERE id=?", hash, uid); err != nil {
		return err
	}
	for _, q := range []string{"DELETE FROM sessions WHERE user_id=?", "DELETE FROM trusted WHERE user_id=?", "DELETE FROM tokens WHERE user_id=?"} {
		if _, err := tx.Exec(q, uid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetDisabled(name string, off bool) bool {
	v := 0
	if off {
		v = 1
	}
	r, err := s.DB.Exec("UPDATE users SET disabled=? WHERE name=?", v, name)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	if n == 1 && off {
		_, _ = s.DB.Exec("DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE name=?)", name)
		_, _ = s.DB.Exec("DELETE FROM trusted WHERE user_id=(SELECT id FROM users WHERE name=?)", name)
	}
	return n == 1
}

func (s *Store) AdminsExcept(name string) int {
	var n int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin' AND disabled=0 AND name<>?", name).Scan(&n) // gagal baca = n 0 = dianggap tidak ada admin lain (aman: penjaga menolak)
	return n
}

func (s *Store) CreateUser(name, passHash string, totpEnc []byte, recoveryHashes []string) error {
	tx, err := s.DB.Begin()
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
	var dis, on, mc int
	err := s.DB.QueryRow("SELECT id,name,pass_hash,totp_enc,totp_last,disabled,totp_on,must_change FROM users WHERE name=?", name).
		Scan(&u.ID, &u.Name, &u.PassHash, &u.TOTPEnc, &u.TOTPLast, &dis, &on, &mc)
	if err != nil {
		return nil
	}
	u.Disabled = dis == 1
	u.TOTPOn = on == 1
	u.MustChange = mc == 1
	return u
}

type UserRow struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
	Created  int64  `json:"created"`
}

func (s *Store) UserRows() []UserRow {
	rows, err := s.DB.Query("SELECT name,role,disabled,created FROM users ORDER BY name")
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []UserRow{}
	for rows.Next() {
		var u UserRow
		var dis int
		if rows.Scan(&u.Name, &u.Role, &dis, &u.Created) != nil {
			continue
		}
		u.Disabled = dis == 1
		out = append(out, u)
	}
	return out
}

// SetRole mengubah peran user dan mencabut sesinya agar peran baru langsung berlaku. Hanya "admin" atau "viewer".
func (s *Store) SetRole(name, role string) bool {
	if role != "admin" && role != "viewer" {
		return false
	}
	r, err := s.DB.Exec("UPDATE users SET role=? WHERE name=?", role, name)
	if err != nil {
		return false
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return false
	}
	_, _ = s.DB.Exec("DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE name=?)", name)
	return true
}

func (s *Store) ListUsers() []string {
	rows, err := s.DB.Query("SELECT name FROM users ORDER BY name")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

// DeleteUser menghapus user beserta sesi dan recovery code-nya (cascade). Mengembalikan false bila tidak ada.
func (s *Store) DeleteUser(name string) bool {
	r, err := s.DB.Exec("DELETE FROM users WHERE name=?", name)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n == 1
}

// SetPassword mengganti hash sandi dan mencabut semua sesi user kecuali sesi yang sedang dipakai.
func (s *Store) SetPassword(uid int64, hash, keepSession string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET pass_hash=?, must_change=0 WHERE id=?", hash, uid); err != nil {
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

// RenameUser mengganti nama login. Kata sandi, 2FA, recovery code, dan perangkat tepercaya tetap; sesi aktif dicabut.
func (s *Store) RenameUser(oldName, newName string) bool {
	tx, err := s.DB.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var uid int64
	if tx.QueryRow("SELECT id FROM users WHERE name=?", oldName).Scan(&uid) != nil {
		return false
	}
	if _, err := tx.Exec("UPDATE users SET name=? WHERE id=?", newName, uid); err != nil {
		return false
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=?", uid); err != nil {
		return false
	}
	return tx.Commit() == nil
}
