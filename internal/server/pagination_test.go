package server_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/testenv"
)

type pageResp struct {
	Rows  []map[string]any `json:"rows"`
	Next  int64            `json:"next"`
	Total int              `json:"total"`
}

type doFn = func(method, path, body, cookie, csrf string, hdr map[string]string) *httptest.ResponseRecorder

// pager membungkus o.Do dengan cookie sesi; tipe opsEnv tidak diekspor, jadi dipakai lewat fungsi.
func pager(t *testing.T, do doFn, cookie string) func(path string) (int, pageResp) {
	return func(path string) (int, pageResp) { return getPage(t, do, cookie, path) }
}

func getPage(t *testing.T, do doFn, cookie, path string) (int, pageResp) {
	t.Helper()
	w := do("GET", path, "", cookie, "", nil)
	var p pageResp
	if w.Code == 200 {
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatalf("%s: bukan JSON: %v %s", path, err, w.Body.String())
		}
	}
	return w.Code, p
}

func TestAuditEndpointPagination(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	get := pager(t, o.Do, o.Cookie)
	for i := 0; i < 80; i++ {
		o.App.Store.Audit("admin", "seed", fmt.Sprintf("catatan-%02d", i), "10.1.1.1", time.Now())
	}
	code, p := get("/api/audit?limit=30")
	if code != 200 || len(p.Rows) != 30 || p.Next == 0 || p.Total < 80 {
		t.Fatalf("halaman 1: code=%d rows=%d next=%d total=%d", code, len(p.Rows), p.Next, p.Total)
	}
	_, p2 := get(fmt.Sprintf("/api/audit?limit=30&cursor=%d", p.Next))
	if len(p2.Rows) != 30 || p2.Rows[0]["id"].(float64) >= p.Rows[29]["id"].(float64) {
		t.Fatalf("halaman 2 tidak lanjut dari halaman 1")
	}
	// pencarian
	_, s := get("/api/audit?q=catatan-07")
	if s.Total != 1 || len(s.Rows) != 1 || !strings.Contains(s.Rows[0]["detail"].(string), "catatan-07") {
		t.Fatalf("cari: %+v", s)
	}
	// tanpa limit: bawaan 25
	if _, d := get("/api/audit"); len(d.Rows) != 25 {
		t.Fatalf("bawaan harus 25 baris, dapat %d", len(d.Rows))
	}
}

func TestPaginationRejectsBadParameters(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	get := pager(t, o.Do, o.Cookie)
	for _, q := range []string{"limit=0", "limit=-1", "limit=201", "limit=abc", "limit=1e3", "cursor=0", "cursor=-5", "cursor=abc", "cursor=99999999999999999999"} {
		for _, base := range []string{"/api/audit", "/api/jobs"} {
			if code, _ := get(base + "?" + q); code != 400 {
				t.Errorf("%s?%s: kode %d, harus 400", base, q, code)
			}
		}
	}
	// batas sah diterima
	for _, q := range []string{"limit=1", "limit=200", "cursor=1", "q="} {
		if code, _ := get("/api/audit?" + q); code != 200 {
			t.Errorf("%s ditolak (%d)", q, code)
		}
	}
}

func TestPaginationNeedsLogin(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for _, p := range []string{"/api/audit?limit=5", "/api/jobs?limit=5"} {
		if w := o.Do("GET", p, "", "", "", nil); w.Code != 401 {
			t.Errorf("%s tanpa login: %d", p, w.Code)
		}
	}
}

func TestJobsEndpointKeepsLegacyKeyAndAddsPaging(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	for i := 0; i < 40; i++ {
		o.App.Store.AddJob("admin", fmt.Sprintf("aksi-%d", i), "ok", "keluaran", 0, time.Now())
	}
	w := o.Do("GET", "/api/jobs?limit=15", "", o.Cookie, "", nil)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"jobs", "rows", "next", "total"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("kunci %q hilang di respons /api/jobs", k)
		}
	}
	var p pageResp
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Rows) != 15 || p.Next == 0 || p.Total < 40 {
		t.Fatalf("%+v", p)
	}
}
