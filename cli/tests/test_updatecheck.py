import json

import pytest
from labcli import updatecheck as uc

PINS = {"postgresql@17", "mysql@8.4"}
SVC = {"nginx", "redis", "netdata"}
LEAVES = {"nginx", "redis", "gh", "composer", "php", "php@8.3", "cloudflared"}


def brew(name, cur, new):
    return {"name": name, "installed_versions": [cur], "current_version": new}


def kinds(rows):
    return {i["name"]: i["kind"] for i in uc.brew_items({"formulae": rows}, PINS, SVC, LEAVES)}


def test_json_from_skips_leading_brew_noise():
    assert uc.json_from('==> Downloading Homebrew API data\n✔︎ JSON API x\n{"a": 1}') == {"a": 1}
    assert uc.json_from("") == {} and uc.json_from("tidak ada json") == {} and uc.json_from("{rusak") == {}


def test_classification_runtime_service_tool_lib():
    k = kinds(
        [
            brew("php", "8.5.1", "8.5.2"),
            brew("php@8.3", "8.3.1", "8.3.2"),
            brew("nginx", "1.1", "1.2"),
            brew("postgresql@17", "17.1", "17.2"),
            brew("cloudflared", "1", "2"),
            brew("gh", "2.1", "2.2"),
            brew("nss", "3.1", "3.2"),
            brew("netdata", "1.0", "1.1"),
        ]
    )
    assert k == {"php": "runtime", "php@8.3": "runtime", "nginx": "service", "postgresql@17": "service", "cloudflared": "service", "gh": "tool", "nss": "lib", "netdata": "service"}


def test_runtime_series_change_is_major_but_patch_is_not():
    rows = {i["name"]: i for i in uc.brew_items({"formulae": [brew("php", "8.5.12_1", "8.6.0"), brew("php@8.3", "8.3.36_1", "8.3.37")]}, PINS, SVC, LEAVES)}
    assert rows["php"]["major"] and not rows["php"]["upgradable"] and rows["php"]["note"] == "major"
    assert not rows["php@8.3"]["major"] and rows["php@8.3"]["upgradable"]


def test_libs_never_individually_upgradable_and_major_flagged():
    rows = {i["name"]: i for i in uc.brew_items({"formulae": [brew("nss", "3.133", "3.134"), brew("libpq", "18.7", "19.0")]}, PINS, SVC, LEAVES)}
    assert rows["nss"]["note"] == "bulk" and not rows["nss"]["upgradable"] and not rows["nss"]["major"]
    assert rows["libpq"]["major"] and rows["libpq"]["note"] == "major"


def test_pinned_marked_and_still_upgradable_with_flag():
    r = uc.brew_items({"formulae": [brew("postgresql@17", "17.1", "17.2")]}, PINS, SVC, LEAVES)[0]
    assert r["pinned"] and r["note"] == "pinned" and r["upgradable"]


@pytest.mark.parametrize(
    "a,b,major", [("1.2.3", "1.9.0", False), ("1.2.3", "2.0.0", True), ("0.5.119", "0.5.123", False), ("0.5.119", "0.6.0", True), ("17.1", "18.0", True), ("2026.11.0", "2026.11.2", False)]
)
def test_is_major(a, b, major):
    assert uc.is_major(a, b) is major


def test_mise_items_only_known_tools():
    rows = uc.mise_items({"pnpm": {"current": "12.21.0", "latest": "12.23.0"}, "ruby": {"current": "1", "latest": "2"}}, {"pnpm", "node"})
    assert [(r["name"], r["kind"], r["target"]) for r in rows] == [("pnpm", "runtime", "mise:pnpm")]


def test_npm_items_allowlist_major_and_missing():
    out = {"9router": {"current": "0.5.119", "latest": "0.5.123"}, "other": {"current": "1.0.0", "latest": "1.0.1"}, "nocur": {"latest": "2"}, "same": {"current": "1", "latest": "1"}}
    rows = {r["name"]: r for r in uc.npm_items("x", out, {"9router"})}
    assert set(rows) == {"9router", "other"}
    assert rows["9router"]["upgradable"] and rows["9router"]["note"] == "restart" and rows["9router"]["target"] == "npm:9router"
    assert not rows["other"]["upgradable"] and rows["other"]["note"] == "manual"
    major = uc.npm_items("x", {"9router": {"current": "0.5.1", "latest": "0.6.0"}}, {"9router"})[0]
    assert major["major"] and not major["upgradable"]


def test_panel_item_only_when_behind():
    assert uc.panel_item("abc", "abc", 0) == []
    assert uc.panel_item("abc", "def", 0) == []
    r = uc.panel_item("abc", "def", 3)[0]
    assert r["kind"] == "panel" and r["behind"] == 3 and not r["upgradable"] and r["note"] == "deploy"


def test_summarize_counts_every_kind():
    c = uc.summarize([{"kind": "lib"}, {"kind": "lib"}, {"kind": "app"}])
    assert c["lib"] == 2 and c["app"] == 1 and c["runtime"] == 0 and set(uc.KINDS) <= set(c)


def test_npm_name_regex_rejects_injection():
    for bad in ["../x", "a b", "x;id", "$(id)", "", "-rf", "A", "a" * 61]:
        assert not uc.NPM_NAME.match(bad)
    assert uc.NPM_NAME.match("9router")


def test_read_npm_apps_ignores_symlinks_and_non_node(tmp_path):
    (tmp_path / "a").mkdir()
    (tmp_path / "a/package.json").write_text("{}")
    (tmp_path / "b").mkdir()
    (tmp_path / "ln").symlink_to(tmp_path / "a")
    (tmp_path / "f.txt").write_text("x")
    assert [p.name for p in uc.read_npm_apps(tmp_path)] == ["a"]
    assert uc.read_npm_apps(tmp_path / "tidak-ada") == []


def test_json_output_roundtrip_is_serializable():
    rows = uc.brew_items({"formulae": [brew("nss", "3.133", "3.134")]}, PINS, SVC, LEAVES)
    assert json.loads(json.dumps(rows))[0]["from"] == "3.133"
