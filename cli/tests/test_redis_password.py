"""Kata sandi Redis: konfigurasi idempoten, env tidak lewat argumen proses, aktif/nonaktif/rollback pada Redis sandbox (port 6391)."""

import os
import socket
import subprocess
import time

import pytest
from labcli import redis as R

SANDBOX_PORT = 6391


# ---------------- unit murni ----------------


def test_patch_conf_adds_block_once_and_removes_cleanly():
    base = "port 6379\nbind 127.0.0.1\n# requirepass foobared\n"
    on = R.patch_conf(base, "abc123")
    assert on.count(R.MARK_BEGIN) == 1 and "requirepass abc123\n" in on
    assert on.startswith(base), "isi lama tidak boleh berubah"
    again = R.patch_conf(on, "xyz789")
    assert again.count(R.MARK_BEGIN) == 1 and "xyz789" in again and "abc123" not in again, "memasang ulang mengganti, bukan menumpuk"
    assert R.patch_conf(on, None) == base, "membuang blok mengembalikan teks persis seperti semula"


def test_patch_conf_without_trailing_newline():
    assert R.patch_conf("port 1", "p").startswith("port 1\n")
    assert R.patch_conf("port 1", None) == "port 1\n"


def test_patch_conf_refuses_manual_requirepass():
    with pytest.raises(ValueError, match="manual"):
        R.patch_conf("port 1\nrequirepass rahasia\n", "x")
    # baris yang dikomentari bukan pengaturan aktif
    R.patch_conf("port 1\n#requirepass rahasia\n", "x")
    # manual + blok kita: tetap ditolak (jangan menyembunyikan pengaturan pemilik)
    with pytest.raises(ValueError):
        R.patch_conf(R.conf_block("a") + "requirepass manual\n", "b")


def test_conf_managed():
    assert R.conf_managed(R.conf_block("a"))
    assert not R.conf_managed("port 1\n")
    assert not R.conf_managed(R.MARK_BEGIN + "\nrequirepass a\n")  # blok tidak lengkap


def test_read_password_variants(tmp_path):
    p = tmp_path / "r.env"
    assert R.read_password(p) is None
    p.write_text("REDIS_PASSWORD=abc-DEF_123\n")
    assert R.read_password(p) == "abc-DEF_123"
    p.write_text("# komentar\nLAIN=x\n  REDIS_PASSWORD =  spasi  \n")
    assert R.read_password(p) == "spasi"
    p.write_text("REDIS_PASSWORD=\n")
    assert R.read_password(p) is None


def test_password_goes_via_env_never_argv(monkeypatch, tmp_path):
    p = tmp_path / "r.env"
    p.write_text("REDIS_PASSWORD=SUPERSECRET123\n")
    monkeypatch.setattr(R, "REDIS_ENV", p)
    seen = {}

    def fake_run(cmd, **kw):
        seen["cmd"], seen["env"] = cmd, kw.get("env", {})
        return subprocess.CompletedProcess(cmd, 0, stdout="1\n", stderr="")

    monkeypatch.setattr(R.subprocess, "run", fake_run)
    R.rcli_raw(0, "DBSIZE")
    assert "SUPERSECRET123" not in " ".join(seen["cmd"]), "kata sandi tidak boleh ada di argumen (terlihat di ps)"
    assert seen["env"].get("REDISCLI_AUTH") == "SUPERSECRET123"
    R.rcli(0, "DBSIZE")
    assert "SUPERSECRET123" not in " ".join(seen["cmd"]) and seen["env"].get("REDISCLI_AUTH") == "SUPERSECRET123"


def test_no_auth_env_when_no_password_file(monkeypatch, tmp_path):
    monkeypatch.setattr(R, "REDIS_ENV", tmp_path / "tidak-ada.env")
    monkeypatch.setenv("REDISCLI_AUTH", "warisan-dari-shell")
    assert "REDISCLI_AUTH" not in R._env(), "nilai warisan dari shell tidak boleh bocor ke redis-cli bila kita tidak mengelola sandi"


def test_noauth_error_has_actionable_message(monkeypatch, tmp_path, capsys):
    monkeypatch.setattr(R, "REDIS_ENV", tmp_path / "tidak-ada.env")
    monkeypatch.setattr(R.subprocess, "run", lambda cmd, **kw: subprocess.CompletedProcess(cmd, 1, stdout="", stderr="NOAUTH Authentication required."))
    with pytest.raises(SystemExit):
        R.rcli_raw(0, "DBSIZE")
    assert "lab redis password status" in capsys.readouterr().err


def test_write_env_is_0600_and_atomic(tmp_path):
    p = tmp_path / "sub" / "r.env"
    R._write_env(p, "pw1")
    assert oct(p.stat().st_mode & 0o777) == "0o600" and R.read_password(p) == "pw1"
    R._write_env(p, "pw2")
    assert R.read_password(p) == "pw2" and not list(p.parent.glob("*.tmp"))


# ---------------- sandbox: Redis sungguhan di port 6391 ----------------

live = pytest.mark.live
REDIS_SERVER = "/opt/homebrew/bin/redis-server"


class Sandbox:
    def __init__(self, tmp):
        self.conf = tmp / "redis.conf"
        self.env = tmp / "redis.env"
        self.conf.write_text(f'port {SANDBOX_PORT}\nbind 127.0.0.1\nsave ""\nappendonly no\ndir {tmp}\n')
        self.proc = None

    def start(self, conf=None):
        self.stop()
        self.proc = subprocess.Popen([REDIS_SERVER, str(conf or self.conf)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    def stop(self):
        if self.proc:
            self.proc.terminate()
            try:
                self.proc.wait(5)
            except subprocess.TimeoutExpired:
                self.proc.kill()
            self.proc = None
        for _ in range(20):
            with socket.socket() as s:
                s.settimeout(0.2)
                if s.connect_ex(("127.0.0.1", SANDBOX_PORT)) != 0:
                    return
            time.sleep(0.1)


@pytest.fixture
def sb(tmp_path, monkeypatch):
    if not os.path.exists(REDIS_SERVER):
        pytest.skip("redis-server tidak terpasang")
    s = Sandbox(tmp_path)
    monkeypatch.setattr(R, "PORT", SANDBOX_PORT)
    monkeypatch.setattr(R, "REDIS_ENV", s.env)
    s.start()
    assert R.wait_for("ok", None, 10), "sandbox Redis tidak naik"
    yield s
    s.stop()


@live
def test_enable_requires_password_then_disable_restores(sb):
    restart = sb.start
    assert R.ping(None) == "ok"
    msg = R.password_enable(sb.conf, sb.env, restart)
    assert msg.startswith("aktif"), msg
    pw = R.read_password(sb.env)
    assert pw and len(pw) >= 24
    assert oct(sb.env.stat().st_mode & 0o777) == "0o600"
    assert R.ping(None) == "noauth", "tanpa kata sandi harus ditolak"
    assert R.ping("salah-besar") == "wrong"
    assert R.ping(pw) == "ok"
    assert R.rcli_raw(0, "DBSIZE")[0] == "0", "lab redis harus bisa bekerja otomatis lewat berkas env"
    baks = list(sb.conf.parent.glob("redis.conf.bak.*"))
    assert len(baks) == 1 and oct(baks[0].stat().st_mode & 0o777) == "0o600"
    assert "requirepass" not in baks[0].read_text(), "cadangan adalah konfigurasi sebelum diubah"
    st = R.password_status(sb.conf, sb.env)
    assert st["managed"] and st["with_password"] == "ok" and st["without_password"] == "noauth" and st["env_mode"] == "0o600"
    assert pw not in str(st), "status tidak boleh menampilkan kata sandi"
    assert R.password_enable(sb.conf, sb.env, restart) == "sudah aktif", "idempoten: tidak mengganti sandi yang sudah berfungsi"
    assert R.read_password(sb.env) == pw

    msg = R.password_disable(sb.conf, sb.env, restart)
    assert msg.startswith("nonaktif"), msg
    assert R.ping(None) == "ok" and not sb.env.exists()
    assert "requirepass" not in sb.conf.read_text()


@live
def test_failed_enable_rolls_back_conf_and_env(sb):
    original = sb.conf.read_text()

    def bad_restart():  # "restart" yang tetap memakai konfigurasi LAMA: kata sandi baru tidak akan berlaku
        sb.start(None)  # sb.conf sudah ditulis ulang, jadi paksa konfigurasi lama lewat berkas lain
        old = sb.conf.parent / "old.conf"
        old.write_text(original)
        sb.start(old)

    with pytest.raises(SystemExit):
        R.password_enable(sb.conf, sb.env, bad_restart)
    assert sb.conf.read_text() == original, "konfigurasi harus kembali persis"
    assert not sb.env.exists(), "berkas env baru harus dibuang"


@live
def test_enable_keeps_previous_env_on_rollback(sb):
    R._write_env(sb.env, "sandi-lama-123")
    original = sb.conf.read_text()

    def bad_restart():
        old = sb.conf.parent / "old.conf"
        old.write_text(original)
        sb.start(old)

    with pytest.raises(SystemExit):
        R.password_enable(sb.conf, sb.env, bad_restart)
    assert R.read_password(sb.env) == "sandi-lama-123"


@live
def test_enable_refuses_manual_requirepass_without_touching_anything(sb):
    sb.conf.write_text(sb.conf.read_text() + "requirepass punya-pemilik\n")
    before = sb.conf.read_text()
    with pytest.raises(ValueError):
        R.password_enable(sb.conf, sb.env, sb.start)
    assert sb.conf.read_text() == before and not sb.env.exists()
    assert not list(sb.conf.parent.glob("redis.conf.bak.*")), "penolakan terjadi sebelum cadangan dibuat"


@live
def test_disable_when_never_enabled_is_harmless(sb):
    before = sb.conf.read_text()
    assert R.password_disable(sb.conf, sb.env, sb.start).startswith("tidak aktif")
    assert sb.conf.read_text() == before
