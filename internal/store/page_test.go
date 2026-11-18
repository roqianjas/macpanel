package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DB.Close() })
	return st
}

func seedAudit(st *Store, n int, start time.Time) {
	for i := 0; i < n; i++ {
		st.Audit("admin", "act", fmt.Sprintf("baris %d", i), "127.0.0.1", start.Add(time.Duration(i)*time.Minute))
	}
}

// Menelusuri semua halaman harus mengembalikan setiap baris tepat sekali, terbaru dulu, tanpa celah atau duplikat.
func TestAuditPagingVisitsEveryRowExactlyOnce(t *testing.T) {
	st := newTestStore(t)
	seedAudit(st, 103, time.Unix(1_700_000_000, 0))
	for _, lim := range []int{1, 7, 25, 100, 103, 104, 200} {
		seen := map[int64]bool{}
		var last int64 = 1 << 62
		cur, pages := int64(0), 0
		for {
			pg, err := st.AuditPage(PageReq{Limit: lim, Cursor: cur})
			if err != nil {
				t.Fatal(err)
			}
			if pg.Total != 103 {
				t.Fatalf("limit %d: total=%d, harus 103", lim, pg.Total)
			}
			for _, r := range pg.Rows {
				if seen[r.ID] || r.ID >= last {
					t.Fatalf("limit %d: id %d duplikat atau tidak menurun", lim, r.ID)
				}
				seen[r.ID], last = true, r.ID
			}
			pages++
			if pg.Next == 0 {
				break
			}
			cur = pg.Next
			if pages > 200 {
				t.Fatal("tidak berhenti")
			}
		}
		if len(seen) != 103 {
			t.Fatalf("limit %d: terbaca %d baris dari 103", lim, len(seen))
		}
	}
}

// Baris yang masuk saat pembaca sedang di halaman 2 tidak boleh menggeser isi halaman berikutnya (alasan memakai cursor).
func TestPagingStableWhileNewRowsArrive(t *testing.T) {
	st := newTestStore(t)
	seedAudit(st, 30, time.Unix(1_700_000_000, 0))
	p1, _ := st.AuditPage(PageReq{Limit: 10})
	seedAudit(st, 5, time.Unix(1_800_000_000, 0))
	p2, _ := st.AuditPage(PageReq{Limit: 10, Cursor: p1.Next})
	if p2.Rows[0].ID != p1.Rows[len(p1.Rows)-1].ID-1 {
		t.Fatalf("halaman 2 bergeser: %d setelah %d", p2.Rows[0].ID, p1.Rows[len(p1.Rows)-1].ID)
	}
}

func TestExactLastPageHasNoNext(t *testing.T) {
	st := newTestStore(t)
	seedAudit(st, 50, time.Unix(1_700_000_000, 0))
	pg, _ := st.AuditPage(PageReq{Limit: 25})
	pg, _ = st.AuditPage(PageReq{Limit: 25, Cursor: pg.Next})
	if len(pg.Rows) != 25 || pg.Next != 0 {
		t.Fatalf("halaman terakhir: rows=%d next=%d", len(pg.Rows), pg.Next)
	}
}

func TestEmptyTableGivesEmptyNotNull(t *testing.T) {
	st := newTestStore(t)
	pg, err := st.AuditPage(PageReq{})
	if err != nil || pg.Rows == nil || len(pg.Rows) != 0 || pg.Total != 0 || pg.Next != 0 {
		t.Fatalf("%+v %v", pg, err)
	}
}

func TestLimitIsClamped(t *testing.T) {
	st := newTestStore(t)
	seedAudit(st, 300, time.Unix(1_700_000_000, 0))
	if pg, _ := st.AuditPage(PageReq{Limit: 100000}); len(pg.Rows) != MaxPageLimit {
		t.Fatalf("limit besar harus dipotong ke %d, dapat %d", MaxPageLimit, len(pg.Rows))
	}
	if pg, _ := st.AuditPage(PageReq{}); len(pg.Rows) != DefaultPageLimit {
		t.Fatalf("limit kosong harus %d, dapat %d", DefaultPageLimit, len(pg.Rows))
	}
}

func TestSearchFiltersAndCountsMatchesOnly(t *testing.T) {
	st := newTestStore(t)
	now := time.Unix(1_700_000_000, 0)
	st.Audit("alice", "login", "ok", "10.0.0.1", now)
	st.Audit("bob", "site.add", "toko.lab", "10.0.0.2", now)
	st.Audit("bob", "site.remove", "TOKO.lab", "10.0.0.2", now)
	st.Audit("carol", "login", "gagal", "10.0.0.3", now)
	pg, _ := st.AuditPage(PageReq{Q: "toko"})
	if pg.Total != 2 || len(pg.Rows) != 2 {
		t.Fatalf("cari 'toko' (tanpa beda huruf): %+v", pg)
	}
	if pg, _ = st.AuditPage(PageReq{Q: "10.0.0.3"}); pg.Total != 1 {
		t.Fatalf("cari IP: %+v", pg)
	}
	if pg, _ = st.AuditPage(PageReq{Q: "tidak-ada-xyz"}); pg.Total != 0 || len(pg.Rows) != 0 {
		t.Fatalf("tanpa hasil: %+v", pg)
	}
}

// Karakter khusus LIKE dan SQL diperlakukan sebagai huruf biasa.
func TestSearchTreatsWildcardsAndQuotesLiterally(t *testing.T) {
	st := newTestStore(t)
	now := time.Unix(1_700_000_000, 0)
	st.Audit("a", "x", "diskon 50% hari ini", "", now)
	st.Audit("a", "x", "diskon 50 persen", "", now)
	st.Audit("a", "x", "nama_file", "", now)
	st.Audit("a", "x", "namaXfile", "", now)
	st.Audit("a", "x", "kutip ' dan \\ ", "", now)
	for q, want := range map[string]int{"50%": 1, "a_f": 1, "%": 1, "_": 1, "'": 1, "\\": 1, "' OR 1=1 --": 0, "x') OR ('1'='1": 0} {
		pg, err := st.AuditPage(PageReq{Q: q})
		if err != nil || pg.Total != want {
			t.Errorf("q=%q: total=%d err=%v, mau %d", q, pg.Total, err, want)
		}
	}
	if n, err := st.VerifyAudit(); err != nil || n != 5 {
		t.Fatalf("tabel utuh: n=%d err=%v", n, err)
	}
}

func TestPruneAuditKeepsChainVerifiable(t *testing.T) {
	st := newTestStore(t)
	t0 := time.Unix(1_700_000_000, 0)
	seedAudit(st, 40, t0)                     // lama
	seedAudit(st, 20, t0.Add(1000*time.Hour)) // baru
	cutoff := t0.Add(500 * time.Hour).Unix()
	n, err := st.PruneAudit(cutoff, 10)
	if err != nil || n != 40 {
		t.Fatalf("dibuang=%d err=%v, mau 40", n, err)
	}
	cnt, err := st.VerifyAudit()
	if err != nil || cnt != 20 {
		t.Fatalf("rantai setelah pemangkasan: n=%d err=%v", cnt, err)
	}
	// baris baru setelah pemangkasan tetap tersambung
	st.Audit("admin", "after", "x", "", t0.Add(2000*time.Hour))
	if cnt, err = st.VerifyAudit(); err != nil || cnt != 21 {
		t.Fatalf("setelah tambah: n=%d err=%v", cnt, err)
	}
	// pemangkasan kedua tanpa kandidat: tidak berbuat apa-apa
	if n, _ = st.PruneAudit(cutoff, 10); n != 0 {
		t.Fatalf("pemangkasan ulang membuang %d", n)
	}
}

func TestPruneAuditStillDetectsTamperAfterwards(t *testing.T) {
	st := newTestStore(t)
	t0 := time.Unix(1_700_000_000, 0)
	seedAudit(st, 30, t0)
	seedAudit(st, 10, t0.Add(1000*time.Hour))
	if _, err := st.PruneAudit(t0.Add(500*time.Hour).Unix(), 5); err != nil {
		t.Fatal(err)
	}
	var id int64
	_ = st.DB.QueryRow("SELECT id FROM audit ORDER BY id LIMIT 1 OFFSET 3").Scan(&id)
	_, _ = st.DB.Exec("UPDATE audit SET detail='diubah' WHERE id=?", id)
	if _, err := st.VerifyAudit(); err == nil {
		t.Fatal("perubahan setelah pemangkasan tidak terdeteksi")
	}
}

func TestPruneAuditRespectsKeepMinimum(t *testing.T) {
	st := newTestStore(t)
	seedAudit(st, 30, time.Unix(1_700_000_000, 0))
	n, err := st.PruneAudit(time.Unix(2_000_000_000, 0).Unix(), 12) // semua lebih tua, tetapi sisakan 12
	if err != nil || n != 18 {
		t.Fatalf("dibuang=%d err=%v, mau 18", n, err)
	}
	if cnt, err := st.VerifyAudit(); err != nil || cnt != 12 {
		t.Fatalf("n=%d err=%v", cnt, err)
	}
}

func TestPruneJobsNeverTouchesRunningJobs(t *testing.T) {
	st := newTestStore(t)
	old := time.Unix(1_000_000_000, 0)
	run := st.AddJob("a", "lama-berjalan", "berjalan", "", 0, old)
	for i := 0; i < 10; i++ {
		st.AddJob("a", "selesai", "ok", "keluaran", 0, old)
	}
	n, err := st.PruneJobs(time.Unix(1_700_000_000, 0).Unix(), 3)
	if err != nil || n != 7 {
		t.Fatalf("dibuang=%d err=%v, mau 7 (10 selesai - 3 sisa)", n, err)
	}
	if st.GetJob(run) == nil {
		t.Fatal("job yang masih berjalan terhapus")
	}
}

func TestJobsPagingAndSearch(t *testing.T) {
	st := newTestStore(t)
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < 60; i++ {
		a := "backup"
		if i%3 == 0 {
			a = "deploy toko"
		}
		st.AddJob("admin", a, "ok", "x", 0, now)
	}
	pg, _ := st.JobsPage(PageReq{Q: "deploy", Limit: 10})
	if pg.Total != 20 || len(pg.Rows) != 10 || pg.Next == 0 {
		t.Fatalf("%+v", pg)
	}
	pg2, _ := st.JobsPage(PageReq{Q: "deploy", Limit: 10, Cursor: pg.Next})
	if len(pg2.Rows) != 10 || pg2.Next != 0 || pg2.Rows[0].ID >= pg.Rows[9].ID {
		t.Fatalf("halaman 2: %+v", pg2)
	}
}
