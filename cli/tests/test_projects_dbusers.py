"""Proyek Go/Python/Next.js dan validator user database: logika murni tanpa menyentuh mesin."""

import io
import sys

import pytest
from labcli import dbusers, projects


def test_free_port_skips_ports_used_by_sites(tmp_path, monkeypatch):
    (tmp_path / "a.conf").write_text("location / { proxy_pass http://127.0.0.1:3100; }")
    (tmp_path / "b.conf.disabled").write_text("proxy_pass http://127.0.0.1:3101;")
    monkeypatch.setattr(projects, "SITES", tmp_path)
    assert projects.free_port() == 3102


def test_project_commands_per_kind():
    b, run = projects.project_cmds("go", "api", "/w/api", "1.27.4", 3100)
    assert b[-5:] == ["go", "build", "-o", "bin/app", "."] and run == "/w/api/bin/app"
    b, run = projects.project_cmds("python", "svc", "/w/svc", "3.13", 3100)
    assert b is None and run == "/w/svc/.venv/bin/python app.py"
    b, run = projects.project_cmds("nextjs", "web", "/w/web", "24", 3105)
    assert b[-3:] == ["pnpm", "run", "build"]
    assert "-p 3105" in run and run.endswith("-H 127.0.0.1")  # selalu loopback


@pytest.mark.parametrize("bad", ["24;rm", "../x", "v24", "1.2.3.4", "", "24 25", "$(id)"])
def test_runtime_version_rejects_junk(bad):
    if bad == "":
        pytest.skip("kosong = pakai versi bawaan")
    with pytest.raises(SystemExit):
        projects.runtime_version("go", bad)


def test_project_kinds_are_the_three_supported():
    assert set(projects.PROJ_KINDS) == {"go", "python", "nextjs"}


# ---------- user database ----------


@pytest.mark.parametrize("name", ["abc", "a_b", "reader01", "a" * 32])
def test_valid_db_usernames(name):
    assert dbusers.UNAME_RE.match(name)


@pytest.mark.parametrize("name", ["", "ab", "1ab", "Ab", "a-b", "a b", "a" * 33, "a;b", "root'--"])
def test_invalid_db_usernames(name):
    assert not dbusers.UNAME_RE.match(name)


def test_password_policy():
    ok = ["SandiPilihanSendiri-9876", "a" * 12, "Abc.def_ghi+1=2:3,4@5%6^7-8"]
    bad = ["pendek", "a" * 101, "ada spasi di sini1", "a'; drop table t;--", 'dengan"kutip12345', "sandi\\garis12345", "satu$dolar12345", "x`y12345678"]
    assert all(dbusers.PW_RE.match(p) for p in ok)
    assert not any(dbusers.PW_RE.match(p) for p in bad), [p for p in bad if dbusers.PW_RE.match(p)]


def test_generated_password_matches_policy_and_is_random():
    a, b = dbusers.new_pw(), dbusers.new_pw()
    assert dbusers.PW_RE.match(a) and a != b


def test_read_pw_from_stdin(monkeypatch):
    from types import SimpleNamespace

    monkeypatch.setattr(sys, "stdin", io.StringIO("SandiPilihanSendiri-9876\n"))
    assert dbusers.read_pw(SimpleNamespace(password_stdin=True)) == ("SandiPilihanSendiri-9876",) * 2
    monkeypatch.setattr(sys, "stdin", io.StringIO("pendek\n"))
    with pytest.raises(SystemExit):
        dbusers.read_pw(SimpleNamespace(password_stdin=True))
    pw, shown = dbusers.read_pw(SimpleNamespace(password_stdin=False))
    assert pw is None and dbusers.PW_RE.match(shown)


def test_scrub_pw_removes_password_from_text():
    assert "RAHASIA" not in dbusers.scrub_pw("gagal login dengan RAHASIA-123 ditolak", "RAHASIA-123")


def test_system_users_are_reserved():
    for u in ["root", "postgres", "mysql", "dev"]:
        assert u in dbusers.SYSTEM_USERS
    assert set(dbusers.PRIVS) == {"ro", "rw", "all"}
