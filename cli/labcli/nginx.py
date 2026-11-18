"""lab nginx: uji, muat ulang, dan sertifikat lokal (mkcert)."""

import os
import re
import sys
from pathlib import Path

from labcli.core import OPS, die, sh

NGINX_CONF = OPS / "nginx/nginx.conf"


CERTS = Path("/opt/homebrew/etc/nginx/certs")


def nginx_test():
    return sh(["/opt/homebrew/bin/nginx", "-t", "-c", str(NGINX_CONF)])


def nginx(a):
    t = nginx_test()
    out = (t.stdout + t.stderr).strip()
    if t.returncode:
        print(out)
        sys.exit(1)
    if a.act == "test":
        print("OK: nginx -t lolos")
        return
    r = sh(["/opt/homebrew/bin/nginx", "-s", "reload"])
    if r.returncode:
        print((r.stdout + r.stderr).strip())
        sys.exit(1)
    print("OK: nginx -t lolos, reload berhasil")


def cert(name, aliases=()):
    if not re.fullmatch(r"[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?", name):
        die("nama situs tidak valid")
    tld = (OPS / "tld").read_text().strip()
    host = f"{name}.{tld}"
    if not (OPS / f"nginx/sites/{name}.conf").exists():
        die("situs tidak ada")
    pem, key = CERTS / f"{host}.pem", CERTS / f"{host}-key.pem"
    names = [host]
    for al in aliases:
        l, _, t = al.partition(".")
        if t != tld or not re.fullmatch(r"[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?", l):
            die(f"domain tambahan tidak valid: {al!r}")
        names.append(al)
    r = sh(["/opt/homebrew/bin/mkcert", "-cert-file", str(pem), "-key-file", str(key), *names])
    if r.returncode:
        die("mkcert gagal: " + (r.stderr or r.stdout)[-200:])
    os.chmod(key, 0o600)
    t = nginx_test()
    if t.returncode:
        die("nginx -t gagal setelah sertifikat baru: " + (t.stdout + t.stderr)[-200:])
    sh(["/opt/homebrew/bin/nginx", "-s", "reload"])
    print(f"OK: sertifikat {', '.join(names)} diperbarui, nginx reload")
