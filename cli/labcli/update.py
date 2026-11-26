"""lab update: periksa dan terapkan pembaruan (Homebrew, mise, aplikasi npm, pustaka; MacPanel hanya diberitahukan)."""

import json
import re
import shutil
import sys
import time
import urllib.error
import urllib.request

from labcli import updatecheck as uc
from labcli.core import BREW, MISE, OPS, ROOT, UID, die, sh, stream
from labcli.runtime import TOOLS
from labcli.services import NR_NPM

# Aplikasi npm yang boleh di-upgrade dari panel: nama paket -> (folder di server/apps, label launchd, port untuk uji sehat).
NPM_APPS = {"9router": ("9router", "com.macminiops.9router", 20128)}


def collect_brew():
    """(item_homebrew, pins). Keluaran brew bisa diawali baris '==> ...', jadi JSON diambil mulai kurung kurawal pertama."""
    pins = set(sh([BREW, "list", "--pinned"]).stdout.split())
    out = uc.json_from(sh([BREW, "outdated", "--json=v2"]).stdout)
    leaves = set(sh([BREW, "leaves"]).stdout.split())
    svc = {ln.split()[0] for ln in sh([BREW, "services", "list"]).stdout.splitlines()[1:] if ln.split()}
    return uc.brew_items(out, pins, svc, leaves), pins


def collect(with_net=True):
    items, pins = collect_brew()
    errors = []
    o = sh([MISE, "outdated", "--json"])
    try:
        items += uc.mise_items(json.loads(o.stdout or "{}"), set(TOOLS))
    except ValueError:
        errors.append("mise")
    if with_net:
        for folder in uc.read_npm_apps(OPS / "apps"):
            res = uc.read_npm_outdated(NR_NPM, folder)
            if res is None:
                errors.append(f"npm:{folder.name}")
            else:
                items += uc.npm_items(folder.name, res, set(NPM_APPS))
        p = uc.fetch_panel(OPS / "panel")
        if p is None:
            if (OPS / "panel/.git").exists():
                errors.append("panel")
        else:
            items += uc.panel_item(*p)
    return items, sorted(pins), errors


def check(a):
    items, pins, errors = collect()
    if getattr(a, "json", False):
        print(json.dumps(dict(checked=int(time.time()), pins=pins, items=items, errors=errors, counts=uc.summarize(items))))
        return
    brew = [i for i in items if i["source"] == "homebrew"]
    print("Homebrew:" if brew else "Homebrew: semua terbaru")
    for i in brew:
        tag = "PIN" if i["pinned"] else ""
        print(f"  {i['name']:18} {i['from']} -> {i['to']}  {tag} {'(MAYOR: butuh dump/restore)' if i['major'] else ''}  [{i['kind']}]")
    mise = [i for i in items if i["source"] == "mise"]
    print("mise:" if mise else "mise: semua terbaru")
    for i in mise:
        print(f"  {i['name']:10} {i['from']} -> {i['to']}")
    npm = [i for i in items if i["source"] == "npm"]
    print("npm:" if npm else "npm: semua terbaru")
    for i in npm:
        print(f"  {i['name']:18} {i['from']} -> {i['to']}  {'(MAYOR)' if i['major'] else ''}")
    pn = [i for i in items if i["kind"] == "panel"]
    print(f"MacPanel: {pn[0]['behind']} commit baru di origin/main ({pn[0]['from']} -> {pn[0]['to']}); upgrade lewat make deploy" if pn else "MacPanel: terbaru")
    if errors:
        print("Gagal diperiksa:", ", ".join(errors))
    print("Pin aktif:", ", ".join(pins) or "-")
    print("Catatan: PostgreSQL/MySQL di-pin. Update patch: lab update apply <formula> --allow-pinned. Mayor: manual lewat dump.")


def services_of(name):
    out = sh([BREW, "services", "list"]).stdout.splitlines()
    return [ln.split()[0] for ln in out if ln.split() and ln.split()[0] == name and "started" in ln]


def _backup_or_die():
    if stream(["/bin/bash", str(OPS / "scripts/backup.sh")]) != 0:
        die("backup gagal, update dibatalkan")


def _log(line):
    log = ROOT / "backup/update-log.txt"
    log.parent.mkdir(exist_ok=True)
    with open(log, "a") as fh:
        fh.write(f"{time.strftime('%F %T')} {line}\n")


def _alive(port, wait=45):
    """True bila server menjawab HTTP (status apa pun di bawah 500) dalam `wait` detik."""
    end = time.time() + wait
    while time.time() < end:
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}/", timeout=3)
            return True
        except urllib.error.HTTPError as e:
            if e.code < 500:
                return True
        except OSError:
            pass
        time.sleep(2)
    return False


def apply_npm(a, name):
    if not uc.NPM_NAME.match(name) or name not in NPM_APPS:
        die(f"aplikasi npm {name!r} tidak dikenal atau tidak boleh di-upgrade dari sini")
    folder_name, label, port = NPM_APPS[name]
    folder = OPS / "apps" / folder_name
    res = uc.read_npm_outdated(NR_NPM, folder)
    if res is None:
        die("npm outdated gagal (jaringan atau npm); tidak ada yang diubah")
    row = res.get(name)
    if not row or not row.get("current") or row.get("current") == row.get("latest"):
        die(f"{name} tidak usang")
    cur, new = row["current"], row["latest"]
    if uc.is_major(cur, new):
        die(f"{cur} -> {new} berganti versi mayor (atau minor di seri 0.x): tidak otomatis, baca catatan rilis dulu")
    print(f"rencana: simpan package*.json -> npm install {name}@{new} (tanpa skrip) -> restart {label} -> uji sehat -> rollback bila gagal")
    if a.dry_run:
        return
    bk = ROOT / "backup" / f"update-npm-{name}-{time.strftime('%Y%m%d-%H%M%S')}"
    bk.mkdir(parents=True, exist_ok=True)
    bk.chmod(0o700)
    keep = [f for f in ("package.json", "package-lock.json") if (folder / f).exists()]
    for f in keep:
        shutil.copy2(folder / f, bk / f)
    _log(f"npm:{name} {cur} -> {new} (cadangan {bk.name})")

    def install(ver):
        return stream([str(NR_NPM), "install", "--prefix", str(folder), f"{name}@{ver}", "--ignore-scripts", "--no-audit", "--no-fund"])

    def restore():
        for f in keep:
            shutil.copy2(bk / f, folder / f)

    if install(new):
        restore()
        die("npm install gagal; package.json dikembalikan. Versi lama masih terpasang.")
    sh(["/bin/launchctl", "kickstart", "-k", f"gui/{UID}/{label}"])
    if not _alive(port):
        print(f"{name} tidak menjawab setelah upgrade; mengembalikan {cur} ...")
        install(cur)
        restore()
        sh(["/bin/launchctl", "kickstart", "-k", f"gui/{UID}/{label}"])
        die(f"upgrade {name} dibatalkan: tidak sehat setelah restart. Dikembalikan ke {cur} ({'sehat' if _alive(port, 30) else 'CEK MANUAL: belum menjawab'}).")
    print(f"OK: {name} {cur} -> {new} | restart {label} | menjawab di :{port}")


def apply_libs(a):
    items, _ = collect_brew()
    libs = [i for i in items if i["kind"] == "lib"]
    ok = [i for i in libs if not i["major"] and not i["pinned"]]
    skip = len(libs) - len(ok)
    if not ok:
        die("tidak ada pustaka usang yang aman diperbarui" + (f" ({skip} berganti mayor, dilewati)" if skip else ""))
    names = [i["name"] for i in ok]
    print(f"rencana: backup -> brew upgrade {len(names)} pustaka: {', '.join(names)}" + (f" (dilewati {skip} mayor)" if skip else ""))
    if a.dry_run:
        return
    _backup_or_die()
    _log(f"lib:all {len(names)} pustaka: {' '.join(names)}")
    if stream([BREW, "upgrade", *names]):
        die("brew upgrade pustaka gagal. Cek: brew doctor. Backup ada di ~/MacPanel/backup/staging")
    print(f"OK: {len(names)} pustaka diperbarui")


def apply(a):
    t = a.target
    if t == "panel":
        die("MacPanel diperbarui lewat make deploy di repo panel, bukan dari sini")
    if t == "lib:all":
        return apply_libs(a)
    if t.startswith("npm:"):
        return apply_npm(a, t[4:])
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
    items, _ = collect_brew()
    row = next((i for i in items if i["name"] == t), None)
    if not row:
        die(f"{t} tidak usang (atau tidak terpasang)")
    if row["kind"] == "lib":
        die(f"{t} adalah pustaka dependensi: perbarui bersama lewat 'lab update apply lib:all'")
    cur, new = row["from"], row["to"]
    if row["major"]:
        die(f"{cur} -> {new} berganti versi mayor: tidak otomatis (dump, pasang, restore)")
    pinned = row["pinned"]
    if pinned and not a.allow_pinned:
        die(f"{t} di-pin; tambahkan --allow-pinned bila yakin (hanya patch/minor)")
    svc = services_of(t)
    print(f"rencana: backup -> {'unpin -> ' if pinned else ''}upgrade {t} {cur} -> {new}{' -> pin' if pinned else ''}{' -> restart service' if svc else ''}")
    if a.dry_run:
        return
    _backup_or_die()
    _log(f"{t} {cur} -> {new}")
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
