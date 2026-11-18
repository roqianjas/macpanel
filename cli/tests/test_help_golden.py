"""Tes golden: bentuk perintah `lab` (argumen, pilihan, teks bantuan) tidak boleh berubah tanpa disengaja.

Panel (Go) memanggil `lab` dengan argumen tertentu; tes ini menjaga kontrak itu. Bila perubahan disengaja,
perbarui snapshot: `python3 cli/tests/update_golden.py` lalu tinjau diff-nya.
"""

import argparse

from labcli.cli import build_parser


def walk(parser, path=()):
    yield " ".join(path) or "(root)", parser.format_help()
    for act in parser._actions:
        if isinstance(act, argparse._SubParsersAction):
            for name, sub in act.choices.items():
                yield from walk(sub, (*path, name))


def test_help_tree_matches_snapshot(golden):
    now = dict(walk(build_parser()))
    assert set(now) == set(golden), f"perintah berubah: tambah={sorted(set(now) - set(golden))} hilang={sorted(set(golden) - set(now))}"
    beda = [k for k in golden if now[k] != golden[k]]
    assert not beda, "teks bantuan berubah untuk: " + ", ".join(beda)


def test_every_top_level_command_is_registered():
    from labcli.cli import HANDLERS

    p = build_parser()
    tops = next(a for a in p._actions if isinstance(a, argparse._SubParsersAction)).choices
    assert set(tops) == set(HANDLERS), "setiap perintah tingkat atas harus punya handler dan sebaliknya"
