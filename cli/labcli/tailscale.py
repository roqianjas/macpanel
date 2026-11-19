"""lab tailscale: status dan pemasangan Tailscale (formula Homebrew + daemon sistem).

Panel dan SSH Anda terjangkau lewat tailnet, jadi pemasangan hanya menyiapkan paket dan daemon. Login (tailscale up) membuka
browser dan harus Anda lakukan sendiri; tidak pernah dijalankan dari sini. Tanpa --apply hanya rencana. Pelepasan: lab uninstall --level 4.
"""

import json
import os
import sys
from pathlib import Path

from labcli.netcheck import tailscale_status
from labcli.uninstall import BREW, TS_FORMULA, Runner, UninstallError, execute, step

DAEMON_PLIST = Path("/Library/LaunchDaemons/sh.brew.tailscale.plist")


class TsEnv:
    def __init__(self, formula, daemon, home=None, brew=BREW):
        self.formula, self.daemon = formula, daemon
        self.home = Path(home or Path.home())
        self.brew = Path(brew)
        self.mp = self.home / "MacPanel"


def detect():
    brew = Path(BREW)
    return TsEnv((brew / "Cellar" / TS_FORMULA).is_dir(), DAEMON_PLIST.exists())


def plan_install(env):
    """Murni. Langkah yang diperlukan agar paket terpasang dan daemon sistem berjalan; kosong bila sudah lengkap."""
    P = []
    if not env.formula:
        P.append(step(0, "install", "exec", "brew install tailscale", argv=[str(env.brew / "bin/brew"), "install", TS_FORMULA]))
    if not env.daemon or not env.formula:
        P.append(step(0, "install", "sudo", "jalankan tailscaled sebagai layanan sistem (hidup sejak boot)", argv=[str(env.brew / "bin/brew"), "services", "start", TS_FORMULA]))
    return P


NEXT = [
    "Login ke tailnet (membuka browser; hanya Anda yang bisa): tailscale up",
    "Pastikan wildcard DNS *.lab mengarah ke IP Tailscale baru: lab sys dns (IP berubah bila perangkat baru dibuat)",
    "Cek: lab tailscale status",
]


def _self_info(raw):
    """IP, nama, dan versi dari keluaran `tailscale status --json` yang sama (tanpa panggilan kedua)."""
    try:
        d = json.loads(raw)
    except ValueError:
        return {}
    me = d.get("Self") if isinstance(d.get("Self"), dict) else {}
    ips = [x for x in (me.get("TailscaleIPs") or []) if isinstance(x, str) and "." in x]
    return {"ip": ips[0] if ips else None, "name": str(me.get("DNSName") or "").rstrip(".")[:80] or None, "version": str(d.get("Version") or "")[:40] or None}


def status():
    import subprocess

    seen = []

    def run(argv, **kw):
        r = subprocess.run(argv, **kw)
        seen.append(r.stdout)
        return r

    data, err = tailscale_status(run)
    if data and seen:
        data.update(_self_info(seen[-1]))
    env = detect()
    return {"formula": env.formula, "daemon_plist": env.daemon, "tailnet": data, "error": err}


def plan_service(env, act):
    """Murni. start/stop/restart daemon (layanan sistem, butuh sudo) atau down (putuskan sambungan, daemon tetap)."""
    brew = str(env.brew / "bin/brew")
    if not env.formula:
        return []
    if act == "down":
        return [step(0, "ts", "exec", "putuskan sambungan tailnet (daemon tetap hidup; sambung lagi dengan: tailscale up)", argv=[str(env.brew / "bin/tailscale"), "down"])]
    words = {"start": "jalankan tailscaled", "stop": "hentikan tailscaled (akses tailnet putus saat ini juga)", "restart": "mulai ulang tailscaled (tailnet putus beberapa detik)"}
    return [step(0, "ts", "sudo", words[act], argv=[brew, "services", act, TS_FORMULA])]


def tailscale_cmd(a):
    if a.tact == "status":
        print(json.dumps(status()))
        return None
    env = detect()
    if a.tact in ("start", "stop", "restart", "down"):
        return _service_cmd(a, env)
    plan = plan_install(env)
    if not plan:
        print("Tailscale sudah terpasang dan daemon sistem terdaftar. Tidak ada yang perlu dilakukan.")
    else:
        print("Rencana pemasangan Tailscale:")
        for i, s in enumerate(plan, 1):
            print(f"  {i}. {'[sudo] ' if s['op'] == 'sudo' else ''}{s['desc']}")
    print("\nLangkah yang Anda jalankan sendiri sesudahnya:")
    for n in NEXT:
        print(f"  - {n}")
    if not a.apply or not plan:
        if plan:
            print("\nINI HANYA RENCANA. Tambahkan --apply untuk menjalankan.")
        return 0
    if os.geteuid() == 0:
        print("lab: jangan jalankan sebagai root; sudo dipanggil per langkah", file=sys.stderr)
        return 1
    try:
        return execute(env, plan, Runner())
    except UninstallError as e:
        print("lab:", e, file=sys.stderr)
        return 1


def _service_cmd(a, env):
    from labcli.uninstall import remote_over_tailnet

    plan = plan_service(env, a.tact)
    if not plan:
        print("lab: Tailscale tidak terpasang lewat Homebrew. Pasang dulu: lab tailscale install", file=sys.stderr)
        return 1
    cuts = a.tact in ("stop", "down")
    print(f"Rencana: {plan[0]['desc']}")
    if cuts:
        print("PERINGATAN: ini memutus akses jarak jauh ke panel dan SSH lewat tailnet sampai dinyalakan lagi dari layar Mac mini.")
    if not a.apply:
        print("\nINI HANYA RENCANA. Tambahkan --apply untuk menjalankan.")
        return 0
    if os.geteuid() == 0:
        print("lab: jangan jalankan sebagai root; sudo dipanggil per langkah", file=sys.stderr)
        return 1
    if cuts and remote_over_tailnet() and not a.allow_remote_cut:
        print(
            "lab: sesi SSH ini lewat tailnet; perintah ini memutus sesi ini dan Anda tidak bisa masuk lagi dari jauh. Jalankan dari layar Mac mini, atau tambahkan --allow-remote-cut bila memang sadar.",
            file=sys.stderr,
        )
        return 1
    try:
        return execute(env, plan, Runner())
    except UninstallError as e:
        print("lab:", e, file=sys.stderr)
        return 1
