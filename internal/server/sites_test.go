package server_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqianjas/macpanel/internal/sites"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestSiteExtrasGenerateAndRoundTrip(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	o.Stepup()
	if c, b := o.Post("/api/sites/demo/auth/users", `{"user":"tim","password":"kata-sandi-1"}`); c != 200 {
		t.Fatalf("tambah user: %d %s", c, b)
	}
	m := testenv.WithExtra(o.GoodModel("php"), `"aliases":["www.lab","api2.lab"],"auth":true,"allow_ips":["100.64.0.10","100.64.0.0/16"]`)
	if c, b := o.Settings("demo", m); c != 200 {
		t.Fatalf("simpan: %d %s", c, b)
	}
	conf := o.SiteConf("demo")
	for _, want := range []string{"server_name demo.lab www.lab api2.lab;", "allow 100.64.0.10;", "allow 100.64.0.0/16;", "deny all;", "auth_basic \"Restricted\";", "auth_basic_user_file " + sites.AuthFile(o.App, "demo") + ";"} {
		if !strings.Contains(conf, want) {
			t.Errorf("config tak memuat %q", want)
		}
	}
	if strings.Count(conf, "deny all;") != 2 {
		t.Errorf("acl harus ada di kedua blok server: %d", strings.Count(conf, "deny all;"))
	}
	for _, blk := range strings.Split(conf, "\nserver {")[1:] {
		if strings.Index(blk, "allow 100.64.0.10;") > strings.Index(blk, "tailnet_only.conf") {
			t.Error("acl harus sebelum include tailnet_only")
		}
	}
	if !strings.Contains(o.Calls(), "CALL:[site][cert][demo][www.lab][api2.lab]") {
		t.Errorf("sertifikat harus memuat alias: %s", o.Calls())
	}
	if b := o.Do("GET", "/api/sites/demo/settings", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, `"kind":"template"`) || !strings.Contains(b, `"aliases":["www.lab","api2.lab"]`) {
		t.Errorf("harus kembali sebagai template dengan alias: %s", b)
	}
	// model lama tanpa field baru tetap menghasilkan config yang sama dengan sebelumnya (tanpa direktif baru)
	if c, b := o.Settings("demo", o.GoodModel("php")); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if conf = o.SiteConf("demo"); strings.Contains(conf, "allow ") || strings.Contains(conf, "auth_basic") || strings.Contains(conf, "www.lab") {
		t.Error("model tanpa ekstra tidak boleh menghasilkan direktif ekstra")
	}
}

func TestAuthUsersEndpoints(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	if c, _ := o.Post("/api/sites/demo/auth/users", `{"user":"tim","password":"kata-sandi-1"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.Stepup()
	for _, b := range []string{`{"user":"a:b","password":"kata-sandi-1"}`, `{"user":"","password":"kata-sandi-1"}`, `{"user":"tim","password":"pendek"}`, `{"user":"tim","password":"kata\nsandi-panjang"}`, `{"user":"../x","password":"kata-sandi-1"}`, `{"user":"` + strings.Repeat("a", 33) + `","password":"kata-sandi-1"}`} {
		if c, r := o.Post("/api/sites/demo/auth/users", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if c, _ := o.Post("/api/sites/panel/auth/users", `{"user":"tim","password":"kata-sandi-1"}`); c != 400 && c != 404 {
		t.Errorf("situs inti harus ditolak: %d", c)
	}
	o.Post("/api/sites/demo/auth/users", `{"user":"tim","password":"kata-sandi-1"}`)
	o.Post("/api/sites/demo/auth/users", `{"user":"ani","password":"kata-sandi-2"}`)
	b, _ := os.ReadFile(sites.AuthFile(o.App, "demo"))
	if !strings.HasPrefix(string(b), "ani:$apr1$") || strings.Contains(string(b), "kata-sandi") {
		t.Fatalf("berkas htpasswd: %q", b)
	}
	if fi, _ := os.Stat(sites.AuthFile(o.App, "demo")); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode htpasswd harus 0600: %v", fi.Mode())
	}
	if g := o.Do("GET", "/api/sites/demo/auth", "", o.Cookie, "", nil).Body.String(); !strings.Contains(g, `["ani","tim"]`) || strings.Contains(g, "apr1") {
		t.Errorf("daftar tidak boleh memuat hash: %s", g)
	}
	o.Settings("demo", testenv.WithExtra(o.GoodModel("php"), `"auth":true`))
	if c, _ := o.Post("/api/sites/demo/auth/users/ani/remove", `{}`); c != 200 {
		t.Errorf("hapus satu: %d", c)
	}
	if c, r := o.Post("/api/sites/demo/auth/users/tim/remove", `{}`); c != 400 || !strings.Contains(r, "terakhir") {
		t.Errorf("pengguna terakhir saat auth aktif harus ditolak: %d %s", c, r)
	}
	if c, _ := o.Post("/api/sites/demo/auth/users/nope/remove", `{}`); c != 404 {
		t.Errorf("tak ada: %d", c)
	}
}

func TestEnvEditor(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	o.Settings("demo", o.GoodModel("php"))
	env := filepath.Join(o.Home, "Developer/proj/.env")
	os.WriteFile(env, []byte("APP_KEY=base64:rahasia\n"), 0o600)
	if w := o.Do("GET", "/api/sites/demo/env", "", o.Cookie, "", nil); w.Code != 403 {
		t.Fatalf("baca tanpa step-up harus 403: %d", w.Code)
	}
	o.Stepup()
	if b := o.Do("GET", "/api/sites/demo/env", "", o.Cookie, "", nil).Body.String(); !strings.Contains(b, "APP_KEY=base64:rahasia") {
		t.Fatalf("%s", b)
	}
	if c, _ := o.Post("/api/sites/demo/env", `{"content":"A=1\nB=2\n","x":1}`); c != 400 {
		t.Errorf("field asing: %d", c)
	}
	if c, _ := o.Post("/api/sites/demo/env", `{"content":"A\u0000B"}`); c != 400 {
		t.Errorf("NUL harus ditolak: %d", c)
	}
	if c, _ := o.Post("/api/sites/demo/env", `{"content":"`+strings.Repeat("a", sites.MaxEnv+1)+`"}`); c != 400 {
		t.Errorf("terlalu besar harus 400: %d", c)
	}
	if c, b := o.Post("/api/sites/demo/env", `{"content":"A=1\nB=2\n"}`); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if b, _ := os.ReadFile(env); string(b) != "A=1\nB=2\n" {
		t.Errorf("isi: %q", b)
	}
	if fi, _ := os.Stat(env); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode .env 0600: %v", fi.Mode())
	}
	baks, _ := filepath.Glob(env + ".bak.*")
	if len(baks) != 1 {
		t.Fatalf("cadangan: %v", baks)
	}
	if b, _ := os.ReadFile(baks[0]); string(b) != "APP_KEY=base64:rahasia\n" {
		t.Errorf("cadangan harus isi lama: %q", b)
	}
	// symlink dan situs proxy/inti ditolak
	os.Remove(env)
	os.Symlink("/etc/hosts", env)
	if c, _ := o.Post("/api/sites/demo/env", `{"content":"A=1"}`); c != 400 {
		t.Errorf("symlink harus ditolak: %d", c)
	}
	if w := o.Do("GET", "/api/sites/demo/env", "", o.Cookie, "", nil); w.Code != 400 {
		t.Errorf("baca symlink harus 400: %d", w.Code)
	}
	if c, _ := o.Post("/api/sites/panel/env", `{"content":"A=1"}`); c != 400 && c != 404 {
		t.Errorf("situs inti: %d", c)
	}
	if c, _ := o.Post("/api/sites/api/env", `{"content":"A=1"}`); c != 400 {
		t.Errorf("situs proxy tidak punya folder: %d", c)
	}
	// viewer tidak boleh membaca rahasia
	o.App.Store.DB.Exec("UPDATE users SET role='viewer'")
	os.Remove(env)
	os.WriteFile(env, []byte("S=1"), 0o600)
	if w := o.Do("GET", "/api/sites/demo/env", "", o.Cookie, "", nil); w.Code != 403 || strings.Contains(w.Body.String(), "S=1") {
		t.Errorf("viewer: %d %s", w.Code, w.Body.String())
	}
}

func TestPolicyAcceptsAllTemplatesAndLegacy(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	for _, m := range []string{
		`{"mode":"php","target":"X","php":"8.2","preset":"laravel","max_body_mb":64,"read_timeout_s":120,"force_https":true,"gzip":true}`,
		`{"mode":"php","target":"X","php":"8.5","preset":"wordpress","max_body_mb":64,"read_timeout_s":120,"force_https":false,"gzip":false,"rewrites":[{"from":"^/old/(.*)$","to":"/new/$1","flag":"permanent"}]}`,
		`{"mode":"statis","target":"X","preset":"static","max_body_mb":1,"read_timeout_s":10,"force_https":true,"gzip":true}`,
		`{"mode":"statis","target":"X","preset":"spa","max_body_mb":1,"read_timeout_s":10,"force_https":false,"gzip":false}`,
		`{"mode":"proxy","target":"3001","max_body_mb":10,"read_timeout_s":60,"force_https":true,"gzip":false}`,
	} {
		m = strings.ReplaceAll(m, `"X"`, `"`+o.Public()+`"`)
		var md sites.Model
		if err := testenv.JsonStrict(m, &md); err != nil {
			t.Fatal(err)
		}
		g, err := sites.Generate(o.App, "toko", md)
		if err != nil {
			t.Fatalf("generate: %v (%s)", err, m)
		}
		if err := sites.CheckConf(o.App, "toko", g); err != nil {
			t.Errorf("template ditolak kebijakan sendiri: %v\n%s", err, g)
		}
	}
	// format yang dibuat addsite.sh (kuma.lab)
	legacy := "server {\n    listen 80; listen [::]:80;\n    server_name toko.lab;\n    include " + o.App.NginxDir + "/snippets/tailnet_only.conf;\n    return 301 https://$host$request_uri;\n}\nserver {\n    listen 443 ssl; listen [::]:443 ssl;\n    http2 on;\n    server_name toko.lab;\n    include " + o.App.NginxDir + "/snippets/tailnet_only.conf;\n    ssl_certificate     " + o.App.CertDir + "/toko.lab.pem;\n    ssl_certificate_key " + o.App.CertDir + "/toko.lab-key.pem;\n    location / {\n        include " + o.App.NginxDir + "/snippets/proxy.conf;\n        proxy_pass http://127.0.0.1:3001;\n    }\n}\n"
	if err := sites.CheckConf(o.App, "toko", legacy); err != nil {
		t.Errorf("format addsite.sh ditolak: %v", err)
	}
}

func TestPolicyRejects(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	if err := sites.CheckConf(o.App, "toko", o.Base()); err != nil {
		t.Fatalf("basis harus sah: %v", err)
	}
	snip := o.App.NginxDir + "/snippets/tailnet_only.conf"
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
			return strings.Replace(s, "root ", "include "+o.App.NginxDir+"/snippets/lain.conf; root ", 1)
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
		"alias":           func(s string) string { return strings.Replace(s, "try_files $uri =404;", "alias "+o.Public()+"/;", 1) },
		"root /etc":       func(s string) string { return strings.Replace(s, "root "+o.Public(), "root /etc", 1) },
		"root variabel":   func(s string) string { return strings.Replace(s, "root "+o.Public(), "root $document_root", 1) },
		"root traversal": func(s string) string {
			return strings.Replace(s, "root "+o.Public(), "root "+o.Public()+"/../../../outside", 1)
		},
		"root symlink keluar": func(s string) string {
			return strings.Replace(s, "root "+o.Public(), "root "+o.Home+"/Developer/escape", 1)
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
			return strings.ReplaceAll(s, o.App.CertDir+"/toko.lab-key.pem", "/etc/ssl/private/k.pem")
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
		if err := sites.CheckConf(o.App, "toko", f(o.Base())); err == nil {
			t.Errorf("harus ditolak: %s", name)
		}
	}
}

func TestPolicyIgnoresComments(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	if err := sites.CheckConf(o.App, "toko", "# allow all; include /etc/passwd; load_module x;\n"+o.Base()); err != nil {
		t.Fatalf("komentar tidak boleh memengaruhi: %v", err)
	}
}

func TestModelValidation(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	pub := o.Public()
	bad := []sites.Model{
		{Mode: "php", Target: pub, PHP: "8.1", MaxBody: 1, ReadTimeout: 60}, {Mode: "php", Target: "3000", PHP: "8.4", MaxBody: 1, ReadTimeout: 60},
		{Mode: "proxy", Target: pub, MaxBody: 1, ReadTimeout: 60}, {Mode: "proxy", Target: "22", MaxBody: 1, ReadTimeout: 60}, {Mode: "statis", Target: "/etc", MaxBody: 1, ReadTimeout: 60},
		{Mode: "xx", Target: pub, MaxBody: 1, ReadTimeout: 60}, {Mode: "statis", Target: pub, MaxBody: 0, ReadTimeout: 60}, {Mode: "statis", Target: pub, MaxBody: 5000, ReadTimeout: 60},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 5}, {Mode: "statis", Target: pub, Preset: "aneh", MaxBody: 1, ReadTimeout: 60},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []sites.Rewrite{{From: "a b", To: "/x"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []sites.Rewrite{{From: "^/a$", To: "/x; allow all"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []sites.Rewrite{{From: "^/a$", To: "evil.com"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []sites.Rewrite{{From: "^/a$", To: "/x", Flag: "x"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []sites.Rewrite{{From: "^/a$\n", To: "/x"}}},
		{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60, Rewrites: []sites.Rewrite{{From: "a{2}", To: "/x"}}},
	}
	for i, m := range bad {
		if _, err := sites.Generate(o.App, "toko", m); err == nil {
			t.Errorf("model buruk #%d lolos: %+v", i, m)
		}
	}
	many := sites.Model{Mode: "statis", Target: pub, MaxBody: 1, ReadTimeout: 60}
	for i := 0; i < 51; i++ {
		many.Rewrites = append(many.Rewrites, sites.Rewrite{From: "^/a$", To: "/b"})
	}
	if _, err := sites.Generate(o.App, "toko", many); err == nil {
		t.Error("51 rewrite harus ditolak")
	}
}

func TestSettingsWriteApplyAndRoundTrip(t *testing.T) {
	o := testenv.NewSiteEnv(t)
	// situs "demo" sudah ada (legacy); simpan pengaturan pertama -> versi lama tersimpan
	before := o.SiteConf("demo")
	c, b := o.Post("/api/sites/demo/settings", o.GoodModel("php"))
	if c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	now := o.SiteConf("demo")
	if !strings.HasPrefix(now, sites.ModelHeader) || !strings.Contains(now, "php-fpm-dev-8.4.sock") || !strings.Contains(now, "access_log "+o.App.LogDir+"/demo.log") {
		t.Fatalf("config tidak sesuai:\n%s", now)
	}
	if !strings.Contains(o.Calls(), "CALL:[nginx][test]") || !strings.Contains(o.Calls(), "CALL:[nginx][reload]") {
		t.Fatalf("nginx -t dan reload harus dipanggil: %s", o.Calls())
	}
	g := o.Do("GET", "/api/sites/demo/settings", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(g, `"kind":"template"`) || !strings.Contains(g, `"php":"8.4"`) {
		t.Fatalf("GET settings: %s", g)
	}
	cfg := o.Do("GET", "/api/sites/demo/config", "", o.Cookie, "", nil).Body.String()
	if !strings.Contains(cfg, "sebelum: pengaturan") || !strings.Contains(cfg, "pengaturan") {
		t.Fatalf("versi tidak tersimpan: %s", cfg)
	}
	if _, err := os.Stat(filepath.Join(o.Home, "sites", ".demo.conf.tmp")); err == nil {
		t.Fatal("file sementara tertinggal")
	}
	if before == now {
		t.Fatal("config tidak berubah")
	}
}
