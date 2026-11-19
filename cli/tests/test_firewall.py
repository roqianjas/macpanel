"""lab firewall: aturan, loader (dijalankan sungguhan dengan pfctl palsu), urutan langkah sudo, tanpa menyentuh sistem."""

import io
import os
import re
import stat
import subprocess

import pytest
from labcli import firewall as F


def test_parse_roundtrip_and_strictness():
    ports = {88: "tailnet", 5000: "none"}
    assert F.parse_ports(F.render_ports(ports)) == ports
    for bad in ("88", "88 tailnet extra", "abc tailnet", "88 lan", "0 none", "70000 none", "88; rm -rf / none"):
        with pytest.raises(F.FirewallError):
            F.parse_ports(bad + "\n")
    assert F.parse_ports("# komentar\n\n  7 none  \n") == {7: "none"}


def test_check_port_args():
    for bad in (0, 65536, -1, True, "88"):
        with pytest.raises(F.FirewallError):
            F.check_port_args(bad, "tailnet")
    with pytest.raises(F.FirewallError, match="dilindungi"):
        F.check_port_args(22, "tailnet")
    with pytest.raises(F.FirewallError):
        F.check_port_args(88, "lan")
    F.check_port_args(88, "none")


def test_render_anchor_rules():
    t = F.render_anchor({88: "tailnet"}, "utun3")
    assert t.splitlines()[0] == "pass in quick on lo0 all"
    assert "pass in quick on utun3 proto { tcp udp } from 100.64.0.0/10 to any port 88" in t
    assert t.index("pass in quick on utun3") < t.index("block drop in quick proto tcp")
    assert "pass in quick on utun" not in F.render_anchor({88: "none"}, "utun3")
    assert "pass in quick on utun" not in F.render_anchor({88: "tailnet"}, None), "tanpa Tailscale: tertutup semua"


def _loader_env(tmp_path, ports_text, ts_present):
    ports = tmp_path / "ports"
    ports.write_text(ports_text)
    anchor, conf, log = tmp_path / "anchor", tmp_path / "conf", tmp_path / "pf.log"
    conf.write_text("x\n")
    pfctl = tmp_path / "pfctl"
    n = tmp_path / "n"
    n.write_text("0")
    pfctl.write_text(f'#!/bin/sh\nk=$(cat {n}); k=$((k+1)); echo $k > {n}\ncp {anchor} {tmp_path}/snap.$k 2>/dev/null\necho "$@" >> {log}\n')
    ifc = tmp_path / "ifconfig"
    ifc.write_text('#!/bin/sh\nif [ "$1" = "-l" ]; then echo "lo0 en1 utun9"; else echo "\\tinet %s --> 1"; fi\n' % ("100.64.0.5" if ts_present else "169.254.0.1"))
    for f in (pfctl, ifc):
        f.chmod(f.stat().st_mode | stat.S_IXUSR)
    src = F.render_loader(str(ports), str(anchor), str(conf), str(pfctl), str(ifc), wait=0)
    ld = tmp_path / "loader"
    ld.write_text(src)
    ld.chmod(0o755)
    return ld, anchor, tmp_path


@pytest.mark.parametrize("ts", [True, False])
def test_loader_output_equals_python_render_and_blocks_first(tmp_path, ts):
    ports = {88: "tailnet", 5000: "none"}
    ld, anchor, d = _loader_env(tmp_path, F.render_ports(ports), ts)
    r = subprocess.run([str(ld)], capture_output=True, text=True, timeout=30)
    assert r.returncode == 0, r.stderr
    assert anchor.read_text() == F.render_anchor(ports, "utun9" if ts else None)
    first = (d / "snap.1").read_text()
    assert first == F.render_anchor(ports, None), "pemuatan pertama harus menutup dulu, sebelum menunggu Tailscale"
    assert ("TIDAK TERDETEKSI" in r.stdout) != ts


def test_loader_ignores_garbage_lines(tmp_path):
    ld, anchor, _ = _loader_env(tmp_path, "# c\n88 tailnet\n99999 none\nabc none\n88; touch /tmp/pwn none\n$(id) none\n\n", True)
    subprocess.run([str(ld)], capture_output=True, text=True, timeout=30)
    t = anchor.read_text()
    assert "port 88" in t and "99999" not in t and "abc" not in t and "pwn" not in t and "id" not in t.replace("quick", "").replace("pass", "")
    assert not os.path.exists("/tmp/pwn")


def test_loader_without_ports_file_still_valid(tmp_path):
    ld, anchor, _ = _loader_env(tmp_path, "", True)
    os.unlink(tmp_path / "ports")
    r = subprocess.run([str(ld)], capture_output=True, text=True, timeout=30)
    assert r.returncode == 0 and anchor.read_text() == "pass in quick on lo0 all\n"


def test_render_loader_rejects_unsafe_paths():
    with pytest.raises(F.FirewallError):
        F.render_loader(ports_file="/tmp/a b")
    with pytest.raises(F.FirewallError):
        F.render_loader(conf="/tmp/$(x)")


def test_build_conf_idempotent_and_preserves():
    base = "scrub-anchor x\n"
    c = F.build_conf(None, base)
    assert c.startswith(base) and f'anchor "{F.ANCHOR}"' in c and F.ANCHOR_FILE in c
    assert F.build_conf(c, base) == c
    ex = 'A\nanchor "macminiops-rustdesk"\nload anchor "macminiops-rustdesk" from "/etc/pf.anchors/macminiops-rustdesk"\n'
    c2 = F.build_conf(ex, base)
    assert c2.startswith(ex) and c2.count("macminiops-rustdesk") == 3, "anchor RustDesk tidak boleh hilang"
    assert F.build_conf("tanpa newline", base).startswith("tanpa newline\nanchor")


@pytest.mark.skipif(not os.path.exists("/sbin/pfctl"), reason="pfctl hanya ada di macOS")
def test_pf_syntax_validation_real_pfctl():
    assert F.pf_syntax_ok(F.render_anchor({88: "tailnet", 8080: "none"}, "utun0"))[0]
    ok, msg = F.pf_syntax_ok("block drop in quick proto tcp from any to any port 99999\n")
    assert not ok and "99999" in msg


def test_install_plan_order_and_safety(tmp_path):
    out = io.StringIO()
    run = F.Runner(dry=True, out=out)
    F.install_plan({88: "tailnet"}, run, base_conf_text="base\n")
    lines = [ln for ln in out.getvalue().splitlines() if ln.startswith("  - ")]
    cmds = [ln.split(": ", 1)[1] for ln in lines]
    assert all(c.startswith("sudo /") for c in cmds), "setiap langkah sudo memakai path absolut, tanpa shell"
    idx = lambda key: next(i for i, c in enumerate(cmds) if key in c)  # noqa: E731
    assert idx("pfctl -n -f") < idx("-m 600") < idx("launchctl bootstrap"), "uji sintaks sebelum mengganti konfigurasi, daemon terakhir"
    assert all("sh -c" not in c and "bash" not in c for c in cmds)
    assert all(("-o root -g wheel" in c) for c in cmds if "/usr/bin/install" in c), "semua berkas dipasang milik root"
    run.cleanup()


def test_install_plan_backs_up_existing_conf_before_replacing(monkeypatch):
    seen = []

    class R(F.Runner):
        def sudo(self, desc, argv, tolerate=False, capture=False):
            seen.append(argv)
            if argv[:1] == ["/bin/cat"]:
                return subprocess.CompletedProcess(argv, 0, 'anchor "macminiops-rustdesk"\n', "")
            return subprocess.CompletedProcess(argv, 0, "", "")

    monkeypatch.setattr(F.os.path, "exists", lambda p: True)
    r = R(dry=True, out=io.StringIO())
    F.install_plan({88: "none"}, r, base_conf_text="b\n")
    names = [a[0] for a in seen]
    assert names.index("/bin/cp") < next(i for i, a in enumerate(seen) if a[:2] == ["/usr/bin/install", "-o"] and a[-1] == F.CONF)
    cp = next(a for a in seen if a[0] == "/bin/cp")
    assert re.search(r"\.bak\.\d{8}-\d{6}$", cp[-1])
    conf_tmp = next(a for a in seen if a[:2] == ["/usr/bin/install", "-o"] and a[-1] == F.CONF)[-2]
    text = open(conf_tmp).read()
    assert "macminiops-rustdesk" in text and F.ANCHOR in text
    r.cleanup()


def test_install_plan_aborts_on_invalid_rules(monkeypatch):
    monkeypatch.setattr(F, "pf_syntax_ok", lambda t: (False, "boom"))
    with pytest.raises(F.FirewallError, match="boom"):
        F.install_plan({88: "none"}, F.Runner(dry=True, out=io.StringIO()))


def test_verify_detects_missing_rules():
    class R(F.Runner):
        def __init__(self, info, rules):
            super().__init__(dry=False, out=io.StringIO())
            self.map = {"info": info, "rules": rules}

        def sudo(self, desc, argv, tolerate=False, capture=False):
            key = "info" if "info" in argv else "rules"
            return subprocess.CompletedProcess(argv, 0, self.map[key], "")

    ok_rules = "block drop in quick proto tcp from any to any port = 88\n"
    assert F.verify({88: "none"}, R("Status: Enabled", ok_rules), rustdesk=False) == []
    assert F.verify({88: "none"}, R("Status: Disabled", ok_rules), rustdesk=False) == ["pf mati"]
    assert F.verify({88: "none"}, R("Status: Enabled", ""), rustdesk=False) == ["port 88 tidak terblokir"]
