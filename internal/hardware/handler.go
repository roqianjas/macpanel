package hardware

import (
	"net/http"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type rangeSpec struct{ Sec, Bucket int64 }

var ranges = map[string]rangeSpec{"1h": {3600, 30}, "24h": {86400, 300}, "7d": {7 * 86400, 1800}}

// Inputs mengumpulkan data terkini untuk peringatan.
func InputsNow(a *core.App) Inputs {
	now := a.Now()
	snap, stale, ssd := Current(a, now)
	var total uint64
	if ssd != nil {
		total = ssd.Total
	}
	_, _, disk, _ := a.Store.Series(now.Unix()-retain, 1800)
	return Inputs{Snap: snap, Stale: stale, SSD: ssd, Forecast: ForecastDisk(disk, total), Now: now}
}

// Hardware: GET /api/hw?range=1h|24h|7d
func Hardware(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	rk := r.URL.Query().Get("range")
	if rk == "" {
		rk = "24h"
	}
	rs, ok := ranges[rk]
	if !ok {
		core.WriteError(w, 400, "range harus 1h, 24h, atau 7d")
		return
	}
	in := InputsNow(a)
	hs := a.Store.HWSeries(in.Now.Unix()-rs.Sec, rs.Bucket)
	core.WriteJSON(w, 200, map[string]any{
		"now": in.Snap, "stale": in.Stale, "ssd": in.SSD, "forecast": in.Forecast,
		"range": rk, "bucket": rs.Bucket, "interval": int(SampleEvery.Seconds()),
		"series": hs.Series, "peak": hs.Peak, "samples": hs.Samples, "throttled_samples": hs.ThrottledSamples,
		"thresholds": map[string]float64{"cpu_warn": CPUWarn, "cpu_bad": CPUBad, "ssd_warn": SSDWarn, "ssd_bad": SSDBad, "press_warn": PressWarn, "press_bad": PressB},
	})
}
