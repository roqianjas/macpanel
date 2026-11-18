package core

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/roqianjas/macpanel/internal/store"
)

// tokenAllowed: rute yang tidak pernah boleh dipanggil dengan token.
func TokenAllowed(method, path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/auth/"), strings.HasPrefix(path, "/api/me/"):
		return false
	case strings.HasSuffix(path, "/creds"), strings.HasSuffix(path, "/env"):
		return false
	case strings.HasPrefix(path, "/api/accounts"), strings.HasPrefix(path, "/api/dbroot"), strings.HasPrefix(path, "/api/notify"):
		return false
	case method != "GET" && method != "HEAD" && strings.HasPrefix(path, "/api/hw/"):
		return false
	case method != "GET" && method != "HEAD" && strings.Contains(path, "/dbusers"):
		return false
	}
	return true
}

// bearer membaca "Authorization: Bearer <token>". Kembalian nil + kode bila tidak sah.
func (a *App) bearer(r *http.Request) (*store.Session, int) {
	ip := a.ClientIP(r)
	if a.Store.CountAttempts("ip:"+ip, a.Now().Add(-LockWindow)) >= MaxPerIP*3 {
		return nil, 429
	}
	ss := a.Store.TokenSession(strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), ip, a.Now())
	if ss == nil {
		a.Store.AddAttempt("ip:"+ip, a.Now())
		a.Store.Audit("token", "token_fail", "", ip, a.Now())
		return nil, 401
	}
	return ss, 0
}

func HumanOnly(w http.ResponseWriter, ss *store.Session) bool {
	if ss.Via == "token" {
		WriteError(w, 403, "tidak tersedia lewat token API")
		return false
	}
	return true
}

func (a *App) Session(r *http.Request) *store.Session {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil
	}
	return a.Store.GetSession(c.Value, a.Now())
}

func (a *App) Wrap(rt Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		SecHeaders(w)
		var ss *store.Session
		if rt.Level != Public {
			ss = a.Session(r)
			if ss == nil && rt.Level == Full && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				var code int
				if ss, code = a.bearer(r); ss == nil {
					WriteError(w, code, map[int]string{401: "token tidak valid", 429: "terlalu banyak percobaan"}[code])
					return
				}
			}
			if ss == nil || (rt.Level == Pwd && ss.Stage != "pwd") || (rt.Level == Full && ss.Stage != "full") {
				WriteError(w, 401, "unauthenticated")
				return
			}
		}
		if ss != nil && ss.MustChange && ss.Via != "token" && ss.Stage == "full" && !mustChangeOK(r.URL.Path) {
			WriteError(w, 403, "ganti kata sandi sementara dulu")
			return
		}
		if ss != nil && ss.Via == "token" {
			if !TokenAllowed(r.Method, r.URL.Path) {
				WriteError(w, 403, "tidak tersedia lewat token API")
				return
			}
			if r.Method != "GET" && r.Method != "HEAD" {
				if ss.Scope == "read" {
					WriteError(w, 403, "token baca-saja")
					return
				}
				if ss.Role == "viewer" {
					WriteError(w, 403, "akun ini hanya-baca")
					return
				}
				a.Store.Audit(ss.User, "api_call", ss.TokenName+" "+r.Method+" "+r.URL.Path, a.ClientIP(r), a.Now())
			}
			rt.H(a, w, r, ss)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if ss != nil && ss.Stage == "full" && ss.Role == "viewer" && !viewerSelfService(r.URL.Path) {
				WriteError(w, 403, "akun ini hanya-baca")
				return
			}
			if !a.originOK(r) {
				WriteError(w, 403, "forbidden")
				return
			}
			if ss != nil && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(ss.CSRF)) != 1 {
				WriteError(w, 403, "forbidden")
				return
			}
		}
		rt.H(a, w, r, ss)
	}
}

func (a *App) Locked(name, ip string) bool {
	since := a.Now().Add(-LockWindow)
	return a.Store.CountAttempts("u:"+name, since) >= MaxPerUser || a.Store.CountAttempts("ip:"+ip, since) >= MaxPerIP
}

func (a *App) Fail(name, ip string) {
	a.Store.AddAttempt("u:"+name, a.Now())
	a.Store.AddAttempt("ip:"+ip, a.Now())
}

// RequireStepUp dipakai aksi destruktif (fase berikutnya).
func (a *App) RequireStepUp(w http.ResponseWriter, ss *store.Session) bool {
	if ss.Via == "token" {
		if ss.Scope == "full" {
			return true
		}
		WriteError(w, 403, "aksi ini butuh token tingkat 'full'")
		return false
	}
	if ss.StepUp > a.Now().Unix() {
		return true
	}
	WriteError(w, 403, "butuh verifikasi kata sandi ulang")
	return false
}

// mustChangeOK: akun dengan kata sandi sementara hanya boleh mengganti kata sandi / menyiapkan 2FA.
func mustChangeOK(p string) bool {
	return p == "/api/me" || p == "/api/me/password" || strings.HasPrefix(p, "/api/me/2fa") || strings.HasPrefix(p, "/api/auth/") || p == "/api/settings/public" || p == "/api/settings"
}

// viewerSelfService: satu-satunya penulisan yang boleh dilakukan akun hanya-baca, semuanya hanya menyangkut akunnya sendiri.
func viewerSelfService(p string) bool {
	return p == "/api/auth/logout" || p == "/api/auth/stepup" || p == "/api/me/password" || strings.HasPrefix(p, "/api/me/2fa/") || p == "/api/me/devices/revoke"
}
