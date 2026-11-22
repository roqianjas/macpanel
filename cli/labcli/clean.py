"""lab clean / lab mem: analisis disk, pembersihan aman berbasis katalog, dan ringkasan memori.

Prinsip keamanan:
- Yang dihapus HANYA entri katalog di bawah (jalur dideklarasikan di kode, bukan dari masukan). Tidak ada jalur bebas.
- Satu tingkat tambahan "detected": subfolder langsung ~/Library/Caches dan ~/.cache yang ditemukan otomatis (lihat cleandetect.py); kelas dihitung ulang tiap dipakai.
- Tiga tingkat: safe (cache yang dibuat ulang sendiri), careful (perlu persetujuan; folder backup sekali pakai), show (hanya ditampilkan, tidak pernah dihapus).
- `run` default hanya pratinjau (dry-run); `--apply` menjalankan. Tingkat careful butuh `--confirm-careful`.
- Entri ditolak bila proses terkait sedang jalan (go, brew, npm, ...). Symlink tidak pernah diikuti keluar dari jalur entri.
- Ukuran dari `du` nyata; yang tidak terbaca ditandai, tidak ditebak.
"""

import json
import os
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path

from labcli import cleandetect
from labcli.core import HOME, ROOT, die

SCAN_FILE = HOME / ".config/macminiops/clean-scan.json"
LOG_DIR = HOME / "Library/Logs"
LOG_AGE_DAYS = 14
CAREFUL_BK = re.compile(r"^(migrasi-[A-Za-z0-9._-]{1,80}|unduh-mac|pre-macpanel-[0-9-]{1,30})$")
MAX_ITEMS = 8


class CleanError(Exception):
    pass


def E(id_, label, tier, paths, why, cost="", busy=(), kind="contents", items=False, **extra):
    return dict(id=id_, label=label, tier=tier, paths=[str(p) for p in paths], why=why, cost=cost, busy=list(busy), kind=kind, items=items, **extra)


def catalog(home=None, root=None):
    """Katalog entri. `home`/`root` bisa diganti untuk pengujian."""
    h = Path(home or HOME)
    r = Path(root or ROOT)
    c = h / "Library/Caches"
    cat = [
        E("go-build", "Cache build Go", "safe", [c / "go-build"], "Hasil kompilasi sementara; Go membuatnya ulang sendiri.", "Build Go berikutnya lebih lambat.", ["go"]),
        E("homebrew-cache", "Cache unduhan Homebrew", "safe", [c / "Homebrew"], "Berkas unduhan paket yang sudah terpasang.", "Instal ulang paket akan mengunduh lagi.", ["brew"]),
        E("npm-cache", "Cache npm", "safe", [h / ".npm/_cacache"], "Salinan paket npm yang pernah diunduh.", "Instal npm berikutnya mengunduh ulang.", ["npm", "pnpm"]),
        E("pnpm-cache", "Cache pnpm", "safe", [c / "pnpm"], "Metadata dan cache unduhan pnpm.", "Instal pnpm berikutnya lebih lambat.", ["pnpm"]),
        E("pip-uv-cache", "Cache pip dan uv", "safe", [c / "pip", h / ".cache/uv"], "Cache paket Python.", "Instal Python berikutnya mengunduh ulang.", ["pip", "pip3", "uv"]),
        E("composer-cache", "Cache Composer", "safe", [c / "composer", h / ".composer/cache"], "Cache paket PHP.", "Composer berikutnya mengunduh ulang.", ["composer"]),
        E("node-gyp", "Cache node-gyp", "safe", [c / "node-gyp"], "Header Node untuk kompilasi modul native.", "Kompilasi modul native mengunduh header lagi.", ["node-gyp"]),
        E("golangci-lint", "Cache golangci-lint", "safe", [c / "golangci-lint"], "Cache analisis lint.", "Lint berikutnya lebih lambat.", ["golangci-lint"]),
        E("colima-cache", "Cache image Colima", "safe", [c / "colima"], "Image OS untuk VM Docker yang sudah diunduh.", "Colima mengunduh image lagi saat dinyalakan.", ["limactl", "colima"]),
        E("panel-cache", "Cache proxy npm panel", "safe", [h / ".cache/macpanel"], "Tarball yang disimpan proxy resume (lab proxy).", "Unduhan berikutnya mengulang dari registry.", []),
        E(
            "chrome-cache",
            "Cache Google Chrome",
            "safe",
            [c / "Google"],
            "Cache halaman dan profil peramban.",
            "Situs dimuat lebih lambat sekali. Chrome harus ditutup.",
            ["Google Chrome", "Google Chrome Helper"],
        ),
        E(
            "puppeteer-cache",
            "Chromium unduhan Puppeteer",
            "safe",
            [h / ".cache/puppeteer"],
            "Peramban uji yang diunduh Puppeteer.",
            "Puppeteer mengunduh ulang (±500 MB) saat dipakai lagi.",
            ["node", "chrome-headless-shell"],
        ),
        E(
            "hermes-pnpm",
            "Cache pnpm milik Hermes",
            "safe",
            [h / ".hermes/cache/pnpm"],
            "Cache unduhan paket untuk alat Hermes.",
            "Pemasangan alat Hermes berikutnya mengunduh ulang.",
            ["pnpm", "npm"],
        ),
        E("hermes-uv", "Cache uv milik Hermes", "safe", [h / ".hermes/cache/uv"], "Cache paket Python untuk alat Hermes.", "Pemasangan berikutnya mengunduh ulang.", ["uv"]),
        E("trash", "Tempat Sampah", "safe", [h / ".Trash"], "Berkas yang sudah Anda buang.", "Tidak bisa dikembalikan setelah dikosongkan.", []),
        E("logs-rotated", f"Log lama (>{LOG_AGE_DAYS} hari)", "safe", [h / "Library/Logs"], "Log berputar (.gz atau .log.N) di ~/Library/Logs.", "Riwayat log lama hilang.", [], kind="logs"),
    ]
    bk = r / "backup"
    if bk.is_dir():
        for d in sorted(bk.iterdir()):
            if d.is_dir() and not d.is_symlink() and CAREFUL_BK.match(d.name):
                cat.append(
                    E(
                        f"bk:{d.name}",
                        f"Folder backup {d.name}",
                        "careful",
                        [d],
                        "Dibuat saat migrasi atau unduhan; bukan backup rutin.",
                        "Tidak bisa dikembalikan; mungkin satu-satunya salinan migrasi. Pastikan agen terkait sudah selesai.",
                        [],
                        kind="tree",
                        items=True,
                    )
                )
    hb = h / ".hermes/backups"
    if hb.is_dir() and not hb.is_symlink():
        cat.append(
            E(
                "cx:hermes-backups",
                "Backup Hermes",
                "careful",
                [hb],
                "Salinan konfigurasi dan database Kanban Hermes sebelum perubahan.",
                "Tidak bisa dikembalikan; bisa jadi satu-satunya salinan konfigurasi lama.",
                [],
                kind="contents",
                items=True,
            )
        )
    show = [
        ("hermes", "Data Hermes (~/.hermes)", [h / ".hermes"], "Profil, sesi, dan cache Hermes serta agen lain. Kelola lewat Hermes."),
        ("documents", "Dokumen (~/Documents)", [h / "Documents"], "File pribadi Anda."),
        ("wwwroot", "Situs (MacPanel/wwwroot)", [r / "wwwroot"], "Kode situs dan aplikasi."),
        (
            "database-data",
            "Data database (Homebrew/var)",
            [Path("/opt/homebrew/var/mysql"), Path("/opt/homebrew/var/mariadb"), Path("/opt/homebrew/var/postgresql@17")],
            "Isi database; hapus lewat halaman Database.",
        ),
        ("backup-routine", "Backup rutin (MacPanel/backup/staging dll.)", [bk / "staging", bk / "db", bk / "config-bak"], "Atur retensi dan jadwal di halaman Backup."),
        ("colima-data", "Data VM Docker (~/.colima)", [h / ".colima"], "Image dan volume Docker; kelola lewat halaman Docker."),
        ("applications", "Aplikasi (/Applications)", [Path("/Applications")], "Hapus lewat Finder."),
        ("app-support", "Application Support", [h / "Library/Application Support"], "Data aplikasi; jangan dihapus manual."),
    ]
    for id_, label, paths, why in show:
        cat.append(E(id_, label, "show", paths, why, "", [], kind="show"))
    return cat


# ------------------------------------------------------------------ ukuran


def du_bytes(path, timeout=180):
    """(byte, terbaca). Tidak mengikuti symlink; galat izin = tidak terbaca penuh."""
    p = str(path)
    if not os.path.lexists(p):
        return 0, True
    if os.path.islink(p):
        return 0, True
    try:
        r = subprocess.run(["/usr/bin/du", "-sk", p], capture_output=True, text=True, timeout=timeout)
    except (subprocess.TimeoutExpired, OSError):
        return 0, False
    m = re.match(r"^(\d+)", r.stdout or "")
    if not m:
        return 0, False
    return int(m.group(1)) * 1024, r.returncode == 0 and "Operation not permitted" not in (r.stderr or "")


def old_logs(directory=LOG_DIR, days=LOG_AGE_DAYS, now=None):
    """Log berputar yang lebih tua dari `days`. Log aktif (.log) tidak pernah masuk."""
    now = now or time.time()
    out = []
    d = Path(directory)
    if not d.is_dir():
        return out
    for f in d.iterdir():
        if not f.is_file() or f.is_symlink():
            continue
        n = f.name
        rotated = n.endswith(".gz") or re.search(r"\.log\.[0-9A-Za-z-]+$", n) is not None
        try:
            if rotated and now - f.stat().st_mtime > days * 86400:
                out.append(f)
        except OSError:
            continue
    return out


def entry_size(e):
    if e["kind"] == "logs":
        fs = old_logs(e["paths"][0])
        total = 0
        for f in fs:
            try:
                total += f.stat().st_size
            except OSError:
                pass
        return total, True
    total, ok = 0, True
    for p in e["paths"]:
        b, k = du_bytes(p)
        total += b
        ok = ok and k
    return total, ok


def entry_items(e, limit=MAX_ITEMS):
    out = []
    for p in e["paths"]:
        d = Path(p)
        if d.is_dir() and not d.is_symlink():
            for f in sorted(d.iterdir())[:limit]:
                out.append(f.name + ("/" if f.is_dir() else ""))
    return out[:limit]


def entry_age_days(e, now=None):
    now = now or time.time()
    try:
        return int((now - os.lstat(e["paths"][0]).st_mtime) // 86400)
    except OSError:
        return None


def running(names):
    if not names:
        return []
    r = subprocess.run(["/bin/ps", "-Ao", "comm="], capture_output=True, text=True)
    comms = {Path(x.strip()).name for x in r.stdout.splitlines()}
    return [n for n in names if n in comms]


# ------------------------------------------------------------------ scan


def detected_entry(id_, home=None, apps=None, comms=None):
    """Entri untuk id "lc:nama"/"xc:nama" (cache aplikasi yang terdeteksi), dengan kelas dihitung ulang sekarang. None bila tidak sah."""
    h = Path(home or HOME)
    d = cleandetect.parse_id(id_, h)
    if d is None:
        return None
    apps = cleandetect.installed_apps(h) if apps is None else apps
    comms = cleandetect.running_comms() if comms is None else comms
    cls, src, orphan, on, why = cleandetect.classify(d.name, apps, comms)
    if cls != "terkunci" and not du_bytes(d, timeout=60)[1]:
        cls, why = "terkunci", "tidak terbaca penuh (izin macOS)"
    return E(id_, d.name, "detected", [d], why, "", [], kind="contents", cls=cls, src=src, orphan=orphan)


def scan(cat=None, out=None):
    detect = cat is None
    cat = cat or catalog()
    rows = []
    for e in cat:
        size, ok = entry_size(e)
        row = dict(id=e["id"], label=e["label"], tier=e["tier"], bytes=size, complete=ok, why=e["why"], cost=e["cost"], paths=[p.replace(str(HOME), "~") for p in e["paths"]])
        if e["tier"] != "show":
            row["busy"] = running(e["busy"])
        if e["items"]:
            row["items"] = entry_items(e)
            row["age_days"] = entry_age_days(e)
        rows.append(row)
    if detect:
        covered = cleandetect.covered(p for e in cat for p in e["paths"])
        rows.extend(cleandetect.scan_rows(HOME, covered, lambda p: du_bytes(p, timeout=60)))
    rows.sort(key=lambda r: ({"safe": 0, "careful": 1, "show": 2, "detected": 3}[r["tier"]], -r["bytes"]))
    safe = sum(r["bytes"] for r in rows if r["tier"] == "safe")
    careful = sum(r["bytes"] for r in rows if r["tier"] == "careful")
    detected = sum(r["bytes"] for r in rows if r["tier"] == "detected" and r["cls"] != "terkunci")
    free = shutil.disk_usage("/System/Volumes/Data" if os.path.exists("/System/Volumes/Data") else "/").free
    return dict(at=int(time.time()), rows=rows, safe_bytes=safe, careful_bytes=careful, detected_bytes=detected, disk_free=free)


def save_scan(data, path=None):
    p = Path(path or SCAN_FILE)
    p.parent.mkdir(parents=True, exist_ok=True)
    tmp = p.with_suffix(".tmp")
    tmp.write_text(json.dumps(data))
    os.chmod(tmp, 0o600)
    os.replace(tmp, p)


def load_scan(path=None):
    try:
        return json.loads(Path(path or SCAN_FILE).read_text())
    except (OSError, ValueError):
        return None


# ------------------------------------------------------------------ hapus


def _rm_child(child):
    if child.is_symlink() or child.is_file():
        child.unlink()
    elif child.is_dir():
        shutil.rmtree(child)


def clean_entry(e, apply):
    """Mengembalikan (byte_yang_dihapus_atau_akan_dihapus, galat). Hanya jalur entri; isi direktori, bukan direktori induknya, kecuali tipe tree."""
    if e["tier"] == "show":
        raise CleanError("entri ini hanya ditampilkan, tidak pernah dihapus")
    busy = running(e["busy"])
    if busy:
        raise CleanError(f"proses terkait sedang berjalan: {', '.join(busy)}")
    size, _ = entry_size(e)
    if not apply:
        return size, None
    errs = []
    if e["kind"] == "logs":
        for f in old_logs(e["paths"][0]):
            try:
                f.unlink()
            except OSError as x:
                errs.append(str(x))
    elif e["kind"] == "tree":
        p = Path(e["paths"][0])
        if p.is_symlink() or not p.is_dir():
            raise CleanError("bukan folder biasa")
        shutil.rmtree(p)
    else:
        for p in e["paths"]:
            d = Path(p)
            if d.is_symlink() or not d.is_dir():
                continue
            for child in d.iterdir():
                try:
                    _rm_child(child)
                except OSError as x:
                    errs.append(f"{child.name}: {x.strerror or x}")
    after, _ = entry_size(e) if e["kind"] != "tree" else (0, True)
    return max(size - after, 0), ("; ".join(errs[:3]) if errs else None)


def run(ids, apply, confirm_careful, cat=None, confirm_review=False):
    cat = cat or catalog()
    by = {e["id"]: e for e in cat}
    apps = comms = None
    items, skipped, total = [], [], 0
    if not ids:
        raise CleanError("sebutkan id entri (lihat: lab clean scan)")
    for i in ids:
        e = by.get(i)
        if e is None and re.match(r"^(lc|xc):", i or ""):
            if apps is None:
                apps, comms = cleandetect.installed_apps(HOME), cleandetect.running_comms()
            e = detected_entry(i, apps=apps, comms=comms)
            if e is None:
                skipped.append(dict(id=i, reason="folder tidak ada atau bukan subfolder langsung yang sah"))
                continue
            if e["cls"] == "terkunci":
                skipped.append(dict(id=i, reason=f"terkunci: {e['why']}"))
                continue
            if e["cls"] == "tinjau" and apply and not confirm_review:
                skipped.append(dict(id=i, reason="cache yang perlu ditinjau butuh --confirm-review"))
                continue
        if e is None:
            skipped.append(dict(id=i, reason="id tidak ada di katalog"))
            continue
        if e["tier"] == "show":
            skipped.append(dict(id=i, reason="hanya ditampilkan, tidak pernah dihapus"))
            continue
        if e["tier"] == "careful" and apply and not confirm_careful:
            skipped.append(dict(id=i, reason="tingkat Hati-hati butuh --confirm-careful"))
            continue
        try:
            b, err = clean_entry(e, apply)
        except CleanError as x:
            skipped.append(dict(id=i, reason=str(x)))
            continue
        total += b
        items.append(dict(id=i, label=e["label"], tier=e["tier"], bytes=b, error=err))
    return dict(dry_run=not apply, items=items, skipped=skipped, bytes=total)


# ------------------------------------------------------------------ memori


LABELS = [
    (re.compile(r"php-fpm: (master|pool)|/php@?([0-9.]+)?/sbin/php-fpm|php-fpm"), "PHP-FPM"),
    (re.compile(r"mysqld"), "MySQL"),
    (re.compile(r"mariadbd"), "MariaDB"),
    (re.compile(r"/postgres( |$)|postgres:"), "PostgreSQL"),
    (re.compile(r"redis-server"), "Redis"),
    (re.compile(r"nginx: (master|worker)|/nginx( |$)"), "nginx"),
    (re.compile(r"netdata|go\.d\.plugin|apps\.plugin"), "Netdata"),
    (re.compile(r"limactl|Virtualization\.framework|colima"), "Docker (Colima VM)"),
    (re.compile(r"Uptime|uptime-kuma|kuma"), "Uptime Kuma"),
    (re.compile(r"9router"), "9router"),
    (re.compile(r"mailpit"), "Mailpit"),
    (re.compile(r"macpanel|/panel( |$)|cmd/panel"), "Panel"),
    (re.compile(r"hermes"), "Hermes"),
    (re.compile(r"phpunit|artisan"), "PHP (CLI/tes)"),
    (re.compile(r"Google Chrome|Brave|Safari|firefox", re.I), "Peramban"),
]


def label_of(cmd):
    for rx, name in LABELS:
        if rx.search(cmd):
            return name
    base = Path(cmd.split(" ")[0]).name
    return base or "?"


def parse_ps(text):
    out = []
    for ln in text.splitlines():
        m = re.match(r"\s*(\d+)\s+(\d+)\s+(.*)$", ln)
        if m:
            out.append((int(m.group(1)), int(m.group(2)), m.group(3)))
    return out


def group_procs(rows, top=10):
    g = {}
    for rss_kb, pid, cmd in rows:
        k = label_of(cmd)
        e = g.setdefault(k, dict(label=k, mb=0, procs=0, pids=[]))
        e["mb"] += rss_kb / 1024
        e["procs"] += 1
        if len(e["pids"]) < 5:
            e["pids"].append(pid)
    out = sorted(g.values(), key=lambda x: -x["mb"])[:top]
    for e in out:
        e["mb"] = round(e["mb"])
    return out


PHP_ETC = Path("/opt/homebrew/etc/php")


def used_php_versions(nginx_dir, php_etc=None):
    """Versi PHP-FPM yang dipakai nginx. Setiap socket di fastcgi_pass (situs DAN snippet, mis. Adminer) dipetakan ke versi yang
    mendefinisikannya: pola php-fpm-dev-X.sock langsung, selain itu dicari `listen = <socket>` di php-fpm.d tiap versi."""
    etc = Path(php_etc or PHP_ETC)
    socks = set()
    d = Path(nginx_dir)
    if d.is_dir():
        for f in d.rglob("*.conf"):
            try:
                socks.update(re.findall(r"fastcgi_pass\s+unix:([^\s;]+)", f.read_text()))
            except OSError:
                pass
    used = set()
    for sock in socks:
        m = re.search(r"php-fpm-dev-([0-9]+\.[0-9]+)\.sock$", sock)
        if m:
            used.add(m.group(1))
            continue
        found = False
        if etc.is_dir():
            for vdir in etc.iterdir():
                for pool in (vdir / "php-fpm.d").glob("*.conf") if (vdir / "php-fpm.d").is_dir() else []:
                    try:
                        if re.search(r"^\s*listen\s*=\s*" + re.escape(sock) + r"\s*$", pool.read_text(), re.M):
                            used.add(vdir.name)
                            found = True
                    except OSError:
                        pass
        if not found:
            used.add("?" + sock)  # socket tak dikenal: tandai, jangan menyimpulkan versi lain bebas dipakai
    return used


def suggestions(services, groups, used_php):
    """Saran penghematan dari layanan opsional yang jalan dan PHP-FPM yang tidak dipakai situs mana pun."""
    mb = {g["label"]: g["mb"] for g in groups}
    out = []
    for s in services:
        if not s.get("installed") or s.get("state") != "running":
            continue
        sid, label = s["id"], s.get("label", s["id"])
        m = re.fullmatch(r"php-([0-9]+\.[0-9]+)", sid)
        if m and m.group(1) not in used_php and not any(u.startswith("?") for u in used_php):
            out.append(dict(id=sid, label=label, action="stop", mb=None, text=f"{label} berjalan tetapi tidak dipakai situs mana pun."))
        elif s.get("optional"):
            guess = {"docker": mb.get("Docker (Colima VM)"), "netdata": mb.get("Netdata"), "mailpit": mb.get("Mailpit"), "kuma": mb.get("Uptime Kuma"), "9router": mb.get("9router")}.get(sid)
            out.append(dict(id=sid, label=label, action="stop", mb=guess, text=f"{label} opsional dan sedang berjalan. Matikan bila tidak dipakai."))
    return sorted(out, key=lambda x: -(x["mb"] or 0))


def vm_numbers():
    out = {}
    try:
        t = subprocess.run(["/usr/bin/vm_stat"], capture_output=True, text=True).stdout
        ps = int(re.search(r"page size of (\d+)", t).group(1))
        for k, rx in (
            ("wired", r"Pages wired down:\s+(\d+)"),
            ("active", r"Pages active:\s+(\d+)"),
            ("inactive", r"Pages inactive:\s+(\d+)"),
            ("compressed", r"Pages occupied by compressor:\s+(\d+)"),
            ("free", r"Pages free:\s+(\d+)"),
        ):
            m = re.search(rx, t)
            if m:
                out[k + "_mb"] = round(int(m.group(1)) * ps / 1048576)
        mp = subprocess.run(["/usr/bin/memory_pressure"], capture_output=True, text=True).stdout
        m = re.search(r"free percentage: (\d+)%", mp)
        if m:
            out["pressure_free_pct"] = int(m.group(1))
        sw = subprocess.run(["/usr/sbin/sysctl", "-n", "vm.swapusage"], capture_output=True, text=True).stdout
        m = re.search(r"used = ([0-9.]+)M", sw)
        if m:
            out["swap_used_mb"] = float(m.group(1))
        out["total_mb"] = round(int(subprocess.run(["/usr/sbin/sysctl", "-n", "hw.memsize"], capture_output=True, text=True).stdout) / 1048576)
    except (OSError, AttributeError, ValueError):
        pass
    return out


def mem_top():
    ps = subprocess.run(["/bin/ps", "-Ao", "rss=,pid=,command="], capture_output=True, text=True).stdout
    groups = group_procs(parse_ps(ps))
    try:
        r = subprocess.run([str(ROOT / "server/scripts/lab"), "service", "list", "--json"], capture_output=True, text=True, timeout=30)
        services = json.loads(r.stdout)
    except (OSError, ValueError, subprocess.TimeoutExpired):
        services = []
    if isinstance(services, dict):
        services = services.get("services", [])
    used = used_php_versions(ROOT / "server/nginx")
    return dict(
        mem=vm_numbers(),
        groups=groups,
        suggestions=suggestions(services, groups, used),
        purge_note="macOS mengelola cache sendiri; purge hanya mengosongkan cache file (aplikasi jadi sedikit lebih lambat setelahnya).",
    )


# ------------------------------------------------------------------ CLI


def clean_cmd(a):
    if a.cact == "scan":
        d = scan()
        save_scan(d)
        print(json.dumps(d))
        return None
    if a.cact == "last":
        print(json.dumps(load_scan()))
        return None
    try:
        res = run(a.ids, a.apply, a.confirm_careful, confirm_review=a.confirm_review)
    except CleanError as x:
        print("lab:", x, file=sys.stderr)
        return 1
    print(json.dumps(res))
    if a.apply and any(i.get("error") for i in res["items"]):
        return None
    return None


def mem_cmd(a):
    if a.mact == "top":
        print(json.dumps(mem_top()))
        return None
    die("perintah mem tidak dikenal")
