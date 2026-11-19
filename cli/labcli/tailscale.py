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


def status():
    data, err = tailscale_status()
    env = detect()
    return {"formula": env.formula, "daemon_plist": env.daemon, "tailnet": data, "error": err}


def tailscale_cmd(a):
    if a.tact == "status":
        print(json.dumps(status()))
        return None
    env = detect()
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
