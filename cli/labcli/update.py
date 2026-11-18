"""lab update: periksa dan terapkan pembaruan Homebrew."""

import json
import re
import sys
import time

from labcli.core import BREW, MISE, OPS, ROOT, die, sh, stream
from labcli.runtime import TOOLS


def check(a):
    d = json.loads(sh([BREW, "outdated", "--json=v2"]).stdout or "{}")
    pins = set(sh([BREW, "list", "--pinned"]).stdout.split())
    rows = d.get("formulae", [])
    print("Homebrew:" if rows else "Homebrew: semua terbaru")
    for f in rows:
        n = f["name"]
        cur = f["installed_versions"][0]
        new = f["current_version"]
        major = cur.split(".")[0] != new.split(".")[0]
        tag = "PIN" if n in pins else ""
        print(f"  {n:18} {cur} -> {new}  {tag} {'(MAYOR: butuh dump/restore)' if major else ''}")
    o = sh([MISE, "outdated", "--json"])
    try:
        mo = json.loads(o.stdout or "{}")
    except Exception:
        mo = {}
    print("mise:" if mo else "mise: semua terbaru")
    for k, v in mo.items():
        print(f"  {k:10} {v.get('current')} -> {v.get('latest')}")
    print("Pin aktif:", ", ".join(sorted(pins)) or "-")
    print("Catatan: PostgreSQL/MySQL di-pin. Update patch: lab update apply <formula> --allow-pinned. Mayor: manual lewat dump.")


def services_of(name):
    out = sh([BREW, "services", "list"]).stdout.splitlines()
    return [l.split()[0] for l in out if l.split() and l.split()[0] == name and "started" in l]


def apply(a):
    t = a.target
    if t.startswith("mise:"):
        tool = t[5:]
        if tool not in TOOLS:
            die("tool mise tidak dikenal")
        if a.dry_run:
            print("dry-run:", MISE, "upgrade", tool)
            return
        sys.exit(stream([MISE, "upgrade", tool]))
    if not re.fullmatch(r"[a-z0-9@._+-]+", t):
        die("nama formula tidak valid")
    d = json.loads(sh([BREW, "outdated", "--json=v2"]).stdout or "{}")
    row = next((f for f in d.get("formulae", []) if f["name"] == t), None)
    if not row:
        die(f"{t} tidak usang (atau tidak terpasang)")
    cur, new = row["installed_versions"][0], row["current_version"]
    if cur.split(".")[0] != new.split(".")[0]:
        die(f"{cur} -> {new} berganti versi mayor: tidak otomatis (dump, pasang, restore)")
    pinned = t in sh([BREW, "list", "--pinned"]).stdout.split()
    if pinned and not a.allow_pinned:
        die(f"{t} di-pin; tambahkan --allow-pinned bila yakin (hanya patch/minor)")
    svc = services_of(t)
    print(f"rencana: backup -> {'unpin -> ' if pinned else ''}upgrade {t} {cur} -> {new}{' -> pin' if pinned else ''}{' -> restart service' if svc else ''}")
    if a.dry_run:
        return
    if stream(["/bin/bash", str(OPS / "scripts/backup.sh")]) != 0:
        die("backup gagal, update dibatalkan")
    log = ROOT / "backup/update-log.txt"
    log.parent.mkdir(exist_ok=True)
    with open(log, "a") as fh:
        fh.write(f"{time.strftime('%F %T')} {t} {cur} -> {new}\n")
    if pinned:
        sh([BREW, "unpin", t], check=True)
    try:
        rc = stream([BREW, "upgrade", t])
    finally:
        if pinned:
            sh([BREW, "pin", t])
    if rc:
        die("brew upgrade gagal (pin dikembalikan). Versi lama masih di Cellar bila tersisa: brew info " + t)
    if svc:
        sh([BREW, "services", "restart", t])
        time.sleep(5)
        if not services_of(t):
            die(f"{t} tidak berjalan setelah update! Cek log. Backup ada di ~/MacPanel/backup/staging")
    now = sh([BREW, "list", "--versions", t]).stdout.strip()
    print(f"OK: {now}" + (" | service jalan" if svc else ""))
