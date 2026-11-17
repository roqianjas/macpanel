package main

import (
	"strings"
	"testing"
	"time"
)

const fakeDev = `case "$1 $2" in
"ext list") printf '{"version":"8.4","builtin":["curl","mbstring"],"managed":[{"name":"redis","label":"Redis","installed":true,"enabled":true,"loaded":true,"zend":false},{"name":"EVIL<x>","label":"x","installed":false,"enabled":false,"loaded":false,"zend":false}]}';;
"ini get") printf '{"version":"8.4","keys":[{"key":"memory_limit","value":"512M","default":"128M","hint":"h"},{"key":"disable_functions","value":"x","default":"","hint":""}]}';;
"composer status") printf '{"installed":true,"version":"2.11.2","php":"8.5.12","outdated":false,"latest":""}';;
"deploy info") printf '{"site":"tokoku","dir":"/x","php":"8.4","steps":[{"id":"git_pull","label":"Tarik"}],"git":{"branch":"main","commit":"abc msg","dirty":false,"remote":true}}';;
"backup list") printf '{"backups":[{"name":"20261116-180701","at":1,"size":10,"files":["a"]}],"offsite":false}';;
*) echo "ok $1 $2";;
esac`

func newDevEnv(t *testing.T) *opsEnv {
	o := newOpsEnv(t, fakeDev)
	o.app.longTimeout = 20 * time.Second
	return o
}

func TestExtListOnlyKnownNames(t *testing.T) {
	o := newDevEnv(t)
	b := o.do("GET", "/api/php/8.4/ext", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, `"name":"redis"`) || strings.Contains(b, "EVIL") {
		t.Fatalf("nama tak dikenal harus dikosongkan: %s", b)
	}
	for _, p := range []string{"/api/php/7.4/ext", "/api/php/8.4;id/ext", "/api/php/8.44/ext"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("%s -> %d", p, w.Code)
		}
	}
}

func TestExtActionGuards(t *testing.T) {
	o := newDevEnv(t)
	before := o.calls()
	if c, _ := o.post("/api/php/8.4/ext/redis/install", `{}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	for _, p := range []string{"/api/php/8.4/ext/memcached/install", "/api/php/8.4/ext/redis/exec", "/api/php/8.4/ext/re;id/install", "/api/php/8.4/ext/-f/install", "/api/php/9.9/ext/redis/install", "/api/php/8.4/ext/REDIS/install"} {
		if c, _ := o.post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
	if c, _ := o.post("/api/php/8.4/ext/redis/install", `{"x":1}`); c != 400 {
		t.Errorf("field asing harus 400: %d", c)
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	for _, act := range []string{"install", "remove", "enable", "disable"} {
		c, b := o.post("/api/php/8.4/ext/redis/"+act, `{}`)
		if c != 202 {
			t.Fatalf("%s: %d %s", act, c, b)
		}
		o.waitJob(jobID(t, b))
		if !strings.Contains(o.calls(), "CALL:[ext]["+act+"][8.4][redis]") {
			t.Errorf("argv %s: %s", act, o.calls())
		}
	}
	o.now = o.now.Add(stepupTTL + time.Second)
	if c, _ := o.post("/api/php/8.4/ext/redis/remove", `{}`); c != 403 {
		t.Fatalf("step-up kedaluwarsa harus 403: %d", c)
	}
}

func TestIniGetFiltersUnknownKeys(t *testing.T) {
	o := newDevEnv(t)
	b := o.do("GET", "/api/php/8.4/ini", "", o.cookie, "", nil).Body.String()
	if !strings.Contains(b, "memory_limit") || strings.Contains(b, "disable_functions") {
		t.Fatalf("hanya kunci yang diizinkan: %s", b)
	}
}

func TestIniSetValidation(t *testing.T) {
	o := newDevEnv(t)
	if c, _ := o.post("/api/php/8.4/ini", `{"values":{"memory_limit":"256M"}}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.stepup()
	before := o.calls()
	bad := []string{
		`{"values":{"disable_functions":""}}`, `{"values":{"auto_prepend_file":"/etc/passwd"}}`, `{"values":{"memory_limit":"256M\nphp_admin_value[x]=1"}}`,
		`{"values":{"memory_limit":"abc"}}`, `{"values":{"memory_limit":"-2"}}`, `{"values":{"upload_max_filesize":"-1"}}`, `{"values":{"max_input_vars":"5"}}`,
		`{"values":{"display_errors":"On"}}`, `{"values":{"date.timezone":"Asia/Jakarta; x"}}`, `{"values":{"date.timezone":"../etc"}}`, `{"values":{}}`, `{}`,
		`{"values":{"memory_limit":"256M","extension":"x.so"}}`, `{"values":{"memory_limit":"99999999M"}}`,
	}
	for _, b := range bad {
		if c, resp := o.post("/api/php/8.4/ini", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, resp)
		}
	}
	if c, _ := o.post("/api/php/7.4/ini", `{"values":{"memory_limit":"256M"}}`); c != 400 {
		t.Errorf("versi buruk: %d", c)
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	c, b := o.post("/api/php/8.4/ini", `{"values":{"upload_max_filesize":"128M","memory_limit":"-1","date.timezone":"Asia/Jakarta"}}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[ini][set][8.4][date.timezone=Asia/Jakarta][memory_limit=-1][upload_max_filesize=128M]") {
		t.Fatalf("argv harus terurut dan utuh: %s", o.calls())
	}
}

func TestComposerEndpoints(t *testing.T) {
	o := newDevEnv(t)
	if b := o.do("GET", "/api/composer", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"version":"2.11.2"`) {
		t.Fatalf("%s", b)
	}
	if c, _ := o.post("/api/composer/update", `{}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.stepup()
	for _, p := range []string{"/api/composer/self-update", "/api/composer/remove", "/api/composer/x;id"} {
		if c, _ := o.post(p, `{}`); c != 400 && c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
	c, b := o.post("/api/composer/update", `{}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[composer][update]") {
		t.Fatalf("%s", o.calls())
	}
}

func TestDeployGuards(t *testing.T) {
	o := newDevEnv(t)
	if b := o.do("GET", "/api/sites/tokoku/deploy", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"git_pull"`) || !strings.Contains(b, `"branch":"main"`) {
		t.Fatalf("%s", b)
	}
	for _, p := range []string{"/api/sites/panel/deploy", "/api/sites/home/deploy", "/api/sites/A_B/deploy", "/api/sites/..%2Fx/deploy"} {
		if w := o.do("GET", p, "", o.cookie, "", nil); w.Code != 400 && w.Code != 404 {
			t.Errorf("GET %s -> %d", p, w.Code)
		}
	}
	if c, _ := o.post("/api/sites/tokoku/deploy", `{"steps":["git_pull"]}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.stepup()
	before := o.calls()
	for _, b := range []string{`{"steps":[]}`, `{"steps":["rm -rf"]}`, `{"steps":["git_pull","x;id"]}`, `{"steps":["git_pull,composer_install"]}`, `{"steps":["git_pull","git_pull","git_pull","git_pull","git_pull","git_pull","git_pull","git_pull","git_pull"]}`, `{}`} {
		if c, resp := o.post("/api/sites/tokoku/deploy", b); c != 400 {
			t.Errorf("%s -> %d %s", b, c, resp)
		}
	}
	if c, _ := o.post("/api/sites/panel/deploy", `{"steps":["git_pull"]}`); c != 400 {
		t.Errorf("situs inti harus 400: %d", c)
	}
	if o.calls() != before {
		t.Fatalf("lab terpanggil oleh input buruk: %s", o.calls())
	}
	c, b := o.post("/api/sites/tokoku/deploy", `{"steps":["git_pull","composer_install","artisan_migrate"]}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[deploy][run][tokoku][git_pull,composer_install,artisan_migrate]") {
		t.Fatalf("%s", o.calls())
	}
}

func TestBackupEndpoints(t *testing.T) {
	o := newDevEnv(t)
	if b := o.do("GET", "/api/backups", "", o.cookie, "", nil).Body.String(); !strings.Contains(b, `"offsite":false`) || !strings.Contains(b, "20261116-180701") {
		t.Fatalf("%s", b)
	}
	if c, _ := o.post("/api/backups/run", `{}`); c != 403 {
		t.Fatalf("tanpa step-up: %d", c)
	}
	o.stepup()
	c, b := o.post("/api/backups/run", `{}`)
	if c != 202 {
		t.Fatalf("%d %s", c, b)
	}
	o.waitJob(jobID(t, b))
	if !strings.Contains(o.calls(), "CALL:[backup][run]") {
		t.Fatalf("%s", o.calls())
	}
}
