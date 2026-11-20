import pytest
from labcli import public

PHP = """server {
    listen 80; listen [::]:80;
    server_name site.lab;
    include /srv/www/snippets/tailnet_only.conf;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl; listen [::]:443 ssl;
    http2 on;
    server_name site.lab;
    include /srv/www/snippets/tailnet_only.conf;
    ssl_certificate     /srv/certs/site.lab.pem;
    ssl_certificate_key /srv/certs/site.lab-key.pem;
    root /srv/www/site/public;
    location / { try_files $uri $uri/ /index.php?$query_string; }
    location ~ ^/admin/ { allow 10.0.0.1; deny all; }
    location ~ ^/old$ { return 301 /new; }
    location ~ \\.php$ { fastcgi_pass unix:/x.sock; fastcgi_param HTTPS on; }
}
"""


def conf(text=PHP, port=8450):
    return public.build_conf("site", port, text)


def test_listener_is_loopback_only_and_has_no_tls_or_tailnet_rule():
    c = conf()
    assert "listen 127.0.0.1:8450;" in c and "listen [::1]:8450;" in c
    assert "443" not in c and "ssl_" not in c and "tailnet_only" not in c and "server_name site.lab" not in c
    assert "listen 80" not in c and "listen [::]" not in c


def test_location_level_rules_and_redirects_are_kept():
    c = conf()
    assert "allow 10.0.0.1; deny all;" in c  # perlindungan situs di dalam location tidak boleh hilang
    assert "return 301 /new;" in c
    assert "root /srv/www/site/public;" in c and "fastcgi_pass unix:/x.sock;" in c


def test_dotfiles_blocked_even_if_site_has_no_rule():
    assert r"location ~ /\.(?!well-known) { return 404; }" in conf()


def test_scheme_forced_https_for_proxied_apps():
    t = PHP.replace("location / {", "location / { proxy_set_header X-Forwarded-Proto $scheme;")
    c = conf(t)
    assert "X-Forwarded-Proto https;" in c and "$scheme" not in c


def test_only_one_https_block_accepted():
    with pytest.raises(SystemExit):
        conf("server { listen 80; }")
    with pytest.raises(SystemExit):
        conf(PHP + PHP)


@pytest.mark.parametrize("port", [3306, 3307, 5432, 6379, 9100, 20128, 8025, 19999, 8080])
def test_proxy_to_internal_service_refused(port):
    t = PHP.replace("root /srv/www/site/public;", f"location /x {{ proxy_pass http://127.0.0.1:{port}; }}")
    with pytest.raises(SystemExit):
        conf(t)


def test_proxy_to_app_port_allowed():
    t = PHP.replace("root /srv/www/site/public;", "location /x { proxy_pass http://127.0.0.1:3100; }")
    assert "proxy_pass http://127.0.0.1:3100;" in conf(t)


@pytest.mark.parametrize("n", ["panel", "db", "home", "kuma", "stats", "9router", "adminer", "nginx", "../x", "A", ""])
def test_system_and_bad_names_refused(n):
    with pytest.raises(SystemExit):
        public.check_name(n)
