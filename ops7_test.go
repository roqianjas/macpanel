package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestApr1MatchesOpenSSL(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl tidak ada")
	}
	for _, c := range [][2]string{{"rahasia123", "abcd1234"}, {"a", "zzzzzzzz"}, {"kata sandi panjang dengan spasi 😀", "./09AZaz"}, {strings.Repeat("x", 40), "Qw3rTy12"}} {
		out, err := exec.Command("openssl", "passwd", "-apr1", "-salt", c[1], c[0]).Output()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := apr1(c[0], c[1]), strings.TrimSpace(string(out)); got != want {
			t.Errorf("apr1(%q,%q) = %s, openssl = %s", c[0], c[1], got, want)
		}
	}
}

func TestInTailnet(t *testing.T) {
	ok := []string{"100.64.0.1", "100.110.21.93", "100.64.0.0/10", "100.100.0.0/16", "127.0.0.1", "127.0.0.0/8", "::1", "fd7a:115c:a1e0::1", "fd7a:115c:a1e0::/48", "fd7a:115c:a1e0:1::/64"}
	bad := []string{"", "all", "0.0.0.0/0", "0.0.0.0", "8.8.8.8", "192.168.0.1", "100.63.255.255", "100.128.0.0", "100.64.0.0/9", "100.0.0.0/8", "127.0.0.0/7", "::/0", "::", "fd7a::/16", "fd7a:115c:a1e0::/47", "100.64.0.1/10", "unix:/tmp/x", "100.64.0.1;", "100.64.0.1 8.8.8.8", "2001:db8::1", "1.2.3.4/33", "::ffff:8.8.8.8"}
	for _, s := range ok {
		if !inTailnet(s) {
			t.Errorf("%q harus diterima", s)
		}
	}
	for _, s := range bad {
		if inTailnet(s) {
			t.Errorf("%q harus ditolak", s)
		}
	}
}

func extrasEnv(t *testing.T) *opsEnv {
	o := newSiteEnv(t)
	o.app.tld = "lab"
	return o
}

func (o *opsEnv) settings(name, model string) (int, string) {
	return o.post("/api/sites/"+name+"/settings", model)
}

func withExtra(m, extra string) string { return strings.TrimSuffix(m, "}") + "," + extra + "}" }

func TestSiteExtrasGenerateAndRoundTrip(t *testing.T) {
	o := extrasEnv(t)
	o.stepup()
	if c, b := o.post("/api/sites/demo/auth/users", `{"user":"tim","password":"kata-sandi-1"}`); c != 200 {
		t.Fatalf("tambah user: %d %s", c, b)
	}
	m := withExtra(o.goodModel("php"), `"aliases":["www.lab","api2.lab"],"auth":true,"allow_ips":["100.110.21.93","100.64.0.0/16"]`)
	if c, b := o.settings("demo", m); c != 200 {
		t.Fatalf("simpan: %d %s", c, b)
	}
	conf := o.siteConf("demo")
	for _, want := range []string{"server_name demo.lab www.lab api2.lab;", "allow 100.110.21.93;", "allow 100.64.0.0/16;", "deny all;", "auth_basic \"Restricted\";", "auth_basic_user_file " + o.app.authFile("demo") + ";"} {
		if !strings.Contains(conf, want) {
			t.Errorf("config tak memuat %q", want)
		}
	}
	if strings.Count(conf, "deny all;") != 2 {
		t.Errorf("acl harus ada di kedua blok server: %d", strings.Count(conf, "deny all;"))
	}
	for _, blk := range strings.Split(conf, "\nserver {")[1:] {
		if strings.Index(blk, "allow 100.110.21.93;") > strings.Index(blk, "tailnet_only.conf") {
			t.Error("acl harus sebelum include tailnet_only")
		}
	}
	if !strings.Contains(o.calls(), "CALL:[site][cert][demo][www.lab][api2.lab]") {
		t.Errorf("sertifikat harus memuat alias: %s", o.calls())
	}
	if b := o.do("GET", "/api/sites/demo/settings", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"kind":"template"`) || !strings.Contains(b, `"aliases":["www.lab","api2.lab"]`) {
		t.Errorf("harus kembali sebagai template dengan alias: %s", b)
	}
	// model lama tanpa field baru tetap menghasilkan config yang sama dengan sebelumnya (tanpa direktif baru)
	if c, b := o.settings("demo", o.goodModel("php")); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	if conf = o.siteConf("demo"); strings.Contains(conf, "allow ") || strings.Contains(conf, "auth_basic") || strings.Contains(conf, "www.lab") {
		t.Error("model tanpa ekstra tidak boleh menghasilkan direktif ekstra")
	}
}

func TestSiteExtrasRejections(t *testing.T) {
	o := extrasEnv(t)
	o.stepup()
	os.WriteFile(filepath.Join(o.home, "sites", "lain.conf"), []byte("server {\n    server_name lain.lab dipakai.lab;\n}"), 0o644)
	before := o.siteConf("demo")
	calls := o.calls()
	bad := map[string]string{
		"alias luar tld":    `"aliases":["evil.com"]`,
		"alias bertingkat":  `"aliases":["a.b.lab"]`,
		"alias traversal":   `"aliases":["../x.lab"]`,
		"alias huruf besar": `"aliases":["Www.lab"]`,
		"alias spasi":       `"aliases":["a b.lab"]`,
		"alias titik koma":  `"aliases":["a.lab;include /etc/passwd"]`,
		"alias ganda":       `"aliases":["a.lab","a.lab"]`,
		"alias 11":          `"aliases":["a1.lab","a2.lab","a3.lab","a4.lab","a5.lab","a6.lab","a7.lab","a8.lab","a9.lab","a10.lab","a11.lab"]`,
		"alias situs inti":  `"aliases":["home.lab"]`,
		"alias panel":       `"aliases":["panel.lab"]`,
		"ip semua":          `"allow_ips":["all"]`,
		"ip 0/0":            `"allow_ips":["0.0.0.0/0"]`,
		"ip publik":         `"allow_ips":["8.8.8.8"]`,
		"ip lan":            `"allow_ips":["192.168.0.1"]`,
		"ip lebih luas":     `"allow_ips":["100.64.0.0/9"]`,
		"ip koma":           `"allow_ips":["100.64.0.1;deny all"]`,
		"auth tanpa user":   `"auth":true`,
	}
	for name, extra := range bad {
		if c, b := o.settings("demo", withExtra(o.goodModel("php"), extra)); c != 400 {
			t.Errorf("%s: %d %s", name, c, b)
		}
	}
	if c, b := o.settings("demo", withExtra(o.goodModel("php"), `"aliases":["dipakai.lab"]`)); c != 409 {
		t.Errorf("alias milik situs lain harus 409: %d %s", c, b)
	}
	if o.siteConf("demo") != before || o.calls() != calls {
		t.Fatalf("config/lab berubah karena input ditolak: %s", o.calls())
	}
}

func TestPolicyAllowDenyAuth(t *testing.T) {
	o := extrasEnv(t)
	o.stepup()
	if c, b := o.settings("demo", o.goodModel("php")); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	orig := o.siteConf("demo")
	loc := "location ~ /\\.(?!well-known) { return 404; }"
	if !strings.Contains(orig, loc) {
		t.Fatal("pola dasar tidak ditemukan")
	}
	try := func(repl string) int {
		c, _ := o.post("/api/sites/demo/config", `{"sha":"`+sha256hex(o.siteConf("demo"))+`","content":`+jsonQuote(strings.Replace(orig, loc, repl, 1))+`}`)
		return c
	}
	for _, evil := range []string{
		"location /x { allow 100.64.0.1; return 404; }",            // tanpa deny all: membuka akses ke luar tailnet
		"location /x { allow 100.64.0.1; deny 1.2.3.4; }",          // diakhiri bukan 'deny all'
		"location /x { allow all; deny all; }",                     // allow all
		"location /x { allow 0.0.0.0/0; deny all; }",               // rentang publik
		"location /x { deny all; allow 100.64.0.1; }",              // urutan: allow terakhir
		"location /x { auth_basic_user_file /etc/passwd; }",        // berkas lain
		"location /x { auth_basic \"$x\"; }",                       // variabel
		"location /x { satisfy any; allow 100.64.0.1; deny all; }", // satisfy tidak diizinkan
		"allow 100.64.0.1;",                                        // server-level tanpa deny all
	} {
		if c := try(evil); c != 400 {
			t.Errorf("harus ditolak (%d): %s", c, evil)
		}
	}
	if c := try("location /x { allow 100.64.0.1; deny all; return 404; }"); c != 200 {
		t.Errorf("allow tailnet + deny all harus diterima: %d", c)
	}
}

func TestAuthUsersEndpoints(t *testing.T) {
	o := extrasEnv(t)
	if c, _ := o.post("/api/sites/demo/auth/users", `{"user":"tim","password":"kata-sandi-1"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	for _, b := range []string{`{"user":"a:b","password":"kata-sandi-1"}`, `{"user":"","password":"kata-sandi-1"}`, `{"user":"tim","password":"pendek"}`, `{"user":"tim","password":"kata\nsandi-panjang"}`, `{"user":"../x","password":"kata-sandi-1"}`, `{"user":"` + strings.Repeat("a", 33) + `","password":"kata-sandi-1"}`} {
		if c, r := o.post("/api/sites/demo/auth/users", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if c, _ := o.post("/api/sites/panel/auth/users", `{"user":"tim","password":"kata-sandi-1"}`); c != 400 && c != 404 {
		t.Errorf("situs inti harus ditolak: %d", c)
	}
	o.post("/api/sites/demo/auth/users", `{"user":"tim","password":"kata-sandi-1"}`)
	o.post("/api/sites/demo/auth/users", `{"user":"ani","password":"kata-sandi-2"}`)
	b, _ := os.ReadFile(o.app.authFile("demo"))
	if !strings.HasPrefix(string(b), "ani:$apr1$") || strings.Contains(string(b), "kata-sandi") {
		t.Fatalf("berkas htpasswd: %q", b)
	}
	if fi, _ := os.Stat(o.app.authFile("demo")); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode htpasswd harus 0600: %v", fi.Mode())
	}
	if g := o.do("GET", "/api/sites/demo/auth", "", o.cookie, "", nil).Body.String(); !strings.Contains(g, `["ani","tim"]`) || strings.Contains(g, "apr1") {
		t.Errorf("daftar tidak boleh memuat hash: %s", g)
	}
	o.settings("demo", withExtra(o.goodModel("php"), `"auth":true`))
	if c, _ := o.post("/api/sites/demo/auth/users/ani/remove", `{}`); c != 200 {
		t.Errorf("hapus satu: %d", c)
	}
	if c, r := o.post("/api/sites/demo/auth/users/tim/remove", `{}`); c != 400 || !strings.Contains(r, "terakhir") {
		t.Errorf("pengguna terakhir saat auth aktif harus ditolak: %d %s", c, r)
	}
	if c, _ := o.post("/api/sites/demo/auth/users/nope/remove", `{}`); c != 404 {
		t.Errorf("tak ada: %d", c)
	}
}

func TestEnvEditor(t *testing.T) {
	o := extrasEnv(t)
	o.settings("demo", o.goodModel("php"))
	env := filepath.Join(o.home, "Developer/proj/.env")
	os.WriteFile(env, []byte("APP_KEY=base64:rahasia\n"), 0o600)
	if w := o.do("GET", "/api/sites/demo/env", "", o.cookie, "", nil); w.Code != 403 {
		t.Fatalf("baca tanpa step-up harus 403: %d", w.Code)
	}
	o.stepup()
	if b := o.do("GET", "/api/sites/demo/env", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, "APP_KEY=base64:rahasia") {
		t.Fatalf("%s", b)
	}
	if c, _ := o.post("/api/sites/demo/env", `{"content":"A=1\nB=2\n","x":1}`); c != 400 {
		t.Errorf("field asing: %d", c)
	}
	if c, _ := o.post("/api/sites/demo/env", `{"content":"A\u0000B"}`); c != 400 {
		t.Errorf("NUL harus ditolak: %d", c)
	}
	if c, _ := o.post("/api/sites/demo/env", `{"content":"`+strings.Repeat("a", maxEnv+1)+`"}`); c != 400 {
		t.Errorf("terlalu besar harus 400: %d", c)
	}
	if c, b := o.post("/api/sites/demo/env", `{"content":"A=1\nB=2\n"}`); c != 200 {
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
	if c, _ := o.post("/api/sites/demo/env", `{"content":"A=1"}`); c != 400 {
		t.Errorf("symlink harus ditolak: %d", c)
	}
	if w := o.do("GET", "/api/sites/demo/env", "", o.cookie, "", nil); w.Code != 400 {
		t.Errorf("baca symlink harus 400: %d", w.Code)
	}
	if c, _ := o.post("/api/sites/panel/env", `{"content":"A=1"}`); c != 400 && c != 404 {
		t.Errorf("situs inti: %d", c)
	}
	if c, _ := o.post("/api/sites/api/env", `{"content":"A=1"}`); c != 400 {
		t.Errorf("situs proxy tidak punya folder: %d", c)
	}
	// viewer tidak boleh membaca rahasia
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	os.Remove(env)
	os.WriteFile(env, []byte("S=1"), 0o600)
	if w := o.do("GET", "/api/sites/demo/env", "", o.cookie, "", nil); w.Code != 403 || strings.Contains(w.Body.String(), "S=1") {
		t.Errorf("viewer: %d %s", w.Code, w.Body.String())
	}
}

func TestRawEditCannotHijackDomains(t *testing.T) {
	o := extrasEnv(t)
	o.stepup()
	os.WriteFile(filepath.Join(o.home, "sites", "lain.conf"), []byte("server {\n    server_name lain.lab;\n}"), 0o644)
	if c, b := o.settings("demo", o.goodModel("php")); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	orig := o.siteConf("demo")
	for _, host := range []string{"home.lab", "panel.lab", "kuma.lab", "lain.lab", "demo.lab"} {
		edit := strings.Replace(orig, "server_name demo.lab;", "server_name demo.lab "+host+";", 2)
		if c, _ := o.post("/api/sites/demo/config", `{"sha":"`+sha256hex(o.siteConf("demo"))+`","content":`+jsonQuote(edit)+`}`); c != 400 {
			t.Errorf("alias %s lewat editor mentah harus ditolak: %d", host, c)
		}
	}
	edit := strings.Replace(orig, "server_name demo.lab;", "server_name demo.lab bebas.lab;", 2)
	if c, b := o.post("/api/sites/demo/config", `{"sha":"`+sha256hex(o.siteConf("demo"))+`","content":`+jsonQuote(edit)+`}`); c != 200 {
		t.Errorf("alias bebas harus diterima: %d %s", c, b)
	}
}
