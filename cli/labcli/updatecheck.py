"""Klasifikasi dan deteksi pembaruan untuk `lab update check --json`.

Satu daftar datar dengan `kind`, supaya panel bisa memisahkannya menjadi tab:
  runtime : PHP, Python/Node/Go dari Homebrew, dan alat mise (node, go, python, pnpm)
  service : layanan yang dijalankan Homebrew (nginx, PostgreSQL, MySQL, Redis, Apache, cloudflared, dnsmasq, ...)
  app     : aplikasi npm di server/apps/* (mis. 9Router)
  tool    : alat CLI yang Anda pasang sendiri (gh, composer, ...)
  lib     : pustaka dependensi yang dipasang Homebrew otomatis (bukan pasangan langsung)
  panel   : MacPanel sendiri (commit baru di origin/main)

Fungsi di sini tidak menyentuh disk atau jaringan, kecuali yang namanya diawali `fetch_` atau `read_`.
"""

import json
import re
import subprocess
from pathlib import Path

RUNTIME_BREW = re.compile(r"^(php(@\d+\.\d+)?|python@[\d.]+|node(@\d+)?|go|openjdk(@\d+)?)$")
SERVICE_BREW = re.compile(r"^(nginx|httpd|redis|dnsmasq|cloudflared|mailpit|netdata|tailscale|postgresql(@\d+)?|mysql(@[\d.]+)?|mariadb(@[\d.]+)?)$")
KINDS = ("runtime", "service", "app", "tool", "lib", "panel")
NPM_NAME = re.compile(r"^[a-z0-9][a-z0-9._-]{0,59}$")


def json_from(text):
    """JSON dari keluaran yang mungkin diawali baris lain (Homebrew kadang mencetak '==> Downloading ...' ke stdout)."""
    i = (text or "").find("{")
    if i < 0:
        return {}
    try:
        return json.loads(text[i:])
    except ValueError:
        return {}


def is_major(cur, new):
    """Berganti versi mayor. Untuk 0.x, perubahan minor juga dianggap berisiko (semver 0.x tidak menjanjikan kompatibilitas)."""
    a, b = str(cur).split("."), str(new).split(".")
    if a[0] != b[0]:
        return True
    return a[0] == "0" and len(a) > 1 and len(b) > 1 and a[1] != b[1]


def series(v):
    """Seri rilis (mayor.minor). Runtime seperti PHP/Python/Node berganti seri = versi baru yang bisa merusak proyek, bukan patch."""
    return tuple(str(v).split(".")[:2])


def classify_brew(name, services, leaves):
    if RUNTIME_BREW.match(name):
        return "runtime"
    if SERVICE_BREW.match(name) or name in services:
        return "service"
    return "tool" if name in leaves else "lib"


def brew_items(outdated, pins, services, leaves):
    out = []
    for f in outdated.get("formulae", []):
        n = f["name"]
        cur, new = f["installed_versions"][0], f["current_version"]
        kind = classify_brew(n, services, leaves)
        major = is_major(cur, new) or (kind == "runtime" and series(cur) != series(new))
        pinned = n in pins
        note = "major" if major else ("pinned" if pinned else ("bulk" if kind == "lib" else ("restart" if kind == "service" else "")))
        out.append(dict(kind=kind, source="homebrew", name=n, **{"from": cur, "to": new}, pinned=pinned, major=major, upgradable=not major and kind != "lib", target=n, note=note))
    return out


def mise_items(outdated, tools):
    out = []
    for k, v in (outdated or {}).items():
        if k not in tools:
            continue
        cur, new = str(v.get("current")), str(v.get("latest"))
        major = is_major(cur, new)
        out.append(dict(kind="runtime", source="mise", name=k, **{"from": cur, "to": new}, pinned=False, major=major, upgradable=not major, target=f"mise:{k}", note="major" if major else ""))
    return out


def npm_items(app, outdated, upgradable_apps):
    """app = nama folder di server/apps; outdated = keluaran `npm outdated --json` di folder itu."""
    out = []
    for name, v in (outdated or {}).items():
        cur, new = v.get("current"), v.get("latest")
        if not cur or not new or cur == new:
            continue
        major = is_major(cur, new)
        can = name in upgradable_apps and not major
        out.append(
            dict(
                kind="app",
                source="npm",
                name=name,
                **{"from": cur, "to": new},
                pinned=False,
                major=major,
                upgradable=can,
                target=f"npm:{name}",
                note="major" if major else ("restart" if can else "manual"),
            )
        )
    return out


def panel_item(head, remote, behind):
    if not behind or head == remote:
        return []
    return [dict(kind="panel", source="git", name="macpanel", **{"from": head, "to": remote}, pinned=False, major=False, upgradable=False, target="panel", note="deploy", behind=behind)]


def summarize(items):
    c = {k: 0 for k in KINDS}
    for i in items:
        c[i["kind"]] = c.get(i["kind"], 0) + 1
    return c


def read_npm_outdated(npm, folder, timeout=45):
    """`npm outdated --json` keluar dengan kode 1 bila ada yang usang; itu normal. None bila gagal diukur."""
    try:
        r = subprocess.run([str(npm), "outdated", "--json"], cwd=str(folder), capture_output=True, text=True, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired):
        return None
    if r.returncode not in (0, 1):
        return None
    return json_from(r.stdout)


def read_npm_apps(apps_dir):
    d = Path(apps_dir)
    if not d.is_dir():
        return []
    return sorted(p for p in d.iterdir() if (p / "package.json").is_file() and not p.is_symlink())


def fetch_panel(repo, timeout=25):
    """(head, remote, behind) atau None bila bukan repo git atau fetch gagal."""
    repo = str(repo)

    def git(*a):
        return subprocess.run(["/usr/bin/git", "-C", repo, *a], capture_output=True, text=True, timeout=timeout)

    try:
        if git("rev-parse", "--is-inside-work-tree").stdout.strip() != "true":
            return None
        if git("fetch", "-q", "origin").returncode != 0:
            return None
        head = git("rev-parse", "--short", "HEAD").stdout.strip()
        remote = git("rev-parse", "--short", "origin/main").stdout.strip()
        behind = int(git("rev-list", "--count", "HEAD..origin/main").stdout.strip() or 0)
    except (OSError, subprocess.TimeoutExpired, ValueError):
        return None
    return head, remote, behind
