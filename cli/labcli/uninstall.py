"""lab uninstall: lepas MacPanel dan (bila diminta) seluruh aplikasi bundelnya. Dijalankan USER di terminal, bukan dari panel.

Tiga tingkat, masing-masing mencakup tingkat sebelumnya:
  1  panel dan integrasinya (LaunchAgent panel/proxy/backup/logrotate, firewall pf buatan `lab firewall`, kunci Keychain, tautan nginx.conf)
  2  + aplikasi bundel dihentikan dan dilepas dari login (nginx, PHP-FPM, database, Redis, Mailpit, Netdata, dnsmasq, Colima, 9router, Kuma,
       kunci pf RustDesk). Paket dan DATA tetap ada.
  3  + backup akhir ke luar ~/MacPanel yang DIVERIFIKASI, lalu paket brew bundel dihapus, data database dan situs dihapus, CA mkcert dilepas,
       ~/MacPanel dan ~/.config/macminiops dihapus.

Tanpa --apply hanya mencetak rencana (aman, tidak mengubah apa pun). Langkah root memakai `sudo` per langkah (argumen tetap, path absolut,
tanpa shell dari string); password diketik di terminal ini.

Tidak pernah disentuh: Tailscale (termasuk /etc/resolver/*, milik tailscaled), aplikasi RustDesk, Hermes (~/.hermes dan LaunchAgent
hermes-dashboard), Homebrew sendiri, toolchain (mise, go, node, mkcert biner, sqlite, composer), dan data pribadi di luar daftar di bawah.
"""

import hashlib
import json
import os
import shutil
import socket
import subprocess
import sys
import tarfile
import time
from pathlib import Path
from typing import Any

BREW = "/opt/homebrew"
PANEL_AGENTS = ["com.macminiops.panel", "com.macminiops.regproxy", "com.macminiops.backup", "com.macminiops.logrotate"]
APP_AGENTS = ["com.macminiops.9router", "com.macminiops.uptime-kuma", "com.macminiops.mariadb", "com.macminiops.colima"]
KEEP_AGENTS = ["com.macminiops.hermes-dashboard"]
USER_SERVICES = ["nginx", "php", "php@8.2", "php@8.3", "php@8.4", "php@8.5", "mysql@8.4", "mariadb@11.8", "postgresql@17", "redis", "mailpit", "netdata", "memcached"]
ROOT_SERVICES = ["dnsmasq"]
FORMULAS = [
    "nginx",
    "php",
    "php@8.2",
    "php@8.3",
    "php@8.4",
    "php@8.5",
    "mysql@8.4",
    "mariadb@11.8",
    "postgresql@17",
    "redis",
    "mailpit",
    "netdata",
    "memcached",
    "dnsmasq",
    "macmon",
    "colima",
    "docker",
    "docker-compose",
    "lima",
]
DATA_DIRS = ["mysql", "mariadb", "postgresql@17", "redis", "netdata", "lib/netdata", "cache/netdata", "log/nginx", "run/nginx"]
LOG_FILES = {1: ["panel.log", "regproxy.log", "macminiops-backup.log", "macminiops-logrotate.log"], 2: ["9router.log", "colima.log", "mariadb.log", "uptime-kuma.log"]}
UNTOUCHED = [
    "Tailscale dan /etc/resolver/* (dikelola tailscaled)",
    "aplikasi RustDesk (hanya kunci firewall pf buatan kita yang dilepas, di tingkat 2)",
    "Hermes: ~/.hermes dan LaunchAgent com.macminiops.hermes-dashboard",
    "Homebrew, mise, go, node, pnpm, composer, mkcert (biner), sqlite",
    "file pribadi di luar daftar rencana",
]


class UninstallError(Exception):
    pass


def hostname():
    return socket.gethostname().split(".")[0]


class Env:
    """Keadaan mesin yang relevan. Dibuat dari mesin nyata (detect) atau dirakit tes."""

    def __init__(
        self, home, mp, uid, brew=BREW, agents=(), formulas=(), services=(), pf_block=False, pf_rustdesk=False, keychain=True, nginx_link=False, cli_links=(), mkcert_caroot=None, root_base="/"
    ):
        self.home, self.mp, self.uid, self.brew = Path(home), Path(mp), uid, Path(brew)
        self.agents, self.formulas, self.services = set(agents), set(formulas), set(services)
        self.pf_block, self.pf_rustdesk, self.keychain, self.nginx_link = pf_block, pf_rustdesk, keychain, nginx_link
        self.cli_links = list(cli_links)
        self.mkcert_caroot = Path(mkcert_caroot) if mkcert_caroot else None
        self.root_base = Path(root_base)


def detect(home=None, mp=None):
    home = Path(home or Path.home())
    mp = Path(mp or os.environ.get("MACPANEL_HOME") or home / "MacPanel")
    la = home / "Library/LaunchAgents"
    agents = {p.stem for p in la.glob("com.macminiops.*.plist")} if la.is_dir() else set()
    brew = Path(BREW)
    fm = set()
    cel = brew / "Cellar"
    if cel.is_dir():
        fm = {p.name for p in cel.iterdir()}
    services = set()
    for pl in la.glob("sh.brew.*.plist") if la.is_dir() else []:
        services.add(pl.stem[len("sh.brew.") :])
    if (Path("/Library/LaunchDaemons") / "sh.brew.dnsmasq.plist").exists():
        services.add("dnsmasq")
    ngx = brew / "etc/nginx/nginx.conf"
    nginx_link = ngx.is_symlink() and str(mp) in os.readlink(ngx)
    links = [p for p in (brew / "bin/lab", brew / "bin/panelctl") if p.is_symlink() and str(mp) in os.readlink(p)]
    caroot = None
    try:
        r = subprocess.run([str(brew / "bin/mkcert"), "-CAROOT"], capture_output=True, text=True, timeout=10)
        if r.returncode == 0 and r.stdout.strip():
            caroot = r.stdout.strip()
    except (OSError, subprocess.TimeoutExpired):
        pass
    kc = subprocess.run(["/usr/bin/security", "find-generic-password", "-a", "panel", "-s", "panel-totp-key"], capture_output=True).returncode == 0
    return Env(
        home, mp, os.getuid(), brew, agents, fm, services, Path("/etc/pf.anchors/macminiops-block").exists(), Path("/etc/pf.anchors/macminiops-rustdesk").exists(), kc, nginx_link, links, caroot
    )


# ---------------------------------------------------------------- rencana


def step(level, phase, op, desc, **kw) -> dict[str, Any]:
    d: dict[str, Any] = {"level": level, "phase": phase, "op": op, "desc": desc, "tolerate": False}
    d.update(kw)
    return d


def allowed_roots(env):
    h = env.home
    return [
        env.mp,
        h / ".config/macminiops",
        h / ".cache/macpanel",
        h / "Library/LaunchAgents",
        h / "Library/Logs",
        env.brew / "var",
        env.brew / "bin",
        env.brew / "etc/nginx/nginx.conf",
        Path(env.mkcert_caroot) if env.mkcert_caroot else None,
    ]


def guard_path(env, path):
    """Hanya path di dalam akar yang diizinkan (dan bukan akar itu sendiri kecuali ~/MacPanel dan folder konfigurasi kita)."""
    p = Path(os.path.abspath(path))
    if p in (env.home, Path("/"), env.home / "Library", env.brew, env.brew / "var", env.brew / "bin", env.home / "Library/LaunchAgents", env.home / "Library/Logs"):
        raise UninstallError(f"menolak menghapus {p}")
    for root in allowed_roots(env):
        if root is None:
            continue
        r = Path(os.path.abspath(str(root)))
        if p == r or r in p.parents:
            return p
    raise UninstallError(f"path di luar daftar yang boleh dihapus: {p}")


def build_plan(env, level, backup_dir=None):
    """Daftar langkah berurutan. Murni: tidak membaca atau mengubah mesin."""
    if level not in (1, 2, 3):
        raise UninstallError("tingkat harus 1, 2, atau 3")
    if level == 3:
        if not backup_dir:
            raise UninstallError("tingkat 3 wajib --backup-dir (lokasi di luar ~/MacPanel)")
        bd = Path(os.path.abspath(backup_dir))
        if bd == env.mp or env.mp in bd.parents or bd == env.home / ".config/macminiops" or (env.home / ".config/macminiops") in bd.parents:
            raise UninstallError("--backup-dir tidak boleh berada di dalam ~/MacPanel atau ~/.config/macminiops (ikut terhapus)")
    P = []
    gl = f"gui/{env.uid}"
    la = env.home / "Library/LaunchAgents"

    def agent_steps(labels, lvl, phase):
        for lab in labels:
            if lab in env.agents:
                P.append(step(lvl, phase, "exec", f"hentikan LaunchAgent {lab}", argv=["/bin/launchctl", "bootout", f"{gl}/{lab}"], tolerate=True))
                P.append(step(lvl, phase, "rm", f"hapus {lab}.plist", path=str(la / f"{lab}.plist")))

    # tingkat 1
    agent_steps(PANEL_AGENTS, 1, "L1 panel")
    if env.pf_block:
        P.append(step(1, "L1 firewall", "sudo", "lepas daemon firewall lab firewall", argv=["/bin/launchctl", "bootout", "system/com.macminiops.pf-block"], tolerate=True))
        P.append(
            step(
                1,
                "L1 firewall",
                "sudo",
                "keluarkan anchor macminiops-block dari /etc/pf.macminiops.conf (cadangan .bak.uninstall)",
                argv=["/usr/bin/sed", "-i", ".bak.uninstall", "/macminiops-block/d", "/etc/pf.macminiops.conf"],
            )
        )
        # Bila kunci RustDesk masih terpasang, muat konfigurasi kita (tanpa anchor blok) agar RustDesk tidak terbuka ke LAN; bila tidak, kembali ke bawaan.
        conf = "/etc/pf.macminiops.conf" if env.pf_rustdesk else "/etc/pf.conf"
        P.append(step(1, "L1 firewall", "sudo", f"muat ulang aturan pf dari {conf}", argv=["/sbin/pfctl", "-f", conf]))
        for f in ("/Library/LaunchDaemons/com.macminiops.pf-block.plist", "/etc/pf.anchors/macminiops-block", "/etc/macminiops-pf.ports", "/usr/local/libexec/macminiops-pf-block"):
            P.append(step(1, "L1 firewall", "sudo", f"hapus {f}", argv=["/bin/rm", "-f", f]))
    if env.keychain:
        P.append(
            step(
                1,
                "L1 panel",
                "exec",
                "hapus kunci enkripsi panel dari Keychain (rahasia TOTP tidak bisa dibaca lagi)",
                argv=["/usr/bin/security", "delete-generic-password", "-a", "panel", "-s", "panel-totp-key"],
                tolerate=True,
            )
        )
    if env.nginx_link:
        P.append(step(1, "L1 panel", "rm", "lepas tautan nginx.conf ke ~/MacPanel", path=str(env.brew / "etc/nginx/nginx.conf")))
        P.append(
            step(1, "L1 panel", "restore_nginx", "pulihkan nginx.conf bawaan Homebrew (bila ada)", default=str(env.brew / "etc/nginx/nginx.conf.default"), dest=str(env.brew / "etc/nginx/nginx.conf"))
        )
    P.append(step(1, "L1 panel", "rm", "hapus cache proxy npm", path=str(env.home / ".cache/macpanel"), missing_ok=True))
    for n in LOG_FILES[1]:
        P.append(step(1, "L1 panel", "rm", f"hapus log {n}", path=str(env.home / "Library/Logs" / n), missing_ok=True))
    if level == 1:
        return _sorted(P)

    # tingkat 2
    P.append(step(2, "L2 aplikasi", "exec", "hentikan Docker/Colima (VM)", argv=[str(env.brew / "bin/colima"), "stop"], tolerate=True))
    agent_steps(APP_AGENTS, 2, "L2 aplikasi")
    for s in USER_SERVICES:
        if s in env.services:
            P.append(step(2, "L2 aplikasi", "exec", f"hentikan dan lepas layanan brew {s}", argv=[str(env.brew / "bin/brew"), "services", "stop", s], tolerate=True))
    for s in ROOT_SERVICES:
        if s in env.services:
            P.append(step(2, "L2 aplikasi", "sudo", f"hentikan dan lepas layanan root {s}", argv=[str(env.brew / "bin/brew"), "services", "stop", s], tolerate=True))
    if env.pf_rustdesk:
        s = str(env.mp / "server/scripts/rustdesk-lock.sh")
        P.append(step(2, "L2 firewall", "exec", "lepas kunci firewall RustDesk (skrip memanggil sudo sendiri)", argv=["/bin/bash", s, "--remove"], tolerate=True))
    for n in LOG_FILES[2]:
        P.append(step(2, "L2 aplikasi", "rm", f"hapus log {n}", path=str(env.home / "Library/Logs" / n), missing_ok=True))
    if level == 2:
        return _sorted(P)

    # tingkat 3
    bd = Path(os.path.abspath(backup_dir))
    P.append(step(3, "L3 backup", "backup", f"backup akhir ke {bd} (backup DB, situs tanpa node_modules, backup lama, konfigurasi, catatan) lalu verifikasi", dest=str(bd)))
    for f in [x for x in FORMULAS if x in env.formulas]:
        P.append(step(3, "L3 hapus", "exec", f"brew uninstall {f}", argv=[str(env.brew / "bin/brew"), "uninstall", "--formula", f], tolerate=True))
    for d in DATA_DIRS:
        P.append(step(3, "L3 hapus", "rm", f"hapus data {d}", path=str(env.brew / "var" / d), missing_ok=True))
    if env.mkcert_caroot:
        P.append(step(3, "L3 hapus", "exec", "lepas CA mkcert dari Keychain (macOS meminta otorisasi)", argv=[str(env.brew / "bin/mkcert"), "-uninstall"], tolerate=True))
        P.append(step(3, "L3 hapus", "rm", "hapus berkas CA mkcert", path=str(env.mkcert_caroot), missing_ok=True))
    P.append(step(3, "L3 hapus", "rm", "hapus pengaturan dan kredensial ~/.config/macminiops", path=str(env.home / ".config/macminiops"), missing_ok=True))
    P.append(step(3, "L3 akhir", "rm", "hapus ~/MacPanel (situs, backup lama, dokumentasi, kode panel)", path=str(env.mp)))
    for l in env.cli_links:
        P.append(step(3, "L3 akhir", "rm", f"hapus tautan {l.name}", path=str(l)))
    return _sorted(P)


_ORDER = {"L1 panel": 10, "L1 firewall": 11, "L2 aplikasi": 20, "L2 firewall": 21, "L3 backup": 30, "L3 hapus": 31, "L3 akhir": 99}


def _sorted(plan):
    """Urut menurut fase; di dalam fase urutan penyusunan dipertahankan (sort stabil)."""
    return sorted(plan, key=lambda s: _ORDER[s["phase"]])


# ---------------------------------------------------------------- backup


def _walk_size(root, skip=("node_modules",)):
    total = 0
    for dp, dn, fn in os.walk(root):
        dn[:] = [d for d in dn if d not in skip]
        for f in fn:
            try:
                total += os.lstat(os.path.join(dp, f)).st_size
            except OSError:
                pass
    return total


def _sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""):
            h.update(b)
    return h.hexdigest()


def _tar(src, out, skip=("node_modules",)):
    def flt(ti):
        return None if any(part in skip for part in Path(ti.name).parts) else ti

    with tarfile.open(out, "w:gz") as t:
        t.add(src, arcname=Path(src).name, filter=flt)


def make_backup(env, dest, run_lab_backup, now=time.time):
    """Backup akhir. Mengembalikan folder backup. Dibatalkan (UninstallError) bila ruang kurang atau verifikasi gagal."""
    dest = Path(dest)
    dest.mkdir(parents=True, exist_ok=True)
    os.chmod(dest, 0o700)
    need = _walk_size(env.mp / "wwwroot") + _walk_size(env.mp / "backup", ()) + _walk_size(env.home / ".config/macminiops", ())
    free = shutil.disk_usage(dest).free
    if free < need * 1.2 + (200 << 20):
        raise UninstallError(f"ruang kosong di {dest} kurang: perlu ±{need * 1.2 / 1e9:.1f} GB, ada {free / 1e9:.1f} GB")
    run_lab_backup()
    bk = env.mp / "backup"
    fresh = [p for p in bk.iterdir() if now() - p.stat().st_mtime < 1800] if bk.is_dir() else []
    if not fresh:
        raise UninstallError("backup baru (dump database) tidak ditemukan setelah `lab backup`; tidak ada yang dihapus")
    out = dest / f"macpanel-final-{time.strftime('%Y%m%d-%H%M%S', time.localtime(now()))}"
    out.mkdir(mode=0o700)
    arts = []
    for name, src in (("wwwroot", env.mp / "wwwroot"), ("config", env.home / ".config/macminiops"), ("notes", env.mp / "notes"), ("docs", env.mp / "docs"), ("server", env.mp / "server")):
        if src.exists():
            o = out / f"{name}.tar.gz"
            _tar(src, o, skip=("node_modules",) if name in ("wwwroot", "server") else ())
            arts.append(o)
    if bk.is_dir():
        shutil.copytree(bk, out / "backup", symlinks=True)
    manifest = {}
    for p in sorted(out.rglob("*")):
        if p.is_file():
            manifest[str(p.relative_to(out))] = _sha256(p)
    (out / "MANIFEST.json").write_text(json.dumps({"dibuat": time.strftime("%Y-%m-%d %H:%M:%S"), "files": manifest}, indent=1))
    for p in arts:
        os.chmod(p, 0o600)
    verify_backup(out)
    return out


def verify_backup(out):
    """Baca ulang: hash cocok dengan manifest, dan setiap arsip bisa dibaca sampai habis. Gagal = UninstallError."""
    out = Path(out)
    try:
        m = json.loads((out / "MANIFEST.json").read_text())["files"]
    except (OSError, ValueError, KeyError) as e:
        raise UninstallError(f"manifest backup tidak terbaca: {e}")
    if not m:
        raise UninstallError("backup kosong")
    for rel, h in m.items():
        p = out / rel
        if not p.is_file() or _sha256(p) != h:
            raise UninstallError(f"verifikasi backup gagal: {rel}")
        if rel.endswith(".tar.gz"):
            try:
                with tarfile.open(p) as t:
                    n = 0
                    for ti in t:
                        n += 1
                        if ti.isreg():
                            f = t.extractfile(ti)
                            while f and f.read(1 << 20):
                                pass
                    if n == 0:
                        raise UninstallError(f"arsip kosong: {rel}")
            except (tarfile.TarError, EOFError, OSError) as e:
                raise UninstallError(f"arsip rusak {rel}: {e}")
    return len(m)


# ---------------------------------------------------------------- eksekusi


class Runner:
    def run(self, argv, sudo=False):
        full = (["/usr/bin/sudo"] + argv) if sudo else argv
        return subprocess.run(full).returncode


def execute(env, plan, runner=None, out=sys.stdout, lab_backup=None):
    runner = runner or Runner()
    os.chdir(env.home)  # jangan berdiri di dalam folder yang akan dihapus
    n = len(plan)
    for i, s in enumerate(plan, 1):
        print(f"[{i}/{n}] {s['desc']}", file=out)
        op = s["op"]
        try:
            if op in ("exec", "sudo"):
                rc = runner.run(s["argv"], sudo=(op == "sudo"))
                if rc and not s["tolerate"]:
                    raise UninstallError(f"kode keluar {rc}")
                if rc:
                    print(f"      (dilewati: kode {rc})", file=out)
            elif op == "rm":
                p = guard_path(env, s["path"])
                if not os.path.lexists(p):
                    if not s.get("missing_ok", False):
                        print("      (sudah tidak ada)", file=out)
                    continue
                if p.is_symlink() or p.is_file():
                    p.unlink()
                else:
                    shutil.rmtree(p)
            elif op == "restore_nginx":
                d, t = Path(s["default"]), Path(s["dest"])
                if d.exists() and not os.path.lexists(t):
                    shutil.copy2(d, t)
            elif op == "backup":
                lb = lab_backup or (lambda: runner.run([str(env.mp / "server/scripts/lab"), "backup"]) == 0 or (_ for _ in ()).throw(UninstallError("lab backup gagal")))
                outdir = make_backup(env, s["dest"], lb)
                print(f"      backup terverifikasi: {outdir}", file=out)
            else:
                raise UninstallError(f"op tidak dikenal: {op}")
        except UninstallError as e:
            print(f"GAGAL pada langkah {i}: {e}. Berhenti; langkah berikutnya tidak dijalankan.", file=out)
            return 1
        except OSError as e:
            print(f"GAGAL pada langkah {i}: {e}. Berhenti.", file=out)
            return 1
    return 0


def print_plan(plan, level, out=sys.stdout):
    names = {1: "panel dan integrasinya", 2: "panel + aplikasi bundel dihentikan (paket dan data tetap)", 3: "SEMUA: paket, data, dan ~/MacPanel dihapus setelah backup terverifikasi"}
    print(f"Rencana uninstall tingkat {level}: {names[level]}", file=out)
    phase = None
    for i, s in enumerate(plan, 1):
        if s["phase"] != phase:
            phase = s["phase"]
            print(f"\n{phase}", file=out)
        tag = {"sudo": "[sudo] ", "exec": "", "rm": "[hapus] ", "backup": "[backup] ", "restore_nginx": ""}[s["op"]]
        print(f"  {i:>2}. {tag}{s['desc']}", file=out)
    print("\nTidak disentuh:", file=out)
    for u in UNTOUCHED:
        print(f"  - {u}", file=out)


def uninstall_cmd(a):
    env = detect()
    try:
        if not env.mp.exists():
            raise UninstallError(f"{env.mp} tidak ada; tidak ada yang bisa dilepas")
        if (env.mp / "server").exists() is False:
            raise UninstallError(f"{env.mp} tidak tampak seperti instalasi MacPanel (tanpa folder server/); dibatalkan")
        if os.geteuid() == 0:
            raise UninstallError("jangan jalankan sebagai root; sudo dipanggil per langkah")
        plan = build_plan(env, a.level, a.backup_dir)
    except UninstallError as e:
        print("lab:", e, file=sys.stderr)
        return 1
    print_plan(plan, a.level)
    if not a.apply:
        print("\nINI HANYA RENCANA. Tidak ada yang diubah. Tambahkan --apply untuk menjalankan.")
        return 0
    if a.level == 3:
        if a.confirm_host != hostname() and not (sys.stdin.isatty() and input(f"\nKetik nama mesin ({hostname()}) untuk MENGHAPUS SEMUANYA: ").strip() == hostname()):
            print("lab: nama mesin tidak cocok; dibatalkan.", file=sys.stderr)
            return 1
    elif not a.yes:
        if not sys.stdin.isatty() or input("\nKetik 'uninstall' untuk melanjutkan: ").strip() != "uninstall":
            print("lab: dibatalkan.", file=sys.stderr)
            return 1
    if any(s["op"] == "sudo" for s in plan):
        if subprocess.run(["/usr/bin/sudo", "-v"]).returncode:
            print("lab: sudo gagal; jalankan dari terminal Anda sendiri.", file=sys.stderr)
            return 1
    return execute(env, plan)
