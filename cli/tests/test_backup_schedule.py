"""Jadwal backup otomatis: bisa dimatikan, interval 1-30 hari; backup.sh menghormatinya (HOME palsu, tidak menyentuh data nyata)."""

import os
import plistlib
import subprocess
import time
import types

import pytest
from labcli import backup as B

SCRIPT = os.path.join(os.path.dirname(__file__), "..", "backup.sh")


def run_script(tmp_path, conf, force=False, last_age_h=None):
    home = tmp_path / "h"
    (home / ".config/macminiops").mkdir(parents=True)
    mp = tmp_path / "mp"
    (mp / "backup/staging").mkdir(parents=True)
    if conf is not None:
        (home / ".config/macminiops/backup.conf").write_text(conf)
    if last_age_h is not None:
        d = mp / "backup/staging/20260101-000000"
        d.mkdir()
        t = time.time() - last_age_h * 3600
        os.utime(d, (t, t))
    env = dict(os.environ, HOME=str(home), MACPANEL_HOME=str(mp))
    env.pop("BACKUP_FORCE", None)
    if force:
        env["BACKUP_FORCE"] = "1"
    r = subprocess.run(["/bin/bash", SCRIPT], env=env, capture_output=True, text=True, timeout=60)
    return r


def test_script_skips_when_disabled(tmp_path):
    r = run_script(tmp_path, "KEEP=7\nENABLED=0\nEVERY_DAYS=1\n")
    assert r.returncode == 0 and "dimatikan" in r.stdout and not list((tmp_path / "mp/backup/staging").glob("*/postgres*"))


def test_script_skips_when_recent_for_interval(tmp_path):
    r = run_script(tmp_path, "ENABLED=1\nEVERY_DAYS=3\n", last_age_h=24)
    assert r.returncode == 0 and "kurang dari 3 hari" in r.stdout


def test_script_runs_when_interval_elapsed_or_daily(tmp_path):
    # tanpa db.env di HOME palsu, langkah dump gagal: itu tandanya skrip TIDAK melewati (bukan "dilewati")
    r = run_script(tmp_path, "ENABLED=1\nEVERY_DAYS=3\n", last_age_h=80)
    assert "dilewati" not in r.stdout and r.returncode != 0
    r = run_script(tmp_path / "x", "ENABLED=1\nEVERY_DAYS=1\n", last_age_h=1) if (tmp_path / "x").mkdir() is None else None
    assert "dilewati" not in r.stdout


def test_force_overrides_disabled_and_interval(tmp_path):
    r = run_script(tmp_path, "ENABLED=0\nEVERY_DAYS=30\n", force=True, last_age_h=1)
    assert "dilewati" not in r.stdout and "dimatikan" not in r.stdout


def test_no_conf_means_daily_enabled(tmp_path):
    r = run_script(tmp_path, None)
    assert "dilewati" not in r.stdout and "dimatikan" not in r.stdout


@pytest.fixture
def cfg(tmp_path, monkeypatch):
    plist = tmp_path / "bk.plist"
    plist.write_bytes(plistlib.dumps({"Label": "x", "StartCalendarInterval": {"Hour": 2, "Minute": 30}}))
    monkeypatch.setattr(B, "BK_PLIST", plist)
    monkeypatch.setattr(B, "BK_CONF", tmp_path / "backup.conf")
    calls = []
    monkeypatch.setattr(B, "sh", lambda cmd, **k: calls.append(cmd) or types.SimpleNamespace(returncode=0, stdout="", stderr=""))
    return calls


def args(**kw):
    d = dict(cact="set", time="03:15", keep=5, days=0, extra_days=0, enabled="yes", every_days=1)
    d.update(kw)
    return types.SimpleNamespace(**d)


def test_set_disabled_disables_agent_and_persists(cfg, capsys):
    B.bk_config(args(enabled="no", every_days=2))
    out = capsys.readouterr().out
    assert "DIMATIKAN" in out
    assert ["/bin/launchctl", "disable", f"gui/{os.getuid()}/{B.BK_LABEL}"] in cfg
    assert not any(c[1] == "bootstrap" for c in cfg), "dimatikan: tidak boleh di-bootstrap lagi"
    assert B.bk_read_sched() == (False, 2)


def test_set_enabled_enables_and_bootstraps(cfg, capsys):
    B.bk_config(args(enabled="no"))
    cfg.clear()
    B.bk_config(args(enabled="yes", every_days=3))
    out = capsys.readouterr().out
    assert "tiap 3 hari" in out
    assert [c[1] for c in cfg][:3] == ["bootout", "enable", "bootstrap"]
    assert B.bk_read_sched() == (True, 3)


def test_get_reports_schedule_and_defaults(cfg, capsys):
    B.bk_config(types.SimpleNamespace(cact="get"))
    d = __import__("json").loads(capsys.readouterr().out)
    assert d["enabled"] is True and d["every_days"] == 1 and d["time"] == "02:30"


@pytest.mark.parametrize("kw", [dict(every_days=0), dict(every_days=31), dict(enabled="maybe"), dict(time="25:00"), dict(keep=0)])
def test_set_rejects_bad_values(cfg, kw):
    with pytest.raises(SystemExit):
        B.bk_config(args(**kw))
    assert not B.BK_CONF.exists()


def test_corrupt_conf_falls_back_to_defaults(cfg):
    B.BK_CONF.write_text("ENABLED=9\nEVERY_DAYS=999\n")
    assert B.bk_read_sched() == (True, 1)


def test_manual_run_forces_backup(monkeypatch):
    seen = {}
    monkeypatch.setattr(B, "stream", lambda cmd, **k: seen.update(k) or 0)
    with pytest.raises(SystemExit):
        B.backup(types.SimpleNamespace(act="run"))
    assert seen["env"]["BACKUP_FORCE"] == "1", "backup manual tidak boleh ikut dilewati oleh jadwal"
