package hardware

import (
	"fmt"
	"time"
)

// Ambang suhu, tekanan memori, dan swap ada di thresholds.go (dapat diubah pemilik).
const (
	ForecastWarnDays = 14.0
	ForecastBadDays  = 3.0
	forecastMinR2    = 0.5
)

// Finding: peringatan turunan; overview mengubahnya menjadi Alert (domain tidak boleh mengimpor overview).
type Finding struct {
	ID, Level, Title, Detail string
}

type Inputs struct {
	Snap     *Snapshot
	Stale    bool
	SSD      *SSD
	Forecast Forecast
	Th       Thresholds
	Now      time.Time
}

func level(v, warn, bad float64) string {
	switch {
	case v >= bad:
		return "bad"
	case v >= warn:
		return "warn"
	}
	return ""
}

// Findings murni (tanpa I/O) agar mudah diuji.
func Findings(in Inputs) []Finding {
	out := []Finding{}
	add := func(id, lv, title, detail string) {
		if lv != "" {
			out = append(out, Finding{id, lv, title, detail})
		}
	}
	if s := in.Snap; s != nil {
		switch {
		case in.Stale:
			add("hw-stale", "warn", "Pemantauan perangkat keras berhenti", fmt.Sprintf("sampel terakhir %d menit lalu", (in.Now.Unix()-s.At)/60))
		case s.Source == "none":
			d := "tidak ada sumber suhu"
			if len(s.Errors) > 0 {
				d = s.Errors[len(s.Errors)-1]
			}
			add("hw-nosensor", "warn", "Sensor suhu tidak terbaca", d)
		default:
			for _, c := range []struct {
				key, name string
				warn, bad float64
			}{{"cpu", "CPU", in.Th.CPUWarn, in.Th.CPUBad}, {"gpu", "GPU", in.Th.CPUWarn, in.Th.CPUBad}, {"ssd", "SSD", in.Th.SSDWarn, in.Th.SSDBad}} {
				if v, ok := s.Temps[c.key]; ok {
					add("hw-temp-"+c.key, level(v, c.warn, c.bad), "Suhu "+c.name+" tinggi", fmt.Sprintf("%.0f °C", v))
				}
			}
			if s.Thermal != nil && s.Thermal.Throttled {
				add("hw-throttle", "bad", "macOS membatasi kecepatan CPU", fmt.Sprintf("batas kecepatan %d%% karena panas", s.Thermal.SpeedLimit))
			}
			if s.Mem != nil {
				if s.Mem.PressurePct != nil {
					add("hw-mempress", level(*s.Mem.PressurePct, in.Th.PressWarn, in.Th.PressBad), "Tekanan memori tinggi", fmt.Sprintf("%.0f%%", *s.Mem.PressurePct))
				}
				if s.Mem.SwapUsedMB != nil && *s.Mem.SwapUsedMB >= in.Th.SwapMB {
					add("hw-swap", "warn", "Swap banyak terpakai", fmt.Sprintf("%.0f MB", *s.Mem.SwapUsedMB))
				}
			}
		}
	}
	if d := in.SSD; d != nil {
		switch d.Smart {
		case "", "Verified", "Not Supported":
		default:
			add("hw-smart", "bad", "SSD melaporkan masalah (SMART)", "status: "+d.Smart)
		}
	}
	if f := in.Forecast; f.OK && f.DaysToFull != nil && f.R2 >= forecastMinR2 {
		switch d := *f.DaysToFull; {
		case d < ForecastBadDays:
			out = append(out, Finding{"hw-diskforecast", "bad", "Disk diperkirakan penuh sangat segera", fmt.Sprintf("sekitar %.0f hari lagi (tren %.2f%%/hari)", d, f.PctPerDay)})
		case d < ForecastWarnDays:
			out = append(out, Finding{"hw-diskforecast", "warn", "Disk diperkirakan penuh dalam waktu dekat", fmt.Sprintf("sekitar %.0f hari lagi (tren %.2f%%/hari)", d, f.PctPerDay)})
		}
	}
	return out
}
