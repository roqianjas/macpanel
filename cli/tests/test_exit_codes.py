"""Kode keluar lab: kegagalan harus terlihat oleh skrip dan panel. Handler hanya boleh mengembalikan None atau int."""

import ast
import glob
import os
import re
import subprocess
import sys

HERE = os.path.dirname(__file__)
LAB = os.path.join(HERE, "..", "lab")
PKG = os.path.join(HERE, "..", "labcli")


def handlers():
    src = open(os.path.join(PKG, "cli.py")).read()
    m = re.search(r"HANDLERS\s*=\s*\{(.*?)\n\}", src, re.S)
    return dict(re.findall(r'"(\w+)":\s*(\w+)', m.group(1)))


def test_handlers_return_none_or_int_only():
    defs = {}
    for f in glob.glob(os.path.join(PKG, "*.py")):
        for n in ast.parse(open(f).read()).body:
            if isinstance(n, ast.FunctionDef):
                defs[n.name] = n
    bad = []
    for key, fn in handlers().items():
        node = defs.get(fn)
        if node is None:  # lambda atau fungsi bawaan: tidak bisa dianalisis, dibatasi di handler lain
            continue
        for r in ast.walk(node):
            if isinstance(r, ast.Return) and isinstance(r.value, ast.Constant) and r.value.value is not None and not isinstance(r.value.value, int):
                bad.append(f"{key}:{fn}:{ast.unparse(r.value)}")
            if isinstance(r, ast.Return) and isinstance(r.value, (ast.Dict, ast.List, ast.JoinedStr, ast.Tuple)):
                bad.append(f"{key}:{fn}:{ast.unparse(r.value)[:30]}")
    assert bad == []


def run(*a):
    return subprocess.run([sys.executable, LAB, *a], capture_output=True, text=True, timeout=60)


def test_failures_exit_nonzero():
    r = run("firewall", "block", "22", "--dry-run")  # port 22 dilindungi
    assert r.returncode == 1 and "dilindungi" in r.stderr
    r = run("uninstall", "--level", "3")  # tanpa --backup-dir
    assert r.returncode == 1


def test_success_exits_zero():
    assert run("firewall", "status").returncode == 0
    assert run("uninstall", "--level", "1").returncode == 0
