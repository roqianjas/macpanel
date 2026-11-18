"""lab runtime: versi Node, Go, Python, pnpm lewat mise."""

import re
import sys

from labcli.core import MISE, die, stream

TOOLS = ["node", "go", "python", "pnpm"]


VER_RE = re.compile(r"^(latest|lts|\d+(\.\d+){0,2})$")


def runtime(a):
    if a.act == "list":
        sys.exit(stream([MISE, "ls"]))
    if a.tool not in TOOLS:
        die(f"tool didukung: {', '.join(TOOLS)}")
    if not VER_RE.match(a.version):
        die("versi: angka (22, 24.1) atau latest/lts")
    spec = f"{a.tool}@{a.version}"
    if a.act == "install":
        sys.exit(stream([MISE, "install", spec]))
    if a.act == "use":
        sys.exit(stream([MISE, "use", "-g", spec]))
    if a.act == "remove":
        sys.exit(stream([MISE, "uninstall", spec]))
