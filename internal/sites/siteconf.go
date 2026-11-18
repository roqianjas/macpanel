package sites

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
)

type Rewrite struct {
	From string `json:"from"`
	To   string `json:"to"`
	Flag string `json:"flag"`
}

type Model struct {
	Mode        string    `json:"mode"`   // php | proxy | statis
	Target      string    `json:"target"` // folder (php/statis) atau port (proxy)
	PHP         string    `json:"php,omitempty"`
	Preset      string    `json:"preset,omitempty"` // php: laravel|wordpress; statis: static|spa
	Rewrites    []Rewrite `json:"rewrites"`
	MaxBody     int       `json:"max_body_mb"`
	ReadTimeout int       `json:"read_timeout_s"`
	ForceHTTPS  bool      `json:"force_https"`
	Gzip        bool      `json:"gzip"`
	Aliases     []string  `json:"aliases,omitempty"`
	Auth        bool      `json:"auth,omitempty"`
	AllowIPs    []string  `json:"allow_ips,omitempty"`
}

const ModelHeader = "# panel-model: "

func validateModel(a *core.App, m *Model) error {
	switch m.Mode {
	case "php":
		if !core.PhpVersions[m.PHP] {
			return errors.New("versi PHP harus 8.2-8.5")
		}
		if m.Preset == "" {
			m.Preset = "laravel"
		}
		if m.Preset != "laravel" && m.Preset != "wordpress" {
			return errors.New("preset PHP: laravel atau wordpress")
		}
	case "statis":
		if m.Preset == "" {
			m.Preset = "static"
		}
		if m.Preset != "static" && m.Preset != "spa" {
			return errors.New("preset statis: static atau spa")
		}
		m.PHP = ""
	case "proxy":
		m.PHP, m.Preset = "", ""
	default:
		return errors.New("mode harus php, proxy, atau statis")
	}
	if m.Mode == "proxy" {
		if !core.NumRE.MatchString(m.Target) {
			return errors.New("target proxy harus port")
		}
	} else if core.NumRE.MatchString(m.Target) {
		return errors.New("target harus folder")
	}
	t, err := a.ValidTarget(m.Target)
	if err != nil {
		return err
	}
	m.Target = t
	if m.MaxBody < 1 || m.MaxBody > 1024 || m.ReadTimeout < 10 || m.ReadTimeout > 600 {
		return errors.New("max_body_mb 1-1024, read_timeout_s 10-600")
	}
	if err := validateExtras(a, m); err != nil {
		return err
	}
	if len(m.Rewrites) > 50 {
		return errors.New("maksimal 50 aturan rewrite")
	}
	if m.Rewrites == nil {
		m.Rewrites = []Rewrite{}
	}
	for _, r := range m.Rewrites {
		if !rewriteOK.MatchString(r.From) || len(r.From) > 200 || len(r.To) > 300 || !rewriteOK.MatchString(r.To) {
			return errors.New("aturan rewrite memuat karakter terlarang")
		}
		if !strings.HasPrefix(r.To, "/") && !strings.HasPrefix(r.To, "http://") && !strings.HasPrefix(r.To, "https://") {
			return errors.New("tujuan rewrite harus diawali / atau http(s)://")
		}
		switch r.Flag {
		case "", "last", "break", "redirect", "permanent":
		default:
			return errors.New("flag rewrite: last, break, redirect, permanent")
		}
	}
	return nil
}

func defaultModel(s SiteInfo) Model {
	m := Model{Mode: s.Mode, Target: s.Target, PHP: s.PHP, MaxBody: 64, ReadTimeout: 120, ForceHTTPS: true, Rewrites: []Rewrite{}}
	switch s.Mode {
	case "php":
		m.Preset = "laravel"
	case "statis":
		m.Preset, m.MaxBody = "static", 10
	default:
		m.MaxBody = 10
	}
	return m
}

func Generate(a *core.App, name string, m Model) (string, error) {
	if err := validateModel(a, &m); err != nil {
		return "", err
	}
	host := name + "." + a.TLD
	if m.Auth && len(authUsers(a, name)) == 0 {
		return "", errors.New("tambahkan minimal satu pengguna sebelum mengaktifkan password dasar")
	}
	names := host
	if len(m.Aliases) > 0 {
		names += " " + strings.Join(m.Aliases, " ")
	}
	acl := ""
	if len(m.AllowIPs) > 0 {
		acl = "    allow 127.0.0.1;\n    allow ::1;\n"
		for _, ip := range m.AllowIPs {
			acl += "    allow " + ip + ";\n"
		}
		acl += "    deny all;\n"
	}
	mj, _ := json.Marshal(m)
	var body strings.Builder
	fmt.Fprintf(&body, "    client_max_body_size %dm;\n", m.MaxBody)
	if m.Gzip {
		body.WriteString("    gzip on;\n    gzip_vary on;\n    gzip_min_length 1024;\n    gzip_types text/css application/javascript application/json image/svg+xml text/plain;\n")
	}
	fmt.Fprintf(&body, "    access_log %s/%s.log;\n", a.LogDir, name)
	if m.Auth {
		fmt.Fprintf(&body, "    auth_basic \"Restricted\";\n    auth_basic_user_file %s;\n", AuthFile(a, name))
	}
	for _, r := range m.Rewrites {
		if r.Flag != "" {
			fmt.Fprintf(&body, "    rewrite %s %s %s;\n", r.From, r.To, r.Flag)
		} else {
			fmt.Fprintf(&body, "    rewrite %s %s;\n", r.From, r.To)
		}
	}
	body.WriteString("    location ~ /\\.(?!well-known) { return 404; }\n")
	switch m.Mode {
	case "php":
		q := "$query_string"
		if m.Preset == "wordpress" {
			q = "$args"
		}
		fmt.Fprintf(&body, "    root %s;\n    index index.php index.html;\n    location / { try_files $uri $uri/ /index.php?%s; }\n", m.Target, q)
		loc := "^/index\\.php(/|$)"
		extra := ""
		if m.Preset == "wordpress" {
			loc, extra = "\\.php$", "        try_files $uri =404;\n"
		}
		fmt.Fprintf(&body, "    location ~ %s {\n%s        fastcgi_split_path_info ^(.+\\.php)(/.+)$;\n        fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-%s.sock;\n        include /opt/homebrew/etc/nginx/fastcgi_params;\n        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;\n        fastcgi_param PATH_INFO $fastcgi_path_info;\n        fastcgi_param HTTPS on;\n        fastcgi_read_timeout %ds;\n    }\n", loc, extra, m.PHP, m.ReadTimeout)
		if m.Preset == "laravel" {
			body.WriteString("    location ~ \\.php$ { return 404; }\n")
		}
	case "statis":
		fb := "=404"
		if m.Preset == "spa" {
			fb = "/index.html"
		}
		fmt.Fprintf(&body, "    root %s;\n    index index.html;\n    location / { try_files $uri $uri/ %s; }\n", m.Target, fb)
	case "proxy":
		fmt.Fprintf(&body, "    location / {\n        include %s/snippets/proxy.conf;\n        proxy_pass http://127.0.0.1:%s;\n        proxy_read_timeout %ds;\n    }\n", a.NginxDir, m.Target, m.ReadTimeout)
	}
	var b strings.Builder
	b.WriteString(ModelHeader + string(mj) + "\n")
	fmt.Fprintf(&b, "server {\n    listen 80; listen [::]:80;\n    server_name %s;\n%s    include %s/snippets/tailnet_only.conf;\n", names, acl, a.NginxDir)
	if m.ForceHTTPS {
		b.WriteString("    return 301 https://$host$request_uri;\n}\n")
	} else {
		b.WriteString(body.String() + "}\n")
	}
	fmt.Fprintf(&b, "server {\n    listen 443 ssl; listen [::]:443 ssl;\n    http2 on;\n    server_name %s;\n%s    include %s/snippets/tailnet_only.conf;\n    ssl_certificate     %s/%s.pem;\n    ssl_certificate_key %s/%s-key.pem;\n", names, acl, a.NginxDir, a.CertDir, host, a.CertDir, host)
	b.WriteString(body.String() + "}\n")
	return b.String(), nil
}

type SiteState struct {
	Kind    string // template | legacy | custom | kustom
	Model   *Model
	Content string
}

func readSite(a *core.App, name string) (*SiteState, error) {
	b, err := os.ReadFile(a.SiteFile(name))
	if err != nil {
		return nil, errors.New("situs tidak ada")
	}
	st := &SiteState{Content: string(b)}
	if first, _, _ := strings.Cut(st.Content, "\n"); strings.HasPrefix(first, ModelHeader) {
		var m Model
		d := json.NewDecoder(strings.NewReader(strings.TrimPrefix(first, ModelHeader)))
		d.DisallowUnknownFields()
		if d.Decode(&m) == nil {
			if g, err := Generate(a, name, m); err == nil && g == st.Content {
				st.Kind, st.Model = "template", &m
				return st, nil
			}
		}
		st.Kind = "custom"
		return st, nil
	}
	for _, s := range listSites(a) {
		if s.Name == name && s.Mode != "kustom" {
			m := defaultModel(s)
			st.Kind, st.Model = "legacy", &m
			return st, nil
		}
	}
	st.Kind = "kustom"
	return st, nil
}

func atomicWrite(path, content string) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp") // tidak berakhiran .conf: tidak ikut di-include nginx
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// applyConf: backup versi lama -> tulis atomik -> nginx -t -> reload; gagal di mana pun = config lama dipulihkan.
func applyConf(a *core.App, user, name, content, note string) core.Result {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	path := a.SiteFile(name)
	old, oldErr := os.ReadFile(path)
	if oldErr == nil {
		a.Store.AddVersion(name, user, string(old), "sebelum: "+note, a.Now())
	}
	_ = os.MkdirAll(a.LogDir, 0o755)
	if err := atomicWrite(path, content); err != nil {
		return core.Result{Code: 1, Out: "gagal menulis config: " + err.Error()}
	}
	revert := func() {
		if oldErr == nil {
			_ = atomicWrite(path, string(old))
		} else {
			os.Remove(path)
		}
	}
	if t := a.RunLab([]string{"nginx", "test"}, ""); t.Code != 0 {
		revert()
		return core.Result{Code: 1, Out: "nginx -t gagal, config lama dipulihkan:\n" + t.Out}
	}
	if r := a.RunLab([]string{"nginx", "reload"}, ""); r.Code != 0 {
		revert()
		a.RunLab([]string{"nginx", "reload"}, "")
		return core.Result{Code: 1, Out: "reload gagal, config lama dipulihkan:\n" + r.Out}
	}
	a.Store.AddVersion(name, user, content, note, a.Now())
	return core.Result{Out: "OK: config diterapkan, nginx reload berhasil"}
}
