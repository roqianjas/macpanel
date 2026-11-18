"""lab sys net: parser Tailscale, ukuran internet, DNS, dan snapshot gabungan."""

import json

from labcli import netcheck as nc

TS = {
    "BackendState": "Running",
    "Self": {"Online": True},
    "Peer": {"a": {"Online": True}, "b": {"Online": False}, "c": {"Online": True}, "d": {}},
    "Health": [],
}


def test_parse_tailscale_counts_online_peers():
    r = nc.parse_tailscale(json.dumps(TS))
    assert r == {"state": "Running", "self_online": True, "peers": 4, "peers_online": 2, "health": []}


def test_parse_tailscale_states_and_health():
    d = dict(TS, BackendState="NeedsLogin", Self={"Online": False}, Health=["a" * 300, "b", "c", "d"])
    r = nc.parse_tailscale(json.dumps(d))
    assert r["state"] == "NeedsLogin" and r["self_online"] is False
    assert len(r["health"]) == 3 and len(r["health"][0]) == 120  # dipotong: keluaran luar tidak dipercaya


def test_parse_tailscale_missing_pieces_do_not_crash():
    r = nc.parse_tailscale(json.dumps({"BackendState": "Stopped"}))
    assert r == {"state": "Stopped", "self_online": False, "peers": 0, "peers_online": 0, "health": []}
    r = nc.parse_tailscale(json.dumps({"BackendState": "Running", "Peer": [1, 2], "Self": "x", "Health": "x"}))
    assert r["peers"] == 0 and r["self_online"] is False and r["health"] == []


def test_parse_tailscale_online_must_be_true_not_truthy():
    r = nc.parse_tailscale(json.dumps({"BackendState": "Running", "Peer": {"a": {"Online": "yes"}, "b": {"Online": 1}}}))
    assert r["peers_online"] == 0


def test_parse_tailscale_garbage_gives_none():
    for bad in ["", "bukan json", "[]", "{}", json.dumps({"BackendState": 5}), "null"]:
        assert nc.parse_tailscale(bad) is None


class FakeSock:
    closed = False

    def close(self):
        self.closed = True


def clock_of(*vals):
    it = iter(vals)
    return lambda: next(it)


def test_tcp_ms_measures_and_closes():
    s = FakeSock()
    ms = nc.tcp_ms("h", 443, connect=lambda a, timeout: s, clock=clock_of(10.0, 10.0056))
    assert ms == 5.6 and s.closed


def test_tcp_ms_failure_is_none_not_zero():
    def boom(a, timeout):
        raise OSError("timeout")

    assert nc.tcp_ms("h", 443, connect=boom) is None


def test_internet_ok_when_one_target_answers_and_uses_fastest():
    ms = {"a": None, "b": 12.5, "c": 7.0}
    r = nc.internet(targets=(("a", 1), ("b", 1), ("c", 1)), probe=lambda h, p: ms[h])
    assert r["ok"] is True and r["ms"] == 7.0 and len(r["tried"]) == 3


def test_internet_down_only_when_all_fail():
    r = nc.internet(targets=(("a", 1), ("b", 1)), probe=lambda h, p: None)
    assert r == {"ok": False, "ms": None, "tried": [{"host": "a", "ms": None}, {"host": "b", "ms": None}]}


def test_resolve_ok_fail_and_empty():
    assert nc.resolve("x", getaddr=lambda n, p: [("addr",)], clock=clock_of(0.0, 0.0062)) == {"name": "x", "ok": True, "ms": 6.2}
    assert nc.resolve("x", getaddr=lambda n, p: []) == {"name": "x", "ok": False, "ms": None}

    def boom(n, p):
        raise OSError("nxdomain")

    assert nc.resolve("x", getaddr=boom) == {"name": "x", "ok": False, "ms": None}


def test_snapshot_assembles_and_keeps_errors():
    s = nc.net_snapshot(
        "panel.lab",
        ts=lambda: (None, "tailscale tidak ditemukan"),
        inet=lambda: {"ok": True, "ms": 5.0, "tried": []},
        dns=lambda n: {"name": n, "ok": True, "ms": 1.0},
        now=lambda: 1700000000,
    )
    assert s["at"] == 1700000000 and s["tailscale"] is None
    assert s["errors"] == ["tailscale tidak ditemukan"]
    assert s["dns_local"]["name"] == "panel.lab" and s["dns_public"]["name"] == nc.PUBLIC_NAME


def test_snapshot_without_local_name_skips_local_dns():
    s = nc.net_snapshot("", ts=lambda: (None, None), inet=lambda: {"ok": True, "ms": 1, "tried": []}, dns=lambda n: {"name": n, "ok": True, "ms": 1})
    assert s["dns_local"] is None and s["errors"] == []


def test_subcommand_registered():
    from labcli.cli import build_parser

    a = build_parser().parse_args(["sys", "net", "--dns-name", "panel.lab"])
    assert a.act == "net" and a.dns_name == "panel.lab"
    assert build_parser().parse_args(["sys", "net"]).dns_name == ""
