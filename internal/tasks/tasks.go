package tasks

import (
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type TaskRow struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	PID      string `json:"pid"`
	Exit     string `json:"exit"`
	Schedule string `json:"schedule"`
	Cmd      string `json:"cmd"`
}

func ParseTasks(out string) []TaskRow {
	res := []TaskRow{}
	for _, l := range strings.Split(out, "\n") {
		head, cmd, _ := strings.Cut(l, "\t") // jadwal dan perintah dipisah tab; format lama tanpa tab tetap terbaca
		f := strings.Fields(head)
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
		res = append(res, TaskRow{Name: name, Status: status, PID: rest[0], Exit: rest[1], Schedule: strings.Join(rest[2:], " "), Cmd: strings.TrimSpace(cmd)})
	}
	return res
}

var taskNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

var cmdCharsRE = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,~ -]{1,300}$`)

var everyRE = regexp.MustCompile(`^[0-9]{1,3}[mh]$`)

var atRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

var cmdPrograms = map[string]map[string]bool{ // program yang boleh -> subperintah pertama yang boleh (nil = bebas)
	"php": nil, "node": nil, "python3": nil, "python": nil,
	"npm": {"run": true, "start": true}, "pnpm": {"run": true, "start": true}, "composer": {"run-script": true, "run": true},
}

var cmdBannedFlags = map[string]bool{
	"-r": true, "-R": true, "-B": true, "-E": true, "-F": true, "-f": true, "-d": true, "-S": true, "-a": true, "-i": true, // php
	"-e": true, "--eval": true, "-p": true, "--print": true, "--require": true, "-c": true, "--interactive": true, // node, python
}

// validCmd memeriksa perintah worker/cron. Worker menjalankan kode proyek seperti Supervisor; yang dicegah adalah
// kode sembarang lewat flag (-r, -e, -c), program asing, jalur di luar ~/MacPanel/wwwroot, dan port debug.
func ValidCmd(a *core.App, cmd string) error {
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
			if _, err := a.UnderDev(t); err != nil {
				return errors.New("path " + t + ": " + err.Error())
			}
		}
	}
	return nil
}

var taskTypes = map[string]bool{"cmd": true, "php": true, "node": true, "python": true, "shell": true, "url": true}

var typePrograms = map[string]map[string]bool{
	"php": {"php": true, "composer": true}, "node": {"node": true, "npm": true, "pnpm": true}, "python": {"python3": true, "python": true},
}

var urlRE = regexp.MustCompile(`^https?://[A-Za-z0-9._~:/?&=%+,@#-]{1,300}$`)

var cronRE = regexp.MustCompile(`^[0-9A-Za-z*/,@ -]{1,100}$`)

const maxScript = 20000

// ValidScript: skrip shell bebas (pilihan pemilik panel, di balik kata sandi ulang dan tercatat), hanya dibatasi ukuran dan NUL.
func ValidScript(s string) error {
	switch {
	case strings.TrimSpace(s) == "":
		return errors.New("skrip kosong")
	case len(s) > maxScript:
		return errors.New("skrip maksimal 20000 byte")
	case strings.ContainsRune(s, 0):
		return errors.New("skrip tidak boleh berisi karakter NUL")
	}
	return nil
}

func sortedKeys(m map[string]bool) []string {
	k := make([]string, 0, len(m))
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

func taskKind(w http.ResponseWriter, r *http.Request) (string, bool) {
	k := r.PathValue("kind")
	if k != "worker" && k != "job" {
		core.WriteError(w, 400, "jenis harus worker atau job")
		return "", false
	}
	return k, true
}

func TasksList(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	k, ok := taskKind(w, r)
	if !ok {
		return
	}
	res := a.RunLab([]string{k, "list"}, "")
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca daftar")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"kind": k, "tasks": ParseTasks(res.Out)})
}

func TaskAdd(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	k, ok := taskKind(w, r)
	if !ok {
		return
	}
	var in struct {
		Name, Dir, Cmd, PHP, Every, At, Cron, Type, Script, URL string
		Procs                                                   int
	}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !taskNameRE.MatchString(in.Name) {
		core.WriteError(w, 400, "nama: huruf kecil/angka/-, diawali huruf, maks 40")
		return
	}
	if in.Type == "" {
		in.Type = "cmd"
	}
	if !taskTypes[in.Type] {
		core.WriteError(w, 400, "type: cmd, php, node, python, shell, atau url")
		return
	}
	dir := ""
	if in.Dir != "" || in.Type != "url" {
		d, err := a.UnderDev(in.Dir)
		if err != nil {
			core.WriteError(w, 400, err.Error())
			return
		}
		if isDir, _ := core.StatDir(d); !isDir {
			core.WriteError(w, 400, "dir bukan folder")
			return
		}
		dir = d
	}
	args := []string{k, "add", in.Name, "--type", in.Type}
	if dir != "" {
		args = append(args, "--dir", dir)
	}
	stdin := ""
	switch in.Type {
	case "shell":
		if err := ValidScript(in.Script); err != nil {
			core.WriteError(w, 400, err.Error())
			return
		}
		args = append(args, "--script-stdin")
		stdin = in.Script
	case "url":
		if !urlRE.MatchString(in.URL) {
			core.WriteError(w, 400, "url harus http:// atau https:// tanpa spasi atau kutip, maks 300 karakter")
			return
		}
		args = append(args, "--url", in.URL)
	default:
		if err := ValidCmd(a, in.Cmd); err != nil {
			core.WriteError(w, 400, err.Error())
			return
		}
		if want := typePrograms[in.Type]; want != nil && !want[strings.Fields(in.Cmd)[0]] {
			core.WriteError(w, 400, "tipe "+in.Type+": program pertama harus "+strings.Join(sortedKeys(want), " atau "))
			return
		}
		args = append(args, "--cmd", in.Cmd)
	}
	if in.Type != "shell" && in.Script != "" || in.Type != "url" && in.URL != "" || (in.Type == "shell" || in.Type == "url") && in.Cmd != "" {
		core.WriteError(w, 400, "script hanya untuk tipe shell, url hanya untuk tipe url, cmd tidak dipakai keduanya")
		return
	}
	if in.PHP != "" {
		if !core.PhpVersions[in.PHP] {
			core.WriteError(w, 400, "versi PHP harus 8.2-8.5")
			return
		}
		args = append(args, "--php", in.PHP)
	}
	if k == "worker" {
		if in.Every != "" || in.At != "" || in.Cron != "" {
			core.WriteError(w, 400, "worker tidak memakai jadwal")
			return
		}
		if in.Procs < 0 || in.Procs > 8 {
			core.WriteError(w, 400, "procs 1-8")
			return
		}
		if in.Procs > 1 {
			args = append(args, "--procs", strconv.Itoa(in.Procs))
		}
	} else {
		if in.Procs != 0 {
			core.WriteError(w, 400, "job tidak memakai procs")
			return
		}
		n := 0
		for _, v := range []string{in.Every, in.At, in.Cron} {
			if v != "" {
				n++
			}
		}
		if n != 1 {
			core.WriteError(w, 400, "isi tepat satu: every (mis. 5m), at (HH:MM), atau cron (5 kolom)")
			return
		}
		if in.Cron != "" {
			if !cronRE.MatchString(in.Cron) {
				core.WriteError(w, 400, "cron hanya boleh angka, huruf, * / , - dan spasi (maks 100 karakter)")
				return
			}
			args = append(args, "--cron", in.Cron)
		} else if in.Every != "" {
			if !everyRE.MatchString(in.Every) || strings.HasPrefix(in.Every, "0") {
				core.WriteError(w, 400, "every: 1-999 diikuti m atau h")
				return
			}
			args = append(args, "--every", in.Every)
		} else {
			if !atRE.MatchString(in.At) {
				core.WriteError(w, 400, "at harus HH:MM")
				return
			}
			args = append(args, "--at", in.At)
		}
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), k+"_add", in.Name, a.Mutate(args, stdin))
}

func taskName(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	k, ok := taskKind(w, r)
	if !ok {
		return "", "", false
	}
	n := r.PathValue("name")
	if !taskNameRE.MatchString(n) {
		core.WriteError(w, 400, "nama tidak valid")
		return "", "", false
	}
	return k, n, true
}

func TaskRestart(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	k, n, ok := taskName(w, r)
	if !ok {
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), k+"_restart", n, a.Mutate([]string{k, "restart", n}, ""))
}

func TaskRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	k, n, ok := taskName(w, r)
	if !ok {
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if in.Confirm != n {
		core.WriteError(w, 400, "ketik ulang nama untuk konfirmasi")
		return
	}
	a.Record(w, ss, a.ClientIP(r), k+"_remove", n, a.Mutate([]string{k, "remove", n}, ""))
}

func TaskLogs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	k, n, ok := taskName(w, r)
	if !ok {
		return
	}
	lines := 40
	if q := r.URL.Query().Get("n"); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || !core.LogN.MatchString(q) || v < 1 || v > 500 {
			core.WriteError(w, 400, "n 1-500")
			return
		}
		lines = v
	}
	res := a.RunLab([]string{k, "logs", n, "-n", strconv.Itoa(lines)}, "")
	if res.Code != 0 {
		core.WriteError(w, 404, "log tidak tersedia")
		return
	}
	core.WriteJSON(w, 200, map[string]any{"name": n, "log": res.Out})
}
