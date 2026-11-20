package sites

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

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

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

func AuthFile(a *core.App, name string) string {
	return filepath.Join(a.NginxDir, "auth", name+".htpasswd")
}

func authUsers(a *core.App, name string) []string {
	b, err := os.ReadFile(AuthFile(a, name))
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

func writeAuth(a *core.App, name string, users map[string]string) error {
	if err := os.MkdirAll(filepath.Join(a.NginxDir, "auth"), 0o700); err != nil {
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
	tmp := AuthFile(a, name) + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, AuthFile(a, name))
}

func readAuthMap(a *core.App, name string) map[string]string {
	m := map[string]string{}
	b, _ := os.ReadFile(AuthFile(a, name))
	for _, l := range strings.Split(string(b), "\n") {
		if u, h, ok := strings.Cut(l, ":"); ok && u != "" {
			m[u] = h
		}
	}
	return m
}

func AuthGet(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	name, ok := a.SiteParam(w, r, false)
	if !ok {
		return
	}
	on := false
	if st, err := readSite(a, name); err == nil && st.Model != nil {
		on = st.Model.Auth
	}
	core.WriteJSON(w, 200, map[string]any{"enabled": on, "users": authUsers(a, name)})
}

func AuthSet(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	var in struct{ User, Password string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !authUserRE.MatchString(in.User) {
		core.WriteError(w, 400, "nama pengguna: huruf, angka, titik, garis, maks 32")
		return
	}
	if len(in.Password) < 8 || len(in.Password) > 128 || strings.ContainsAny(in.Password, "\x00\r\n") {
		core.WriteError(w, 400, "kata sandi 8-128 karakter tanpa baris baru")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Mu.Lock()
	defer a.Mu.Unlock()
	m := readAuthMap(a, name)
	if _, exists := m[in.User]; !exists && len(m) >= 20 {
		core.WriteError(w, 400, "maksimal 20 pengguna")
		return
	}
	m[in.User] = apr1(in.Password, newSalt())
	if err := writeAuth(a, name, m); err != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "site_auth_user", name+" "+in.User, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"users": authUsers(a, name)})
}

func AuthDel(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	name, ok := a.SiteParam(w, r, true)
	if !ok {
		return
	}
	user := r.PathValue("user")
	var in struct{}
	if !authUserRE.MatchString(user) || !core.DecodeJSON(w, r, &in) {
		if !authUserRE.MatchString(user) {
			core.WriteError(w, 400, "nama pengguna tidak valid")
		}
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Mu.Lock()
	defer a.Mu.Unlock()
	m := readAuthMap(a, name)
	if _, exists := m[user]; !exists {
		core.WriteError(w, 404, "pengguna tidak ada")
		return
	}
	if st, err := readSite(a, name); err == nil && st.Model != nil && st.Model.Auth && len(m) == 1 {
		core.WriteError(w, 400, "ini pengguna terakhir; matikan password dasar dulu")
		return
	}
	delete(m, user)
	if err := writeAuth(a, name, m); err != nil {
		core.WriteError(w, 500, "gagal menyimpan")
		return
	}
	a.Store.Audit(ss.User, "site_auth_user_del", name+" "+user, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"users": authUsers(a, name)})
}

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

func aliasOK(a *core.App, alias string) bool {
	l, tld, ok := strings.Cut(alias, ".")
	return ok && tld == a.TLD && aliasLabel.MatchString(l) && !core.ProtectedSites[l]
}

var hostLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// publicHostOK: nama host umum (mis. mesin.tailnet.ts.net atau contoh.com). Tanpa wildcard, tanpa IP, tanpa TLD lokal
// (.lab milik aliasOK), minimal dua label. Nama ini tidak mendapat sertifikat lokal.
func publicHostOK(a *core.App, h string) bool {
	if len(h) > 253 || strings.HasSuffix(h, "."+a.TLD) || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".localhost") {
		return false
	}
	ls := strings.Split(h, ".")
	if len(ls) < 2 {
		return false
	}
	for _, l := range ls {
		if !hostLabel.MatchString(l) {
			return false
		}
	}
	if net.ParseIP(h) != nil || regexp.MustCompile(`^[0-9.]+$`).MatchString(h) {
		return false
	}
	return true
}

// aliasValid: nama .lab atau nama host umum.
func aliasValid(a *core.App, al string) bool { return aliasOK(a, al) || publicHostOK(a, al) }

// localAliases: hanya nama .lab, satu-satunya yang boleh masuk sertifikat mkcert.
func localAliases(a *core.App, als []string) []string {
	var out []string
	for _, al := range als {
		if aliasOK(a, al) {
			out = append(out, al)
		}
	}
	return out
}

func hasPublicAlias(a *core.App, als []string) bool {
	for _, al := range als {
		if !aliasOK(a, al) {
			return true
		}
	}
	return false
}

var serverNameRE = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)

// aliasConflict: nama sudah dipakai situs lain.
func aliasConflict(a *core.App, name, alias string) bool {
	files, _ := filepath.Glob(filepath.Join(a.SitesDir, "*.conf*"))
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

func validateExtras(a *core.App, m *Model) error {
	if len(m.Aliases) > 10 {
		return errors.New("maksimal 10 domain tambahan")
	}
	seen := map[string]bool{}
	for _, al := range m.Aliases {
		if !aliasValid(a, al) {
			return fmt.Errorf("domain tambahan %q harus berbentuk nama.%s atau nama host umum (huruf kecil, angka, tanda hubung, tanpa wildcard/IP)", al, a.TLD)
		}
		if seen[al] {
			return fmt.Errorf("domain tambahan %q ganda", al)
		}
		seen[al] = true
	}
	if hasPublicAlias(a, m.Aliases) && !m.PublicAck {
		return errors.New("domain di luar ." + a.TLD + " bisa membuat situs terjangkau dari internet (mis. lewat Tailscale Funnel); centang konfirmasi paparan publik")
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
