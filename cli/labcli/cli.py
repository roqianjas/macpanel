"""Tabel perintah `lab`: satu parser untuk semua modul, lalu dispatch ke fungsi tiap domain.

Bentuk perintah (argumen, pilihan, teks bantuan) dijaga identik dengan CLI lama lewat tes golden di tests/cli.
Menambah perintah: tulis fungsinya di modul domain, daftarkan parser di fungsi reg_* yang sesuai, tambahkan ke HANDLERS.
"""

import argparse
import json
import sys

from labcli.apache import apache
from labcli.backup import backup
from labcli.clean import clean_cmd, mem_cmd
from labcli.composer import composer
from labcli.core import PHPS
from labcli.db import cmd_create, cmd_drop, cmd_dump, cmd_list, cmd_restore
from labcli.dbcfg import dbcfg
from labcli.dbcore import ENGINES
from labcli.dbusers import (
    PRIVS,
    cmd_creds,
    cmd_passwd,
    cmd_root_passwd,
    cmd_user_add,
    cmd_user_list,
    cmd_user_passwd,
    cmd_user_privs,
    cmd_user_remove,
    cmd_users,
)
from labcli.deploy import deploy
from labcli.docker import (
    cmd_compose,
    cmd_container,
    cmd_image,
    containers,
    dk,
    images,
    jlines,
    need_running,
    projects,
    status,
)
from labcli.ext import ext_act, ext_list
from labcli.firewall import firewall_cmd
from labcli.ini import ini_get, ini_set
from labcli.install import app_install, site_clone
from labcli.nginx import nginx
from labcli.php import php
from labcli.phpdefault import php_default
from labcli.public import public
from labcli.redis import redis_cmd
from labcli.regproxy import proxy_cmd
from labcli.runtime import runtime
from labcli.services import app, service
from labcli.site import site
from labcli.sysinfo import sysx
from labcli.tailscale import tailscale_cmd
from labcli.uninstall import uninstall_cmd
from labcli.update import apply, check
from labcli.worker import PHP, TYPES, run

# ---------- pendaftaran parser, satu fungsi per kelompok perintah ----------


def reg_db(top):
    db = top.add_parser("db").add_subparsers(dest="act", required=True)
    db.add_parser("list").set_defaults(f=cmd_list)
    for nm, fn in (("create", cmd_create), ("dump", cmd_dump), ("drop", cmd_drop), ("restore", cmd_restore), ("users", cmd_users), ("passwd", cmd_passwd), ("creds", cmd_creds)):
        p = db.add_parser(nm)
        p.add_argument("name")
        p.add_argument("--engine", choices=ENGINES, default="mariadb")
        p.set_defaults(f=fn)
        if nm == "create":
            p.add_argument("--env")
        if nm == "restore":
            p.add_argument("--file")
        if nm == "passwd":
            p.add_argument("--password-stdin", action="store_true")
    du = db.add_parser("user").add_subparsers(dest="uact", required=True)
    for nm, fn in (("list", cmd_user_list), ("add", cmd_user_add), ("passwd", cmd_user_passwd), ("privs", cmd_user_privs), ("remove", cmd_user_remove)):
        p = du.add_parser(nm)
        p.add_argument("name")
        p.add_argument("--engine", choices=ENGINES, default="mariadb")
        p.set_defaults(f=fn)
        if nm != "list":
            p.add_argument("user")
        if nm in ("add", "privs"):
            p.add_argument("--privs", choices=PRIVS, required=True)
        if nm in ("add", "passwd"):
            p.add_argument("--password-stdin", action="store_true")
    p = db.add_parser("root-passwd")
    p.add_argument("--engine", choices=ENGINES, required=True)
    p.add_argument("--password-stdin", action="store_true")
    p.set_defaults(f=cmd_root_passwd)


def reg_worker(top):
    for kind in ("worker", "job"):
        s = top.add_parser(kind).add_subparsers(dest="act", required=True)
        s.add_parser("list")
        p = s.add_parser("add")
        p.add_argument("name")
        p.add_argument("--dir", help="folder kerja di bawah ~/MacPanel/wwwroot (bawaan: akar wwwroot)")
        p.add_argument("--type", choices=list(TYPES), default="cmd", help="cmd/php/node/python: --cmd; shell: skrip lewat --script-stdin; url: --url")
        p.add_argument("--cmd")
        p.add_argument("--url")
        p.add_argument("--script-stdin", dest="script_stdin", action="store_true", help="baca isi skrip shell dari stdin")
        p.add_argument("--php", choices=list(PHP))
        p.add_argument("--env", action="append", default=[])
        if kind == "worker":
            p.add_argument("--procs", type=int, default=1)
        else:
            p.add_argument("--every")
            p.add_argument("--at")
            p.add_argument("--cron", help="5 kolom cron (menit jam tanggal bulan hari) atau @reboot/@hourly/@daily/@weekly/@monthly")
        for x in ("status", "restart", "remove", "logs"):
            q = s.add_parser(x)
            q.add_argument("name")
            if x == "logs":
                q.add_argument("-n", type=int, default=40)


def reg_sys(top):
    s = top.add_parser("site").add_subparsers(dest="act", required=True)
    s.add_parser("list")
    p = s.add_parser("add")
    p.add_argument("name")
    p.add_argument("target")
    p.add_argument("--php", choices=PHPS)
    p.add_argument("--wordpress", action="store_true")
    p.add_argument("--apache", action="store_true", help="layani lewat Apache (.htaccess) di belakang nginx")
    s.add_parser("remove").add_argument("name")
    q = s.add_parser("cert")
    q.add_argument("name")
    q.add_argument("aliases", nargs="*")
    for x in ("enable", "disable"):
        s.add_parser(x).add_argument("name")
    sv = top.add_parser("service").add_subparsers(dest="act", required=True)
    q = sv.add_parser("list")
    q.add_argument("--json", action="store_true")
    for x in ("start", "stop", "restart"):
        sv.add_parser(x).add_argument("id")
    q = sv.add_parser("logs")
    q.add_argument("id")
    q.add_argument("-n", default="40")
    ap_ = top.add_parser("app").add_subparsers(dest="act", required=True)
    ap_.add_parser("list")
    for x in ("install", "remove"):
        ap_.add_parser(x).add_argument("id")
    ax = top.add_parser("apache").add_subparsers(dest="act", required=True)
    ax.add_parser("config")
    ax.add_parser("sites")
    q = ax.add_parser("site-add")
    q.add_argument("name")
    q.add_argument("host")
    q.add_argument("dir")
    q.add_argument("--php", choices=PHPS)
    ax.add_parser("site-remove").add_argument("name")
    pb = top.add_parser("public", help="buka satu situs ke publik sementara (Cloudflare Quick Tunnel)").add_subparsers(dest="act", required=True)
    q = pb.add_parser("on")
    q.add_argument("name")
    q.add_argument("--ttl", default="120", help="batas waktu menit (5-1440, bawaan 120)")
    q = pb.add_parser("off")
    q.add_argument("name", nargs="?")
    q.add_argument("--all", action="store_true")
    q = pb.add_parser("status")
    q.add_argument("name", nargs="?")
    q.add_argument("--json", action="store_true")
    q = pb.add_parser("_run")
    q.add_argument("name")
    q.add_argument("port")
    q.add_argument("ttl")
    ng = top.add_parser("nginx").add_subparsers(dest="act", required=True)
    ng.add_parser("test")
    ng.add_parser("reload")
    ph = top.add_parser("php").add_subparsers(dest="act", required=True)
    ph.add_parser("list")
    for x in ("install", "remove"):
        ph.add_parser(x).add_argument("version")
    ph.add_parser("default", help="PHP CLI default global (tanpa argumen: tampilkan)").add_argument("version", nargs="?")
    q = ph.add_parser("which", help="versi PHP CLI yang terpakai di folder ini dan alasannya")
    q.add_argument("dir", nargs="?")
    q = ph.add_parser("status", help="default CLI, shim, dan versi web per situs")
    q.add_argument("--json", action="store_true")
    q = ph.add_parser("shims", help="pasang shim php (memilih versi per proyek) dan blok PATH di ~/.zshrc")
    q.add_argument("--remove", action="store_true", help="hapus blok PATH dari ~/.zshrc")
    r = top.add_parser("runtime").add_subparsers(dest="act", required=True)
    r.add_parser("list")
    for x in ("install", "use", "remove"):
        q = r.add_parser(x)
        q.add_argument("tool")
        q.add_argument("version")
    u = top.add_parser("update").add_subparsers(dest="act", required=True)
    u.add_parser("check")
    q = u.add_parser("apply")
    q.add_argument("target")
    q.add_argument("--allow-pinned", action="store_true")
    q.add_argument("--dry-run", action="store_true")


def reg_dev(top):
    e = top.add_parser("ext").add_subparsers(dest="act", required=True)
    q = e.add_parser("list")
    q.add_argument("php")
    for x in ("install", "remove", "enable", "disable"):
        q = e.add_parser(x)
        q.add_argument("php")
        q.add_argument("name")
    i = top.add_parser("ini").add_subparsers(dest="act", required=True)
    q = i.add_parser("get")
    q.add_argument("php")
    q = i.add_parser("set")
    q.add_argument("php")
    q.add_argument("pairs", nargs="+")
    c = top.add_parser("composer").add_subparsers(dest="act", required=True)
    for x in ("status", "install", "update"):
        c.add_parser(x)
    q = c.add_parser("run")
    q.add_argument("site")
    q.add_argument("cmd", choices=["install", "install-nodev", "update", "dump-autoload", "validate"])
    d = top.add_parser("deploy").add_subparsers(dest="act", required=True)
    d.add_parser("info").add_argument("site")
    q = d.add_parser("run")
    q.add_argument("site")
    q.add_argument("steps")
    ins = top.add_parser("appinstall")
    ins.add_argument("kind", choices=["laravel", "wordpress", "go", "python", "nextjs"])
    ins.add_argument("--runtime")
    ins.add_argument("--port", type=int)
    ins.add_argument("name")
    ins.add_argument("--php", default="8.4")
    ins.add_argument("--db", choices=["mariadb", "mysql"], default="mariadb")
    cl = top.add_parser("siteclone")
    cl.add_argument("src")
    cl.add_argument("dst")
    cl.add_argument("--with-db", action="store_true")
    rd = top.add_parser("redis").add_subparsers(dest="act", required=True)
    for x in ("info", "scan", "get", "del"):
        q = rd.add_parser(x)
        q.add_argument("--db", type=int, default=0)
        q.add_argument("arg", nargs="?", default="")
    rd.add_parser("password", help="kata sandi Redis: status | enable | disable").add_argument("mode", choices=["status", "enable", "disable"])
    px = top.add_parser("proxy", help="proxy registry npm lokal yang melanjutkan unduhan terputus")
    px.add_argument("mode", choices=["status", "enable", "disable", "purge", "serve"])
    fw = top.add_parser("firewall", help="tutup port dari LAN lewat pf (sudo dijalankan di terminal Anda)")
    fw.add_argument("mode", choices=["status", "block", "unblock", "verify"])
    fw.add_argument("port", nargs="?", type=int, help="nomor port (block/unblock)")
    fw.add_argument("--allow", choices=["tailnet", "none"], default="tailnet", help="block: tailnet tetap boleh (bawaan) atau hanya loopback")
    fw.add_argument("--dry-run", action="store_true", help="tampilkan langkah tanpa menjalankan sudo")
    un = top.add_parser("uninstall", help="lepas MacPanel dan aplikasi bundelnya (tanpa --apply hanya rencana)")
    un.add_argument("--level", type=int, choices=[1, 2, 3, 4], default=1, help="1 panel; 2 + aplikasi dihentikan; 3 + paket, data, dan ~/MacPanel dihapus; 4 HANYA Tailscale (berdiri sendiri)")
    un.add_argument("--backup-dir", default="", help="tingkat 3: folder backup akhir di LUAR ~/MacPanel")
    un.add_argument("--apply", action="store_true", help="jalankan; tanpa ini hanya mencetak rencana")
    un.add_argument("--yes", action="store_true", help="lewati pertanyaan tingkat 1-2 (tidak berlaku untuk tingkat 3)")
    un.add_argument("--with-docker-tools", action="store_true", help="tingkat 3: hapus juga colima, docker, docker-compose, lima")
    un.add_argument("--allow-remote-cut", action="store_true", help="tingkat 4: izinkan walau sesi SSH ini lewat tailnet (sesi akan putus)")
    un.add_argument("--confirm-host", default="", help="tingkat 3 non-interaktif: nama mesin")
    ts = top.add_parser("tailscale", help="status dan pemasangan Tailscale (tanpa --apply hanya rencana)").add_subparsers(dest="tact", required=True)
    ts.add_parser("status", help="terpasang, daemon, dan status tailnet")
    ti = ts.add_parser("install", help="rencana pemasangan (Homebrew + daemon sistem)")
    ti.add_argument("--apply", action="store_true", help="jalankan; tanpa ini hanya rencana")
    for act, hlp in (
        ("start", "jalankan daemon (sudo)"),
        ("stop", "hentikan daemon (sudo; memutus akses jarak jauh)"),
        ("restart", "mulai ulang daemon (sudo)"),
        ("down", "putuskan sambungan tailnet, daemon tetap hidup"),
    ):
        sp = ts.add_parser(act, help=hlp)
        sp.add_argument("--apply", action="store_true", help="jalankan; tanpa ini hanya rencana")
        sp.add_argument("--allow-remote-cut", action="store_true", help="izinkan walau sesi SSH ini lewat tailnet (sesi akan putus)")
    cl = top.add_parser("clean", help="analisis disk dan pembersihan aman (tanpa --apply hanya pratinjau)").add_subparsers(dest="cact", required=True)
    cl.add_parser("scan", help="ukur katalog lokasi dan simpan hasilnya")
    cl.add_parser("last", help="tampilkan hasil analisis terakhir")
    q = cl.add_parser("run", help="bersihkan entri katalog (pratinjau bila tanpa --apply)")
    q.add_argument("ids", nargs="+", help="id entri dari lab clean scan")
    q.add_argument("--apply", action="store_true", help="jalankan; tanpa ini hanya pratinjau")
    q.add_argument("--confirm-careful", action="store_true", help="wajib untuk entri tingkat Hati-hati")
    q.add_argument("--confirm-review", action="store_true", help="wajib untuk cache terdeteksi kelas Tinjau")
    mm = top.add_parser("mem", help="ringkasan memori dan saran penghematan").add_subparsers(dest="mact", required=True)
    mm.add_parser("top")
    sy = top.add_parser("sys").add_subparsers(dest="act", required=True)
    for x in ("procs", "disk", "security", "docker", "dns", "hw", "ssd", "exposure"):
        sy.add_parser(x)
    sy.add_parser("net").add_argument("--dns-name", default="", help="nama lokal yang dites resolusinya (mis. panel.lab)")
    sy.add_parser("kill").add_argument("pid", type=int)
    sy.add_parser("stats").add_argument("name")
    dc = top.add_parser("dbcfg").add_subparsers(dest="act", required=True)
    for x in ("get", "slow"):
        q = dc.add_parser(x)
        q.add_argument("engine", choices=["mariadb", "mysql", "redis"])
    q = dc.add_parser("set")
    q.add_argument("engine", choices=["mariadb", "mysql", "redis"])
    q.add_argument("pairs", nargs="+")
    b = top.add_parser("backup").add_subparsers(dest="act", required=True)
    b.add_parser("list")
    b.add_parser("run")
    q = b.add_parser("config")
    q.add_argument("cact", choices=["get", "set"])
    q.add_argument("--time")
    q.add_argument("--keep", type=int, default=7)
    q.add_argument("--days", type=int, default=0)
    q.add_argument("--extra-days", dest="extra_days", type=int, default=0)
    q.add_argument("--enabled", default="yes", help="yes atau no: nyalakan/matikan backup otomatis")
    q.add_argument("--every-days", dest="every_days", type=int, default=1, help="interval backup otomatis, 1-30 hari")
    q = b.add_parser("prune")
    q.add_argument("--keep", type=int)
    q.add_argument("--days", type=int)
    q.add_argument("--dry-run", action="store_true")
    q.add_argument("--auto", action="store_true")
    q = b.add_parser("delete")
    q.add_argument("names", nargs="+")
    q = b.add_parser("extras")
    q.add_argument("act2", choices=["info", "db", "restore"])
    q.add_argument("--days", type=int, default=0)
    q.add_argument("--dry-run", action="store_true")
    q = b.add_parser("ls")
    q.add_argument("name")
    q.add_argument("part")
    q.add_argument("prefix", nargs="?", default="")
    q = b.add_parser("extract")
    q.add_argument("name")
    q.add_argument("part")
    q.add_argument("path")


def reg_docker(top):
    d = top.add_parser("docker").add_subparsers(dest="cmd", required=True)
    d.add_parser("status")
    d.add_parser("overview")
    d.add_parser("stats")
    c = d.add_parser("container").add_subparsers(dest="act", required=True)
    for x in ("start", "stop", "restart", "remove"):
        c.add_parser(x).add_argument("target")
    q = c.add_parser("logs")
    q.add_argument("target")
    q.add_argument("-n", type=int, default=100)
    i = d.add_parser("image").add_subparsers(dest="act", required=True)
    i.add_parser("pull").add_argument("ref")
    i.add_parser("remove").add_argument("ref")
    i.add_parser("prune")
    m = d.add_parser("compose").add_subparsers(dest="act", required=True)
    m.add_parser("list")
    for x in ("up", "down", "ps"):
        m.add_parser(x).add_argument("project")
    q = m.add_parser("logs")
    q.add_argument("project")
    q.add_argument("-n", type=int, default=100)


def build_parser():
    ap = argparse.ArgumentParser(prog="lab")
    top = ap.add_subparsers(dest="mod", required=True)
    reg_db(top)
    reg_worker(top)
    reg_sys(top)
    reg_dev(top)
    reg_docker(top)
    return ap


# ---------- dispatch ----------


def run_docker(a):
    if a.cmd == "status":
        print(json.dumps(status()))
        return
    if a.cmd == "overview":
        st = status()
        if st["running"]:
            print(json.dumps(dict(status=st, containers=containers(), images=images(), projects=projects())))
        else:
            print(json.dumps(dict(status=st, containers=[], images=[], projects=projects())))
        return
    if a.cmd == "stats":
        need_running()
        print(
            json.dumps(
                dict(
                    stats=[
                        dict(name=s.get("Name", ""), cpu=s.get("CPUPerc", ""), mem=s.get("MemUsage", ""), memp=s.get("MemPerc", ""))
                        for s in jlines(dk("stats", "--no-stream", "--format", "{{json .}}", check=True, timeout=30).stdout)
                    ]
                )
            )
        )
        return
        return
    {"container": cmd_container, "image": cmd_image, "compose": cmd_compose}[a.cmd](a)


HANDLERS = {
    "db": lambda a: a.f(a),
    "worker": lambda a: run("worker", a),
    "job": lambda a: run("job", a),
    "site": site,
    "php": lambda a: php_default(a) if a.act in ("default", "which", "status", "shims") else php(a),
    "runtime": runtime,
    "nginx": nginx,
    "apache": apache,
    "public": public,
    "service": service,
    "app": app,
    "update": lambda a: check(a) if a.act == "check" else apply(a),
    "ext": lambda a: ext_list(a.php) if a.act == "list" else ext_act(a),
    "ini": lambda a: ini_get(a.php) if a.act == "get" else ini_set(a),
    "composer": composer,
    "deploy": deploy,
    "backup": backup,
    "dbcfg": dbcfg,
    "sys": sysx,
    "redis": redis_cmd,
    "proxy": proxy_cmd,
    "firewall": firewall_cmd,
    "clean": clean_cmd,
    "tailscale": tailscale_cmd,
    "mem": mem_cmd,
    "uninstall": uninstall_cmd,
    "appinstall": app_install,
    "siteclone": site_clone,
    "docker": run_docker,
}


def main():
    a = build_parser().parse_args()
    rc = HANDLERS[a.mod](a)
    # Kode keluar harus mencerminkan kegagalan: skrip dan panel membacanya. Handler yang tidak mengembalikan angka = sukses.
    if isinstance(rc, int) and not isinstance(rc, bool) and rc:
        sys.exit(rc)
