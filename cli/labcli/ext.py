"""lab ext: ekstensi PHP (PECL dan bawaan)."""

import json
import re
import shutil
import subprocess
import time
from pathlib import Path

from labcli.core import BREW, ENV, die, sh, stream
from labcli.phpenv import conf_dir, need_php, php_binary, php_home, restart_fpm

# Hanya ekstensi yang sudah diuji pasang di mesin ini.
EXTS = {
    "redis": dict(label="Redis (phpredis)", zend=False, brew=[]),
    "apcu": dict(label="APCu (cache memori)", zend=False, brew=["pcre2"]),
    "xdebug": dict(label="Xdebug", zend=True, brew=[], ini=["xdebug.mode=develop,debug"]),
    "imagick": dict(label="ImageMagick (imagick)", zend=False, brew=["pkgconf", "imagemagick"], cfg='with-imagick="/opt/homebrew/opt/imagemagick"'),
    "igbinary": dict(label="igbinary (serialisasi cepat)", zend=False, brew=[]),
    "msgpack": dict(label="MessagePack", zend=False, brew=[]),
    "pcov": dict(label="PCOV (code coverage)", zend=False, brew=[]),
    "yaml": dict(label="YAML", zend=False, brew=["libyaml"], cfg='with-yaml="/opt/homebrew/opt/libyaml"'),
    "memcached": dict(
        label="Memcached (klien)",
        zend=False,
        brew=["libmemcached", "zlib"],
        cfg='enable-memcached-session="yes" enable-memcached-igbinary="no" enable-memcached-msgpack="no" enable-memcached-json="no" with-libmemcached-dir="/opt/homebrew/opt/libmemcached" with-zlib-dir="/opt/homebrew/opt/zlib" enable-memcached-sasl="no" with-system-fastlz="no" enable-memcached-protocol="no"',
    ),
    "mongodb": dict(label="MongoDB", zend=False, brew=["openssl@3"], cfg='with-mongodb-system-libs="no" with-mongodb-ssl="openssl" with-mongodb-sasl="no" with-mongodb-client-side-encryption="no"'),
}


def ext_dir(v):
    r = sh([php_binary(v), "-r", "echo ini_get('extension_dir');"])
    return Path(r.stdout.strip())


def modules(v):
    out = sh([php_binary(v), "-m"]).stdout
    return [l.strip() for l in out.splitlines() if l.strip() and not l.startswith("[")]


def ext_ini(v, n):
    return conf_dir(v) / "conf.d" / f"ext-lab-{n}.ini"


def ext_list(v):
    need_php(v)
    mods = modules(v)
    low = {m.lower() for m in mods}
    ed = ext_dir(v)
    managed = []
    for n, c in EXTS.items():
        on = ext_ini(v, n)
        managed.append(dict(name=n, label=c["label"], installed=(ed / f"{n}.so").exists(), enabled=on.exists(), loaded=n in low, zend=c["zend"]))
    builtin = sorted({m for m in mods if m.lower() not in EXTS}, key=str.lower)
    print(json.dumps(dict(version=v, builtin=builtin, managed=managed)))


def strip_pecl_ini_lines(v, n):
    """`pecl install` menyisipkan extension="x.so" ke php.ini; pindahkan ke conf.d agar bisa dimatikan satu per satu."""
    p = conf_dir(v) / "php.ini"
    if not p.exists():
        return
    pat = re.compile(rf'^\s*(zend_)?extension\s*=\s*"?{re.escape(n)}(\.so)?"?\s*$')
    lines = p.read_text().splitlines(keepends=True)
    keep = [l for l in lines if not pat.match(l.strip())]
    if len(keep) != len(lines):
        shutil.copy2(p, f"{p}.bak.{time.strftime('%Y%m%d-%H%M%S')}")
        p.write_text("".join(keep))


def write_ext_ini(v, n):
    c = EXTS[n]
    d = conf_dir(v) / "conf.d"
    d.mkdir(parents=True, exist_ok=True)
    key = "zend_extension" if c["zend"] else "extension"
    ini = ext_ini(v, n)
    ini.write_text(f"; Dikelola `lab ext`. Hapus lewat panel atau `lab ext remove`.\n{key}={n}.so\n" + "".join(x + "\n" for x in c.get("ini", [])))
    Path(str(ini) + ".disabled").unlink(missing_ok=True)


def ext_act(a):
    v, n = a.php, a.name
    need_php(v)
    if n not in EXTS:
        die(f"ekstensi yang didukung: {', '.join(EXTS)}")
    ini = ext_ini(v, n)
    off = Path(str(ini) + ".disabled")
    if a.act == "install":
        if not (ext_dir(v) / f"{n}.so").exists():
            for b in EXTS[n]["brew"]:
                if sh([BREW, "list", "--versions", b]).stdout.strip():
                    continue
                print(f"memasang dependensi {b} ...", flush=True)
                for i in range(4):
                    if stream([BREW, "install", b]) == 0:
                        break
                    print(f"brew install {b}: percobaan {i + 1} gagal, ulangi...", flush=True)
                    time.sleep(8)
                else:
                    die(f"brew install {b} gagal setelah 4 percobaan (jaringan?)")
            pecl = str(php_home(v) / "bin/pecl")
            # Cellar/<php>/pecl adalah symlink ke lib/php/pecl; bila tujuan belum ada, pecl gagal mkdir.
            try:
                (php_home(v) / "pecl").resolve().mkdir(parents=True, exist_ok=True)
            except OSError as e:
                die(f"tidak bisa membuat folder pecl: {e}")
            env = dict(
                ENV, PKG_CONFIG_PATH="/opt/homebrew/lib/pkgconfig:/opt/homebrew/share/pkgconfig", CPPFLAGS="-I/opt/homebrew/include", CFLAGS="-I/opt/homebrew/include", LDFLAGS="-L/opt/homebrew/lib"
            )
            for i in range(3):
                r = subprocess.run([pecl, "install", "-f"] + ([f"--configureoptions={EXTS[n]['cfg']}"] if EXTS[n].get("cfg") else []) + [n], env=env, input="\n" * 10, text=True, capture_output=True)
                if r.returncode == 0 and (ext_dir(v) / f"{n}.so").exists():
                    break
                errs = [l.strip()[:200] for l in (r.stdout + r.stderr).splitlines() if re.search(r"error|fatal|not found", l, re.IGNORECASE)][-4:]
                print(f"percobaan {i + 1} gagal: {' | '.join(errs) or (r.stdout + r.stderr).strip()[-200:]}", flush=True)
                time.sleep(5)
            else:
                die("pecl install gagal (penyebab di atas)")
        strip_pecl_ini_lines(v, n)
        write_ext_ini(v, n)
    elif a.act == "enable":
        if not (ext_dir(v) / f"{n}.so").exists():
            die(f"{n} belum terpasang di PHP {v}")
        write_ext_ini(v, n)
    elif a.act == "disable":
        if not ini.exists():
            die(f"{n} sudah nonaktif atau belum terpasang")
        off.unlink(missing_ok=True)
        ini.rename(off)
    elif a.act == "remove":
        ini.unlink(missing_ok=True)
        off.unlink(missing_ok=True)
        so = ext_dir(v) / f"{n}.so"
        if so.exists():
            r = sh([str(php_home(v) / "bin/pecl"), "uninstall", n], input="")
            strip_pecl_ini_lines(v, n)
            if so.exists():
                so.unlink()
    ok = restart_fpm(v)
    mods = {m.lower() for m in modules(v)}
    want = a.act in ("install", "enable")
    if want and n not in mods:
        ini.unlink(missing_ok=True)
        restart_fpm(v)
        die(f"{n} tidak termuat di PHP {v}; perubahan dibatalkan")
    if not want and n in mods:
        die(f"{n} masih termuat di PHP {v}")
    if not ok:
        die(f"FPM {v} tidak menjawab setelah restart; periksa log")
    print(f"OK: {n} {a.act} pada PHP {v}; FPM dimulai ulang dan {'termuat' if want else 'tidak termuat'} (php -m)")
