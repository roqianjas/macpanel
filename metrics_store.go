package main

import (
	"database/sql"
	"math"
	"sort"
)

const metricsSchema = `
CREATE TABLE IF NOT EXISTS metrics(t INTEGER PRIMARY KEY, cpu REAL, mem REAL, disk REAL, load1 REAL);
CREATE TABLE IF NOT EXISTS netm(t INTEGER NOT NULL, iface TEXT NOT NULL, label TEXT NOT NULL, rx_bps REAL NOT NULL, tx_bps REAL NOT NULL,
  rx_b INTEGER NOT NULL, tx_b INTEGER NOT NULL, PRIMARY KEY(t, iface));
`

func r1(v float64) float64 { return math.Round(v*10) / 10 }

// AddSample menyimpan satu sampel. CPU/memori yang gagal dibaca disimpan NULL (bukan 0) supaya grafik tidak berbohong.
func (s *Store) AddSample(t int64, cpu, mem, disk, load float64, cpuOK, memOK bool) {
	var c, m any
	if cpuOK {
		c = cpu
	}
	if memOK {
		m = mem
	}
	_, _ = s.db.Exec("INSERT OR REPLACE INTO metrics(t,cpu,mem,disk,load1) VALUES(?,?,?,?,?)", t, c, m, disk, load)
}

func (s *Store) AddNet(t int64, iface, label string, rxBps, txBps float64, rxB, txB int64) {
	_, _ = s.db.Exec("INSERT OR REPLACE INTO netm(t,iface,label,rx_bps,tx_bps,rx_b,tx_b) VALUES(?,?,?,?,?,?,?)", t, iface, label, rxBps, txBps, rxB, txB)
}

func (s *Store) PruneMetrics(before int64) {
	_, _ = s.db.Exec("DELETE FROM metrics WHERE t<?", before)
	_, _ = s.db.Exec("DELETE FROM netm WHERE t<?", before)
}

func (s *Store) Series(since, bucket int64) (cpu, mem, disk, load [][2]float64) {
	cpu, mem, disk, load = [][2]float64{}, [][2]float64{}, [][2]float64{}, [][2]float64{}
	rows, err := s.db.Query("SELECT (t/?)*? AS b, AVG(cpu), AVG(mem), AVG(disk), AVG(load1) FROM metrics WHERE t>=? GROUP BY b ORDER BY b", bucket, bucket, since)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var b int64
		var c, m, d, l sql.NullFloat64
		if rows.Scan(&b, &c, &m, &d, &l) != nil {
			continue
		}
		t := float64(b)
		if c.Valid {
			cpu = append(cpu, [2]float64{t, r1(c.Float64)})
		}
		if m.Valid {
			mem = append(mem, [2]float64{t, r1(m.Float64)})
		}
		if d.Valid {
			disk = append(disk, [2]float64{t, r1(d.Float64)})
		}
		if l.Valid {
			load = append(load, [2]float64{t, math.Round(l.Float64*100) / 100})
		}
	}
	return
}

func (s *Store) NetSeries(since, bucket int64) []NetSeries {
	out := []NetSeries{}
	idx := map[string]int{}
	rows, err := s.db.Query("SELECT (t/?)*? AS b, iface, MAX(label), AVG(rx_bps), AVG(tx_bps), SUM(rx_b), SUM(tx_b) FROM netm WHERE t>=? GROUP BY b, iface ORDER BY iface, b", bucket, bucket, since)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var b, rb, tb int64
		var iface, label string
		var rx, tx float64
		if rows.Scan(&b, &iface, &label, &rx, &tx, &rb, &tb) != nil {
			continue
		}
		i, ok := idx[iface]
		if !ok {
			out = append(out, NetSeries{Iface: iface, Label: label, RX: [][2]float64{}, TX: [][2]float64{}})
			i = len(out) - 1
			idx[iface] = i
		}
		out[i].RX = append(out[i].RX, [2]float64{float64(b), math.Round(rx)})
		out[i].TX = append(out[i].TX, [2]float64{float64(b), math.Round(tx)})
		out[i].TotalRX += rb
		out[i].TotalTX += tb
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Iface < out[j].Iface })
	return out
}

func (s *Store) LastRate(iface string) (rx, tx float64) {
	_ = s.db.QueryRow("SELECT rx_bps, tx_bps FROM netm WHERE iface=? ORDER BY t DESC LIMIT 1", iface).Scan(&rx, &tx)
	return
}
