"""lab service / app: layanan launchd, katalog aplikasi opsional, pemasang 9Router dan Docker."""

import argparse
import json
import os
import shutil
import socket
import time
from pathlib import Path

from labcli.core import BREW, HOME, OPS, ROOT, UID, die, sh, stream
from labcli.site import SITES_DIR

LAUNCH = HOME / "Library/LaunchAgents"


NGINX_LOG_ROOT = Path("/opt/homebrew/var/log")


def S(id, label, group, kind, name, **kw):
    return dict(
        id=id,
        label=label,
        group=group,
        kind=kind,
        name=name,
        port=kw.get("port"),
        sock=kw.get("sock"),
        log=kw.get("log"),
        ctl=kw.get("ctl", True),
        optional=kw.get("optional", False),
        proc=kw.get("proc"),
        desc=kw.get("desc", ""),
    )


CATALOG = [
    S("nginx", "nginx", "Web server", "brew", "nginx", port=443, log=str(NGINX_LOG_ROOT / "nginx/error.log"), desc="Reverse proxy dan web server semua situs .lab"),
    S(
        "php-8.2",
        "PHP-FPM 8.2",
        "PHP",
        "brew",
        "php@8.2",
        sock="/opt/homebrew/var/run/php-fpm-dev-8.2.sock",
        log=str(NGINX_LOG_ROOT / "php-fpm.log"),
        desc="Pool dev (unix socket); log master FPM dipakai bersama semua versi PHP",
    ),
    S(
        "php-8.3",
        "PHP-FPM 8.3",
        "PHP",
        "brew",
        "php@8.3",
        sock="/opt/homebrew/var/run/php-fpm-dev-8.3.sock",
        log=str(NGINX_LOG_ROOT / "php-fpm.log"),
        desc="Pool dev (unix socket); log master FPM dipakai bersama semua versi PHP",
    ),
    S(
        "php-8.4",
        "PHP-FPM 8.4",
        "PHP",
        "brew",
        "php@8.4",
        sock="/opt/homebrew/var/run/php-fpm-dev-8.4.sock",
        log=str(NGINX_LOG_ROOT / "php-fpm.log"),
        desc="Pool dev (unix socket); log master FPM dipakai bersama semua versi PHP",
    ),
    S(
        "php-8.5",
        "PHP-FPM 8.5",
        "PHP",
        "brew",
        "php",
        sock="/opt/homebrew/var/run/php-fpm-dev-8.5.sock",
        log=str(NGINX_LOG_ROOT / "php-fpm.log"),
        desc="Pool dev (unix socket); log master FPM dipakai bersama semua versi PHP",
    ),
    S("postgresql", "PostgreSQL 17", "Database", "brew", "postgresql@17", port=5432, log=str(NGINX_LOG_ROOT / "postgresql@17.log"), desc="127.0.0.1:5432"),
    S("mariadb", "MariaDB 11.8", "Database", "launchd", "com.macminiops.mariadb", port=3306, log=str(HOME / "Library/Logs/mariadb.log"), desc="127.0.0.1:3306 (port bawaan MySQL/MariaDB)"),
    S("mysql", "MySQL 8.4", "Database", "brew", "mysql@8.4", port=3307, log="/opt/homebrew/var/mysql/*.err", desc="127.0.0.1:3307"),
    S("redis", "Redis", "Cache dan antrean", "brew", "redis", port=6379, log=str(NGINX_LOG_ROOT / "redis.log"), desc="127.0.0.1:6379"),
    S("memcached", "Memcached", "Cache dan antrean", "brew", "memcached", port=11211, optional=True, desc="Cache memori, opsional (port 11211)"),
    S(
        "docker",
        "Docker (Colima)",
        "Alat pengembangan",
        "launchd",
        "com.macminiops.colima",
        sock=str(HOME / ".colima/default/docker.sock"),
        optional=True,
        log=str(HOME / "Library/Logs/colima.log"),
        desc="Runtime Docker tanpa GUI (VM 2 CPU / 4 GB, hanya wwwroot dibagikan). Dikelola di menu Docker",
    ),
    S(
        "9router",
        "9Router",
        "Alat pengembangan",
        "launchd",
        "com.macminiops.9router",
        port=20128,
        optional=True,
        log=str(HOME / "Library/Logs/9router.log"),
        desc="Router/proxy model AI lokal (127.0.0.1:20128, lewat 9router.lab di tailnet). Password bawaan hanya bisa diganti dari Mac ini sendiri di http://127.0.0.1:20128",
    ),
    S(
        "httpd",
        "Apache",
        "Web server",
        "brew",
        "httpd",
        port=8080,
        optional=True,
        log=str(NGINX_LOG_ROOT / "httpd/error.log"),
        desc="Web server untuk situs yang butuh .htaccess. Hanya 127.0.0.1:8080 di belakang nginx",
    ),
    S(
        "cloudflared",
        "Cloudflare Tunnel",
        "Jaringan",
        "brew",
        "cloudflared",
        proc="cloudflared",
        optional=True,
        desc="Membuka satu situs ke internet sementara (tanpa akun/domain). Tidak punya proses sendiri: nyalakan per situs di tab Publik. Berhenti = semua tunnel dimatikan",
    ),
    S("mailpit", "Mailpit", "Alat pengembangan", "brew", "mailpit", port=8025, optional=True, desc="Penangkap email dev: SMTP 1025, UI 127.0.0.1:8025"),
    S("netdata", "Netdata", "Monitoring", "brew", "netdata", port=19999, log=str(NGINX_LOG_ROOT / "netdata/daemon.log"), desc="Metrik sistem real-time (stats.lab)"),
    S("uptime-kuma", "Uptime Kuma", "Monitoring", "launchd", "com.macminiops.uptime-kuma", port=3001, log=str(HOME / "Library/Logs/uptime-kuma.log"), desc="Pemantau uptime (kuma.lab)"),
    S("dnsmasq", "dnsmasq", "Jaringan", "brew", "dnsmasq", ctl=False, proc="dnsmasq", desc="DNS *.lab; dijalankan sebagai root, kendali lewat sudo di terminal"),
    S("panel", "Panel", "Sistem", "launchd", "com.macminiops.panel", port=9100, ctl=False, log=str(HOME / "Library/Logs/panel.log"), desc="Panel ini; tidak bisa dihentikan dari dirinya sendiri"),
]


BY_ID = {c["id"]: c for c in CATALOG}


def tcp_ok(port):
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.4):
            return True
    except OSError:
        return False


def unix_ok(path):
    try:
        c = socket.socket(socket.AF_UNIX)
        c.settimeout(0.4)
        c.connect(path)
        c.close()
        return True
    except OSError:
        return False


def brew_versions():
    r = sh([BREW, "list", "--versions"])
    return {l.split()[0]: l.split()[-1] for l in r.stdout.splitlines() if l.strip()}


def brew_status():
    r = sh([BREW, "services", "list", "--json"])
    try:
        return {x["name"]: x.get("status") for x in json.loads(r.stdout or "[]")}
    except Exception:
        return {}


def probe(c):
    if c["port"]:
        return tcp_ok(c["port"])
    if c["sock"]:
        return unix_ok(c["sock"])
    if c["proc"]:
        return sh(["/usr/bin/pgrep", "-x", c["proc"]]).returncode == 0
    return False


def launchd_loaded(label):
    return sh(["/bin/launchctl", "print", f"gui/{UID}/{label}"]).returncode == 0


def snapshot():
    # Satu panggilan `brew list` untuk semuanya; `brew services` (0,4 dtk) hanya dibaca bila ada layanan brew yang tidak merespons.
    vers, rows, bst = brew_versions(), [], None
    for c in CATALOG:
        if c["kind"] == "brew":
            installed, ver = c["name"] in vers, vers.get(c["name"], "")
        else:
            installed, ver = (LAUNCH / f"{c['name']}.plist").exists(), ""
            if installed and c["id"] == "mariadb":
                ver = vers.get("mariadb@11.8", "")
            if installed and c["id"] == "docker":
                ver = vers.get("docker", "")
            if installed and c["id"] == "9router":
                try:
                    ver = json.loads((NR_DIR / "node_modules/9router/package.json").read_text()).get("version", "")
                except (OSError, ValueError):
                    pass
        if not installed:
            state = "not_installed"
        elif probe(c):
            state = "running"
        elif c["kind"] == "brew" and (bst := bst if bst is not None else brew_status()).get(c["name"]) == "error":
            state = "error"
        else:
            state = "stopped"
        rows.append(
            dict(
                id=c["id"],
                label=c["label"],
                group=c["group"],
                kind=c["kind"],
                version=ver,
                state=state,
                port=c["port"],
                installed=installed,
                ctl=c["ctl"],
                optional=c["optional"],
                desc=c["desc"],
                has_log=bool(c["log"]),
            )
        )
    return rows


def service(a):
    if a.act == "list":
        rows = snapshot()
        if a.json:
            print(json.dumps(rows))
            return
        for r in rows:
            print(f"{r['id']:14}{r['state']:15}{r['version']:10}{r['label']}")
        return
    c = BY_ID.get(a.id)
    if not c:
        die(f"layanan '{a.id}' tidak dikenal")
    if a.act == "logs":
        if not c["log"]:
            die("layanan ini tidak punya log yang bisa ditampilkan")
        import glob

        fs = sorted(glob.glob(c["log"]), key=os.path.getmtime)
        if not fs:
            die("log belum ada")
        n = max(1, min(int(a.n), 500))
        print(f"== {fs[-1]}")
        print("\n".join(Path(fs[-1]).read_text(errors="replace").splitlines()[-n:]))
        return
    if not c["ctl"]:
        die(f"'{c['id']}' tidak bisa dikendalikan dari sini ({c['desc']})")
    if not snapshot_one(c)[0]:
        die(f"'{c['id']}' belum terpasang")
    if c["id"] == "cloudflared":
        from labcli import public

        if a.act == "start":
            print("OK: Cloudflare Tunnel siap. Tidak ada proses sendiri; nyalakan akses publik per situs (lab public on SITUS atau tab Publik).")
            return
        public.off(argparse.Namespace(all=True, name=None))
        return
    if c["kind"] == "brew":
        extra = []
        if c["name"] == "mailpit" and a.act in ("start", "restart"):
            extra = [f"--file={mailpit_plist()}"]
        if c["name"] == "httpd" and a.act in ("start", "restart"):
            extra = [f"--file={httpd_plist()}"]
        r = sh([BREW, "services", a.act, c["name"], *extra])
    else:
        plist = str(LAUNCH / f"{c['name']}.plist")
        tgt = f"gui/{UID}/{c['name']}"
        # Docker (Colima) tidak boleh menyala sendiri setelah login/reboot: VM 4 GB RAM. `disable` bertahan melewati reboot,
        # jadi start harus `enable` dulu dan stop `disable` lagi. Layanan launchd lain tidak diubah perilakunya.
        manual = c["id"] == "docker"
        if manual and a.act in ("start", "restart"):
            sh(["/bin/launchctl", "enable", tgt])
        if a.act == "restart":
            # bootout (SIGTERM, shutdown bersih) lalu bootstrap; kickstart -k memakai SIGKILL dan membahayakan database
            if launchd_loaded(c["name"]):
                sh(["/bin/launchctl", "bootout", tgt])
                for _ in range(180):
                    if not launchd_loaded(c["name"]):
                        break
                    time.sleep(0.5)
            r = sh(["/bin/launchctl", "bootstrap", f"gui/{UID}", plist])
        elif a.act == "stop":
            r = sh(["/bin/launchctl", "bootout", tgt])
            if manual:
                sh(["/bin/launchctl", "disable", tgt])
                # bootout hanya melepas job `colima start --foreground`; VM (limactl/Virtualization) bisa tetap hidup dan memakai 4 GB
                sh(["/opt/homebrew/bin/colima", "stop"], timeout=150)
        else:
            r = sh(["/bin/launchctl", "bootstrap", f"gui/{UID}", plist]) if not launchd_loaded(c["name"]) else sh(["/usr/bin/true"])
    if r.returncode and not (a.act == "stop" and "No such process" in (r.stderr or "")):  # stop pada layanan yang sudah mati = sudah tercapai
        die((r.stderr or r.stdout).strip()[-300:] or "gagal")
    want = a.act != "stop"
    for _ in range(600 if c["id"] == "docker" else 120 if c["id"] in ("9router", "mariadb") else 20):  # 9router menyiapkan runtime SQLite saat start pertama (±20 dtk)
        time.sleep(0.5)
        if probe(c) == want:
            break
    now = probe(c)
    if now != want:
        die(f"perintah {a.act} dikirim, tetapi {c['label']} {'belum berjalan' if want else 'masih berjalan'}")
    print(f"OK: {c['label']} {'berjalan' if now else 'berhenti'}")


def mailpit_plist():
    """Plist bawaan Homebrew membuka Mailpit di semua antarmuka (LAN, tanpa login). Pakai plist sendiri yang terikat ke loopback."""
    import plistlib

    f = HOME / ".config/macminiops/mailpit.plist"
    f.parent.mkdir(parents=True, exist_ok=True)
    f.write_bytes(
        plistlib.dumps(
            {
                "Label": "sh.brew.mailpit",
                "KeepAlive": True,
                "RunAtLoad": True,
                "ProgramArguments": ["/opt/homebrew/opt/mailpit/bin/mailpit", "--listen", "127.0.0.1:8025", "--smtp", "127.0.0.1:1025"],
                "StandardOutPath": "/opt/homebrew/var/log/mailpit.log",
                "StandardErrorPath": "/opt/homebrew/var/log/mailpit.log",
            }
        )
    )
    os.chmod(f, 0o600)
    return str(f)


def httpd_plist():
    """Plist bawaan Homebrew memakai config bawaan (Listen 8080 di semua antarmuka). Pakai config sendiri yang terikat ke loopback."""
    import plistlib

    from labcli import apache

    apache.write_conf()
    r = apache.check()
    if r.returncode:
        die("config Apache tidak valid: " + (r.stderr or r.stdout).strip()[-300:])
    f = HOME / ".config/macminiops/httpd.plist"
    f.parent.mkdir(parents=True, exist_ok=True)
    f.write_bytes(
        plistlib.dumps(
            {
                "Label": "sh.brew.httpd",
                "KeepAlive": True,
                "RunAtLoad": True,
                "ProgramArguments": [apache.HTTPD, "-f", str(apache.CONF), "-D", "FOREGROUND"],
                "StandardOutPath": "/opt/homebrew/var/log/httpd/launchd.log",
                "StandardErrorPath": "/opt/homebrew/var/log/httpd/launchd.log",
            }
        )
    )
    os.chmod(f, 0o600)
    return str(f)


def snapshot_one(c):
    if c["kind"] == "brew":
        return (c["name"] in brew_versions(),)
    return ((LAUNCH / f"{c['name']}.plist").exists(),)


NR_LABEL, NR_PORT = "com.macminiops.9router", 20128


NR_DIR = ROOT / "server/apps/9router"


NR_NODE = HOME / ".local/share/mise/shims/node"


NR_NPM = HOME / ".local/share/mise/shims/npm"


def nr_plist():
    cli = NR_DIR / "node_modules/9router/cli.js"
    # --host 127.0.0.1 WAJIB: bawaan 9router adalah 0.0.0.0 (terbuka ke semua jaringan). HOSTNAME ikut dipaksa karena server Next membacanya.
    return {
        "Label": NR_LABEL,
        "RunAtLoad": True,
        "KeepAlive": True,
        "ThrottleInterval": 30,
        "ProcessType": "Background",
        "Umask": 63,
        "ProgramArguments": [str(NR_NODE), str(cli), "--host", "127.0.0.1", "--port", str(NR_PORT), "--no-browser", "--skip-update", "--log"],
        "WorkingDirectory": str(NR_DIR),
        "EnvironmentVariables": {"HOME": str(HOME), "PATH": f"{HOME}/.local/share/mise/shims:/opt/homebrew/bin:/usr/bin:/bin", "HOSTNAME": "127.0.0.1", "PORT": str(NR_PORT), "NODE_ENV": "production"},
        "StandardOutPath": str(HOME / "Library/Logs/9router.log"),
        "StandardErrorPath": str(HOME / "Library/Logs/9router.log"),
    }


def nr_install():
    import plistlib

    plist = LAUNCH / f"{NR_LABEL}.plist"
    if plist.exists() and (NR_DIR / "node_modules/9router/cli.js").exists():
        print("sudah terpasang: 9Router")
        return
    if not NR_NODE.exists() or not NR_NPM.exists():
        die("node/npm (mise) tidak ditemukan; pasang Node lewat 'lab runtime' dulu")
    NR_DIR.mkdir(parents=True, exist_ok=True)
    print("[1/4] npm install 9router (tanpa skrip instalasi)")
    if stream([str(NR_NPM), "install", "--prefix", str(NR_DIR), "9router", "--ignore-scripts", "--no-audit", "--no-fund"]):
        die("npm install 9router gagal")
    if not (NR_DIR / "node_modules/9router/cli.js").exists():
        die("paket 9router tidak lengkap setelah npm install")
    print("[2/4] folder data ~/.9router (izin 0700: berisi token akun AI)")
    d = HOME / ".9router"
    d.mkdir(exist_ok=True)
    d.chmod(0o700)
    print("[3/4] LaunchAgent (bind 127.0.0.1:20128)")
    LAUNCH.mkdir(parents=True, exist_ok=True)
    plist.write_bytes(plistlib.dumps(nr_plist()))
    plist.chmod(0o644)
    print("[4/4] situs https://9router.lab (khusus tailnet)")
    if not (SITES_DIR / "9router.conf").exists() and stream([str(OPS / "scripts/addsite.sh"), "9router", str(NR_PORT)]):
        die("pembuatan situs 9router.lab gagal")
    print(
        "OK: 9Router terpasang. Jalankan dari halaman Aplikasi. Login jarak jauh DITOLAK sampai password bawaan diganti: ganti dari Mac ini di http://127.0.0.1:20128 (dari laptop: ssh -L 20128:127.0.0.1:20128 <mac> lalu buka http://localhost:20128), baru pakai https://9router.lab."
    )


def nr_remove():
    plist = LAUNCH / f"{NR_LABEL}.plist"
    sh(["/bin/launchctl", "bootout", f"gui/{UID}/{NR_LABEL}"])
    plist.unlink(missing_ok=True)
    if (SITES_DIR / "9router.conf").exists() or (SITES_DIR / "9router.conf.disabled").exists():
        (SITES_DIR / "9router.conf.disabled").unlink(missing_ok=True)
        stream([str(OPS / "scripts/addsite.sh"), "--remove", "9router"])
    if NR_DIR.exists() and NR_DIR.resolve() == (ROOT / "server/apps/9router").resolve() and not NR_DIR.is_symlink():
        shutil.rmtree(NR_DIR)
    print("OK: 9Router dihapus. Data akun di ~/.9router TIDAK dihapus (hapus sendiri bila tidak dipakai lagi).")


DK_LABEL = "com.macminiops.colima"


DK_SOCK = HOME / ".colima/default/docker.sock"


DK_BREW = ("colima", "docker", "docker-compose")


def dk_plist():
    wwwroot = ROOT / "wwwroot"
    # VM dibatasi 2 CPU / 4 GB; hanya wwwroot yang dibagikan (bukan seluruh home); runtime docker.
    return {
        "Label": DK_LABEL,
        "RunAtLoad": True,
        "KeepAlive": True,
        "ThrottleInterval": 30,
        "ProcessType": "Background",
        "ProgramArguments": [
            "/opt/homebrew/bin/colima",
            "start",
            "--foreground",
            "--runtime",
            "docker",
            "--vm-type",
            "vz",
            "--cpu",
            "2",
            "--memory",
            "4",
            "--disk",
            "40",
            "--mount-type",
            "virtiofs",
            "--mount",
            f"{wwwroot}:w",
        ],
        "EnvironmentVariables": {"HOME": str(HOME), "PATH": "/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"},
        "StandardOutPath": str(HOME / "Library/Logs/colima.log"),
        "StandardErrorPath": str(HOME / "Library/Logs/colima.log"),
    }


def dk_install():
    import plistlib

    plist = LAUNCH / f"{DK_LABEL}.plist"
    have = brew_versions()
    missing = [f for f in DK_BREW if f not in have]
    if not missing and plist.exists():
        print("sudah terpasang: Docker (Colima)")
        return
    print(f"[1/3] brew install {' '.join(missing) or '(sudah ada)'}")
    if missing and stream([BREW, "install", *missing]):
        die("brew install gagal")
    print("[2/3] plugin compose untuk docker CLI")
    cfg = HOME / ".docker/config.json"
    cfg.parent.mkdir(exist_ok=True)
    try:
        d = json.loads(cfg.read_text()) if cfg.exists() else {}
    except ValueError:
        d = {}
    dirs = d.setdefault("cliPluginsExtraDirs", [])
    if "/opt/homebrew/lib/docker/cli-plugins" not in dirs:
        if cfg.exists():
            shutil.copy2(cfg, f"{cfg}.bak.{time.strftime('%Y%m%d-%H%M%S')}")
        dirs.append("/opt/homebrew/lib/docker/cli-plugins")
        cfg.write_text(json.dumps(d, indent=2))
        cfg.chmod(0o600)
    print("[3/3] LaunchAgent colima (VM 2 CPU / 4 GB, hanya wwwroot dibagikan)")
    LAUNCH.mkdir(parents=True, exist_ok=True)
    plist.write_bytes(plistlib.dumps(dk_plist()))
    plist.chmod(0o644)
    print("OK: Docker terpasang. Jalankan dari halaman Aplikasi atau menu Docker; start pertama mengunduh image VM (beberapa menit).")


def dk_remove():
    plist = LAUNCH / f"{DK_LABEL}.plist"
    sh(["/bin/launchctl", "bootout", f"gui/{UID}/{DK_LABEL}"])
    if Path("/opt/homebrew/bin/colima").exists():
        sh(["/opt/homebrew/bin/colima", "stop"])
        sh(["/opt/homebrew/bin/colima", "delete", "--force"])
    plist.unlink(missing_ok=True)
    have = brew_versions()
    present = [f for f in DK_BREW if f in have]
    if present and stream([BREW, "uninstall", *present]):
        die("brew uninstall gagal")
    print("OK: Docker (Colima) dihapus, termasuk VM dan semua container/image di dalamnya.")


def app(a):
    opt = {c["id"]: c for c in CATALOG if c["optional"]}
    if a.act == "list":
        for r in snapshot():
            if r["optional"]:
                print(f"{r['id']:12}{'terpasang' if r['installed'] else 'belum':10}{r['version']}")
        return
    c = opt.get(a.id)
    if not c:
        die(f"aplikasi '{a.id}' tidak ada di katalog (opsional: {', '.join(opt)})")
    if c["id"] == "9router":
        nr_install() if a.act == "install" else nr_remove()
        return
    if c["id"] == "docker":
        dk_install() if a.act == "install" else dk_remove()
        return
    installed = c["name"] in brew_versions()
    if a.act == "install":
        if installed:
            print(f"sudah terpasang: {c['label']}")
            return
        if stream([BREW, "install", c["name"]]):
            die("brew install gagal")
        print(f"OK: {c['label']} terpasang. Jalankan dari halaman Aplikasi.")
    else:
        if not installed:
            print(f"belum terpasang: {c['label']}")
            return
        if c["id"] == "cloudflared":
            from labcli import public

            public.off(argparse.Namespace(all=True, name=None))
        else:
            sh([BREW, "services", "stop", c["name"]])
        if stream([BREW, "uninstall", c["name"]]):
            die("brew uninstall gagal")
        print(f"OK: {c['label']} dihapus")
