package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Docker (Colima): hanya mengelola, tanpa 'docker run' bebas. Kebijakan keamanan Compose ada di `lab docker compose up`.

var (
	dockerNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,100}$`)
	dockerRefRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9._/:@-]{0,200}$`)
	dockerProjRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,60}$`)
)

func hDockerOverview(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	labJSON(a, w, []string{"docker", "overview"}, 40e9, "gagal membaca Docker")
}

func hDockerStats(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	labJSON(a, w, []string{"docker", "stats"}, 40e9, "Docker belum berjalan")
}

func dockerLogN(r *http.Request) string {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n < 1 || n > 500 {
		n = 100
	}
	return strconv.Itoa(n)
}

func hDockerLogs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	id := r.PathValue("id")
	if !dockerNameRE.MatchString(id) {
		jerr(w, 400, "nama container tidak valid")
		return
	}
	res := a.runLabT([]string{"docker", "container", "logs", id, "-n", dockerLogN(r)}, "", 30e9)
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca log container")
		return
	}
	jsonOut(w, 200, map[string]string{"log": strings.TrimSpace(scrub(res.Out))})
}

func hDockerContainer(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	id, act := r.PathValue("id"), r.PathValue("action")
	if !dockerNameRE.MatchString(id) {
		jerr(w, 400, "nama container tidak valid")
		return
	}
	if act != "start" && act != "stop" && act != "restart" && act != "remove" {
		jerr(w, 400, "aksi: start, stop, restart, remove")
		return
	}
	var in struct{ Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if act == "remove" {
		if in.Confirm != id {
			jerr(w, 400, "ketik ulang nama container untuk konfirmasi")
			return
		}
		if !a.requireStepUp(w, ss) {
			return
		}
	}
	a.record(w, ss, a.clientIP(r), "docker_"+act, id, a.mutate([]string{"docker", "container", act, id}, ""))
}

func hDockerPull(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Ref string }
	if !decode(w, r, &in) {
		return
	}
	if !dockerRefRE.MatchString(in.Ref) {
		jerr(w, 400, "referensi image tidak valid (mis. nginx:1.27 atau postgres:17)")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "docker_pull", in.Ref, []string{"docker", "image", "pull", in.Ref}, ""))
}

func hDockerImageRemove(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{ Ref, Confirm string }
	if !decode(w, r, &in) {
		return
	}
	if !dockerRefRE.MatchString(in.Ref) {
		jerr(w, 400, "referensi image tidak valid")
		return
	}
	if in.Confirm != in.Ref {
		jerr(w, 400, "ketik ulang image untuk konfirmasi")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "docker_image_remove", in.Ref, a.mutate([]string{"docker", "image", "remove", in.Ref}, ""))
}

func hDockerPrune(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	a.record(w, ss, a.clientIP(r), "docker_image_prune", "", a.mutate([]string{"docker", "image", "prune"}, ""))
}

func dockerProj(w http.ResponseWriter, r *http.Request) (string, bool) {
	p := r.PathValue("project")
	if !dockerProjRE.MatchString(p) {
		jerr(w, 400, "nama proyek tidak valid")
		return "", false
	}
	return p, true
}

func hDockerComposeAction(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	p, ok := dockerProj(w, r)
	if !ok {
		return
	}
	act := r.PathValue("action")
	if act != "up" && act != "down" {
		jerr(w, 400, "aksi: up atau down")
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "docker_compose_"+act, p, []string{"docker", "compose", act, p}, ""))
}

func hDockerComposePS(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	p, ok := dockerProj(w, r)
	if !ok {
		return
	}
	labJSON(a, w, []string{"docker", "compose", "ps", p}, 40e9, "gagal membaca proyek compose")
}

func hDockerComposeLogs(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	p, ok := dockerProj(w, r)
	if !ok {
		return
	}
	res := a.runLabT([]string{"docker", "compose", "logs", p, "-n", dockerLogN(r)}, "", 40e9)
	if res.Code != 0 {
		jerr(w, 502, "gagal membaca log proyek")
		return
	}
	jsonOut(w, 200, map[string]string{"log": strings.TrimSpace(scrub(res.Out))})
}
