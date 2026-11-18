"""Fondasi bersama: jalur, lingkungan, die/sh/stream/lab, validator situs. Tidak mengimpor modul domain."""

import os
import re
import subprocess
import sys
from pathlib import Path

HOME = Path.home()


ROOT = Path(os.environ.get("MACPANEL_HOME") or (HOME / "MacPanel"))


OPS = ROOT / "server"


BREW = "/opt/homebrew/bin/brew"


MISE = "/opt/homebrew/bin/mise"


ENV = dict(os.environ, PATH="/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin", HOMEBREW_NO_AUTO_UPDATE="1", HOMEBREW_CURL_RETRIES="5")


PHPS = ["8.2", "8.3", "8.4", "8.5"]


SITES = OPS / "nginx/sites"


PROTECTED = {"home", "nginx", "db", "kuma", "stats", "hello", "panel"}


def formula(v):
    return "php" if v == "8.5" else f"php@{v}"


def du_kb(path, timeout=40):
    try:
        r = subprocess.run(["/usr/bin/du", "-sk", str(path)], capture_output=True, text=True, timeout=timeout)
        return int(r.stdout.split()[0]) if r.stdout.strip() else None
    except (subprocess.TimeoutExpired, ValueError, IndexError):
        return None


APPS_DIR = ROOT / "wwwroot"


LABBIN = str(OPS / "scripts/lab")


SITE_NAME_RE = re.compile(r"^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$")


def lab(*args, inp=None, timeout=900):
    r = subprocess.run([LABBIN, *args], env=ENV, text=True, capture_output=True, input=inp, timeout=timeout)
    return r


def need(r, what):
    if r.returncode:
        die(f"{what} gagal: " + ((r.stderr or r.stdout).strip()[-300:] or "tanpa pesan"))
    return r.stdout


def site_exists(n):
    return (OPS / "nginx/sites" / f"{n}.conf").exists() or (OPS / "nginx/sites" / f"{n}.conf.disabled").exists()


UID = os.getuid()


def die(m, code=1):
    print("lab:", m, file=sys.stderr)
    sys.exit(code)


def sh(cmd, check=False, **kw):
    """Jalankan perintah (list, tanpa shell). Mengembalikan CompletedProcess; check=True menghentikan program bila gagal."""
    env = kw.pop("env", ENV)
    capture = kw.pop("capture", True)
    r = subprocess.run(cmd, env=env, text=True, capture_output=capture, **kw)
    if check and r.returncode:
        die(f"{' '.join(cmd[:3])} gagal: {(r.stderr or r.stdout).strip()[-300:]}")
    return r


def stream(cmd, cwd=None, env=None, stdin=None):
    """Jalankan perintah dengan keluaran langsung ke terminal; mengembalikan kode keluar."""
    return subprocess.run(cmd, env=env or ENV, cwd=cwd, stdin=stdin).returncode
