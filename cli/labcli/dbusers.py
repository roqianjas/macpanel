"""lab db user / root-passwd: user database tambahan, hak, dan kata sandi root."""

import json
import os
import re
import secrets
import subprocess
import sys
import time

from labcli.core import die
from labcli.dbcore import (
    CRED,
    ENG,
    PG,
    PROJ,
    RESERVED,
    check_name,
    exists,
    is_my,
    load_root,
    my_run,
    pg_run,
    project_cred,
    write_env_file,
)


def cmd_users(a):
    n, eng = a.name, a.engine
    check_name(n)
    if not exists(eng, n):
        die("database tidak ada")
    if is_my(eng):
        r = my_run(f"SELECT user,host FROM mysql.user WHERE user='{n}'", ["-N", "-B"], eng=eng)
    else:
        r = pg_run(f"SELECT rolname FROM pg_roles WHERE rolname='{n}'", ["-At"])
    if r.returncode:
        die("gagal membaca user: " + r.stderr.strip()[:200])
    rows = [ln.split("\t") for ln in r.stdout.splitlines() if ln.strip()]
    cf = PROJ / f"{n}.{eng}.env"
    print(json.dumps(dict(users=[dict(user=x[0], host=(x[1] if len(x) > 1 else "")) for x in rows], cred=cf.exists(), rotated=int(cf.stat().st_mtime) if cf.exists() else 0)))


def cmd_passwd(a):
    n, eng = a.name, a.engine
    check_name(n)
    if not exists(eng, n):
        die("database tidak ada")
    kv = project_cred(n, eng)
    chosen, pw = read_pw(a)
    if is_my(eng):
        r = my_run(f"ALTER USER '{n}'@'127.0.0.1' IDENTIFIED BY '{pw}'; ALTER USER '{n}'@'localhost' IDENTIFIED BY '{pw}'; FLUSH PRIVILEGES;", eng=eng)
    else:
        r = pg_run(f"ALTER ROLE \"{n}\" PASSWORD '{pw}';")
    if r.returncode:
        die("ganti kata sandi gagal: " + scrub_pw(r.stderr.strip()[:200], pw))
    kv["DB_PASSWORD"] = pw
    write_env_file(PROJ / f"{n}.{eng}.env", kv)
    print(f"OK: kata sandi user '{n}' ({eng}) diganti dan disimpan di {PROJ}/{n}.{eng}.env. Perbarui .env aplikasi yang memakainya.")


def cmd_creds(a):
    n, eng = a.name, a.engine
    check_name(n)
    print(json.dumps({k: v for k, v in project_cred(n, eng).items() if k.startswith("DB_")}))


# Hanya user yang dibuat lewat perintah ini yang boleh diubah/dihapus (daftar di dbusers.json); akun lain tidak disentuh.
# Host dibatasi 127.0.0.1 dan localhost. Tidak ada hak global. Kata sandi pilihan sendiri dibaca dari stdin (tidak lewat argumen).
USERS_F = CRED / "dbusers.json"


UNAME_RE = re.compile(r"^[a-z][a-z0-9_]{2,31}$")


PW_RE = re.compile(r"^[A-Za-z0-9._+=:,@%^-]{12,100}$")  # aman untuk SQL, db.env yang di-source shell, dan argumen


PRIVS = ("ro", "rw", "all")


ROOT_KEYS = {"mariadb": "MARIADB_ROOT_PASSWORD", "mysql": "MYSQL_ROOT_PASSWORD", "pgsql": "POSTGRES_PASSWORD"}


SYSTEM_USERS = {"root", "mysql", "mariadb.sys", "mysql.sys", "mysql.session", "mysql.infoschema", "dev", "postgres", "admin", "mariadb", "public"}


def new_pw():
    return secrets.token_urlsafe(24).replace("-", "x").replace("_", "y")


def read_pw(a):
    if not getattr(a, "password_stdin", False):
        return None, new_pw()
    pw = sys.stdin.readline().rstrip("\r\n")
    if not PW_RE.match(pw):
        die("kata sandi: 12-100 karakter dari huruf, angka, dan . _ + = : , @ % ^ -")
    return pw, pw


def scrub_pw(text, pw):
    return text.replace(pw, "***") if pw else text


def users_load():
    try:
        return json.loads(USERS_F.read_text())
    except (OSError, ValueError):
        return {}


def users_save(d):
    tmp = USERS_F.with_suffix(".tmp")
    tmp.write_text(json.dumps(d, indent=1))
    os.chmod(tmp, 0o600)
    os.replace(tmp, USERS_F)


def my_priv_sql(p):
    return {"ro": "SELECT, SHOW VIEW", "rw": "SELECT, INSERT, UPDATE, DELETE, SHOW VIEW, EXECUTE", "all": "ALL PRIVILEGES"}[p]


def pg_apply(db, u, p):
    """Atur ulang hak user PostgreSQL pada satu database (sebagai superuser dev)."""
    sq = (
        f'REVOKE ALL ON ALL TABLES IN SCHEMA public FROM "{u}"; REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM "{u}"; '
        f'ALTER DEFAULT PRIVILEGES FOR ROLE "{db}" IN SCHEMA public REVOKE ALL ON TABLES FROM "{u}"; '
        f'ALTER DEFAULT PRIVILEGES FOR ROLE "{db}" IN SCHEMA public REVOKE ALL ON SEQUENCES FROM "{u}"; '
        f'REVOKE "{db}" FROM "{u}"; GRANT CONNECT ON DATABASE "{db}" TO "{u}"; GRANT USAGE ON SCHEMA public TO "{u}"; '
    )
    if p == "ro":
        sq += f'GRANT SELECT ON ALL TABLES IN SCHEMA public TO "{u}"; ALTER DEFAULT PRIVILEGES FOR ROLE "{db}" IN SCHEMA public GRANT SELECT ON TABLES TO "{u}";'
    elif p == "rw":
        sq += (
            f'GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "{u}"; GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO "{u}"; '
            f'ALTER DEFAULT PRIVILEGES FOR ROLE "{db}" IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO "{u}"; '
            f'ALTER DEFAULT PRIVILEGES FOR ROLE "{db}" IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO "{u}";'
        )
    else:
        sq += f'GRANT "{db}" TO "{u}" WITH INHERIT TRUE, SET TRUE;'
    return pg_run(sq, db=db)


def user_gate(eng, db, u, need_registered):
    check_name(db)
    if not exists(eng, db):
        die("database tidak ada")
    if not UNAME_RE.match(u) or u in RESERVED or u in SYSTEM_USERS:
        die("nama user: huruf kecil, angka, _; 3-32 karakter; bukan nama sistem")
    reg = users_load().get(f"{eng}/{u}")
    if need_registered:
        if not reg or reg["db"] != db:
            die("user ini tidak dikelola panel untuk database tersebut")
    return reg


def cmd_user_list(a):
    n, eng = a.name, a.engine
    check_name(n)
    if not exists(eng, n):
        die("database tidak ada")
    out = [dict(user=n, host="127.0.0.1, localhost" if is_my(eng) else "127.0.0.1", privs="all", primary=True)]
    for k, v in sorted(users_load().items()):
        e, u = k.split("/", 1)
        if e == eng and v["db"] == n:
            out.append(dict(user=u, host="127.0.0.1, localhost" if is_my(eng) else "127.0.0.1", privs=v["privs"], primary=False, created=v.get("created", 0)))
    print(json.dumps(dict(users=out, port=ENG[eng]["port"] if is_my(eng) else 5432)))


def cmd_user_add(a):
    n, eng, u, p = a.name, a.engine, a.user, a.privs
    if user_gate(eng, n, u, False):
        die("user sudah ada")
    if exists(eng, u):
        die("nama itu dipakai database proyek lain")
    chosen, pw = read_pw(a)
    if is_my(eng):
        r = my_run(f"SELECT COUNT(*) FROM mysql.user WHERE user='{u}'", ["-N"], eng=eng)
        if r.returncode or r.stdout.strip() != "0":
            die("user dengan nama itu sudah ada di server (bukan buatan panel)")
        g = my_priv_sql(p)
        r = my_run(
            f"CREATE USER '{u}'@'127.0.0.1' IDENTIFIED BY '{pw}'; CREATE USER '{u}'@'localhost' IDENTIFIED BY '{pw}'; "
            f"GRANT {g} ON `{n}`.* TO '{u}'@'127.0.0.1'; GRANT {g} ON `{n}`.* TO '{u}'@'localhost'; FLUSH PRIVILEGES;",
            eng=eng,
        )
    else:
        r = pg_run(f"SELECT COUNT(*) FROM pg_roles WHERE rolname='{u}'", ["-At"])
        if r.returncode or r.stdout.strip() != "0":
            die("role dengan nama itu sudah ada di server (bukan buatan panel)")
        r = pg_run(f'CREATE ROLE "{u}" LOGIN PASSWORD \'{pw}\'; GRANT "{u}" TO dev WITH SET TRUE;')  # dev bukan superuser: butuh keanggotaan agar bisa mengelola/menghapusnya
        if not r.returncode:
            r = pg_apply(n, u, p)
    if r.returncode:
        (my_run(f"DROP USER IF EXISTS '{u}'@'127.0.0.1'; DROP USER IF EXISTS '{u}'@'localhost';", eng=eng) if is_my(eng) else pg_run(f'DROP ROLE IF EXISTS "{u}";'))
        die("gagal membuat user: " + scrub_pw(r.stderr.strip()[:200], pw))
    d = users_load()
    d[f"{eng}/{u}"] = dict(db=n, privs=p, created=int(time.time()))
    users_save(d)
    res = dict(ok=True, user=u, db=n, privs=p, host="127.0.0.1", port=ENG[eng]["port"] if is_my(eng) else 5432)
    if not chosen:
        res["password"] = pw
    print(json.dumps(res))


def cmd_user_passwd(a):
    n, eng, u = a.name, a.engine, a.user
    user_gate(eng, n, u, True)
    chosen, pw = read_pw(a)
    if is_my(eng):
        r = my_run(f"ALTER USER '{u}'@'127.0.0.1' IDENTIFIED BY '{pw}'; ALTER USER '{u}'@'localhost' IDENTIFIED BY '{pw}'; FLUSH PRIVILEGES;", eng=eng)
    else:
        r = pg_run(f"ALTER ROLE \"{u}\" PASSWORD '{pw}';")
    if r.returncode:
        die("gagal mengganti kata sandi: " + scrub_pw(r.stderr.strip()[:200], pw))
    res = dict(ok=True, user=u)
    if not chosen:
        res["password"] = pw
    print(json.dumps(res))


def cmd_user_privs(a):
    n, eng, u, p = a.name, a.engine, a.user, a.privs
    user_gate(eng, n, u, True)
    if is_my(eng):
        g = my_priv_sql(p)
        sq = ""
        for h in ("127.0.0.1", "localhost"):
            sq += f"REVOKE ALL PRIVILEGES ON `{n}`.* FROM '{u}'@'{h}'; GRANT {g} ON `{n}`.* TO '{u}'@'{h}'; "
        r = my_run(sq + "FLUSH PRIVILEGES;", eng=eng)
    else:
        r = pg_apply(n, u, p)
    if r.returncode:
        die("gagal mengubah hak: " + r.stderr.strip()[:200])
    d = users_load()
    d[f"{eng}/{u}"]["privs"] = p
    users_save(d)
    print(json.dumps(dict(ok=True, user=u, privs=p)))


def drop_user(eng, db, u):
    if is_my(eng):
        r = my_run(f"DROP USER IF EXISTS '{u}'@'127.0.0.1'; DROP USER IF EXISTS '{u}'@'localhost'; FLUSH PRIVILEGES;", eng=eng)
    else:
        r = pg_run(f'DROP OWNED BY "{u}";', db=db)
        if not r.returncode:
            r = pg_run(f'DROP ROLE IF EXISTS "{u}";')
    return r


def cmd_user_remove(a):
    n, eng, u = a.name, a.engine, a.user
    user_gate(eng, n, u, True)
    r = drop_user(eng, n, u)
    if r.returncode:
        die("gagal menghapus user: " + r.stderr.strip()[:200])
    d = users_load()
    d.pop(f"{eng}/{u}", None)
    users_save(d)
    print(json.dumps(dict(ok=True, user=u)))


def purge_users(eng, db):
    d = users_load()
    ch = False
    for k, v in list(d.items()):
        e, u = k.split("/", 1)
        if e == eng and v["db"] == db:
            if drop_user(eng, db, u).returncode:
                die(f"gagal menghapus user '{u}'; database tidak dihapus")
            d.pop(k)
            ch = True
    if ch:
        users_save(d)


def sql_as(eng, pw, sql):
    """Jalankan SQL sebagai root dengan kata sandi eksplisit (bukan dari db.env)."""
    if is_my(eng):
        g = ENG[eng]
        return subprocess.run([g["bin"] + g["cli"], "-h127.0.0.1", f"-P{g['port']}", "-uroot", "-N", "-e", sql], env=dict(os.environ, MYSQL_PWD=pw), text=True, capture_output=True)
    return subprocess.run([PG + "psql", "-h127.0.0.1", "-Udev", "-d", "postgres", "-At", "-v", "ON_ERROR_STOP=1", "-c", sql], env=dict(os.environ, PGPASSWORD=pw), text=True, capture_output=True)


def write_root_env(key, val):
    p = CRED / "db.env"
    old = p.read_text()
    bak = CRED / f"db.env.bak.{time.strftime('%Y%m%d-%H%M%S')}"
    bak.write_text(old)
    os.chmod(bak, 0o600)
    for b in sorted(CRED.glob("db.env.bak.*"))[:-2]:
        b.unlink()  # simpan 2 terakhir saja (memuat sandi lama)
    lines = [l for l in old.splitlines() if l.split("=", 1)[0] != key] + [f"{key}={val}"]
    tmp = CRED / "db.env.tmp"
    tmp.write_text("\n".join(lines) + "\n")
    os.chmod(tmp, 0o600)
    os.replace(tmp, p)


def cmd_root_passwd(a):
    eng = a.engine
    key = ROOT_KEYS[eng]
    cur = load_root().get(key)
    if not cur:
        die(f"{key} tidak ada di db.env")
    if sql_as(eng, cur, "SELECT 1").returncode:
        die("kata sandi root saat ini di db.env tidak cocok dengan server; tidak diubah")
    chosen, pw = read_pw(a)
    if pw == cur:
        die("kata sandi baru sama dengan yang lama")
    if is_my(eng):
        r = sql_as(eng, cur, f"ALTER USER 'root'@'localhost' IDENTIFIED BY '{pw}'; " + ("ALTER USER 'root'@'127.0.0.1' IDENTIFIED BY '%s'; " % pw if eng == "mariadb" else "") + "FLUSH PRIVILEGES;")
    else:
        r = sql_as(eng, cur, f"ALTER ROLE dev PASSWORD '{pw}'")
    if r.returncode:
        die("gagal mengganti kata sandi root (tidak berubah): " + scrub_pw(r.stderr.strip()[:200], pw))
    write_root_env(key, pw)
    if sql_as(eng, pw, "SELECT 1").returncode:  # tidak bisa masuk dengan sandi baru: kembalikan
        write_root_env(key, cur)
        die("verifikasi sandi baru gagal; db.env dikembalikan. Periksa server database sebelum mencoba lagi")
    res = dict(ok=True, engine=eng)
    if not chosen:
        res["password"] = pw
    print(json.dumps(res))
