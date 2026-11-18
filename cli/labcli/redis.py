"""lab redis: penelusur kunci Redis (argumen berupa list, tanpa shell)."""

import json
import re
import subprocess

from labcli.core import die

RCLI = "/opt/homebrew/bin/redis-cli"


KEY_OK = re.compile(r"^[\x21-\x7e]{1,200}$")


PAT_OK = re.compile(r"^[A-Za-z0-9:_.*?\-]{1,100}$")


def rcli(db, *args, timeout=10):
    r = subprocess.run([RCLI, "-h", "127.0.0.1", "-n", str(db), "--no-raw", *args], capture_output=True, text=True, timeout=timeout)
    if r.returncode:
        die("redis-cli gagal: " + (r.stderr or r.stdout).strip()[-200:])
    return r.stdout


def rcli_raw(db, *args):
    r = subprocess.run([RCLI, "-h", "127.0.0.1", "-n", str(db), "--raw", *args], capture_output=True, text=True, timeout=10)
    if r.returncode:
        die("redis-cli gagal: " + (r.stderr or r.stdout).strip()[-200:])
    return r.stdout.splitlines()


def redis_cmd(a):
    if not (0 <= a.db <= 15):
        die("db 0-15")
    if a.act == "info":
        n = rcli_raw(a.db, "DBSIZE")[0]
        m = {k: v for k, v in (l.split(":", 1) for l in rcli_raw(a.db, "INFO", "memory") if ":" in l and not l.startswith("#"))}
        sz = {}
        for l in rcli_raw(a.db, "INFO", "keyspace"):
            if l.startswith("db"):
                sz[l.split(":")[0]] = l.split(":", 1)[1]
        print(json.dumps(dict(dbsize=int(n), used_memory=m.get("used_memory_human", ""), maxmemory=m.get("maxmemory_human", ""), keyspace=sz)))
        return
    if a.act == "scan":
        pat = a.arg or "*"
        if not PAT_OK.match(pat):
            die("pola: huruf, angka, : _ . - * ?")
        keys, cur = [], "0"
        for _ in range(60):
            out = rcli_raw(a.db, "SCAN", cur, "MATCH", pat, "COUNT", "500")
            cur, ks = out[0], out[1:]
            keys += ks
            if cur == "0" or len(keys) >= 200:
                break
        keys = keys[:200]
        rows = []
        for k in keys:
            t = rcli_raw(a.db, "TYPE", k)[0]
            ttl = rcli_raw(a.db, "TTL", k)[0]
            rows.append(dict(key=k, type=t, ttl=int(ttl) if ttl.lstrip("-").isdigit() else -1))
        print(json.dumps(dict(keys=rows, more=(cur != "0"))))
        return
    if not KEY_OK.match(a.arg or ""):
        die("nama kunci tidak valid (ASCII tampak, maks 200)")
    if a.act == "del":
        n = rcli_raw(a.db, "DEL", a.arg)[0]
        print(f"OK: {n} kunci dihapus")
        return
    t = rcli_raw(a.db, "TYPE", a.arg)[0]
    if t == "none":
        die("kunci tidak ada")
    ttl = int(rcli_raw(a.db, "TTL", a.arg)[0])
    if t == "string":
        ln = int(rcli_raw(a.db, "STRLEN", a.arg)[0])
        val = rcli_raw(a.db, "GETRANGE", a.arg, "0", "4095")
        body = "\n".join(val)
        extra = ln
    elif t == "list":
        body = "\n".join(rcli_raw(a.db, "LRANGE", a.arg, "0", "99"))
        extra = int(rcli_raw(a.db, "LLEN", a.arg)[0])
    elif t == "set":
        body = "\n".join(rcli_raw(a.db, "SRANDMEMBER", a.arg, "100"))
        extra = int(rcli_raw(a.db, "SCARD", a.arg)[0])
    elif t == "zset":
        body = "\n".join(rcli_raw(a.db, "ZRANGE", a.arg, "0", "99", "WITHSCORES"))
        extra = int(rcli_raw(a.db, "ZCARD", a.arg)[0])
    elif t == "hash":
        body = "\n".join(rcli_raw(a.db, "HGETALL", a.arg)[:200])
        extra = int(rcli_raw(a.db, "HLEN", a.arg)[0])
    else:
        body = f"(tipe {t} tidak ditampilkan)"
        extra = 0
    print(json.dumps(dict(key=a.arg, type=t, ttl=ttl, size=extra, value=body[:8192], truncated=len(body) > 8192 or extra > 100)))
