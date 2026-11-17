package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const maxOutput = 256 << 10

type limitedBuf struct {
	b []byte
}

func (l *limitedBuf) Write(p []byte) (int, error) {
	if room := maxOutput - len(l.b); room > 0 {
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

func scrub(s string) string {
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
func (a *App) runLab(args []string, stdin string) Result { return a.runLabT(args, stdin, a.timeout) }

func (a *App) runLabT(args []string, stdin string, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.labBin, args...)
	cmd.Env = []string{"PATH=/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + a.home, "LANG=en_US.UTF-8"}
	cmd.Dir = a.home
	cmd.Stdin = strings.NewReader(stdin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	var buf limitedBuf
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	res := Result{Out: scrub(string(buf.b))}
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

// ---------- validator (sama ketatnya dengan `lab`) ----------

var (
	siteRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)
	dbRE   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	numRE  = regexp.MustCompile(`^[0-9]{1,5}$`)
)

var protectedSites = map[string]bool{"home": true, "nginx": true, "db": true, "kuma": true, "stats": true, "hello": true, "panel": true}
var blockedPorts = map[int]bool{3306: true, 5432: true, 6379: true, 9100: true, 22: true}
var phpVersions = map[string]bool{"8.2": true, "8.3": true, "8.4": true, "8.5": true}

// underDev mengembalikan path nyata (symlink diikuti) bila berada di bawah ~/MacPanel/wwwroot.
func (a *App) underDev(p string) (string, error) {
	if p == "" || strings.ContainsAny(p, "\x00\n\r") {
		return "", errors.New("path tidak valid")
	}
	if strings.HasPrefix(p, "~/") {
		p = filepath.Join(a.home, p[2:])
	}
	if !filepath.IsAbs(p) {
		return "", errors.New("path harus absolut atau diawali ~/")
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(p))
	if err != nil {
		return "", errors.New("folder tidak ada")
	}
	root, err := filepath.EvalSymlinks(a.wwwroot)
	if err != nil || !strings.HasPrefix(real+"/", root+"/") || real == root {
		return "", errors.New("folder harus di bawah ~/MacPanel/wwwroot")
	}
	return real, nil
}

func (a *App) validTarget(t string) (string, error) {
	if numRE.MatchString(t) {
		n, _ := strconv.Atoi(t)
		if n < 1024 || n > 65535 || blockedPorts[n] {
			return "", errors.New("port tidak diizinkan (1024-65535, bukan port layanan inti)")
		}
		return t, nil
	}
	real, err := a.underDev(t)
	if err != nil {
		return "", err
	}
	if fi, err := statDir(real); err != nil || !fi {
		return "", errors.New("target bukan folder")
	}
	return real, nil
}

func (a *App) validEnvFile(p string) (string, error) {
	real, err := a.underDev(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	b := filepath.Base(p)
	if b != ".env" && !strings.HasPrefix(b, ".env.") || strings.ContainsAny(b, "/\x00") {
		return "", errors.New("file harus bernama .env atau .env.*")
	}
	return filepath.Join(real, b), nil
}
