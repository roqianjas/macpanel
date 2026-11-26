import os
import time

import pytest
from labcli import clean


def mk(tmp_path):
    h = tmp_path / "home"
    r = h / "MacPanel"
    for d in (
        "Library/Caches/go-build/a",
        "Library/Caches/Homebrew",
        ".npm/_cacache",
        "Library/Logs",
        "MacPanel/backup/staging",
        "MacPanel/backup/migrasi-x-bot",
        "MacPanel/backup/unduh-mac",
        "MacPanel/backup/rutin-penting",
        "Documents",
        ".hermes",
        "MacPanel/wwwroot",
    ):
        (h / d).mkdir(parents=True, exist_ok=True)
    (h / "Library/Caches/go-build/a/f.bin").write_bytes(b"x" * 5000)
    (h / "Library/Caches/Homebrew/b").write_bytes(b"y" * 3000)
    (h / "Documents/penting.txt").write_text("jangan hapus")
    (h / ".hermes/state.db").write_text("jangan hapus")
    (h / "MacPanel/backup/migrasi-x-bot/k.db").write_text("z")
    (h / "MacPanel/backup/unduh-mac/z.dmg").write_text("z")
    (h / "MacPanel/backup/rutin-penting/x").write_text("rutin")
    (h / "MacPanel/backup/staging/s").write_text("s")
    return h, r


@pytest.fixture
def env(tmp_path, monkeypatch):
    h, r = mk(tmp_path)
    monkeypatch.setattr(clean, "running", lambda names: [])
    return h, r, clean.catalog(h, r)


def test_catalog_tiers(env):
    h, r, cat = env
    by = {e["id"]: e for e in cat}
    assert by["go-build"]["tier"] == "safe" and by["bk:migrasi-x-bot"]["tier"] == "careful" and by["bk:unduh-mac"]["tier"] == "careful"
    assert "bk:rutin-penting" not in by and "bk:staging" not in by  # folder yang tidak cocok pola tidak pernah masuk
    for i in ("hermes", "documents", "wwwroot", "backup-routine", "database-data"):
        assert by[i]["tier"] == "show"


def test_dry_run_deletes_nothing(env):
    h, r, cat = env
    res = clean.run(["go-build", "homebrew-cache"], False, False, cat)
    assert res["dry_run"] and res["bytes"] >= 8000
    assert (h / "Library/Caches/go-build/a/f.bin").exists()


def test_apply_safe_removes_contents_keeps_dir(env):
    h, r, cat = env
    res = clean.run(["go-build"], True, False, cat)
    assert res["items"][0]["bytes"] >= 5000 and not res["skipped"]
    assert (h / "Library/Caches/go-build").is_dir() and not list((h / "Library/Caches/go-build").iterdir())
    assert (h / "Library/Caches/Homebrew/b").exists()  # entri lain tidak tersentuh


def test_careful_needs_confirm(env):
    h, r, cat = env
    res = clean.run(["bk:migrasi-x-bot"], True, False, cat)
    assert res["skipped"][0]["reason"].startswith("tingkat Hati-hati") and (h / "MacPanel/backup/migrasi-x-bot/k.db").exists()
    res = clean.run(["bk:migrasi-x-bot"], True, True, cat)
    assert not (h / "MacPanel/backup/migrasi-x-bot").exists() and (h / "MacPanel/backup/unduh-mac/z.dmg").exists()


@pytest.mark.parametrize("bad", ["hermes", "documents", "wwwroot", "backup-routine", "database-data", "colima-data", "../../etc", "/etc", "bk:rutin-penting", "bk:../staging", "", "nonexistent"])
def test_never_deletes_outside_catalog(env, bad):
    h, r, cat = env
    if bad == "":
        with pytest.raises(clean.CleanError):
            clean.run([], True, True, cat)
        return
    res = clean.run([bad], True, True, cat)
    assert res["items"] == [] and res["skipped"], bad
    assert (h / "Documents/penting.txt").exists() and (h / ".hermes/state.db").exists() and (h / "MacPanel/backup/rutin-penting/x").exists()


def test_busy_process_skips(tmp_path, monkeypatch):
    h, r = mk(tmp_path)
    monkeypatch.setattr(clean, "running", lambda names: ["go"] if "go" in names else [])
    res = clean.run(["go-build"], True, False, clean.catalog(h, r))
    assert "go" in res["skipped"][0]["reason"] and (h / "Library/Caches/go-build/a/f.bin").exists()


def test_symlink_not_followed(env, tmp_path):
    h, r, cat = env
    outside = tmp_path / "luar"
    outside.mkdir()
    (outside / "rahasia").write_text("jangan")
    (h / "Library/Caches/go-build/tautan").symlink_to(outside)
    clean.run(["go-build"], True, False, cat)
    assert (outside / "rahasia").exists() and not (h / "Library/Caches/go-build/tautan").exists()


def test_entry_dir_itself_symlink_ignored(tmp_path, monkeypatch):
    h, r = mk(tmp_path)
    monkeypatch.setattr(clean, "running", lambda names: [])
    outside = tmp_path / "luar2"
    outside.mkdir()
    (outside / "rahasia").write_text("jangan")
    import shutil as sh

    sh.rmtree(h / "Library/Caches/Homebrew")
    (h / "Library/Caches/Homebrew").symlink_to(outside)
    res = clean.run(["homebrew-cache"], True, False, clean.catalog(h, r))
    assert (outside / "rahasia").exists()
    assert res["bytes"] == 0


def test_old_logs_only_rotated(tmp_path):
    d = tmp_path
    old = time.time() - 40 * 86400
    for n in ("a.log.1", "b.gz", "aktif.log", "c.log.2024-01-01"):
        (d / n).write_text("x")
        os.utime(d / n, (old, old))
    (d / "baru.gz").write_text("x")
    names = {f.name for f in clean.old_logs(d)}
    assert names == {"a.log.1", "b.gz", "c.log.2024-01-01"}


def test_scan_reports_unreadable_and_free(env, monkeypatch):
    h, r, cat = env
    real = clean.du_bytes
    monkeypatch.setattr(clean, "du_bytes", lambda p, timeout=180: (0, False) if "Homebrew" in str(p) else real(p, timeout))
    d = clean.scan(cat)
    row = next(x for x in d["rows"] if x["id"] == "homebrew-cache")
    assert row["complete"] is False and d["disk_free"] > 0 and d["safe_bytes"] >= 5000
    carefuls = [x for x in d["rows"] if x["tier"] == "careful"]
    assert all("items" in x and "age_days" in x for x in carefuls)


def test_scan_saved_private(tmp_path):
    p = tmp_path / "s.json"
    clean.save_scan({"a": 1}, p)
    assert oct(p.stat().st_mode & 0o777) == "0o600" and clean.load_scan(p) == {"a": 1}
    assert clean.load_scan(tmp_path / "tidak-ada") is None


def test_label_and_group():
    rows = [(2048000, 1, "/opt/homebrew/opt/php@8.3/sbin/php-fpm --nodaemonize"), (500000, 2, "php-fpm: pool dev"), (100, 3, "/usr/bin/foo bar")]
    g = clean.group_procs(rows)
    assert g[0]["label"] == "PHP-FPM" and g[0]["procs"] == 2 and g[0]["mb"] == round((2048000 + 500000) / 1024)
    assert clean.label_of("/usr/bin/foo bar") == "foo"


def mk_nginx(tmp_path, sites, snippets=None):
    n = tmp_path / "nginx"
    (n / "sites").mkdir(parents=True)
    (n / "snippets").mkdir()
    for k, v in sites.items():
        (n / "sites" / k).write_text(v)
    for k, v in (snippets or {}).items():
        (n / "snippets" / k).write_text(v)
    return n


def test_php_use_maps_pool_sockets_to_versions(tmp_path):
    etc = tmp_path / "etc"
    (etc / "8.5/php-fpm.d").mkdir(parents=True)
    (etc / "8.5/php-fpm.d/adminer.conf").write_text("[adminer]\nlisten = /opt/homebrew/var/run/php-fpm-adminer.sock\n")
    (etc / "8.3/php-fpm.d").mkdir(parents=True)
    n = mk_nginx(tmp_path, {"a.conf": "fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-8.4.sock;"}, {"adminer.conf": "fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-adminer.sock;"})
    assert clean.used_php_versions(n, etc) == {"8.4", "8.5"}


def test_php_use_unknown_socket_blocks_php_suggestions(tmp_path):
    etc = tmp_path / "etc"
    etc.mkdir()
    n = mk_nginx(tmp_path, {"a.conf": "fastcgi_pass unix:/tmp/aneh.sock;"})
    used = clean.used_php_versions(n, etc)
    assert any(u.startswith("?") for u in used)
    svcs = [dict(id="php-8.3", label="PHP 8.3", installed=True, state="running")]
    assert clean.suggestions(svcs, [], used) == []


def test_suggestions_never_stop_php_in_use(tmp_path):
    n = mk_nginx(tmp_path, {"a.conf": "fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-8.4.sock;"})
    used = clean.used_php_versions(n, tmp_path / "kosong")
    svcs = [
        dict(id="php-8.3", label="PHP 8.3", installed=True, state="running"),
        dict(id="php-8.4", label="PHP 8.4", installed=True, state="running"),
        dict(id="docker", label="Docker", installed=True, state="running", optional=True),
        dict(id="netdata", label="Netdata", installed=True, state="stopped", optional=True),
    ]
    out = clean.suggestions(svcs, [dict(label="Docker (Colima VM)", mb=4000)], used)
    ids = [s["id"] for s in out]
    assert "php-8.3" in ids and "php-8.4" not in ids and "netdata" not in ids and out[0]["id"] == "docker" and out[0]["mb"] == 4000


def test_cli_exit_and_dry_default(env, monkeypatch, capsys):
    h, r, cat = env
    monkeypatch.setattr(clean, "catalog", lambda *a, **k: cat)
    import types

    a = types.SimpleNamespace(cact="run", ids=["go-build"], apply=False, confirm_careful=False, confirm_review=False)
    assert clean.clean_cmd(a) is None
    assert (h / "Library/Caches/go-build/a/f.bin").exists()
    a = types.SimpleNamespace(cact="run", ids=[], apply=True, confirm_careful=False, confirm_review=False)
    assert clean.clean_cmd(a) == 1


def test_clean_entry_guards_standalone(env, tmp_path):
    h, r, cat = env
    by = {e["id"]: e for e in cat}
    with pytest.raises(clean.CleanError):
        clean.clean_entry(by["documents"], True)
    outside = tmp_path / "luar3"
    outside.mkdir()
    (outside / "rahasia").write_text("jangan")
    link = h / "MacPanel/backup/migrasi-link"
    link.symlink_to(outside)
    e = clean.E("bk:migrasi-link", "x", "careful", [link], "", "", [], kind="tree")
    with pytest.raises(clean.CleanError):
        clean.clean_entry(e, True)
    assert (outside / "rahasia").exists()


def test_mem_top_reads_service_list_as_json(monkeypatch, tmp_path):
    """Regresi: `lab service list` tanpa --json bukan JSON, sehingga saran penghematan diam-diam kosong."""
    import json as _json
    import subprocess as sp

    calls = []

    def fake_run(cmd, *a, **k):
        calls.append(list(cmd))
        if "service" in cmd:
            ok = "--json" in cmd
            out = _json.dumps([dict(id="docker", label="Docker", installed=True, state="running", optional=True)]) if ok else "tabel teks bukan json"
            return sp.CompletedProcess(cmd, 0, out, "")
        if cmd[0].endswith("ps"):
            return sp.CompletedProcess(cmd, 0, "  204800 11 /usr/bin/limactl hostagent\n", "")
        return sp.CompletedProcess(cmd, 0, "", "")

    monkeypatch.setattr(clean.subprocess, "run", fake_run)
    monkeypatch.setattr(clean, "ROOT", tmp_path)
    res = clean.mem_top()
    assert any(c[-3:] == ["service", "list", "--json"] for c in calls)
    assert [x["id"] for x in res["suggestions"]] == ["docker"]
    assert res["groups"][0]["label"] == "Docker (Colima VM)"


def test_extra_caches_and_hermes_backups(env):
    h, r, cat = env
    (h / ".hermes/backups").mkdir(parents=True, exist_ok=True)
    (h / ".hermes/backups/config.bak").write_text("x")
    (h / ".hermes/cache/pnpm").mkdir(parents=True)
    (h / ".hermes/cache/pnpm/a").write_text("x" * 100)
    (h / "Library/Caches/Google").mkdir(parents=True)
    cat = clean.catalog(h, r)
    by = {e["id"]: e for e in cat}
    assert by["chrome-cache"]["tier"] == "safe" and "Google Chrome" in by["chrome-cache"]["busy"]
    assert by["hermes-pnpm"]["tier"] == "safe" and by["cx:hermes-backups"]["tier"] == "careful"
    assert clean.run(["cx:hermes-backups"], True, False, cat)["skipped"], "careful butuh konfirmasi"
    assert (h / ".hermes/backups/config.bak").exists()
    clean.run(["hermes-pnpm"], True, False, cat)
    assert not (h / ".hermes/cache/pnpm/a").exists() and (h / ".hermes/cache").is_dir()
    assert (h / ".hermes/state.db").exists()
