"""lab apache: Apache httpd opsional untuk situs yang butuh .htaccess. Selalu di 127.0.0.1:8080 di belakang nginx (TLS dan batas tailnet tetap di nginx)."""

import os
import re
import subprocess
from pathlib import Path

from labcli.core import ROOT, die

HTTPD = "/opt/homebrew/opt/httpd/bin/httpd"
BREW_PREFIX = "/opt/homebrew"
PORT = 8080
APACHE_DIR = ROOT / "server/apache"
SITES = APACHE_DIR / "sites"
CONF = APACHE_DIR / "httpd.conf"
NAME_RE = re.compile(r"^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$")
HOST_RE = re.compile(r"^[a-z0-9]([a-z0-9.-]{0,60}[a-z0-9])?$")
PHP_RE = re.compile(r"^8\.[2-5]$")

MODULES = [
    "mpm_event", "authz_core", "mime", "log_config", "setenvif", "headers", "unixd", "dir", "alias",
    "rewrite", "deflate", "expires", "proxy", "proxy_fcgi",
]  # fmt: skip

TEMPLATE = """# Dibangkitkan oleh `lab apache config`. Jangan diedit tangan: perubahan ditimpa.
# Hanya loopback: akses luar selalu lewat nginx (HTTPS + pembatasan tailnet).
ServerRoot "{prefix}/opt/httpd"
ServerName localhost
Listen 127.0.0.1:{port}
PidFile "{prefix}/var/run/httpd/httpd.pid"
ServerTokens Prod
ServerSignature Off
TraceEnable Off
Timeout 60
KeepAlive On

{modules}

ErrorLog "{prefix}/var/log/httpd/error.log"
LogLevel warn
LogFormat "%{{X-Forwarded-For}}i %h %l %u %t \\"%r\\" %>s %b \\"%{{Referer}}i\\" \\"%{{User-Agent}}i\\"" combined
CustomLog "{prefix}/var/log/httpd/access.log" combined
TypesConfig {prefix}/etc/httpd/mime.types

# Bawaan menolak semuanya; tiap situs membuka direktorinya sendiri.
<Directory />
  AllowOverride None
  Require all denied
</Directory>
<FilesMatch "^\\.ht">
  Require all denied
</FilesMatch>
DirectoryIndex index.php index.html
# nginx memegang TLS: teruskan skema agar aplikasi PHP tahu ini HTTPS.
SetEnvIf X-Forwarded-Proto https HTTPS=on

# Host tak dikenal -> 403 (vhost pertama menjadi bawaan)
<VirtualHost 127.0.0.1:{port}>
  ServerName default.invalid
  <Location />
    Require all denied
  </Location>
</VirtualHost>

IncludeOptional "{sites}/*.conf"
"""


def conf_text():
    mods = "\n".join(f"LoadModule {m}_module lib/httpd/modules/mod_{m}.so" for m in MODULES)
    return TEMPLATE.format(prefix=BREW_PREFIX, port=PORT, modules=mods, sites=SITES)


def vhost_text(host, directory, php=None):
    """Satu vhost per situs. AllowOverride All = .htaccess berlaku (alasan Apache dipakai)."""
    if not HOST_RE.fullmatch(host):
        die("host tidak valid")
    d = str(directory)
    if not d.startswith("/") or any(c in d for c in '"\n\r\0 ') or ".." in Path(d).parts:
        die("direktori tidak valid (harus path absolut tanpa spasi atau tanda kutip)")
    if php is not None and not PHP_RE.fullmatch(php):
        die("versi PHP harus 8.2 sampai 8.5")
    fpm = ""
    if php:
        fpm = f'  <FilesMatch "\\.php$">\n    SetHandler "proxy:unix:{BREW_PREFIX}/var/run/php-fpm-dev-{php}.sock|fcgi://localhost"\n  </FilesMatch>\n'
    return (
        f'<VirtualHost 127.0.0.1:{PORT}>\n  ServerName {host}\n  DocumentRoot "{d}"\n'
        f'  <Directory "{d}">\n    Options -Indexes +FollowSymLinks\n    AllowOverride All\n    Require all granted\n  </Directory>\n{fpm}</VirtualHost>\n'
    )


def write_conf():
    for d in (APACHE_DIR, SITES, Path(f"{BREW_PREFIX}/var/log/httpd"), Path(f"{BREW_PREFIX}/var/run/httpd")):
        d.mkdir(parents=True, exist_ok=True)
    CONF.write_text(conf_text())
    os.chmod(CONF, 0o644)


def check():
    return subprocess.run([HTTPD, "-t", "-f", str(CONF)], capture_output=True, text=True)


def running():
    pid = Path(f"{BREW_PREFIX}/var/run/httpd/httpd.pid")
    try:
        os.kill(int(pid.read_text()), 0)
        return True
    except (OSError, ValueError):
        return False


def graceful():
    """Muat ulang tanpa memutus koneksi bila Apache sedang jalan; tidak jalan = tidak apa-apa (vhost dibaca saat start)."""
    if running():
        subprocess.run([HTTPD, "-f", str(CONF), "-k", "graceful"], capture_output=True, text=True)


def site_add(name, host, directory, php=None):
    if not NAME_RE.fullmatch(name or ""):
        die("nama situs tidak valid")
    write_conf()
    f = SITES / f"{name}.conf"
    old = f.read_text() if f.exists() else None
    f.write_text(vhost_text(host, directory, php))
    r = check()
    if r.returncode:
        f.unlink() if old is None else f.write_text(old)
        die("config Apache tidak valid, dibatalkan: " + (r.stderr or r.stdout).strip()[-300:])
    graceful()


def site_remove(name):
    if not NAME_RE.fullmatch(name or ""):
        die("nama situs tidak valid")
    f = SITES / f"{name}.conf"
    if f.exists():
        f.unlink()
        graceful()


def apache(a):
    if a.act == "config":
        write_conf()
        r = check()
        print((r.stderr or r.stdout).strip())
        raise SystemExit(r.returncode)
    if a.act == "site-add":
        site_add(a.name, a.host, a.dir, a.php)
        print(f"OK: vhost Apache {a.host}")
    elif a.act == "site-remove":
        site_remove(a.name)
        print(f"OK: vhost Apache {a.name} dihapus")
    elif a.act == "sites":
        for f in sorted(SITES.glob("*.conf")):
            print(f.stem)
