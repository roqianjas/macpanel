package server_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestPolicyAllowDenyAuth(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	o.Stepup()
	if c, b := o.Settings("demo", o.GoodModel("php")); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	orig := o.SiteConf("demo")
	loc := "location ~ /\\.(?!well-known) { return 404; }"
	if !strings.Contains(orig, loc) {
		t.Fatal("pola dasar tidak ditemukan")
	}
	try := func(repl string) int {
		c, _ := o.Post("/api/sites/demo/config", `{"sha":"`+crypto.Sha256hex(o.SiteConf("demo"))+`","content":`+testenv.JsonQuote(strings.Replace(orig, loc, repl, 1))+`}`)
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

func TestRawEditCannotHijackDomains(t *testing.T) {
	o := testenv.ExtrasEnv(t)
	o.Stepup()
	os.WriteFile(filepath.Join(o.Home, "sites", "lain.conf"), []byte("server {\n    server_name lain.lab;\n}"), 0o644)
	if c, b := o.Settings("demo", o.GoodModel("php")); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	orig := o.SiteConf("demo")
	for _, host := range []string{"home.lab", "panel.lab", "kuma.lab", "lain.lab", "demo.lab"} {
		edit := strings.Replace(orig, "server_name demo.lab;", "server_name demo.lab "+host+";", 2)
		if c, _ := o.Post("/api/sites/demo/config", `{"sha":"`+crypto.Sha256hex(o.SiteConf("demo"))+`","content":`+testenv.JsonQuote(edit)+`}`); c != 400 {
			t.Errorf("alias %s lewat editor mentah harus ditolak: %d", host, c)
		}
	}
	edit := strings.Replace(orig, "server_name demo.lab;", "server_name demo.lab bebas.lab;", 2)
	if c, b := o.Post("/api/sites/demo/config", `{"sha":"`+crypto.Sha256hex(o.SiteConf("demo"))+`","content":`+testenv.JsonQuote(edit)+`}`); c != 200 {
		t.Errorf("alias bebas harus diterima: %d %s", c, b)
	}
}

func TestSecretEncryptedAtRest(t *testing.T) {
	e := testenv.NewEnv(t)
	raw, _ := os.ReadFile(e.Dbp)
	if bytes.Contains(raw, []byte(crypto.B32.EncodeToString(e.Secret))) || bytes.Contains(raw, e.Secret) {
		t.Fatal("secret TOTP tersimpan polos")
	}
	if _, err := crypto.Open(bytes.Repeat([]byte{9}, 32), testenv.MustUserEnc(e)); err == nil {
		t.Fatal("kunci salah bisa membuka secret")
	}
}
