package jobs

import (
	"net/http"
	"strconv"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func Jobs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	p, ok := core.PageParams(w, r)
	if !ok {
		return
	}
	pg, err := a.Store.JobsPage(p)
	if err != nil {
		core.WriteError(w, 500, "gagal membaca riwayat job")
		return
	}
	// "jobs" dipertahankan untuk klien lama; "rows", "next", "total" adalah kontrak pagination baru
	core.WriteJSON(w, 200, map[string]any{"jobs": pg.Rows, "rows": pg.Rows, "next": pg.Next, "total": pg.Total})
}

func Job(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		core.WriteError(w, 400, "id tidak valid")
		return
	}
	j := a.Store.GetJob(id)
	if j == nil {
		core.WriteError(w, 404, "job tidak ada")
		return
	}
	core.WriteJSON(w, 200, j)
}
