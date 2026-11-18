package system

import (
	"log"
	"time"

	"github.com/roqianjas/macpanel/internal/store"
)

// Aturan retensi riwayat. Audit disimpan lama (jejak keamanan); keluaran job tidak perlu selama itu.
// Selalu ada sisa minimum supaya riwayat tidak kosong di instalasi yang jarang dipakai.
const (
	auditKeepDays = 365
	auditKeepMin  = 1000
	jobsKeepDays  = 90
	jobsKeepMin   = 200
)

func pruneHistory(st *store.Store, now time.Time) {
	day := int64(24 * 3600)
	if n, err := st.PruneAudit(now.Unix()-auditKeepDays*day, auditKeepMin); err != nil {
		log.Printf("retensi audit gagal: %v", err)
	} else if n > 0 {
		log.Printf("retensi audit: %d baris lama dibuang (jangkar rantai diperbarui)", n)
	}
	if n, err := st.PruneJobs(now.Unix()-jobsKeepDays*day, jobsKeepMin); err != nil {
		log.Printf("retensi job gagal: %v", err)
	} else if n > 0 {
		log.Printf("retensi job: %d baris lama dibuang", n)
	}
}
