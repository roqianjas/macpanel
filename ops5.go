package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ---------- unggah dump ----------

const maxUpload = 256 << 20

// hDBUpload menyimpan dump unggahan ke ~/MacPanel/backup/db sebagai <db>-<engine>-<waktu>.sql.gz (gzip bila belum).
// Isi tidak dipercaya: `lab db restore` memindai dan menjalankannya sebagai user proyek, hanya ke database kosong.
func hDBUpload(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	br := bufio.NewReader(r.Body)
	head, _ := br.Peek(2)
	isGz := len(head) == 2 && head[0] == 0x1f && head[1] == 0x8b
	if len(head) == 0 {
		jerr(w, 400, "berkas kosong")
		return
	}
	dir := a.dumpDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		jerr(w, 500, "folder dump tidak bisa dibuat")
		return
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		jerr(w, 500, "tidak bisa menyimpan")
		return
	}
	defer os.Remove(tmp.Name())
	tmp.Chmod(0o600)
	var cerr error
	if isGz {
		_, cerr = io.Copy(tmp, br)
	} else {
		gz := gzip.NewWriter(tmp)
		if _, cerr = io.Copy(gz, br); cerr == nil {
			cerr = gz.Close()
		}
	}
	if cerr == nil {
		cerr = tmp.Close()
	}
	if cerr != nil {
		code, msg := 400, "unggahan gagal"
		if _, big := cerr.(*http.MaxBytesError); big {
			code, msg = 413, fmt.Sprintf("berkas lebih dari %d MB", maxUpload>>20)
		}
		jerr(w, code, msg)
		return
	}
	var final string
	for i := 0; i < 20; i++ {
		final = fmt.Sprintf("%s-%s-%s.sql.gz", name, engine, a.now().Add(time.Duration(i)*time.Second).Format("20060102-150405"))
		if _, err := os.Lstat(filepath.Join(dir, final)); os.IsNotExist(err) {
			break
		}
		final = ""
	}
	if final == "" {
		jerr(w, 409, "terlalu banyak unggahan dalam waktu dekat")
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, final)); err != nil {
		jerr(w, 500, "tidak bisa menyimpan")
		return
	}
	fi, _ := os.Stat(filepath.Join(dir, final))
	a.st.Audit(ss.User, "db_upload", engine+"/"+name+" "+final, a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"file": final, "size": fi.Size()})
}

// ---------- peringatan ----------

type Alert struct {
	ID     string `json:"id"`
	Level  string `json:"level"` // bad | warn
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Link   string `json:"link"`
}

// computeAlerts murni (tanpa I/O) agar mudah diuji. Semua ambang ada di sini.
func computeAlerts(si *SystemInfo, svcs []ServiceInfo, sites []SiteRow, now time.Time, probed bool) []Alert {
	out := []Alert{}
	if si != nil && si.DiskTotal > 0 {
		used := 100 * (1 - float64(si.DiskFree)/float64(si.DiskTotal))
		switch {
		case used >= 92:
			out = append(out, Alert{"disk", "bad", "Disk hampir penuh", fmt.Sprintf("%.0f%% terpakai, sisa %.1f GB", used, float64(si.DiskFree)/1e9), "#/pantau"})
		case used >= 85:
			out = append(out, Alert{"disk", "warn", "Disk mulai penuh", fmt.Sprintf("%.0f%% terpakai, sisa %.1f GB", used, float64(si.DiskFree)/1e9), "#/pantau"})
		}
	}
	if si != nil && si.Mem >= 95 {
		out = append(out, Alert{"mem", "warn", "Memori hampir habis", fmt.Sprintf("%.0f%% terpakai", si.Mem), "#/pantau"})
	}
	for _, s := range svcs {
		if s.Installed && !s.Optional && s.State != "running" {
			out = append(out, Alert{"svc-" + s.ID, "bad", s.Label + " tidak berjalan", "Status: " + s.State, "#/aplikasi"})
		}
	}
	for _, s := range sites {
		if !s.Enabled {
			continue
		}
		switch {
		case s.HTTP == 0 && probed:
			out = append(out, Alert{"site-" + s.Name, "bad", s.Host + " tidak menjawab", "Pemeriksaan HTTP gagal", "#/website"})
		case s.HTTP >= 500:
			out = append(out, Alert{"site-" + s.Name, "bad", s.Host + " error server", fmt.Sprintf("HTTP %d", s.HTTP), "#/website"})
		}
		if s.SSLDays != nil {
			switch {
			case *s.SSLDays <= 0:
				out = append(out, Alert{"ssl-" + s.Name, "bad", "Sertifikat " + s.Host + " kedaluwarsa", "Perbarui di tab SSL", "#/website"})
			case *s.SSLDays < 14:
				out = append(out, Alert{"ssl-" + s.Name, "warn", "Sertifikat " + s.Host + " segera habis", fmt.Sprintf("%d hari lagi", *s.SSLDays), "#/website"})
			}
		}
	}
	if si != nil {
		switch {
		case si.Backup == nil:
			out = append(out, Alert{"backup", "warn", "Belum ada backup", "Jalankan backup pertama", "#/backup"})
		case now.Unix()-si.Backup.At > 36*3600:
			out = append(out, Alert{"backup", "warn", "Backup terakhir sudah lama", fmt.Sprintf("%d jam lalu", (now.Unix()-si.Backup.At)/3600), "#/backup"})
		}
	}
	return out
}

func hAlerts(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	a.alMu.Lock()
	defer a.alMu.Unlock()
	if a.alCache == nil || a.now().Sub(a.alAt) > 30*time.Second {
		a.sysMu.Lock()
		if a.sysCache == nil || a.now().Sub(a.sysAt) > 8*time.Second {
			si := a.systemInfo()
			a.sysCache, a.sysAt = &si, a.now()
		}
		si := *a.sysCache
		a.sysMu.Unlock()
		var svcs []ServiceInfo
		if res := a.runLabT([]string{"service", "list", "--json"}, "", 40*time.Second); res.Code == 0 {
			json.Unmarshal([]byte(res.Out), &svcs)
		}
		for i := range svcs {
			if !svcStates[svcs[i].State] {
				svcs[i].State = "error"
			}
		}
		a.alCache, a.alAt = computeAlerts(&si, svcs, a.listSiteRows(), a.now(), a.probeAddr != ""), a.now()
	}
	jsonOut(w, 200, map[string]any{"alerts": a.alCache, "at": a.alAt.Unix(), "notify": "hanya ditampilkan di panel; tidak ada pesan keluar"})
}

// ---------- pengguna ----------

func hUsers(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	if ss.Role == "viewer" {
		jerr(w, 403, "hanya admin")
		return
	}
	jsonOut(w, 200, map[string]any{"users": a.st.UserRows(), "howto": "Tambah atau ubah peran lewat terminal Mac mini: panel user add <nama> --role viewer; panel user role <nama> admin|viewer"})
}
