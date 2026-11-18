package core

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/roqianjas/macpanel/internal/store"
)

const MaxOutput = 256 << 10

type limitedBuf struct {
	b []byte
}

func (l *limitedBuf) Write(p []byte) (int, error) {
	if room := MaxOutput - len(l.b); room > 0 {
		if len(p) > room {
			l.b = append(l.b, p[:room]...)
		} else {
			l.b = append(l.b, p...)
		}
	}
	return len(p), nil // selalu "sukses" agar proses tidak macet; sisa output dibuang
}

type Result struct {
	Code     int
	Out      string
	TimedOut bool
}

var scrubRE = regexp.MustCompile(`(?i)((?:pass(?:word)?|token|secret|key)\s*[=:]\s*)\S+|(Bearer\s+)\S+|[A-Za-z0-9_\-]{40,}`)

func Scrub(s string) string {
	return scrubRE.ReplaceAllStringFunc(s, func(m string) string {
		if i := strings.IndexAny(m, "=:"); i >= 0 {
			return m[:i+1] + " [disamarkan]"
		}
		if strings.HasPrefix(strings.ToLower(m), "bearer") {
			return "Bearer [disamarkan]"
		}
		return "[disamarkan]"
	})
}

// runLab satu-satunya cara panel menyentuh sistem: biner `lab` (path tetap), argv berupa list (tanpa shell),
// env dibersihkan, timeout, batas output, seluruh grup proses dimatikan saat timeout.
func (a *App) RunLab(args []string, stdin string) Result {
	return a.RunLabTimeout(args, stdin, a.Timeout)
}

func (a *App) RunLabTimeout(args []string, stdin string, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.LabBin, args...)
	cmd.Env = []string{"PATH=/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + a.Home, "MACPANEL_HOME=" + a.Root, "LANG=en_US.UTF-8"}
	cmd.Dir = a.Home
	cmd.Stdin = strings.NewReader(stdin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	var buf limitedBuf
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	res := Result{Out: Scrub(string(buf.b))}
	var ee *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.TimedOut, res.Code = true, -1
	case errors.As(err, &ee):
		res.Code = ee.ExitCode()
	case err != nil:
		res.Code, res.Out = -1, "gagal menjalankan: "+err.Error()
	}
	return res
}

var SiteRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

var DbRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

var NumRE = regexp.MustCompile(`^[0-9]{1,5}$`)

var ProtectedSites = map[string]bool{"home": true, "nginx": true, "db": true, "kuma": true, "stats": true, "hello": true, "panel": true}

var BlockedPorts = map[int]bool{3306: true, 3307: true, 5432: true, 6379: true, 9100: true, 22: true}

var PhpVersions = map[string]bool{"8.2": true, "8.3": true, "8.4": true, "8.5": true}

// UnderDev mengembalikan path nyata (symlink diikuti) bila berada di bawah ~/MacPanel/wwwroot.
func (a *App) UnderDev(p string) (string, error) {
	if p == "" || strings.ContainsAny(p, "\x00\n\r") {
		return "", errors.New("path tidak valid")
	}
	if strings.HasPrefix(p, "~/") {
		p = filepath.Join(a.Home, p[2:])
	}
	if !filepath.IsAbs(p) {
		return "", errors.New("path harus absolut atau diawali ~/")
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(p))
	if err != nil {
		return "", errors.New("folder tidak ada")
	}
	root, err := filepath.EvalSymlinks(a.Wwwroot)
	if err != nil || !strings.HasPrefix(real+"/", root+"/") || real == root {
		return "", errors.New("folder harus di bawah ~/MacPanel/wwwroot")
	}
	return real, nil
}

func (a *App) ValidTarget(t string) (string, error) {
	if NumRE.MatchString(t) {
		n, _ := strconv.Atoi(t)
		if n < 1024 || n > 65535 || BlockedPorts[n] {
			return "", errors.New("port tidak diizinkan (1024-65535, bukan port layanan inti)")
		}
		return t, nil
	}
	real, err := a.UnderDev(t)
	if err != nil {
		return "", err
	}
	if fi, err := StatDir(real); err != nil || !fi {
		return "", errors.New("target bukan folder")
	}
	return real, nil
}

func (a *App) ValidEnvFile(p string) (string, error) {
	real, err := a.UnderDev(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	b := filepath.Base(p)
	if b != ".env" && !strings.HasPrefix(b, ".env.") || strings.ContainsAny(b, "/\x00") {
		return "", errors.New("file harus bernama .env atau .env.*")
	}
	return filepath.Join(real, b), nil
}

// Record menyimpan hasil aksi ke tabel job dan audit, lalu menjawab klien.
func (a *App) Record(w http.ResponseWriter, ss *store.Session, ip, action, detail string, res Result) {
	status := "ok"
	switch {
	case res.TimedOut:
		status = "timeout"
	case res.Code != 0:
		status = "gagal"
	}
	id := a.Store.AddJob(ss.User, action+" "+detail, status, res.Out, res.Code, a.Now())
	a.Store.Audit(ss.User, action, detail+" -> "+status, ip, a.Now())
	code := 200
	if status != "ok" {
		code = 422
	}
	WriteJSON(w, code, map[string]any{"job": id, "status": status, "output": res.Out})
}

func (a *App) Mutate(args []string, stdin string) Result {
	a.Mu.Lock() // satu aksi mutasi pada satu waktu
	defer a.Mu.Unlock()
	return a.RunLab(args, stdin)
}

func StatusOf(res Result) string {
	switch {
	case res.TimedOut:
		return "timeout"
	case res.Code != 0:
		return "gagal"
	}
	return "ok"
}

// StartAsync menjalankan aksi lama (install, update) di latar belakang; klien memantau lewat GET /api/jobs/{id}.
func (a *App) StartAsync(ss *store.Session, ip, action, detail string, args []string, stdin string) int64 {
	id := a.Store.AddJob(ss.User, action+" "+detail, "berjalan", "", 0, a.Now())
	a.Store.Audit(ss.User, action, detail+" -> mulai", ip, a.Now())
	user := ss.User
	a.BG.Add(1)
	go func() {
		defer a.BG.Done()
		a.Mu.Lock()
		res := a.RunLabTimeout(args, stdin, a.LongTimeout)
		a.Mu.Unlock()
		st := StatusOf(res)
		a.Store.FinishJob(id, st, res.Code, res.Out)
		a.Store.Audit(user, action, detail+" -> "+st, ip, a.Now())
	}()
	return id
}

func RespondAsync(w http.ResponseWriter, id int64) {
	WriteJSON(w, 202, map[string]any{"job": id, "status": "berjalan"})
}

// LabSecret menjalankan lab dengan kata sandi lewat stdin dan menjaga agar tidak pernah muncul di galat.
func (a *App) LabSecret(w http.ResponseWriter, args []string, pw string) (map[string]any, bool) {
	stdin := ""
	if pw != "" {
		args = append(args, "--password-stdin")
		stdin = pw + "\n"
	}
	res := a.Mutate(args, stdin)
	var out map[string]any
	if res.Code != 0 || !ParseLabJSON(res.Out, &out) {
		msg := strings.TrimSpace(Scrub(res.Out))
		if pw != "" {
			msg = strings.ReplaceAll(msg, pw, "***")
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		WriteError(w, 422, msg)
		return nil, false
	}
	return out, true
}

func LabJSON(a *App, w http.ResponseWriter, args []string, timeout time.Duration, msg string) {
	res := a.RunLabTimeout(args, "", timeout)
	var raw map[string]any
	if res.Code != 0 || !ParseLabJSON(res.Out, &raw) {
		WriteError(w, 502, msg)
		return
	}
	WriteJSON(w, 200, raw)
}
