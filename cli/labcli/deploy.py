"""lab deploy: langkah deploy per situs (git, composer, node, build, restart)."""

import json
import re
import subprocess
from pathlib import Path

from labcli.core import (
    APPS_DIR,
    ENV,
    LABBIN,
    MISE,
    PROTECTED,
    ROOT,
    SITE_NAME_RE,
    SITES,
    die,
    sh,
    stream,
)
from labcli.phpenv import COMPOSER, php_binary, restart_fpm
from labcli.projects import meta_of, mise_x


def site_conf(name):
    if not SITE_NAME_RE.match(name or ""):
        die("nama situs tidak valid")
    if name in PROTECTED:
        die(f"situs sistem '{name}' tidak bisa di-deploy dari sini")
    p = SITES / f"{name}.conf"
    if not p.exists():
        die(f"situs '{name}' tidak ada atau sedang dijeda")
    return p.read_text()


def project(name):
    """Folder proyek dari `root` nginx (satu tingkat di atas public/), harus di bawah ~/MacPanel/wwwroot. Versi PHP dari socket FPM."""
    t = site_conf(name)
    if meta_of(APPS_DIR / name):
        return (APPS_DIR / name).resolve(), None  # proyek Go/Python/Next.js
    m = re.search(r"^\s*root\s+([^;\s]+);", t, re.MULTILINE)
    if not m:
        die("situs ini tidak punya root (bukan situs PHP/statis)")
    root = Path(m.group(1)).resolve()
    d = root.parent if root.name == "public" else root
    base = (ROOT / "wwwroot").resolve()
    if base not in d.parents:
        die("folder proyek harus di bawah ~/MacPanel/wwwroot")
    if not d.is_dir():
        die("folder proyek tidak ada")
    v = re.search(r"php-fpm-dev-(8\.\d)\.sock", t)
    return d, (v.group(1) if v else None)


def node_pm(d):
    if (d / "pnpm-lock.yaml").exists():
        return "pnpm"
    if (d / "package-lock.json").exists():
        return "npm"
    return None


def pkg_script(d, name):
    try:
        return name in (json.loads((d / "package.json").read_text()).get("scripts") or {})
    except (OSError, ValueError):
        return False


def available(d, v):
    s = []
    if (d / ".git").exists():
        s.append(("git_pull", "Tarik perubahan terbaru (git pull --ff-only)"))
    if (d / "composer.json").exists() and Path(COMPOSER).exists():
        s += [
            ("composer_install", "composer install"),
            ("composer_install_nodev", "composer install --no-dev -o"),
            ("composer_update", "composer update"),
            ("dump_autoload", "composer dump-autoload -o"),
        ]
    if node_pm(d):
        s.append(("node_install", f"{node_pm(d)} install (sesuai lockfile)"))
    if node_pm(d) and pkg_script(d, "build"):
        s.append(("node_build", f"{node_pm(d)} run build"))
    if (d / "artisan").exists() and v:
        s += [("artisan_migrate", "php artisan migrate --force"), ("artisan_optimize", "php artisan optimize")]
    if v:
        s.append(("fpm_restart", f"Mulai ulang FPM {v} (bersihkan opcache)"))
    m = meta_of(d)
    if m:
        if m["kind"] == "go":
            s.append(("go_build", "go build -o bin/app"))
        if m["kind"] == "python" and (d / "requirements.txt").exists():
            s.append(("pip_install", "pip install -r requirements.txt"))
        s.append(("app_restart", "Mulai ulang aplikasi (worker)"))
    return s


def deploy_info(name):
    d, v = project(name)
    info = dict(site=name, dir=str(d), php=v, steps=[dict(id=i, label=l) for i, l in available(d, v)], git=None)
    if (d / ".git").exists():
        g = lambda *x: sh(["/usr/bin/git", "-C", str(d), *x]).stdout.strip()
        info["git"] = dict(branch=g("rev-parse", "--abbrev-ref", "HEAD"), commit=g("log", "-1", "--format=%h %s")[:120], dirty=bool(g("status", "--porcelain")), remote=bool(g("remote")))
    print(json.dumps(info))


def run_step(step, d, v):
    genv = dict(ENV, GIT_TERMINAL_PROMPT="0", GIT_SSH_COMMAND="ssh -o BatchMode=yes", COMPOSER_NO_INTERACTION="1", COMPOSER_ALLOW_SUPERUSER="0")
    phpx = php_binary(v) if v else "/opt/homebrew/bin/php"
    pm = node_pm(d)
    mt = meta_of(d)
    cmds = {
        "git_pull": ["/usr/bin/git", "-C", str(d), "pull", "--ff-only"],
        "composer_install": [phpx, COMPOSER, "install", "--no-interaction", "--prefer-dist"],
        "composer_install_nodev": [phpx, COMPOSER, "install", "--no-interaction", "--prefer-dist", "--no-dev", "-o"],
        "composer_update": [phpx, COMPOSER, "update", "--no-interaction"],
        "dump_autoload": [phpx, COMPOSER, "dump-autoload", "-o", "--no-interaction"],
        "validate": [phpx, COMPOSER, "validate", "--no-interaction"],
        "artisan_migrate": [phpx, "artisan", "migrate", "--force"],
        "artisan_optimize": [phpx, "artisan", "optimize"],
    }
    if pm == "pnpm":
        cmds["node_install"] = [MISE, "exec", "--", "pnpm", "install", "--frozen-lockfile"]
        cmds["node_build"] = [MISE, "exec", "--", "pnpm", "run", "build"]
    if pm == "npm":
        cmds["node_install"] = [MISE, "exec", "--", "npm", "ci"]
        cmds["node_build"] = [MISE, "exec", "--", "npm", "run", "build"]
    if mt:
        rv = mt["runtime"]
        if mt["kind"] == "go":
            cmds["go_build"] = mise_x("go", rv, "go", "build", "-o", "bin/app", ".")
        if mt["kind"] == "python":
            cmds["pip_install"] = [str(d / ".venv/bin/python"), "-m", "pip", "install", "-r", "requirements.txt"]
        if mt["kind"] == "nextjs" and pm:
            args = {"pnpm": [["pnpm", "install", "--frozen-lockfile"], ["pnpm", "run", "build"]], "npm": [["npm", "ci"], ["npm", "run", "build"]]}[pm]
            cmds["node_install"], cmds["node_build"] = mise_x("nextjs", rv, *args[0]), mise_x("nextjs", rv, *args[1])
    print(f"== {step} ==", flush=True)
    if step == "app_restart":
        if not mt:
            print("bukan proyek aplikasi")
            return False
        r = sh([LABBIN, "worker", "restart", d.name])
        print((r.stdout + r.stderr).strip())
        return r.returncode == 0
    if step == "fpm_restart":
        if not v:
            print("situs ini tidak memakai FPM")
            return False
        ok = restart_fpm(v)
        print("FPM dimulai ulang" if ok else "FPM tidak menjawab")
        return ok
    if step not in cmds:
        print(f"langkah tidak tersedia untuk proyek ini: {step}")
        return False
    if step in ("git_pull",) and not (d / ".git").exists():
        print("bukan repo git")
        return False
    rc = stream(cmds[step], cwd=str(d), env=genv, stdin=subprocess.DEVNULL)
    print(f"-- {step}: {'ok' if rc == 0 else 'gagal (kode ' + str(rc) + ')'}", flush=True)
    return rc == 0


def deploy(a):
    if a.act == "info":
        return deploy_info(a.site)
    d, v = project(a.site)
    ok_ids = {i for i, _ in available(d, v)}
    steps = [s for s in a.steps.split(",") if s]
    if not steps or len(steps) > 8:
        die("pilih 1-8 langkah")
    for s in steps:
        if s not in ok_ids:
            die(f"langkah '{s}' tidak tersedia untuk proyek ini (tersedia: {', '.join(sorted(ok_ids)) or '-'})")
    for s in steps:
        if not run_step(s, d, v):
            die(f"berhenti di langkah '{s}'")
    print("OK: deploy selesai")
