from types import SimpleNamespace

import pytest
from labcli import update


def args(target, dry=False, pinned=False):
    return SimpleNamespace(target=target, dry_run=dry, allow_pinned=pinned)


@pytest.fixture
def env(tmp_path, monkeypatch):
    ops, root = tmp_path / "server", tmp_path
    app = ops / "apps/9router"
    app.mkdir(parents=True)
    (app / "package.json").write_text('{"dependencies":{"9router":"^0.5.119"}}')
    (app / "package-lock.json").write_text('{"v":1}')
    monkeypatch.setattr(update, "OPS", ops)
    monkeypatch.setattr(update, "ROOT", root)
    calls = []
    state = {"alive": True, "install_rc": 0, "outdated": {"9router": {"current": "0.5.119", "latest": "0.5.123"}}}

    def stream(cmd, **kw):
        calls.append(list(map(str, cmd)))
        if "install" in cmd and "--prefix" in cmd:
            (app / "package.json").write_text('{"dependencies":{"9router":"^NEW"}}')
            return state["install_rc"]
        return 0

    monkeypatch.setattr(update, "stream", stream)
    monkeypatch.setattr(update, "sh", lambda cmd, **kw: calls.append(list(map(str, cmd))) or SimpleNamespace(stdout="", stderr="", returncode=0))
    monkeypatch.setattr(update.uc, "read_npm_outdated", lambda npm, folder: state["outdated"])
    monkeypatch.setattr(update, "_alive", lambda port, wait=45: state["alive"])
    return SimpleNamespace(app=app, calls=calls, state=state, root=root)


def installs(env):
    return [c for c in env.calls if "--prefix" in c]


def kicks(env):
    return [c for c in env.calls if "kickstart" in c]


def test_panel_target_is_refused_not_executed(env):
    with pytest.raises(SystemExit):
        update.apply(args("panel"))
    assert env.calls == []


@pytest.mark.parametrize("name", ["lain", "../x", "x;id", "9router2", ""])
def test_npm_unknown_or_unsafe_names_refused(env, name):
    with pytest.raises(SystemExit):
        update.apply(args(f"npm:{name}"))
    assert env.calls == []


def test_npm_major_refused(env):
    env.state["outdated"] = {"9router": {"current": "0.5.119", "latest": "0.6.0"}}
    with pytest.raises(SystemExit):
        update.apply(args("npm:9router"))
    assert installs(env) == []


def test_npm_not_outdated_refused(env):
    env.state["outdated"] = {}
    with pytest.raises(SystemExit):
        update.apply(args("npm:9router"))
    assert installs(env) == []


def test_npm_dry_run_changes_nothing(env):
    update.apply(args("npm:9router", dry=True))
    assert env.calls == [] and not (env.root / "backup").exists()


def test_npm_success_backs_up_installs_without_scripts_and_restarts(env):
    update.apply(args("npm:9router"))
    bk = next((env.root / "backup").glob("update-npm-9router-*"))
    assert (bk / "package.json").read_text().count("^0.5.119") == 1 and (bk / "package-lock.json").exists()
    inst = installs(env)
    assert len(inst) == 1 and "9router@0.5.123" in inst[0] and "--ignore-scripts" in inst[0]
    assert len(kicks(env)) == 1 and "com.macminiops.9router" in kicks(env)[0][-1]


def test_npm_unhealthy_rolls_back_files_and_version(env):
    env.state["alive"] = False
    with pytest.raises(SystemExit):
        update.apply(args("npm:9router"))
    inst = installs(env)
    assert len(inst) == 2 and "9router@0.5.123" in inst[0] and "9router@0.5.119" in inst[1]
    assert "^0.5.119" in (env.app / "package.json").read_text()
    assert len(kicks(env)) == 2


def test_npm_install_failure_restores_and_never_restarts(env):
    env.state["install_rc"] = 1
    with pytest.raises(SystemExit):
        update.apply(args("npm:9router"))
    assert "^0.5.119" in (env.app / "package.json").read_text()
    assert kicks(env) == []


def brew_rows(*rows):
    base = dict(source="homebrew", pinned=False)
    return [dict(base, kind=k, name=n, **{"from": f, "to": t}, major=m, upgradable=False, target=n, note="") for k, n, f, t, m in rows], set()


@pytest.fixture
def libs(env, monkeypatch):
    items = brew_rows(("lib", "nss", "3.1", "3.2", False), ("lib", "libpq", "18", "19", True), ("tool", "gh", "2.1", "2.2", False), ("lib", "zstd", "1.5", "1.6", False))
    monkeypatch.setattr(update, "collect_brew", lambda: items)
    return env


def test_lib_all_upgrades_only_safe_libs_after_backup(libs):
    update.apply(args("lib:all"))
    cmds = libs.calls
    assert any("backup.sh" in " ".join(c) for c in cmds)
    up = next(c for c in cmds if c[1:2] == ["upgrade"])
    assert up[2:] == ["nss", "zstd"]
    assert cmds.index(next(c for c in cmds if "backup.sh" in " ".join(c))) < cmds.index(up)


def test_lib_all_dry_run_touches_nothing(libs):
    update.apply(args("lib:all", dry=True))
    assert libs.calls == []


def test_lib_all_with_nothing_safe_refuses(env, monkeypatch):
    monkeypatch.setattr(update, "collect_brew", lambda: brew_rows(("lib", "libpq", "18", "19", True)))
    with pytest.raises(SystemExit):
        update.apply(args("lib:all"))
    assert env.calls == []


def test_single_lib_and_major_formula_refused(libs):
    for t in ("nss", "libpq"):
        with pytest.raises(SystemExit):
            update.apply(args(t))
    assert not any(c[1:2] == ["upgrade"] for c in libs.calls)
