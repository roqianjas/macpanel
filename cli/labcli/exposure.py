"""lab sys exposure: layanan yang mendengar di luar loopback dan benar-benar terjangkau dari jaringan rumah.

Sumber dan batasnya (jujur):
- `lsof` memberi nama proses, tetapi hanya untuk proses milik user ini. Layanan milik root (mis. SSH, Screen Sharing)
  tidak terlihat. `netstat` di macOS ini tidak lagi mendaftar listener, jadi tidak dipakai.
- Untuk menutup celah itu, port 1-1024 dan beberapa port penting disambung dari IP LAN mesin ini. Port yang terjangkau
  tetapi tidak dijelaskan lsof dilaporkan sebagai "proses tidak terlihat" dengan tebakan nama layanan dari nomor portnya.
- "Terjangkau dari LAN" berarti sambungan TCP ke IP LAN berhasil. Itu BUKAN berarti aman diakses: nginx menjawab tetapi
  menolak (403) yang bukan dari tailnet. Daftar pengecualian yang disengaja ada di sisi panel.
Tanpa sudo.
"""

import concurrent.futures
import ipaddress
import json
import socket
import subprocess
import time

from labcli.core import ENV

TAILNET4 = ipaddress.ip_network("100.64.0.0/10")
TAILNET6 = ipaddress.ip_network("fd7a:115c:a1e0::/48")
EXTRA_PORTS = (1025, 3000, 3001, 3306, 3307, 5000, 5432, 5900, 6379, 7000, 8025, 8080, 8443, 9100, 9101, 9119, 11211, 19999, 20128, 27017)
SYSTEM_PREFIXES = ("/System/", "/usr/libexec/", "/usr/sbin/", "/sbin/")
MAX_LISTENERS = 200


def classify_bind(bind):
    """loopback | tailnet | all (semua antarmuka) | lan (alamat tertentu di luar loopback/tailnet)."""
    b = (bind or "").strip().strip("[]")
    if b in ("*", "0.0.0.0", "::", ""):
        return "all"
    if b == "localhost":
        return "loopback"
    try:
        ip = ipaddress.ip_address(b.split("%")[0])
    except ValueError:
        return "lan"
    if ip.is_loopback:
        return "loopback"
    if (ip.version == 4 and ip in TAILNET4) or (ip.version == 6 and ip in TAILNET6):
        return "tailnet"
    return "lan"


def parse_lsof(text):
    """Keluaran `lsof +c 0 -nP -iTCP -sTCP:LISTEN -F pcn` -> [{pid, cmd, bind, port}], satu baris per (nama, alamat, port)."""
    out, seen = [], {}
    pid, cmd = None, ""
    for line in text.splitlines():
        if not line:
            continue
        tag, val = line[0], line[1:]
        if tag == "p":
            pid, cmd = (int(val) if val.isdigit() else None), ""
        elif tag == "c":
            cmd = val
        elif tag == "n" and pid is not None and ":" in val:
            host, _, port = val.rpartition(":")
            if not port.isdigit() or not (1 <= int(port) <= 65535):
                continue
            key = (cmd, host, int(port))  # nginx punya banyak worker pada port yang sama: satu baris cukup (pid terkecil)
            if key not in seen:
                seen[key] = len(out)
                out.append({"pid": pid, "cmd": cmd, "bind": host, "port": int(port)})
            elif pid < out[seen[key]]["pid"]:
                out[seen[key]]["pid"] = pid
    return out


def is_system_path(path):
    return bool(path) and path.startswith(SYSTEM_PREFIXES)


def service_name(port):
    try:
        return socket.getservbyport(port, "tcp")
    except OSError:
        return ""


def lan_ip():
    """IP antarmuka utama (rute bawaan), atau None."""
    r = subprocess.run(["/sbin/route", "-n", "get", "default"], capture_output=True, text=True, env=ENV, timeout=5)
    iface = next((ln.split(":", 1)[1].strip() for ln in r.stdout.splitlines() if ln.strip().startswith("interface:")), "")
    for cand in ([iface] if iface else []) + ["en0", "en1"]:
        g = subprocess.run(["/usr/sbin/ipconfig", "getifaddr", cand], capture_output=True, text=True, env=ENV, timeout=5)
        if g.returncode == 0 and g.stdout.strip():
            return g.stdout.strip()
    return None


def probe(host, port, timeout=0.4):
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def scan(host, ports, timeout=0.4, workers=64):
    ports = sorted(set(ports))
    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as ex:
        res = list(ex.map(lambda p: (p, probe(host, p, timeout)), ports))
    return {p for p, ok in res if ok}


def build(lsof_rows, paths, reachable, ip):
    """Gabungkan hasil lsof, path proses, dan port yang terjangkau dari LAN (himpunan) menjadi daftar pendengar. Murni."""
    rows, known_ports = [], set()
    for r in lsof_rows[:MAX_LISTENERS]:
        scope = classify_bind(r["bind"])
        known_ports.add(r["port"])
        path = paths.get(r["pid"], "")
        reach = None
        if scope in ("all", "lan") and ip:
            reach = r["port"] in reachable
        rows.append(
            {
                "proc": r["cmd"] or "?",
                "pid": r["pid"],
                "port": r["port"],
                "bind": r["bind"] or "*",
                "scope": scope,
                "system": is_system_path(path),
                "lan_reachable": reach,
                "service": service_name(r["port"]),
                "hidden": False,
            }
        )
    for p in sorted(reachable - known_ports):
        rows.append({"proc": "?", "pid": None, "port": p, "bind": "?", "scope": "all", "system": False, "lan_reachable": True, "service": service_name(p), "hidden": True})
    rows.sort(key=lambda x: (x["scope"] == "loopback", x["port"], x["proc"]))
    return rows[:MAX_LISTENERS]


def sys_exposure():
    errors = []
    r = subprocess.run(["/usr/sbin/lsof", "+c", "0", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcn"], capture_output=True, text=True, env=ENV, timeout=20)
    rows = parse_lsof(r.stdout)
    if r.returncode not in (0, 1) or (not rows and r.stderr.strip()):
        errors.append("lsof gagal: " + r.stderr.strip()[-120:])
    paths = {}
    for pid in {x["pid"] for x in rows}:
        p = subprocess.run(["/bin/ps", "-o", "comm=", "-p", str(pid)], capture_output=True, text=True, env=ENV, timeout=5)
        paths[pid] = p.stdout.strip()
    ip = lan_ip()
    reachable = set()
    if ip:
        ports = set(range(1, 1025)) | set(EXTRA_PORTS) | {x["port"] for x in rows}
        reachable = scan(ip, ports)
    else:
        errors.append("IP LAN tidak terbaca: keterjangkauan dari LAN tidak diuji")
    print(json.dumps({"at": int(time.time()), "lan_ip": ip, "listeners": build(rows, paths, reachable, ip), "errors": errors}))
