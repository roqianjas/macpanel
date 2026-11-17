package main

import (
	"strings"
	"testing"
	"time"
)

const fakeCfg = `case "$1 $2" in
"dbcfg get") printf '{"engine":"mysql","keys":[{"key":"max_connections","value":"200","live":"200","hint":"h"},{"key":"skip-grant-tables","value":"1","live":"","hint":""}]}';;
"dbcfg slow") echo "slow log token=RAHASIA123";;
"ini get") printf '{"version":"8.4","keys":[{"key":"pm.max_children","value":"8","default":"","hint":"h","group":"fpm"},{"key":"disable_functions","value":"","default":"","hint":"","group":"php"}]}';;
*) echo "ok $1 $2";;
esac`

func newCfgEnv(t *testing.T) *opsEnv {
	o := newOpsEnv(t, fakeCfg)
	o.app.longTimeout = 20 * time.Second
	return o
}

func TestDBCfgGetFiltersAndScrubs(t *testing.T) {
	o := newCfgEnv(t)
	b := o.do("GET", "/api/dbcfg/mysql", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, "max_connections") || strings.Contains(b, "skip-grant") {
		t.Fatalf("kunci tak dikenal harus disaring: %s", b)
	}
	if w := o.do("GET", "/api/dbcfg/postgres", "", o.cookie, "", nil); w.Code != 400 {
		t.Errorf("engine buruk: %d", w.Code)
	}
	if s := o.do("GET", "/api/dbcfg/mysql/slow", "", o.cookie, "", nil).Body.String(); strings.Contains(s, "RAHASIA123") {
		t.Errorf("slow log harus disaring: %s", s)
	}
}

func TestDBCfgSetValidation(t *testing.T) {
	o := newCfgEnv(t)
	if c, _ := o.post("/api/dbcfg/mysql", `{"values":{"max_connections":"200"}}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	before := o.calls()
	for _, p := range [][2]string{
		{"mysql", `{"values":{"skip-grant-tables":"1"}}`}, {"mysql", `{"values":{"datadir":"/tmp"}}`}, {"mysql", `{"values":{"max_connections":"5"}}`},
		{"mysql", `{"values":{"max_connections":"3000"}}`}, {"mysql", `{"values":{"max_connections":"200\nx=1"}}`}, {"mysql", `{"values":{"innodb_buffer_pool_size":"99G"}}`},
		{"mysql", `{"values":{"innodb_buffer_pool_size":"10"}}`}, {"mysql", `{"values":{"slow_query_log":"2"}}`}, {"mysql", `{"values":{"long_query_time":"61"}}`},
		{"mysql", `{"values":{}}`}, {"redis", `{"values":{"requirepass":"x"}}`}, {"redis", `{"values":{"maxmemory":"abc"}}`}, {"redis", `{"values":{"maxmemory-policy":"x"}}`},
		{"redis", `{"values":{"save":"1 1"}}`}, {"redis", `{"values":{"appendonly":"maybe"}}`}, {"redis", `{"values":{"maxmemory":"256mb","dir":"/tmp"}}`},
	} {
		if c, b := o.post("/api/dbcfg/"+p[0], p[1]); c != 400 {
			t.Errorf("%s %s -> %d %s", p[0], p[1], c, b)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	c, b := o.post("/api/dbcfg/mysql", `{"values":{"slow_query_log":"1","long_query_time":"2.5","max_connections":"300","innodb_buffer_pool_size":"1G"}}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[dbcfg][set][mysql][innodb_buffer_pool_size=1G][long_query_time=2.5][max_connections=300][slow_query_log=1]") {
		t.Fatalf("argv: %s", o.calls())
	}
	c, b = o.post("/api/dbcfg/redis", `{"values":{"maxmemory":"0","maxmemory-policy":"allkeys-lru","appendonly":"yes","save":"off"}}`)
	if c != 202 {
		t.Fatalf("redis %d %s", c, b)
	}
	o.waitJob(jobID(t, b))
}

func TestPoolKeysViaIni(t *testing.T) {
	o := newCfgEnv(t)
	if b := o.do("GET", "/api/php/8.4/ini", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"group":"fpm"`) || strings.Contains(b, "disable_functions") {
		t.Fatalf("%s", b)
	}
	o.stepup()
	before := o.calls()
	for _, b := range []string{`{"values":{"pm":"nonsense"}}`, `{"values":{"pm.max_children":"0"}}`, `{"values":{"pm.max_children":"999"}}`, `{"values":{"pm.max_children":"5\nx"}}`,
		`{"values":{"pm.process_idle_timeout":"30"}}`, `{"values":{"pm.max_requests":"100001"}}`, `{"values":{"pm.listen":"x"}}`, `{"values":{"listen":"0.0.0.0:9000"}}`} {
		if c, r := o.post("/api/php/8.4/ini", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, r)
		}
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil: %s", o.calls())
	}
	c, b := o.post("/api/php/8.4/ini", `{"values":{"pm":"dynamic","pm.max_children":"20","pm.start_servers":"3","memory_limit":"1G"}}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[ini][set][8.4][memory_limit=1G][pm=dynamic][pm.max_children=20][pm.start_servers=3]") {
		t.Fatalf("%s", o.calls())
	}
}
