package docker

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

var dockerNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,100}$`)

var dockerRefRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._/:@-]{0,200}$`)

var dockerProjRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,60}$`)

func DockerOverview(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"docker", "overview"}, 40e9, "gagal membaca Docker")
}

func DockerStats(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"docker", "stats"}, 40e9, "Docker belum berjalan")
}

func dockerLogN(r *http.Request) string {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n < 1 || n > 500 {
		n = 100
	}
	return strconv.Itoa(n)
}

func DockerLogs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	id := r.PathValue("id")
	if !dockerNameRE.MatchString(id) {
		core.WriteError(w, 400, "nama container tidak valid")
		return
	}
	res := a.RunLabTimeout([]string{"docker", "container", "logs", id, "-n", dockerLogN(r)}, "", 30e9)
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca log container")
		return
	}
	core.WriteJSON(w, 200, map[string]string{"log": strings.TrimSpace(core.Scrub(res.Out))})
}

func DockerContainer(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	id, act := r.PathValue("id"), r.PathValue("action")
	if !dockerNameRE.MatchString(id) {
		core.WriteError(w, 400, "nama container tidak valid")
		return
	}
	if act != "start" && act != "stop" && act != "restart" && act != "remove" {
		core.WriteError(w, 400, "aksi: start, stop, restart, remove")
		return
	}
	var in struct{ Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if act == "remove" {
		if in.Confirm != id {
			core.WriteError(w, 400, "ketik ulang nama container untuk konfirmasi")
			return
		}
		if !a.RequireStepUp(w, ss) {
			return
		}
	}
	a.Record(w, ss, a.ClientIP(r), "docker_"+act, id, a.Mutate([]string{"docker", "container", act, id}, ""))
}

func DockerPull(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Ref string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !dockerRefRE.MatchString(in.Ref) {
		core.WriteError(w, 400, "referensi image tidak valid (mis. nginx:1.27 atau postgres:17)")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "docker_pull", in.Ref, []string{"docker", "image", "pull", in.Ref}, ""))
}

func DockerImageRemove(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{ Ref, Confirm string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !dockerRefRE.MatchString(in.Ref) {
		core.WriteError(w, 400, "referensi image tidak valid")
		return
	}
	if in.Confirm != in.Ref {
		core.WriteError(w, 400, "ketik ulang image untuk konfirmasi")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "docker_image_remove", in.Ref, a.Mutate([]string{"docker", "image", "remove", in.Ref}, ""))
}

func DockerPrune(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	a.Record(w, ss, a.ClientIP(r), "docker_image_prune", "", a.Mutate([]string{"docker", "image", "prune"}, ""))
}

func dockerProj(w http.ResponseWriter, r *http.Request) (string, bool) {
	p := r.PathValue("project")
	if !dockerProjRE.MatchString(p) {
		core.WriteError(w, 400, "nama proyek tidak valid")
		return "", false
	}
	return p, true
}

func DockerComposeAction(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	p, ok := dockerProj(w, r)
	if !ok {
		return
	}
	act := r.PathValue("action")
	if act != "up" && act != "down" {
		core.WriteError(w, 400, "aksi: up atau down")
		return
	}
	var in struct{}
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "docker_compose_"+act, p, []string{"docker", "compose", act, p}, ""))
}

func DockerComposePS(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	p, ok := dockerProj(w, r)
	if !ok {
		return
	}
	core.LabJSON(a, w, []string{"docker", "compose", "ps", p}, 40e9, "gagal membaca proyek compose")
}

func DockerComposeLogs(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	p, ok := dockerProj(w, r)
	if !ok {
		return
	}
	res := a.RunLabTimeout([]string{"docker", "compose", "logs", p, "-n", dockerLogN(r)}, "", 40e9)
	if res.Code != 0 {
		core.WriteError(w, 502, "gagal membaca log proyek")
		return
	}
	core.WriteJSON(w, 200, map[string]string{"log": strings.TrimSpace(core.Scrub(res.Out))})
}

func SysDocker(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.LabJSON(a, w, []string{"sys", "docker"}, 30e9, "gagal membaca Docker")
}
