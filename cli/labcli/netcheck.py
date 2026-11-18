"""lab sys net: konektivitas (Tailscale, internet, DNS). Semua tanpa sudo, tanpa paket tambahan.

Yang diukur dan yang tidak:
- Tailscale: status dari `tailscale status --json` (BackendState, Self.Online, jumlah peer online).
- Internet: waktu sambung TCP ke 1.1.1.1:443 dan 8.8.8.8:443 (bukan ICMP ping; tidak butuh izin khusus dan menembus
  firewall yang memblokir ping). Latensi = yang tercepat; internet dianggap mati bila KEDUANYA gagal.
- DNS: resolusi nama lokal (nama yang diberikan panel, memakai resolver sistem seperti browser) dan satu nama publik.
Nilai yang gagal diukur dilaporkan null/false, tidak dikarang.
"""

import json
import shutil
import socket
import subprocess
import time

from labcli.core import ENV

INTERNET_TARGETS = (("1.1.1.1", 443), ("8.8.8.8", 443))
PUBLIC_NAME = "example.com"
TS_FALLBACK = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"


def parse_tailscale(text):
    """Keluaran `tailscale status --json` -> ringkasan, atau None bila bukan JSON yang dikenal."""
    try:
        d = json.loads(text)
    except ValueError:
        return None
    if not isinstance(d, dict) or not isinstance(d.get("BackendState"), str):
        return None
    peers = d.get("Peer") if isinstance(d.get("Peer"), dict) else {}
    online = sum(1 for p in peers.values() if isinstance(p, dict) and p.get("Online") is True)
    me = d.get("Self") if isinstance(d.get("Self"), dict) else {}
    health = d.get("Health") if isinstance(d.get("Health"), list) else []
    return {
        "state": d["BackendState"],  # Running, Stopped, NeedsLogin, Starting, ...
        "self_online": me.get("Online") is True,
        "peers": len(peers),
        "peers_online": online,
        "health": [str(h)[:120] for h in health[:3]],
    }


def tailscale_status(run=None):
    exe = shutil.which("tailscale", path=ENV.get("PATH"))
    if not exe:
        import os

        exe = TS_FALLBACK if os.access(TS_FALLBACK, os.X_OK) else None
    if not exe:
        return None, "tailscale tidak ditemukan"
    try:
        r = (run or subprocess.run)([exe, "status", "--json"], env=ENV, capture_output=True, text=True, timeout=8)
    except subprocess.TimeoutExpired:
        return None, "tailscale status timeout"
    d = parse_tailscale(r.stdout) if r.returncode == 0 else None
    return d, None if d else "tailscale status gagal atau keluarannya tidak dikenal"


def tcp_ms(host, port, timeout=3.0, connect=socket.create_connection, clock=time.monotonic):
    """Waktu sambung TCP dalam ms (1 desimal), atau None bila gagal."""
    t = clock()
    try:
        s = connect((host, port), timeout=timeout)
    except OSError:
        return None
    try:
        s.close()
    except OSError:
        pass
    return round((clock() - t) * 1000, 1)


def internet(targets=INTERNET_TARGETS, probe=tcp_ms):
    tried = [{"host": h, "ms": probe(h, p)} for h, p in targets]
    ok = [t["ms"] for t in tried if t["ms"] is not None]
    return {"ok": bool(ok), "ms": min(ok) if ok else None, "tried": tried}


def resolve(name, getaddr=socket.getaddrinfo, clock=time.monotonic):
    t = clock()
    try:
        r = getaddr(name, None)
    except OSError:
        return {"name": name, "ok": False, "ms": None}
    if not r:
        return {"name": name, "ok": False, "ms": None}
    return {"name": name, "ok": True, "ms": round((clock() - t) * 1000, 1)}


def net_snapshot(local_name, ts=tailscale_status, inet=internet, dns=resolve, now=time.time):
    d, err = ts()
    errors = [err] if err else []
    return {
        "at": int(now()),
        "tailscale": d,
        "internet": inet(),
        "dns_local": dns(local_name) if local_name else None,
        "dns_public": dns(PUBLIC_NAME),
        "errors": errors,
    }


def sys_net(local_name=""):
    print(json.dumps(net_snapshot(local_name)))
