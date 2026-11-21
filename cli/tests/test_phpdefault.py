import os
import subprocess

import pytest
from labcli import phpdefault as pd


@pytest.fixture
def env(tmp_path, monkeypatch):
    tmp_path = tmp_path.resolve()  # macOS: /var -> /private/var; $PWD memakai jalur nyata
    home = tmp_path / "home"
    root = home / "MacPanel"
    sites = root / "server/nginx/sites"
    sites.mkdir(parents=True)
    prefix = tmp_path / "opt"
    for v, d in (("8.2", "php@8.2"), ("8.3", "php@8.3"), ("8.4", "php@8.4"), ("8.5", "php")):
        b = prefix / d / "bin"
        b.mkdir(parents=True)
        (b / "php").write_text(f'#!/bin/sh\necho "PHP {v} $@"\n')
        (b / "php").chmod(0o755)
    shim_dir = root / "server/shims"
    monkeypatch.setattr(pd, "SHIM_DIR", shim_dir)
    monkeypatch.setattr(pd, "DEFAULT_FILE", home / ".config/macminiops/php-default")
    monkeypatch.setattr(pd, "SITES", sites)
    monkeypatch.setattr(pd, "ROOT", root)
    monkeypatch.setattr(pd, "ZSHRC", home / ".zshrc")
    monkeypatch.setattr(pd, "HOME", home)
    monkeypatch.setattr(pd, "php_home", lambda v: prefix / ("php" if v == "8.5" else f"php@{v}"))
    pd.write_shims()
    base = dict(os.environ, HOME=str(home), MACPANEL_HOME=str(root), MACPANEL_PHP_PREFIX=str(prefix), MACPANEL_PHP_DEFAULT_FILE=str(home / ".config/macminiops/php-default"))

    def run(cwd, cmd="php", *args, print_mode=False):
        e = dict(base)
        if print_mode:
            e["MACPANEL_PHP_SHIM_PRINT"] = "1"
        r = subprocess.run(["/bin/sh", str(shim_dir / cmd), *args], cwd=cwd, env=e, capture_output=True, text=True)
        return r

    def site(name, root_dir, ver):
        (sites / f"{name}.conf").write_text(f"server {{\n    root {root_dir}/public;\n    location ~ \\.php$ {{ fastcgi_pass unix:/x/php-fpm-dev-{ver}.sock; }}\n}}\n")

    return type("E", (), {"home": home, "tmp": tmp_path, "run": staticmethod(run), "site": staticmethod(site), "default": home / ".config/macminiops/php-default"})


def test_default_global_used_outside_projects(env):
    pd.set_default("8.4")
    d = env.tmp / "elsewhere"
    d.mkdir()
    r = env.run(d, "php", "-v")
    assert r.stdout.strip() == "PHP 8.4 -v", r


def test_site_version_wins_over_default_and_matches_subfolders(env):
    pd.set_default("8.4")
    proj = env.tmp / "www/urusin"
    (proj / "app/Http").mkdir(parents=True)
    env.site("urusin", proj, "8.3")
    assert env.run(proj, "php").stdout.strip() == "PHP 8.3"
    assert env.run(proj / "app/Http", "php").stdout.strip() == "PHP 8.3"
    other = env.tmp / "www/urusin-lain"
    other.mkdir()
    assert env.run(other, "php").stdout.strip() == "PHP 8.4"  # awalan nama yang sama bukan situs itu


def test_php_version_file_wins_over_site(env):
    proj = env.tmp / "www/p"
    (proj / "sub").mkdir(parents=True)
    env.site("p", proj, "8.3")
    (proj / ".php-version").write_text("8.5\n")
    assert env.run(proj / "sub", "php").stdout.strip() == "PHP 8.5"


def test_bad_or_missing_version_falls_through(env):
    pd.set_default("8.4")
    proj = env.tmp / "www/q"
    proj.mkdir(parents=True)
    (proj / ".php-version").write_text("7.4\n")
    assert env.run(proj, "php").stdout.strip() == "PHP 8.4"
    (proj / ".php-version").write_text("8.3; rm -rf /\n")
    assert env.run(proj, "php").stdout.strip() == "PHP 8.4"


def test_no_default_falls_back_to_homebrew_default(env):
    d = env.tmp / "x"
    d.mkdir()
    assert env.run(d, "php").stdout.strip() == "PHP 8.5"


def test_versioned_shims_force_their_version(env):
    pd.set_default("8.4")
    d = env.tmp / "x"
    d.mkdir()
    assert env.run(d, "php8.2", "-r", "1").stdout.strip() == "PHP 8.2 -r 1"
    r = env.run(d, "php8.3", print_mode=True)
    assert r.stdout.split("\t")[0] == "8.3" and "dipaksa" in r.stdout


def test_arguments_with_spaces_passed_intact(env):
    pd.set_default("8.3")
    d = env.tmp / "x"
    d.mkdir()
    assert env.run(d, "php", "a b", "c").stdout.strip() == "PHP 8.3 a b c"


def test_set_default_rejects_bad_and_uninstalled(env, monkeypatch):
    for bad in ("7.4", "8.3; x", "", "8.", "../8.3"):
        with pytest.raises(SystemExit):
            pd.set_default(bad)
    monkeypatch.setattr(pd, "php_home", lambda v: env.tmp / "nope")
    with pytest.raises(SystemExit):
        pd.set_default("8.3")
    assert not env.default.exists()


def test_zshrc_block_added_once_removed_cleanly_with_backup(env):
    z = env.home / ".zshrc"
    z.write_text("export A=1\n")
    assert pd.install_zshrc() is True
    assert pd.install_zshrc() is False
    t = z.read_text()
    assert t.count(pd.BEGIN) == 1 and "export A=1" in t and "$HOME/MacPanel/server/shims" in t
    assert len(list(env.home.glob(".zshrc.bak.*"))) == 1
    assert pd.remove_zshrc() is True
    assert pd.BEGIN not in z.read_text() and "export A=1" in z.read_text()
    assert pd.remove_zshrc() is False


def test_zshrc_block_moves_shim_to_front_even_if_already_in_path(env):
    """Bug nyata 1: PATH yang diwarisi sudah memuat shim di belakang Homebrew; blok lama melewatinya sehingga php jatuh ke Homebrew.
    Bug nyata 2: garis miring literal di pola ${PATH//..//..} memecah pola di zsh dan merusak entri PATH lain."""
    import shutil

    d = str(pd.SHIM_DIR)
    inner = pd.zshrc_block().splitlines()[2].replace("$HOME", str(env.home))
    assert "$HOME" not in inner
    others = ["/usr/bin", "/opt/homebrew/opt/libpq/bin", "/srv/tools/mise/installs/node/lts/bin", "/bin"]
    starts = [":".join(others[:2] + [d] + others[2:]), d + ":" + ":".join(others), ":".join(others), ":".join(["/a", d, "/b", d])]
    for sh in [s for s in ("/bin/bash", shutil.which("zsh")) if s]:
        for start in starts:
            cmd = f'PATH={start!r}; HOME={str(env.home)!r}; {pd.zshrc_block().splitlines()[2]}; print -l ${{(s.:.)PATH}} 2>/dev/null || echo "$PATH"'
            r = subprocess.run([sh, "-c", cmd.replace("print -l ${(s.:.)PATH} 2>/dev/null || ", "")], capture_output=True, text=True)
            parts = r.stdout.strip().split(":")
            allowed = set(start.split(":")) | {d}
            assert parts[0] == d, (sh, start, r.stdout)
            assert parts.count(d) == 1, (sh, start, r.stdout)
            assert "" not in parts, (sh, start, r.stdout)
            assert set(parts) <= allowed, ("entri PATH rusak", sh, start, sorted(set(parts) - allowed))
    del inner


def test_install_replaces_old_style_block(env):
    z = env.home / ".zshrc"
    z.write_text("export A=1\n\n" + pd.BEGIN + '\ncase ":$PATH:" in *":x:"*) ;; *) export PATH="x:$PATH" ;; esac\n' + pd.END + "\n")
    assert pd.install_zshrc() is True
    t = z.read_text()
    assert t.count(pd.BEGIN) == 1 and "case" not in t and "export A=1" in t
    assert pd.install_zshrc() is False
