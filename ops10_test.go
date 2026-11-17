package main

import (
	"strings"
	"testing"
	"time"
)

const fakeRedis = `case "$1 $2" in
"redis info") printf '{"dbsize":2,"used_memory":"1M","maxmemory":"0B","keyspace":{}}';;
"redis scan") printf '{"keys":[{"key":"a","type":"string","ttl":-1}],"more":false}';;
"redis get") printf '{"key":"k","type":"string","ttl":-1,"size":3,"value":"nilai-rahasia","truncated":false}';;
*) echo "OK";;
esac`

func TestRedisEndpoints(t *testing.T) {
	o := newOpsEnv(t, fakeRedis)
	o.app.longTimeout = 20 * time.Second
	if w := o.do("GET", "/api/redis/scan?pattern=user:*", "", o.cookie, "", nil); w.Code != 200 {
		t.Fatalf("scan: %d", w.Code)
	}
	before := o.calls()
	for _, q := range []string{"/api/redis/scan?pattern=a%20b", "/api/redis/scan?pattern=a%3Bid", "/api/redis/scan?pattern=$(id)", "/api/redis/scan?db=16", "/api/redis/scan?db=-1", "/api/redis/scan?db=01", "/api/redis/scan?db=x", "/api/redis/info?db=99"} {
		if w := o.do("GET", q, "", o.cookie, "", nil); w.Code != 400 {
			t.Errorf("%s -> %d", q, w.Code)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil: %s", o.calls())
	}
	if c, b := o.post("/api/redis/get", `{"key":"k"}`); c != 403 || strings.Contains(b, "rahasia") {
		t.Fatalf("get tanpa step-up: %d %s", c, b)
	}
	if c, _ := o.post("/api/redis/del", `{"key":"k"}`); c != 403 {
		t.Fatalf("del tanpa step-up: %d", c)
	}
	o.stepup()
	for _, b := range []string{`{"key":""}`, `{"key":"a b"}`, `{"key":"-x"}`, `{"key":"a\nFLUSHALL"}`, `{"key":"` + strings.Repeat("a", 201) + `"}`, `{"key":"k","db":"16"}`, `{"key":"k","x":1}`, `{"key":"é"}`} {
		for _, p := range []string{"/api/redis/get", "/api/redis/del"} {
			if c, r := o.post(p, b); c != 400 {
				t.Errorf("%s %s -> %d %s", p, b, c, r)
			}
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	if c, b := o.post("/api/redis/get", `{"key":"sess:abc","db":"2"}`); c != 200 || !strings.Contains(o.calls(), "CALL:[redis][get][--db][2][sess:abc]") {
		t.Fatalf("get: %d %s %s", c, b, o.calls())
	}
	if c, _ := o.post("/api/redis/del", `{"key":"sess:abc"}`); c != 200 || !strings.Contains(o.calls(), "CALL:[redis][del][--db][0][sess:abc]") {
		t.Fatalf("del: %s", o.calls())
	}
	if au := o.do("GET", "/api/audit", "", o.cookie, "", nil).Body.String(); !strings.Contains(au, "redis_view") || strings.Contains(au, "rahasia") {
		t.Errorf("audit: %s", au)
	}
	o.app.st.db.Exec("UPDATE users SET role='viewer'")
	for _, p := range []string{"/api/redis/scan", "/api/redis/info"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 403 {
			t.Errorf("viewer %s: %d", p, w.Code)
		}
	}
	if c, b := o.post("/api/redis/get", `{"key":"k"}`); c != 403 || strings.Contains(b, "rahasia") {
		t.Errorf("viewer get: %d %s", c, b)
	}
}

func TestChangeOwnPassword(t *testing.T) {
	o := newOpsEnv(t, "")
	const old, nw = "kata-sandi-panjang-123", "sandi-baru-yang-panjang-456"
	// sesi kedua milik user yang sama harus dicabut
	o.now = o.now.Add(31 * time.Second)
	c2, _, w := o.login(o.code(0))
	if w.Code != 200 {
		t.Fatalf("login kedua: %d", w.Code)
	}
	if c, _ := o.post("/api/me/password", `{"current":"`+old+`","new":"`+nw+`"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	for _, b := range []string{`{"current":"salah","new":"` + nw + `"}`, `{"current":"` + old + `","new":"pendek"}`, `{"current":"` + old + `","new":"` + old + `"}`, `{"current":"` + old + `","new":"roki"}`, `{"current":"` + old + `","new":"` + strings.Repeat("a", 257) + `"}`, `{"current":"` + old + `","new":"` + nw + `","x":1}`} {
		if c, r := o.post("/api/me/password", b); c != 400 && c != 401 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if w := o.do("POST", "/api/auth/login", `{"name":"roki","password":"`+nw+`"}`, "", "", nil); w.Code == 200 {
		t.Fatal("sandi baru tidak boleh aktif setelah permintaan ditolak")
	}
	if c, b := o.post("/api/me/password", `{"current":"`+old+`","new":"`+nw+`"}`); c != 200 {
		t.Fatalf("ganti sah: %d %s", c, b)
	}
	if w := o.do("GET", "/api/me", "", c2, "", nil); w.Code != 401 {
		t.Errorf("sesi lain harus dicabut: %d", w.Code)
	}
	if w := o.do("GET", "/api/me", "", o.cookie, "", nil); w.Code != 200 {
		t.Errorf("sesi saat ini harus tetap hidup: %d", w.Code)
	}
	if w := o.do("POST", "/api/auth/login", `{"name":"roki","password":"`+old+`"}`, "", "", nil); w.Code == 200 {
		t.Error("sandi lama harus ditolak")
	}
	if w := o.do("POST", "/api/auth/login", `{"name":"roki","password":"`+nw+`"}`, "", "", nil); w.Code != 200 {
		t.Errorf("sandi baru harus berlaku: %d", w.Code)
	}
	if au := o.do("GET", "/api/audit", "", o.cookie, "", nil).Body.String(); !strings.Contains(au, "password_change") || strings.Contains(au, nw) || strings.Contains(au, old) {
		t.Errorf("audit: %s", au)
	}
}
