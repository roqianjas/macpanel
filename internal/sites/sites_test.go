package sites

import (
	"os/exec"
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
	ok := []string{"100.64.0.1", "100.64.0.10", "100.64.0.0/10", "100.100.0.0/16", "127.0.0.1", "127.0.0.0/8", "::1", "fd7a:115c:a1e0::1", "fd7a:115c:a1e0::/48", "fd7a:115c:a1e0:1::/64"}
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
