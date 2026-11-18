package core

import (
	"net/http"
	"strconv"

	"github.com/roqianjas/macpanel/internal/store"
)

// PageParams membaca ?limit=&cursor=&q= untuk daftar yang dipaginasi di server.
// Nilai tidak valid ditolak (galat ditulis ke w, ok=false) agar klien tahu salahnya, bukan diam-diam diganti.
func PageParams(w http.ResponseWriter, r *http.Request) (store.PageReq, bool) {
	var p store.PageReq
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > store.MaxPageLimit {
			WriteError(w, 400, "limit harus 1-"+strconv.Itoa(store.MaxPageLimit))
			return p, false
		}
		p.Limit = n
	}
	if v := q.Get("cursor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			WriteError(w, 400, "cursor tidak valid")
			return p, false
		}
		p.Cursor = n
	}
	p.Q = store.Clean(q.Get("q"), 100)
	return p, true
}
