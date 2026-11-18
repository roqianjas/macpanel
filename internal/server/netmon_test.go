package server_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/netmon"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func netSnap(inetOK bool, ms float64, tsState string) netmon.Snapshot {
	m := ms
	s := netmon.Snapshot{
		Tailscale: &netmon.Tailscale{State: tsState, SelfOnline: tsState == "Running", Peers: 6, PeersOnline: 2},
		Internet:  netmon.Internet{OK: inetOK},
		DNSLocal:  &netmon.DNS{Name: "panel.lab", OK: true, MS: &m},
		DNSPublic: &netmon.DNS{Name: "example.com", OK: true, MS: &m},
	}
	if inetOK {
		s.Internet.MS = &m
	}
	return s
}

type netResp struct {
	Now *struct {
		At        int64 `json:"at"`
		Tailscale struct {
			State string `json:"state"`
		} `json:"tailscale"`
	} `json:"now"`
	Stale     bool                    `json:"stale"`
	Series    map[string][][2]float64 `json:"series"`
	Samples   int                     `json:"samples"`
	InetDown  int                     `json:"inet_down"`
	TSDown    int                     `json:"ts_down"`
	InetMaxMS float64                 `json:"inet_max_ms"`
}

func TestNetmonSampleStoreAndEndpoint(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	i := 0
	netmon.SetFetch(o.App, func() (netmon.Snapshot, error) {
		i++
		if i == 2 {
			return netSnap(false, 0, "Running"), nil
		}
		return netSnap(true, float64(5+i), "Running"), nil
	})
	base := o.App.Now()
	for k := 0; k < 4; k++ {
		netmon.Sample(o.App, base.Add(time.Duration(k-3)*time.Minute))
	}
	var r netResp
	w := o.Do("GET", "/api/netmon?range=1h", "", o.Cookie, "", nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &r) != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if r.Samples != 4 || r.InetDown != 1 || r.TSDown != 0 {
		t.Fatalf("samples=%d inet_down=%d ts_down=%d", r.Samples, r.InetDown, r.TSDown)
	}
	if len(r.Series["internet"]) == 0 || r.InetMaxMS != 9 {
		t.Fatalf("series=%v max=%v", r.Series["internet"], r.InetMaxMS)
	}
	if r.Now == nil || r.Now.Tailscale.State != "Running" || r.Now.At != base.Unix() {
		t.Fatalf("now=%+v", r.Now)
	}
}

func TestNetmonFailedMeasurementIsNotRecordedAsOutage(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	netmon.SetFetch(o.App, func() (netmon.Snapshot, error) { return netmon.Snapshot{}, errors.New("lab mati") })
	netmon.Sample(o.App, o.App.Now())
	var r netResp
	_ = json.Unmarshal(o.Do("GET", "/api/netmon", "", o.Cookie, "", nil).Body.Bytes(), &r)
	if r.Samples != 0 || r.InetDown != 0 || r.Now != nil {
		t.Fatalf("tidak mampu mengukur tidak boleh tercatat sebagai internet mati: %+v", r)
	}
}

func TestNetmonValidationAuthAndStale(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for _, q := range []string{"range=2h", "range=7D", "range=x"} {
		if w := o.Do("GET", "/api/netmon?"+q, "", o.Cookie, "", nil); w.Code != 400 {
			t.Errorf("%s: %d", q, w.Code)
		}
	}
	if w := o.Do("GET", "/api/netmon", "", "", "", nil); w.Code != 401 {
		t.Fatalf("tanpa login: %d", w.Code)
	}
	netmon.SetFetch(o.App, func() (netmon.Snapshot, error) { return netSnap(true, 5, "Running"), nil })
	netmon.Sample(o.App, o.App.Now().Add(-20*time.Minute))
	var r netResp
	_ = json.Unmarshal(o.Do("GET", "/api/netmon", "", o.Cookie, "", nil).Body.Bytes(), &r)
	if !r.Stale {
		t.Fatal("sampel 20 menit lalu harus basi")
	}
}

func TestNetmonTailscaleDownRaisesAlertOnlyAfterTwoSamples(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	netmon.SetFetch(o.App, func() (netmon.Snapshot, error) { return netSnap(true, 5, "Stopped"), nil })
	base := o.App.Now()
	alerts := func() map[string]string {
		w := o.Do("GET", "/api/alerts", "", o.Cookie, "", nil)
		var r struct {
			Alerts []struct{ ID, Level string } `json:"alerts"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &r)
		m := map[string]string{}
		for _, a := range r.Alerts {
			m[a.ID] = a.Level
		}
		return m
	}
	netmon.Sample(o.App, base.Add(-time.Minute))
	if _, has := alerts()["net-tailscale"]; has {
		t.Fatal("satu sampel tidak boleh memicu alert")
	}
	netmon.Sample(o.App, base)
	o.Now = o.Now.Add(31 * time.Second) // lewati cache alert 30 detik
	if got := alerts()["net-tailscale"]; got != "bad" {
		t.Fatalf("dua sampel Tailscale mati berturut-turut harus memicu alert 'bad', dapat %q", got)
	}
}
