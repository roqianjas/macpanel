package main

import (
	"strings"
	"testing"
	"time"
)

func TestInstallAppKinds(t *testing.T) {
	o := newOpsEnv(t, "")
	if c, _ := o.post("/api/installer", `{"kind":"go","name":"api-go"}`); c != 403 {
		t.Fatalf("tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	for _, bad := range []string{
		`{"kind":"ruby","name":"x-app"}`, `{"kind":"go","name":"1go"}`, `{"kind":"go","name":"Go App"}`, `{"kind":"go","name":"api-go","runtime":"24;rm"}`,
		`{"kind":"nextjs","name":"web","runtime":"../x"}`, `{"kind":"python","name":"svc","port":80}`, `{"kind":"python","name":"svc","port":9100}`, `{"kind":"python","name":"svc","port":3306}`, `{"kind":"python","name":"svc","port":9101}`, `{"kind":"go","name":"panel"}`,
	} {
		if c, b := o.post("/api/installer", bad); c != 400 {
			t.Fatalf("%s harus 400: %d %s", bad, c, b)
		}
	}
	if strings.Contains(o.calls(), "appinstall") {
		t.Fatal("lab dipanggil dengan input tak valid")
	}
	for _, tc := range []struct{ body, want string }{
		{`{"kind":"go","name":"api-go"}`, "[appinstall][go][api-go]"},
		{`{"kind":"python","name":"svc-py","runtime":"3.13","port":3500}`, "[appinstall][python][svc-py][--runtime][3.13][--port][3500]"},
		{`{"kind":"nextjs","name":"web","runtime":"24.22.3"}`, "[appinstall][nextjs][web][--runtime][24.22.3]"},
	} {
		if c, b := o.post("/api/installer", tc.body); c != 202 {
			t.Fatalf("%s: %d %s", tc.body, c, b)
		}
		deadline := time.Now().Add(3 * time.Second)
		for !strings.Contains(o.calls(), tc.want) && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if !strings.Contains(o.calls(), tc.want) {
			t.Fatalf("argumen salah, harus memuat %s: %s", tc.want, o.calls())
		}
	}
	if strings.Contains(o.calls(), "[--php]") && strings.Contains(o.calls(), "[go][api-go][--php]") {
		t.Fatal("proyek aplikasi tidak boleh membawa --php")
	}
}

func TestDockerEndpoints(t *testing.T) {
	o := newOpsEnv(t, "")
	for _, bad := range []string{"/api/docker/containers/a;b/start", "/api/docker/containers/-x/start", "/api/docker/containers/web/exec", "/api/docker/containers/web/attach"} {
		if c, _ := o.post(bad, `{}`); c != 400 && c != 404 {
			t.Fatalf("%s harus ditolak: %d", bad, c)
		}
	}
	if strings.Contains(o.calls(), "docker") {
		t.Fatal("lab dipanggil dengan input tak valid")
	}
	if c, _ := o.post("/api/docker/containers/web/start", `{}`); c != 200 {
		t.Fatalf("start tanpa step-up harus boleh: %d", c)
	}
	if c, _ := o.post("/api/docker/containers/web/remove", `{"confirm":"web"}`); c != 403 {
		t.Fatalf("hapus tanpa step-up harus 403: %d", c)
	}
	o.stepup()
	if c, _ := o.post("/api/docker/containers/web/remove", `{"confirm":"salah"}`); c != 400 {
		t.Fatalf("hapus tanpa ketik ulang nama harus 400: %d", c)
	}
	for _, bad := range []string{`{"ref":"NGINX"}`, `{"ref":"a b"}`, `{"ref":"x;rm -rf /"}`, `{"ref":"--privileged"}`, `{"ref":""}`} {
		if c, _ := o.post("/api/docker/images/pull", bad); c != 400 {
			t.Fatalf("pull %s harus 400: %d", bad, c)
		}
	}
	if c, _ := o.post("/api/docker/images/pull", `{"ref":"nginx:1.27"}`); c != 202 {
		t.Fatalf("pull sah: %d", c)
	}
	if c, _ := o.post("/api/docker/images/remove", `{"ref":"nginx:1.27","confirm":"x"}`); c != 400 {
		t.Fatalf("hapus image tanpa konfirmasi: %d", c)
	}
	if c, _ := o.post("/api/docker/compose/..%2Fetc/up", `{}`); c != 400 && c != 404 {
		t.Fatalf("proyek traversal harus ditolak: %d", c)
	}
	if c, _ := o.post("/api/docker/compose/Web/up", `{}`); c != 400 {
		t.Fatalf("nama huruf besar harus 400: %d", c)
	}
	if c, _ := o.post("/api/docker/compose/web/exec", `{}`); c != 400 {
		t.Fatalf("aksi compose liar harus 400: %d", c)
	}
	if c, _ := o.post("/api/docker/compose/web/up", `{}`); c != 202 {
		t.Fatalf("compose up sah: %d", c)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(o.calls(), "[docker][compose][up][web]") && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(o.calls(), "[docker][compose][up][web]") {
		t.Fatalf("argumen compose salah: %s", o.calls())
	}
}
