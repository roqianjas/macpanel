"""lab worker / job: pembuatan plist launchd dan validasinya (tanpa menyentuh launchd sungguhan)."""

from types import SimpleNamespace

import pytest
from labcli import worker


@pytest.fixture
def env(tmp_path, monkeypatch):
    dev = tmp_path / "wwwroot"
    (dev / "app").mkdir(parents=True)
    monkeypatch.setattr(worker, "DEV_ROOT", dev.resolve())
    monkeypatch.setattr(worker, "LOGS", tmp_path / "logs")
    return dev.resolve()


def args(root, **kw):
    base = dict(dir=str(root / "app"), cmd="/bin/echo hi", php=None, env=[], every=None, at=None, procs=1)
    base.update(kw)
    return SimpleNamespace(**base)


def test_worker_plist_keeps_alive_and_runs_in_folder(env):
    nm, p = worker.build("worker", "api", args(env))
    assert nm == "api" and p["Label"] == "com.lab.worker.api"
    assert p["KeepAlive"] is True and p["RunAtLoad"] is True
    assert p["WorkingDirectory"] == str(env / "app")
    assert p["ProgramArguments"] == ["/bin/echo", "hi"]
    assert p["EnvironmentVariables"]["HOME"]


@pytest.mark.parametrize("name", ["", "Api", "1api", "a b", "a_b", "x" * 41, "../x"])
def test_bad_names_rejected(env, name):
    with pytest.raises(SystemExit):
        worker.build("worker", name, args(env))


def test_folder_must_be_inside_wwwroot(env, tmp_path):
    outside = tmp_path / "lain"
    outside.mkdir()
    with pytest.raises(SystemExit):
        worker.build("worker", "api", args(env, dir=str(outside)))
    with pytest.raises(SystemExit):
        worker.build("worker", "api", args(env, dir=str(env / "tidak-ada")))


def test_unknown_program_rejected(env):
    with pytest.raises(SystemExit):
        worker.build("worker", "api", args(env, cmd="programtakada-xyz"))


def test_env_vars_validated_and_merged(env):
    _, p = worker.build("worker", "api", args(env, env=["PORT=3100", "HOST=127.0.0.1"]))
    assert p["EnvironmentVariables"]["PORT"] == "3100"
    for bad in ["port=1", "PATH=/x", "HOME=/x", "A=$(id)", "A=a b", "NOEQUALS", "A=;rm"]:
        with pytest.raises(SystemExit):
            worker.build("worker", "api", args(env, env=[bad]))


def test_cron_needs_exactly_a_schedule(env):
    with pytest.raises(SystemExit):
        worker.build("job", "tugas", args(env))
    _, p = worker.build("job", "tugas", args(env, every="5m"))
    assert p["StartInterval"] == 300 and "KeepAlive" not in p
    _, p = worker.build("job", "tugas", args(env, every="2h"))
    assert p["StartInterval"] == 7200
    _, p = worker.build("job", "tugas", args(env, at="03:30"))
    assert p["StartCalendarInterval"] == {"Hour": 3, "Minute": 30}
    for bad in [dict(every="0m"), dict(every="5s"), dict(every="m"), dict(at="24:00"), dict(at="3:30"), dict(at="03:60")]:
        with pytest.raises(SystemExit):
            worker.build("job", "tugas", args(env, **bad))


def test_php_path_prepended(env):
    _, p = worker.build("worker", "api", args(env, php="8.4"))
    assert p["EnvironmentVariables"]["PATH"].startswith(worker.PHP["8.4"])
