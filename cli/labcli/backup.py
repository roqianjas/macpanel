"""lab backup: backup terjadwal, daftar, ekstrak, pembersihan."""

import json
import os
import re
import shutil
import subprocess
import sys
import time

from labcli.core import ENV, HOME, OPS, ROOT, die, du_kb, sh, stream

STAGING = ROOT / "backup/staging"


BK_LABEL = "com.macminiops.backup"


BK_PLIST = HOME / "Library/LaunchAgents" / f"{BK_LABEL}.plist"


BK_CONF = HOME / ".config/macminiops/backup.conf"


RESTORE_DIR = ROOT / "backup/restore"


BK_NAME = re.compile(r"^\d{8}-\d{6}$")


PARTS = {"developer": "developer.tar.gz", "dotfiles": "dotfiles.tar.gz"}


PATH_OK = re.compile(r"^[A-Za-z0-9._@+ /-]{1,300}$")


def bk_archive(name, part):
    if not BK_NAME.match(name or ""):
        die("nama backup tidak valid")
    if part not in PARTS:
        die(f"bagian: {', '.join(PARTS)}")
    f = STAGING / name / PARTS[part]
    if not f.is_file():
        die("arsip tidak ada")
    return f


BK_DB = ROOT / "backup/db"


BK_RESTORE = ROOT / "backup/restore"


BK_RETIRED = ROOT / "backup/retired"


DUMP_RE = re.compile(r"^[a-z0-9_-]+-(mariadb|mysql|pgsql)-\d{8}-\d{6}\.sql\.gz$")


EXTRACT_RE = re.compile(r"^\d{8}-\d{6}-(developer|dotfiles)-\d{6}$")


def bk_read_sched():
    """(aktif, tiap_n_hari) dari backup.conf. Rusak/tidak ada = aktif, tiap 1 hari."""
    en, every = True, 1
    if BK_CONF.exists():
        t = BK_CONF.read_text()
        m = re.search(r"^ENABLED=([01])$", t, re.MULTILINE)
        if m:
            en = m.group(1) == "1"
        m = re.search(r"^EVERY_DAYS=(\d{1,2})$", t, re.MULTILINE)
        if m and 1 <= int(m.group(1)) <= 30:
            every = int(m.group(1))
    return en, every


def bk_read_conf():
    keep, days, extra = 7, 0, 0
    if BK_CONF.exists():
        t = BK_CONF.read_text()
        for k in ("KEEP", "DAYS", "EXTRA_DAYS"):
            m = re.search(rf"^{k}=(\d{{1,4}})$", t, re.MULTILINE)
            if m:
                v = int(m.group(1))
                if k == "KEEP" and 1 <= v <= 60:
                    keep = v
                if k == "DAYS" and v <= 3650:
                    days = v
                if k == "EXTRA_DAYS" and v <= 3650:
                    extra = v
    return keep, days, extra


def bk_names():
    if not STAGING.exists():
        return []
    return sorted((p.name for p in STAGING.iterdir() if BK_NAME.match(p.name) and p.is_dir() and not p.is_symlink()), reverse=True)


def bk_age_days(name):
    return (time.time() - time.mktime(time.strptime(name, "%Y%m%d-%H%M%S"))) / 86400


def dir_kb(p):
    return du_kb(p) or 0


def bk_plan(keep, days):
    """Daftar backup yang akan dihapus (terbaru -> lama). Yang terbaru TIDAK PERNAH dihapus."""
    plan = []
    for i, n in enumerate(bk_names()):
        if i == 0:
            continue
        if i >= keep:
            plan.append((n, f"melebihi {keep} backup terakhir"))
        elif days and bk_age_days(n) > days:
            plan.append((n, f"lebih tua dari {days} hari"))
    return plan


def bk_remove(name):
    if not BK_NAME.match(name or ""):
        die("nama backup tidak valid")
    p = STAGING / name
    if p.is_symlink() or not p.is_dir() or p.resolve().parent != STAGING.resolve():
        die("backup tidak ada")
    shutil.rmtree(p)


def extras_list(kind):
    if kind == "db":
        base = BK_DB
        items = [p for p in base.iterdir() if DUMP_RE.match(p.name) and p.is_file() and not p.is_symlink()] if base.exists() else []
    else:
        base = BK_RESTORE
        items = [p for p in base.iterdir() if EXTRACT_RE.match(p.name) and p.is_dir() and not p.is_symlink()] if base.exists() else []
    return items


def extras_remove(kind, days):
    gone = 0
    kb = 0
    for p in extras_list(kind):
        if days and (time.time() - p.stat().st_mtime) / 86400 <= days:
            continue
        k = dir_kb(p)
        shutil.rmtree(p) if p.is_dir() else p.unlink()
        gone += 1
        kb += k
    return gone, kb


def bk_prune(a):
    keep, days, extra = bk_read_conf()
    if a.keep is not None:
        if not 1 <= a.keep <= 60:
            die("simpan 1-60 backup")
        keep = a.keep
    if a.days is not None:
        if not 0 <= a.days <= 3650:
            die("umur 0-3650 hari")
        days = a.days
    plan = bk_plan(keep, days)
    if a.dry_run:
        print(json.dumps(dict(keep=keep, days=days, remove=[dict(name=n, reason=r, kb=dir_kb(STAGING / n)) for n, r in plan], kept=len(bk_names()) - len(plan))))
        return
    for n, r in plan:
        bk_remove(n)
        print(f"dihapus: {n} ({r})")
    if a.auto and extra:
        for kind in ("db", "restore"):
            g, kb = extras_remove(kind, extra)
            if g:
                print(f"dihapus: {g} item {kind} lebih tua dari {extra} hari ({kb // 1024} MB)")
    print(f"OK: {len(plan)} backup dihapus, {len(bk_names())} tersisa")


def bk_delete(a):
    names = bk_names()
    if not names:
        die("tidak ada backup")
    for n in a.names:
        if not BK_NAME.match(n):
            die(f"nama backup tidak valid: {n!r}")
        if n == names[0]:
            die(f"{n} adalah backup terbaru; tidak bisa dihapus agar selalu ada satu backup")
        if n not in names:
            die(f"backup {n} tidak ada")
    kb = 0
    for n in a.names:
        kb += dir_kb(STAGING / n)
        bk_remove(n)
    print(f"OK: {len(a.names)} backup dihapus, {kb // 1024} MB dibebaskan")


def bk_extras(a):
    if a.act2 == "info":
        out = {}
        for kind in ("db", "restore"):
            its = extras_list(kind)
            out[kind] = dict(count=len(its), kb=sum(dir_kb(p) for p in its), oldest=int(min((p.stat().st_mtime for p in its), default=0)))
        out["retired"] = dict(kb=dir_kb(BK_RETIRED) if BK_RETIRED.exists() else 0, note="data lama hasil migrasi (mis. MariaDB); hapus manual lewat Terminal bila yakin")
        print(json.dumps(out))
        return
    if a.days < 0 or a.days > 3650:
        die("umur 0-3650 hari (0 = semua)")
    if a.dry_run:
        its = [p for p in extras_list(a.act2) if not a.days or (time.time() - p.stat().st_mtime) / 86400 > a.days]
        print(json.dumps(dict(count=len(its), kb=sum(dir_kb(p) for p in its))))
        return
    g, kb = extras_remove(a.act2, a.days)
    print(f"OK: {g} item dihapus, {kb // 1024} MB dibebaskan")


def bk_config(a):
    import plistlib

    if a.cact == "get":
        t, keep, days, extra = "02:30", 7, 0, 0
        try:
            pl = plistlib.loads(BK_PLIST.read_bytes())
            c = pl["StartCalendarInterval"]
            t = f"{c['Hour']:02d}:{c['Minute']:02d}"
        except (OSError, KeyError, ValueError):
            pass
        if BK_CONF.exists():
            m = re.search(r"^KEEP=(\d{1,3})$", BK_CONF.read_text(), re.MULTILINE)
            if m:
                keep = int(m.group(1))
        keep, days, extra = bk_read_conf()
        en, every = bk_read_sched()
        print(json.dumps(dict(time=t, keep=keep, days=days, extra_days=extra, enabled=en, every_days=every)))
        return
    tm = re.fullmatch(r"([01]\d|2[0-3]):([0-5]\d)", a.time or "")
    if not tm:
        die("jam format HH:MM (00:00-23:59)")
    if not (1 <= a.keep <= 60):
        die("simpan 1-60 backup")
    if not (0 <= a.days <= 3650):
        die("umur maksimal 0-3650 hari (0 = tanpa batas umur)")
    if not (0 <= a.extra_days <= 3650):
        die("umur dump/ekstrak 0-3650 hari (0 = jangan hapus otomatis)")
    if a.enabled not in ("yes", "no"):
        die("--enabled harus yes atau no")
    if not (1 <= a.every_days <= 30):
        die("interval 1-30 hari")
    en = a.enabled == "yes"
    BK_CONF.parent.mkdir(parents=True, exist_ok=True)
    BK_CONF.write_text(f"KEEP={a.keep}\nDAYS={a.days}\nEXTRA_DAYS={a.extra_days}\nENABLED={int(en)}\nEVERY_DAYS={a.every_days}\n")
    os.chmod(BK_CONF, 0o600)
    pl = plistlib.loads(BK_PLIST.read_bytes())
    shutil.copy2(BK_PLIST, f"{BK_PLIST}.bak.{time.strftime('%Y%m%d-%H%M%S')}")
    pl["StartCalendarInterval"] = {"Hour": int(tm.group(1)), "Minute": int(tm.group(2))}
    BK_PLIST.write_bytes(plistlib.dumps(pl))
    uid = os.getuid()
    sh(["/bin/launchctl", "bootout", f"gui/{uid}/{BK_LABEL}"])
    if not en:
        # disable bertahan melewati reboot; plist tetap ada sehingga bisa dinyalakan lagi
        sh(["/bin/launchctl", "disable", f"gui/{uid}/{BK_LABEL}"])
        print("OK: backup otomatis DIMATIKAN. Backup manual (lab backup run) tetap bisa dijalankan.")
        return
    sh(["/bin/launchctl", "enable", f"gui/{uid}/{BK_LABEL}"])
    r = sh(["/bin/launchctl", "bootstrap", f"gui/{uid}", str(BK_PLIST)])
    if r.returncode:
        die("launchctl gagal: " + (r.stderr or r.stdout).strip()[-200:])
    print(
        f"OK: backup {'harian' if a.every_days == 1 else f'tiap {a.every_days} hari'} pukul {a.time}, simpan {a.keep} terakhir"
        + (f", hapus yang lebih tua dari {a.days} hari" if a.days else "")
        + (f"; dump/ekstrak > {a.extra_days} hari dibersihkan otomatis" if a.extra_days else "")
    )


def bk_ls(a):
    f = bk_archive(a.name, a.part)
    pre = (a.prefix or "").strip("/")
    if pre and (not PATH_OK.match(pre) or ".." in pre.split("/")):
        die("path tidak valid")
    r = subprocess.run(["/usr/bin/tar", "-tzf", str(f)], capture_output=True, text=True)
    if r.returncode:
        die("arsip tidak terbaca")
    seen = {}
    for l in r.stdout.splitlines():
        l = l.strip("/")
        if pre and not (l == pre or l.startswith(pre + "/")):
            continue
        rest = l[len(pre) :].lstrip("/") if pre else l
        if not rest:
            continue
        head, _, tail = rest.partition("/")
        if any(x in head for x in ("node_modules", ".git")) and tail:
            seen.setdefault(head, True)
        seen[head] = seen.get(head, False) or bool(tail)
    ents = sorted(seen.items(), key=lambda kv: (not kv[1], kv[0].lower()))[:500]
    print(json.dumps(dict(prefix=pre, entries=[dict(name=n, dir=d) for n, d in ents])))


def bk_extract(a):
    f = bk_archive(a.name, a.part)
    p = (a.path or "").strip("/")
    if not p or not PATH_OK.match(p) or ".." in p.split("/") or p.startswith("-"):
        die("path tidak valid")
    RESTORE_DIR.mkdir(parents=True, exist_ok=True)
    os.chmod(RESTORE_DIR, 0o700)
    dest = RESTORE_DIR / f"{a.name}-{a.part}-{time.strftime('%H%M%S')}"
    dest.mkdir(mode=0o700)
    r = subprocess.run(["/usr/bin/tar", "-xzf", str(f), "-C", str(dest), "--", p], capture_output=True, text=True)
    if r.returncode:
        shutil.rmtree(dest, ignore_errors=True)
        die("ekstrak gagal: " + (r.stderr or "").strip()[-200:])
    n = sum(1 for _ in dest.rglob("*"))
    print(f"OK: {n} entri diekstrak ke {dest} (berkas asli tidak disentuh; salin manual bila perlu)")


def backup(a):
    if a.act == "config":
        return bk_config(a)
    if a.act == "ls":
        return bk_ls(a)
    if a.act == "extract":
        return bk_extract(a)
    if a.act == "prune":
        return bk_prune(a)
    if a.act == "delete":
        return bk_delete(a)
    if a.act == "extras":
        return bk_extras(a)
    if a.act == "run":
        sys.exit(stream([str(OPS / "scripts/backup.sh")], env={**ENV, "BACKUP_FORCE": "1"}))
    rows = []
    for d in sorted(STAGING.glob("20*"), reverse=True)[:60] if STAGING.exists() else []:
        if not d.is_dir():
            continue
        fs = [f for f in d.iterdir() if f.is_file()]
        rows.append(dict(name=d.name, at=int(d.stat().st_mtime), size=sum(f.stat().st_size for f in fs), files=sorted(f.name for f in fs)))
    print(json.dumps(dict(backups=rows, offsite=(HOME / ".config/macminiops/offsite").exists())))
