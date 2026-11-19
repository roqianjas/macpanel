"""Proyek aplikasi Go, Python, dan Next.js (worker launchd + situs proxy)."""

import json
import re
import shutil
import socket
import subprocess
import time
from pathlib import Path

from labcli.core import (
    APPS_DIR,
    ENV,
    LABBIN,
    MISE,
    SITES,
    die,
    lab,
    need,
    sh,
    site_exists,
    stream,
)

META = ".macpanel.json"


PROJ_KINDS = ("go", "python", "nextjs")


RT_TOOL = {"go": "go", "python": "python", "nextjs": "node"}


RT_RE = re.compile(r"^\d{1,3}(\.\d{1,3}){0,2}$")


def meta_of(d):
    try:
        return json.loads((Path(d) / META).read_text())
    except (OSError, ValueError):
        return None


def free_port():
    used = set()
    for f in SITES.glob("*.conf*"):
        used |= {int(x) for x in re.findall(r"proxy_pass http://127\.0\.0\.1:(\d+)", f.read_text(errors="replace"))}
    for p in range(3100, 4000):
        if p in used:
            continue
        s = socket.socket()
        try:
            s.bind(("127.0.0.1", p))
            s.close()
            return p
        except OSError:
            s.close()
    die("tidak ada port kosong di 3100-3999")


def runtime_version(kind, want):
    tool = RT_TOOL[kind]
    if want:
        if not RT_RE.match(want):
            die("versi runtime tidak valid (contoh 24 atau 24.22.3)")
        r = sh([MISE, "where", f"{tool}@{want}"])
        if r.returncode:
            die(f"{tool} {want} belum terpasang (pasang di menu PHP dan runtime)")
        return want
    r = sh([MISE, "current", tool])
    v = r.stdout.strip().split()[0] if r.returncode == 0 and r.stdout.strip() else ""
    if not v:
        die(f"{tool} belum terpasang (pasang di menu PHP dan runtime)")
    return v


GO_MAIN = """package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	host, port := os.Getenv("HOST"), os.Getenv("PORT")
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Halo dari %s (Go)\\n", "{name}")
	})
	log.Printf("mendengarkan di %s:%s", host, port)
	log.Fatal(http.ListenAndServe(host+":"+port, nil))
}
"""


PY_APP = """import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class H(BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"Halo dari {name} (Python)\\n"
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


if __name__ == "__main__":
    host, port = os.environ.get("HOST", "127.0.0.1"), int(os.environ.get("PORT", "8080"))
    print(f"mendengarkan di {host}:{port}", flush=True)
    ThreadingHTTPServer((host, port), H).serve_forever()
"""


def mise_x(kind, rv, *cmd):
    return [MISE, "exec", f"{RT_TOOL[kind]}@{rv}", "--", *cmd]


def proxy_env(env):
    """Salinan env dengan registry diarahkan ke proxy resume bila diaktifkan dan sehat (lab proxy enable)."""
    from labcli.regproxy import env_overrides

    return {**(env or {}), **env_overrides()}


def wait_http(port, secs=60):
    import urllib.request

    for _ in range(secs * 2):
        try:
            with urllib.request.urlopen(f"http://127.0.0.1:{port}/", timeout=2) as r:
                return r.status
        except Exception:
            time.sleep(0.5)
    return None


def project_cmds(kind, n, dest, rv, port):
    """(perintah build, perintah jalan) untuk worker."""
    if kind == "go":
        return mise_x("go", rv, "go", "build", "-o", "bin/app", "."), f"{dest}/bin/app"
    if kind == "python":
        return None, f"{dest}/.venv/bin/python app.py"
    return mise_x("nextjs", rv, "pnpm", "run", "build"), " ".join([MISE, "exec", f"node@{rv}", "--", "pnpm", "start", "-p", str(port), "-H", "127.0.0.1"])


def app_project(a):
    n, kind = a.name, a.kind
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,28}[a-z0-9]", n or ""):
        die("nama: huruf kecil, angka, tanda hubung; diawali huruf (maks 30)")
    if n in ("home", "panel", "db", "nginx", "kuma", "stats", "dev", "mysql", "postgres", "root", "mariadb"):
        die("nama itu dicadangkan")
    dest = APPS_DIR / n
    if dest.exists() or site_exists(n):
        die("nama sudah dipakai (folder atau situs)")
    if sh([LABBIN, "worker", "status", n]).returncode == 0:
        die("sudah ada worker bernama itu")
    rv = runtime_version(kind, a.runtime)
    if kind == "nextjs" and sh([MISE, "where", "pnpm"]).returncode:
        die("pnpm belum terpasang (menu PHP dan runtime)")
    port = a.port or free_port()
    if a.port:
        s = socket.socket()
        try:
            s.bind(("127.0.0.1", port))
        except OSError:
            die(f"port {port} sedang dipakai")
        finally:
            s.close()
    build, run = project_cmds(kind, n, dest, rv, port)
    made_site = made_worker = False
    try:
        APPS_DIR.mkdir(parents=True, exist_ok=True)
        dest.mkdir()
        env = proxy_env(dict(ENV, CI="1", GIT_TERMINAL_PROMPT="0"))
        print(f"== siapkan proyek {kind} ({RT_TOOL[kind]} {rv}) ==", flush=True)
        if kind == "go":
            (dest / "go.mod").write_text(f"module {n.replace('-', '_')}\n\ngo {'.'.join(rv.split('.')[:2])}\n")
            (dest / "main.go").write_text(GO_MAIN.replace("{name}", n))
        elif kind == "python":
            (dest / "app.py").write_text(PY_APP.replace("{name}", n))
            (dest / "requirements.txt").write_text("# dependensi proyek (pip install -r requirements.txt dari langkah Deploy)\n")
            if stream(mise_x("python", rv, "python", "-m", "venv", ".venv"), cwd=str(dest), env=env):
                die("membuat venv gagal")
        else:
            print("== create-next-app (butuh internet, beberapa menit) ==", flush=True)
            cmd = mise_x(
                "nextjs",
                rv,
                "pnpm",
                "create",
                "next-app@latest",
                ".",
                "--yes",
                "--ts",
                "--app",
                "--eslint",
                "--no-tailwind",
                "--no-src-dir",
                "--import-alias",
                "@/*",
                "--use-pnpm",
                "--disable-git",
                "--skip-install",
            )
            for attempt in range(3):
                if not stream(cmd, cwd=str(dest), env=env, stdin=subprocess.DEVNULL):
                    break
                if attempt == 2:
                    die("create-next-app gagal setelah 3 percobaan (cek koneksi internet)")
                print(f"percobaan {attempt + 1} gagal, mengulang…", flush=True)
                time.sleep(4)
            print("== pnpm install ==", flush=True)
            ok = False
            pi = mise_x("nextjs", rv, "pnpm", "install", "--network-concurrency", "2", "--fetch-timeout", "180000")
            if not stream(pi, cwd=str(dest), env=env, stdin=subprocess.DEVNULL) and (dest / "node_modules/.bin/next").exists():
                ok = True
            for attempt in range(3):  # jaringan putus-putus: npm lebih tahan banting dan memakai cache sendiri
                if ok:
                    break
                print(f"== pnpm gagal; mencoba npm install (percobaan {attempt + 1}/3) ==", flush=True)
                if (
                    not stream(
                        mise_x("nextjs", rv, "npm", "install", "--no-audit", "--no-fund", "--fetch-retries=10", "--fetch-retry-mintimeout=2000", "--maxsockets=3"),
                        cwd=str(dest),
                        env=env,
                        stdin=subprocess.DEVNULL,
                    )
                    and (dest / "node_modules/.bin/next").exists()
                ):
                    ok = True
                else:
                    time.sleep(5)
            if not ok:
                die("install dependensi gagal (cek koneksi internet)")
        if build:
            print("== build ==", flush=True)
            if stream(build, cwd=str(dest), env=env, stdin=subprocess.DEVNULL):
                die("build gagal")
        (dest / META).write_text(json.dumps(dict(kind=kind, port=port, runtime=rv, run=run), indent=1) + "\n")
        print("== jalankan sebagai worker (launchd) ==", flush=True)
        wargs = ["worker", "add", n, "--dir", str(dest), "--cmd", run, "--env", f"PORT={port}", "--env", "HOST=127.0.0.1"]
        if kind == "nextjs":
            wargs += ["--env", "NODE_ENV=production"]
        need(lab(*wargs), "worker")
        made_worker = True
        code = wait_http(port)
        if code is None:
            die(f"aplikasi tidak menjawab di 127.0.0.1:{port}; lihat log: lab worker logs {n}")
        print(f"aplikasi menjawab HTTP {code} di 127.0.0.1:{port}", flush=True)
        print("== daftarkan situs ==", flush=True)
        need(lab("site", "add", n, str(port)), "tambah situs")
        made_site = True
    except SystemExit:
        if made_site:
            lab("site", "remove", n)
        if made_worker:
            lab("worker", "remove", n)
        shutil.rmtree(dest, ignore_errors=True)
        print("dibatalkan: folder, worker, dan situs yang baru dibuat dibersihkan")
        raise
    print(f"OK: {kind} terpasang di {dest}, berjalan di 127.0.0.1:{port}. Buka https://{n}.lab. Mulai ulang: lab worker restart {n}")
