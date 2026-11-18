"""Bagian bersama perintah database: mesin, koneksi, berkas kredensial proyek."""

import os
import re
import subprocess
from pathlib import Path

from labcli.core import HOME, die

CRED = HOME / ".config/macminiops"


PROJ = CRED / "projects"


MY = "/opt/homebrew/opt/mysql@8.4/bin/"


MA = "/opt/homebrew/opt/mariadb@11.8/bin/"


# Keluarga MySQL: dua mesin terpisah. MariaDB memakai port bawaan 3306; MySQL 3307 (lihat my.cnf.d/10-local.cnf).
ENG = {
    "mariadb": dict(bin=MA, cli="mariadb", dump="mariadb-dump", port=3306, key="MARIADB_ROOT_PASSWORD", label="MariaDB"),
    "mysql": dict(bin=MY, cli="mysql", dump="mysqldump", port=3307, key="MYSQL_ROOT_PASSWORD", label="MySQL"),
}


ENGINES = list(ENG) + ["pgsql"]


def is_my(e):
    return e in ENG


PG = "/opt/homebrew/opt/postgresql@17/bin/"


DBNAME_RE = re.compile(r"^[a-z][a-z0-9_]{0,31}$")


RESERVED = {"mysql", "sys", "information_schema", "performance_schema", "postgres", "template0", "template1", "root", "dev", "admin"}


def load_root():
    env = {}
    for ln in (CRED / "db.env").read_text().splitlines():
        if "=" in ln and not ln.startswith("#"):
            k, v = ln.split("=", 1)
            env[k.strip()] = v.strip().strip("'\"")
    return env


def check_name(n):
    if not DBNAME_RE.match(n):
        die("nama harus huruf kecil/angka/_ , diawali huruf, maks 32 karakter")
    if n in RESERVED:
        die(f"nama '{n}' dicadangkan")


def my_run(sql=None, args=(), stdin=None, binary="cli", eng="mysql"):
    root, g = load_root(), ENG[eng]
    if g["key"] not in root:
        die(f"kata sandi root {g['label']} tidak ada di db.env")
    e = dict(os.environ, MYSQL_PWD=root[g["key"]])
    cmd = [g["bin"] + g["dump" if binary == "dump" else "cli"], "-h127.0.0.1", f"-P{g['port']}", "-uroot", *args]
    return subprocess.run(cmd, env=e, input=sql, text=True, capture_output=True, stdin=stdin)


def pg_run(sql=None, args=(), db="postgres", binary="psql"):
    root = load_root()
    e = dict(os.environ, PGPASSWORD=root["POSTGRES_PASSWORD"])
    cmd = [PG + binary, "-h127.0.0.1", "-Udev", "-v", "ON_ERROR_STOP=1", *args] if binary == "psql" else [PG + binary, "-h127.0.0.1", "-Udev", *args]
    if binary == "psql":
        cmd += ["-d", db]
    return subprocess.run(cmd, env=e, input=sql, text=True, capture_output=True)


def exists(engine, n):
    if is_my(engine):
        r = my_run("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='%s'" % n, ["-N"], eng=engine)
    else:
        r = pg_run("SELECT COUNT(*) FROM pg_database WHERE datname='%s'" % n, ["-At"])
    if r.returncode:
        die(r.stderr.strip()[:200])
    return r.stdout.strip() == "1"


def write_env_file(path, kv):
    p = Path(path).expanduser()
    lines = p.read_text().splitlines() if p.exists() else []
    keys = {k for k in kv}
    lines = [l for l in lines if l.split("=", 1)[0] not in keys]
    lines += [f"{k}={v}" for k, v in kv.items()]
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text("\n".join(lines) + "\n")
    os.chmod(p, 0o600)


def project_cred(n, eng):
    p = PROJ / f"{n}.{eng}.env"
    if not p.exists():
        die(f"kredensial proyek {n} ({eng}) tidak ada; database harus dibuat dengan `lab db create`")
    kv = {}
    for ln in p.read_text().splitlines():
        if "=" in ln and not ln.startswith("#"):
            k, v = ln.split("=", 1)
            kv[k.strip()] = v.strip().strip("'\"")
    if not kv.get("DB_USERNAME") or not kv.get("DB_PASSWORD"):
        die("kredensial proyek tidak lengkap")
    return kv
