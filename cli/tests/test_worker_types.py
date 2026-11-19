"""Tipe tugas (shell/php/node/python/url) dan jadwal cron lengkap."""

import plistlib
from types import SimpleNamespace

import pytest
from labcli import cronspec, worker


@pytest.fixture
def env(tmp_path, monkeypatch):
    dev = tmp_path / "wwwroot"
    (dev / "app").mkdir(parents=True)
    monkeypatch.setattr(worker, "DEV_ROOT", dev.resolve())
    monkeypatch.setattr(worker, "LOGS", tmp_path / "logs")
    monkeypatch.setattr(worker, "SCRIPTS", tmp_path / "tasks")
    return dev.resolve()


def A(root, **kw):
    base = dict(dir=str(root / "app"), cmd=None, php=None, env=[], every=None, at=None, cron=None, procs=1, type="cmd", url=None, script_text="")
    base.update(kw)
    return SimpleNamespace(**base)


# ---- cron ----
@pytest.mark.parametrize(
    "expr,kind,n",
    [
        ("* * * * *", "interval", 60),
        ("*/5 * * * *", "calendar", 12),
        ("0 3 * * *", "calendar", 1),
        ("30 8-17/3 * * mon-fri", "calendar", 20),
        ("0 0 1 * *", "calendar", 1),
        ("0 0 * * 7", "calendar", 1),
        ("0 0 * * 0", "calendar", 1),
        ("@reboot", "boot", None),
        ("@hourly", "calendar", 1),
        ("@weekly", "calendar", 1),
        ("1,2,3 * * * *", "calendar", 3),
        ("0 9 * jan,jul *", "calendar", 2),
    ],
)
def test_cron_valid(expr, kind, n):
    k, v = cronspec.parse(expr)
    assert k == kind
    if kind == "calendar":
        assert len(v) == n
    elif kind == "interval":
        assert v == n


@pytest.mark.parametrize(
    "expr",
    ["", "* * * *", "* * * * * *", "61 * * * *", "* 24 * * *", "* * 0 * *", "* * * 13 *", "*/0 * * * *", "5-1 * * * *", "a * * * *", "0 9 15 * 1", ",1 * * * *", "x"],
)
def test_cron_rejected(expr):
    with pytest.raises(cronspec.CronError):
        cronspec.parse(expr)


def test_cron_too_dense_rejected():
    with pytest.raises(cronspec.CronError, match="terlalu rapat"):
        cronspec.parse("*/2 */2 1-31/2 * *")


def test_cron_weekday_values_and_sunday_seven():
    _, v = cronspec.parse("0 0 * * 1-5,7")
    assert sorted(d["Weekday"] for d in v) == [0, 1, 2, 3, 4, 5]


# ---- plist ----
def test_job_cron_goes_into_calendar_list_and_keeps_expression(env):
    _, p = worker.build("job", "j", A(env, cmd="/bin/echo hi", cron="*/20 9 * * 1-5"))
    assert isinstance(p["StartCalendarInterval"], list) and len(p["StartCalendarInterval"]) == 15
    assert p["LabSchedule"] == "*/20 9 * * 1-5"
    plistlib.dumps(p)  # harus bisa diserialkan


def test_job_every_minute_uses_interval_and_reboot_uses_runatload(env):
    assert worker.build("job", "j", A(env, cmd="/bin/echo", cron="* * * * *"))[1]["StartInterval"] == 60
    p = worker.build("job", "j", A(env, cmd="/bin/echo", cron="@reboot"))[1]
    assert p["RunAtLoad"] is True and "StartInterval" not in p and "StartCalendarInterval" not in p


def test_job_needs_exactly_one_schedule(env):
    for kw in (dict(), dict(every="5m", cron="* * * * *"), dict(at="03:00", every="5m")):
        with pytest.raises(SystemExit):
            worker.build("job", "j", A(env, cmd="/bin/echo", **kw))


def test_bad_cron_dies_not_crashes(env):
    with pytest.raises(SystemExit):
        worker.build("job", "j", A(env, cmd="/bin/echo", cron="0 9 15 * 1"))


# ---- tipe ----
def test_shell_type_writes_private_script_and_runs_with_zsh(env, tmp_path, monkeypatch):
    nm, p = worker.build("job", "bk", A(env, type="shell", script_text="echo halo\nls\n", every="5m"))
    assert p["ProgramArguments"] == ["/bin/zsh", str(tmp_path / "tasks/bk.sh")] and p["LabType"] == "shell"
    text = p.pop("_script_text")
    assert text.startswith("echo halo")
    # simpan lewat install dengan launchctl palsu
    monkeypatch.setattr(worker, "lc", lambda *a: SimpleNamespace(returncode=0, stdout="", stderr=""))
    monkeypatch.setattr(worker, "AGENTS", tmp_path / "agents")
    monkeypatch.setattr(worker, "plist_path", lambda kind, n: tmp_path / f"{kind}.{n}.plist")
    p["_script_text"] = text
    worker.install("job", nm, p)
    f = tmp_path / "tasks/bk.sh"
    assert f.read_text() == "echo halo\nls\n" and oct(f.stat().st_mode)[-3:] == "700"
    assert oct((tmp_path / "tasks").stat().st_mode)[-3:] == "700"
    assert "_script_text" not in plistlib.loads((tmp_path / "job.bk.plist").read_bytes())


@pytest.mark.parametrize("text", ["", "   \n", "a\0b", "x" * 20001])
def test_shell_script_limits(env, text):
    with pytest.raises(SystemExit):
        worker.build("job", "bk", A(env, type="shell", script_text=text, every="5m"))


def test_url_type_uses_curl_without_shell(env):
    _, p = worker.build("job", "ping", A(env, type="url", url="https://app.lab/schedule/run?x=1&y=2", cron="@hourly", dir=None))
    assert p["ProgramArguments"][0] == "/usr/bin/curl" and p["ProgramArguments"][-1] == "https://app.lab/schedule/run?x=1&y=2"
    assert p["WorkingDirectory"] == str(env)


@pytest.mark.parametrize("url", ["", "ftp://x", "https://a b", "https://a/'x", "javascript:1", "https://a/$(id)", "-o/etc/x"])
def test_url_rejects_unsafe(env, url):
    with pytest.raises(SystemExit):
        worker.build("job", "ping", A(env, type="url", url=url, every="5m"))


def test_runtime_types_require_matching_program(env):
    worker.build("worker", "w", A(env, type="python", cmd="python3 w.py")) if worker.shutil.which("python3") else None
    with pytest.raises(SystemExit):
        worker.build("worker", "w", A(env, type="php", cmd="/bin/echo hi"))


def test_remove_deletes_script_and_failed_install_leaves_none(env, tmp_path, monkeypatch):
    monkeypatch.setattr(worker, "AGENTS", tmp_path / "agents")
    monkeypatch.setattr(worker, "plist_path", lambda kind, n: tmp_path / f"{kind}.{n}.plist")
    calls = []
    monkeypatch.setattr(worker, "lc", lambda *a: calls.append(a) or SimpleNamespace(returncode=int(a[0] == "bootstrap" and fail["on"]), stdout="", stderr="x"))
    fail = {"on": True}
    nm, p = worker.build("job", "gagal", A(env, type="shell", script_text="ls", every="5m"))
    with pytest.raises(SystemExit):
        worker.install("job", nm, p)  # launchctl menolak
    assert not (tmp_path / "tasks/gagal.sh").exists(), "skrip tertinggal setelah pemasangan gagal"
    fail["on"] = False
    nm, p = worker.build("job", "ok", A(env, type="shell", script_text="ls", every="5m"))
    worker.install("job", nm, p)
    assert (tmp_path / "tasks/ok.sh").exists()
    monkeypatch.setattr(worker, "names", lambda kind: ["ok"])
    worker.run("job", SimpleNamespace(act="remove", name="ok"))
    assert not (tmp_path / "tasks/ok.sh").exists() and not (tmp_path / "job.ok.plist").exists()
