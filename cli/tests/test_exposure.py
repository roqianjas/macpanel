"""lab sys exposure: parsing lsof, klasifikasi alamat bind, dan penggabungan (murni, tanpa jaringan)."""

from labcli import exposure as E

LSOF = """p100
cnginx
f6
n*:80
f7
n*:443
p101
cnginx
f6
n*:80
f7
n*:443
p200
cmailpit
f6
n127.0.0.1:1025
f7
n127.0.0.1:8025
p300
cpostgres
n[::1]:5432
n127.0.0.1:5432
p400
cpython3.14
n100.64.0.10:9119
p500
cControlCenter
n*:5000
"""


def test_parse_lsof_dedupes_workers_and_keeps_distinct_binds():
    rows = E.parse_lsof(LSOF)
    keys = {(r["cmd"], r["bind"], r["port"]) for r in rows}
    assert ("nginx", "*", 80) in keys and ("nginx", "*", 443) in keys
    assert sum(1 for r in rows if r["cmd"] == "nginx") == 2, "dua worker nginx pada port yang sama menjadi satu baris per port"
    assert {r["pid"] for r in rows if r["cmd"] == "nginx"} == {100}, "pid terkecil dipertahankan"
    assert ("postgres", "[::1]", 5432) in keys and ("postgres", "127.0.0.1", 5432) in keys, "IPv4 dan IPv6 adalah dua pendengar berbeda"
    assert ("python3.14", "100.64.0.10", 9119) in keys


def test_parse_lsof_ignores_garbage():
    assert E.parse_lsof("") == []
    assert E.parse_lsof("nx:80\n") == [], "baris n tanpa proses dikenal diabaikan"
    assert E.parse_lsof("p1\ncx\nn*:abc\nn*:0\nn*:70000\nnnocolon\n") == [], "port tidak sah diabaikan"
    assert E.parse_lsof("pabc\ncx\nn*:80\n") == [], "pid bukan angka diabaikan"


def test_classify_bind():
    c = E.classify_bind
    assert c("127.0.0.1") == "loopback" and c("[::1]") == "loopback" and c("::1") == "loopback" and c("localhost") == "loopback"
    assert c("*") == "all" and c("0.0.0.0") == "all" and c("[::]") == "all" and c("") == "all"
    assert c("100.64.0.10") == "tailnet" and c("100.64.0.1") == "tailnet" and c(".".join(["100", str(127), "255", "254"])) == "tailnet"
    assert c("100.63.255.255") == "lan", "tepat di luar 100.64.0.0/10"
    assert c("100.128.0.1") == "lan"
    assert c("fd7a:115c:a1e0::1") == "tailnet" and c("[fd7a:115c:a1e0:ab12::5]") == "tailnet"
    assert c("192.168.0.103") == "lan" and c("10.0.0.5") == "lan" and c("8.8.8.8") == "lan"
    assert c("fe80::1%en0") == "lan" and c("[fe80::1]") == "lan"
    assert c("aneh-bukan-ip") == "lan", "tidak dikenal diperlakukan hati-hati (bukan loopback)"


def test_is_system_path():
    assert E.is_system_path("/System/Library/CoreServices/ControlCenter.app/Contents/MacOS/ControlCenter")
    assert E.is_system_path("/usr/libexec/rapportd") and E.is_system_path("/usr/sbin/sshd")
    assert not E.is_system_path("/usr/bin/python3"), "/usr/bin bisa berisi skrip pengguna: bukan sistem"
    assert not E.is_system_path("/opt/homebrew/opt/mailpit/bin/mailpit")
    assert not E.is_system_path("") and not E.is_system_path(None)
    assert not E.is_system_path("/Users/someuser/System/evil"), "harus diawali, bukan mengandung"


def test_build_scope_reachability_and_hidden():
    rows = E.parse_lsof(LSOF)
    paths = {100: "/opt/homebrew/bin/nginx", 200: "/opt/homebrew/opt/mailpit/bin/mailpit", 500: "/System/Library/CoreServices/ControlCenter.app/Contents/MacOS/ControlCenter"}
    reach = {80, 443, 5000, 22, 8025}  # 8025 terjangkau walau lsof menyebutnya loopback: tidak boleh dilaporkan sebagai LAN
    out = E.build(rows, paths, reach, "192.168.0.103")
    by = {(r["proc"], r["port"], r["bind"]): r for r in out}
    n80 = by[("nginx", 80, "*")]
    assert n80["scope"] == "all" and n80["lan_reachable"] is True and n80["system"] is False and n80["hidden"] is False
    assert by[("mailpit", 8025, "127.0.0.1")]["scope"] == "loopback" and by[("mailpit", 8025, "127.0.0.1")]["lan_reachable"] is None, "loopback tidak diuji dari LAN"
    assert by[("ControlCenter", 5000, "*")]["system"] is True
    assert by[("python3.14", 9119, "100.64.0.10")]["scope"] == "tailnet" and by[("python3.14", 9119, "100.64.0.10")]["lan_reachable"] is None
    hidden = [r for r in out if r["hidden"]]
    assert [(h["proc"], h["port"], h["service"]) for h in hidden] == [("?", 22, "ssh")], "port terjangkau yang tidak dijelaskan lsof"
    assert hidden[0]["scope"] == "all" and hidden[0]["lan_reachable"] is True and hidden[0]["pid"] is None


def test_build_without_lan_ip_reports_unknown_not_false():
    out = E.build(E.parse_lsof(LSOF), {}, set(), None)
    nginx = next(r for r in out if r["proc"] == "nginx")
    assert nginx["lan_reachable"] is None, "tidak tahu IP LAN = tidak tahu, bukan 'tidak terjangkau'"


def test_build_unreachable_wildcard_is_false():
    out = E.build(E.parse_lsof(LSOF), {}, {443}, "192.168.0.103")
    assert next(r for r in out if r["port"] == 80)["lan_reachable"] is False
    assert next(r for r in out if r["port"] == 443)["lan_reachable"] is True


def test_build_sorts_exposed_first_and_caps_size():
    out = E.build(E.parse_lsof(LSOF), {}, {80}, "1.2.3.4")
    scopes = [r["scope"] for r in out]
    assert scopes.index("loopback") > max(i for i, s in enumerate(scopes) if s != "loopback"), "yang non-loopback tampil dulu"
    many = [{"pid": i, "cmd": f"c{i}", "bind": "*", "port": 1000 + i} for i in range(1, 500)]
    assert len(E.build(many, {}, set(), "1.2.3.4")) <= E.MAX_LISTENERS


def test_scan_finds_only_open_ports_on_local_socket():
    import socket

    srv = socket.socket()
    srv.bind(("127.0.0.1", 0))
    srv.listen(1)
    port = srv.getsockname()[1]
    try:
        found = E.scan("127.0.0.1", [port, port + 1, 1], timeout=0.3)
        assert port in found and (port + 1) not in found
    finally:
        srv.close()
    assert E.probe("127.0.0.1", port) is False, "setelah ditutup tidak terjangkau"
