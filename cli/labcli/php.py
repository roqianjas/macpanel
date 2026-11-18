"""lab php: pasang dan hapus versi PHP beserta pool FPM."""

import os
import time
from pathlib import Path

from labcli.core import BREW, HOME, OPS, PHPS, die, formula, sh, stream
from labcli.phpenv import php_prefix

POOL = """; Pool dev PHP {v}. Dikelola `lab php`. Unix socket 0600, tanpa TCP.
[dev]
user = {user}
group = staff
listen = /opt/homebrew/var/run/php-fpm-dev-{v}.sock
listen.mode = 0600
pm = ondemand
pm.max_children = 8
pm.process_idle_timeout = 30s
security.limit_extensions = .php
php_admin_value[expose_php] = 0
php_admin_value[memory_limit] = 512M
php_admin_value[upload_max_filesize] = 64M
php_admin_value[post_max_size] = 64M
php_admin_value[error_log] = /opt/homebrew/var/log/php-fpm-dev-{v}.log
php_admin_flag[log_errors] = on
"""


def php(a):
    if a.act == "list":
        for v in PHPS:
            ok = Path(php_prefix(v)).exists()
            sock = Path(f"/opt/homebrew/var/run/php-fpm-dev-{v}.sock").exists()
            ver = sh([php_prefix(v) + "/bin/php", "-r", "echo PHP_VERSION;"]).stdout if ok else "-"
            print(f"{v}  terpasang={'ya' if ok else 'tidak':5} versi={ver:8} fpm_socket={'ada' if sock else '-'}")
        return
    v = a.version
    if v not in PHPS:
        die(f"versi didukung: {', '.join(PHPS)} (8.1 dan lebih lama dinyatakan deprecated di Homebrew)")
    f = formula(v)
    if a.act == "install":
        if not Path(php_prefix(v)).exists():
            for i in range(3):
                if stream([BREW, "install", f]) == 0:
                    break
                print("percobaan", i + 1, "gagal, ulangi...")
                time.sleep(8)
            else:
                die("brew install gagal")
        d = Path(f"/opt/homebrew/etc/php/{v}/php-fpm.d")
        www = d / "www.conf"
        if www.exists():
            www.rename(d / "www.conf.disabled")  # www.conf bawaan membuka TCP 127.0.0.1:9000
        (d / "dev.conf").write_text(POOL.format(v=v, user=os.environ.get("USER", HOME.name)))
        t = sh([php_prefix(v) + "/sbin/php-fpm", "-t"])
        if t.returncode:
            die("php-fpm -t gagal: " + t.stderr[-300:])
        sh([BREW, "services", "restart", f], check=True)
        time.sleep(3)
        if not Path(f"/opt/homebrew/var/run/php-fpm-dev-{v}.sock").exists():
            die("socket FPM tidak muncul")
        print(f"OK: PHP {v} terpasang, FPM socket aktif. Pakai: lab site add <nama> <folder> --php {v}")
    elif a.act == "remove":
        used = [p.name for p in (OPS / "nginx/sites").glob("*.conf") if f"php-fpm-dev-{v}.sock" in p.read_text()]
        if used:
            die(f"masih dipakai situs: {', '.join(used)}")
        sh([BREW, "services", "stop", f])
        sh([BREW, "uninstall", f], check=True)
        print(f"OK: PHP {v} dihapus")
