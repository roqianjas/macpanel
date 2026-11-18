"""lab db: buat, hapus, dump, restore, daftar database (MariaDB, MySQL, PostgreSQL)."""

import gzip
import os
import re
import secrets
import subprocess
import time
from pathlib import Path

from labcli.core import ROOT, die
from labcli.dbcore import (
    ENG,
    PG,
    PROJ,
    check_name,
    exists,
    is_my,
    load_root,
    my_run,
    pg_run,
    project_cred,
    write_env_file,
)
from labcli.dbusers import purge_users

BK = ROOT / "backup/db"


def cmd_list(a):
    for eng, g in ENG.items():
        print(g["label"] + ":")
        r = my_run("SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema','mysql','performance_schema','sys') ORDER BY 1", ["-N"], eng=eng)
        print("  " + (r.stdout.strip().replace("\n", "\n  ") or "(kosong)") if not r.returncode else "  (tidak terjangkau: " + r.stderr.strip()[:80] + ")")
    print("PostgreSQL:")
    r = pg_run("SELECT datname FROM pg_database WHERE NOT datistemplate AND datname NOT IN ('postgres') ORDER BY 1", ["-At"])
    print("  " + (r.stdout.strip().replace("\n", "\n  ") or "(kosong)") if not r.returncode else r.stderr)


def cmd_create(a):
    n, eng = a.name, a.engine
    check_name(n)
    if exists(eng, n):
        die(f"database '{n}' sudah ada di {eng}")
    pw = secrets.token_urlsafe(24).replace("-", "x").replace("_", "y")
    if is_my(eng):
        r = my_run(
            f"CREATE DATABASE `{n}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; "
            f"CREATE USER '{n}'@'127.0.0.1' IDENTIFIED BY '{pw}'; CREATE USER '{n}'@'localhost' IDENTIFIED BY '{pw}'; "
            f"GRANT ALL ON `{n}`.* TO '{n}'@'127.0.0.1'; GRANT ALL ON `{n}`.* TO '{n}'@'localhost';",
            eng=eng,
        )
        port, conn = ENG[eng]["port"], "mysql"  # driver 'mysql' juga dipahami MariaDB; sah untuk semua versi Laravel/WordPress
    else:
        r = pg_run(f'CREATE ROLE "{n}" LOGIN PASSWORD \'{pw}\'; GRANT "{n}" TO dev WITH SET TRUE; CREATE DATABASE "{n}" OWNER "{n}" ENCODING \'UTF8\'; REVOKE ALL ON DATABASE "{n}" FROM PUBLIC;')
        port, conn = 5432, "pgsql"
    if not r.returncode and eng == "pgsql":  # idempoten: user proyek tidak boleh konek ke database lain
        for d in ("dev", "postgres"):
            pg_run(f'REVOKE CONNECT ON DATABASE "{d}" FROM PUBLIC;')
    if r.returncode:
        cleanup(eng, n)
        die("gagal: " + r.stderr.strip()[:200])
    kv = {"DB_CONNECTION": conn, "DB_HOST": "127.0.0.1", "DB_PORT": str(port), "DB_DATABASE": n, "DB_USERNAME": n, "DB_PASSWORD": pw}
    PROJ.mkdir(parents=True, exist_ok=True)
    os.chmod(PROJ, 0o700)
    write_env_file(PROJ / f"{n}.{eng}.env", kv)
    print(f"OK: {eng} database+user '{n}' dibuat. Kredensial: {PROJ}/{n}.{eng}.env (0600)")
    if a.env:
        write_env_file(a.env, kv)
        print(f"DB_* ditulis ke {a.env} (0600). Pastikan file itu ada di .gitignore.")


def cleanup(eng, n):
    if is_my(eng):
        my_run(f"DROP DATABASE IF EXISTS `{n}`; DROP USER IF EXISTS '{n}'@'127.0.0.1'; DROP USER IF EXISTS '{n}'@'localhost';", eng=eng)
    else:
        pg_run(f'DROP DATABASE IF EXISTS "{n}"; DROP ROLE IF EXISTS "{n}";')


def cmd_dump(a):
    n, eng = a.name, a.engine
    check_name(n)
    if not exists(eng, n):
        die("database tidak ada")
    BK.mkdir(parents=True, exist_ok=True)
    os.chmod(BK, 0o700)
    out = BK / f"{n}-{eng}-{time.strftime('%Y%m%d-%H%M%S')}.sql.gz"
    if is_my(eng):
        g = ENG[eng]
        root = load_root()
        e = dict(os.environ, MYSQL_PWD=root.get(g["key"], ""))
        extra = ["--no-tablespaces"] if eng == "mysql" else []
        p = subprocess.run([g["bin"] + g["dump"], "-h127.0.0.1", f"-P{g['port']}", "-uroot", "--single-transaction", *extra, "--routines", "--events", n], env=e, capture_output=True)
    else:
        root = load_root()
        e = dict(os.environ, PGPASSWORD=root["POSTGRES_PASSWORD"])
        p = subprocess.run([PG + "pg_dump", "-h127.0.0.1", "-Udev", n], env=e, capture_output=True)
    if p.returncode:
        die("dump gagal: " + p.stderr.decode()[:200])
    with gzip.open(out, "wb") as f:
        f.write(p.stdout)
    os.chmod(out, 0o600)
    print("OK:", out, f"({out.stat().st_size} byte)")


def cmd_drop(a):
    n, eng = a.name, a.engine
    check_name(n)
    if not exists(eng, n):
        die("database tidak ada")
    if input(f"Hapus {eng} database dan user '{n}' PERMANEN. Ketik ulang namanya: ").strip() != n:
        die("nama tidak cocok, dibatalkan")
    a.engine = eng
    cmd_dump(a)  # selalu dump dulu
    purge_users(eng, n)
    cleanup(eng, n)
    f = PROJ / f"{n}.{eng}.env"
    if f.exists():
        f.unlink()
    print(f"OK: '{n}' dihapus (dump terakhir ada di {BK})")


def dump_files(n, eng):
    return sorted(BK.glob(f"{n}-{eng}-[0-9]*.sql.gz"), key=lambda f: f.name)


def table_count(eng, n):
    if is_my(eng):
        r = my_run("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='%s'" % n, ["-N"], eng=eng)
    else:
        r = pg_run("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')", ["-At"], db=n)
    if r.returncode:
        die("gagal memeriksa isi database: " + r.stderr.strip()[:200])
    return int(r.stdout.strip() or 0)


MAX_DUMP = 4 << 30  # batas ukuran setelah dekompresi


MY_WORD = re.compile(r"^\s*(system|source|tee|pager|edit|notee|nopager|\\)", re.IGNORECASE)


MY_BAD = re.compile(r"\\[!.PeT]")  # \! (shell), \. (source), \P (pager), \e (edit), \T (tee)


PG_OK = set("NntrbfvxX01234567.")


PG_SAFE_LINE = re.compile(r"^\\(un)?restrict \S+\s*$")


DEFINER = re.compile(r"DEFINER\s*=\s*(`[^`]*`|[^\s@]+)@(`[^`]*`|[^\s*]+)", re.IGNORECASE)


PG_STRIP = re.compile(r"^(ALTER .* OWNER TO |GRANT |REVOKE )", re.IGNORECASE)


class Reject(Exception):
    pass


def line_check(eng, s, no):
    t = s.replace("\\\\", "")  # pasangan backslash ganda bukan awal perintah
    if is_my(eng):
        if MY_WORD.match(s) or MY_BAD.search(t):
            raise Reject(f"baris {no} memuat perintah klien MySQL yang tidak diizinkan (shell/berkas)")
    else:
        if PG_SAFE_LINE.match(s):
            return
        for m in re.finditer(r"\\(.?)", t):
            if m.group(1) not in PG_OK:
                raise Reject(f"baris {no} memuat meta-perintah psql yang tidak diizinkan (mis. shell atau copy berkas)")


def clean_lines(path, eng):
    """Alirkan dump per baris: periksa, hapus DEFINER (MySQL) dan OWNER/GRANT/REVOKE (PostgreSQL) yang butuh hak lebih tinggi."""
    total = 0
    with gzip.open(path, "rb") as g:
        no = 0
        while True:
            raw = g.readline(1 << 26)
            if not raw:
                break
            no += 1
            total += len(raw)
            if total > MAX_DUMP:
                raise Reject("dump terlalu besar setelah dekompresi (>4 GB)")
            s = raw.decode("latin-1")
            line_check(eng, s, no)
            if is_my(eng) and "DEFINER" in s.upper():
                s = DEFINER.sub("", s)
            elif eng == "pgsql" and PG_STRIP.match(s):
                s = "\n"
            yield s.encode("latin-1")


def scan_dump(path, eng):
    try:
        for _ in clean_lines(path, eng):
            pass
    except Reject as e:
        die("dump ditolak: " + str(e))
    except (OSError, EOFError) as e:
        die("dump tidak bisa dibaca (bukan gzip yang valid?): " + str(e)[:80])


def cmd_restore(a):
    """Pulihkan dump ke database proyek yang sudah ada dan KOSONG. Tidak pernah menimpa data."""
    n, eng = a.name, a.engine
    check_name(n)
    if not exists(eng, n):
        die(f"database '{n}' belum ada; buat dulu: lab db create {n} --engine {eng}")
    if a.file:
        f = Path(a.file)
        f = (BK / f.name) if f.parent in (Path("."), BK) else f
        try:
            f = f.resolve(strict=True)
        except OSError:
            die("berkas dump tidak ada")
        if f.parent != BK.resolve():
            die("dump harus ada di ~/MacPanel/backup/db")
        if not re.fullmatch(re.escape(n) + "-" + eng + r"-\d{8}-\d{6}\.sql\.gz", f.name):
            die(f"nama berkas harus {n}-{eng}-<tanggal>-<jam>.sql.gz (dump milik database ini)")
    else:
        fs = dump_files(n, eng)
        if not fs:
            die(f"tidak ada dump untuk '{n}' ({eng}) di {BK}")
        f = fs[-1]
    c = table_count(eng, n)
    if c:
        die(f"database '{n}' tidak kosong ({c} tabel); restore tidak menimpa data. Hapus isinya dulu atau pakai database baru")
    cred = project_cred(n, eng)  # dijalankan sebagai user proyek (hak hanya pada database itu), bukan root
    scan_dump(f, eng)  # tolak dump yang memuat perintah klien (shell, baca/tulis berkas) SEBELUM menyentuh database
    if is_my(eng):
        g = ENG[eng]
        e = dict(os.environ, MYSQL_PWD=cred["DB_PASSWORD"])
        cmd = [g["bin"] + g["cli"], "-h127.0.0.1", f"-P{g['port']}", "-u" + cred["DB_USERNAME"], n]
    else:
        e = dict(os.environ, PGPASSWORD=cred["DB_PASSWORD"])
        cmd = [PG + "psql", "-X", "-h127.0.0.1", "-U", cred["DB_USERNAME"], "-v", "ON_ERROR_STOP=1", "-q", "-d", n]
    pr = subprocess.Popen(cmd, env=e, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        for raw in clean_lines(f, eng):
            pr.stdin.write(raw)
    except (BrokenPipeError, OSError, EOFError):
        pass
    try:
        pr.stdin.close()
    except OSError:
        pass
    err = pr.stderr.read().decode(errors="replace")
    rc = pr.wait()
    if rc:
        die("restore gagal: " + err.strip()[:300] + f" (database '{n}' mungkin terisi sebagian; hapus lalu buat ulang)")
    print(f"OK: {f.name} dipulihkan ke '{n}' ({table_count(eng, n)} tabel)")
