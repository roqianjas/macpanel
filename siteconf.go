package main

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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
}

const modelHeader = "# panel-model: "

func (a *App) validateModel(m *Model) error {
	switch m.Mode {
	case "php":
		if !phpVersions[m.PHP] {
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
		if !numRE.MatchString(m.Target) {
			return errors.New("target proxy harus port")
		}
	} else if numRE.MatchString(m.Target) {
		return errors.New("target harus folder")
	}
	t, err := a.validTarget(m.Target)
	if err != nil {
		return err
	}
	m.Target = t
	if m.MaxBody < 1 || m.MaxBody > 1024 || m.ReadTimeout < 10 || m.ReadTimeout > 600 {
		return errors.New("max_body_mb 1-1024, read_timeout_s 10-600")
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

func (a *App) generate(name string, m Model) (string, error) {
	if err := a.validateModel(&m); err != nil {
		return "", err
	}
	host := name + "." + a.tld
	mj, _ := json.Marshal(m)
	var body strings.Builder
	fmt.Fprintf(&body, "    client_max_body_size %dm;\n", m.MaxBody)
	if m.Gzip {
		body.WriteString("    gzip on;\n    gzip_vary on;\n    gzip_min_length 1024;\n    gzip_types text/css application/javascript application/json image/svg+xml text/plain;\n")
	}
	fmt.Fprintf(&body, "    access_log %s/%s.log;\n", a.logDir, name)
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
		fmt.Fprintf(&body, "    location / {\n        include %s/snippets/proxy.conf;\n        proxy_pass http://127.0.0.1:%s;\n        proxy_read_timeout %ds;\n    }\n", a.nginxDir, m.Target, m.ReadTimeout)
	}
	var b strings.Builder
	b.WriteString(modelHeader + string(mj) + "\n")
	fmt.Fprintf(&b, "server {\n    listen 80; listen [::]:80;\n    server_name %s;\n    include %s/snippets/tailnet_only.conf;\n", host, a.nginxDir)
	if m.ForceHTTPS {
		b.WriteString("    return 301 https://$host$request_uri;\n}\n")
	} else {
		b.WriteString(body.String() + "}\n")
	}
	fmt.Fprintf(&b, "server {\n    listen 443 ssl; listen [::]:443 ssl;\n    http2 on;\n    server_name %s;\n    include %s/snippets/tailnet_only.conf;\n    ssl_certificate     %s/%s.pem;\n    ssl_certificate_key %s/%s-key.pem;\n", host, a.nginxDir, a.certDir, host, a.certDir, host)
	b.WriteString(body.String() + "}\n")
	return b.String(), nil
}

type SiteState struct {
	Kind    string // template | legacy | custom | kustom
	Model   *Model
	Content string
}

func (a *App) siteFile(name string) string { return filepath.Join(a.sitesDir, name+".conf") }

func (a *App) readSite(name string) (*SiteState, error) {
	b, err := os.ReadFile(a.siteFile(name))
	if err != nil {
		return nil, errors.New("situs tidak ada")
	}
	st := &SiteState{Content: string(b)}
	if first, _, _ := strings.Cut(st.Content, "\n"); strings.HasPrefix(first, modelHeader) {
		var m Model
		d := json.NewDecoder(strings.NewReader(strings.TrimPrefix(first, modelHeader)))
		d.DisallowUnknownFields()
		if d.Decode(&m) == nil {
			if g, err := a.generate(name, m); err == nil && g == st.Content {
				st.Kind, st.Model = "template", &m
				return st, nil
			}
		}
		st.Kind = "custom"
		return st, nil
	}
	for _, s := range a.listSites() {
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
func (a *App) applyConf(user, name, content, note string) Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := a.siteFile(name)
	old, oldErr := os.ReadFile(path)
	if oldErr == nil {
		a.st.AddVersion(name, user, string(old), "sebelum: "+note, a.now())
	}
	os.MkdirAll(a.logDir, 0o755)
	if err := atomicWrite(path, content); err != nil {
		return Result{Code: 1, Out: "gagal menulis config: " + err.Error()}
	}
	revert := func() {
		if oldErr == nil {
			atomicWrite(path, string(old))
		} else {
			os.Remove(path)
		}
	}
	if t := a.runLab([]string{"nginx", "test"}, ""); t.Code != 0 {
		revert()
		return Result{Code: 1, Out: "nginx -t gagal, config lama dipulihkan:\n" + t.Out}
	}
	if r := a.runLab([]string{"nginx", "reload"}, ""); r.Code != 0 {
		revert()
		a.runLab([]string{"nginx", "reload"}, "")
		return Result{Code: 1, Out: "reload gagal, config lama dipulihkan:\n" + r.Out}
	}
	a.st.AddVersion(name, user, content, note, a.now())
	return Result{Out: "OK: config diterapkan, nginx reload berhasil"}
}

// ---------- versi (store) ----------

func (s *Store) AddVersion(site, user, content, note string, now time.Time) {
	sha := sha256hex(content)
	var last string
	s.db.QueryRow("SELECT sha FROM conf_versions WHERE site=? ORDER BY id DESC LIMIT 1", site).Scan(&last)
	if last == sha {
		return
	}
	s.db.Exec("INSERT INTO conf_versions(site,at,usr,sha,note,content) VALUES(?,?,?,?,?,?)", site, now.Unix(), user, sha, clean(note, 120), content)
	s.db.Exec("DELETE FROM conf_versions WHERE site=? AND id NOT IN (SELECT id FROM conf_versions WHERE site=? ORDER BY id DESC LIMIT 20)", site, site)
}

type VersionRow struct {
	ID   int64  `json:"id"`
	At   int64  `json:"at"`
	User string `json:"user"`
	Note string `json:"note"`
}

func (s *Store) Versions(site string) []VersionRow {
	out := []VersionRow{}
	rows, err := s.db.Query("SELECT id,at,usr,note FROM conf_versions WHERE site=? ORDER BY id DESC", site)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var v VersionRow
		rows.Scan(&v.ID, &v.At, &v.User, &v.Note)
		out = append(out, v)
	}
	return out
}

func (s *Store) VersionContent(site string, id int64) (string, bool) {
	var c string
	err := s.db.QueryRow("SELECT content FROM conf_versions WHERE site=? AND id=?", site, id).Scan(&c)
	return c, err == nil
}

// ---------- handler ----------

func (a *App) siteParam(w http.ResponseWriter, r *http.Request, write bool) (string, bool) {
	name := r.PathValue("name")
	if !siteRE.MatchString(name) {
		jerr(w, 400, "nama situs tidak valid")
		return "", false
	}
	if _, err := os.Stat(a.siteFile(name)); err != nil {
		jerr(w, 404, "situs tidak ada")
		return "", false
	}
	if write && protectedSites[name] {
		jerr(w, 400, "situs sistem hanya bisa dilihat, tidak diubah dari panel")
		return "", false
	}
	return name, true
}

func hSettingsGet(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	st, err := a.readSite(name)
	if err != nil {
		jerr(w, 404, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"name": name, "kind": st.Kind, "model": st.Model, "system": protectedSites[name]})
}

func hSettingsSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Model
		Replace bool `json:"replace"`
	}
	if !decode(w, r, &in) {
		return
	}
	st, err := a.readSite(name)
	if err != nil {
		jerr(w, 404, err.Error())
		return
	}
	if st.Kind == "kustom" {
		jerr(w, 400, "situs ini memakai config kustom; ubah lewat editor config")
		return
	}
	if st.Kind == "custom" && !in.Replace {
		jerr(w, 409, "config sudah diedit manual; kirim replace=true untuk menimpanya (versi lama tersimpan)")
		return
	}
	m := in.Model
	if m.Mode == "" {
		jerr(w, 400, "mode wajib diisi")
		return
	}
	content, err := a.generate(name, m)
	if err != nil {
		jerr(w, 400, err.Error())
		return
	}
	if err := a.checkConf(name, content); err != nil { // sabuk pengaman: hasil template harus lolos kebijakan yang sama
		jerr(w, 500, "template menghasilkan config yang ditolak kebijakan: "+err.Error())
		return
	}
	a.record(w, ss, a.clientIP(r), "site_settings", name, a.applyConf(ss.User, name, content, "pengaturan"))
}

func hConfigGet(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	b, _ := os.ReadFile(a.siteFile(name))
	jsonOut(w, 200, map[string]any{"name": name, "content": string(b), "sha": sha256hex(string(b)), "system": protectedSites[name], "versions": a.st.Versions(name)})
}

func hConfigSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ Content, SHA string }
	if !decodeN(w, r, &in, 2*maxConfSize+1024) { // JSON memperbesar (escape); isi sebenarnya tetap dibatasi maxConfSize oleh parser
		return
	}
	cur, _ := os.ReadFile(a.siteFile(name))
	if in.SHA != sha256hex(string(cur)) {
		jerr(w, 409, "config berubah sejak dibuka; muat ulang")
		return
	}
	if err := a.checkConf(name, in.Content); err != nil {
		jerr(w, 400, "ditolak kebijakan: "+err.Error())
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "site_config", name, a.applyConf(ss.User, name, in.Content, "editor config"))
}

func hRestore(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ ID int64 }
	if !decode(w, r, &in) {
		return
	}
	c, found := a.st.VersionContent(name, in.ID)
	if !found {
		jerr(w, 404, "versi tidak ada")
		return
	}
	if err := a.checkConf(name, c); err != nil {
		jerr(w, 400, "versi itu ditolak kebijakan saat ini: "+err.Error())
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "site_restore", name+"#"+strconv.FormatInt(in.ID, 10), a.applyConf(ss.User, name, c, "pulihkan versi "+strconv.FormatInt(in.ID, 10)))
}

func hSSLGet(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	host := name + "." + a.tld
	b, err := os.ReadFile(a.certDir + "/" + host + ".pem")
	if err != nil {
		jsonOut(w, 200, map[string]any{"host": host, "exists": false})
		return
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		jerr(w, 500, "sertifikat tidak terbaca")
		return
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		jerr(w, 500, "sertifikat tidak valid")
		return
	}
	days := int(c.NotAfter.Sub(a.now()).Hours() / 24)
	jsonOut(w, 200, map[string]any{"host": host, "exists": true, "issuer": c.Issuer.CommonName, "dns": c.DNSNames,
		"not_before": c.NotBefore.Unix(), "not_after": c.NotAfter.Unix(), "days_left": days, "expired": days < 0,
		"note": "Sertifikat lokal (mkcert) untuk domain .lab. Let's Encrypt tidak berlaku untuk .lab."})
}

func hSSLRenew(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "ssl_renew", name, a.mutate([]string{"site", "cert", name}, ""))
}

var logN = regexp.MustCompile(`^[0-9]{1,3}$`)

func tailLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, _ := f.Stat()
	const win = 1 << 20
	off := int64(0)
	if st.Size() > win {
		off = st.Size() - win
	}
	f.Seek(off, io.SeekStart)
	b, _ := io.ReadAll(f)
	if off > 0 {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func hLogs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	n := 100
	if q := r.URL.Query().Get("n"); q != "" {
		if !logN.MatchString(q) {
			jerr(w, 400, "n tidak valid")
			return
		}
		n, _ = strconv.Atoi(q)
		if n < 1 || n > 500 {
			jerr(w, 400, "n 1-500")
			return
		}
	}
	kind := r.URL.Query().Get("kind")
	var lines []string
	var err error
	switch kind {
	case "access":
		lines, err = tailLines(a.logDir+"/"+name+".log", n)
	case "error":
		var all []string
		all, err = tailLines(a.errLog, 5000)
		host := "server: " + name + "." + a.tld
		for _, l := range all {
			if strings.Contains(l, host) {
				lines = append(lines, l)
			}
		}
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	default:
		jerr(w, 400, "kind: access atau error")
		return
	}
	if err != nil || lines == nil {
		lines = []string{}
	}
	for i := range lines {
		lines[i] = scrub(clean(lines[i], 600))
	}
	jsonOut(w, 200, map[string]any{"kind": kind, "lines": lines, "note": "log akses per situs aktif setelah pengaturan disimpan lewat panel"})
}
