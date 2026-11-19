"""Tailscale: rencana pasang, tingkat 4 uninstall, dan pengaman sesi SSH lewat tailnet. Semua pada env palsu; sudo tidak pernah dijalankan."""

import types

from labcli import tailscale as T
from labcli import uninstall as U
from test_uninstall import Rec, make_env


def tsenv(tmp_path, formula=True, daemon=True):
    return T.TsEnv(formula, daemon, home=tmp_path, brew=tmp_path / "brew")


def test_install_plan_complete_when_missing(tmp_path):
    p = T.plan_install(tsenv(tmp_path, False, False))
    assert [s["op"] for s in p] == ["exec", "sudo"]
    assert p[0]["argv"][-2:] == ["install", "tailscale"] and p[1]["argv"][-2:] == ["start", "tailscale"]
    assert all(s["argv"][0].startswith("/") for s in p)  # path absolut, tanpa shell


def test_install_plan_partial_and_empty(tmp_path):
    assert [s["op"] for s in T.plan_install(tsenv(tmp_path, True, False))] == ["sudo"]
    assert T.plan_install(tsenv(tmp_path, True, True)) == []


def test_install_never_runs_tailscale_up(tmp_path):
    for p in (T.plan_install(tsenv(tmp_path, False, False)), T.plan_install(tsenv(tmp_path, True, False))):
        assert not any("up" in s["argv"] or "login" in s["argv"] for s in p)


def test_level4_only_tailscale(tmp_path):
    env = make_env(tmp_path)
    env.formulas.add("tailscale")
    plan = U.build_plan(env, 4)
    assert [s["op"] for s in plan] == ["sudo", "exec"] and {s["level"] for s in plan} == {4}
    blob = " ".join(" ".join(s["argv"]) for s in plan)
    assert "tailscale" in blob and "nginx" not in blob and "colima" not in blob
    assert not any(s["op"] == "rm" for s in plan)  # /etc/resolver, data node, ~/MacPanel tidak disentuh


def test_level4_empty_when_not_installed(tmp_path):
    assert U.build_plan(make_env(tmp_path), 4) == []


def test_levels_1_to_3_never_include_tailscale(tmp_path):
    env = make_env(tmp_path)
    env.formulas.add("tailscale")
    for lv in (1, 2, 3):
        plan = U.build_plan(env, lv, backup_dir=str(tmp_path / "bk") if lv == 3 else None)
        assert not any("tailscale" in " ".join(s.get("argv", [])) for s in plan), lv


def test_level4_execute_with_recorder(tmp_path):
    env = make_env(tmp_path)
    env.formulas.add("tailscale")
    r = Rec()
    assert U.execute(env, U.build_plan(env, 4), runner=r) == 0
    assert [c[0] for c in r.calls] == [True, False]
    assert r.calls[1][1][-1] == "tailscale"


def test_remote_over_tailnet():
    assert U.remote_over_tailnet({"SSH_CONNECTION": "100.64.0.10 5000 100.64.0.11 22"})
    assert U.remote_over_tailnet({"SSH_CONNECTION": "fd7a:115c:a1e0::1 5000 fd7a:115c:a1e0::2 22"})
    assert U.remote_over_tailnet({"SSH_CONNECTION": "garbage 1 2 3"})  # tak terbaca = berisiko
    assert not U.remote_over_tailnet({"SSH_CONNECTION": "192.168.1.5 5000 192.168.1.2 22"})
    assert not U.remote_over_tailnet({})


def args(**kw):
    d = dict(level=4, backup_dir="", apply=True, yes=False, with_docker_tools=False, confirm_host="", allow_remote_cut=False)
    d.update(kw)
    return types.SimpleNamespace(**d)


def test_cmd_level4_refuses_over_tailnet_ssh(tmp_path, monkeypatch, capsys):
    env = make_env(tmp_path)
    env.formulas.add("tailscale")
    monkeypatch.setattr(U, "detect", lambda: env)
    monkeypatch.setattr(U, "execute", lambda *a, **k: (_ for _ in ()).throw(AssertionError("tidak boleh dieksekusi")))
    monkeypatch.setenv("SSH_CONNECTION", "100.64.0.10 1 100.64.0.11 22")
    assert U.uninstall_cmd(args()) == 1
    assert "memutus sesi" in capsys.readouterr().err


def test_cmd_level4_needs_hostname_and_plan_only_changes_nothing(tmp_path, monkeypatch, capsys):
    env = make_env(tmp_path)
    env.formulas.add("tailscale")
    monkeypatch.setattr(U, "detect", lambda: env)
    monkeypatch.setattr(U, "execute", lambda *a, **k: (_ for _ in ()).throw(AssertionError("tidak boleh dieksekusi")))
    monkeypatch.delenv("SSH_CONNECTION", raising=False)
    monkeypatch.setattr(U.os, "geteuid", lambda: 501)
    assert U.uninstall_cmd(args(apply=False)) == 0
    monkeypatch.setattr("sys.stdin", types.SimpleNamespace(isatty=lambda: False))
    assert U.uninstall_cmd(args(confirm_host="salah")) == 1  # nama mesin salah
    assert "dibatalkan" in capsys.readouterr().err


def test_cmd_level4_not_installed(tmp_path, monkeypatch, capsys):
    env = make_env(tmp_path)
    monkeypatch.setattr(U, "detect", lambda: env)
    monkeypatch.setattr(U.os, "geteuid", lambda: 501)
    monkeypatch.delenv("SSH_CONNECTION", raising=False)
    assert U.uninstall_cmd(args(apply=False)) == 1
    assert "tidak terpasang" in capsys.readouterr().err
