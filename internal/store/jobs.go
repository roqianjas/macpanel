package store

import (
	"time"
)

func (s *Store) FinishJob(id int64, status string, code int, out string) {
	_, _ = s.DB.Exec("UPDATE jobs SET status=?, code=?, output=? WHERE id=?", status, code, out, id)
}

// MarkStaleJobs menandai job yang masih "berjalan" saat panel baru start (prosesnya sudah mati bersama panel).
func (s *Store) MarkStaleJobs() {
	_, _ = s.DB.Exec("UPDATE jobs SET status='terputus' WHERE status='berjalan'")
}

func (s *Store) GetJob(id int64) *JobRow {
	var j JobRow
	if err := s.DB.QueryRow("SELECT id,at,usr,action,status,code,output FROM jobs WHERE id=?", id).Scan(&j.ID, &j.At, &j.User, &j.Action, &j.Status, &j.Code, &j.Output); err != nil {
		return nil
	}
	return &j
}

func (s *Store) AddJob(usr, action, status, output string, code int, now time.Time) int64 {
	r, err := s.DB.Exec("INSERT INTO jobs(at,usr,action,status,code,output) VALUES(?,?,?,?,?,?)", now.Unix(), usr, Clean(action, 200), status, code, output)
	if err != nil {
		return 0
	}
	id, _ := r.LastInsertId()
	return id
}

type JobRow struct {
	ID     int64  `json:"id"`
	At     int64  `json:"at"`
	User   string `json:"user"`
	Action string `json:"action"`
	Status string `json:"status"`
	Code   int    `json:"code"`
	Output string `json:"output"`
}

// ListJobs mengembalikan n job terbaru.
func (s *Store) ListJobs(n int) []JobRow {
	pg, _ := s.JobsPage(PageReq{Limit: n})
	return pg.Rows
}

// JobsPage: halaman riwayat job, terbaru dulu. Q mencari di user, aksi, dan status (bukan di keluaran).
func (s *Store) JobsPage(p PageReq) (Page[JobRow], error) {
	return paged(s, "jobs", "id,at,usr,action,status,code,output", []string{"usr", "action", "status"}, p,
		func(r interface{ Scan(...any) error }) (JobRow, int64, error) {
			var j JobRow
			err := r.Scan(&j.ID, &j.At, &j.User, &j.Action, &j.Status, &j.Code, &j.Output)
			return j, j.ID, err
		})
}

// PruneJobs membuang job selesai yang lebih tua dari before, menyisakan keepMin terbaru. Job "berjalan" tidak disentuh.
func (s *Store) PruneJobs(before int64, keepMin int) (int64, error) {
	if keepMin < 1 {
		keepMin = 1
	}
	r, err := s.DB.Exec(`DELETE FROM jobs WHERE at<? AND status<>'berjalan' AND id NOT IN (SELECT id FROM jobs ORDER BY id DESC LIMIT ?)`, before, keepMin)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}
