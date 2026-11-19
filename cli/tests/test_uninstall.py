"""lab uninstall: rencana, pengaman path, backup terverifikasi, urutan, dan eksekusi pada HOME palsu (tidak menyentuh mesin)."""

import json
import os
import tarfile
import types

import pytest
from labcli import uninstall as U


def make_env(tmp_path, pf=True, extra_agents=()):
    home = tmp_path / "home"
    mp = home / "MacPanel"
    brew = tmp_path / "brew"
    for d in (
        mp / "wwwroot/site1/node_modules/x",
        mp / "wwwroot/site1/public",
        mp / "server/scripts",
        mp / "notes",
        mp / "docs",
        mp / "backup",
        home / ".config/macminiops",
        brew / "var/mysql",
        brew / "var/postgresql@17",
        brew / "bin",
        brew / "etc/nginx",
        home / "Library/LaunchAgents",
        home / "Library/Logs",
        home / ".cache/macpanel",
        tmp_path / "caroot",
    ):
        d.mkdir(parents=True, exist_ok=True)
    (mp / "wwwroot/site1/public/index.php").write_text("<?php echo 1;")
    (mp / "wwwroot/site1/node_modules/x/big.js").write_text("x" * 5000)
    (mp / "notes/n.md").write_text("catatan")
    (home / ".config/macminiops/db.env").write_text("RAHASIA=1")
    (brew / "var/mysql/data").write_text("db")
    (brew / "etc/nginx/nginx.conf.default").write_text("default")
    os.symlink(mp / "server/nginx.conf", brew / "etc/nginx/nginx.conf")
    for n in (*U.PANEL_AGENTS, *U.APP_AGENTS, *U.KEEP_AGENTS, *extra_agents):
        (home / "Library/LaunchAgents" / f"{n}.plist").write_text("x")
    for n in ("panel.log", "9router.log"):
        (home / "Library/Logs" / n).write_text("log")
    (home / "pribadi.txt").write_text("jangan hapus")
    os.symlink(mp / "server/scripts/lab", brew / "bin/lab")
    return U.Env(
        home,
        mp,
        501,
        brew,
        agents=set(U.PANEL_AGENTS + U.APP_AGENTS + U.KEEP_AGENTS),
        formulas={"nginx", "mysql@8.4", "colima"},
        services={"nginx", "dnsmasq", "redis"},
        pf_block=pf,
        pf_rustdesk=True,
        keychain=True,
        nginx_link=True,
        cli_links=[brew / "bin/lab"],
        mkcert_caroot=tmp_path / "caroot",
    )


class Rec:
    def __init__(self, fail_on=None):
        self.calls, self.fail_on = [], fail_on

    def run(self, argv, sudo=False):
        self.calls.append((sudo, argv))
        return 1 if self.fail_on and self.fail_on in " ".join(argv) else 0


def fresh_dump(env):
    def f():
        (env.mp / "backup/dump-baru.sql.gz").write_bytes(b"dump")
        return True

    return f


def test_levels_are_nested_and_protect_hermes(tmp_path):
    env = make_env(tmp_path)
    ids = {lv: [(s["op"], s["desc"]) for s in U.build_plan(env, lv, str(tmp_path / "out") if lv == 3 else None)] for lv in (1, 2, 3)}
    assert set(ids[1]) < set(ids[2]) < set(ids[3])
    for lv in (1, 2, 3):
        text = json.dumps(U.build_plan(env, lv, str(tmp_path / "out")))
        assert "hermes-dashboard" not in text, "LaunchAgent Hermes tidak boleh disentuh"
        assert "/etc/resolver" not in text and "tailscale" not in text.lower()
    p1 = json.dumps(U.build_plan(env, 1))
    assert "wwwroot" not in p1 and "brew uninstall" not in p1 and str(env.mp) not in [s.get("path") for s in U.build_plan(env, 1)]
    assert not any(s["op"] == "rm" and s["path"] == str(env.mp) for s in U.build_plan(env, 2))


def test_level3_requires_safe_backup_dir(tmp_path):
    env = make_env(tmp_path)
    with pytest.raises(U.UninstallError, match="wajib"):
        U.build_plan(env, 3)
    for bad in (env.mp, env.mp / "backup", env.home / ".config/macminiops/x"):
        with pytest.raises(U.UninstallError, match="tidak boleh"):
            U.build_plan(env, 3, str(bad))
    with pytest.raises(U.UninstallError):
        U.build_plan(env, 4)


def test_order_backup_before_delete_and_mp_last(tmp_path):
    env = make_env(tmp_path)
    plan = U.build_plan(env, 3, str(tmp_path / "out"))
    ops = [(s["op"], s.get("path"), s["desc"]) for s in plan]
    ib = next(i for i, o in enumerate(ops) if o[0] == "backup")
    first_destructive = next(i for i, s in enumerate(plan) if s["phase"] in ("L3 hapus", "L3 akhir"))
    assert ib < first_destructive
    imp = next(i for i, s in enumerate(plan) if s["op"] == "rm" and s["path"] == str(env.mp))
    assert imp > max(i for i, s in enumerate(plan) if s["phase"] == "L3 hapus")
    assert all(s["phase"] == "L3 akhir" for s in plan[imp:])
    assert plan[-1]["path"].endswith("bin/lab"), "tautan CLI paling akhir"


def test_firewall_steps_keep_rustdesk_protected(tmp_path):
    env = make_env(tmp_path)
    argvs = [s["argv"] for s in U.build_plan(env, 1) if s["op"] == "sudo"]
    assert ["/sbin/pfctl", "-f", "/etc/pf.macminiops.conf"] in argvs, "RustDesk masih terpasang: jangan kembali ke pf.conf bawaan"
    assert ["/sbin/pfctl", "-f", "/etc/pf.conf"] not in argvs
    env.pf_rustdesk = False
    assert ["/sbin/pfctl", "-f", "/etc/pf.conf"] in [s["argv"] for s in U.build_plan(env, 1) if s["op"] == "sudo"]
    env.pf_block = False
    assert not [s for s in U.build_plan(env, 1) if s["op"] == "sudo"]


def test_all_sudo_and_exec_steps_are_fixed_absolute_argv(tmp_path):
    env = make_env(tmp_path)
    for s in U.build_plan(env, 3, str(tmp_path / "o")):
        if s["op"] in ("sudo", "exec"):
            assert isinstance(s["argv"], list) and all(isinstance(a, str) for a in s["argv"])
            assert s["argv"][0].startswith("/"), s
            assert not any(x in s["argv"][0] for x in ("sh -c",)) and s["argv"][0] != "/bin/sh"
    plan = U.build_plan(env, 1)
    assert all(s["op"] != "sudo" or s["argv"][0].startswith("/") for s in plan)


def test_guard_path(tmp_path):
    env = make_env(tmp_path)
    for bad in (env.home, "/", env.home / "pribadi.txt", env.home / "Library", env.brew / "var", tmp_path / "lain", env.home / "MacPanel/../pribadi.txt", env.home / "Documents"):
        with pytest.raises(U.UninstallError):
            U.guard_path(env, bad)
    for ok in (env.mp, env.mp / "wwwroot", env.home / ".config/macminiops", env.brew / "var/mysql", env.home / "Library/LaunchAgents/com.macminiops.panel.plist"):
        U.guard_path(env, ok)


def test_execute_level1_leaves_data(tmp_path):
    env = make_env(tmp_path)
    out = tmp_path / "log.txt"
    with open(out, "w") as f:
        rc = U.execute(env, U.build_plan(env, 1), Rec(), f)
    assert rc == 0
    la = env.home / "Library/LaunchAgents"
    assert not (la / "com.macminiops.panel.plist").exists() and (la / "com.macminiops.9router.plist").exists()
    assert (env.mp / "wwwroot/site1/public/index.php").exists() and (env.home / ".config/macminiops/db.env").exists()
    assert not (env.brew / "etc/nginx/nginx.conf").is_symlink() and (env.brew / "etc/nginx/nginx.conf").read_text() == "default"
    assert not (env.home / "Library/Logs/panel.log").exists() and (env.home / "Library/Logs/9router.log").exists()


def test_execute_level3_full_with_verified_backup(tmp_path):
    env = make_env(tmp_path)
    rec = Rec()
    dest = tmp_path / "out"
    with open(tmp_path / "log.txt", "w") as f:
        rc = U.execute(env, U.build_plan(env, 3, str(dest)), rec, f, lab_backup=fresh_dump(env))
    assert rc == 0, open(tmp_path / "log.txt").read()
    assert not env.mp.exists() and not (env.home / ".config/macminiops").exists() and not (env.brew / "var/mysql").exists()
    assert not (env.brew / "bin/lab").is_symlink() and not (tmp_path / "caroot").exists()
    assert (env.home / "pribadi.txt").read_text() == "jangan hapus"
    assert (env.home / "Library/LaunchAgents/com.macminiops.hermes-dashboard.plist").exists()
    bk = next(dest.glob("macpanel-final-*"))
    assert U.verify_backup(bk) >= 6
    with tarfile.open(bk / "wwwroot.tar.gz") as t:
        names = t.getnames()
    assert any(n.endswith("index.php") for n in names) and not any("node_modules" in n for n in names)
    assert (bk / "backup/dump-baru.sql.gz").exists()
    assert oct(os.stat(bk).st_mode & 0o777) == "0o700" and oct(os.stat(bk / "config.tar.gz").st_mode & 0o777) == "0o600"
    assert any(r for r, a in rec.calls), "ada langkah root yang lewat sudo"
    for r, a in rec.calls:
        if a and a[0].startswith(str(env.brew)) and "uninstall" in a:
            assert not r, "brew uninstall berjalan sebagai user biasa"


def test_backup_without_fresh_dump_aborts_and_deletes_nothing(tmp_path):
    env = make_env(tmp_path)
    with open(tmp_path / "log.txt", "w") as f:
        rc = U.execute(env, U.build_plan(env, 3, str(tmp_path / "out")), Rec(), f, lab_backup=lambda: True)
    assert rc == 1 and "GAGAL" in open(tmp_path / "log.txt").read()
    assert env.mp.exists() and (env.brew / "var/mysql/data").exists() and (env.home / ".config/macminiops/db.env").exists()


def test_backup_lab_failure_aborts(tmp_path):
    env = make_env(tmp_path)

    def boom():
        raise U.UninstallError("lab backup gagal")

    with open(tmp_path / "log.txt", "w") as f:
        assert U.execute(env, U.build_plan(env, 3, str(tmp_path / "out")), Rec(), f, lab_backup=boom) == 1
    assert env.mp.exists()


def test_not_enough_space_aborts(tmp_path, monkeypatch):
    env = make_env(tmp_path)
    monkeypatch.setattr(U.shutil, "disk_usage", lambda p: types.SimpleNamespace(free=1000, total=1, used=1))
    with open(tmp_path / "log.txt", "w") as f:
        assert U.execute(env, U.build_plan(env, 3, str(tmp_path / "out")), Rec(), f, lab_backup=fresh_dump(env)) == 1
    assert "ruang kosong" in open(tmp_path / "log.txt").read() and env.mp.exists()


def test_verify_detects_tamper_and_truncation(tmp_path):
    env = make_env(tmp_path)
    out = U.make_backup(env, tmp_path / "o", fresh_dump(env))
    U.verify_backup(out)
    f = out / "notes.tar.gz"
    data = f.read_bytes()
    f.write_bytes(data[:-10])
    with pytest.raises(U.UninstallError):
        U.verify_backup(out)
    f.write_bytes(data)
    U.verify_backup(out)
    (out / "backup/dump-baru.sql.gz").write_bytes(b"lain")
    with pytest.raises(U.UninstallError, match="verifikasi"):
        U.verify_backup(out)
    (out / "MANIFEST.json").unlink()
    with pytest.raises(U.UninstallError):
        U.verify_backup(out)


def test_failed_required_step_stops(tmp_path):
    env = make_env(tmp_path)
    plan = U.build_plan(env, 1)
    plan[2]["tolerate"] = False
    rc = U.execute(env, plan, Rec(fail_on="pf.macminiops"), open(tmp_path / "l", "w"))
    assert rc == 1


def args(**kw):
    d = dict(level=1, backup_dir="", apply=False, yes=False, confirm_host="")
    d.update(kw)
    return types.SimpleNamespace(**d)


def test_cmd_plan_only_changes_nothing(tmp_path, monkeypatch, capsys):
    env = make_env(tmp_path)
    monkeypatch.setattr(U, "detect", lambda: env)
    assert U.uninstall_cmd(args(level=3, backup_dir=str(tmp_path / "o"))) == 0
    assert "HANYA RENCANA" in capsys.readouterr().out and env.mp.exists()


def test_cmd_apply_requires_confirmation(tmp_path, monkeypatch, capsys):
    env = make_env(tmp_path)
    monkeypatch.setattr(U, "detect", lambda: env)
    monkeypatch.setattr(U, "execute", lambda *a, **k: 99)  # jangan pernah menjalankan sungguhan di tes
    monkeypatch.setattr(U.subprocess, "run", lambda *a, **k: types.SimpleNamespace(returncode=1))
    monkeypatch.setattr(U.sys.stdin, "isatty", lambda: False)
    assert U.uninstall_cmd(args(level=1, apply=True)) == 1
    assert U.uninstall_cmd(args(level=3, backup_dir=str(tmp_path / "o"), apply=True, yes=True)) == 1, "--yes tidak melewati konfirmasi tingkat 3"
    assert U.uninstall_cmd(args(level=3, backup_dir=str(tmp_path / "o"), apply=True, confirm_host="salah")) == 1
    monkeypatch.setattr(U.subprocess, "run", lambda *a, **k: types.SimpleNamespace(returncode=0))
    assert U.uninstall_cmd(args(level=3, backup_dir=str(tmp_path / "o"), apply=True, confirm_host=U.hostname())) == 99, "nama mesin benar: lanjut ke eksekusi"
    assert U.uninstall_cmd(args(level=3, backup_dir=str(tmp_path / "o"), apply=True, confirm_host="salah")) == 1
    assert env.mp.exists() and (env.home / "Library/LaunchAgents/com.macminiops.panel.plist").exists()


def test_cmd_refuses_missing_or_foreign_mp(tmp_path, monkeypatch):
    env = make_env(tmp_path)
    monkeypatch.setattr(U, "detect", lambda: env)
    os.rmdir(env.mp / "server/scripts") if not any((env.mp / "server/scripts").iterdir()) else None
    import shutil

    shutil.rmtree(env.mp / "server")
    assert U.uninstall_cmd(args()) == 1
    env.mp = tmp_path / "nonexistent"
    assert U.uninstall_cmd(args()) == 1


def test_backup_verification_failure_aborts_before_any_delete(tmp_path, monkeypatch):
    env = make_env(tmp_path)

    def bad(out):
        raise U.UninstallError("verifikasi gagal (simulasi)")

    monkeypatch.setattr(U, "verify_backup", bad)
    with open(tmp_path / "log.txt", "w") as f:
        assert U.execute(env, U.build_plan(env, 3, str(tmp_path / "out")), Rec(), f, lab_backup=fresh_dump(env)) == 1
    assert env.mp.exists() and (env.brew / "var/mysql/data").exists()


def test_exit_code_reflects_failure():
    import subprocess as sp
    import sys

    r = sp.run([sys.executable, os.path.join(os.path.dirname(__file__), "..", "lab"), "uninstall", "--level", "3"], capture_output=True, text=True)
    assert r.returncode == 1 and "wajib" in r.stderr
