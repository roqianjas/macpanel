"""lab site: tambah, hapus, aktifkan, nonaktifkan, dan sertifikat situs nginx."""

import re
import sys

from labcli.core import OPS, PROTECTED, ROOT, die, sh, stream
from labcli.nginx import cert, nginx_test

SITE_NAME_PAT = r"[a-z0-9]([a-z0-9-]*[a-z0-9])?"


SITES_DIR = OPS / "nginx/sites"


def site_toggle(name, enable):
    """Jeda/aktifkan situs dengan mengganti nama berkas config (nginx hanya memuat *.conf). Gagal uji = dikembalikan."""
    if not re.fullmatch(SITE_NAME_PAT, name or ""):
        die("nama situs tidak valid")
    if name in PROTECTED:
        die(f"situs sistem '{name}' tidak boleh dijeda")
    on, off = SITES_DIR / f"{name}.conf", SITES_DIR / f"{name}.conf.disabled"
    src, dst = (off, on) if enable else (on, off)
    if not src.exists():
        die(f"situs '{name}' tidak ada" if not dst.exists() else f"situs '{name}' sudah {'aktif' if enable else 'nonaktif'}")
    if dst.exists():
        die(f"{dst.name} sudah ada")
    src.rename(dst)
    t = nginx_test()
    if t.returncode:
        dst.rename(src)
        die("nginx -t gagal, dikembalikan: " + (t.stdout + t.stderr).strip()[-300:])
    r = sh(["/opt/homebrew/bin/nginx", "-s", "reload"])
    if r.returncode:
        dst.rename(src)
        sh(["/opt/homebrew/bin/nginx", "-s", "reload"])
        die("reload gagal, dikembalikan: " + (r.stdout + r.stderr).strip()[-200:])
    print(f"OK: {name}.lab {'diaktifkan' if enable else 'dijeda'}")


def site(a):
    s = str(OPS / "scripts/addsite.sh")
    if a.act == "list":
        sys.exit(stream([s, "--list"]))
    if a.act == "remove":
        if re.fullmatch(SITE_NAME_PAT, a.name or ""):
            (SITES_DIR / f"{a.name}.conf.disabled").unlink(missing_ok=True)
            if (ROOT / "wwwroot" / a.name / ".macpanel.json").exists():
                sh([str(OPS / "scripts/lab"), "worker", "remove", a.name])  # proyek Go/Python/Next.js: hentikan prosesnya juga
        sys.exit(stream([s, "--remove", a.name]))
    if a.act in ("enable", "disable"):
        site_toggle(a.name, a.act == "enable")
        return
    if a.act == "cert":
        cert(a.name, a.aliases)
        return
    cmd = [s, a.name, a.target] + (["--php", a.php] if a.php else []) + (["--wordpress"] if getattr(a, "wordpress", False) else [])
    sys.exit(stream(cmd))
