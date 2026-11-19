import subprocess

import pytest
from labcli import apache


def test_vhost_has_htaccess_and_php_handler():
    v = apache.vhost_text("blog.lab", "/srv/www/blog", "8.4")
    assert "AllowOverride All" in v and "Require all granted" in v
    assert "php-fpm-dev-8.4.sock" in v and "ServerName blog.lab" in v
    assert "-Indexes" in v  # tanpa daftar isi folder


def test_vhost_without_php_has_no_handler():
    assert "SetHandler" not in apache.vhost_text("a.lab", "/srv/www/a")


@pytest.mark.parametrize("d", ["/a b/c", '/a"b', "/a\nb", "rel/path", "/a/../etc", "/a\0b"])
def test_vhost_rejects_dangerous_directory(d):
    with pytest.raises(SystemExit):
        apache.vhost_text("a.lab", d)


@pytest.mark.parametrize("h", ["A.lab", "a b", "a;b", "", "-a", "a\nServerName evil"])
def test_vhost_rejects_bad_host(h):
    with pytest.raises(SystemExit):
        apache.vhost_text(h, "/srv/www/a")


def test_vhost_rejects_bad_php():
    with pytest.raises(SystemExit):
        apache.vhost_text("a.lab", "/srv/www/a", "8.4;id")


def test_conf_is_loopback_only_and_denies_by_default():
    c = apache.conf_text()
    assert "Listen 127.0.0.1:8080" in c
    assert "\nListen 8080" not in c and "Listen 0.0.0.0" not in c
    assert "Require all denied" in c and "TraceEnable Off" in c and "ServerTokens Prod" in c
    assert c.count("LoadModule") == len(apache.MODULES)
    assert str(apache.SITES) in c


def test_site_add_rolls_back_when_config_invalid(tmp_path, monkeypatch):
    monkeypatch.setattr(apache, "APACHE_DIR", tmp_path)
    monkeypatch.setattr(apache, "SITES", tmp_path / "sites")
    monkeypatch.setattr(apache, "CONF", tmp_path / "httpd.conf")
    monkeypatch.setattr(apache, "write_conf", lambda: (tmp_path / "sites").mkdir(exist_ok=True))
    monkeypatch.setattr(apache, "check", lambda: subprocess.CompletedProcess([], 1, "", "Syntax error"))
    with pytest.raises(SystemExit):
        apache.site_add("blog", "blog.lab", "/srv/www/blog")
    assert not (tmp_path / "sites" / "blog.conf").exists()


def test_site_remove_is_idempotent_and_validates(tmp_path, monkeypatch):
    monkeypatch.setattr(apache, "SITES", tmp_path)
    monkeypatch.setattr(apache, "graceful", lambda: None)
    (tmp_path / "blog.conf").write_text("x")
    apache.site_remove("blog")
    apache.site_remove("blog")
    assert not (tmp_path / "blog.conf").exists()
    with pytest.raises(SystemExit):
        apache.site_remove("../etc")
