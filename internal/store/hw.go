package store

import (
	"database/sql"
	"math"
)

const hwSchema = `
CREATE TABLE IF NOT EXISTS hw(t INTEGER PRIMARY KEY, cpu REAL, gpu REAL, ssd REAL, memt REAL, ambient REAL,
  fan REAL, power REAL, press REAL, swap REAL, src TEXT NOT NULL DEFAULT '', throttled INTEGER NOT NULL DEFAULT 0);
`

// HWPoint: satu sampel perangkat keras. Nilai nil = tidak terbaca (disimpan NULL, bukan 0, supaya grafik tidak berbohong).
type HWPoint struct {
	CPU, GPU, SSD, Mem, Ambient *float64 // suhu, °C
	Fan                         *float64 // rpm
	Power                       *float64 // watt, daya sistem
	Press                       *float64 // tekanan memori, %
	Swap                        *float64 // swap terpakai, MB
	Src                         string
	Throttled                   bool
}

func (s *Store) AddHW(t int64, p HWPoint) {
	th := 0
	if p.Throttled {
		th = 1
	}
	_, _ = s.DB.Exec("INSERT OR REPLACE INTO hw(t,cpu,gpu,ssd,memt,ambient,fan,power,press,swap,src,throttled) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)",
		t, nf(p.CPU), nf(p.GPU), nf(p.SSD), nf(p.Mem), nf(p.Ambient), nf(p.Fan), nf(p.Power), nf(p.Press), nf(p.Swap), p.Src, th)
}

func nf(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// HWSeries: rata-rata per ember waktu untuk tiap metrik, puncak sepanjang rentang, dan jumlah sampel (termasuk yang dibatasi macOS).
type HWSeries struct {
	Series           map[string][][2]float64 `json:"series"`
	Peak             map[string]float64      `json:"peak"`
	Samples          int                     `json:"samples"`
	ThrottledSamples int                     `json:"throttled_samples"`
}

var hwCols = []struct{ key, col string }{
	{"cpu", "cpu"}, {"gpu", "gpu"}, {"ssd", "ssd"}, {"memory", "memt"}, {"ambient", "ambient"},
	{"fan", "fan"}, {"power", "power"}, {"pressure", "press"}, {"swap", "swap"},
}

func (s *Store) HWSeries(since, bucket int64) HWSeries {
	out := HWSeries{Series: map[string][][2]float64{}, Peak: map[string]float64{}}
	for _, c := range hwCols {
		out.Series[c.key] = [][2]float64{}
		rows, err := s.DB.Query("SELECT (t/?)*? AS b, AVG("+c.col+") FROM hw WHERE t>=? AND "+c.col+" IS NOT NULL GROUP BY b ORDER BY b", bucket, bucket, since)
		if err != nil {
			continue
		}
		for rows.Next() {
			var b int64
			var v sql.NullFloat64
			if rows.Scan(&b, &v) == nil && v.Valid {
				out.Series[c.key] = append(out.Series[c.key], [2]float64{float64(b), math.Round(v.Float64*10) / 10})
			}
		}
		_ = rows.Close()
		var mx sql.NullFloat64
		if s.DB.QueryRow("SELECT MAX("+c.col+") FROM hw WHERE t>=?", since).Scan(&mx) == nil && mx.Valid {
			out.Peak[c.key] = math.Round(mx.Float64*10) / 10
		}
	}
	_ = s.DB.QueryRow("SELECT COUNT(*), COALESCE(SUM(throttled),0) FROM hw WHERE t>=?", since).Scan(&out.Samples, &out.ThrottledSamples)
	return out
}

func (s *Store) PruneHW(before int64) {
	_, _ = s.DB.Exec("DELETE FROM hw WHERE t<?", before)
}
