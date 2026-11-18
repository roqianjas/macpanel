package main

import (
	"crypto/md5"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---------- htpasswd (apr1) ----------

const b64 = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func to64(v uint32, n int) string {
	s := make([]byte, n)
	for i := 0; i < n; i++ {
		s[i] = b64[v&0x3f]
		v >>= 6
	}
	return string(s)
}

// apr1 menghitung hash $apr1$ (Apache MD5) yang dipahami nginx auth_basic.
func apr1(pw, salt string) string {
	p, sl := []byte(pw), []byte(salt)
	alt := md5.Sum(append(append(append([]byte{}, p...), sl...), p...))
	c := md5.New()
	c.Write(p)
	c.Write([]byte("$apr1$"))
	c.Write(sl)
	for n := len(p); n > 0; n -= 16 {
		k := n
		if k > 16 {
			k = 16
		}
		c.Write(alt[:k])
	}
	for i := len(p); i > 0; i >>= 1 {
		if i&1 == 1 {
			c.Write([]byte{0})
		} else if len(p) > 0 {
			c.Write(p[:1])
		}
	}
	f := c.Sum(nil)
	for i := 0; i < 1000; i++ {
		d := md5.New()
		if i&1 == 1 {
			d.Write(p)
		} else {
			d.Write(f)
		}
		if i%3 != 0 {
			d.Write(sl)
		}
		if i%7 != 0 {
			d.Write(p)
		}
		if i&1 == 1 {
			d.Write(f)
		} else {
			d.Write(p)
		}
		f = d.Sum(nil)
	}
	w := func(a, b, c int) uint32 { return uint32(f[a])<<16 | uint32(f[b])<<8 | uint32(f[c]) }
	return "$apr1$" + salt + "$" + to64(w(0, 6, 12), 4) + to64(w(1, 7, 13), 4) + to64(w(2, 8, 14), 4) + to64(w(3, 9, 15), 4) + to64(w(4, 10, 5), 4) + to64(uint32(f[11]), 2)
}

func newSalt() string {
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = b64[int(b[i])%len(b64)]
	}
	return string(b)
}

var authUserRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

func (a *App) authFile(name string) string {
	return filepath.Join(a.nginxDir, "auth", name+".htpasswd")
}

func (a *App) authUsers(name string) []string {
	b, err := os.ReadFile(a.authFile(name))
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, l := range strings.Split(string(b), "\n") {
		if u, _, ok := strings.Cut(l, ":"); ok && u != "" {
			out = append(out, u)
		}
	}
	sort.Strings(out)
	return out
}

func (a *App) writeAuth(name string, users map[string]string) error {
	if err := os.MkdirAll(filepath.Join(a.nginxDir, "auth"), 0o700); err != nil {
		return err
	}
	keys := make([]string, 0, len(users))
	for k := range users {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k + ":" + users[k] + "\n")
	}
	tmp := a.authFile(name) + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.authFile(name))
}

func (a *App) readAuthMap(name string) map[string]string {
	m := map[string]string{}
	b, _ := os.ReadFile(a.authFile(name))
	for _, l := range strings.Split(string(b), "\n") {
		if u, h, ok := strings.Cut(l, ":"); ok && u != "" {
			m[u] = h
		}
	}
	return m
}

func hAuthGet(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	on := false
	if st, err := a.readSite(name); err == nil && st.Model != nil {
		on = st.Model.Auth
	}
	jsonOut(w, 200, map[string]any{"enabled": on, "users": a.authUsers(name)})
}

func hAuthSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ User, Password string }
	if !decode(w, r, &in) {
		return
	}
	if !authUserRE.MatchString(in.User) {
		jerr(w, 400, "nama pengguna: huruf, angka, titik, garis, maks 32")
		return
	}
	if len(in.Password) < 8 || len(in.Password) > 128 || strings.ContainsAny(in.Password, "\x00\r\n") {
		jerr(w, 400, "kata sandi 8-128 karakter tanpa baris baru")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	m := a.readAuthMap(name)
	if _, exists := m[in.User]; !exists && len(m) >= 20 {
		jerr(w, 400, "maksimal 20 pengguna")
		return
	}
	m[in.User] = apr1(in.Password, newSalt())
	if err := a.writeAuth(name, m); err != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "site_auth_user", name+" "+in.User, a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"users": a.authUsers(name)})
}

func hAuthDel(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	user := r.PathValue("user")
	var in struct{}
	if !authUserRE.MatchString(user) || !decode(w, r, &in) {
		if !authUserRE.MatchString(user) {
			jerr(w, 400, "nama pengguna tidak valid")
		}
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	m := a.readAuthMap(name)
	if _, exists := m[user]; !exists {
		jerr(w, 404, "pengguna tidak ada")
		return
	}
	if st, err := a.readSite(name); err == nil && st.Model != nil && st.Model.Auth && len(m) == 1 {
		jerr(w, 400, "ini pengguna terakhir; matikan password dasar dulu")
		return
	}
	delete(m, user)
	if err := a.writeAuth(name, m); err != nil {
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "site_auth_user_del", name+" "+user, a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"users": a.authUsers(name)})
}

// ---------- batas IP (hanya boleh menyempit dari tailnet) ----------

var tailnetNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{"127.0.0.0/8", "::1/128", "100.64.0.0/10", "fd7a:115c:a1e0::/48"} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}()

// inTailnet benar bila IP atau CIDR berada seluruhnya di dalam rentang yang sudah diizinkan tailnet_only.
func inTailnet(s string) bool {
	var ip net.IP
	var bits int
	if strings.Contains(s, "/") {
		i, n, err := net.ParseCIDR(s)
		if err != nil || !i.Equal(n.IP) {
			return false
		}
		ip = n.IP
		bits, _ = n.Mask.Size()
	} else {
		ip = net.ParseIP(s)
		if ip == nil {
			return false
		}
		bits = 8 * len(ip)
		if v4 := ip.To4(); v4 != nil {
			ip, bits = v4, 32
		}
	}
	if ip.To4() != nil {
		bits0 := bits
		for _, n := range tailnetNets {
			ones, total := n.Mask.Size()
			if total == 32 && n.Contains(ip) && bits0 >= ones {
				return true
			}
		}
		return false
	}
	for _, n := range tailnetNets {
		ones, total := n.Mask.Size()
		if total == 128 && n.Contains(ip) && bits >= ones {
			return true
		}
	}
	return false
}

var aliasLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

func (a *App) aliasOK(alias string) bool {
	l, tld, ok := strings.Cut(alias, ".")
	return ok && tld == a.tld && aliasLabel.MatchString(l) && !protectedSites[l]
}

var serverNameRE = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)

// aliasConflict: nama sudah dipakai situs lain.
func (a *App) aliasConflict(name, alias string) bool {
	files, _ := filepath.Glob(filepath.Join(a.sitesDir, "*.conf*"))
	for _, f := range files {
		base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(f), ".disabled"), ".conf")
		if base == name {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range serverNameRE.FindAllStringSubmatch(string(b), -1) {
			for _, t := range strings.Fields(m[1]) {
				if t == alias {
					return true
				}
			}
		}
	}
	return false
}

func (a *App) validateExtras(m *Model) error {
	if len(m.Aliases) > 10 {
		return errors.New("maksimal 10 domain tambahan")
	}
	seen := map[string]bool{}
	for _, al := range m.Aliases {
		if !a.aliasOK(al) {
			return fmt.Errorf("domain tambahan %q harus berbentuk nama.%s (huruf kecil, angka, tanda hubung)", al, a.tld)
		}
		if seen[al] {
			return fmt.Errorf("domain tambahan %q ganda", al)
		}
		seen[al] = true
	}
	if len(m.AllowIPs) > 20 {
		return errors.New("maksimal 20 aturan IP")
	}
	for _, ip := range m.AllowIPs {
		if !inTailnet(ip) {
			return fmt.Errorf("IP %q harus berada di dalam jaringan tailnet/loopback (100.64.0.0/10, fd7a:115c:a1e0::/48, 127.0.0.0/8); aturan ini hanya boleh mempersempit akses", ip)
		}
	}
	return nil
}

// ---------- .env proyek ----------

const maxEnv = 64 << 10

func (a *App) envPath(name string) (string, error) {
	st, err := a.readSite(name)
	if err != nil {
		return "", errors.New("situs tidak ada")
	}
	if st.Model == nil || (st.Model.Mode != "php" && st.Model.Mode != "statis") {
		return "", errors.New("situs ini tidak punya folder proyek yang dikenali")
	}
	root, err := a.underDev(st.Model.Target)
	if err != nil {
		return "", err
	}
	dir := root
	if filepath.Base(root) == "public" {
		dir = filepath.Dir(root)
	}
	if dir, err = a.underDev(dir); err != nil {
		return "", errors.New("folder proyek harus di bawah ~/MacPanel/wwwroot")
	}
	return filepath.Join(dir, ".env"), nil
}

func hEnvGet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, false)
	if !ok {
		return
	}
	if ss.Role == "viewer" {
		jerr(w, 403, "hanya admin")
		return
	}
	p, err := a.envPath(name)
	if err != nil {
		jerr(w, 400, err.Error())
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	fi, err := os.Lstat(p)
	if os.IsNotExist(err) {
		jsonOut(w, 200, map[string]any{"exists": false, "content": "", "path": filepath.Base(filepath.Dir(p)) + "/.env"})
		return
	}
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxEnv {
		jerr(w, 400, ".env bukan berkas biasa atau lebih dari 64 KB")
		return
	}
	b, _ := os.ReadFile(p)
	a.st.Audit(ss.User, "env_read", name, a.clientIP(r), a.now())
	jsonOut(w, 200, map[string]any{"exists": true, "content": string(b), "path": filepath.Base(filepath.Dir(p)) + "/.env"})
}

func hEnvSet(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	name, ok := a.siteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ Content string }
	if !decodeN(w, r, &in, 2*maxEnv+1024) {
		return
	}
	if len(in.Content) > maxEnv || strings.ContainsRune(in.Content, 0) {
		jerr(w, 400, "isi .env maksimal 64 KB tanpa karakter NUL")
		return
	}
	p, err := a.envPath(name)
	if err != nil {
		jerr(w, 400, err.Error())
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if fi, err := os.Lstat(p); err == nil {
		if !fi.Mode().IsRegular() {
			jerr(w, 400, ".env bukan berkas biasa")
			return
		}
		old, _ := os.ReadFile(p)
		bak := p + ".bak." + a.now().Format("20060102-150405")
		if err := os.WriteFile(bak, old, 0o600); err != nil {
			jerr(w, 500, "gagal membuat cadangan")
			return
		}
		baks, _ := filepath.Glob(p + ".bak.*")
		sort.Strings(baks)
		for len(baks) > 3 {
			_ = os.Remove(baks[0])
			baks = baks[1:]
		}
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(in.Content), 0o600); err != nil {
		jerr(w, 500, "gagal menulis")
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		jerr(w, 500, "gagal menyimpan")
		return
	}
	a.st.Audit(ss.User, "env_write", name+" ("+fmt.Sprint(len(in.Content))+" byte)", a.clientIP(r), a.now())
	_ = time.Now
	jsonOut(w, 200, map[string]any{"saved": true})
}
