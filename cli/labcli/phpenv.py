"""Jalur dan operasi PHP bersama (home, socket, FPM, Composer)."""

import socket
import time
from pathlib import Path

from labcli.core import BREW, PHPS, die, formula, sh

COMPOSER = "/opt/homebrew/bin/composer"


def php_home(v):
    return Path(f"/opt/homebrew/opt/{formula(v)}")


def php_binary(v):
    return str(php_home(v) / "bin/php")


def conf_dir(v):
    return Path(f"/opt/homebrew/etc/php/{v}")


def sock(v):
    return Path(f"/opt/homebrew/var/run/php-fpm-dev-{v}.sock")


def need_php(v):
    if v not in PHPS:
        die(f"versi PHP: {', '.join(PHPS)}")
    if not php_home(v).exists():
        die(f"PHP {v} belum terpasang")


def sock_ok(v):
    try:
        c = socket.socket(socket.AF_UNIX)
        c.settimeout(1)
        c.connect(str(sock(v)))
        c.close()
        return True
    except OSError:
        return False


def restart_fpm(v):
    """Restart FPM dan tunggu socket benar-benar menjawab (bukan hanya `brew services` bilang started)."""
    sh([BREW, "services", "restart", formula(v)])
    for _ in range(20):
        time.sleep(0.5)
        if sock_ok(v):
            return True
    return False


def php_prefix(v):
    return f"/opt/homebrew/opt/{formula(v)}"
