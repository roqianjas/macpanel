// Package diskclean: analisis disk, pembersihan berbasis katalog, dan ringkasan memori. Semua kerja lewat `lab clean` dan `lab mem`.
// Panel tidak pernah mengirim jalur: hanya id entri katalog yang divalidasi bentuknya di sini dan dicek ulang oleh CLI.
package diskclean

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

const maxIDs = 20

// Bentuk id katalog: "go-build" atau "bk:nama-folder". Tanpa garis miring, titik-titik, atau awalan "-".
var idRE = regexp.MustCompile(`^(?:bk:|cx:)?[A-Za-z0-9][A-Za-z0-9._-]{0,89}$`)

func validIDs(ids []string) (string, bool) {
	if len(ids) < 1 || len(ids) > maxIDs {
		return "pilih 1 sampai 20 entri", false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !idRE.MatchString(id) || strings.Contains(id, "..") {
			return "id entri tidak valid", false
		}
		if seen[id] {
			return "id entri ganda", false
		}
		seen[id] = true
	}
	return "", true
}

func careful(ids []string) []string {
	var out []string
	for _, id := range ids {
		if strings.HasPrefix(id, "bk:") || strings.HasPrefix(id, "cx:") {
			out = append(out, id)
		}
	}
	return out
}

func pass(w http.ResponseWriter, res core.Result, fail string) (json.RawMessage, bool) {
	if res.Code != 0 {
		core.WriteError(w, 502, fail)
		return nil, false
	}
	out := strings.TrimSpace(res.Out)
	if !json.Valid([]byte(out)) {
		core.WriteError(w, 502, fail)
		return nil, false
	}
	return json.RawMessage(out), true
}

// Last: GET /api/clean. Hasil analisis terakhir (null bila belum pernah).
func Last(a *core.App, w http.ResponseWriter, _ *http.Request, _ *store.Session) {
	raw, ok := pass(w, a.RunLab([]string{"clean", "last"}, ""), "tidak bisa membaca hasil analisis")
	if !ok {
		return
	}
	core.WriteJSON(w, 200, map[string]any{"scan": raw})
}

// Scan: POST /api/clean/scan. Pengukuran berat (puluhan detik), jadi berjalan sebagai job.
func Scan(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{}
	if !core.DecodeJSON(w, r, &in) || !core.AdminOnly(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "clean_scan", "disk", []string{"clean", "scan"}, ""))
}

type runIn struct {
	IDs   []string `json:"ids"`
	Typed string   `json:"typed"`
}

// Preview: POST /api/clean/preview. Pratinjau (dry-run) tanpa menghapus apa pun.
func Preview(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in runIn
	if !core.DecodeJSON(w, r, &in) || !core.AdminOnly(w, ss) {
		return
	}
	if msg, ok := validIDs(in.IDs); !ok {
		core.WriteError(w, 400, msg)
		return
	}
	args := append([]string{"clean", "run"}, in.IDs...)
	raw, ok := pass(w, a.RunLabTimeout(args, "", a.LongTimeout), "pratinjau gagal")
	if !ok {
		return
	}
	core.WriteJSON(w, 200, map[string]any{"preview": raw})
}

// Run: POST /api/clean/run. Menghapus; step-up wajib. Tingkat Hati-hati: tepat satu folder per permintaan dan nama
// folder harus diketik ulang (`typed`).
func Run(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in runIn
	if !core.DecodeJSON(w, r, &in) || !core.AdminOnly(w, ss) {
		return
	}
	if msg, ok := validIDs(in.IDs); !ok {
		core.WriteError(w, 400, msg)
		return
	}
	args := append([]string{"clean", "run"}, in.IDs...)
	if c := careful(in.IDs); len(c) > 0 {
		if len(in.IDs) != 1 {
			core.WriteError(w, 400, "folder tingkat Hati-hati dihapus satu per satu, tanpa digabung")
			return
		}
		if in.Typed != strings.TrimPrefix(strings.TrimPrefix(c[0], "bk:"), "cx:") {
			core.WriteError(w, 400, "ketik ulang nama folder persis untuk melanjutkan")
			return
		}
		args = append(args, "--confirm-careful")
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	args = append(args, "--apply")
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "clean_run", strings.Join(in.IDs, ","), args, ""))
}

// Mem: GET /api/mem. Ringkasan memori, proses terbesar per layanan, dan saran penghematan.
func Mem(a *core.App, w http.ResponseWriter, _ *http.Request, _ *store.Session) {
	raw, ok := pass(w, a.RunLab([]string{"mem", "top"}, ""), "tidak bisa membaca memori")
	if !ok {
		return
	}
	core.WriteJSON(w, 200, raw)
}
