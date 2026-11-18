"""lab redis: penelusur kunci Redis (argumen berupa list, tanpa shell)."""

import json
import os
import re
import secrets
import shutil
import subprocess
import time
from pathlib import Path

from labcli.core import ENV, die

RCLI = "/opt/homebrew/bin/redis-cli"
PORT = 6379
REDIS_CONF = Path("/opt/homebrew/etc/redis.conf")
REDIS_ENV = Path.home() / ".config/macminiops/redis.env"
MARK_BEGIN = "# >>> macpanel requirepass (dikelola: lab redis password) >>>"
MARK_END = "# <<< macpanel requirepass <<<"


KEY_OK = re.compile(r"^[\x21-\x7e]{1,200}$")


PAT_OK = re.compile(r"^[A-Za-z0-9:_.*?\-]{1,100}$")


def read_password(path=None):
    """Kata sandi Redis dari berkas env (KEY=VALUE, 0600), atau None bila tidak ada."""
    p = Path(path) if path else REDIS_ENV
    try:
        for line in p.read_text().splitlines():
            m = re.match(r"^\s*REDIS_PASSWORD\s*=\s*(\S+)\s*$", line)
            if m:
                return m.group(1)
    except OSError:
        return None
    return None


def _env(pw=None):
    env = dict(os.environ)
    pw = pw if pw is not None else read_password()
    if pw:
        env["REDISCLI_AUTH"] = pw
    else:
        env.pop("REDISCLI_AUTH", None)
    return env


def _fail(r):
    msg = (r.stderr or r.stdout).strip()
    if "NOAUTH" in msg or "WRONGPASS" in msg:
        die("Redis meminta kata sandi tetapi ~/.config/macminiops/redis.env tidak ada atau salah. Periksa: lab redis password status")
    die("redis-cli gagal: " + msg[-200:])


def rcli(db, *args, timeout=10):
    r = subprocess.run([RCLI, "-h", "127.0.0.1", "-p", str(PORT), "-n", str(db), "--no-raw", *args], capture_output=True, text=True, timeout=timeout, env=_env())
    if r.returncode or r.stdout.startswith(("NOAUTH", "WRONGPASS")):
        _fail(r)
    return r.stdout


def rcli_raw(db, *args):
    r = subprocess.run([RCLI, "-h", "127.0.0.1", "-p", str(PORT), "-n", str(db), "--raw", *args], capture_output=True, text=True, timeout=10, env=_env())
    if r.returncode or r.stdout.startswith(("NOAUTH", "WRONGPASS")):
        _fail(r)
    return r.stdout.splitlines()


# ---------------- kata sandi Redis ----------------


def conf_block(pw):
    return f"{MARK_BEGIN}\nrequirepass {pw}\n{MARK_END}\n"


def patch_conf(text, pw):
    """Kembalikan teks konfigurasi dengan blok requirepass kita dipasang (pw) atau dibuang (pw=None). Idempoten.
    Menolak bila ada `requirepass` aktif yang bukan milik kita: itu pengaturan manual pemilik, tidak boleh ditimpa diam-diam."""
    block = re.compile(re.escape(MARK_BEGIN) + r".*?" + re.escape(MARK_END) + r"\n?", re.S)
    stripped = block.sub("", text)
    if re.search(r"(?m)^\s*requirepass\s+\S", stripped):
        raise ValueError("redis.conf sudah memuat requirepass yang diatur manual; hapus atau atur sendiri dulu")
    if not stripped.endswith("\n"):
        stripped += "\n"
    return stripped if pw is None else stripped + conf_block(pw)


def conf_managed(text):
    return MARK_BEGIN in text and MARK_END in text


def ping(pw=None, timeout=3):
    """'ok' bila PONG, 'noauth' bila Redis meminta kata sandi, 'down' bila tidak menjawab, 'wrong' bila sandi ditolak."""
    env = dict(os.environ)
    env.pop("REDISCLI_AUTH", None)
    if pw:
        env["REDISCLI_AUTH"] = pw
    try:
        r = subprocess.run([RCLI, "-h", "127.0.0.1", "-p", str(PORT), "ping"], capture_output=True, text=True, timeout=timeout, env=env)
    except subprocess.TimeoutExpired:
        return "down"
    out = (r.stdout + r.stderr).strip()
    if out.startswith("PONG"):
        return "ok"
    if "WRONGPASS" in out or "invalid password" in out.lower() or "AUTH failed" in out:
        return "wrong"
    if "NOAUTH" in out:
        return "noauth"
    return "down"


def restart_redis():
    r = subprocess.run(["/opt/homebrew/bin/brew", "services", "restart", "redis"], capture_output=True, text=True, env=ENV, timeout=60)
    if r.returncode:
        raise RuntimeError((r.stderr or r.stdout).strip()[-200:])


def wait_for(state, pw=None, secs=20):
    end = time.time() + secs
    while time.time() < end:
        if ping(pw) == state:
            return True
        time.sleep(0.5)
    return False


def _write_env(path, pw):
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(path.name + ".tmp")
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w") as f:
        f.write(f"REDIS_PASSWORD={pw}\n")
    os.replace(tmp, path)
    os.chmod(path, 0o600)


def _backup(conf):
    bak = conf.with_name(f"{conf.name}.bak.{time.strftime('%Y%m%d-%H%M%S')}")
    shutil.copy2(conf, bak)
    os.chmod(bak, 0o600)
    return bak


def password_status(conf=None, envp=None):
    conf, envp = Path(conf or REDIS_CONF), Path(envp or REDIS_ENV)
    managed = conf_managed(conf.read_text()) if conf.exists() else False
    pw = read_password(envp)
    return {
        "managed": managed,
        "env_file": envp.exists(),
        "env_mode": oct(envp.stat().st_mode & 0o777) if envp.exists() else None,
        "with_password": ping(pw) if pw else None,
        "without_password": ping(None),
    }


def password_enable(conf=None, envp=None, restart=None):
    conf, envp, restart = Path(conf or REDIS_CONF), Path(envp or REDIS_ENV), restart or restart_redis
    old_text = conf.read_text()
    cur = read_password(envp)
    if conf_managed(old_text) and cur and ping(cur) == "ok" and ping(None) == "noauth":
        return "sudah aktif"
    pw = secrets.token_urlsafe(24)  # hanya A-Za-z0-9_-: aman di redis.conf tanpa tanda kutip
    new_text = patch_conf(old_text, pw)  # bisa ValueError bila ada requirepass manual: berhenti sebelum mengubah apa pun
    bak = _backup(conf)
    old_env = envp.read_text() if envp.exists() else None
    try:
        _write_env(envp, pw)
        conf.write_text(new_text)
        restart()
        if not wait_for("ok", pw):
            raise RuntimeError("Redis tidak menjawab dengan kata sandi baru setelah restart")
        if ping(None) != "noauth":
            raise RuntimeError("Redis masih menerima koneksi tanpa kata sandi")
    except Exception as e:  # noqa: BLE001 - rollback penuh apa pun penyebabnya
        shutil.copy2(bak, conf)
        if old_env is None:
            envp.unlink(missing_ok=True)
        else:
            _write_env(envp, old_env.split("=", 1)[1].strip())
        try:
            restart()
        except Exception:  # noqa: BLE001
            pass
        wait_for("ok", None, 15)
        die(f"gagal mengaktifkan kata sandi Redis, konfigurasi dikembalikan (cadangan {bak.name}): {e}")
    return f"aktif (cadangan konfigurasi: {bak.name})"


def password_disable(conf=None, envp=None, restart=None):
    conf, envp, restart = Path(conf or REDIS_CONF), Path(envp or REDIS_ENV), restart or restart_redis
    old_text = conf.read_text()
    if not conf_managed(old_text):
        return "tidak aktif (tidak ada blok yang dikelola lab)"
    pw = read_password(envp)
    bak = _backup(conf)
    try:
        conf.write_text(patch_conf(old_text, None))
        restart()
        if not wait_for("ok", None):
            raise RuntimeError("Redis tidak menjawab tanpa kata sandi setelah restart")
    except Exception as e:  # noqa: BLE001
        shutil.copy2(bak, conf)
        try:
            restart()
        except Exception:  # noqa: BLE001
            pass
        wait_for("ok", pw, 15)
        die(f"gagal menonaktifkan kata sandi Redis, konfigurasi dikembalikan: {e}")
    envp.unlink(missing_ok=True)
    return f"nonaktif (cadangan konfigurasi: {bak.name})"


def password_cmd(a):
    if a.mode == "status":
        print(json.dumps(password_status(), indent=2))
        return
    try:
        msg = password_enable() if a.mode == "enable" else password_disable()
    except ValueError as e:
        die(str(e))
    print("OK:", msg)


def redis_cmd(a):
    if a.act == "password":
        return password_cmd(a)
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
