import subprocess
import sys

import pytest
from labcli import core


def test_die_exits_with_message(capsys):
    with pytest.raises(SystemExit) as e:
        core.die("rusak")
    assert e.value.code == 1
    assert capsys.readouterr().err == "lab: rusak\n"


def test_die_custom_code():
    with pytest.raises(SystemExit) as e:
        core.die("x", 3)
    assert e.value.code == 3


def test_sh_captures_output_and_does_not_use_a_shell():
    r = core.sh(["echo", "a;b && c"])
    assert r.returncode == 0 and r.stdout == "a;b && c\n"  # metakarakter shell tidak ditafsirkan


def test_sh_check_dies_on_failure(capsys):
    with pytest.raises(SystemExit):
        core.sh([sys.executable, "-c", "import sys; print('boom', file=sys.stderr); sys.exit(2)"], check=True)
    assert "boom" in capsys.readouterr().err


def test_sh_without_check_returns_failure():
    assert core.sh([sys.executable, "-c", "import sys; sys.exit(4)"]).returncode == 4


def test_stream_returns_exit_code():
    assert core.stream([sys.executable, "-c", "import sys; sys.exit(5)"]) == 5


def test_site_name_regex():
    ok = ["toko", "a", "my-site", "a1", "x" * 30]
    bad = ["", "-a", "a-", "UP", "a_b", "a.b", "a b", "x" * 40, "../etc", "a;b"]
    assert all(core.SITE_NAME_RE.match(n) for n in ok), [n for n in ok if not core.SITE_NAME_RE.match(n)]
    assert not any(core.SITE_NAME_RE.match(n) for n in bad), [n for n in bad if core.SITE_NAME_RE.match(n)]


def test_env_has_no_surprises():
    assert core.ENV["PATH"].startswith("/opt/homebrew/bin")
    assert core.ENV["HOMEBREW_NO_AUTO_UPDATE"] == "1"


def test_core_does_not_import_domain_modules():
    # fondasi tidak boleh bergantung pada modul domain (arah ketergantungan: domain -> core)
    src = subprocess.run(
        [sys.executable, "-c", "import labcli.core as c, sys; print(sorted(m for m in sys.modules if m.startswith('labcli.') and m != 'labcli.core'))"],
        capture_output=True,
        text=True,
        cwd=str(__import__("pathlib").Path(core.__file__).parents[1]),
    ).stdout
    assert src.strip() == "[]", src
