"""Daftar layanan dibuka tiap kali halaman Aplikasi dimuat, jadi jumlah subproses `brew` menentukan lamanya."""

from labcli import services


class _R:
    def __init__(self, out="", rc=0):
        self.stdout, self.returncode = out, rc


def _fake(calls, status_out="[]"):
    def sh(cmd, **kw):
        calls.append(list(cmd))
        if cmd[1:3] == ["list", "--versions"]:
            return _R("nginx 1.0\nmariadb@11.8 11.8.1\ndocker 29.0\n")
        if cmd[1:3] == ["services", "list"]:
            return _R(status_out)
        return _R("", 1)

    return sh


def _brew(calls, *what):
    return [c for c in calls if c[1:3] == list(what)]


def test_versions_dibaca_sekali_dan_brew_services_dilewati_bila_semua_hidup(monkeypatch):
    calls = []
    monkeypatch.setattr(services, "sh", _fake(calls))
    monkeypatch.setattr(services, "probe", lambda c: True)
    monkeypatch.setattr(services, "launchd_loaded", lambda label: True)
    rows = services.snapshot()
    assert rows and all(r["state"] in ("running", "not_installed") for r in rows)
    assert len(_brew(calls, "list", "--versions")) == 1, "brew list dipanggil lebih dari sekali (mariadb/docker memanggil ulang)"
    assert _brew(calls, "services", "list") == [], "brew services dibaca padahal semua layanan merespons"


def test_brew_services_dibaca_sekali_saja_bila_ada_layanan_mati(monkeypatch):
    calls = []
    monkeypatch.setattr(services, "sh", _fake(calls, '[{"name":"nginx","status":"error"}]'))
    monkeypatch.setattr(services, "probe", lambda c: False)
    rows = services.snapshot()
    assert len(_brew(calls, "services", "list")) == 1
    assert next(r for r in rows if r["id"] == "nginx")["state"] == "error"
