"""lab dbcfg: pengaturan server MariaDB/MySQL/Redis (berkas milik panel, bukan my.cnf asli)."""

import json
import os
import re
import shutil
import socket
import subprocess
import time
from pathlib import Path

from labcli.core import BREW, ENV, HOME, die, sh

MY_SIZE = re.compile(r"^\d{1,5}[MG]$")


MYSQL_KEYS = {
    "max_connections": (lambda s: re.fullmatch(r"\d{2,4}", s) and 10 <= int(s) <= 2000, "10-2000"),
    "innodb_buffer_pool_size": (lambda s: MY_SIZE.match(s) and 64 <= (int(s[:-1]) * (1024 if s[-1] == "G" else 1)) <= 8192, "64M-8G, mis. 512M"),
    "max_allowed_packet": (lambda s: MY_SIZE.match(s) and 1 <= (int(s[:-1]) * (1024 if s[-1] == "G" else 1)) <= 1024, "1M-1G, mis. 64M"),
    "slow_query_log": (lambda s: s in ("0", "1"), "0 atau 1"),
    "long_query_time": (lambda s: re.fullmatch(r"\d{1,2}(\.\d)?", s) and float(s) <= 60, "detik, 0-60"),
}


DBPORT = {"mariadb": 3306, "mysql": 3307}


MYSQL_FILE = Path("/opt/homebrew/etc/my.cnf.d/20-macpanel.cnf")


MYSQL_SLOW = "/opt/homebrew/var/mysql/macpanel-slow.log"


MARIADB_DIR = Path("/opt/homebrew/etc/mariadb")


MARIADB_FILE = MARIADB_DIR / "my.cnf.d/20-macpanel.cnf"


MARIADB_SLOW = "/opt/homebrew/var/mariadb/macpanel-slow.log"


MARIADB_LABEL = "com.macminiops.mariadb"


REDIS_KEYS = {
    "maxmemory": (lambda s: s == "0" or re.fullmatch(r"\d{1,5}(mb|gb)", s), "0 (tanpa batas) atau mis. 256mb, 2gb"),
    "maxmemory-policy": (lambda s: s in ("noeviction", "allkeys-lru", "volatile-lru", "allkeys-lfu", "volatile-lfu", "allkeys-random", "volatile-ttl"), "noeviction, allkeys-lru, volatile-lru, ..."),
    "appendonly": (lambda s: s in ("yes", "no"), "yes atau no"),
    "save": (lambda s: s in ("off", "default"), "off atau default (3600 1 300 100 60 10000)"),
}


REDIS_CONF = Path("/opt/homebrew/etc/redis.conf")


REDIS_INC = Path("/opt/homebrew/etc/redis.d/macpanel.conf")


def _kv_file(path, keys):
    cur = {}
    if path.exists():
        for l in path.read_text().splitlines():
            m = re.match(r"^([a-z_-]+)\s+(.*?)\s*$", l) if path == REDIS_INC else re.match(r"^([a-z_]+)\s*=\s*(.*?)\s*$", l)
            if m and m.group(1) in keys:
                cur[m.group(1)] = m.group(2).strip('"')
    return cur


def my_live(eng, k):
    root = {}
    for ln in (HOME / ".config/macminiops/db.env").read_text().splitlines():
        if "=" in ln and not ln.startswith("#"):
            a, b = ln.split("=", 1)
            root[a.strip()] = b.strip().strip("'\"")
    if eng == "mariadb":
        cli, key = "/opt/homebrew/opt/mariadb@11.8/bin/mariadb", "MARIADB_ROOT_PASSWORD"
    else:
        cli, key = "/opt/homebrew/opt/mysql@8.4/bin/mysql", "MYSQL_ROOT_PASSWORD"
    e = dict(ENV, MYSQL_PWD=root.get(key, ""))
    r = subprocess.run([cli, "-h127.0.0.1", f"-P{DBPORT[eng]}", "-uroot", "-N", "-e", f"SELECT @@{k}"], env=e, text=True, capture_output=True)
    return r.stdout.strip() if r.returncode == 0 else ""


def restart_launchd(label, wait=90):
    """Hentikan dengan SIGTERM (bootout, bukan kickstart -k yang SIGKILL: berbahaya untuk database), tunggu benar-benar mati, lalu muat lagi."""
    plist = str(HOME / "Library/LaunchAgents" / f"{label}.plist")
    dom = f"gui/{os.getuid()}"
    sh(["/bin/launchctl", "bootout", f"{dom}/{label}"])
    for _ in range(wait * 2):
        if sh(["/bin/launchctl", "print", f"{dom}/{label}"]).returncode != 0:
            break
        time.sleep(0.5)
    return sh(["/bin/launchctl", "bootstrap", dom, plist])


def redis_live(k):
    r = sh(["/opt/homebrew/bin/redis-cli", "-h", "127.0.0.1", "CONFIG", "GET", k])
    l = r.stdout.splitlines()
    return l[1] if r.returncode == 0 and len(l) >= 2 else ""


def dbcfg(a):
    eng = a.engine
    keys, path = {"mysql": (MYSQL_KEYS, MYSQL_FILE), "mariadb": (MYSQL_KEYS, MARIADB_FILE)}.get(eng, (REDIS_KEYS, REDIS_INC))
    live = (lambda k: my_live(eng, k)) if eng in DBPORT else redis_live
    if a.act == "get":
        cur = _kv_file(path, keys)
        print(json.dumps(dict(engine=eng, keys=[dict(key=k, value=cur.get(k, ""), live=live(k), hint=h) for k, (_, h) in keys.items()])))
        return
    if a.act == "slow":
        if eng not in DBPORT:
            die("slow log hanya untuk MariaDB/MySQL")
        p = Path(MARIADB_SLOW if eng == "mariadb" else MYSQL_SLOW)
        print("\n".join(p.read_text(errors="replace").splitlines()[-200:]) if p.exists() else "(log kosong; aktifkan slow_query_log)")
        return
    pairs = {}
    for kv in a.pairs:
        if "=" not in kv:
            die(f"format kunci=nilai: {kv!r}")
        k, val = kv.split("=", 1)
        if k not in keys:
            die(f"kunci tidak didukung: {k!r} (boleh: {', '.join(keys)})")
        if not keys[k][0](val):
            die(f"nilai {k} tidak valid ({keys[k][1]})")
        pairs[k] = val
    if not pairs:
        die("tidak ada nilai")
    cur = _kv_file(path, keys)
    cur.update(pairs)
    old = path.read_text() if path.exists() else None
    path.parent.mkdir(parents=True, exist_ok=True)
    if old is not None:
        shutil.copy2(path, f"{path}.bak.{time.strftime('%Y%m%d-%H%M%S')}")
    if eng in DBPORT:
        slow = MARIADB_SLOW if eng == "mariadb" else MYSQL_SLOW
        body = "# Dikelola MacPanel (`lab dbcfg`). Jangan edit manual.\n[mysqld]\n" + "".join(f"{k} = {v}\n" for k, v in cur.items())
        if cur.get("slow_query_log") == "1":
            body += f"slow_query_log_file = {slow}\n"
        svc, port = ("mariadb", 3306) if eng == "mariadb" else ("mysql@8.4", 3307)
        if eng == "mariadb":  # my.cnf MariaDB hanya dibaca lewat --defaults-file; pastikan folder include-nya dimuat
            mc = MARIADB_DIR / "my.cnf"
            inc = f"!includedir {MARIADB_DIR}/my.cnf.d"
            t = mc.read_text()
            if inc not in t:
                shutil.copy2(mc, f"{mc}.bak.{time.strftime('%Y%m%d-%H%M%S')}")
                mc.write_text(t.rstrip("\n") + f"\n\n{inc}\n")
    else:
        body = "# Dikelola MacPanel (`lab dbcfg`). Jangan edit manual.\n"
        for k, v in cur.items():
            if k == "save":
                body += 'save ""\n' if v == "off" else "save 3600 1 300 100 60 10000\n"
            else:
                body += f"{k} {v}\n"
        svc, port = "redis", 6379
        inc = f"include {REDIS_INC}"
        t = REDIS_CONF.read_text()
        if inc not in t:
            shutil.copy2(REDIS_CONF, f"{REDIS_CONF}.bak.{time.strftime('%Y%m%d-%H%M%S')}")
            REDIS_CONF.write_text(t.rstrip("\n") + f"\n\n# MacPanel\n{inc}\n")
    path.write_text(body)
    restart_launchd(MARIADB_LABEL) if eng == "mariadb" else sh([BREW, "services", "restart", svc])
    ok = False
    for _ in range(40):
        time.sleep(0.5)
        try:
            c = socket.create_connection(("127.0.0.1", port), 1)
            c.close()
            ok = True
            break
        except OSError:
            pass
    good = ok and all(live(k) != "" for k in pairs)
    if not good:
        if old is None:
            path.unlink(missing_ok=True)
        else:
            path.write_text(old)
        restart_launchd(MARIADB_LABEL) if eng == "mariadb" else sh([BREW, "services", "restart", svc])
        die(f"{eng} tidak naik dengan pengaturan baru; dikembalikan")
    print("OK: " + ", ".join(f"{k}={pairs[k]} (aktif: {live(k)})" for k in pairs))
