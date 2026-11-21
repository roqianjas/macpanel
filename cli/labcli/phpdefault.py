"""lab php default|which|status|shims: versi PHP untuk CLI (terminal) dan ringkasan versi web per situs.

Web: tiap situs memilih versi lewat soket FPM di config nginx-nya (php-fpm-dev-X.Y.sock).
CLI: perintah `php` di terminal lewat shim kecil yang memilih versi dengan urutan
  1. berkas .php-version di folder ini atau di atasnya,
  2. situs MacPanel yang foldernya mencakup folder ini (versi sama dengan web),
  3. default global (~/.config/macminiops/php-default).
Satu-satunya sumber aturan itu adalah skrip shim; `lab php which` hanya menjalankannya dalam mode cetak.
"""

import json
import os
import re
import stat
import subprocess
import time

from labcli.core import HOME, OPS, PHPS, ROOT, SITES, die
from labcli.phpenv import php_home

SHIM_DIR = OPS / "shims"
DEFAULT_FILE = HOME / ".config/macminiops/php-default"
ZSHRC = HOME / ".zshrc"
BEGIN, END = "# >>> macpanel php shims >>>", "# <<< macpanel php shims <<<"
VER_RE = re.compile(r"^8\.[2-5]$")

SHIM = r"""#!/bin/sh
# Dibangkitkan oleh `lab php shims`. Jangan diedit; perubahan ditimpa.
# Memilih versi PHP: .php-version (folder ini atau di atasnya) > situs MacPanel pada folder ini > default global.
# Mode cetak (untuk `lab php which` dan tes): MACPANEL_PHP_SHIM_PRINT=1 mencetak "versi<TAB>alasan<TAB>biner" lalu keluar.
ROOT="${MACPANEL_HOME:-$HOME/MacPanel}"
PREFIX="${MACPANEL_PHP_PREFIX:-/opt/homebrew/opt}"
DEFAULT_FILE="${MACPANEL_PHP_DEFAULT_FILE:-$HOME/.config/macminiops/php-default}"
FORCE="__FORCE__"

bin_of() {
  case "$1" in
    8.5) p="$PREFIX/php/bin/php" ;;
    8.[2-4]) p="$PREFIX/php@$1/bin/php" ;;
    *) return 1 ;;
  esac
  [ -x "$p" ] && printf '%s' "$p"
}

pick() {
  d="$PWD"
  while :; do
    if [ -f "$d/.php-version" ]; then
      v=$(head -n 1 "$d/.php-version" 2>/dev/null | tr -d ' \r\t')
      if b=$(bin_of "$v"); then printf '%s\t.php-version (%s)\t%s\n' "$v" "$d/.php-version" "$b"; return 0; fi
    fi
    [ "$d" = "/" ] && break
    d=$(dirname "$d")
  done
  v=$(grep -HE '^[[:space:]]*root[[:space:]]|php-fpm-dev-[0-9.]+\.sock' "$ROOT"/server/nginx/sites/*.conf 2>/dev/null | awk -v cwd="$PWD" '
    { i = index($0, ":"); f = substr($0, 1, i - 1); l = substr($0, i + 1) }
    l ~ /^[ \t]*root[ \t]/ { r = l; sub(/^[ \t]*root[ \t]+/, "", r); sub(/;.*$/, "", r); sub(/\/public$/, "", r); root[f] = r }
    match(l, /php-fpm-dev-[0-9.]+\.sock/) { ver[f] = substr(l, RSTART + 12, RLENGTH - 17) }
    END { for (f in root) if (ver[f] != "" && (cwd == root[f] || index(cwd, root[f] "/") == 1)) { print ver[f] "\t" f; exit } }')
  if [ -n "$v" ]; then
    site=$(printf '%s' "$v" | cut -f2); v=$(printf '%s' "$v" | cut -f1)
    if b=$(bin_of "$v"); then printf '%s\tsitus %s\t%s\n' "$v" "$(basename "$site" .conf)" "$b"; return 0; fi
  fi
  v=$(head -n 1 "$DEFAULT_FILE" 2>/dev/null | tr -d ' \r\t')
  if b=$(bin_of "$v"); then printf '%s\tdefault global\t%s\n' "$v" "$b"; return 0; fi
  b=$(bin_of 8.5) || b=/opt/homebrew/bin/php
  printf '%s\tbawaan Homebrew\t%s\n' "8.5" "$b"
}

if [ -n "$FORCE" ]; then
  b=$(bin_of "$FORCE") || { echo "php$FORCE belum terpasang" >&2; exit 127; }
  r="$FORCE\tdipaksa lewat nama perintah\t$b"
else
  r=$(pick)
fi
if [ -n "$MACPANEL_PHP_SHIM_PRINT" ]; then printf '%b\n' "$r"; exit 0; fi
exec "$(printf '%b' "$r" | cut -f3)" "$@"
"""


def shim_text(force=""):
    return SHIM.replace("__FORCE__", force)


def installed_shims():
    return sorted(p.name for p in SHIM_DIR.glob("php*") if p.is_file()) if SHIM_DIR.exists() else []


def write_shims():
    SHIM_DIR.mkdir(parents=True, exist_ok=True)
    names = {"php": ""} | {f"php{v}": v for v in PHPS}
    for n, force in names.items():
        p = SHIM_DIR / n
        p.write_text(shim_text(force))
        p.chmod(p.stat().st_mode | stat.S_IXUSR)
    return sorted(names)


def zshrc_block():
    d = str(SHIM_DIR).replace(str(HOME), "$HOME")
    # Selalu pindahkan ke depan: PATH yang diwarisi (mis. terminal VS Code) bisa sudah memuat shim di belakang /opt/homebrew/bin.
    # Garis miring di dalam pola ${PATH//..//..} dibaca sebagai pemisah pola, jadi jalur masuk lewat variabel, bukan literal.
    return (
        f"{BEGIN}\n# Perintah php memilih versi per proyek (lab php default / lab php which). Dikelola MacPanel.\n"
        f'_mp_shims="{d}"; PATH=":$PATH:"; PATH="${{PATH//:${{_mp_shims}}:/:}}"; PATH="${{PATH#:}}"; PATH="${{PATH%:}}"; export PATH="$_mp_shims:$PATH"; unset _mp_shims\n{END}\n'
    )


def zshrc_has_block():
    try:
        return BEGIN in ZSHRC.read_text()
    except OSError:
        return False


def install_zshrc():
    """Tambahkan blok PATH ke ~/.zshrc sekali, dengan cadangan berstempel. Idempotent; blok versi lama diganti."""
    text = ZSHRC.read_text() if ZSHRC.exists() else ""
    if BEGIN in text and zshrc_block() in text:
        return False
    if BEGIN in text:
        (ZSHRC.parent / f".zshrc.bak.{time.strftime('%Y%m%d-%H%M%S')}").write_text(text)
        pat = re.compile(rf"\n*{re.escape(BEGIN)}.*?{re.escape(END)}\n?", re.S)
        ZSHRC.write_text(pat.sub("\n", text).rstrip("\n") + "\n\n" + zshrc_block())
        return True
    if ZSHRC.exists():
        (ZSHRC.parent / f".zshrc.bak.{time.strftime('%Y%m%d-%H%M%S')}").write_text(text)
    ZSHRC.write_text(text.rstrip("\n") + ("\n\n" if text else "") + zshrc_block())
    return True


def remove_zshrc():
    text = ZSHRC.read_text() if ZSHRC.exists() else ""
    if BEGIN not in text:
        return False
    (ZSHRC.parent / f".zshrc.bak.{time.strftime('%Y%m%d-%H%M%S')}").write_text(text)
    pat = re.compile(rf"\n*{re.escape(BEGIN)}.*?{re.escape(END)}\n?", re.S)
    ZSHRC.write_text(pat.sub("\n", text).rstrip("\n") + "\n")
    return True


def read_default():
    try:
        v = DEFAULT_FILE.read_text().strip().splitlines()[0].strip()
    except (OSError, IndexError):
        return ""
    return v if VER_RE.fullmatch(v) else ""


def which(cwd=None):
    env = dict(os.environ, MACPANEL_PHP_SHIM_PRINT="1", MACPANEL_HOME=str(ROOT))
    f = SHIM_DIR / "php"
    if not f.exists():
        die("shim belum dibuat (lab php shims)")
    r = subprocess.run(["/bin/sh", str(f)], cwd=cwd or os.getcwd(), env=env, capture_output=True, text=True)
    parts = r.stdout.strip().split("\t")
    if r.returncode or len(parts) != 3:
        die("gagal menentukan versi: " + (r.stderr or r.stdout).strip()[-200:])
    return {"version": parts[0], "reason": parts[1], "binary": parts[2]}


SOCK_RE = re.compile(r"php-fpm-dev-(8\.[2-5])\.sock")
ROOT_RE = re.compile(r"^\s*root\s+([^;]+);", re.M)


def site_versions():
    out = []
    for f in sorted(SITES.glob("*.conf")):
        t = f.read_text(errors="replace")
        m, r = SOCK_RE.search(t), ROOT_RE.search(t)
        out.append({"site": f.stem, "version": m.group(1) if m else None, "root": r.group(1).strip() if r else ""})
    return out


def brew_linked():
    try:
        tgt = os.readlink("/opt/homebrew/bin/php")
    except OSError:
        return ""
    m = re.search(r"/php(?:@(8\.\d))?/(\d+\.\d+)\.", tgt)
    return (m.group(1) or m.group(2)) if m else ""


def status():
    installed = [v for v in PHPS if php_home(v).exists()]
    return {
        "default": read_default(),
        "installed": installed,
        "shims": installed_shims(),
        "shell_ready": zshrc_has_block(),
        "brew_linked": brew_linked(),
        "sites": site_versions(),
    }


def set_default(v):
    if not VER_RE.fullmatch(v or ""):
        die("versi PHP: " + ", ".join(PHPS))
    if not php_home(v).exists():
        die(f"PHP {v} belum terpasang")
    DEFAULT_FILE.parent.mkdir(parents=True, exist_ok=True)
    DEFAULT_FILE.write_text(v + "\n")
    os.chmod(DEFAULT_FILE, 0o600)


def php_default(a):
    if a.act == "status":
        s = status()
        if a.json:
            print(json.dumps(s))
            return
        print(f"default CLI : {s['default'] or '(belum diatur, memakai bawaan Homebrew ' + s['brew_linked'] + ')'}")
        print(f"shim        : {'ya' if s['shims'] else 'belum'} | shell siap: {'ya' if s['shell_ready'] else 'belum'}")
        for x in s["sites"]:
            print(f"  web {x['site']:24} PHP {x['version'] or '-'}")
        return
    if a.act == "which":
        w = which(a.dir)
        print(f"PHP {w['version']}  ({w['reason']})\n{w['binary']}")
        return
    if a.act == "shims":
        if a.remove:
            n = remove_zshrc()
            print("OK: blok PATH dihapus dari ~/.zshrc (cadangan dibuat). Buka terminal baru." if n else "tidak ada blok di ~/.zshrc")
            return
        names = write_shims()
        added = install_zshrc()
        if not read_default():
            v = "8.3" if php_home("8.3").exists() else ""
            if v:
                set_default(v)
        print(f"OK: shim {', '.join(names)} di {SHIM_DIR}")
        print("OK: ~/.zshrc ditambah blok PATH (cadangan dibuat). Buka terminal baru." if added else "~/.zshrc sudah memuat blok PATH")
        return
    if a.version:
        set_default(a.version)
        print(f"OK: PHP CLI default = {a.version}" + ("" if installed_shims() else " (jalankan: lab php shims)"))
    else:
        print(read_default() or "(belum diatur)")
