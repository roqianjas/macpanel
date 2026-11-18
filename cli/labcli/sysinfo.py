"""lab sys: proses, disk, keamanan, statistik, DNS."""

import json
import os
import re
import shutil
import subprocess
import time
from pathlib import Path

from labcli.core import ENV, HOME, OPS, ROOT, die, du_kb, sh

DEV_COMM = {"node", "php", "python", "python3", "go", "npm", "pnpm", "bun", "deno", "ruby", "vite", "esbuild", "tsx", "next-server", "composer", "java", "cargo"}


PROTECTED_COMM = {
    "panel",
    "launchd",
    "nginx",
    "mysqld",
    "postgres",
    "redis-server",
    "php-fpm",
    "dnsmasq",
    "sshd",
    "tailscaled",
    "Tailscale",
    "WindowServer",
    "loginwindow",
    "Finder",
    "netdata",
    "kernel_task",
}


def ps_rows():
    r = sh(["/bin/ps", "-axo", "pid=,user=,pcpu=,pmem=,rss=,etime=,comm="])
    me = os.environ.get("USER", HOME.name)
    out = []
    for l in r.stdout.splitlines():
        f = l.split(None, 6)
        if len(f) < 7:
            continue
        pid, user, cpu, mem, rss, et, comm = f
        out.append(dict(pid=int(pid), user=user, cpu=float(cpu), mem=float(mem), rss_mb=round(int(rss) / 1024, 1), etime=et, name=os.path.basename(comm), mine=(user == me)))
    return out


PROTECT_ARGS = re.compile(r"hermes|macpanel|/panel( |$)|launchd|netdata|uptime-kuma|tailscale|dnsmasq|nginx|mysqld|postgres|redis-server|php-fpm|browser_harness|chrome", re.IGNORECASE)


def cmd_map():
    r = sh(["/bin/ps", "-axo", "pid=,command="])
    out = {}
    for l in r.stdout.splitlines():
        f = l.strip().split(None, 1)
        if len(f) == 2 and f[0].isdigit():
            out[int(f[0])] = f[1]
    return out


def killable(row, cmds):
    return row["name"] in DEV_COMM and row["name"] not in PROTECTED_COMM and not PROTECT_ARGS.search(cmds.get(row["pid"], ""))


def sys_procs():
    rows = [r for r in ps_rows() if r["mine"]]
    cmds = cmd_map()
    for r in rows:
        r["killable"] = killable(r, cmds)
    rows.sort(key=lambda r: (-r["cpu"], -r["rss_mb"]))
    print(json.dumps(dict(procs=rows[:40], total=len(rows))))


def sys_kill(pid):
    if pid <= 1:
        die("pid tidak valid")
    row = next((r for r in ps_rows() if r["pid"] == pid), None)
    if not row:
        die("proses tidak ada")
    if not row["mine"]:
        die("proses milik user lain")
    if not killable(row, cmd_map()):
        die(f"'{row['name']}' dilindungi atau bukan proses pengembangan; hentikan lewat halaman Aplikasi atau terminal")
    if False:
        die(f"'{row['name']}' bukan proses pengembangan; hentikan lewat halaman Aplikasi atau terminal")
    try:
        os.kill(pid, 15)
    except OSError as e:
        die(f"gagal: {e}")
    for _ in range(20):
        time.sleep(0.25)
        if not any(r["pid"] == pid for r in ps_rows()):
            print(f"OK: {row['name']} ({pid}) dihentikan (SIGTERM)")
            return
    print(f"SIGTERM terkirim ke {row['name']} ({pid}) tetapi proses belum berhenti; coba lagi nanti")


def sys_disk():
    items = []
    base = ROOT / "wwwroot"
    for p in sorted(base.iterdir()) if base.exists() else []:
        if p.is_dir() and not p.is_symlink():
            items.append((f"wwwroot/{p.name}", p))
    for lab, p in (("backup", ROOT / "backup"), ("Homebrew (Cellar)", Path("/opt/homebrew/Cellar")), ("mise (runtime)", HOME / ".local/share/mise")):
        if p.exists():
            items.append((lab, p))
    rows = [dict(name=n, kb=du_kb(p)) for n, p in items[:60]]
    rows = sorted([r for r in rows if r["kb"] is not None], key=lambda r: -r["kb"])[:30]
    st = shutil.disk_usage("/System/Volumes/Data")
    print(json.dumps(dict(rows=rows, total=st.total, free=st.free)))


def sys_security():
    fw = sh(["/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate"]).stdout.strip()
    stealth = sh(["/usr/libexec/ApplicationFirewall/socketfilterfw", "--getstealthmode"]).stdout.strip()
    sshcfg = ""
    for p in [Path("/etc/ssh/sshd_config")] + sorted(Path("/etc/ssh/sshd_config.d").glob("*.conf")) if Path("/etc/ssh").exists() else []:
        try:
            sshcfg += p.read_text(errors="replace") + "\n"
        except OSError:
            pass

    def opt(k, default):
        m = re.findall(rf"^\s*{k}\s+(\S+)", sshcfg, re.IGNORECASE | re.MULTILINE)
        return (m[0] if m else default).lower()

    r = sh(["/usr/sbin/lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcn"])
    ports, cmd = {}, ""
    for l in r.stdout.splitlines():
        if l.startswith("c"):
            cmd = l[1:]
        elif l.startswith("n"):
            addr, _, port = l[1:].rpartition(":")
            local = addr in ("127.0.0.1", "[::1]", "localhost")
            ports[(addr, port, cmd)] = dict(addr=addr, port=int(port) if port.isdigit() else 0, proc=cmd, exposed=not local)
    rows = sorted(ports.values(), key=lambda x: (not x["exposed"], x["port"]))
    print(
        json.dumps(
            dict(
                firewall=fw,
                stealth=stealth,
                ssh=dict(
                    password_auth=opt("PasswordAuthentication", "yes (bawaan)"),
                    root_login=opt("PermitRootLogin", "prohibit-password (bawaan)"),
                    kbd_interactive=opt("KbdInteractiveAuthentication", "yes (bawaan)"),
                ),
                ports=rows[:80],
            )
        )
    )


def sys_docker():
    exe = shutil.which("docker", path=ENV["PATH"])
    if not exe:
        print(json.dumps(dict(installed=False)))
        return
    r = sh([exe, "ps", "--format", "{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"], timeout=15)
    if r.returncode:
        print(json.dumps(dict(installed=True, running=False, error=(r.stderr or "").strip()[-200:])))
        return
    cs = [dict(zip(("name", "image", "status", "ports"), l.split("\t"))) for l in r.stdout.splitlines() if l.strip()]
    print(json.dumps(dict(installed=True, running=True, containers=cs)))


LOG_RE = re.compile(r'^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) (\S+)[^"]*" (\d{3}) (\d+|-)')


LOGD = Path("/opt/homebrew/var/log/nginx/sites")


def sys_stats(name):
    if not re.fullmatch(r"[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?", name or ""):
        die("nama situs tidak valid")
    f = LOGD / f"{name}.log"
    if not f.exists():
        print(json.dumps(dict(exists=False, requests=0, bytes=0, days=[], status={}, paths=[], ips=[], note="")))
        return
    size = f.stat().st_size
    with open(f, "rb") as fh:
        fh.seek(max(0, size - 8 * 1024 * 1024))
        data = fh.read().decode("utf-8", "replace")
    lines = data.splitlines()[1:] if size > 8 * 1024 * 1024 else data.splitlines()
    from collections import Counter

    days, status, paths, ips = Counter(), Counter(), Counter(), Counter()
    total_b = n = 0
    for l in lines[-300000:]:
        m = LOG_RE.match(l)
        if not m:
            continue
        ip, ts, _meth, path, code, b = m.groups()
        n += 1
        days[ts[:11]] += 1
        status[code[0] + "xx"] += 1
        paths[path.split("?")[0][:80]] += 1
        ips[ip] += 1
        total_b += int(b) if b.isdigit() else 0
    ds = sorted(days.items(), key=lambda kv: time.strptime(kv[0], "%d/%b/%Y"))[-14:]
    print(
        json.dumps(
            dict(
                exists=True,
                requests=n,
                bytes=total_b,
                days=[dict(day=k, n=v) for k, v in ds],
                status=dict(status),
                paths=[dict(path=k, n=v) for k, v in paths.most_common(10)],
                ips=[dict(ip=k, n=v) for k, v in ips.most_common(8)],
                note=f"dari {len(lines)} baris terakhir log",
            )
        )
    )


def sys_dns():
    """Baca saja: konfigurasi dnsmasq, resolver macOS, dan hasil resolusi nyata tiap situs. Tidak mengubah apa pun."""
    tld = (OPS / "tld").read_text().strip() if (OPS / "tld").exists() else "lab"
    conf = Path("/opt/homebrew/etc/dnsmasq.conf")
    lines = []
    if conf.exists():
        lines = [l.strip() for l in conf.read_text(errors="replace").splitlines() if l.strip() and not l.strip().startswith("#")]
    keep = [l for l in lines if l.split("=")[0] in ("address", "listen-address", "port", "server", "no-resolv", "domain-needed")]
    res = Path(f"/etc/resolver/{tld}")
    resolver = res.read_text(errors="replace").strip() if res.exists() else None
    wild = next((l.split("/")[-1] for l in keep if l.startswith(f"address=/{tld}/")), None)
    tsip = None
    for exe in ("/Applications/Tailscale.app/Contents/MacOS/Tailscale", "/opt/homebrew/bin/tailscale", "/usr/local/bin/tailscale"):
        if os.path.exists(exe):
            try:
                ts = sh([exe, "ip", "-4"], timeout=5, env=dict(ENV, HOME=str(HOME), USER=HOME.name))
                cand = ts.stdout.split()[0] if ts.stdout.split() else ""
                if ts.returncode == 0 and re.fullmatch(r"100\.(?:6[4-9]|[7-9]\d|1[01]\d|12[0-7])\.\d{1,3}\.\d{1,3}", cand):
                    tsip = cand
                    break
            except (OSError, subprocess.TimeoutExpired):
                pass
    names = [p.name[:-5] for p in sorted((OPS / "nginx/sites").glob("*.conf"))][:60]
    rows = []
    for n in names:
        r = sh(["/usr/bin/dig", "+short", "+time=2", "+tries=1", "@127.0.0.1", f"{n}.{tld}"])
        ip = r.stdout.split()[0] if r.stdout.split() else None
        rows.append(dict(name=f"{n}.{tld}", ip=ip, ok=bool(ip) and (wild is None or ip == wild)))
    port_up = sh(["/usr/bin/dig", "+short", "+time=2", "+tries=1", "@127.0.0.1", f"x-uji.{tld}"]).stdout.split()
    print(
        json.dumps(
            dict(tld=tld, config=keep, resolver=resolver, wildcard=wild, tailscale_ip=tsip, wildcard_matches_tailscale=(tsip == wild) if tsip and wild else None, responding=bool(port_up), rows=rows)
        )
    )


def sysx(a):
    if a.act == "hw":
        from labcli.hardware import sys_hw

        return sys_hw()
    if a.act == "ssd":
        from labcli.hardware import sys_ssd

        return sys_ssd()
    if a.act == "dns":
        return sys_dns()
    if a.act == "procs":
        return sys_procs()
    if a.act == "kill":
        return sys_kill(a.pid)
    if a.act == "disk":
        return sys_disk()
    if a.act == "security":
        return sys_security()
    if a.act == "docker":
        return sys_docker()
    if a.act == "stats":
        return sys_stats(a.name)
