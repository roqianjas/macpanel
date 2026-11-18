"""lab ini: pengaturan php.ini dan pool FPM."""

import json
import re
import shutil
import time
from pathlib import Path

from labcli.core import die, sh
from labcli.phpenv import conf_dir, need_php, php_binary, php_home, restart_fpm

SIZE = re.compile(r"^\d{1,5}[KMG]$")


INT = re.compile(r"^(-1|\d{1,6})$")


INI_KEYS = {
    "memory_limit": (lambda s: s == "-1" or SIZE.match(s), "mis. 512M, 2G, atau -1"),
    "upload_max_filesize": (lambda s: SIZE.match(s), "mis. 64M"),
    "post_max_size": (lambda s: SIZE.match(s), "mis. 64M"),
    "max_execution_time": (lambda s: INT.match(s), "detik, 0 = tanpa batas"),
    "max_input_time": (lambda s: INT.match(s), "detik"),
    "max_input_vars": (lambda s: re.fullmatch(r"\d{3,6}", s), "mis. 3000"),
    "display_errors": (lambda s: s in ("0", "1"), "0 atau 1"),
    "date.timezone": (lambda s: re.fullmatch(r"[A-Za-z0-9_+/-]{1,40}", s), "mis. Asia/Jakarta"),
}


POOL_KEYS = {
    "pm": (lambda s: s in ("static", "dynamic", "ondemand"), "static, dynamic, atau ondemand"),
    "pm.max_children": (lambda s: re.fullmatch(r"\d{1,3}", s) and 1 <= int(s) <= 200, "1-200 proses"),
    "pm.start_servers": (lambda s: re.fullmatch(r"\d{1,3}", s) and 1 <= int(s) <= 200, "mode dynamic: proses awal"),
    "pm.min_spare_servers": (lambda s: re.fullmatch(r"\d{1,3}", s) and 1 <= int(s) <= 200, "mode dynamic: minimum menganggur"),
    "pm.max_spare_servers": (lambda s: re.fullmatch(r"\d{1,3}", s) and 1 <= int(s) <= 200, "mode dynamic: maksimum menganggur"),
    "pm.process_idle_timeout": (lambda s: re.fullmatch(r"\d{1,4}s", s), "mode ondemand, mis. 30s"),
    "pm.max_requests": (lambda s: re.fullmatch(r"\d{1,6}", s) and int(s) <= 100000, "0 = tanpa daur ulang"),
}


def pool_file(v):
    return conf_dir(v) / "php-fpm.d/dev.conf"


def pool_values(v):
    vals = {}
    p = pool_file(v)
    if p.exists():
        for l in p.read_text().splitlines():
            m = re.match(r"^php_admin_value\[([a-z_.]+)\]\s*=\s*(\S.*?)\s*$", l)
            if m:
                vals[m.group(1)] = m.group(2)
            m = re.match(r"^(pm(?:\.[a-z_]+)?)\s*=\s*(\S.*?)\s*$", l)
            if m and m.group(1) in POOL_KEYS:
                vals[m.group(1)] = m.group(2)
    return vals


def ini_get(v):
    need_php(v)
    cur = pool_values(v)
    rows = []
    for k, (_, hint) in INI_KEYS.items():
        eff = sh([php_binary(v), "-n", "-r", f"echo ini_get('{k}');"]).stdout.strip()
        rows.append(dict(key=k, value=cur.get(k, ""), default=eff, hint=hint, group="php"))
    for k, (_, hint) in POOL_KEYS.items():
        rows.append(dict(key=k, value=cur.get(k, ""), default="", hint=hint, group="fpm"))
    print(json.dumps(dict(version=v, keys=rows)))


def tz_ok(v, tz):
    return sh([php_binary(v), "-n", "-r", "exit(@timezone_open($argv[1]) ? 0 : 1);", "--", tz]).returncode == 0


def ini_set(a):
    v = a.php
    need_php(v)
    pairs = {}
    for kv in a.pairs:
        if "=" not in kv:
            die(f"format kunci=nilai: {kv!r}")
        k, val = kv.split("=", 1)
        if k in POOL_KEYS:
            if not POOL_KEYS[k][0](val):
                die(f"nilai {k} tidak valid ({POOL_KEYS[k][1]})")
            pairs[k] = val
            continue
        if k not in INI_KEYS:
            die(f"kunci tidak didukung: {k!r} (boleh: {', '.join(list(INI_KEYS) + list(POOL_KEYS))})")
        if not INI_KEYS[k][0](val):
            die(f"nilai {k} tidak valid ({INI_KEYS[k][1]})")
        if k == "date.timezone" and not tz_ok(v, val):
            die("zona waktu tidak dikenal PHP")
        pairs[k] = val
    p = pool_file(v)
    if not p.exists():
        die(f"{p} tidak ada; jalankan `lab php install {v}`")
    old = p.read_text()
    bak = f"{p}.bak.{time.strftime('%Y%m%d-%H%M%S')}"
    new = old if old.endswith("\n") else old + "\n"
    for k, val in pairs.items():
        if k in POOL_KEYS:
            pat = re.compile(rf"^{re.escape(k)}\s*=.*$", re.MULTILINE)
            line = f"{k} = {val}"
        else:
            pat = re.compile(rf"^php_admin_value\[{re.escape(k)}\]\s*=.*$", re.MULTILINE)
            line = f"php_admin_value[{k}] = {val}"
        new = pat.sub(line, new, count=1) if pat.search(new) else new + line + "\n"
    eff = {}
    for l in new.splitlines():
        m = re.match(r"^(pm(?:\.[a-z_]+)?)\s*=\s*(\S+)", l)
        if m and m.group(1) in POOL_KEYS:
            eff[m.group(1)] = m.group(2)
    if eff.get("pm") == "dynamic":
        try:
            mn, st, mx, ch = (int(eff[k]) for k in ("pm.min_spare_servers", "pm.start_servers", "pm.max_spare_servers", "pm.max_children"))
        except (KeyError, ValueError):
            die("mode dynamic butuh pm.start_servers, pm.min_spare_servers, pm.max_spare_servers, dan pm.max_children")
        if not (1 <= mn <= st <= mx <= ch):
            die("mode dynamic: min_spare <= start_servers <= max_spare <= max_children")
    if eff.get("pm") == "static" and "pm.max_children" not in eff:
        die("mode static butuh pm.max_children")
    shutil.copy2(p, bak)
    p.write_text(new)
    t = sh([str(php_home(v) / "sbin/php-fpm"), "-t"])
    if t.returncode:
        p.write_text(old)
        die("php-fpm -t gagal, dikembalikan: " + (t.stderr or t.stdout).strip()[-200:])
    if not restart_fpm(v):
        p.write_text(old)
        restart_fpm(v)
        die("FPM tidak menjawab setelah restart, dikembalikan")
    print(f"OK: {', '.join(f'{k}={x}' for k, x in pairs.items())} diterapkan ke FPM {v} (cadangan: {Path(bak).name})")
