package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- Layanan dan aplikasi ----------

type ServiceInfo struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Version   string `json:"version"`
	State     string `json:"state"`
	Port      int    `json:"port,omitempty"`
	Installed bool   `json:"installed"`
	Ctl       bool   `json:"ctl"`
	Optional  bool   `json:"optional"`
	Desc      string `json:"desc"`
	HasLog    bool   `json:"has_log"`
}

var (
	svcIDRE     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,19}$`)
	svcActions  = map[string]bool{"start": true, "stop": true, "restart": true}
	optionalApp = map[string]bool{"memcached": true, "mailpit": true}
	svcStates   = map[string]bool{"running": true, "stopped": true, "not_installed": true, "error": true}
)

func hServicesList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLabT([]string{"service", "list", "--json"}, "", 40*time.Second)
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca daftar layanan")
		return
	}
	rows := []ServiceInfo{}
	if err := json.Unmarshal([]byte(res.Out), &rows); err != nil {
		jerr(w, 502, "keluaran layanan tidak terbaca")
		return
	}
	for i := range rows { // nilai dari lab dibatasi lagi di sisi panel
		if !svcStates[rows[i].State] {
			rows[i].State = "error"
		}
	}
	jsonOut(w, 200, map[string]any{"services": rows})
}

func svcParams(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !svcIDRE.MatchString(id) {
		jerr(w, 400, "id layanan tidak valid")
		return "", false
	}
	return id, true
}

func hServiceAction(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	id, ok := svcParams(w, r)
	if !ok {
		return
	}
	act := r.PathValue("action")
	if !svcActions[act] {
		jerr(w, 400, "aksi harus start, stop, atau restart")
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if id == "panel" || id == "dnsmasq" {
		jerr(w, 400, "layanan ini tidak dikendalikan dari panel")
		return
	}
	if act != "start" && !a.requireStepUp(w, ss) { // menghentikan/memulai ulang bisa memutus situs dan database
		return
	}
	a.record(w, ss, a.clientIP(r), "service_"+act, id, a.mutate([]string{"service", act, id}, ""))
}

func hServiceLogs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	id, ok := svcParams(w, r)
	if !ok {
		return
	}
	n := 60
	if q := r.URL.Query().Get("n"); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || !logN.MatchString(q) || v < 1 || v > 500 {
			jerr(w, 400, "n 1-500")
			return
		}
		n = v
	}
	res := a.runLab([]string{"service", "logs", id, "-n", strconv.Itoa(n)}, "")
	if res.Code != 0 {
		jerr(w, 404, "log tidak tersedia untuk layanan ini")
		return
	}
	jsonOut(w, 200, map[string]any{"id": id, "log": res.Out})
}

func hAppInstall(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	id := r.PathValue("id")
	if !optionalApp[id] {
		jerr(w, 400, "aplikasi tidak ada di katalog")
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "app_install", id, []string{"app", "install", id}, ""))
}

func hAppRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	id := r.PathValue("id")
	if !optionalApp[id] {
		jerr(w, 400, "aplikasi tidak ada di katalog")
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if in.Confirm != id {
		jerr(w, 400, "ketik ulang nama aplikasi untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "app_remove", id, []string{"app", "remove", id}, ""))
}

// ---------- Situs: daftar kaya, jeda/aktifkan ----------

type SiteRow struct {
	SiteInfo
	Enabled bool  `json:"enabled"`
	HTTP    int   `json:"http"`
	MS      int   `json:"ms"`
	SSLDays *int  `json:"ssl_days"`
	Created int64 `json:"created"`
}

func (a *App) sslDays(host string) *int {
	b, err := os.ReadFile(a.certDir + "/" + host + ".pem")
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
	d := int(c.NotAfter.Sub(a.now()).Hours() / 24)
	return &d
}

// probeSite memeriksa situs lewat loopback saja (127.0.0.1:443 dengan SNI situs itu); tidak pernah ke alamat lain.
func (a *App) probeSite(host string) (int, int) {
	start := time.Now()
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("tcp", a.probeAddr)
	if err != nil {
		return 0, 0
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	tc := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, ServerName: host, NextProtos: []string{"http/1.1"}})
	if tc.Handshake() != nil {
		return 0, 0
	}
	req := "HEAD / HTTP/1.1\r\nHost: " + host + "\r\nConnection: close\r\nUser-Agent: panel-probe\r\n\r\n"
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

func (a *App) listSiteRows() []SiteRow {
	base := a.listSites()
	rows := make([]SiteRow, 0, len(base)+2)
	for _, s := range base {
		rows = append(rows, SiteRow{SiteInfo: s, Enabled: true})
	}
	dis, _ := filepath.Glob(filepath.Join(a.sitesDir, "*.conf.disabled"))
	for _, f := range dis {
		name := strings.TrimSuffix(filepath.Base(f), ".conf.disabled")
		if !siteRE.MatchString(name) {
			continue
		}
		b, _ := os.ReadFile(f)
		s := SiteInfo{Name: name, Host: name + "." + a.tld, Mode: "kustom", System: protectedSites[name]}
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
		if fi, err := os.Stat(a.siteFile(rows[i].Name)); err == nil {
			rows[i].Created = fi.ModTime().Unix()
		}
		rows[i].SSLDays = a.sslDays(rows[i].Host)
		if !rows[i].Enabled || a.probeAddr == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows[i].HTTP, rows[i].MS = a.probeSite(rows[i].Host)
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

func hSitesRich(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	jsonOut(w, 200, map[string]any{"sites": a.listSiteRows()})
}

func hSiteToggle(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name := r.PathValue("name")
	act := r.PathValue("action")
	if !siteRE.MatchString(name) || protectedSites[name] {
		jerr(w, 400, "situs ini dilindungi atau namanya tidak valid")
		return
	}
	if act != "enable" && act != "disable" {
		jerr(w, 400, "aksi harus enable atau disable")
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if act == "disable" && !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "site_"+act, name, a.mutate([]string{"site", act, name}, ""))
}
