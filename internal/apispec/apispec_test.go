package apispec

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

func h(*core.App, http.ResponseWriter, *http.Request, *store.Session) {}

func rt(m, p string, l core.Level) core.Route { return core.Route{Method: m, Path: p, Level: l, H: h} }

func TestOpsSortedWithParamsLevelsAndTokenFlag(t *testing.T) {
	ops, err := Ops([]core.Route{
		rt("POST", "/api/sites/{name}/disable", core.Full),
		rt("GET", "/api/sites", core.Full),
		rt("GET", "/healthz", core.Public),
		rt("GET", "/api/me/devices", core.Full),
		rt("GET", "/api/db/{engine}/{name}/dbusers", core.Full),
	})
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, o := range ops {
		order = append(order, o.Method+" "+o.Path)
	}
	want := "GET /api/db/{engine}/{name}/dbusers|GET /api/me/devices|GET /api/sites|POST /api/sites/{name}/disable|GET /healthz"
	if strings.Join(order, "|") != want {
		t.Fatalf("urutan:\n got %v\nwant %s", order, want)
	}
	by := map[string]Op{}
	for _, o := range ops {
		by[o.Method+" "+o.Path] = o
	}
	if p := by["GET /api/db/{engine}/{name}/dbusers"].Params; len(p) != 2 || p[0] != "engine" || p[1] != "name" {
		t.Fatalf("params: %v", p)
	}
	if by["GET /healthz"].Level != "public" || by["GET /api/sites"].Level != "full" {
		t.Fatalf("level: %+v", by)
	}
	if by["GET /api/me/devices"].Token {
		t.Fatal("/api/me/* tidak boleh lewat token")
	}
	if !by["GET /api/sites"].Token {
		t.Fatal("/api/sites boleh lewat token")
	}
}

func TestOpsRejectsDuplicateBadMethodAndBadPath(t *testing.T) {
	if _, err := Ops([]core.Route{rt("GET", "/a", core.Full), rt("GET", "/a", core.Full)}); err == nil {
		t.Error("rute ganda harus ditolak")
	}
	if _, err := Ops([]core.Route{rt("DELETE", "/a", core.Full)}); err == nil {
		t.Error("metode tak dikenal harus ditolak (jangan diam-diam hilang dari tipe)")
	}
	if _, err := Ops([]core.Route{rt("GET", "api/x", core.Full)}); err == nil {
		t.Error("path tanpa / harus ditolak")
	}
	if _, err := Ops([]core.Route{rt("GET", "/a", core.Full), rt("POST", "/a", core.Full)}); err != nil {
		t.Errorf("metode berbeda pada path sama sah: %v", err)
	}
}

func TestOperationIDStableAndUnique(t *testing.T) {
	ops, _ := Ops([]core.Route{rt("GET", "/api/sites/{name}/settings", core.Full), rt("POST", "/api/sites/{name}/settings", core.Full), rt("GET", "/healthz", core.Public)})
	ids := map[string]bool{}
	for _, o := range ops {
		ids[OperationID(o)] = true
	}
	for _, id := range []string{"get_api_sites_name_settings", "post_api_sites_name_settings", "get_healthz"} {
		if !ids[id] {
			t.Errorf("operationId %q tidak ada: %v", id, ids)
		}
	}
	if len(ids) != 3 {
		t.Errorf("operationId harus unik: %v", ids)
	}
}

func TestOpenAPIStructureAndDeterminism(t *testing.T) {
	routes := []core.Route{rt("GET", "/api/sites/{name}/ssl", core.Full), rt("POST", "/api/sites/{name}/ssl", core.Full), rt("GET", "/healthz", core.Public)}
	ops, _ := Ops(routes)
	a, err := OpenAPI(ops)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := OpenAPI(ops)
	if string(a) != string(b) {
		t.Fatal("keluaran harus deterministik")
	}
	if !strings.HasSuffix(string(a), "\n") {
		t.Fatal("harus diakhiri newline")
	}
	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Level       string `json:"x-level"`
			Token       bool   `json:"x-token"`
			Parameters  []struct {
				Name     string `json:"name"`
				In       string `json:"in"`
				Required bool   `json:"required"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(a, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OpenAPI != "3.1.0" || len(doc.Paths) != 2 {
		t.Fatalf("%+v", doc)
	}
	g := doc.Paths["/api/sites/{name}/ssl"]["get"]
	if g.OperationID != "get_api_sites_name_ssl" || g.Level != "full" || !g.Token || len(g.Parameters) != 1 || g.Parameters[0].In != "path" || !g.Parameters[0].Required {
		t.Fatalf("get ssl: %+v", g)
	}
	if _, ok := doc.Paths["/api/sites/{name}/ssl"]["post"]; !ok {
		t.Fatal("post ssl hilang")
	}
	if doc.Paths["/healthz"]["get"].Level != "public" || len(doc.Paths["/healthz"]["get"].Parameters) != 0 {
		t.Fatalf("healthz: %+v", doc.Paths["/healthz"]["get"])
	}
}

func TestTypeScriptUnionsAndTokenBlocked(t *testing.T) {
	ops, _ := Ops([]core.Route{rt("GET", "/api/sites", core.Full), rt("GET", "/api/sites/{name}/ssl", core.Full), rt("POST", "/api/sites/{name}/disable", core.Full), rt("GET", "/api/me/devices", core.Full)})
	ts := string(TypeScript(ops))
	for _, want := range []string{
		"export type GetPath =",
		`  | "/api/sites"`,
		"  | `/api/sites/${string}/ssl`",
		"export type PostPath =",
		"  | `/api/sites/${string}/disable`",
		"export type GetUrl = GetPath | `${GetPath}?${string}`;",
		`"GET /api/me/devices",`,
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("tidak ada %q dalam:\n%s", want, ts)
		}
	}
	if strings.Contains(ts, `"GET /api/sites",`) {
		t.Error("/api/sites boleh lewat token, tidak boleh ada di TOKEN_BLOCKED")
	}
	if i := strings.Index(ts, "export type PostPath"); strings.Contains(ts[:i], "disable") {
		t.Error("rute POST bocor ke GetPath")
	}
}

func TestTypeScriptWithNoPostRoutesIsNever(t *testing.T) {
	ops, _ := Ops([]core.Route{rt("GET", "/a", core.Full)})
	if !strings.Contains(string(TypeScript(ops)), "export type PostPath =never;") {
		t.Fatalf("%s", TypeScript(ops))
	}
}
