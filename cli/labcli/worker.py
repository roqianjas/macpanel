"""lab worker / job: proses latar belakang dan cron lewat launchd."""

import os
import plistlib
import re
import shlex
import shutil
import subprocess
from pathlib import Path

from labcli.core import HOME, ROOT, UID, die

AGENTS = HOME / "Library/LaunchAgents"


LOGS = HOME / "Library/Logs/lab"


TASK_NAME_RE = re.compile(r"^[a-z][a-z0-9-]{0,39}$")


BASE_PATH = [str(HOME / ".local/share/mise/shims"), "/opt/homebrew/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"]


PHP = {"8.3": "/opt/homebrew/opt/php@8.3/bin", "8.4": "/opt/homebrew/opt/php@8.4/bin", "8.5": "/opt/homebrew/opt/php/bin"}


DEV_ROOT = ROOT / "wwwroot"


def label(kind, n):
    return f"com.lab.{kind}.{n}"


def plist_path(kind, n):
    return AGENTS / f"{label(kind, n)}.plist"


def lc(*a):
    return subprocess.run(["launchctl", *a], capture_output=True, text=True)


def build(kind, n, a, idx=None):
    if not TASK_NAME_RE.match(n):
        die("nama: huruf kecil/angka/-, diawali huruf, maks 40")
    d = Path(a.dir).expanduser().resolve()
    if not d.is_dir():
        die(f"folder tidak ada: {d}")
    if DEV_ROOT not in d.parents and d != DEV_ROOT:
        die("folder harus di bawah ~/MacPanel/wwwroot")
    argv = shlex.split(a.cmd)
    if not argv:
        die("--cmd kosong")
    path = ([PHP[a.php]] if a.php else []) + BASE_PATH
    extra = {}
    for kv in getattr(a, "env", None) or []:
        k, sep, v = kv.partition("=")
        if not sep or not re.fullmatch(r"[A-Z][A-Z0-9_]{0,40}", k) or k in ("PATH", "HOME") or not re.fullmatch(r"[A-Za-z0-9_.:/@+-]{0,200}", v):
            die(f"--env tidak valid: {k!r} (KUNCI huruf besar, nilai sederhana)")
        extra[k] = v
    exe = argv[0] if "/" in argv[0] else shutil.which(argv[0], path=":".join(path))
    if not exe or not os.access(exe, os.X_OK):
        die(f"program tidak ditemukan: {argv[0]}")
    argv[0] = exe
    nm = f"{n}-{idx}" if idx else n
    LOGS.mkdir(parents=True, exist_ok=True)
    p = {
        "Label": label(kind, nm),
        "ProgramArguments": argv,
        "WorkingDirectory": str(d),
        "EnvironmentVariables": {"PATH": ":".join(path), "HOME": str(HOME), **extra},
        "StandardOutPath": str(LOGS / f"{nm}.log"),
        "StandardErrorPath": str(LOGS / f"{nm}.log"),
        "ProcessType": "Background",
        "Nice": 5,
    }
    if kind == "worker":
        p.update(RunAtLoad=True, KeepAlive=True, ThrottleInterval=5)
    else:
        if a.every:
            m = re.fullmatch(r"(\d+)([mh])", a.every)
            if not m or int(m.group(1)) < 1:
                die("--every format: 1m, 5m, 1h")
            p["StartInterval"] = int(m.group(1)) * (60 if m.group(2) == "m" else 3600)
        elif a.at:
            m = re.fullmatch(r"([01]\d|2[0-3]):([0-5]\d)", a.at)
            if not m:
                die("--at format HH:MM")
            p["StartCalendarInterval"] = {"Hour": int(m.group(1)), "Minute": int(m.group(2))}
        else:
            die("pilih --every atau --at")
    return nm, p


def install(kind, nm, p):
    f = plist_path(kind, nm)
    if f.exists():
        die(f"'{nm}' sudah ada (lab {kind} remove {nm})")
    AGENTS.mkdir(exist_ok=True)
    with open(f, "wb") as fh:
        plistlib.dump(p, fh)
    os.chmod(f, 0o644)
    lint = subprocess.run(["plutil", "-lint", str(f)], capture_output=True, text=True)
    if lint.returncode:
        f.unlink()
        die("plist tidak valid: " + lint.stdout)
    r = lc("bootstrap", f"gui/{UID}", str(f))
    if r.returncode:
        f.unlink()
        die("launchctl gagal: " + (r.stderr or r.stdout).strip()[:200])
    print(f"OK: {kind} '{nm}' aktif ({label(kind, nm)})")


def names(kind):
    return sorted(f.name[len(f"com.lab.{kind}.") : -6] for f in AGENTS.glob(f"com.lab.{kind}.*.plist"))


def info(kind, nm):
    r = lc("print", f"gui/{UID}/{label(kind, nm)}")
    out = r.stdout
    g = lambda k: (re.search(rf"\b{k} = ([^\n(]\S*|\(never exited\))", out) or [None, "-"])[1].replace("(never exited)", "-")
    return ("jalan" if r.returncode == 0 and g("state") == "running" else ("terjadwal" if r.returncode == 0 else "tidak dimuat")), g("pid"), g("last exit code"), g("runs")


def cmd_add(kind, a):
    if kind == "worker" and a.procs > 1:
        if a.procs > 8:
            die("--procs maks 8")
        for i in range(1, a.procs + 1):
            nm, p = build(kind, a.name, a, i)
            install(kind, nm, p)
    else:
        nm, p = build(kind, a.name, a)
        install(kind, nm, p)


def targets(kind, n):
    t = [x for x in names(kind) if x == n or re.fullmatch(re.escape(n) + r"-\d+", x)]
    if not t:
        die(f"{kind} '{n}' tidak ada")
    return t


def run(kind, a):
    if a.act == "list":
        print(f"{'NAMA':24}{'STATUS':14}{'PID':8}{'EXIT':6}JADWAL/CMD")
        for n in names(kind):
            p = plistlib.loads(plist_path(kind, n).read_bytes())
            s, pid, ex, _ = info(kind, n)
            sch = (
                f"tiap {p['StartInterval']}s"
                if "StartInterval" in p
                else ("harian %02d:%02d" % (p["StartCalendarInterval"]["Hour"], p["StartCalendarInterval"]["Minute"]) if "StartCalendarInterval" in p else "KeepAlive")
            )
            print(f"{n:24}{s:14}{pid:8}{ex:6}{sch}  {' '.join(shlex.quote(x) for x in p['ProgramArguments'][:4])}")
    elif a.act == "add":
        cmd_add(kind, a)
    elif a.act == "status":
        for n in targets(kind, a.name):
            print(n, *info(kind, n))
    elif a.act == "logs":
        for n in targets(kind, a.name):
            f = LOGS / f"{n}.log"
            print(f"== {f}")
            if f.exists():
                print("\n".join(f.read_text(errors="replace").splitlines()[-a.n :]))
    elif a.act == "restart":
        for n in targets(kind, a.name):
            r = lc("kickstart", "-k", f"gui/{UID}/{label(kind, n)}")
            print(n, "restart" if not r.returncode else r.stderr.strip())
    elif a.act == "remove":
        for n in targets(kind, a.name):
            lc("bootout", f"gui/{UID}/{label(kind, n)}")
            plist_path(kind, n).unlink(missing_ok=True)
            print("dihapus:", n, "(log dipertahankan)")
