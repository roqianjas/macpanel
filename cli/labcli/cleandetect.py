"""Deteksi dinamis cache aplikasi untuk `lab clean`: subfolder langsung di ~/Library/Caches dan ~/.cache.

Prinsip keamanan (lebih ketat dari katalog tetap karena isinya tidak dikenal):
- Hanya SUBFOLDER LANGSUNG kedua akar itu, bukan symlink, dengan nama yang lolos pola ketat. Tidak ada jalur dari masukan.
- Id berbentuk "lc:<nama>" (Library/Caches) dan "xc:<nama>" (.cache); jalur dibangun ulang dari nama saat dipakai.
- Kelas dihitung ulang setiap kali dipakai, tidak dipercaya dari hasil analisis lama:
    terkunci : cache sistem Apple (com.apple.*), aplikasinya sedang berjalan, tidak terbaca (izin), atau nama tak didukung.
               Tidak pernah dihapus.
    aman     : cache aplikasi yatim (aplikasinya sudah tidak terpasang). Boleh dihapus.
    tinjau   : cache aplikasi terpasang atau alat yang isinya tidak bisa dibuktikan aman. Dihapus hanya dengan persetujuan eksplisit.
- Yang sudah ada di katalog tetap dilewati (tidak ganda).
"""

import plistlib
import re
import subprocess
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

NAME_OK = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,89}$")
ROOTS = {"lc": "Library/Caches", "xc": ".cache"}
APP_DIRS = ("/Applications", "~/Applications", "/System/Applications", "/Applications/Utilities")


def installed_apps(home, dirs=None):
    """{bundle_id_kecil: nama_app} dari Info.plist aplikasi terpasang (satu tingkat, termasuk subfolder seperti Utilities)."""
    out = {}
    roots = [Path(str(d).replace("~", str(home))) for d in (dirs or APP_DIRS)]
    for root in roots:
        if not root.is_dir():
            continue
        try:
            apps = list(root.glob("*.app")) + list(root.glob("*/*.app"))
        except OSError:
            continue
        for app in apps:
            try:
                info = plistlib.loads((app / "Contents/Info.plist").read_bytes())
            except (OSError, ValueError, plistlib.InvalidFileException):
                continue
            bid = str(info.get("CFBundleIdentifier") or "").lower()
            if bid:
                out[bid] = str(info.get("CFBundleName") or app.stem)
    return out


def running_comms():
    r = subprocess.run(["/bin/ps", "-Ao", "comm="], capture_output=True, text=True)
    return {Path(x.strip()).name.lower() for x in r.stdout.splitlines() if x.strip()}


def bundle_like(name):
    return name.count(".") >= 2 and bool(re.match(r"^[A-Za-z0-9-]+\.[A-Za-z0-9-]+\.", name))


def _related(folder, bid):
    f = folder.lower()
    return f == bid or f.startswith(bid + ".") or bid.startswith(f + ".")


def classify(name, apps, comms):
    """(kelas, sumber, yatim, berjalan, alasan). Murni: tidak menyentuh disk, supaya mudah diuji."""
    n = name.lower()
    if not NAME_OK.match(name):
        return "terkunci", "lainnya", False, False, "nama tidak didukung"
    if n.startswith("com.apple."):
        return "terkunci", "apple", False, False, "cache sistem Apple"
    src = "app" if bundle_like(name) else "lainnya"
    match = [(b, label) for b, label in apps.items() if _related(name, b)]
    names = {name.lower(), *(label.lower() for _, label in match)}
    on = sorted(x for x in names if x in comms)
    if on:
        return "terkunci", src, False, True, "aplikasinya sedang berjalan: " + ", ".join(on)
    orphan = src == "app" and not match
    if orphan:
        return "aman", src, True, False, "tidak ada aplikasi terpasang dengan bundle id ini"
    return "tinjau", src, False, False, "isinya tidak bisa dibuktikan aman; hapus hanya bila Anda yakin"


def covered(catalog_paths):
    return {str(Path(p)) for p in catalog_paths}


def candidates(home, covered_paths):
    """[(id, nama, akar_tampil, path)] subfolder langsung yang layak dipindai."""
    out = []
    for key, rel in ROOTS.items():
        root = Path(home) / rel
        if not root.is_dir() or root.is_symlink():
            continue
        try:
            kids = sorted(root.iterdir())
        except OSError:
            continue
        for d in kids:
            if d.name.startswith(".") or d.is_symlink() or not d.is_dir():
                continue
            if str(d) in covered_paths:
                continue
            out.append((f"{key}:{d.name}", d.name, "~/" + rel, d))
    return out


def parse_id(id_, home):
    """Path dari id "lc:nama"/"xc:nama", atau None bila bentuknya tidak sah atau tidak ada."""
    m = re.fullmatch(r"(lc|xc):(.+)", id_ or "")
    if not m or not NAME_OK.match(m.group(2)) or ".." in m.group(2):
        return None
    d = Path(home) / ROOTS[m.group(1)] / m.group(2)
    if d.is_symlink() or not d.is_dir():
        return None
    if d.parent != Path(home) / ROOTS[m.group(1)]:
        return None
    return d


def scan_rows(home, covered_paths, size_fn, apps=None, comms=None, workers=8):
    """Semua baris terdeteksi, terurut menurut ukuran (penyaringan ukuran dilakukan UI). size_fn(path) -> (byte, terbaca)."""
    apps = installed_apps(home) if apps is None else apps
    comms = running_comms() if comms is None else comms
    cands = candidates(home, covered_paths)
    with ThreadPoolExecutor(max_workers=workers) as ex:
        sizes = list(ex.map(lambda c: size_fn(c[3]), cands))
    rows = []
    for (id_, name, loc, path), (b, ok) in zip(cands, sizes):
        cls, src, orphan, on, why = classify(name, apps, comms)
        if not ok and cls != "terkunci":
            cls, why = "terkunci", "tidak terbaca penuh (izin macOS)"
        if b <= 0:
            continue
        rows.append(dict(id=id_, label=name, tier="detected", cls=cls, src=src, orphan=orphan, running=on, loc=loc, bytes=b, complete=ok, why=why, cost="", paths=[str(path).replace(str(home), "~")]))
    rows.sort(key=lambda r: -r["bytes"])
    return rows
