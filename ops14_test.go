package main

import (
	"strings"
	"testing"
	"time"
)

func TestSysDNSReadOnly(t *testing.T) {
	o := newOpsEnv(t, `case "$2" in dns) printf '{"tld":"lab","wildcard":"100.1.1.1","rows":[]}';; *) echo ok;; esac`)
	o.app.longTimeout = 20 * time.Second
	if w := o.do("GET", "/api/sys/dns", "", o.cookie, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "wildcard") {
		t.Fatalf("dns: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(o.calls(), "CALL:[sys][dns]") {
		t.Fatalf("argv: %s", o.calls())
	}
	// hanya baca: tidak ada rute tulis untuk DNS
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		if w := o.do(m, "/api/sys/dns", "{}", o.cookie, o.csrf, nil); w.Code != 404 && w.Code != 405 {
			t.Errorf("%s /api/sys/dns harus tidak ada: %d", m, w.Code)
		}
	}
}
