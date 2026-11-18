#!/usr/bin/env python3
"""Perbarui snapshot golden teks bantuan `lab`. Jalankan hanya bila perubahan perintah disengaja, lalu tinjau diff-nya."""

import json
import os
import sys
from pathlib import Path

os.environ["COLUMNS"] = "100"
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from labcli.cli import build_parser  # noqa: E402
from test_help_golden import walk  # noqa: E402

out = Path(__file__).parent / "golden" / "help.json"
out.parent.mkdir(exist_ok=True)
out.write_text(json.dumps(dict(walk(build_parser())), indent=1, sort_keys=True, ensure_ascii=False) + "\n")
print("snapshot ditulis:", out)
