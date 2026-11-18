package store

import (
	"fmt"
	"strings"
)

// Kontrak pagination untuk daftar yang disimpan di database (audit, jobs).
// Memakai cursor (id terakhir yang sudah dilihat), bukan offset: baris baru terus masuk di depan,
// dan offset akan menggeser halaman saat dibaca.

const (
	DefaultPageLimit = 25
	MaxPageLimit     = 200
)

// PageReq: Limit di luar 1..MaxPageLimit dipaksa ke batas; Cursor 0 = mulai dari yang terbaru;
// Q mencari (tanpa membedakan huruf besar/kecil) di kolom teks.
type PageReq struct {
	Limit  int
	Cursor int64
	Q      string
}

// Page: Next > 0 berarti masih ada halaman berikutnya (kirim sebagai cursor); Total = jumlah baris yang cocok Q.
type Page[T any] struct {
	Rows  []T   `json:"rows"`
	Next  int64 `json:"next"`
	Total int   `json:"total"`
}

func (p PageReq) limit() int {
	switch {
	case p.Limit <= 0:
		return DefaultPageLimit
	case p.Limit > MaxPageLimit:
		return MaxPageLimit
	}
	return p.Limit
}

// likePattern membuat pola LIKE aman: % _ dan \ dalam input diperlakukan sebagai huruf biasa.
func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.ToLower(Clean(q, 100))) + "%"
}

// where merakit klausa WHERE untuk kolom teks; cols adalah nama kolom tetap dari kode (bukan input pengguna).
func (p PageReq) where(cols ...string) (string, []any) {
	var conds []string
	var args []any
	if q := strings.TrimSpace(p.Q); q != "" {
		var ors []string
		pat := likePattern(q)
		for _, c := range cols {
			ors = append(ors, fmt.Sprintf("LOWER(%s) LIKE ? ESCAPE '\\'", c))
			args = append(args, pat)
		}
		conds = append(conds, "("+strings.Join(ors, " OR ")+")")
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// paged menjalankan pola yang sama untuk semua tabel berurut id menurun.
// scan membaca satu baris (id selalu kolom pertama) dan mengembalikan nilainya beserta id-nya.
func paged[T any](s *Store, table, cols string, textCols []string, p PageReq, scan func(rows interface{ Scan(...any) error }) (T, int64, error)) (Page[T], error) {
	out := Page[T]{Rows: []T{}}
	w, args := p.where(textCols...)
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM "+table+w, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	qargs := append([]any{}, args...)
	cond := w
	if p.Cursor > 0 {
		if cond == "" {
			cond = " WHERE id < ?"
		} else {
			cond += " AND id < ?"
		}
		qargs = append(qargs, p.Cursor)
	}
	lim := p.limit()
	qargs = append(qargs, lim+1) // satu baris ekstra untuk tahu apakah masih ada halaman berikutnya
	rows, err := s.DB.Query("SELECT "+cols+" FROM "+table+cond+" ORDER BY id DESC LIMIT ?", qargs...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var lastID int64
	for rows.Next() {
		v, id, err := scan(rows)
		if err != nil {
			return out, err
		}
		if len(out.Rows) == lim {
			out.Next = lastID
			break
		}
		out.Rows = append(out.Rows, v)
		lastID = id
	}
	return out, rows.Err()
}
