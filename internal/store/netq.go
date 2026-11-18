package store

import (
	"database/sql"
	"math"
)

const netqSchema = `
CREATE TABLE IF NOT EXISTS netq(t INTEGER PRIMARY KEY, ts_state TEXT NOT NULL DEFAULT '', ts_self INTEGER NOT NULL DEFAULT -1,
  ts_peers INTEGER NOT NULL DEFAULT 0, ts_online INTEGER NOT NULL DEFAULT 0,
  inet_ok INTEGER NOT NULL DEFAULT 0, inet_ms REAL, dnsl_ok INTEGER NOT NULL DEFAULT -1, dnsl_ms REAL,
  dnsp_ok INTEGER NOT NULL DEFAULT 0, dnsp_ms REAL);
`

// NetqPoint: satu sampel konektivitas. Self/DNSLocal: -1 = tidak diukur, 0 = gagal, 1 = baik. ms nil = gagal diukur.
type NetqPoint struct {
	T                 int64
	TSState           string
	TSSelf            int
	TSPeers, TSOnline int
	InetOK            bool
	InetMS            *float64
	DNSLocal          int
	DNSLocalMS        *float64
	DNSPublicOK       bool
	DNSPublicMS       *float64
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) AddNetq(t int64, p NetqPoint) {
	_, _ = s.DB.Exec("INSERT OR REPLACE INTO netq(t,ts_state,ts_self,ts_peers,ts_online,inet_ok,inet_ms,dnsl_ok,dnsl_ms,dnsp_ok,dnsp_ms) VALUES(?,?,?,?,?,?,?,?,?,?,?)",
		t, Clean(p.TSState, 20), p.TSSelf, p.TSPeers, p.TSOnline, b2i(p.InetOK), nf(p.InetMS), p.DNSLocal, nf(p.DNSLocalMS), b2i(p.DNSPublicOK), nf(p.DNSPublicMS))
}

// NetqRecent mengembalikan n sampel terbaru, terbaru dulu.
func (s *Store) NetqRecent(n int) []NetqPoint {
	out := []NetqPoint{}
	rows, err := s.DB.Query("SELECT t,ts_state,ts_self,ts_peers,ts_online,inet_ok,inet_ms,dnsl_ok,dnsl_ms,dnsp_ok,dnsp_ms FROM netq ORDER BY t DESC LIMIT ?", n)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var p NetqPoint
		var io, po int
		var im, dl, dp sql.NullFloat64
		if rows.Scan(&p.T, &p.TSState, &p.TSSelf, &p.TSPeers, &p.TSOnline, &io, &im, &p.DNSLocal, &dl, &po, &dp) != nil {
			continue
		}
		p.InetOK, p.DNSPublicOK = io == 1, po == 1
		if im.Valid {
			p.InetMS = &im.Float64
		}
		if dl.Valid {
			p.DNSLocalMS = &dl.Float64
		}
		if dp.Valid {
			p.DNSPublicMS = &dp.Float64
		}
		out = append(out, p)
	}
	return out
}

// NetqStats: ringkasan rentang waktu. Persen dihitung dari sampel yang ada, bukan dari waktu (panel yang mati tidak dihitung 'turun').
type NetqStats struct {
	Series       map[string][][2]float64 `json:"series"`
	Samples      int                     `json:"samples"`
	TSDown       int                     `json:"ts_down"`
	InetDown     int                     `json:"inet_down"`
	DNSLocalDown int                     `json:"dns_local_down"`
	DNSLocalN    int                     `json:"dns_local_n"`
	InetMaxMS    float64                 `json:"inet_max_ms"`
}

var netqCols = []struct{ key, col string }{{"internet", "inet_ms"}, {"dns_local", "dnsl_ms"}, {"dns_public", "dnsp_ms"}}

func (s *Store) NetqStats(since, bucket int64) NetqStats {
	out := NetqStats{Series: map[string][][2]float64{}}
	for _, c := range netqCols {
		out.Series[c.key] = [][2]float64{}
		rows, err := s.DB.Query("SELECT (t/?)*? AS b, AVG("+c.col+") FROM netq WHERE t>=? AND "+c.col+" IS NOT NULL GROUP BY b ORDER BY b", bucket, bucket, since)
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
	}
	var mx sql.NullFloat64
	_ = s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN ts_state<>'' AND (ts_state<>'Running' OR ts_self=0) THEN 1 ELSE 0 END),0),
	  COALESCE(SUM(CASE WHEN inet_ok=0 THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN dnsl_ok=0 THEN 1 ELSE 0 END),0),
	  COALESCE(SUM(CASE WHEN dnsl_ok>=0 THEN 1 ELSE 0 END),0), MAX(inet_ms) FROM netq WHERE t>=?`, since).
		Scan(&out.Samples, &out.TSDown, &out.InetDown, &out.DNSLocalDown, &out.DNSLocalN, &mx)
	if mx.Valid {
		out.InetMaxMS = math.Round(mx.Float64*10) / 10
	}
	return out
}

func (s *Store) PruneNetq(before int64) {
	_, _ = s.DB.Exec("DELETE FROM netq WHERE t<?", before)
}
