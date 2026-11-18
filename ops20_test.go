package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const dbFake = `case "$1 $2" in
"db user") printf '{"ok":true,"user":"x","password":"PwAcakDariLab1234567890"}\n';;
"db root-passwd") printf '{"ok":true,"engine":"x"}\n';;
"db passwd") echo "OK: diganti";;
*) echo "ok $1 $2";; esac`

func (o *opsEnv) login2(name, pw string) (cookie, csrf string, code int) {
	w := o.do("POST", "/api/auth/login", `{"name":"`+name+`","password":"`+pw+`"}`, "", "", nil)
	return cookieOf(w), csrfOf(w), w.Code
}

func (o *opsEnv) as(cookie, csrf, method, path, body string) (int, string) {
	w := o.do(method, path, body, cookie, csrf, nil)
	return w.Code, w.Body.String()
}

func TestAccountManagement(t *testing.T) {
	o := newOpsEnv(t, dbFake)
	if c, _ := o.post("/api/accounts", `{"name":"budi","role":"viewer"}`); c != 403 {
		t.Fatalf("buat akun tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	for _, bad := range []string{`{"name":"B","role":"viewer"}`, `{"name":"ab","role":"viewer"}`, `{"name":"budi","role":"root"}`, `{"name":"bu di","role":"viewer"}`} {
		if c, _ := o.post("/api/accounts", bad); c != 400 {
			t.Fatalf("%s harus 400: %d", bad, c)
		}
	}
	c, b := o.post("/api/accounts", `{"name":"budi","role":"admin"}`)
	var mk struct {
		Temp string `json:"temp_password"`
	}
	json.Unmarshal([]byte(b), &mk)
	if c != 200 || len(mk.Temp) < 16 {
		t.Fatalf("buat akun: %d %s", c, b)
	}
	if c, _ := o.post("/api/accounts", `{"name":"budi","role":"viewer"}`); c != 409 {
		t.Fatalf("nama kembar harus 409: %d", c)
	}
	if strings.Contains(strings.Join(func() []string {
		var s []string
		for _, r := range o.app.st.RecentAudit(50) {
			s = append(s, r.Detail)
		}
		return s
	}(), "|"), mk.Temp) {
		t.Fatal("kata sandi sementara bocor ke audit")
	}
	// login pertama: 2FA mati, tapi dikunci sampai kata sandi diganti
	bc, bs, code := o.login2("budi", mk.Temp)
	if code != 200 || bc == "" {
		t.Fatalf("login akun baru: %d", code)
	}
	if c, b := o.as(bc, bs, "GET", "/api/me", ""); c != 200 || !strings.Contains(b, `"must_change":true`) {
		t.Fatalf("me harus menandai must_change: %d %s", c, b)
	}
	for _, p := range []string{"/api/sites", "/api/db", "/api/accounts", "/api/system"} {
		if c, b := o.as(bc, bs, "GET", p, ""); c != 403 || !strings.Contains(b, "ganti kata sandi sementara") {
			t.Fatalf("%s harus terkunci sebelum ganti sandi: %d %s", p, c, b)
		}
	}
	o.as(bc, bs, "POST", "/api/auth/stepup", `{"password":"`+mk.Temp+`"}`)
	if c, b := o.as(bc, bs, "POST", "/api/me/password", `{"current":"`+mk.Temp+`","new":"sandi-baru-budi-12345"}`); c != 200 {
		t.Fatalf("ganti sandi sementara: %d %s", c, b)
	}
	if c, _ := o.as(bc, bs, "GET", "/api/sites", ""); c != 200 {
		t.Fatalf("setelah ganti sandi harus terbuka: %d", c)
	}
	if _, _, code := o.login2("budi", mk.Temp); code == 200 && o.app.st.GetUser("budi").MustChange {
		t.Fatal("sandi sementara masih berlaku")
	}

	// penjagaan: diri sendiri
	o.stepup()
	for _, p := range []string{"role", "disable", "enable", "reset", "2fa-off", "remove"} {
		if c, _ := o.post("/api/accounts/admin/"+p, `{"role":"viewer","confirm":"admin"}`); c != 400 {
			t.Fatalf("%s pada diri sendiri harus 400: %d", p, c)
		}
	}
	if c, _ := o.post("/api/accounts/tidakada/disable", `{}`); c != 404 {
		t.Fatalf("akun tak ada harus 404: %d", c)
	}
	// nonaktifkan: sesi budi mati, login ditolak, aktifkan lagi
	if c, b := o.post("/api/accounts/budi/disable", `{}`); c != 200 {
		t.Fatalf("disable: %d %s", c, b)
	}
	if c, _ := o.as(bc, bs, "GET", "/api/me", ""); c != 401 {
		t.Fatalf("sesi akun nonaktif harus mati: %d", c)
	}
	if _, _, code := o.login2("budi", "sandi-baru-budi-12345"); code == 200 {
		t.Fatal("akun nonaktif bisa login")
	}
	o.post("/api/accounts/budi/enable", `{}`)
	// ubah peran ke viewer: token dicabut
	o.app.st.AddToken(o.app.st.GetUser("budi").ID, "t", "full", 30, o.now)
	if c, _ := o.post("/api/accounts/budi/role", `{"role":"viewer"}`); c != 200 {
		t.Fatal("role gagal")
	}
	var n int
	o.app.st.db.QueryRow("SELECT COUNT(*) FROM tokens WHERE user_id=?", o.app.st.GetUser("budi").ID).Scan(&n)
	if n != 0 {
		t.Fatal("token tidak dicabut saat peran berubah")
	}
	// reset: sandi lama mati, must_change menyala
	c, b = o.post("/api/accounts/budi/reset", `{}`)
	json.Unmarshal([]byte(b), &mk)
	if c != 200 || len(mk.Temp) < 16 || !o.app.st.GetUser("budi").MustChange {
		t.Fatalf("reset: %d %s", c, b)
	}
	if _, _, code := o.login2("budi", "sandi-baru-budi-12345"); code == 200 {
		t.Fatal("sandi lama masih berlaku setelah reset")
	}
	// 2FA off milik orang lain
	if c, _ := o.post("/api/accounts/budi/2fa-off", `{}`); c != 409 {
		t.Fatalf("2FA budi sudah mati: %d", c)
	}
	// hapus: wajib ketik nama
	if c, _ := o.post("/api/accounts/budi/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("konfirmasi salah harus 400: %d", c)
	}
	if c, _ := o.post("/api/accounts/budi/remove", `{"confirm":"budi"}`); c != 200 || o.app.st.GetUser("budi") != nil {
		t.Fatal("hapus gagal")
	}
	// admin terakhir tidak bisa disingkirkan oleh admin lain: dua admin, satu menurunkan yang lain lalu sebaliknya
	o.post("/api/accounts", `{"name":"citra","role":"admin"}`)
	if c, _ := o.post("/api/accounts/citra/role", `{"role":"viewer"}`); c != 200 {
		t.Fatal("turunkan admin kedua harus boleh (masih ada admin lain)")
	}
	if o.app.st.AdminsExcept("citra") < 1 {
		t.Fatal("tidak ada admin tersisa")
	}
}

func TestAccountsNotForViewerOrToken(t *testing.T) {
	o := newOpsEnv(t, dbFake)
	o.stepup()
	tok, _ := o.mkToken("full")
	for _, p := range [][2]string{{"GET", "/api/accounts"}, {"POST", "/api/accounts"}, {"POST", "/api/accounts/x/remove"}, {"POST", "/api/dbroot/mysql/passwd"}} {
		if c, b := o.bearer(p[0], p[1], `{}`, tok); c != 403 || !strings.Contains(b, "tidak tersedia lewat token") {
			t.Fatalf("%s %s harus 403 bagi token full: %d %s", p[0], p[1], c, b)
		}
	}
	o.app.st.SetRole("admin", "viewer")
	vc, vs, _ := o.login(o.code(1))
	if c, _ := o.as(vc, vs, "GET", "/api/accounts", ""); c != 403 {
		t.Fatalf("viewer tidak boleh melihat akun: %d", c)
	}
}

func TestDBUsersAndRoot(t *testing.T) {
	o := newOpsEnv(t, dbFake)
	pw := "SandiPilihanSendiri-9876"
	base := "/api/db/mariadb/proj/dbusers"
	if c, _ := o.post(base, `{"user":"pembaca","privs":"ro"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	for _, bad := range []string{`{"user":"AB","privs":"ro"}`, `{"user":"pembaca","privs":"root"}`, `{"user":"pembaca","privs":"ro","password":"pendek"}`, `{"user":"x'; drop","privs":"ro"}`, `{"user":"pembaca","privs":"ro","password":"a'b'c'd'e'f'g'h'i'j"}`} {
		if c, _ := o.post(base, bad); c != 400 {
			t.Fatalf("%s harus 400: %d", bad, c)
		}
	}
	if strings.Contains(o.calls(), "[user][add]") {
		t.Fatal("lab dipanggil dengan input tak valid")
	}
	// acak: kata sandi dikembalikan sekali dari lab; tidak masuk audit/job
	c, b := o.post(base, `{"user":"pembaca","privs":"ro"}`)
	if c != 200 || !strings.Contains(b, "PwAcakDariLab") {
		t.Fatalf("tambah user acak: %d %s", c, b)
	}
	if strings.Contains(o.calls(), "--password-stdin") {
		t.Fatal("tanpa sandi pilihan, stdin tidak dipakai")
	}
	// pilihan sendiri: lewat stdin, tidak pernah di argumen
	if c, b := o.post(base, `{"user":"penulis","privs":"rw","password":"`+pw+`"}`); c != 200 {
		t.Fatalf("tambah user dengan sandi sendiri: %d %s", c, b)
	}
	last := o.calls()[strings.LastIndex(o.calls(), "CALL:"):]
	if !strings.Contains(last, "[--password-stdin]") || !strings.Contains(last, "STDIN=["+pw+"]") {
		t.Fatalf("sandi harus lewat stdin: %s", last)
	}
	if strings.Count(last, pw) != 1 {
		t.Fatalf("sandi muncul di argumen: %s", last)
	}
	for _, r := range o.app.st.RecentAudit(50) {
		if strings.Contains(r.Detail, pw) || strings.Contains(r.Detail, "PwAcak") {
			t.Fatalf("sandi bocor ke audit: %s", r.Detail)
		}
	}
	if c, _ := o.post(base+"/penulis/privs", `{"privs":"all"}`); c != 200 || !strings.Contains(o.calls(), "[privs][proj][penulis]") {
		t.Fatal("ubah hak gagal")
	}
	if c, _ := o.post(base+"/penulis/privs", `{"privs":"super"}`); c != 400 {
		t.Fatal("hak tidak valid harus 400")
	}
	if c, _ := o.post(base+"/penulis/passwd", `{"password":"x"}`); c != 400 {
		t.Fatal("sandi pendek harus 400")
	}
	if c, _ := o.post(base+"/penulis/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatal("hapus tanpa konfirmasi harus 400")
	}
	if c, _ := o.post(base+"/penulis/remove", `{"confirm":"penulis"}`); c != 200 || !strings.Contains(o.calls(), "[user][remove][proj][penulis]") {
		t.Fatal("hapus user gagal")
	}
	// token: baca boleh, tulis tidak
	tok, _ := o.mkToken("full")
	if c, _ := o.bearer("GET", base, "", tok); c != 200 {
		t.Fatalf("token boleh membaca daftar: %d", c)
	}
	if c, b := o.bearer("POST", base, `{"user":"agen","privs":"all"}`, tok); c != 403 || !strings.Contains(b, "tidak tersedia lewat token") {
		t.Fatalf("token tidak boleh membuat user DB: %d %s", c, b)
	}
	// root
	if c, _ := o.post("/api/dbroot/mysql/passwd", `{"confirm":"mariadb"}`); c != 400 {
		t.Fatal("konfirmasi root harus nama mesin")
	}
	if c, _ := o.post("/api/dbroot/redis/passwd", `{"confirm":"redis"}`); c != 400 {
		t.Fatal("mesin tak dikenal harus 400")
	}
	if c, _ := o.post("/api/dbroot/mysql/passwd", `{"confirm":"mysql","password":"pendek"}`); c != 400 {
		t.Fatal("sandi root pendek harus 400")
	}
	if c, b := o.post("/api/dbroot/mysql/passwd", `{"confirm":"mysql","password":"`+pw+`"}`); c != 200 {
		t.Fatalf("root passwd: %d %s", c, b)
	}
	last = o.calls()[strings.LastIndex(o.calls(), "CALL:"):]
	if !strings.Contains(last, "[root-passwd]") || strings.Count(last, pw) != 1 {
		t.Fatalf("root: sandi harus hanya di stdin: %s", last)
	}
	// sandi proyek pilihan sendiri
	if c, _ := o.post("/api/db/mariadb/proj/passwd", `{"confirm":"proj","password":"`+pw+`"}`); c != 200 || !strings.Contains(o.calls(), "STDIN=["+pw+"]") {
		t.Fatal("sandi proyek pilihan sendiri gagal")
	}
}

func TestDBRootErrorScrubsPassword(t *testing.T) {
	o := newOpsEnv(t, `echo "ERROR: syntax near PASSWORD 'SandiBocor-123456'" >&2; exit 1`)
	o.stepup()
	pw := "SandiBocor-123456"
	c, b := o.post("/api/dbroot/pgsql/passwd", `{"confirm":"pgsql","password":"`+pw+`"}`)
	if c != 422 || strings.Contains(b, pw) {
		t.Fatalf("galat tidak boleh memuat sandi: %d %s", c, b)
	}
}

func TestAccountActionsNeedStepUpAndRevoke(t *testing.T) {
	o := newOpsEnv(t, dbFake)
	o.stepup()
	_, b := o.post("/api/accounts", `{"name":"budi","role":"admin"}`)
	var mk struct {
		Temp string `json:"temp_password"`
	}
	json.Unmarshal([]byte(b), &mk)
	o.app.st.SetTempPassword(o.app.st.GetUser("budi").ID, hashPassword("sandi-budi-12345678"))
	o.app.st.db.Exec("UPDATE users SET must_change=0 WHERE name='budi'")
	bc, bs, code := o.login2("budi", "sandi-budi-12345678")
	if code != 200 {
		t.Fatalf("login budi: %d", code)
	}
	budi := o.app.st.GetUser("budi")
	o.app.st.AddToken(budi.ID, "t", "full", 30, o.now)
	o.app.st.AddTrusted(budi.ID, "hp", o.now)
	// step-up kedaluwarsa: semua aksi sensitif harus 403
	o.app.st.db.Exec("UPDATE sessions SET stepup=0")
	for _, p := range [][2]string{
		{"/api/accounts", `{"name":"dewi","role":"viewer"}`}, {"/api/accounts/budi/role", `{"role":"viewer"}`}, {"/api/accounts/budi/disable", `{}`},
		{"/api/accounts/budi/reset", `{}`}, {"/api/accounts/budi/remove", `{"confirm":"budi"}`}, {"/api/dbroot/mysql/passwd", `{"confirm":"mysql"}`},
		{"/api/db/mariadb/proj/dbusers", `{"user":"abc","privs":"ro"}`},
	} {
		if c, b := o.post(p[0], p[1]); c != 403 || !strings.Contains(b, "kata sandi ulang") {
			t.Fatalf("%s tanpa step-up harus 403: %d %s", p[0], c, b)
		}
	}
	if o.app.st.GetUser("budi") == nil || o.app.st.GetUser("budi").MustChange {
		t.Fatal("aksi tanpa step-up tidak boleh mengubah apa pun")
	}
	o.stepup()
	if c, _ := o.post("/api/accounts/budi/role", `{"role":"viewer"}`); c != 200 {
		t.Fatal("role gagal")
	}
	role := ""
	for _, r := range o.app.st.AccountRows("admin") {
		if r.Name == "budi" {
			role = r.Role
		}
	}
	if role != "viewer" {
		t.Fatalf("peran tidak tersimpan: %q", role)
	}
	if c, _ := o.as(bc, bs, "GET", "/api/me", ""); c != 401 {
		t.Fatalf("sesi budi harus dicabut saat peran berubah: %d", c)
	}
	// reset mencabut sesi, perangkat tepercaya, token
	bc, bs, _ = o.login2("budi", "sandi-budi-12345678")
	o.app.st.AddToken(budi.ID, "t2", "full", 30, o.now)
	o.app.st.AddTrusted(budi.ID, "hp2", o.now)
	if c, _ := o.post("/api/accounts/budi/reset", `{}`); c != 200 {
		t.Fatal("reset gagal")
	}
	if c, _ := o.as(bc, bs, "GET", "/api/me", ""); c != 401 {
		t.Fatalf("sesi harus dicabut saat reset: %d", c)
	}
	var nt, ntr int
	o.app.st.db.QueryRow("SELECT COUNT(*) FROM tokens WHERE user_id=?", budi.ID).Scan(&nt)
	o.app.st.db.QueryRow("SELECT COUNT(*) FROM trusted WHERE user_id=?", budi.ID).Scan(&ntr)
	if nt != 0 || ntr != 0 {
		t.Fatalf("token (%d) atau perangkat tepercaya (%d) tidak dicabut saat reset", nt, ntr)
	}
}

func TestViewerCanOnlyManageOwnAccount(t *testing.T) {
	o := newOpsEnv(t, dbFake)
	o.stepup()
	_, b := o.post("/api/accounts", `{"name":"dewi","role":"viewer"}`)
	var mk struct {
		Temp string `json:"temp_password"`
	}
	json.Unmarshal([]byte(b), &mk)
	vc, vs, code := o.login2("dewi", mk.Temp)
	if code != 200 {
		t.Fatalf("login viewer: %d", code)
	}
	if c, b := o.as(vc, vs, "POST", "/api/auth/stepup", `{"password":"`+mk.Temp+`"}`); c != 200 {
		t.Fatalf("viewer harus bisa verifikasi ulang: %d %s", c, b)
	}
	if c, b := o.as(vc, vs, "POST", "/api/me/password", `{"current":"`+mk.Temp+`","new":"sandi-dewi-baru-12345"}`); c != 200 {
		t.Fatalf("viewer harus bisa ganti sandi sementara: %d %s", c, b)
	}
	if c, _ := o.as(vc, vs, "GET", "/api/sites", ""); c != 200 {
		t.Fatalf("setelah ganti sandi viewer bisa membaca: %d", c)
	}
	for _, p := range [][2]string{{"/api/settings", `{"name":"X"}`}, {"/api/sites", `{"name":"x","target":"~/x"}`}, {"/api/accounts", `{"name":"zzz"}`}, {"/api/db/mariadb/proj/dbusers", `{"user":"abc","privs":"all"}`}, {"/api/me/tokens", `{"name":"t","scope":"full"}`}} {
		if c, b := o.as(vc, vs, "POST", p[0], p[1]); c != 403 {
			t.Fatalf("viewer tidak boleh POST %s: %d %s", p[0], c, b)
		}
	}
}
