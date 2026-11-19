"""regproxy: unduhan dilanjutkan dengan Range, integritas diperiksa, banyak klien satu unduhan. Upstream palsu lokal yang memutus koneksi."""

import hashlib
import http.client
import json
import os
import threading
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest
from labcli import regproxy as P

BODY = os.urandom(300_000)
SHA512 = "sha512-" + __import__("base64").b64encode(hashlib.sha512(BODY).digest()).decode()


class Upstream:
    """Memutus koneksi setelah `cut` byte pada `cuts` permintaan pertama; mendukung Range kecuali ignore_range."""

    def __init__(self, cut=100_000, cuts=3, ignore_range=False, body=BODY, bad_status=None):
        self.cut, self.cuts, self.ignore_range, self.body, self.bad_status = cut, cuts, ignore_range, body, bad_status
        self.requests = []
        up = self

        class H(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *a):
                pass

            def do_GET(self):
                up.requests.append((self.path, self.headers.get("Range")))
                if up.bad_status:
                    self.send_response(up.bad_status)
                    self.send_header("Content-Length", "0")
                    self.end_headers()
                    return
                if self.path.endswith(".tgz"):
                    start, status = 0, 200
                    rg = self.headers.get("Range")
                    if rg and not up.ignore_range:
                        start, status = int(rg.split("=")[1].rstrip("-")), 206
                    chunk = up.body[start:]
                    self.send_response(status)
                    self.send_header("Content-Length", str(len(chunk)))
                    if status == 206:
                        self.send_header("Content-Range", f"bytes {start}-{len(up.body) - 1}/{len(up.body)}")
                    self.end_headers()
                    if len([r for r in up.requests if r[0].endswith(".tgz")]) <= up.cuts:
                        self.wfile.write(chunk[: up.cut])
                        self.wfile.flush()
                        self.close_connection = True
                        return
                    self.wfile.write(chunk)
                    return
                doc = {"name": "pk", "versions": {"1.0.0": {"dist": {"tarball": f"{up.base}/pk/-/pk-1.0.0.tgz", "integrity": SHA512}}}}
                raw = json.dumps(doc).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

        self.srv = ThreadingHTTPServer(("127.0.0.1", 0), H)
        self.base = f"http://127.0.0.1:{self.srv.server_address[1]}"
        threading.Thread(target=self.srv.serve_forever, daemon=True).start()

    def close(self):
        self.srv.shutdown()


@pytest.fixture
def stack(tmp_path):
    made = []

    def build(**kw):
        up = Upstream(**kw)
        px = P.make_server(port=0, upstream=up.base, cache=tmp_path / "c", _testing=True, sleep=lambda s: None)
        threading.Thread(target=px.serve_forever, daemon=True).start()
        made.append((up, px))
        return up, px, f"http://127.0.0.1:{px.server_address[1]}"

    yield build
    for up, px in made:
        px.shutdown()
        up.close()


def fetch(url, timeout=20):
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return r.read()


def test_resumes_after_repeated_cuts_and_matches_bytes(stack):
    up, px, url = stack(cut=100_000, cuts=3)
    got = fetch(url + "/pk/-/pk-1.0.0.tgz")
    assert got == BODY
    ranges = [r[1] for r in up.requests if r[0].endswith(".tgz")]
    assert ranges[0] is None and all(r and r.startswith("bytes=") for r in ranges[1:]), ranges
    assert len(ranges) >= 3, "harus mencoba ulang dengan Range, bukan dari awal"


def test_metadata_tarball_rewritten_to_loopback_proxy(stack):
    up, px, url = stack()
    doc = json.loads(fetch(url + "/pk"))
    t = doc["versions"]["1.0.0"]["dist"]["tarball"]
    assert t.startswith(f"http://127.0.0.1:{px.server_address[1]}/pk/-/"), t
    assert up.base not in t


def test_second_request_served_from_cache(stack):
    up, px, url = stack(cuts=0)
    fetch(url + "/pk")
    assert fetch(url + "/pk/-/pk-1.0.0.tgz") == BODY
    n = len(up.requests)
    assert fetch(url + "/pk/-/pk-1.0.0.tgz") == BODY
    assert len(up.requests) == n, "cache: tanpa permintaan upstream baru"


def test_integrity_mismatch_rejected_and_not_cached(stack):
    up, px, url = stack(cuts=0)
    fetch(url + "/pk")
    up.body = os.urandom(len(BODY))  # isi berbeda dari yang dijanjikan metadata
    with pytest.raises((urllib.error.URLError, http.client.HTTPException, OSError)):
        fetch(url + "/pk/-/pk-1.0.0.tgz")  # klien TIDAK boleh menerima berkas utuh yang integritasnya salah
    files = list((px.manager.cache / "files").glob("*")) if (px.manager.cache / "files").exists() else []
    assert not [f for f in files if f.suffix != ".part"], "berkas yang integritasnya salah tidak boleh masuk cache"


def test_server_ignoring_range_restarts_cleanly(stack):
    up, px, url = stack(cut=100_000, cuts=2, ignore_range=True)
    got = None
    for _ in range(4):  # klien sungguhan mengulang saat koneksi diputus di tengah
        try:
            got = fetch(url + "/pk/-/pk-1.0.0.tgz", timeout=30)
            break
        except (http.client.HTTPException, OSError):
            continue
    assert got == BODY


def test_upstream_404_is_not_retried_forever(stack):
    up, px, url = stack(bad_status=404)
    with pytest.raises(urllib.error.HTTPError) as e:
        fetch(url + "/pk/-/pk-1.0.0.tgz")
    assert e.value.code == 502
    assert len(up.requests) <= 2


def test_many_clients_one_upstream_download(stack):
    up, px, url = stack(cut=100_000, cuts=2)
    out = []
    ts = [threading.Thread(target=lambda: out.append(fetch(url + "/pk/-/pk-1.0.0.tgz"))) for _ in range(5)]
    [t.start() for t in ts]
    [t.join(30) for t in ts]
    assert len(out) == 5 and all(o == BODY for o in out)
    assert px.manager.starts == 1


def test_client_disconnect_does_not_stop_download(stack):
    up, px, url = stack(cut=50_000, cuts=2)
    host, port = url.replace("http://", "").split(":")
    c = http.client.HTTPConnection(host, int(port), timeout=10)
    c.request("GET", "/pk/-/pk-1.0.0.tgz")
    r = c.getresponse()
    r.read(1000)
    c.close()  # klien menyerah di tengah
    assert fetch(url + "/pk/-/pk-1.0.0.tgz") == BODY, "percobaan berikutnya berhasil dari unduhan yang tetap berjalan"


def test_rejects_bad_methods_paths_and_absolute_uri(stack):
    up, px, url = stack()
    host, port = url.replace("http://", "").split(":")
    for method in ("POST", "PUT", "DELETE"):
        c = http.client.HTTPConnection(host, int(port), timeout=5)
        c.request(method, "/pk", body=b"x")
        assert c.getresponse().status == 405
    for path in ("/../etc/passwd", "/a//b", "/a%20b", "/%2e%2e/x", "/" + "a/" * 6 + "z"):
        c = http.client.HTTPConnection(host, int(port), timeout=5)
        c.putrequest("GET", path)
        c.endheaders()
        assert c.getresponse().status in (400, 404), path
    c = http.client.HTTPConnection(host, int(port), timeout=5)
    c.putrequest("GET", "http://evil.example/pk", skip_host=True)
    c.putheader("Host", "evil.example")
    c.endheaders()
    assert c.getresponse().status == 400
    assert up.requests == [], "tidak ada permintaan sah yang diteruskan"


def test_only_allowed_upstreams(tmp_path):
    with pytest.raises(ValueError):
        P.make_server(port=0, upstream="https://evil.example", cache=tmp_path)
    assert P.make_server(port=0, upstream="https://registry.yarnpkg.com", cache=tmp_path).server_close() is None


def test_binds_loopback_only(tmp_path):
    s = P.make_server(port=0, cache=tmp_path)
    assert s.server_address[0] == "127.0.0.1"
    s.server_close()


def test_parse_integrity():
    assert P.parse_integrity(SHA512)[0] == "sha512"
    assert P.parse_integrity("sha1-" + "QUJD") == ("sha1", b"ABC")
    assert P.parse_integrity("a" * 40)[0] == "sha1"
    for bad in (None, "", "md5-abc", "sha512-!!!", 5, "zz" * 20):
        assert P.parse_integrity(bad) is None


def test_prune_old_partial_and_files(tmp_path):
    m = P.Manager(tmp_path)
    d = tmp_path / "files"
    d.mkdir()
    old_part, new_part, old_file = d / "a.part", d / "b.part", d / "c"
    for f in (old_part, new_part, old_file):
        f.write_bytes(b"x")
    now = 10_000_000
    os.utime(old_part, (now - P.PART_MAX_AGE - 5, now - P.PART_MAX_AGE - 5))
    os.utime(new_part, (now - 10, now - 10))
    os.utime(old_file, (now - P.FILE_MAX_AGE - 5, now - P.FILE_MAX_AGE - 5))
    assert m.prune(now) == 2 and new_part.exists() and not old_part.exists() and not old_file.exists()


def test_env_overrides_only_when_enabled_and_healthy(monkeypatch):
    monkeypatch.setattr(P, "load_config", lambda: {"enabled": False, "upstream": P.ALLOWED_UPSTREAMS[0]})
    monkeypatch.setattr(P, "healthy", lambda *a, **k: True)
    assert P.env_overrides() == {}
    monkeypatch.setattr(P, "load_config", lambda: {"enabled": True, "upstream": P.ALLOWED_UPSTREAMS[0]})
    monkeypatch.setattr(P, "healthy", lambda *a, **k: False)
    assert P.env_overrides() == {}, "proxy mati: langsung ke registry, jangan memutus instalasi"
    monkeypatch.setattr(P, "healthy", lambda *a, **k: True)
    assert P.env_overrides() == {"npm_config_registry": f"http://127.0.0.1:{P.PORT}/"}


def test_single_version_document_is_rewritten_too(stack):
    up, px, url = stack()
    doc = {}
    P.rewrite_packument(doc, up.base, url, {})
    one = {"name": "pk", "dist": {"tarball": f"{up.base}/pk/-/pk-1.0.0.tgz", "integrity": SHA512}}
    integ = {}
    P.rewrite_packument(one, up.base, url, integ)
    assert one["dist"]["tarball"] == f"{url}/pk/-/pk-1.0.0.tgz"
    assert integ["/pk/-/pk-1.0.0.tgz"] == SHA512


def test_foreign_tarball_hosts_are_left_alone():
    d = {"versions": {"1": {"dist": {"tarball": "https://evil.example/x.tgz"}}}}
    P.rewrite_packument(d, "https://registry.npmjs.org", "http://127.0.0.1:1", {})
    assert d["versions"]["1"]["dist"]["tarball"] == "https://evil.example/x.tgz", "hanya host upstream yang ditulis ulang"
