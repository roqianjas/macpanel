package sites

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type SiteRow struct {
	SiteInfo
	Enabled bool  `json:"enabled"`
	HTTP    int   `json:"http"`
	MS      int   `json:"ms"`
	SSLDays *int  `json:"ssl_days"`
	Created int64 `json:"created"`
}

func sslDays(a *core.App, host string) *int {
	b, err := os.ReadFile(a.CertDir + "/" + host + ".pem")
	if err != nil {
		return nil
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return nil
	}
	d := int(c.NotAfter.Sub(a.Now()).Hours() / 24)
	return &d
}

// probeSite memeriksa situs lewat loopback saja (127.0.0.1:443 dengan SNI situs itu); tidak pernah ke alamat lain.
func probeSite(a *core.App, host string) (int, int) {
	start := time.Now()
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("tcp", a.ProbeAddr)
	if err != nil {
		return 0, 0
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	tc := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, ServerName: host, NextProtos: []string{"http/1.1"}})
	if tc.Handshake() != nil {
		return 0, 0
	}
	req := "GET / HTTP/1.1\r\nHost: " + host + "\r\nConnection: close\r\nUser-Agent: panel-probe\r\n\r\n"
	if _, err := tc.Write([]byte(req)); err != nil {
		return 0, 0
	}
	buf := make([]byte, 64)
	n, _ := tc.Read(buf)
	line := string(buf[:n])
	if !strings.HasPrefix(line, "HTTP/") || len(line) < 12 {
		return 0, 0
	}
	code, _ := strconv.Atoi(line[9:12])
	return code, int(time.Since(start).Milliseconds())
}

func ListSiteRows(a *core.App) []SiteRow {
	base := listSites(a)
	rows := make([]SiteRow, 0, len(base)+2)
	for _, s := range base {
		rows = append(rows, SiteRow{SiteInfo: s, Enabled: true})
	}
	dis, _ := filepath.Glob(filepath.Join(a.SitesDir, "*.conf.disabled"))
	for _, f := range dis {
		name := strings.TrimSuffix(filepath.Base(f), ".conf.disabled")
		if !core.SiteRE.MatchString(name) {
			continue
		}
		b, _ := os.ReadFile(f)
		s := SiteInfo{Name: name, Host: name + "." + a.TLD, Mode: "kustom", System: core.ProtectedSites[name]}
		if m := phpSockRE.FindStringSubmatch(string(b)); m != nil {
			s.Mode, s.PHP = "php", m[1]
		} else if m := proxyRE.FindStringSubmatch(string(b)); m != nil {
			s.Mode, s.Target = "proxy", m[1]
		} else if m := rootRE.FindStringSubmatch(string(b)); m != nil {
			s.Mode, s.Target = "statis", strings.TrimSpace(m[1])
		}
		rows = append(rows, SiteRow{SiteInfo: s, Enabled: false})
	}
	var wg sync.WaitGroup
	for i := range rows {
		i := i
		if fi, err := os.Stat(a.SiteFile(rows[i].Name)); err == nil {
			rows[i].Created = fi.ModTime().Unix()
		}
		rows[i].SSLDays = sslDays(a, rows[i].Host)
		if !rows[i].Enabled || a.ProbeAddr == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows[i].HTTP, rows[i].MS = probeSite(a, rows[i].Host)
		}()
	}
	wg.Wait()
	sortSiteRows(rows)
	return rows
}

func sortSiteRows(rows []SiteRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].Name < rows[j-1].Name; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func SitesRich(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.WriteJSON(w, 200, map[string]any{"sites": ListSiteRows(a)})
}

func SiteToggle(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name := r.PathValue("name")
	act := r.PathValue("action")
	if !core.SiteRE.MatchString(name) || core.ProtectedSites[name] {
		core.WriteError(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	if act != "enable" && act != "disable" {
		core.WriteError(w, 400, "aksi harus enable atau disable")
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if act == "disable" && !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "site_"+act, name, a.Mutate([]string{"site", act, name}, ""))
}

type SiteInfo struct {
	Name   string `json:"name"`
	Host   string `json:"host"`
	Mode   string `json:"mode"`
	Target string `json:"target"`
	PHP    string `json:"php,omitempty"`
	System bool   `json:"system"`
}

var phpSockRE = regexp.MustCompile(`php-fpm-dev-(8\.[0-9])\.sock`)

var proxyRE = regexp.MustCompile(`proxy_pass http://127\.0\.0\.1:(\d+)`)

var rootRE = regexp.MustCompile(`(?m)^\s*root\s+([^;]+);`)

func listSites(a *core.App) []SiteInfo {
	out := []SiteInfo{}
	files, _ := filepath.Glob(filepath.Join(a.SitesDir, "*.conf"))
	sort.Strings(files)
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".conf")
		b, _ := os.ReadFile(f)
		s := SiteInfo{Name: name, Host: name + "." + a.TLD, Mode: "kustom", System: core.ProtectedSites[name]}
		txt := string(b)
		if m := phpSockRE.FindStringSubmatch(txt); m != nil {
			s.Mode, s.PHP = "php", m[1]
		} else if m := proxyRE.FindStringSubmatch(txt); m != nil {
			s.Mode, s.Target = "proxy", m[1]
		} else if rootRE.MatchString(txt) {
			s.Mode = "statis"
		}
		if m := rootRE.FindStringSubmatch(txt); m != nil && s.Mode != "proxy" {
			s.Target = strings.TrimSpace(m[1])
		}
		out = append(out, s)
	}
	return out
}

func SiteAdd(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Name, Target, PHP string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !core.SiteRE.MatchString(in.Name) || core.ProtectedSites[in.Name] {
		core.WriteError(w, 400, "nama situs tidak valid atau dicadangkan")
		return
	}
	target, err := a.ValidTarget(in.Target)
	if err != nil {
		core.WriteError(w, 400, err.Error())
		return
	}
	args := []string{"site", "add", in.Name, target}
	if in.PHP != "" {
		if !core.PhpVersions[in.PHP] || core.NumRE.MatchString(target) {
			core.WriteError(w, 400, "versi PHP tidak valid, atau PHP butuh folder (bukan port)")
			return
		}
		args = append(args, "--php", in.PHP)
	}
	a.Record(w, ss, a.ClientIP(r), "site_add", in.Name, a.Mutate(args, ""))
}

func SiteRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name := r.PathValue("name")
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !core.SiteRE.MatchString(name) || core.ProtectedSites[name] {
		core.WriteError(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	if in.Confirm != name {
		core.WriteError(w, 400, "ketik ulang nama situs untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "site_remove", name, a.Mutate([]string{"site", "remove", name}, ""))
}
