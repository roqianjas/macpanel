package main

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------- job asinkron ----------

func (s *Store) FinishJob(id int64, status string, code int, out string) {
	s.db.Exec("UPDATE jobs SET status=?, code=?, output=? WHERE id=?", status, code, out, id)
}

// MarkStaleJobs menandai job yang masih "berjalan" saat panel baru start (prosesnya sudah mati bersama panel).
func (s *Store) MarkStaleJobs() {
	s.db.Exec("UPDATE jobs SET status='terputus' WHERE status='berjalan'")
}

func (s *Store) GetJob(id int64) *JobRow {
	var j JobRow
	if err := s.db.QueryRow("SELECT id,at,usr,action,status,code,output FROM jobs WHERE id=?", id).Scan(&j.ID, &j.At, &j.User, &j.Action, &j.Status, &j.Code, &j.Output); err != nil {
		return nil
	}
	return &j
}

func statusOf(res Result) string {
	switch {
	case res.TimedOut:
		return "timeout"
	case res.Code != 0:
		return "gagal"
	}
	return "ok"
}

// startAsync menjalankan aksi lama (install, update) di latar belakang; klien memantau lewat GET /api/jobs/{id}.
func (a *App) startAsync(ss *Session, ip, action, detail string, args []string, stdin string) int64 {
	id := a.st.AddJob(ss.User, action+" "+detail, "berjalan", "", 0, a.now())
	a.st.Audit(ss.User, action, detail+" -> mulai", ip, a.now())
	user := ss.User
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		a.mu.Lock()
		res := a.runLabT(args, stdin, a.longTimeout)
		a.mu.Unlock()
		st := statusOf(res)
		a.st.FinishJob(id, st, res.Code, res.Out)
		a.st.Audit(user, action, detail+" -> "+st, ip, a.now())
	}()
	return id
}

func respondAsync(w http.ResponseWriter, id int64) {
	jsonOut(w, 202, map[string]any{"job": id, "status": "berjalan"})
}

func hJob(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		jerr(w, 400, "id tidak valid")
		return
	}
	j := a.st.GetJob(id)
	if j == nil {
		jerr(w, 404, "job tidak ada")
		return
	}
	jsonOut(w, 200, j)
}

// ---------- parser keluaran lab ----------

type PHPInfo struct {
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	Full      string `json:"full,omitempty"`
	FPM       bool   `json:"fpm"`
}

var phpLineRE = regexp.MustCompile(`^(8\.[0-9])\s+terpasang=(ya|tidak)\s+versi=(\S+)\s+fpm_socket=(\S+)`)

func parsePHP(out string) []PHPInfo {
	res := []PHPInfo{}
	for _, l := range strings.Split(out, "\n") {
		if m := phpLineRE.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			p := PHPInfo{Version: m[1], Installed: m[2] == "ya", FPM: m[4] == "ada"}
			if p.Installed {
				p.Full = m[3]
			}
			res = append(res, p)
		}
	}
	return res
}

type RuntimeInfo struct {
	Tool    string `json:"tool"`
	Version string `json:"version"`
	Default bool   `json:"default"`
	Pruned  bool   `json:"pruned"`
}

var runtimeLineRE = regexp.MustCompile(`^(node|go|python|pnpm)\s+(\S+)(.*)$`)

func parseRuntime(out string) []RuntimeInfo {
	res := []RuntimeInfo{}
	for _, l := range strings.Split(out, "\n") {
		if m := runtimeLineRE.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			res = append(res, RuntimeInfo{Tool: m[1], Version: m[2], Default: strings.Contains(m[3], "config.toml"), Pruned: strings.Contains(m[3], "pruned")})
		}
	}
	return res
}

type UpdateRow struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	From   string `json:"from"`
	To     string `json:"to"`
	Pinned bool   `json:"pinned"`
	Major  bool   `json:"major"`
}

var updLineRE = regexp.MustCompile(`^\s{2}(\S+)\s+(\S+)\s+->\s+(\S+)(.*)$`)

func parseUpdates(out string) ([]UpdateRow, []string) {
	rows, pins, src := []UpdateRow{}, []string{}, ""
	for _, l := range strings.Split(out, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "Homebrew:"):
			src = "homebrew"
		case strings.HasPrefix(t, "mise:"):
			src = "mise"
		case strings.HasPrefix(t, "Pin aktif:"):
			for _, p := range strings.Split(strings.TrimPrefix(t, "Pin aktif:"), ",") {
				if p = strings.TrimSpace(p); p != "" && p != "-" {
					pins = append(pins, p)
				}
			}
			src = ""
		default:
			if m := updLineRE.FindStringSubmatch(l); m != nil && src != "" {
				rows = append(rows, UpdateRow{Source: src, Name: m[1], From: m[2], To: m[3], Pinned: strings.Contains(m[4], "PIN"), Major: strings.Contains(m[4], "MAYOR")})
			}
		}
	}
	return rows, pins
}

type TaskRow struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	PID      string `json:"pid"`
	Exit     string `json:"exit"`
	Schedule string `json:"schedule"`
}

func parseTasks(out string) []TaskRow {
	res := []TaskRow{}
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) < 4 || f[0] == "NAMA" {
			continue
		}
		name, status, rest := f[0], f[1], f[2:]
		if status == "tidak" && len(rest) > 0 && rest[0] == "dimuat" {
			status, rest = "tidak dimuat", rest[1:]
		}
		if len(rest) < 2 {
			continue
		}
		res = append(res, TaskRow{Name: name, Status: status, PID: rest[0], Exit: rest[1], Schedule: strings.Join(rest[2:], " ")})
	}
	return res
}

// ---------- PHP ----------

func hPHPList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLab([]string{"php", "list"}, "")
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca daftar PHP")
		return
	}
	jsonOut(w, 200, map[string]any{"php": parsePHP(res.Out)})
}

func phpVer(w http.ResponseWriter, r *http.Request) (string, bool) {
	v := r.PathValue("version")
	if !phpVersions[v] {
		jerr(w, 400, "versi PHP harus 8.2-8.5")
		return "", false
	}
	return v, true
}

func hPHPInstall(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "php_install", v, []string{"php", "install", v}, ""))
}

func hPHPRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	v, ok := phpVer(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if in.Confirm != v {
		jerr(w, 400, "ketik ulang versi PHP untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "php_remove", v, []string{"php", "remove", v}, ""))
}

// ---------- Runtime ----------

var runtimeTools = map[string]bool{"node": true, "go": true, "python": true, "pnpm": true}
var versionRE = regexp.MustCompile(`^(latest|lts|[0-9]+(\.[0-9]+){0,2})$`)

func hRuntimeList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLab([]string{"runtime", "list"}, "")
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca daftar runtime")
		return
	}
	jsonOut(w, 200, map[string]any{"runtimes": parseRuntime(res.Out), "tools": []string{"node", "go", "python", "pnpm"}})
}

func rtParams(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	t, v := r.PathValue("tool"), r.PathValue("version")
	if !runtimeTools[t] || !versionRE.MatchString(v) {
		jerr(w, 400, "tool atau versi tidak valid")
		return "", "", false
	}
	return t, v, true
}

func hRuntimeInstall(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	t, v, ok := rtParams(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "runtime_install", t+"@"+v, []string{"runtime", "install", t, v}, ""))
}

func hRuntimeUse(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	t, v, ok := rtParams(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	a.record(w, ss, a.clientIP(r), "runtime_use", t+"@"+v, a.mutate([]string{"runtime", "use", t, v}, ""))
}

func hRuntimeRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	t, v, ok := rtParams(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if in.Confirm != t+"@"+v {
		jerr(w, 400, "ketik ulang "+t+"@"+v+" untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "runtime_remove", t+"@"+v, a.mutate([]string{"runtime", "remove", t, v}, ""))
}

// ---------- Worker dan Cron ----------

var (
	taskNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	cmdCharsRE = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,~ -]{1,300}$`) // tanpa kutip, $, ;, |, &, backslash: pemecahan token identik di panel dan di lab
	everyRE    = regexp.MustCompile(`^[0-9]{1,3}[mh]$`)
	atRE       = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

var cmdPrograms = map[string]map[string]bool{ // program yang boleh -> subperintah pertama yang boleh (nil = bebas)
	"php": nil, "node": nil, "python3": nil, "python": nil,
	"npm": {"run": true, "start": true}, "pnpm": {"run": true, "start": true}, "composer": {"run-script": true, "run": true},
}

var cmdBannedFlags = map[string]bool{
	"-r": true, "-R": true, "-B": true, "-E": true, "-F": true, "-f": true, "-d": true, "-S": true, "-a": true, "-i": true, // php
	"-e": true, "--eval": true, "-p": true, "--print": true, "--require": true, "-c": true, "--interactive": true, // node, python
}

// validCmd memeriksa perintah worker/cron. Worker menjalankan kode proyek seperti Supervisor; yang dicegah adalah
// kode sembarang lewat flag (-r, -e, -c), program asing, jalur di luar ~/Developer, dan port debug.
func (a *App) validCmd(cmd string) error {
	if !cmdCharsRE.MatchString(cmd) {
		return errors.New("perintah hanya boleh huruf, angka, dan _ . / : = @ % + , ~ - (tanpa kutip, $, ;, |, &)")
	}
	f := strings.Fields(cmd)
	allowed, ok := cmdPrograms[f[0]]
	if !ok {
		return errors.New("program harus salah satu: php, node, python3, python, npm, pnpm, composer")
	}
	if allowed != nil && (len(f) < 2 || !allowed[f[1]]) {
		return errors.New(f[0] + " hanya boleh dengan subperintah run/start")
	}
	for _, t := range f[1:] {
		switch {
		case cmdBannedFlags[t] || strings.HasPrefix(t, "--inspect") || t == "--eval" || strings.HasPrefix(t, "--eval="):
			return errors.New("flag " + t + " dilarang (kode sembarang atau port debug)")
		case strings.Contains(t, ".."):
			return errors.New("'..' dilarang")
		case strings.HasPrefix(t, "~"):
			return errors.New("~ tidak diperluas; pakai path absolut atau relatif")
		case strings.HasPrefix(t, "/"):
			if _, err := a.underDev(t); err != nil {
				return errors.New("path " + t + ": " + err.Error())
			}
		}
	}
	return nil
}

func taskKind(w http.ResponseWriter, r *http.Request) (string, bool) {
	k := r.PathValue("kind")
	if k != "worker" && k != "job" {
		jerr(w, 400, "jenis harus worker atau job")
		return "", false
	}
	return k, true
}

func hTasksList(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	k, ok := taskKind(w, r)
	if !ok {
		return
	}
	res := a.runLab([]string{k, "list"}, "")
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca daftar")
		return
	}
	jsonOut(w, 200, map[string]any{"kind": k, "tasks": parseTasks(res.Out)})
}

func hTaskAdd(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	k, ok := taskKind(w, r)
	if !ok {
		return
	}
	var in struct {
		Name, Dir, Cmd, PHP, Every, At string
		Procs                          int
	}
	if !decode(w, r, &in) {
		return
	}
	if !taskNameRE.MatchString(in.Name) {
		jerr(w, 400, "nama: huruf kecil/angka/-, diawali huruf, maks 40")
		return
	}
	dir, err := a.underDev(in.Dir)
	if err != nil {
		jerr(w, 400, err.Error())
		return
	}
	if isDir, _ := statDir(dir); !isDir {
		jerr(w, 400, "dir bukan folder")
		return
	}
	if err := a.validCmd(in.Cmd); err != nil {
		jerr(w, 400, err.Error())
		return
	}
	args := []string{k, "add", in.Name, "--dir", dir, "--cmd", in.Cmd}
	if in.PHP != "" {
		if !phpVersions[in.PHP] {
			jerr(w, 400, "versi PHP harus 8.2-8.5")
			return
		}
		args = append(args, "--php", in.PHP)
	}
	if k == "worker" {
		if in.Every != "" || in.At != "" {
			jerr(w, 400, "worker tidak memakai jadwal")
			return
		}
		if in.Procs < 0 || in.Procs > 8 {
			jerr(w, 400, "procs 1-8")
			return
		}
		if in.Procs > 1 {
			args = append(args, "--procs", strconv.Itoa(in.Procs))
		}
	} else {
		if in.Procs != 0 {
			jerr(w, 400, "job tidak memakai procs")
			return
		}
		if (in.Every == "") == (in.At == "") {
			jerr(w, 400, "isi tepat satu: every (mis. 5m) atau at (HH:MM)")
			return
		}
		if in.Every != "" {
			if !everyRE.MatchString(in.Every) || strings.HasPrefix(in.Every, "0") {
				jerr(w, 400, "every: 1-999 diikuti m atau h")
				return
			}
			args = append(args, "--every", in.Every)
		} else {
			if !atRE.MatchString(in.At) {
				jerr(w, 400, "at harus HH:MM")
				return
			}
			args = append(args, "--at", in.At)
		}
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), k+"_add", in.Name, a.mutate(args, ""))
}

func taskName(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	k, ok := taskKind(w, r)
	if !ok {
		return "", "", false
	}
	n := r.PathValue("name")
	if !taskNameRE.MatchString(n) {
		jerr(w, 400, "nama tidak valid")
		return "", "", false
	}
	return k, n, true
}

func hTaskRestart(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	k, n, ok := taskName(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	a.record(w, ss, a.clientIP(r), k+"_restart", n, a.mutate([]string{k, "restart", n}, ""))
}

func hTaskRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	k, n, ok := taskName(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if in.Confirm != n {
		jerr(w, 400, "ketik ulang nama untuk konfirmasi")
		return
	}
	a.record(w, ss, a.clientIP(r), k+"_remove", n, a.mutate([]string{k, "remove", n}, ""))
}

func hTaskLogs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	k, n, ok := taskName(w, r)
	if !ok {
		return
	}
	lines := 40
	if q := r.URL.Query().Get("n"); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || !logN.MatchString(q) || v < 1 || v > 500 {
			jerr(w, 400, "n 1-500")
			return
		}
		lines = v
	}
	res := a.runLab([]string{k, "logs", n, "-n", strconv.Itoa(lines)}, "")
	if res.Code != 0 {
		jerr(w, 404, "log tidak tersedia")
		return
	}
	jsonOut(w, 200, map[string]any{"name": n, "log": res.Out})
}

// ---------- Update ----------

var formulaRE = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]{0,59}$`) // tidak boleh diawali '-' (injeksi opsi)

func hUpdatesCheck(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	res := a.runLabT([]string{"update", "check"}, "", a.timeout)
	if res.Code != 0 {
		jerr(w, 502, "gagal memeriksa pembaruan")
		return
	}
	rows, pins := parseUpdates(res.Out)
	jsonOut(w, 200, map[string]any{"updates": rows, "pins": pins, "checked": a.now().Unix()})
}

func hUpdatesApply(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct {
		Target, Confirm string
		AllowPinned     bool `json:"allow_pinned"`
	}
	if !decode(w, r, &in) {
		return
	}
	t := in.Target
	if strings.HasPrefix(t, "mise:") {
		if !runtimeTools[strings.TrimPrefix(t, "mise:")] {
			jerr(w, 400, "tool mise tidak dikenal")
			return
		}
	} else if !formulaRE.MatchString(t) {
		jerr(w, 400, "nama formula tidak valid")
		return
	}
	if in.Confirm != t {
		jerr(w, 400, "ketik ulang nama target untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	args := []string{"update", "apply", t}
	if in.AllowPinned {
		args = append(args, "--allow-pinned")
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "update_apply", t, args, ""))
}

var _ = time.Second
