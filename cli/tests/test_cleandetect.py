import os

import pytest
from labcli import clean, cleandetect

APPS = {"com.google.chrome": "Google Chrome", "com.acme.tool": "Acme Tool"}


def cl(name, comms=()):
    return cleandetect.classify(name, APPS, set(comms))[0]


def test_classify_rules():
    assert cl("com.apple.Music") == "terkunci"
    assert cl("COM.APPLE.helpd") == "terkunci"
    assert cl("com.google.Chrome") == "tinjau"  # terpasang
    assert cl("com.google.Chrome.helper") == "tinjau"  # turunan dari aplikasi terpasang
    assert cl("org.removed.app") == "aman"  # yatim
    assert cl("Dia") == "tinjau"  # nama polos: yatim tidak bisa dibuktikan
    assert cl("go-build") == "tinjau"
    assert cl("com.google.Chrome", comms=["google chrome"]) == "terkunci"  # sedang berjalan
    assert cl("org.removed.app", comms=["org.removed.app"]) == "terkunci"
    for bad in ("a b", "x;rm", "-x", "", "a/b"):
        assert cl(bad) == "terkunci"


def test_candidates_only_direct_real_dirs_not_covered(tmp_path):
    h = tmp_path
    c = h / "Library/Caches"
    c.mkdir(parents=True)
    (c / "real").mkdir()
    (c / ".hid").mkdir()
    (c / "file.txt").write_text("x")
    (c / "link").symlink_to(h)
    (c / "covered").mkdir()
    (c / "real/nested").mkdir()
    (h / ".cache").mkdir()
    (h / ".cache/tool").mkdir()
    got = {i for i, *_ in cleandetect.candidates(h, {str(c / "covered")})}
    assert got == {"lc:real", "xc:tool"}


@pytest.mark.parametrize("bad", ["lc:../x", "lc:..", "lc:a/b", "zz:real", "lc:", "real", "lc:real; rm", "lc:link", "lc:nope", "lc:.hid", "xc:real", ""])
def test_parse_id_rejects(tmp_path, bad):
    c = tmp_path / "Library/Caches"
    c.mkdir(parents=True)
    (c / "real").mkdir()
    (c / "link").symlink_to(tmp_path)
    assert cleandetect.parse_id(bad, tmp_path) is None


def test_parse_id_accepts_real(tmp_path):
    c = tmp_path / "Library/Caches"
    (c / "real").mkdir(parents=True)
    assert cleandetect.parse_id("lc:real", tmp_path) == c / "real"


def test_scan_rows_sorts_drops_empty_and_locks_unreadable(tmp_path):
    c = tmp_path / "Library/Caches"
    for n in ("a", "b", "empty", "locked"):
        (c / n).mkdir(parents=True)
    sizes = {"a": (10, True), "b": (500, True), "empty": (0, True), "locked": (900, False)}
    rows = cleandetect.scan_rows(tmp_path, set(), lambda p: sizes[p.name], apps={}, comms=set())
    assert [r["label"] for r in rows] == ["locked", "b", "a"]
    assert {r["label"]: r["cls"] for r in rows}["locked"] == "terkunci"
    assert all(r["tier"] == "detected" and r["loc"] == "~/Library/Caches" for r in rows)


@pytest.fixture
def home(tmp_path, monkeypatch):
    monkeypatch.setattr(clean, "HOME", tmp_path)
    monkeypatch.setattr(cleandetect, "installed_apps", lambda h, dirs=None: dict(APPS))
    monkeypatch.setattr(cleandetect, "running_comms", lambda: set())
    c = tmp_path / "Library/Caches"
    for n, files in (("org.removed.app", 3), ("com.acme.tool", 2), ("com.apple.helpd", 1)):
        d = c / n
        d.mkdir(parents=True)
        for i in range(files):
            (d / f"f{i}").write_text("x" * 100)
    return tmp_path


def run(ids, apply, review=False):
    return clean.run(ids, apply, False, cat=[], confirm_review=review)


def test_run_dry_run_deletes_nothing(home):
    r = run(["lc:org.removed.app", "lc:com.acme.tool"], False)
    assert {i["id"] for i in r["items"]} == {"lc:org.removed.app", "lc:com.acme.tool"}
    assert len(list((home / "Library/Caches/org.removed.app").iterdir())) == 3


def test_run_aman_deleted_contents_only_folder_kept(home):
    r = run(["lc:org.removed.app"], True)
    d = home / "Library/Caches/org.removed.app"
    assert d.is_dir() and list(d.iterdir()) == []
    assert r["items"][0]["bytes"] >= 0 and not r["skipped"]


def test_run_tinjau_needs_review_flag(home):
    r = run(["lc:com.acme.tool"], True)
    assert r["skipped"] and "confirm-review" in r["skipped"][0]["reason"]
    assert len(list((home / "Library/Caches/com.acme.tool").iterdir())) == 2
    r = run(["lc:com.acme.tool"], True, review=True)
    assert not r["skipped"] and list((home / "Library/Caches/com.acme.tool").iterdir()) == []


def test_run_terkunci_never_deleted_even_with_flags(home):
    r = run(["lc:com.apple.helpd"], True, review=True)
    assert r["skipped"] and "terkunci" in r["skipped"][0]["reason"]
    assert len(list((home / "Library/Caches/com.apple.helpd").iterdir())) == 1


def test_run_running_app_locked(home, monkeypatch):
    monkeypatch.setattr(cleandetect, "running_comms", lambda: {"acme tool"})
    r = run(["lc:com.acme.tool"], True, review=True)
    assert r["skipped"] and "berjalan" in r["skipped"][0]["reason"]


def test_run_symlink_inside_not_followed(home):
    outside = home / "outside"
    outside.mkdir()
    (outside / "keep.txt").write_text("penting")
    d = home / "Library/Caches/org.removed.app"
    (d / "evil").symlink_to(outside)
    run(["lc:org.removed.app"], True)
    assert (outside / "keep.txt").read_text() == "penting"
    assert not os.path.lexists(d / "evil")


@pytest.mark.parametrize("bad", ["lc:../outside", "lc:nope", "xc:org.removed.app", "lc:a/b"])
def test_run_bad_ids_skipped_not_executed(home, bad):
    r = run([bad], True, review=True)
    assert r["skipped"] and not r["items"]
