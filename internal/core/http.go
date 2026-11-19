package core

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func SecHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
}

func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, code int, msg string) {
	WriteJSON(w, code, map[string]string{"error": msg})
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return DecodeJSONLimit(w, r, v, MaxBodySize)
}

// decodeN membatasi ukuran body per rute; editor config butuh lebih besar dari default.
func DecodeJSONLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		code := 400
		if strings.Contains(err.Error(), "too large") {
			code = 413
		}
		WriteError(w, code, "bad request")
		return false
	}
	return true
}

func (a *App) ClientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			p := strings.Split(xff, ",")
			last := strings.TrimSpace(p[len(p)-1]) // nilai terakhir ditambahkan nginx; yang lebih awal bisa dipalsukan klien
			if net.ParseIP(last) != nil {
				return last
			}
		}
	}
	return host
}

func (a *App) originOK(r *http.Request) bool {
	if sf := r.Header.Get("Sec-Fetch-Site"); sf != "" && sf != "same-origin" && sf != "none" {
		return false
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	allowed := a.Cfg.Origins
	if len(allowed) == 0 {
		allowed = []string{"https://" + r.Host, "http://" + r.Host}
	}
	for _, x := range allowed {
		if o == x {
			return true
		}
	}
	return false
}

func (a *App) SetCookie(w http.ResponseWriter, tok string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: tok, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func TooMany(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(int(LockWindow.Seconds())))
	WriteError(w, 429, "terlalu banyak percobaan, coba lagi nanti")
}
