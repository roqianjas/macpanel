package jobs

import (
	"net/http"
	"strconv"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func Jobs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.WriteJSON(w, 200, map[string]any{"jobs": a.Store.ListJobs(50)})
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
