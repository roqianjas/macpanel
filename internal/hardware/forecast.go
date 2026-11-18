package hardware

import "math"

const (
	minSpanDays   = 1.0  // butuh data minimal sehari: tren dari beberapa jam hanya menangkap satu unduhan
	minPoints     = 24   // dan cukup titik
	flatPctPerDay = 0.02 // di bawah ini dianggap stabil
	maxDays       = 3650 // lebih dari 10 tahun tidak berarti
)

// Forecast: perkiraan kapan disk penuh dari tren linear pemakaian disk (persen).
type Forecast struct {
	OK         bool     `json:"ok"`
	Reason     string   `json:"reason,omitempty"`
	PctPerDay  float64  `json:"pct_per_day"`
	GBPerDay   *float64 `json:"gb_per_day,omitempty"`
	DaysToFull *float64 `json:"days_to_full,omitempty"`
	R2         float64  `json:"r2"`
	SpanDays   float64  `json:"span_days"`
}

// ForecastDisk memakai regresi kuadrat terkecil. pts = [waktu detik, persen terpakai]. R2 rendah berarti
// pemakaian naik-turun tidak beraturan (mis. sering hapus), jadi perkiraan tidak boleh dijadikan alert.
func ForecastDisk(pts [][2]float64, totalBytes uint64) Forecast {
	f := Forecast{}
	n := len(pts)
	if n < minPoints {
		f.Reason = "data belum cukup (butuh minimal 24 jam)"
		return f
	}
	t0 := pts[0][0]
	span := (pts[n-1][0] - t0) / 86400
	f.SpanDays = math.Round(span*100) / 100
	if span < minSpanDays {
		f.Reason = "data belum cukup (butuh minimal 24 jam)"
		return f
	}
	var sx, sy, sxx, sxy, syy float64
	for _, p := range pts {
		x := (p[0] - t0) / 86400
		sx, sy, sxx, sxy, syy = sx+x, sy+p[1], sxx+x*x, sxy+x*p[1], syy+p[1]*p[1]
	}
	fn := float64(n)
	den := fn*sxx - sx*sx
	if den == 0 {
		f.Reason = "data tidak valid"
		return f
	}
	slope := (fn*sxy - sx*sy) / den
	if vy := fn*syy - sy*sy; vy > 0 {
		f.R2 = math.Round(((fn*sxy-sx*sy)*(fn*sxy-sx*sy)/(den*vy))*100) / 100
	} else {
		f.R2 = 1 // semua nilai sama: garis datar sempurna
	}
	f.OK = true
	f.PctPerDay = math.Round(slope*1000) / 1000
	if totalBytes > 0 {
		g := math.Round(slope/100*float64(totalBytes)/1e9*100) / 100
		f.GBPerDay = &g
	}
	if slope <= flatPctPerDay {
		f.Reason = "stabil atau menurun"
		return f
	}
	last := pts[n-1][1]
	if d := (100 - last) / slope; d >= 0 && d <= maxDays {
		d = math.Round(d*10) / 10
		f.DaysToFull = &d
	} else if d > maxDays {
		f.Reason = "lebih dari 10 tahun"
	}
	return f
}
