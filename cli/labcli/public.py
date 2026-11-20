"""lab public: buka SATU situs ke internet sementara lewat Cloudflare Quick Tunnel (cloudflared, tanpa akun/domain).

Aman secara desain:
- Tiap situs punya listener loopback sendiri (127.0.0.1:PORT, tanpa tailnet_only karena trafik tunnel datang dari mesin ini). Tunnel hanya
  menunjuk ke listener itu, jadi Host palsu tidak bisa pindah ke situs lain.
- Listener adalah salinan blok server HTTPS situs itu (php/static/proxy/auth tetap berlaku), tanpa TLS lokal.
- Selalu ada batas waktu (TTL). Setelah itu tunnel, listener, dan state dibersihkan otomatis.
- Situs sistem dan proxy ke layanan internal ditolak. Tidak dinyalakan lagi setelah reboot.
"""

import json
import os
import plistlib
import re
import signal
import subprocess
import sys
import time
from pathlib import Path

from labcli.core import HOME, LABBIN, OPS, PROTECTED, SITE_NAME_RE, SITES, UID, die, sh
from labcli.nginx import NGINX_CONF, nginx_test

CLOUDFLARED = "/opt/homebrew/bin/cloudflared"
PUBLIC_DIR = HOME / ".config/macminiops/public"
CONF_DIR = OPS / "nginx/public"  # di luar sites/: panel dan `lab site` tidak boleh menganggapnya situs
INCLUDE_LINE = f"include {CONF_DIR}/*.conf;"
PORT_MIN, PORT_MAX = 8450, 8499
TTL_DEFAULT, TTL_MIN, TTL_MAX = 120, 5, 1440
DENY = PROTECTED | {"9router", "adminer", "mailpit", "netdata"}
INTERNAL_PORTS = {22, 3306, 3307, 5432, 6379, 8025, 8080, 9100, 9101, 19999, 20128, 4873}
URL_RE = re.compile(r"https://[a-z0-9-]+\.trycloudflare\.com")
LABEL = "com.macminiops.public."


def conf_path(name):
    return CONF_DIR / f"public-{name}.conf"


def ensure_include():
    """nginx.conf lama belum memuat nginx/public/*.conf: tambahkan sekali (dengan cadangan, dibatalkan bila nginx -t gagal)."""
    text = NGINX_CONF.read_text()
    if INCLUDE_LINE in text:
        return
    m = re.search(r"^([ \t]*)include\s+\S*/nginx/sites/\*\.conf;[ \t]*$", text, re.M)
    if not m:
        die("nginx.conf tidak memuat nginx/sites/*.conf; tambahkan sendiri: " + INCLUDE_LINE)
    bak = NGINX_CONF.with_name(f"nginx.conf.bak.{time.strftime('%Y%m%d-%H%M%S')}")
    bak.write_text(text)
    NGINX_CONF.write_text(text[: m.end()] + f"\n{m.group(1)}{INCLUDE_LINE}" + text[m.end() :])
    t = nginx_test()
    if t.returncode:
        NGINX_CONF.write_text(text)
        die("nginx -t gagal setelah menambah include: " + (t.stdout + t.stderr).strip()[-200:])


def state_path(name):
    return PUBLIC_DIR / f"{name}.json"


def read_state(name):
    try:
        return json.loads(state_path(name).read_text())
    except (OSError, ValueError):
        return None


def write_state(name, st):
    PUBLIC_DIR.mkdir(parents=True, exist_ok=True)
    os.chmod(PUBLIC_DIR, 0o700)
    p = state_path(name)
    tmp = p.with_suffix(".tmp")
    tmp.write_text(json.dumps(st))
    os.chmod(tmp, 0o600)
    tmp.replace(p)


def registered(name):
    return sh(["/bin/launchctl", "print", f"gui/{UID}/{LABEL}{name}"]).returncode == 0


def loaded(name):
    """Job terdaftar DAN prosesnya hidup (job yang sudah keluar tetap terdaftar di launchd sampai bootout)."""
    r = sh(["/bin/launchctl", "print", f"gui/{UID}/{LABEL}{name}"])
    return r.returncode == 0 and re.search(r"^\s*state = running\s*$", r.stdout, re.M) is not None


def check_name(name):
    if not SITE_NAME_RE.fullmatch(name or ""):
        die("nama situs tidak valid")
    if name in DENY:
        die(f"situs sistem '{name}' tidak boleh dibuka ke publik")
    if not (SITES / f"{name}.conf").exists():
        die("situs tidak ada atau sedang dijeda")


def server_blocks(text):
    """(awal, akhir) tiap blok `server { ... }` tingkat atas."""
    out, i = [], 0
    while True:
        m = re.compile(r"(?m)^\s*server\s*\{").search(text, i)
        if not m:
            return out
        depth, j = 0, m.end() - 1
        while j < len(text):
            depth += (text[j] == "{") - (text[j] == "}")
            if depth == 0:
                break
            j += 1
        out.append((m.start(), j + 1))
        i = j + 1


DROP = re.compile(r"^\s*(listen|server_name|ssl_[a-z_]+|http2|allow|deny)\b.*$|^\s*include\s+\S*tailnet_only\.conf\s*;.*$")


def top_level_filter(rest):
    """Buang direktif TLS/tailnet hanya di tingkat server; allow/deny di dalam location tetap (itu perlindungan situs)."""
    out, depth = [], 0
    for ln in rest.splitlines():
        if not (depth == 0 and DROP.match(ln)):
            out.append(ln)
        depth += ln.count("{") - ln.count("}")
    return out


def build_conf(name, port, text):
    """Salin blok HTTPS situs menjadi listener loopback tanpa TLS dan tanpa pembatas tailnet."""
    blocks = [b for b in server_blocks(text) if re.search(r"listen\s+(\[::\]:)?443\b", text[b[0] : b[1]])]
    if len(blocks) != 1:
        die("config situs tidak punya tepat satu blok HTTPS; tidak bisa dibuka ke publik otomatis")
    body = text[blocks[0][0] : blocks[0][1]]
    if re.search(r"proxy_pass\s+https?://127\.0\.0\.1:8080\b", body):
        die("situs Apache belum didukung untuk akses publik")
    for m in re.finditer(r"proxy_pass\s+https?://(?:127\.0\.0\.1|localhost|\[::1\]):(\d+)", body):
        if int(m.group(1)) in INTERNAL_PORTS:
            die(f"proxy ke layanan internal (port {m.group(1)}) tidak boleh dibuka ke publik")
    head, rest = body.split("{", 1)
    lines = top_level_filter(rest)
    inner = "\n".join(lines).replace("$scheme", "https")
    if not re.search(r"location\s+~\s+/\\\.", inner):  # file tersembunyi (.env, .git) tidak boleh terbaca lewat internet
        inner = "    location ~ /\\.(?!well-known) { return 404; }\n" + inner
    return (
        f"# Dibangkitkan oleh `lab public on {name}`. Jangan diedit; hilang otomatis saat tunnel mati.\n"
        f"server {{\n    listen 127.0.0.1:{port};\n    listen [::1]:{port};\n    server_name _;\n    server_tokens off;\n"
        f"    if ($request_method !~ ^(GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS)$) {{ return 405; }}\n{inner}\n"
    )


def pick_port():
    used = set()
    for p in PUBLIC_DIR.glob("*.json") if PUBLIC_DIR.exists() else []:
        try:
            used.add(json.loads(p.read_text())["port"])
        except (OSError, ValueError, KeyError):
            pass
    for port in range(PORT_MIN, PORT_MAX + 1):
        if port in used:
            continue
        if sh(["/usr/sbin/lsof", "-nP", f"-iTCP:{port}", "-sTCP:LISTEN"]).stdout.strip():
            continue
        return port
    die("semua port publik terpakai (maks 50 situs)")


def reload_nginx():
    t = nginx_test()
    if t.returncode:
        return (t.stdout + t.stderr).strip()[-300:]
    sh(["/opt/homebrew/bin/nginx", "-s", "reload"])
    return ""


def cleanup(name):
    """Idempotent: hapus listener + state, muat ulang nginx."""
    had = conf_path(name).exists()
    conf_path(name).unlink(missing_ok=True)
    state_path(name).unlink(missing_ok=True)
    if had:
        reload_nginx()


def plist_for(name, port, ttl):
    f = PUBLIC_DIR / f"{name}.plist"
    PUBLIC_DIR.mkdir(parents=True, exist_ok=True)
    os.chmod(PUBLIC_DIR, 0o700)
    f.write_bytes(
        plistlib.dumps(
            {
                "Label": LABEL + name,
                "ProgramArguments": [LABBIN, "public", "_run", name, str(port), str(ttl)],
                "RunAtLoad": True,
                "KeepAlive": False,
                "ProcessType": "Background",
                "StandardOutPath": str(PUBLIC_DIR / f"{name}.log"),
                "StandardErrorPath": str(PUBLIC_DIR / f"{name}.log"),
            }
        )
    )
    os.chmod(f, 0o600)
    return f


def stop_job(name):
    if registered(name):
        sh(["/bin/launchctl", "bootout", f"gui/{UID}/{LABEL}{name}"])
        for _ in range(40):
            if not registered(name):
                break
            time.sleep(0.25)


def on(a):
    name = a.name
    check_name(name)
    if not Path(CLOUDFLARED).exists():
        die("Cloudflare Tunnel belum terpasang (lab app install cloudflared)")
    ttl = int(a.ttl)
    if not TTL_MIN <= ttl <= TTL_MAX:
        die(f"batas waktu {TTL_MIN}-{TTL_MAX} menit")
    st = read_state(name)
    if st and loaded(name):
        print(f"OK: sudah publik: {st.get('url', '(menunggu alamat)')}")
        return
    stop_job(name)
    cleanup(name)
    port = pick_port()
    text = (SITES / f"{name}.conf").read_text()
    new = build_conf(name, port, text)
    ensure_include()
    CONF_DIR.mkdir(parents=True, exist_ok=True)
    conf_path(name).write_text(new)
    err = reload_nginx()
    if err:
        conf_path(name).unlink(missing_ok=True)
        die("nginx menolak listener publik: " + err)
    write_state(name, {"port": port, "ttl_min": ttl, "started": int(time.time()), "expires": int(time.time()) + ttl * 60, "url": ""})
    plist = plist_for(name, port, ttl)
    r = sh(["/bin/launchctl", "bootstrap", f"gui/{UID}", str(plist)])
    if r.returncode:
        cleanup(name)
        die("gagal memulai tunnel: " + (r.stderr or r.stdout).strip()[-200:])
    for _ in range(80):
        time.sleep(0.5)
        st = read_state(name)
        if st and st.get("url"):
            print(f"OK: {name} terbuka ke publik sampai {time.strftime('%H:%M', time.localtime(st['expires']))}\n{st['url']}")
            return
        if not loaded(name):
            break
    stop_job(name)
    cleanup(name)
    tail = ""
    try:
        tail = (PUBLIC_DIR / f"{name}.log").read_text(errors="replace")[-200:]
    except OSError:
        pass
    die("tunnel tidak mendapat alamat publik (cek koneksi internet). " + tail.strip().replace("\n", " "))


def off_one(name):
    stop_job(name)
    cleanup(name)
    (PUBLIC_DIR / f"{name}.plist").unlink(missing_ok=True)
    (PUBLIC_DIR / f"{name}.log").unlink(missing_ok=True)


def names_active():
    if not PUBLIC_DIR.exists():
        return []
    return sorted({p.stem for p in PUBLIC_DIR.glob("*.json")} | {p.stem for p in PUBLIC_DIR.glob("*.plist")})


def off(a):
    if a.all:
        ns = names_active()
        for n in ns:
            off_one(n)
        for p in CONF_DIR.glob("public-*.conf"):
            n = p.stem[len("public-") :]
            cleanup(n)
        print(f"OK: {len(ns)} tunnel dimatikan")
        return
    if not a.name or not SITE_NAME_RE.fullmatch(a.name):
        die("nama situs tidak valid (atau pakai --all)")
    off_one(a.name)
    print(f"OK: {a.name} tidak lagi publik")


def status_rows():
    rows = []
    now = int(time.time())
    for n in names_active():
        st = read_state(n) or {}
        if not loaded(n):
            if now - st.get("started", 0) > 90:  # sisa sesudah reboot/crash/TTL habis: bersihkan
                off_one(n)
            continue
        rows.append(
            {
                "site": n,
                "active": True,
                "url": st.get("url", ""),
                "port": st.get("port", 0),
                "started": st.get("started", 0),
                "expires": st.get("expires", 0),
                "seconds_left": max(0, st.get("expires", now) - now),
            }
        )
    return rows


def status(a):
    rows = status_rows()
    if a.name:
        rows = [r for r in rows if r["site"] == a.name]
    info = {"installed": Path(CLOUDFLARED).exists(), "ttl_default": TTL_DEFAULT, "ttl_min": TTL_MIN, "ttl_max": TTL_MAX, "tunnels": rows}
    if a.json:
        print(json.dumps(info))
        return
    if not rows:
        print("tidak ada situs yang terbuka ke publik")
    for r in rows:
        print(f"{r['site']:20}{r['seconds_left'] // 60:>5} mnt  {r['url'] or '(menunggu alamat)'}")


def run(a):
    """Dijalankan launchd: jalankan cloudflared, tangkap alamat, matikan saat TTL habis atau diminta berhenti."""
    name, port, ttl = a.name, int(a.port), int(a.ttl)
    deadline = time.time() + ttl * 60
    p = subprocess.Popen(
        [CLOUDFLARED, "tunnel", "--no-autoupdate", "--url", f"http://127.0.0.1:{port}"],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        env={"PATH": "/opt/homebrew/bin:/usr/bin:/bin", "HOME": str(HOME)},
    )

    def bye(*_):
        try:
            p.terminate()
            p.wait(timeout=8)
        except Exception:
            p.kill()
        cleanup(name)
        (PUBLIC_DIR / f"{name}.plist").unlink(missing_ok=True)
        sys.exit(0)

    signal.signal(signal.SIGTERM, bye)
    signal.signal(signal.SIGINT, bye)
    import select

    assert p.stdout is not None
    got, url = False, ""
    while time.time() < deadline and p.poll() is None:
        r, _, _ = select.select([p.stdout], [], [], 1.0)
        if not r:
            continue
        line = p.stdout.readline()
        if not line:
            break
        if (m := URL_RE.search(line)) and not url:
            url = m.group(0)
        if url and not got and "Registered tunnel connection" in line:  # alamat baru diumumkan setelah tunnel benar-benar tersambung
            st = read_state(name) or {}
            st["url"] = url
            write_state(name, st)
            got = True
    bye()


def public(a):
    {"on": on, "off": off, "status": status, "_run": run}[a.act](a)
