"""lab composer: status, pasang, dan jalankan Composer."""

import json
import re
import sys
from pathlib import Path

from labcli.core import BREW, sh, stream
from labcli.deploy import project, run_step
from labcli.phpenv import COMPOSER


def composer_status():
    inst = Path(COMPOSER).exists()
    ver, php = "", ""
    if inst:
        r = sh([COMPOSER, "--version", "--no-ansi"])
        m = re.search(r"Composer version (\S+)", r.stdout)
        ver = m.group(1) if m else ""
        m = re.search(r"PHP version (\S+)", r.stdout)
        php = m.group(1) if m else ""
    out = sh([BREW, "outdated", "--json=v2", "composer"]).stdout or "{}"
    try:
        newer = (json.loads(out).get("formulae") or [{}])[0].get("current_version", "")
    except (ValueError, IndexError, AttributeError):
        newer = ""
    print(json.dumps(dict(installed=inst, version=ver, php=php, outdated=bool(newer), latest=newer)))


def composer(a):
    if a.act == "status":
        return composer_status()
    if a.act == "install":
        if Path(COMPOSER).exists():
            print("Composer sudah terpasang")
            return
        sys.exit(stream([BREW, "install", "composer"]))
    if a.act == "update":
        sys.exit(stream([BREW, "upgrade", "composer"]))
    if a.act == "run":
        d, v = project(a.site)
        sys.exit(0 if run_step(a.cmd.replace("-", "_"), d, v) else 1)
