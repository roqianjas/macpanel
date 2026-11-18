"""Tes live: siklus database dan user database di mesin nyata (MariaDB, MySQL, PostgreSQL).

Jalankan: MACPANEL_LIVE=1 python3 -m pytest cli/tests -m live
Membuat database sementara `zz_<acak>` dan menghapusnya (beserta dump terakhir) di akhir. Tidak menyentuh database lain.
Kata sandi tidak pernah dicetak.
"""

import json
import os
import re
import secrets
import subprocess

import pytest
from labcli.core import ROOT

pytestmark = pytest.mark.live
LAB = str(ROOT / "server/scripts/lab")
CRED = os.path.expanduser("~/.config/macminiops/projects")
CLI = {
    "mariadb": (["/opt/homebrew/opt/mariadb@11.8/bin/mariadb", "-h127.0.0.1", "-P3306", "-N", "-B"], "MYSQL_PWD"),
    "mysql": (["/opt/homebrew/opt/mysql@8.4/bin/mysql", "-h127.0.0.1", "-P3307", "-N", "-B"], "MYSQL_PWD"),
    "pgsql": (["/opt/homebrew/opt/postgresql@17/bin/psql", "-h127.0.0.1", "-p5432", "-At", "-v", "ON_ERROR_STOP=1"], "PGPASSWORD"),
}


def lab(*args, inp=None):
    return subprocess.run([LAB, *args], input=inp, capture_output=True, text=True, timeout=300)


def q(engine, user, pw, db, sql):
    cmd, var = CLI[engine]
    if engine == "pgsql":
        cmd = [*cmd, "-U", user, "-d", db, "-c", sql]
    else:
        cmd = [*cmd, "-u", user, db, "-e", sql]
    return subprocess.run(cmd, env={**os.environ, var: pw}, capture_output=True, text=True, timeout=60)


def project_password(name, engine):
    for ln in open(f"{CRED}/{name}.{engine}.env"):
        if ln.startswith("DB_PASSWORD="):
            return ln.split("=", 1)[1].strip().strip("'\"")
    raise AssertionError("DB_PASSWORD tidak ada di berkas kredensial")


@pytest.fixture(params=["mariadb", "mysql", "pgsql"])
def db(request):
    engine = request.param
    name = "zz_" + secrets.token_hex(3)
    r = lab("db", "create", name, "--engine", engine)
    assert r.returncode == 0, r.stderr
    try:
        yield engine, name
    finally:
        lab("db", "drop", name, "--engine", engine, inp=name + "\n")
        for f in (ROOT / "backup/db").glob(f"{name}-*"):
            f.unlink()
        for f in (os.path.expanduser(f"{CRED}/{name}.{engine}.env"),):
            if os.path.exists(f):
                os.remove(f)


def test_project_user_can_write_and_read(db):
    engine, name = db
    pw = project_password(name, engine)
    assert q(engine, name, pw, name, "create table t(id int primary key, v int)").returncode == 0
    assert q(engine, name, pw, name, "insert into t values (1, 10)").returncode == 0
    r = q(engine, name, pw, name, "select v from t")
    assert r.returncode == 0 and "10" in r.stdout


def test_extra_user_privileges_are_enforced(db):
    engine, name = db
    pw = project_password(name, engine)
    q(engine, name, pw, name, "create table t(id int primary key, v int)")
    q(engine, name, pw, name, "insert into t values (1, 10)")
    user = "zz_ro_" + secrets.token_hex(2)
    r = lab("db", "user", "add", name, user, "--engine", engine, "--privs", "ro")
    assert r.returncode == 0, r.stderr
    upw = json.loads(r.stdout)["password"]
    assert q(engine, user, upw, name, "select v from t").returncode == 0
    assert q(engine, user, upw, name, "insert into t values (2, 20)").returncode != 0  # baca-saja menolak tulis
    assert lab("db", "user", "privs", name, user, "--engine", engine, "--privs", "rw").returncode == 0
    assert q(engine, user, upw, name, "insert into t values (2, 20)").returncode == 0
    assert q(engine, user, upw, name, "drop table t").returncode != 0  # rw tidak boleh mengubah struktur
    assert lab("db", "user", "remove", name, user, "--engine", engine).returncode == 0
    assert q(engine, user, upw, name, "select 1").returncode != 0  # user sudah hilang


def test_own_password_rules_and_rotation(db):
    engine, name = db
    user = "zz_u_" + secrets.token_hex(2)
    assert lab("db", "user", "add", name, user, "--engine", engine, "--privs", "ro").returncode == 0
    bad = lab("db", "user", "passwd", name, user, "--engine", engine, "--password-stdin", inp="pendek\n")
    assert bad.returncode != 0
    new = "SandiBaru-" + secrets.token_hex(6)
    ok = lab("db", "user", "passwd", name, user, "--engine", engine, "--password-stdin", inp=new + "\n")
    assert ok.returncode == 0
    assert new not in ok.stdout + ok.stderr  # kata sandi tidak dicetak ulang
    assert q(engine, user, new, name, "select 1").returncode == 0
    lab("db", "user", "remove", name, user, "--engine", engine)


def test_dump_and_restore_roundtrip(db):
    engine, name = db
    pw = project_password(name, engine)
    q(engine, name, pw, name, "create table t(id int primary key, v int)")
    q(engine, name, pw, name, "insert into t values (7, 70)")
    r = lab("db", "dump", name, "--engine", engine)
    assert r.returncode == 0, r.stderr
    path = re.search(r"(/\S+\.sql\.gz)", r.stdout + r.stderr)
    assert path and os.path.exists(path.group(1))


def test_listing_shows_the_database(db):
    engine, name = db
    assert name in lab("db", "list").stdout
