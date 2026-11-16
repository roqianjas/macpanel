package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeNginx = `D=$(dirname "$0")
case "$1 $2" in
"nginx test") [ -f "$D/fail_test" ] && { echo "nginx: [emerg] direktif salah"; exit 1; }; echo "test ok";;
"nginx reload") [ -f "$D/fail_reload" ] && { echo "reload gagal"; exit 1; }; echo "reload ok";;
*) echo "ok $1 $2";;
esac`

// siteEnv: opsEnv + direktori nginx/cert/log sementara dan situs yang bisa disunting.
func newSiteEnv(t *testing.T) *opsEnv {
	t.Helper()
	o := newOpsEnv(t, fakeNginx)
	o.app.nginxDir = filepath.Join(o.home, "nginx")
	o.app.certDir = filepath.Join(o.home, "certs")
	o.app.logDir = filepath.Join(o.home, "logs")
	o.app.errLog = filepath.Join(o.home, "error.log")
	os.MkdirAll(o.app.certDir, 0o755)
	return o
}

func (o *opsEnv) siteConf(name string) string {
	b, _ := os.ReadFile(filepath.Join(o.home, "sites", name+".conf"))
	return string(b)
}

func (o *opsEnv) public() string { return filepath.Join(o.home, "Developer/proj/public") }

func (o *opsEnv) goodModel(mode string) string {
	switch mode {
	case "php":
		return `{"mode":"php","target":"` + o.public() + `","php":"8.4","preset":"laravel","max_body_mb":64,"read_timeout_s":120,"force_https":true,"gzip":true}`
	case "statis":
		return `{"mode":"statis","target":"` + o.public() + `","preset":"spa","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":false}`
	}
	return `{"mode":"proxy","target":"3001","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":false}`
}

// ---------- parser ----------

func TestParserTokenization(t *testing.T) {
	d, err := parseNginx(`server { # komentar include /etc/x;
  return 200 "a;b {c}";
  add_header X 'q"r';
  location ~ \.php$ { return 404; }
}`)
	if err != nil || len(d) != 1 || len(d[0].Block) != 3 {
		t.Fatalf("%v %+v", err, d)
	}
	if got := d[0].Block[0].Args; len(got) != 2 || got[1] != "a;b {c}" {
		t.Fatalf("string berkutip salah: %q", got)
	}
	if d[0].Block[2].Args[1] != `\.php$` {
		t.Fatalf("regex: %q", d[0].Block[2].Args)
	}
}

func TestParserNoHiddenDirectives(t *testing.T) {
	// upaya menyelundupkan direktif lewat kutip, backslash, dan komentar: tidak boleh muncul sebagai direktif
	for _, src := range []string{
		`server { return 200 "x\"; allow all; \""; }`,
		`server { return 200 foo\;allow all; }`,
		`server { return 200 'x'; # ; allow all;
}`,
		"server { add_header X \"a\n; allow all;\"; }",
	} {
		d, err := parseNginx(src)
		if err != nil {
			continue
		}
		var walk func([]*Dir)
		walk = func(ds []*Dir) {
			for _, x := range ds {
				if x.Name == "allow" || x.Name == "deny" {
					t.Errorf("direktif %s terselundup di: %s", x.Name, src)
				}
				walk(x.Block)
			}
		}
		walk(d)
	}
}

func TestParserRejects(t *testing.T) {
	for name, src := range map[string]string{
		"kutip tak ditutup": `server { return 200 "x; }`, "kurung kurang": `server { return 200;`, "kurung lebih": `server { } }`, "titik koma sendiri": `server { ; }`,
		"${": `server { return 200 ${x}; }`, "nul": "server { return 200 \x00; }", "terlalu dalam": `a{b{c{d{e{f{}}}}}}`, "kutip lalu huruf": `server { return 200 "x"y; }`,
		"utf8 rusak": "server { return 200 \xff; }", "backslash akhir": `server { return 200 \`,
		"${ yang tampak sah sebagai blok": "server { location /x${ return 404; } }", "$ lalu kurung": "server { location /a${b} { return 404; } }",
	} {
		if _, err := parseNginx(src); err == nil {
			t.Errorf("%s: harus ditolak", name)
		}
	}
	if _, err := parseNginx(strings.Repeat("#", maxConfSize+1)); err == nil {
		t.Error("file terlalu besar harus ditolak")
	}
}

// ---------- kebijakan ----------

func TestPolicyAcceptsAllTemplatesAndLegacy(t *testing.T) {
	o := newSiteEnv(t)
	for _, m := range []string{
		`{"mode":"php","target":"X","php":"8.2","preset":"laravel","max_body_mb":64,"read_timeout_s":120,"force_https":true,"gzip":true}`,
		`{"mode":"php","target":"X","php":"8.5","preset":"wordpress","max_body_mb":64,"read_timeout_s":120,"force_https":false,"gzip":false,"rewrites":[{"from":"^/old/(.*)$","to":"/new/$1","flag":"permanent"}]}`,
		`{"mode":"statis","target":"X","preset":"static","max_body_mb":1,"read_timeout_s":10,"force_https":true,"gzip":true}`,
		`{"mode":"statis","target":"X","preset":"spa","max_body_mb":1,"read_timeout_s":10,"force_https":false,"gzip":false}`,
		`{"mode":"proxy","target":"3001","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":false}`,
	} {
		m = strings.ReplaceAll(m, `"X"`, `"`+o.public()+`"`)
		var md Model
		if err := jsonStrict(m, &md); err != nil {
			t.Fatal(err)
		}
		g, err := o.app.generate("toko", md)
		if err != nil {
			t.Fatalf("generate: %v (%s)", err, m)
		}
		if err := o.app.checkConf("toko", g); err != nil {
			t.Errorf("template ditolak kebijakan sendiri: %v\n%s", err, g)
		}
	}
	// format yang dibuat addsite.sh (kuma.lab)
	legacy := "server {\n    listen 80; listen [::]:80;\n    server_name toko.lab;\n    include " + o.app.nginxDir + "/snippets/tailnet_only.conf;\n    return 301 https://$host$request_uri;\n}\nserver {\n    listen 443 ssl; listen [::]:443 ssl;\n    http2 on;\n    server_name toko.lab;\n    include " + o.app.nginxDir + "/snippets/tailnet_only.conf;\n    ssl_certificate     " + o.app.certDir + "/toko.lab.pem;\n    ssl_certificate_key " + o.app.certDir + "/toko.lab-key.pem;\n    location / {\n        include " + o.app.nginxDir + "/snippets/proxy.conf;\n        proxy_pass http://127.0.0.1:3001;\n    }\n}\n"
	if err := o.app.checkConf("toko", legacy); err != nil {
		t.Errorf("format addsite.sh ditolak: %v", err)
	}
}

func jsonStrict(s string, v any) error {
	return decodeJSONString(s, v)
}

// base menghasilkan config sah yang kemudian dirusak satu hal per kasus.
func (o *opsEnv) base() string {
	return "server {\n    listen 80;\n    server_name toko.lab;\n    include " + o.app.nginxDir + "/snippets/tailnet_only.conf;\n    return 301 https://$host$request_uri;\n}\nserver {\n    listen 443 ssl;\n    server_name toko.lab;\n    include " + o.app.nginxDir + "/snippets/tailnet_only.conf;\n    ssl_certificate " + o.app.certDir + "/toko.lab.pem;\n    ssl_certificate_key " + o.app.certDir + "/toko.lab-key.pem;\n    root " + o.public() + ";\n    location / { try_files $uri =404; }\n}\n"
}

func TestPolicyRejects(t *testing.T) {
	o := newSiteEnv(t)
	if err := o.app.checkConf("toko", o.base()); err != nil {
		t.Fatalf("basis harus sah: %v", err)
	}
	snip := o.app.nginxDir + "/snippets/tailnet_only.conf"
	cases := map[string]func(string) string{
		"tanpa pembatas tailnet":  func(s string) string { return strings.ReplaceAll(s, "include "+snip+";", "") },
		"tailnet hanya di 1 blok": func(s string) string { return strings.Replace(s, "include "+snip+";", "", 1) },
		"allow all di location": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "allow all; try_files $uri =404;", 1)
		},
		"deny":                func(s string) string { return strings.Replace(s, "try_files $uri =404;", "deny 1.2.3.4;", 1) },
		"satisfy any":         func(s string) string { return strings.Replace(s, "try_files $uri =404;", "satisfy any;", 1) },
		"include /etc/passwd": func(s string) string { return strings.Replace(s, "root ", "include /etc/passwd; root ", 1) },
		"include glob":        func(s string) string { return strings.Replace(s, "root ", "include /etc/nginx/*.conf; root ", 1) },
		"include snippet lain": func(s string) string {
			return strings.Replace(s, "root ", "include "+o.app.nginxDir+"/snippets/lain.conf; root ", 1)
		},
		"load_module":     func(s string) string { return "load_module /x.so;\n" + s },
		"http block":      func(s string) string { return "http { }\n" + s },
		"map":             func(s string) string { return "map $a $b { default 1; }\n" + s },
		"upstream":        func(s string) string { return "upstream x { server 1.1.1.1; }\n" + s },
		"if":              func(s string) string { return strings.Replace(s, "root ", "if ($a) { return 200; } root ", 1) },
		"set":             func(s string) string { return strings.Replace(s, "root ", "set $a b; root ", 1) },
		"perl":            func(s string) string { return strings.Replace(s, "root ", "perl_set $x 'sub {}'; root ", 1) },
		"lua":             func(s string) string { return strings.Replace(s, "root ", "content_by_lua_block { }; root ", 1) },
		"error_log":       func(s string) string { return strings.Replace(s, "root ", "error_log /etc/x; root ", 1) },
		"access_log luar": func(s string) string { return strings.Replace(s, "root ", "access_log /etc/x; root ", 1) },
		"alias":           func(s string) string { return strings.Replace(s, "try_files $uri =404;", "alias "+o.public()+"/;", 1) },
		"root /etc":       func(s string) string { return strings.Replace(s, "root "+o.public(), "root /etc", 1) },
		"root variabel":   func(s string) string { return strings.Replace(s, "root "+o.public(), "root $document_root", 1) },
		"root traversal": func(s string) string {
			return strings.Replace(s, "root "+o.public(), "root "+o.public()+"/../../../outside", 1)
		},
		"root symlink keluar": func(s string) string {
			return strings.Replace(s, "root "+o.public(), "root "+o.home+"/Developer/escape", 1)
		},
		"proxy ke host lain": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "proxy_pass http://10.0.0.5:80;", 1)
		},
		"proxy ke port ssh": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "proxy_pass http://127.0.0.1:22;", 1)
		},
		"proxy ke db": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "proxy_pass http://127.0.0.1:5432;", 1)
		},
		"proxy userinfo": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "proxy_pass http://127.0.0.1:3000@evil.com/;", 1)
		},
		"proxy localhost": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "proxy_pass http://localhost:3000;", 1)
		},
		"proxy variabel": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "proxy_pass $scheme://$host;", 1)
		},
		"fastcgi tcp": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "fastcgi_pass 127.0.0.1:9000;", 1)
		},
		"fastcgi socket lain": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "fastcgi_pass unix:/tmp/x.sock;", 1)
		},
		"fastcgi PHP_VALUE": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", `fastcgi_param PHP_VALUE "auto_prepend_file=/x";`, 1)
		},
		"fastcgi PHP_ADMIN_VALUE": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", `fastcgi_param PHP_ADMIN_VALUE "open_basedir=/";`, 1)
		},
		"SCRIPT_FILENAME bebas": func(s string) string {
			return strings.Replace(s, "try_files $uri =404;", "fastcgi_param SCRIPT_FILENAME /etc/passwd;", 1)
		},
		"listen 8080":            func(s string) string { return strings.Replace(s, "listen 80;", "listen 8080;", 1) },
		"listen semua antarmuka": func(s string) string { return strings.Replace(s, "listen 80;", "listen 0.0.0.0:80;", 1) },
		"listen default_server":  func(s string) string { return strings.Replace(s, "listen 80;", "listen 80 default_server;", 1) },
		"server_name lain":       func(s string) string { return strings.Replace(s, "server_name toko.lab;", "server_name lain.lab;", 1) },
		"server_name wildcard":   func(s string) string { return strings.Replace(s, "server_name toko.lab;", "server_name *.lab;", 1) },
		"server_name ganda": func(s string) string {
			return strings.Replace(s, "server_name toko.lab;", "server_name toko.lab home.lab;", 1)
		},
		"sertifikat situs lain": func(s string) string { return strings.ReplaceAll(s, "toko.lab.pem", "home.lab.pem") },
		"kunci di luar": func(s string) string {
			return strings.ReplaceAll(s, o.app.certDir+"/toko.lab-key.pem", "/etc/ssl/private/k.pem")
		},
		"server bersarang":         func(s string) string { return strings.Replace(s, "root ", "server { } root ", 1) },
		"location tanpa blok":      func(s string) string { return strings.Replace(s, "root ", "location /x; root ", 1) },
		"direktif asing":           func(s string) string { return strings.Replace(s, "root ", "user root; root ", 1) },
		"blok pada direktif biasa": func(s string) string { return strings.Replace(s, "root ", "return 200 { } root ", 1) },
		"modifier location aneh":   func(s string) string { return strings.Replace(s, "location /", "location @x /", 1) },
		"rewrite flag asing":       func(s string) string { return strings.Replace(s, "root ", "rewrite a b evil; root ", 1) },
		"4 server":                 func(s string) string { return s + s },
		"tanpa server":             func(s string) string { return "# kosong\n" },
	}
	for name, f := range cases {
		if err := o.app.checkConf("toko", f(o.base())); err == nil {
			t.Errorf("harus ditolak: %s", name)
		}
	}
}

func TestPolicyIgnoresComments(t *testing.T) {
	o := newSiteEnv(t)
	if err := o.app.checkConf("toko", "# allow all; include /etc/passwd; load_module x;\n"+o.base()); err != nil {
		t.Fatalf("komentar tidak boleh memengaruhi: %v", err)
	}
}

func TestModelValidation(t *testing.T) {
	o := newSiteEnv(t)
	pub := o.public()
	bad := []Model{
		{Mode: "php", Target: pub, PHP: "8.1", MaxBody: 1, ReadTimeout: 60}, {Mode: "php", Target: "3000", PHP: "8.4", MaxBody: 1, ReadTimeout: 60},
		{Mode: "proxy", Target: pub, MaxBody: 1, ReadTimeout: 60}, {Mode: "proxy", Target: "22", MaxBody: 1, ReadTimeout: 60}, {Mode: "statis", Target: "/etc", MaxBody: 1, ReadTimeout: 60},
		{Mode: "xx", Target: pub, MaxBody: 1, ReadTimeout: 60}, {Mode: "statis", Target: pub, MaxBody: 0, ReadTimeout: 60}, {Mode: "statis", Target: pub, MaxBody: 5000, ReadTimeout: 60},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 5}, {Mode: "statis", Target: pub, Preset: "aneh", MaxBody: 1, ReadTimeout: 60},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []Rewrite{{From: "a b", To: "/x"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []Rewrite{{From: "^/a$", To: "/x; allow all"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []Rewrite{{From: "^/a$", To: "evil.com"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []Rewrite{{From: "^/a$", To: "/x", Flag: "x"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []Rewrite{{From: "^/a$\n", To: "/x"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []Rewrite{{From: "a{2}", To: "/x"}}},
	}
	for i, m := range bad {
		if _, err := o.app.generate("toko", m); err == nil {
			t.Errorf("model buruk #%d lolos: %+v", i, m)
		}
	}
	many := Model{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60}
	for i := 0; i < 51; i++ {
		many.Rewrites = append(many.Rewrites, Rewrite{From: "^/a$", To: "/b"})
	}
	if _, err := o.app.generate("toko", many); err == nil {
		t.Error("51 rewrite harus ditolak")
	}
}

// ---------- pipeline ----------

func TestSettingsWriteApplyAndRoundTrip(t *testing.T) {
	o := newSiteEnv(t)
	// situs "demo" sudah ada (legacy); simpan pengaturan pertama -> versi lama tersimpan
	before := o.siteConf("demo")
	c, b := o.post("/api/sites/demo/settings", o.goodModel("php"))
	if c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	now := o.siteConf("demo")
	if !strings.HasPrefix(now, modelHeader) || !strings.Contains(now, "php-fpm-dev-8.4.sock") || !strings.Contains(now, "access_log "+o.app.logDir+"/demo.log") {
		t.Fatalf("config tidak sesuai:\n%s", now)
	}
	if !strings.Contains(o.calls(), "CALL:[nginx][test]") || !strings.Contains(o.calls(), "CALL:[nginx][reload]") {
		t.Fatalf("nginx -t dan reload harus dipanggil: %s", o.calls())
	}
	g := o.do("GET", "/api/sites/demo/settings", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(g, `"kind":"template"`) || !strings.Contains(g, `"php":"8.4"`) {
		t.Fatalf("GET settings: %s", g)
	}
	cfg := o.do("GET", "/api/sites/demo/config", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(cfg, "sebelum: pengaturan") || !strings.Contains(cfg, "pengaturan") {
		t.Fatalf("versi tidak tersimpan: %s", cfg)
	}
	if _, err := os.Stat(filepath.Join(o.home, "sites", ".demo.conf.tmp")); err == nil {
		t.Fatal("file sementara tertinggal")
	}
	if before == now {
		t.Fatal("config tidak berubah")
	}
}

func TestSettingsRejectedBeforeAnyWrite(t *testing.T) {
	o := newSiteEnv(t)
	orig := o.siteConf("demo")
	for _, b := range []string{
		`{"mode":"php","target":"/etc","php":"8.4","max_body_mb":1,"read_timeout_s":60}`,
		`{"mode":"php","target":"` + o.public() + `","php":"8.1","max_body_mb":1,"read_timeout_s":60}`,
		`{"mode":"statis","target":"` + o.public() + `","max_body_mb":1,"read_timeout_s":60,"rewrites":[{"from":"^/a$","to":"/x;allow all","flag":""}]}`,
		`{"target":"3001"}`, `{"mode":"statis","target":"` + o.public() + `","max_body_mb":1,"read_timeout_s":60,"hack":true}`,
	} {
		if c, _ := o.post("/api/sites/demo/settings", b); c != 400 {
			t.Errorf("harus 400: %s -> %d", b, c)
		}
	}
	if o.siteConf("demo") != orig || o.calls() != "" {
		t.Fatal("config berubah atau nginx dipanggil padahal input ditolak")
	}
}

func TestSystemSitesReadOnly(t *testing.T) {
	o := newSiteEnv(t)
	o.stepup()
	for _, n := range []string{"home", "db", "nginx"} {
		os.WriteFile(filepath.Join(o.home, "sites", n+".conf"), []byte("server {\n}\n"), 0o644)
		if c, _ := o.post("/api/sites/"+n+"/settings", o.goodModel("proxy")); c != 400 {
			t.Errorf("settings %s: %d", n, c)
		}
		if c, _ := o.post("/api/sites/"+n+"/config", `{"content":"x","sha":"x"}`); c != 400 {
			t.Errorf("config %s: %d", n, c)
		}
		if c, _ := o.post("/api/sites/"+n+"/restore", `{"id":1}`); c != 400 {
			t.Errorf("restore %s: %d", n, c)
		}
		if w := o.do("GET", "/api/sites/"+n+"/config", "", o.cookie, "", nil); w.Code != 200 {
			t.Errorf("baca %s harus boleh: %d", n, w.Code)
		}
	}
	if o.calls() != "" {
		t.Fatal("lab terpanggil untuk situs sistem")
	}
}

func TestUnknownOrWeirdSiteName(t *testing.T) {
	o := newSiteEnv(t)
	for _, p := range []string{"tidakada", "a;b", "-x", "..%2F..%2Fetc", "A"} {
		if w := o.do("GET", "/api/sites/"+p+"/config", "", o.cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
}

func (o *opsEnv) rawEdit(name, content string) (int, string) {
	cur := o.siteConf(name)
	return o.post("/api/sites/"+name+"/config", `{"sha":"`+sha256hex(cur)+`","content":`+jsonQuote(content)+`}`)
}

func TestRawEditStepUpShaPolicyAndNoWriteOnReject(t *testing.T) {
	o := newSiteEnv(t)
	os.WriteFile(filepath.Join(o.home, "sites", "toko.conf"), []byte(o.base()), 0o644)
	orig := o.siteConf("toko")
	edit := strings.Replace(orig, "try_files $uri =404;", "try_files $uri /index.html;", 1)
	if c, _ := o.rawEdit("toko", edit); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	if c, _ := o.post("/api/sites/toko/config", `{"sha":"salah","content":`+jsonQuote(edit)+`}`); c != 409 {
		t.Fatalf("sha basi harus 409: %d", c)
	}
	evil := strings.Replace(orig, "try_files $uri =404;", "allow all;", 1)
	if c, b := o.rawEdit("toko", evil); c != 400 || !strings.Contains(b, "ditolak kebijakan") {
		t.Fatalf("kebijakan: %d %s", c, b)
	}
	if o.siteConf("toko") != orig || o.calls() != "" {
		t.Fatal("file berubah/nginx dipanggil untuk edit yang ditolak")
	}
	if c, b := o.rawEdit("toko", edit); c != 200 {
		t.Fatalf("edit sah: %d %s", c, b)
	}
	if o.siteConf("toko") != edit {
		t.Fatal("edit tidak tertulis")
	}
}

func TestNginxTestFailureRollsBackByteIdentical(t *testing.T) {
	o := newSiteEnv(t)
	os.WriteFile(filepath.Join(o.home, "sites", "toko.conf"), []byte(o.base()), 0o644)
	orig := o.siteConf("toko")
	o.stepup()
	os.WriteFile(filepath.Join(o.home, "fail_test"), nil, 0o644)
	edit := strings.Replace(orig, "try_files $uri =404;", "try_files $uri /index.html;", 1)
	c, b := o.rawEdit("toko", edit)
	if c != 422 || !strings.Contains(b, "config lama dipulihkan") {
		t.Fatalf("harus 422 + pesan rollback: %d %s", c, b)
	}
	if o.siteConf("toko") != orig {
		t.Fatal("ROLLBACK GAGAL: file tidak identik dengan sebelumnya")
	}
	if strings.Contains(o.calls(), "[reload]") {
		t.Fatal("reload tidak boleh dipanggil setelah nginx -t gagal")
	}
	if vs := o.app.st.Versions("toko"); len(vs) != 1 || !strings.HasPrefix(vs[0].Note, "sebelum:") {
		t.Fatalf("hanya versi 'sebelum' yang boleh ada: %+v", vs)
	}
}

func TestReloadFailureRollsBackAndReloadsAgain(t *testing.T) {
	o := newSiteEnv(t)
	os.WriteFile(filepath.Join(o.home, "sites", "toko.conf"), []byte(o.base()), 0o644)
	orig := o.siteConf("toko")
	o.stepup()
	os.WriteFile(filepath.Join(o.home, "fail_reload"), nil, 0o644)
	c, _ := o.rawEdit("toko", strings.Replace(orig, "try_files $uri =404;", "try_files $uri /i.html;", 1))
	if c != 422 || o.siteConf("toko") != orig {
		t.Fatalf("rollback saat reload gagal: %d", c)
	}
	if n := strings.Count(o.calls(), "[nginx][reload]"); n != 2 {
		t.Fatalf("setelah rollback reload harus dicoba lagi (2x), dapat %d", n)
	}
}

func TestNewSiteFileRemovedWhenFirstWriteFails(t *testing.T) {
	o := newSiteEnv(t)
	os.WriteFile(filepath.Join(o.home, "fail_test"), nil, 0o644)
	m := o.goodModel("statis")
	before := o.siteConf("demo")
	if c, _ := o.post("/api/sites/demo/settings", m); c != 422 {
		t.Fatalf("harus 422: %d", c)
	}
	if o.siteConf("demo") != before {
		t.Fatal("config legacy berubah walau nginx -t gagal")
	}
}

func TestRestoreVersion(t *testing.T) {
	o := newSiteEnv(t)
	os.WriteFile(filepath.Join(o.home, "sites", "toko.conf"), []byte(o.base()), 0o644)
	orig := o.siteConf("toko")
	o.stepup()
	o.rawEdit("toko", strings.Replace(orig, "try_files $uri =404;", "try_files $uri /index.html;", 1))
	vs := o.app.st.Versions("toko")
	var origID int64
	for _, v := range vs {
		if c, _ := o.app.st.VersionContent("toko", v.ID); c == orig {
			origID = v.ID
		}
	}
	if origID == 0 {
		t.Fatalf("versi asli tidak tersimpan: %+v", vs)
	}
	o.now = o.now.Add(stepupTTL + time.Second) // step-up habis
	if c, _ := o.post("/api/sites/toko/restore", `{"id":`+itoa(origID)+`}`); c != 403 {
		t.Fatalf("restore tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	if c, b := o.post("/api/sites/toko/restore", `{"id":`+itoa(origID)+`}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if o.siteConf("toko") != orig {
		t.Fatal("versi tidak dipulihkan")
	}
	if c, _ := o.post("/api/sites/toko/restore", `{"id":99999}`); c != 404 {
		t.Fatalf("id tak ada: %d", c)
	}
	// versi yang kini melanggar kebijakan tidak boleh dipulihkan
	o.app.st.db.Exec("INSERT INTO conf_versions(site,at,usr,sha,note,content) VALUES('toko',1,'x','h','n',?)", "server { allow all; }")
	var badID int64
	o.app.st.db.QueryRow("SELECT MAX(id) FROM conf_versions").Scan(&badID)
	if c, _ := o.post("/api/sites/toko/restore", `{"id":`+itoa(badID)+`}`); c != 400 {
		t.Fatalf("versi berbahaya harus ditolak: %d", c)
	}
}

func TestVersionsPrunedTo20AndDeduped(t *testing.T) {
	o := newSiteEnv(t)
	for i := 0; i < 30; i++ {
		o.app.st.AddVersion("toko", "u", "isi-"+itoa(int64(i)), "n", o.now)
	}
	o.app.st.AddVersion("toko", "u", "isi-29", "n", o.now) // duplikat berturut-turut
	if n := len(o.app.st.Versions("toko")); n != 20 {
		t.Fatalf("harus 20 versi: %d", n)
	}
}

func TestCustomConfigNeedsReplaceFlag(t *testing.T) {
	o := newSiteEnv(t)
	o.post("/api/sites/demo/settings", o.goodModel("php"))
	o.stepup()
	cur := o.siteConf("demo")
	if c, b := o.rawEdit("demo", cur+"# manual\n"); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if k := o.do("GET", "/api/sites/demo/settings", "", o.cookie, "", nil).Body.String(); !strings.Contains(k, `"kind":"custom"`) {
		t.Fatalf("harus custom: %s", k)
	}
	if c, _ := o.post("/api/sites/demo/settings", o.goodModel("php")); c != 409 {
		t.Fatalf("menimpa config manual tanpa replace harus 409: %d", c)
	}
	if c, _ := o.post("/api/sites/demo/settings", strings.Replace(o.goodModel("php"), "{", `{"replace":true,`, 1)); c != 200 {
		t.Fatalf("replace=true harus berhasil: %d", c)
	}
}

func TestKustomSiteRefusesSettings(t *testing.T) {
	o := newSiteEnv(t)
	os.WriteFile(filepath.Join(o.home, "sites", "aneh.conf"), []byte("server {\n    listen 80;\n}\n"), 0o644)
	if c, _ := o.post("/api/sites/aneh/settings", o.goodModel("proxy")); c != 400 {
		t.Fatalf("%d", c)
	}
}

// ---------- SSL & log ----------

func writeCert(t *testing.T, dir, host string, notAfter time.Time) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: notAfter.Add(-24 * time.Hour), NotAfter: notAfter, Issuer: pkix.Name{CommonName: "mkcert uji"}}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	os.WriteFile(filepath.Join(dir, host+".pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func TestSSLInfoAndRenew(t *testing.T) {
	o := newSiteEnv(t)
	if b := o.do("GET", "/api/sites/demo/ssl", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"exists":false`) {
		t.Fatalf("tanpa sertifikat: %s", b)
	}
	writeCert(t, o.app.certDir, "demo.lab", o.now.Add(10*24*time.Hour))
	b := o.do("GET", "/api/sites/demo/ssl", "", o.cookie, "", nil).Body.String()
	for _, s := range []string{`"exists":true`, `"days_left":10`, `"demo.lab"`, `"expired":false`, "Let's Encrypt tidak berlaku"} {
		if !strings.Contains(b, s) {
			t.Errorf("SSL info tanpa %s: %s", s, b)
		}
	}
	writeCert(t, o.app.certDir, "demo.lab", o.now.Add(-48*time.Hour))
	if b := o.do("GET", "/api/sites/demo/ssl", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"expired":true`) {
		t.Fatalf("kedaluwarsa tidak terdeteksi: %s", b)
	}
	if c, _ := o.post("/api/sites/demo/ssl/renew", `{}`); c != 403 {
		t.Fatalf("renew tanpa step-up: %d", c)
	}
	o.stepup()
	if c, _ := o.post("/api/sites/demo/ssl/renew", `{}`); c != 200 || !strings.Contains(o.calls(), "CALL:[site][cert][demo]") {
		t.Fatalf("renew: %d %s", c, o.calls())
	}
}

func TestLogsTailScrubAndLimits(t *testing.T) {
	o := newSiteEnv(t)
	os.MkdirAll(o.app.logDir, 0o755)
	var lines []string
	for i := 0; i < 300; i++ {
		lines = append(lines, "GET /x?token=RAHASIA"+itoa(int64(i))+" 200")
	}
	os.WriteFile(filepath.Join(o.app.logDir, "demo.log"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	os.WriteFile(o.app.errLog, []byte("2026 [error] server: demo.lab, request: x\n2026 [error] server: lain.lab, request: y\n"), 0o644)
	b := o.do("GET", "/api/sites/demo/logs?kind=access&n=5", "", o.cookie, "", nil).Body.String()
	if strings.Contains(b, "RAHASIA") || strings.Count(b, "200") != 5 {
		t.Fatalf("log akses: %s", b)
	}
	e := o.do("GET", "/api/sites/demo/logs?kind=error", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(e, "demo.lab") || strings.Contains(e, "lain.lab") {
		t.Fatalf("log error harus tersaring per situs: %s", e)
	}
	for _, q := range []string{"kind=lain", "kind=access&n=0", "kind=access&n=501", "kind=access&n=abc", "kind=access&n=-1", "kind=../../etc/passwd"} {
		if w := o.do("GET", "/api/sites/demo/logs?"+q, "", o.cookie, "", nil); w.Code != 400 {
			t.Errorf("%s -> %d", q, w.Code)
		}
	}
	if w := o.do("GET", "/api/sites/home/logs?kind=access", "", o.cookie, "", nil); w.Code != 200 {
		t.Errorf("situs sistem log boleh dibaca: %d", w.Code)
	}
}
