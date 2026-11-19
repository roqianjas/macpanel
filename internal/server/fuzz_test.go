package server_test

// Fuzz validasi sisi server: semua rute POST non-auth diberi masukan buruk. Dua hal yang diperiksa:
//  1. tidak ada 5xx dan tidak ada panic (server tidak boleh jatuh karena masukan);
//  2. nilai buruk TIDAK sampai ke `lab` (fake lab mencatat setiap argumen). Nilai yang lolos ke lab = validasi hilang.
// Fake lab dari testenv, jadi tidak ada yang menyentuh mesin.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/server"
	"github.com/roqianjas/macpanel/internal/store"
	"github.com/roqianjas/macpanel/internal/testenv"
)

// harvestFields mengumpulkan nama dan jenis field dari struct `var in struct {...}` di seluruh handler, supaya kunci JSON yang dikirim
// bertipe benar (tipe salah ditolak decoder sebelum validasi dan menyembunyikan celah).
func harvestFields(t *testing.T) map[string]map[string]bool {
	t.Helper()
	kinds := map[string]map[string]bool{}
	add := func(k, kind string) {
		if kinds[k] == nil {
			kinds[k] = map[string]bool{}
		}
		kinds[k][kind] = true
	}
	reBlock := regexp.MustCompile(`(?s)(?:var \w+|\w+ :=|\w+ =) ?struct \{(.*?)\n\t+\}`)
	reField := regexp.MustCompile("(?m)^\\s*([A-Za-z][A-Za-z0-9_]*)\\s+(\\S+)(?:\\s+`json:\"([a-z_0-9]+))?")
	_ = filepath.Walk("..", func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for _, m := range reBlock.FindAllStringSubmatch(string(b), -1) {
			for _, f := range reField.FindAllStringSubmatch(m[1], -1) {
				key := strings.ToLower(f[1])
				if f[3] != "" {
					key = f[3]
				}
				ty := f[2]
				switch {
				case ty == "string":
					add(key, "string")
				case strings.HasPrefix(ty, "int") || strings.HasPrefix(ty, "uint") || ty == "*int" || ty == "float64":
					add(key, "int")
				case ty == "bool" || ty == "*bool":
					add(key, "bool")
				case ty == "[]string":
					add(key, "strs")
				}
			}
		}
		return nil
	})
	for _, k := range []string{"name", "user", "username", "password", "engine", "db", "host", "path", "file", "ref", "branch", "pattern", "entry", "target", "mode", "php", "version", "id", "label", "command", "cmd", "dir", "src", "dst", "domain", "content", "text", "key", "value", "kind", "type", "token", "scope", "word", "typed", "confirm", "from", "to", "url", "title", "role", "code"} {
		add(k, "string")
	}
	return kinds
}

// payload membuat objek JSON bertipe. preferStr: bila sebuah kunci punya beberapa tipe di handler berbeda, pilih string (true) atau angka (false).
func payload(kinds map[string]map[string]bool, val string, preferStr bool) string {
	keys := make([]string, 0, len(kinds))
	for k := range kinds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for _, k := range keys {
		m := kinds[k]
		var v string
		switch {
		case m["string"] && (preferStr || (!m["int"] && !m["bool"] && !m["strs"])):
			v = jsonStr(val)
		case m["strs"]:
			v = "[" + jsonStr(val) + "]"
		case m["int"]:
			v = "1"
		case m["bool"]:
			v = "true"
		case m["string"]:
			v = jsonStr(val)
		default:
			continue
		}
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(`"` + k + `":` + v)
	}
	sb.WriteString("}")
	return sb.String()
}

// validVals: nilai wajar per kunci supaya permintaan dasar lolos validasi dan mencapai lab; satu kunci lalu diganti nilai buruk.
var routeVals = map[string]map[string]string{
	"/api/installer":      {"kind": `"laravel"`, "runtime": `"node"`},
	"/api/backups/extras": {"kind": `"db"`},
	"/api/redis/get":      {"db": `0`},
	"/api/redis/del":      {"db": `0`},
	"/api/db/{engine}/{name}/dbusers/{user}/privs": {"privs": `"ro"`},
	"/api/docker/images/remove":                    {"confirm": `"nginx:1.27"`},
	"/api/dbroot/{engine}/passwd":                  {"confirm": `"mysql"`},
	"/api/php/{version}/remove":                    {"confirm": `"8.3"`},
	"/api/sites/{name}/deploy":                     {},
	"/api/exposure/allow":                          {"entry": `"foo:9999"`},
	"/api/exposure/revoke":                         {"entry": `"foo:9999"`},
}

var validVals = map[string]string{
	"name": `"demo"`, "engine": `"mysql"`, "db": `"proj"`, "user": `"alice"`, "username": `"alice"`, "password": `"kata-sandi-panjang-123"`, "host": `"demo.lab"`,
	"path": `"proj"`, "file": `"proj"`, "ref": `"nginx:1.27"`, "branch": `"main"`, "pattern": `"user:*"`, "entry": `"nginx:80"`, "target": `"3000"`, "mode": `"proxy"`,
	"php": `"8.3"`, "version": `"8.3"`, "id": `"nginx"`, "label": `"demo"`, "command": `"php artisan queue:work"`, "cmd": `"php artisan schedule:run"`, "dir": `"proj"`,
	"src": `"demo"`, "dst": `"demo2"`, "domain": `"demo.lab"`, "content": `"A=1"`, "text": `"A=1"`, "key": `"k"`, "value": `"v"`, "kind": `"dump"`, "type": `"laravel"`,
	"scope": `"read"`, "word": `"demo"`, "typed": `"demo"`, "confirm": `"demo"`, "url": `"http://127.0.0.1:3000"`, "title": `"x"`, "role": `"viewer"`, "code": `"123456"`,
	"time": `"02:30"`, "every": `"5m"`, "at": `"02:30"`, "port": `3000`, "procs": `1`, "keep": `7`, "days": `0`, "extra_days": `0`, "every_days": `1`, "enabled": `true`,
}

func baseline(kinds map[string]map[string]bool, override, val string, extra map[string]string) string {
	keys := make([]string, 0, len(kinds))
	for k := range kinds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		m := kinds[k]
		v, ok := validVals[k]
		if e, has := extra[k]; has {
			v, ok = e, true
		}
		if !ok {
			switch {
			case m["string"]:
				v = `"demo"`
			case m["int"]:
				v = "1"
			case m["bool"]:
				v = "true"
			case m["strs"]:
				v = `["demo"]`
			default:
				continue
			}
		}
		if m["strs"] && !m["string"] && !strings.HasPrefix(v, "[") {
			v = "[" + v + "]"
		}
		if k == override {
			if m["strs"] && !m["string"] {
				v = "[" + jsonStr(val) + "]"
			} else {
				v = jsonStr(val)
			}
		}
		parts = append(parts, `"`+k+`":`+v)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// routeFields membaca handler lewat go/ast: struct yang dipakai DecodeJSON di handler rute itu. Decoder menolak kunci tak dikenal,
// jadi payload harus memakai kunci handler itu sendiri, bukan gabungan semua handler.
func routeFields(t *testing.T, h func(*core.App, http.ResponseWriter, *http.Request, *store.Session)) map[string]map[string]bool {
	t.Helper()
	full := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	i := strings.LastIndex(full, "/internal/")
	if i < 0 {
		return nil
	}
	rest := full[i+len("/internal/"):] // pkg.Func
	dot := strings.Index(rest, ".")
	pkg, fn := rest[:dot], rest[dot+1:]
	fset := token.NewFileSet()
	ents, err := os.ReadDir(filepath.Join("..", pkg))
	if err != nil {
		return nil
	}
	types := map[string]*ast.StructType{}
	var decl *ast.FuncDecl
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join("..", pkg, e.Name()), nil, 0)
		if err != nil {
			continue
		}
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							types[ts.Name.Name] = st
						}
					}
				}
			case *ast.FuncDecl:
				if x.Name.Name == fn && x.Recv == nil {
					decl = x
				}
			}
		}
	}
	if decl == nil || decl.Body == nil {
		return nil
	}
	var st *ast.StructType
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		if st != nil {
			return false
		}
		switch x := n.(type) {
		case *ast.ValueSpec: // var in struct{...}  /  var in T
			if len(x.Names) == 1 {
				switch ty := x.Type.(type) {
				case *ast.StructType:
					st = ty
				case *ast.Ident:
					if types[ty.Name] != nil && (x.Names[0].Name == "in" || x.Names[0].Name == "req" || x.Names[0].Name == "body" || x.Names[0].Name == "p") {
						st = types[ty.Name]
					}
				}
			}
		}
		return true
	})
	if st == nil {
		return nil
	}
	out := map[string]map[string]bool{}
	for _, f := range st.Fields.List {
		kind := ""
		switch ty := f.Type.(type) {
		case *ast.Ident:
			switch {
			case ty.Name == "string":
				kind = "string"
			case ty.Name == "bool":
				kind = "bool"
			case strings.HasPrefix(ty.Name, "int") || strings.HasPrefix(ty.Name, "uint") || ty.Name == "float64":
				kind = "int"
			}
		case *ast.StarExpr:
			if id, ok := ty.X.(*ast.Ident); ok {
				if id.Name == "bool" {
					kind = "bool"
				} else if strings.HasPrefix(id.Name, "int") {
					kind = "int"
				}
			}
		case *ast.ArrayType:
			if id, ok := ty.Elt.(*ast.Ident); ok && id.Name == "string" {
				kind = "strs"
			}
		}
		for _, nm := range f.Names {
			key := strings.ToLower(nm.Name)
			if f.Tag != nil {
				if m := regexp.MustCompile(`json:"([A-Za-z_0-9]+)`).FindStringSubmatch(f.Tag.Value); m != nil {
					key = m[1]
				}
			}
			if kind == "" {
				kind = "other"
			}
			out[key] = map[string]bool{kind: true}
		}
	}
	return out
}

type hostile struct{ tag, val string }

var hostileStrings = []hostile{
	{"zzflag", "--zzflag-x"},
	{"zzdash", "-zzrf"},
	{"zztrav", "../zztrav/../../etc/passwd"},
	{"zzabs", "/etc/zzabs"},
	{"zznul", "zznul\x00x"},
	{"zzsub", "zzsub$(id)"},
	{"zztick", "zztick`id`"},
	{"zzsemi", "zzsemi;rm -rf /"},
	{"zznl", "zznl\nINJECT"},
	{"zzspace", "zz space"},
	{"zzquote", "zzq'\"\\"},
	{"zzlong", "zzlong" + strings.Repeat("A", 6000)},
	{"zzuni", "zzuni\u202e\u0000\ufeff"},
	{"zzglob", "zzglob*?[a-z]"},
}

func TestFuzzPostRoutesRejectHostileInput(t *testing.T) {
	o := testenv.NewOpsEnv(t, "")
	o.Stepup()
	fields := harvestFields(t)
	t.Logf("kunci dikenal: %d", len(fields))
	var routes []string
	findings := map[string][]string{}
	var fivexx []string
	codes := map[int]int{}
	per := map[string]map[int]int{}

	for _, rt := range server.Routes() {
		if rt.Method != "POST" || strings.HasPrefix(rt.Path, "/api/auth/") {
			continue
		}
		routes = append(routes, rt.Path)
		for _, h := range hostileStrings {
			path := regexp.MustCompile(`\{[a-z_]+\}`).ReplaceAllStringFunc(rt.Path, func(string) string { return url.PathEscape(h.val) })
			objs := []string{payload(fields, h.val, true), payload(fields, h.val, false), `{}`, `null`, `[]`, `"zz"`, `{"name":`, `{"port":-1,"procs":99999999999,"keep":-5,"days":-1,"every_days":999}`}
			for _, body := range objs {
				before := len(o.Calls())
				w := o.Do("POST", path, body, o.Cookie, o.Csrf, nil)
				codes[w.Code]++
				if per[rt.Path] == nil {
					per[rt.Path] = map[int]int{}
				}
				per[rt.Path][w.Code]++
				if w.Code >= 500 && w.Code != 502 {
					fivexx = append(fivexx, rt.Path+" "+h.tag+" -> "+itoa(w.Code))
				}
				newCalls := o.Calls()[before:]
				if strings.Contains(newCalls, h.tag) {
					findings[rt.Path] = appendUniq(findings[rt.Path], h.tag)
				}
			}
		}
	}
	for p, m := range per {
		if m[429]+m[404]+m[409]+m[200]+m[202] > 0 {
			t.Logf("%s %v", p, m)
		}
	}
	// Per kunci: permintaan dasar yang valid, lalu satu kunci diganti nilai buruk.
	unexercised := []string{}
	for _, rt := range server.Routes() {
		if rt.Method != "POST" || strings.HasPrefix(rt.Path, "/api/auth/") {
			continue
		}
		pathOf := func(v string) string {
			return regexp.MustCompile(`\{[a-z_]+\}`).ReplaceAllStringFunc(rt.Path, func(m string) string {
				switch m {
				case "{engine}":
					return "mysql"
				case "{id}":
					return "nginx"
				case "{version}":
					return "8.3"
				case "{tool}":
					return "node"
				case "{kind}":
					return "worker"
				case "{user}":
					return "alice"
				case "{project}":
					return "proj"
				case "{action}":
					switch {
					case strings.HasPrefix(rt.Path, "/api/composer"):
						return "install"
					case strings.Contains(rt.Path, "/ext/"):
						return "enable"
					case strings.HasPrefix(rt.Path, "/api/docker/compose"):
						return "up"
					case strings.HasPrefix(rt.Path, "/api/docker/containers"):
						return "restart"
					case strings.HasPrefix(rt.Path, "/api/sites"):
						return "enable"
					}
					return "restart"
				}
				return v
			})
		}
		rf := routeFields(t, rt.H)
		keys := make([]string, 0, len(rf))
		for k := range rf {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		before := len(o.Calls())
		w := o.Do("POST", pathOf("demo"), baseline(rf, "", "", routeVals[rt.Path]), o.Cookie, o.Csrf, nil)
		if w.Code == 202 {
			time.Sleep(150 * time.Millisecond) // pekerjaan asinkron: lab dipanggil setelah respons
		}
		reached := len(o.Calls()) > before
		if !reached || w.Code >= 300 {
			unexercised = append(unexercised, rt.Path+" ("+strconv.Itoa(w.Code)+")")
			continue
		}
		for _, k := range keys {
			if !rf[k]["string"] && !rf[k]["strs"] {
				continue
			}
			for _, h := range hostileStrings {
				before := len(o.Calls())
				w := o.Do("POST", pathOf("demo"), baseline(rf, k, h.val, routeVals[rt.Path]), o.Cookie, o.Csrf, nil)
				if w.Code == 202 {
					time.Sleep(60 * time.Millisecond)
				}
				if w.Code >= 500 && w.Code != 502 {
					fivexx = append(fivexx, rt.Path+" kunci "+k+" "+h.tag+" -> "+strconv.Itoa(w.Code))
				}
				if strings.Contains(o.Calls()[before:], h.tag) {
					findings[rt.Path+" kunci="+k] = appendUniq(findings[rt.Path+" kunci="+k], h.tag)
				}
			}
		}
	}
	t.Logf("rute POST yang dasarnya tidak mencapai lab (tidak teruji per kunci): %d: %s", len(unexercised), strings.Join(unexercised, "; "))
	t.Logf("rute=%d kode=%v panggilan_lab=%d", len(routes), codes, strings.Count(o.Calls(), "CALL:"))
	if len(routes) < 50 {
		t.Fatalf("hanya %d rute POST terdeteksi; daftar rute berubah?", len(routes))
	}
	if len(fivexx) > 0 {
		t.Errorf("respons 5xx untuk masukan buruk:\n%s", strings.Join(fivexx, "\n"))
	}
	if os.Getenv("FUZZ_REPORT") != "" || len(findings) > 0 {
		var keys []string
		for k := range findings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var lines []string
		for _, k := range keys {
			lines = append(lines, k+" <- "+strings.Join(findings[k], ","))
		}
		if len(lines) > 0 {
			t.Errorf("nilai buruk sampai ke lab (%d rute dari %d):\n%s", len(lines), len(routes), strings.Join(lines, "\n"))
		}
	}
}

func appendUniq(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func jsonStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20:
			b.WriteString(`\u00`)
			b.WriteString(string("0123456789abcdef"[r>>4]))
			b.WriteString(string("0123456789abcdef"[r&15]))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
