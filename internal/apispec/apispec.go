// Package apispec menurunkan spesifikasi OpenAPI dan tipe path TypeScript dari tabel rute Go (satu sumber kebenaran).
// Keluarannya deterministik (diurutkan) supaya bisa dibandingkan byte per byte di gerbang `make check`.
//
// Cakupan yang jujur: path, metode, tingkat akses, parameter path, dan apakah token API boleh memanggil.
// BUKAN skema badan permintaan/respons: handler menulis map dinamis, jadi skemanya tidak dapat diturunkan tanpa
// mengubah handler. Tipe respons tetap ditulis di sisi UI.
package apispec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
)

var param = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// Op adalah satu operasi (metode + path) dengan metadata yang relevan bagi klien.
type Op struct {
	Method string
	Path   string
	Level  string
	Token  bool // boleh dipanggil dengan token API
	Params []string
}

func levelName(l core.Level) string {
	switch l {
	case core.Public:
		return "public"
	case core.Pwd:
		return "password"
	case core.Full:
		return "full"
	case core.Any:
		return "any"
	}
	return fmt.Sprintf("level%d", int(l))
}

// Ops mengubah tabel rute menjadi daftar operasi terurut. Gagal bila ada rute ganda (metode+path sama) atau metode tak dikenal.
func Ops(routes []core.Route) ([]Op, error) {
	seen := map[string]bool{}
	var out []Op
	for _, r := range routes {
		if r.Method != "GET" && r.Method != "POST" {
			return nil, fmt.Errorf("metode %q pada %s tidak didukung generator", r.Method, r.Path)
		}
		if !strings.HasPrefix(r.Path, "/") {
			return nil, fmt.Errorf("path %q harus diawali /", r.Path)
		}
		key := r.Method + " " + r.Path
		if seen[key] {
			return nil, fmt.Errorf("rute ganda: %s", key)
		}
		seen[key] = true
		var ps []string
		for _, m := range param.FindAllStringSubmatch(r.Path, -1) {
			ps = append(ps, m[1])
		}
		out = append(out, Op{Method: r.Method, Path: r.Path, Level: levelName(r.Level), Token: core.TokenAllowed(r.Method, r.Path), Params: ps})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}

// OperationID unik dan stabil, mis. "get_api_sites_name_settings".
func OperationID(o Op) string {
	p := param.ReplaceAllString(o.Path, "$1")
	p = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(p, "_")
	return strings.ToLower(o.Method) + "_" + strings.Trim(strings.ToLower(p), "_")
}

// OpenAPI membangun dokumen OpenAPI 3.1 (JSON berindentasi, diakhiri newline).
func OpenAPI(ops []Op) ([]byte, error) {
	paths := map[string]map[string]any{}
	for _, o := range ops {
		item := paths[o.Path]
		if item == nil {
			item = map[string]any{}
			paths[o.Path] = item
		}
		op := map[string]any{
			"operationId": OperationID(o),
			"x-level":     o.Level,
			"x-token":     o.Token,
			"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
		}
		if len(o.Params) > 0 {
			var ps []map[string]any
			for _, n := range o.Params {
				ps = append(ps, map[string]any{"name": n, "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
			}
			op["parameters"] = ps
		}
		item[strings.ToLower(o.Method)] = op
	}
	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "MacPanel API",
			"version":     "1",
			"description": "Diturunkan otomatis dari tabel rute Go (cmd/genapi). Hanya path, metode, tingkat akses (x-level), dan ketersediaan lewat token API (x-token). Skema badan permintaan dan respons tidak disertakan.",
		},
		"paths": paths,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil // encoding/json mengurutkan kunci map, jadi keluaran deterministik
}

// tsPath mengubah "/api/sites/{name}/ssl" menjadi tipe literal templat "`/api/sites/${string}/ssl`".
func tsPath(p string) string {
	if !strings.Contains(p, "{") {
		return fmt.Sprintf("%q", p)
	}
	return "`" + param.ReplaceAllLiteralString(p, "${string}") + "`"
}

func union(ops []Op, method string) string {
	var parts []string
	for _, o := range ops {
		if o.Method == method {
			parts = append(parts, "  | "+tsPath(o.Path))
		}
	}
	if len(parts) == 0 {
		return "never"
	}
	return "\n" + strings.Join(parts, "\n")
}

// TypeScript membangun api.gen.ts: tipe path yang sah per metode, agar salah ketik path gagal saat kompilasi.
func TypeScript(ops []Op) []byte {
	var b strings.Builder
	b.WriteString("// DIHASILKAN oleh cmd/genapi dari tabel rute Go (internal/server). JANGAN diedit tangan; jalankan: make gen-api\n")
	b.WriteString("// Isi: path yang sah per metode. Skema respons tidak diturunkan (lihat internal/apispec).\n\n")
	b.WriteString("export type GetPath =" + union(ops, "GET") + ";\n\n")
	b.WriteString("export type PostPath =" + union(ops, "POST") + ";\n\n")
	b.WriteString("/** Path GET dengan query string opsional (mis. ?limit=25). */\nexport type GetUrl = GetPath | `${GetPath}?${string}`;\nexport type PostUrl = PostPath | `${PostPath}?${string}`;\n\n")
	b.WriteString("/** Rute yang tidak dapat dipanggil dengan token API (hanya sesi manusia). */\nexport const TOKEN_BLOCKED: readonly string[] = [\n")
	for _, o := range ops {
		if !o.Token {
			fmt.Fprintf(&b, "  %q,\n", o.Method+" "+o.Path)
		}
	}
	b.WriteString("];\n")
	return []byte(b.String())
}
