package overview

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidServer(t *testing.T) {
	ok := map[string]string{
		"https://ntfy.sh":               "https://ntfy.sh",
		"https://ntfy.sh/":              "https://ntfy.sh",
		"  https://ntfy.example.com  ":  "https://ntfy.example.com",
		"https://ntfy.example.com/sub":  "https://ntfy.example.com/sub",
		"https://ntfy.example.com:8443": "https://ntfy.example.com:8443",
		"https://100.64.0.10":           "https://100.64.0.10", // alamat tailnet (bukan privat) boleh untuk ntfy milik sendiri
	}
	for in, want := range ok {
		got, err := ValidServer(in, false)
		if err != nil || got != want {
			t.Errorf("%q: %q %v, mau %q", in, got, err, want)
		}
	}
	bad := []string{
		"", "ntfy.sh", "http://ntfy.sh", "ftp://ntfy.sh", "https://", "https:///x",
		"https://user:pw@ntfy.sh", "https://user@ntfy.sh",
		"https://ntfy.sh?x=1", "https://ntfy.sh#a",
		"https://127.0.0.1", "https://[::1]", "https://10.0.0.5", "https://192.168.1.2", "https://172.16.0.1",
		"https://169.254.169.254", "https://0.0.0.0", "https://224.0.0.1", "https://[fe80::1]", "https://[fc00::1]",
		"https://localhost", "https://LOCALHOST:8080", "https://panel.lab", "https://x.local", "https://db.internal",
	}
	for _, in := range bad {
		if got, err := ValidServer(in, false); err == nil {
			t.Errorf("%q harus ditolak, malah diterima sebagai %q", in, got)
		}
	}
}

func TestValidServerAllowLocalOnlyForTests(t *testing.T) {
	if _, err := ValidServer("http://127.0.0.1:9999", true); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidServer("http://127.0.0.1:9999", false); err == nil {
		t.Fatal("http loopback harus ditolak di produksi")
	}
	if _, err := ValidServer("http://x@127.0.0.1", true); err == nil {
		t.Fatal("kredensial di URL harus tetap ditolak walau allowLocal")
	}
}

// Penjaga SSRF harus bekerja saat koneksi dibuat, bukan hanya saat input: server loopback sungguhan harus tidak terjangkau.
func TestGuardedClientRefusesLoopbackAtDialTime(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()
	resp, err := guardedClient().Get(srv.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || hit {
		t.Fatalf("klien produksi tidak boleh menjangkau loopback (hit=%v err=%v)", hit, err)
	}
	if !strings.Contains(err.Error(), "ditolak") {
		t.Fatalf("galat harus berasal dari penjaga alamat: %v", err)
	}
}

func TestGuardedClientDoesNotFollowRedirects(t *testing.T) {
	cl := guardedClient()
	if err := cl.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatalf("redirect tidak boleh diikuti (bisa dibelokkan ke alamat internal): %v", err)
	}
	if cl.Timeout == 0 {
		t.Fatal("klien harus punya batas waktu")
	}
	if tr, ok := cl.Transport.(*http.Transport); !ok || tr.Proxy != nil {
		t.Fatal("klien tidak boleh memakai proxy lingkungan")
	}
}

func TestTopicRegex(t *testing.T) {
	for _, s := range []string{"abcdefghijklmnop", "A_b-c_d-e_f-g_h-i", strings.Repeat("a", 64)} {
		if !topicRE.MatchString(s) {
			t.Errorf("%q harus sah", s)
		}
	}
	for _, s := range []string{"", "pendek", strings.Repeat("a", 65), "spasi di sini aaaaaaaaaa", "titik.titik.titik.titik", "garis/miring/aaaaaaaa", "../../etc/passwd/aaaa", "tanya?x=1aaaaaaaaaaaa", "kutip'aaaaaaaaaaaaaaa", "huruf\u00e9aaaaaaaaaaaaaaa"} {
		if topicRE.MatchString(s) {
			t.Errorf("%q harus ditolak", s)
		}
	}
}

func TestScrubRemovesTopic(t *testing.T) {
	got := scrub("Post https://ntfy.sh/RahasiaTopikPanjang123: dial tcp: timeout", "RahasiaTopikPanjang123")
	if strings.Contains(got, "RahasiaTopikPanjang123") || !strings.Contains(got, "***") {
		t.Fatal(got)
	}
	if len(scrub(strings.Repeat("x", 500), "")) > 160 {
		t.Fatal("pesan galat harus dipotong")
	}
}

func TestLevelOK(t *testing.T) {
	cases := []struct {
		min, lv string
		want    bool
	}{{"bad", "bad", true}, {"bad", "warn", false}, {"warn", "warn", true}, {"warn", "bad", true}, {"bad", "", false}, {"", "warn", false}}
	for _, c := range cases {
		if levelOK(c.min, c.lv) != c.want {
			t.Errorf("%+v", c)
		}
	}
}
