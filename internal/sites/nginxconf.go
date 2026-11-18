package sites

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/roqianjas/macpanel/internal/core"
)

// Dir adalah satu direktif nginx (dengan blok bila Open).
type Dir struct {
	Name  string
	Args  []string
	Block []*Dir
	Open  bool
	Line  int
}

const (
	maxConfSize = 64 << 10
	maxDepth    = 4
)

func isDelim(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == ';' || r == '{' || r == '}'
}

// parseNginx adalah parser ketat yang meniru aturan token nginx (komentar hanya di awal token,
// backslash mengutip karakter berikutnya, kutip ' dan "). Apa pun yang tidak jelas ditolak (fail-closed).
func parseNginx(src string) ([]*Dir, error) {
	if len(src) > maxConfSize {
		return nil, errors.New("config terlalu besar (maks 64 KB)")
	}
	if !utf8.ValidString(src) || strings.ContainsRune(src, 0) {
		return nil, errors.New("karakter tidak valid")
	}
	if strings.Contains(src, "${") {
		return nil, errors.New("${...} tidak diizinkan")
	}
	rs := []rune(src)
	var top []*Dir
	var stack []*Dir
	var toks []string
	line, tokLine := 1, 1
	add := func(d *Dir) {
		if len(stack) == 0 {
			top = append(top, d)
		} else {
			p := stack[len(stack)-1]
			p.Block = append(p.Block, d)
		}
	}
	fail := func(m string) ([]*Dir, error) { return nil, fmt.Errorf("baris %d: %s", line, m) }
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case c == ';':
			if len(toks) == 0 {
				return fail("';' tak terduga")
			}
			add(&Dir{Name: toks[0], Args: toks[1:], Line: tokLine})
			toks = nil
			i++
		case c == '{':
			if len(toks) == 0 {
				return fail("'{' tak terduga")
			}
			d := &Dir{Name: toks[0], Args: toks[1:], Open: true, Line: tokLine}
			add(d)
			stack = append(stack, d)
			if len(stack) > maxDepth {
				return fail("blok terlalu dalam")
			}
			toks = nil
			i++
		case c == '}':
			if len(toks) != 0 || len(stack) == 0 {
				return fail("'}' tak terduga")
			}
			stack = stack[:len(stack)-1]
			i++
		default:
			if len(toks) == 0 {
				tokLine = line
			}
			var sb strings.Builder
			if c == '"' || c == '\'' {
				q := c
				i++
				for {
					if i >= len(rs) {
						return fail("tanda kutip tidak ditutup")
					}
					ch := rs[i]
					if ch == '\\' {
						if i+1 >= len(rs) {
							return fail("backslash di akhir")
						}
						n := rs[i+1]
						if n != '"' && n != '\'' && n != '\\' {
							sb.WriteRune('\\')
						}
						sb.WriteRune(n)
						i += 2
						continue
					}
					if ch == q {
						i++
						break
					}
					if ch == '\n' {
						line++
					}
					sb.WriteRune(ch)
					i++
				}
				if i < len(rs) && !isDelim(rs[i]) {
					return fail("karakter tepat setelah tanda kutip")
				}
			} else {
				for i < len(rs) {
					ch := rs[i]
					if ch == '\\' {
						if i+1 >= len(rs) {
							return fail("backslash di akhir")
						}
						sb.WriteRune(ch)
						sb.WriteRune(rs[i+1])
						i += 2
						continue
					}
					if isDelim(ch) {
						break
					}
					sb.WriteRune(ch)
					i++
				}
			}
			toks = append(toks, sb.String())
		}
	}
	if len(toks) != 0 || len(stack) != 0 {
		return fail("config tidak lengkap (kurang ';' atau '}')")
	}
	return top, nil
}

var phpSockOK = regexp.MustCompile(`^unix:/opt/homebrew/var/run/php-fpm-dev-8\.[2-5]\.sock$`)

var proxyPassOK = regexp.MustCompile(`^http://127\.0\.0\.1:([0-9]{1,5})(/[A-Za-z0-9._~/%-]*)?$`)

var rewriteOK = regexp.MustCompile(`^[A-Za-z0-9_\-./^$()*+?|\[\]:%=&~,@!]+$`)

type policy struct {
	a    *core.App
	name string
	host string
}

// direktif yang BOLEH. Selain ini ditolak (include, load_module, if, set, map, upstream, perl/lua/js, allow, deny, alias, error_log, dll).
var allowedDirs = map[string]bool{
	"server": true, "listen": true, "server_name": true, "http2": true, "include": true, "ssl_certificate": true, "ssl_certificate_key": true,
	"location": true, "root": true, "index": true, "return": true, "rewrite": true, "try_files": true, "client_max_body_size": true,
	"add_header": true, "access_log": true, "gzip": true, "gzip_types": true, "gzip_min_length": true, "gzip_vary": true, "charset": true,
	"error_page": true, "expires": true, "etag": true, "proxy_pass": true, "proxy_http_version": true, "proxy_set_header": true,
	"proxy_read_timeout": true, "proxy_send_timeout": true, "proxy_connect_timeout": true, "proxy_buffering": true,
	"fastcgi_pass": true, "fastcgi_param": true, "fastcgi_split_path_info": true, "fastcgi_read_timeout": true,
	"fastcgi_send_timeout": true, "fastcgi_buffers": true, "fastcgi_buffer_size": true, "fastcgi_busy_buffers_size": true,
	"fastcgi_intercept_errors": true, "sendfile": true, "keepalive_timeout": true,
	"allow": true, "deny": true, "auth_basic": true, "auth_basic_user_file": true,
}

var serverOnly = map[string]bool{"listen": true, "server_name": true, "http2": true, "ssl_certificate": true, "ssl_certificate_key": true}

func CheckConf(a *core.App, name, src string) error {
	top, err := parseNginx(src)
	if err != nil {
		return err
	}
	p := &policy{a: a, name: name, host: name + "." + a.TLD}
	if len(top) == 0 || len(top) > 3 {
		return errors.New("harus berisi 1-3 blok server")
	}
	for _, d := range top {
		if d.Name != "server" || !d.Open || len(d.Args) != 0 {
			return fmt.Errorf("baris %d: hanya blok 'server { }' yang boleh di tingkat atas", d.Line)
		}
		if err := p.server(d); err != nil {
			return err
		}
	}
	return nil
}

func (p *policy) server(s *Dir) error {
	hasGuard, hasName, hasListen := false, false, false
	for _, d := range s.Block {
		if d.Name == "include" && len(d.Args) == 1 && d.Args[0] == p.a.NginxDir+"/snippets/tailnet_only.conf" && !d.Open {
			hasGuard = true
		}
		if d.Name == "server_name" {
			hasName = true
		}
		if d.Name == "listen" {
			hasListen = true
		}
	}
	if !hasGuard {
		return fmt.Errorf("baris %d: setiap blok server wajib 'include %s/snippets/tailnet_only.conf;' (pembatas tailnet)", s.Line, p.a.NginxDir)
	}
	if !hasName || !hasListen {
		return fmt.Errorf("baris %d: server wajib punya listen dan server_name", s.Line)
	}
	return p.block(s.Block, true)
}

func (p *policy) block(ds []*Dir, inServer bool) error {
	// Blok yang punya allow/deny harus berakhir dengan 'deny all'; kalau tidak, blok location bisa membuka akses di luar tailnet.
	var lastACL *Dir
	for _, d := range ds {
		if d.Name == "allow" || d.Name == "deny" {
			lastACL = d
		}
	}
	if lastACL != nil && !(lastACL.Name == "deny" && len(lastACL.Args) == 1 && lastACL.Args[0] == "all") {
		return bad(lastACL, "daftar allow/deny harus diakhiri 'deny all'")
	}
	for _, d := range ds {
		if !allowedDirs[d.Name] {
			return fmt.Errorf("baris %d: direktif '%s' tidak diizinkan", d.Line, d.Name)
		}
		if serverOnly[d.Name] && !inServer {
			return fmt.Errorf("baris %d: '%s' hanya boleh di tingkat server", d.Line, d.Name)
		}
		if d.Open && d.Name != "location" {
			return fmt.Errorf("baris %d: '%s' tidak boleh punya blok", d.Line, d.Name)
		}
		if !d.Open && d.Name == "location" {
			return fmt.Errorf("baris %d: location butuh blok", d.Line)
		}
		for _, x := range d.Args {
			if len(x) > 500 {
				return fmt.Errorf("baris %d: argumen terlalu panjang", d.Line)
			}
		}
		if err := p.one(d); err != nil {
			return err
		}
		if d.Open {
			if err := p.block(d.Block, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func bad(d *Dir, m string) error { return fmt.Errorf("baris %d: %s: %s", d.Line, d.Name, m) }

func (p *policy) one(d *Dir) error {
	a := p.a
	switch d.Name {
	case "listen":
		j := strings.Join(d.Args, " ")
		if j != "80" && j != "[::]:80" && j != "443 ssl" && j != "[::]:443 ssl" {
			return bad(d, "hanya 80, [::]:80, 443 ssl, [::]:443 ssl")
		}
	case "server_name":
		if len(d.Args) < 1 || len(d.Args) > 11 || d.Args[0] != p.host {
			return bad(d, "nama pertama harus tepat "+p.host+" (maks 10 domain tambahan)")
		}
		for _, al := range d.Args[1:] {
			if !aliasOK(a, al) {
				return bad(d, "domain tambahan harus berbentuk nama."+a.TLD)
			}
			if al == p.host || aliasConflict(a, p.name, al) {
				return bad(d, "domain "+al+" sudah dipakai situs lain")
			}
		}
	case "allow":
		if len(d.Args) != 1 || !inTailnet(d.Args[0]) {
			return bad(d, "hanya IP/CIDR di dalam tailnet atau loopback (aturan hanya boleh mempersempit)")
		}
	case "deny":
		if len(d.Args) != 1 || d.Args[0] != "all" {
			return bad(d, "hanya 'deny all'")
		}
	case "auth_basic":
		if len(d.Args) != 1 || strings.ContainsAny(d.Args[0], "$") || len(d.Args[0]) > 60 {
			return bad(d, "satu string tanpa variabel, atau off")
		}
	case "auth_basic_user_file":
		if len(d.Args) != 1 || d.Args[0] != AuthFile(a, p.name) {
			return bad(d, "harus berkas htpasswd milik situs ini")
		}
	case "include":
		ok := len(d.Args) == 1 && (d.Args[0] == a.NginxDir+"/snippets/tailnet_only.conf" || d.Args[0] == a.NginxDir+"/snippets/proxy.conf" || d.Args[0] == "/opt/homebrew/etc/nginx/fastcgi_params")
		if !ok {
			return bad(d, "hanya snippet tailnet_only.conf, proxy.conf, atau fastcgi_params")
		}
	case "ssl_certificate":
		if len(d.Args) != 1 || d.Args[0] != a.CertDir+"/"+p.host+".pem" {
			return bad(d, "path sertifikat harus milik situs ini")
		}
	case "ssl_certificate_key":
		if len(d.Args) != 1 || d.Args[0] != a.CertDir+"/"+p.host+"-key.pem" {
			return bad(d, "path kunci harus milik situs ini")
		}
	case "root":
		if len(d.Args) != 1 || strings.Contains(d.Args[0], "$") {
			return bad(d, "satu path tetap tanpa variabel")
		}
		if _, err := a.UnderDev(d.Args[0]); err != nil {
			return bad(d, err.Error())
		}
	case "proxy_pass":
		if len(d.Args) != 1 {
			return bad(d, "satu argumen")
		}
		m := proxyPassOK.FindStringSubmatch(d.Args[0])
		if m == nil {
			return bad(d, "hanya http://127.0.0.1:PORT[/path]")
		}
		if _, err := a.ValidTarget(m[1]); err != nil {
			return bad(d, err.Error())
		}
	case "fastcgi_pass":
		if len(d.Args) != 1 || !phpSockOK.MatchString(d.Args[0]) {
			return bad(d, "hanya socket php-fpm-dev-8.2..8.5")
		}
	case "fastcgi_param":
		if len(d.Args) < 2 || len(d.Args) > 3 {
			return bad(d, "format: nama nilai")
		}
		n := d.Args[0]
		if n == "PHP_VALUE" || n == "PHP_ADMIN_VALUE" || (n == "HTTP_PROXY" && d.Args[1] != "") {
			return bad(d, "parameter "+n+" dilarang")
		}
		if n == "SCRIPT_FILENAME" && d.Args[1] != "$realpath_root$fastcgi_script_name" && d.Args[1] != "$document_root$fastcgi_script_name" {
			return bad(d, "SCRIPT_FILENAME harus $realpath_root$fastcgi_script_name")
		}
	case "access_log":
		if len(d.Args) != 1 || (d.Args[0] != "off" && d.Args[0] != a.LogDir+"/"+p.name+".log") {
			return bad(d, "hanya 'off' atau log situs ini")
		}
	case "location":
		if len(d.Args) < 1 || len(d.Args) > 2 {
			return bad(d, "argumen location tidak valid")
		}
		if len(d.Args) == 2 {
			switch d.Args[0] {
			case "~", "~*", "=", "^~":
			default:
				return bad(d, "modifier tidak valid")
			}
		}
		if len(d.Args[len(d.Args)-1]) > 200 {
			return bad(d, "pola terlalu panjang")
		}
	case "rewrite":
		if len(d.Args) < 2 || len(d.Args) > 3 {
			return bad(d, "format: regex pengganti [flag]")
		}
		if len(d.Args) == 3 {
			switch d.Args[2] {
			case "last", "break", "redirect", "permanent":
			default:
				return bad(d, "flag tidak valid")
			}
		}
	case "return":
		if len(d.Args) < 1 || len(d.Args) > 2 {
			return bad(d, "argumen tidak valid")
		}
	}
	return nil
}
