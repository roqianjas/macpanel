"""Tes live perintah pengubah sistem: site, php, update, app, worker. Menyentuh mesin nyata dengan sandbox.

Jalankan: MACPANEL_LIVE=1 python3 -m pytest cli/tests/test_live_system.py -m live
Sandbox:
- situs `zz-live-<acak>` (folder statis di wwwroot), dihapus di akhir termasuk sertifikatnya;
- `php install 8.5` hanya pada versi yang tidak dipakai situs mana pun (idempoten: tidak memasang ulang);
- `update apply --dry-run` (tidak mengubah apa pun), dan penolakan tanpa --dry-run untuk kasus yang harus ditolak;
- aplikasi opsional `memcached` (kecil): pasang lalu hapus, hanya bila belum terpasang sebelumnya;
- worker `zz-live-<acak>` dengan perintah tidak berbahaya.
Tidak menyentuh situs, database, atau layanan milik pengguna.
"""

import hashlib
import os
import secrets
import ssl
import subprocess
import time
from pathlib import Path

import pytest
from labcli.core import ROOT

pytestmark = pytest.mark.live
LAB = str(ROOT / "server/scripts/lab")
SITES = ROOT / "server/nginx/sites"
CERTS = Path("/opt/homebrew/etc/nginx/certs")
BREW = "/opt/homebrew/bin"
ENV = dict(os.environ, PATH=f"{BREW}:/usr/bin:/bin:/usr/sbin:/sbin", HOMEBREW_NO_AUTO_UPDATE="1")


def lab(*a, inp=None, timeout=300):
    return subprocess.run([LAB, *a], input=inp, capture_output=True, text=True, timeout=timeout, env=ENV)


def tld():
    return (ROOT / "server/tld").read_text().strip()


def nginx_ok():
    return subprocess.run([f"{BREW}/nginx", "-t", "-c", str(ROOT / "server/nginx/nginx.conf")], capture_output=True, text=True, env=ENV).returncode == 0


def http(host, path="/", timeout=8):
    """GET https://<host>/ ke 127.0.0.1 dengan SNI dan Host = host (tanpa bergantung DNS). Mengembalikan (kode, isi) atau (None, galat)."""
    import socket

    ctx = ssl.create_default_context()
    ctx.check_hostname, ctx.verify_mode = False, ssl.CERT_NONE
    try:
        with socket.create_connection(("127.0.0.1", 443), timeout=timeout) as raw, ctx.wrap_socket(raw, server_hostname=host) as s:
            s.sendall(f"GET {path} HTTP/1.1\r\nHost: {host}\r\nConnection: close\r\n\r\n".encode())
            buf = b""
            while chunk := s.recv(65536):
                buf += chunk
        head, _, body = buf.partition(b"\r\n\r\n")
        return int(head.split(b" ", 2)[1]), body.decode(errors="replace")
    except Exception as e:  # noqa: BLE001
        return None, str(e)


@pytest.fixture
def site():
    name = "zz-live-" + secrets.token_hex(3)
    folder = ROOT / "wwwroot" / name
    folder.mkdir(parents=True)
    (folder / "index.html").write_text(f"<h1>{name}-ok</h1>")
    yield name, folder
    lab("site", "remove", name)
    subprocess.run(["rm", "-rf", str(folder)])
    for f in (SITES / f"{name}.conf", SITES / f"{name}.conf.disabled", CERTS / f"{name}.{tld()}.pem", CERTS / f"{name}.{tld()}-key.pem"):
        f.unlink(missing_ok=True)


# ---------------- site ----------------


def test_site_full_lifecycle(site):
    name, folder = site
    host = f"{name}.{tld()}"
    r = lab("site", "add", name, str(folder))
    assert r.returncode == 0, r.stdout + r.stderr
    assert (SITES / f"{name}.conf").exists()
    assert (CERTS / f"{host}.pem").exists() and (CERTS / f"{host}-key.pem").exists(), "sertifikat harus dibuat"
    assert nginx_ok(), "nginx -t harus lulus setelah add"
    assert host in lab("site", "list").stdout
    time.sleep(1)
    code, body = http(host)
    assert code == 200 and f"{name}-ok" in body, (code, body[:100])

    r = lab("site", "disable", name)
    assert r.returncode == 0, r.stdout + r.stderr
    assert nginx_ok()
    time.sleep(1)
    code, body = http(host)
    assert code != 200 or f"{name}-ok" not in body, "situs nonaktif tidak boleh menyajikan isinya"

    r = lab("site", "enable", name)
    assert r.returncode == 0, r.stdout + r.stderr
    time.sleep(1)
    code, body = http(host)
    assert code == 200 and f"{name}-ok" in body, "setelah enable harus kembali melayani"

    r = lab("site", "remove", name)
    assert r.returncode == 0, r.stdout + r.stderr
    assert not (SITES / f"{name}.conf").exists() and not (SITES / f"{name}.conf.disabled").exists()
    assert not (CERTS / f"{host}.pem").exists() and not (CERTS / f"{host}-key.pem").exists(), "sertifikat harus ikut terhapus"
    assert nginx_ok(), "nginx -t harus lulus setelah remove"
    assert host not in lab("site", "list").stdout


def test_site_rejects_bad_names_and_changes_nothing():
    before = sorted(p.name for p in SITES.iterdir())
    for bad in ["../x", "a b", "UP", "-x", "x;rm", "a/b", "", "x" * 41, "x" * 64, "a.b"]:  # 41 melewati batas panel (1-40); 64 melewati batas label DNS
        r = lab("site", "add", bad, str(ROOT / "wwwroot"))
        assert r.returncode != 0, f"nama {bad!r} harus ditolak"
    assert sorted(p.name for p in SITES.iterdir()) == before, "penolakan tidak boleh meninggalkan berkas"
    assert nginx_ok()


def test_site_add_with_missing_folder_fails_cleanly():
    name = "zz-live-" + secrets.token_hex(3)
    r = lab("site", "add", name, str(ROOT / "wwwroot" / "tidak-ada-" / name))
    assert r.returncode != 0
    assert not (SITES / f"{name}.conf").exists() and not (CERTS / f"{name}.{tld()}.pem").exists()
    assert nginx_ok()


def test_site_remove_of_unknown_name_is_harmless():
    before = sorted(p.name for p in SITES.iterdir())
    lab("site", "remove", "zz-tidak-pernah-ada")
    assert sorted(p.name for p in SITES.iterdir()) == before and nginx_ok()


def test_site_add_duplicate_does_not_break_nginx(site):
    name, folder = site
    assert lab("site", "add", name, str(folder)).returncode == 0
    time.sleep(1)
    dup = lab("site", "add", name, str(folder))
    assert dup.returncode != 0 and "sudah ada" in dup.stdout + dup.stderr, "add ganda harus ditolak, bukan menimpa diam-diam"
    assert nginx_ok()
    time.sleep(1)
    code, body = http(f"{name}.{tld()}")
    assert code == 200 and f"{name}-ok" in body


# ---------------- php ----------------


def test_php_list_and_validation():
    r = lab("php", "list")
    assert r.returncode == 0 and "8.4" in r.stdout and "terpasang=" in r.stdout
    for v in ["9.9", "8.1", "8", "abc", "8.4; id", "../8.4", ""]:
        assert lab("php", "install", v).returncode != 0, f"versi {v!r} harus ditolak"
        assert lab("php", "remove", v).returncode != 0


def test_php_remove_refused_while_in_use_and_nothing_changes():
    used = [v for v in ("8.3", "8.4", "8.5") if any(f"php-fpm-dev-{v}.sock" in p.read_text() for p in SITES.glob("*.conf"))]
    if not used:
        pytest.skip("tidak ada situs PHP yang aktif untuk dipakai sebagai bukti")
    v = used[0]
    r = lab("php", "remove", v)
    assert r.returncode != 0 and "masih dipakai" in (r.stdout + r.stderr), r.stdout + r.stderr
    assert f"{v}  terpasang=ya" in lab("php", "list").stdout, "versi yang dipakai tidak boleh terhapus"
    assert Path(f"/opt/homebrew/var/run/php-fpm-dev-{v}.sock").exists(), "FPM yang dipakai tidak boleh berhenti"


def test_php_install_on_installed_version_is_idempotent():
    v = "8.5"
    if f"{v}  terpasang=ya" not in lab("php", "list").stdout:
        pytest.skip("PHP 8.5 tidak terpasang")
    if any(f"php-fpm-dev-{v}.sock" in p.read_text() for p in SITES.glob("*.conf")):
        pytest.skip("PHP 8.5 dipakai situs: restart FPM akan mengganggu")
    conf = Path(f"/opt/homebrew/etc/php/{v}/php-fpm.d/dev.conf")
    before = hashlib.sha256(conf.read_bytes()).hexdigest()
    ver_before = subprocess.run(["/opt/homebrew/opt/php/bin/php", "-r", "echo PHP_VERSION;"], capture_output=True, text=True).stdout
    r = lab("php", "install", v, timeout=400)
    assert r.returncode == 0, r.stdout + r.stderr
    assert hashlib.sha256(conf.read_bytes()).hexdigest() == before, "pool dev harus identik setelah install ulang"
    assert not (conf.parent / "www.conf").exists(), "www.conf bawaan (TCP 9000) tidak boleh kembali"
    assert Path(f"/opt/homebrew/var/run/php-fpm-dev-{v}.sock").exists()
    assert subprocess.run(["/opt/homebrew/opt/php/bin/php", "-r", "echo PHP_VERSION;"], capture_output=True, text=True).stdout == ver_before, "versi tidak boleh berubah"


# ---------------- update ----------------


def test_update_check_and_dry_run_changes_nothing():
    r = lab("update", "check")
    assert r.returncode == 0 and "Homebrew:" in r.stdout
    outdated = [ln.split()[0] for ln in r.stdout.splitlines() if ln.startswith("  ") and "->" in ln and "PIN" not in ln]
    cand = next((p for p in outdated if p in ("gh", "dav1d", "libomp", "libksba")), None)
    if not cand:
        pytest.skip("tidak ada paket usang kecil yang aman untuk dry-run")
    before = subprocess.run([f"{BREW}/brew", "list", "--versions", cand], capture_output=True, text=True, env=ENV).stdout
    r = lab("update", "apply", cand, "--dry-run")
    assert r.returncode == 0, r.stdout + r.stderr
    assert subprocess.run([f"{BREW}/brew", "list", "--versions", cand], capture_output=True, text=True, env=ENV).stdout == before, "dry-run tidak boleh mengubah versi"


def test_update_apply_refuses_invalid_and_unsafe_targets():
    for bad in ["../evil", "a b", "x;id", "$(id)", "", "tidak-ada-paket-ini"]:
        assert lab("update", "apply", bad).returncode != 0, f"target {bad!r} harus ditolak"
    for pinned in ("mysql@8.4", "postgresql@17"):
        r = lab("update", "apply", pinned)
        assert r.returncode != 0, f"{pinned} (di-pin, tidak usang) harus ditolak tanpa --allow-pinned"


# ---------------- app ----------------


def test_app_rejects_unknown_and_core_services():
    for bad in ["tidak-ada", "nginx", "mariadb", "../x", "mysql;id"]:
        assert lab("app", "install", bad).returncode != 0, f"aplikasi {bad!r} harus ditolak"
        assert lab("app", "remove", bad).returncode != 0


def test_app_memcached_install_remove_roundtrip():
    if "memcached   terpasang" in lab("app", "list").stdout:
        pytest.skip("memcached sudah terpasang sebelum tes: tidak dihapus agar tidak mengubah mesin")
    try:
        r = lab("app", "install", "memcached", timeout=600)
        assert r.returncode == 0, "pasang gagal (jaringan?): " + (r.stdout + r.stderr)[-300:]
        assert "memcached   terpasang" in lab("app", "list").stdout
        assert subprocess.run(["/usr/bin/nc", "-z", "-w", "2", "127.0.0.1", "11211"]).returncode != 0, "pasang saja tidak boleh menyalakan layanan"
        st = lab("service", "start", "memcached")
        assert st.returncode == 0, st.stdout + st.stderr
        for _ in range(10):
            if subprocess.run(["/usr/bin/nc", "-z", "-w", "2", "127.0.0.1", "11211"]).returncode == 0:
                break
            time.sleep(1)
        assert subprocess.run(["/usr/bin/nc", "-z", "-w", "3", "127.0.0.1", "11211"]).returncode == 0, "memcached harus mendengar di 11211 setelah start"
        addr = subprocess.run(["/usr/sbin/lsof", "-nP", "-iTCP:11211", "-sTCP:LISTEN"], capture_output=True, text=True).stdout
        assert "127.0.0.1" in addr and "*:11211" not in addr, "harus bind loopback saja: " + addr
    finally:
        r = lab("app", "remove", "memcached", timeout=300)
    assert r.returncode == 0, r.stdout + r.stderr
    assert "memcached   belum" in lab("app", "list").stdout
    assert subprocess.run(["/usr/bin/nc", "-z", "-w", "2", "127.0.0.1", "11211"]).returncode != 0, "setelah hapus port harus tertutup"


# ---------------- worker ----------------


def test_worker_lifecycle(tmp_path):
    name = "zz-live-" + secrets.token_hex(3)
    folder = ROOT / "wwwroot" / name
    folder.mkdir(parents=True)
    label = f"com.lab.worker.{name}"
    try:
        r = lab("worker", "add", name, "--dir", str(folder), "--cmd", "/bin/sleep 600")
        assert r.returncode == 0, r.stdout + r.stderr
        time.sleep(2)
        assert name in lab("worker", "list").stdout
        out = subprocess.run(["launchctl", "list"], capture_output=True, text=True).stdout
        assert label in out, "label launchd harus terdaftar"
        st = lab("worker", "status", name)
        assert st.returncode == 0 and "jalan" in st.stdout, st.stdout + st.stderr
        assert lab("worker", "restart", name).returncode == 0
        time.sleep(2)
        assert "jalan" in lab("worker", "status", name).stdout
        assert lab("worker", "logs", name, "-n", "5").returncode == 0
        dup = lab("worker", "add", name, "--dir", str(folder), "--cmd", "/bin/sleep 600")
        assert dup.returncode in (0, 1, 2)  # menimpa atau menolak; tidak boleh meninggalkan dua proses
        procs = subprocess.run(["pgrep", "-f", "sleep 600"], capture_output=True, text=True).stdout.split()
        assert len(procs) <= 2, f"proses ganda setelah add ulang: {procs}"
    finally:
        r = lab("worker", "remove", name)
        subprocess.run(["rm", "-rf", str(folder)])
    assert r.returncode == 0, r.stdout + r.stderr
    time.sleep(1)
    assert label not in subprocess.run(["launchctl", "list"], capture_output=True, text=True).stdout, "label harus hilang"
    assert subprocess.run(["pgrep", "-f", "sleep 600"], capture_output=True, text=True).stdout.strip() == "", "proses harus mati"
    assert name not in lab("worker", "list").stdout


def test_worker_rejects_unsafe_input(tmp_path):
    folder = ROOT / "wwwroot" / ("zz-live-" + secrets.token_hex(3))
    folder.mkdir(parents=True)
    try:
        for args in (
            ["BAD", "--dir", str(folder), "--cmd", "/bin/echo hi"],
            ["zz-x", "--dir", "/etc", "--cmd", "/bin/echo hi"],
            ["zz-x", "--dir", str(folder), "--cmd", "programtakada-xyz"],
            ["zz-x", "--dir", str(folder), "--cmd", "/bin/echo hi", "--env", "PATH=/evil"],
        ):
            assert lab("worker", "add", *args).returncode != 0, args
    finally:
        subprocess.run(["rm", "-rf", str(folder)])
    assert "zz-x" not in lab("worker", "list").stdout
