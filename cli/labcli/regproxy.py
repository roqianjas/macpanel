"""lab proxy: proxy registry npm lokal yang MELANJUTKAN unduhan yang terputus.

Masalah yang diselesaikan: jaringan sering memutus unduhan besar (mis. tarball `next` 43,7 MB berhenti di ~25 MB dengan
ECONNRESET). pnpm dan npm mengulang dari awal dan tidak melanjutkan, sehingga instalasi gagal berulang. `curl -C -`
terbukti melanjutkan dan hasilnya identik dengan integritas SHA-512 registry. Proxy ini membawa perilaku itu ke pnpm/npm.

Cara kerja (hanya stdlib, bind 127.0.0.1):
- GET /<paket>: diteruskan ke registry; `dist.tarball` ditulis ulang agar menunjuk ke proxy ini.
- GET /<paket>/-/<berkas>.tgz: diunduh ke cache dengan `Range` dan diulang sampai utuh, sambil dialirkan ke klien.
  Bila klien putus atau timeout, unduhan TETAP berjalan; percobaan klien berikutnya langsung memakai cache.
  Setengah-unduhan bertahan di disk (juga setelah proxy dimulai ulang). Integritas (sha512/sha1) diperiksa bila diketahui
  dari metadata; klien tetap memeriksa lockfile-nya sendiri.
- Hanya GET, hanya path npm yang sah, hanya ke registry yang diizinkan. Bukan proxy umum.
"""

import base64
import gzip
import hashlib
import http.client
import json
import os
import plistlib
import re
import shutil
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from labcli.core import ENV, ROOT, die

PORT = 4873
ALLOWED_UPSTREAMS = ("https://registry.npmjs.org", "https://registry.yarnpkg.com")
CACHE_DIR = Path.home() / ".cache/macpanel/regproxy"
CONFIG = Path.home() / ".config/macminiops/regproxy.json"
LABEL = "com.macminiops.regproxy"
PLIST = Path.home() / "Library/LaunchAgents" / f"{LABEL}.plist"
LOG = Path.home() / "Library/Logs/regproxy.log"
SEG = r"(?:[A-Za-z0-9@._~+-]|%2[fF])+"  # satu-satunya persen-encoding yang sah di path npm: %2f (paket ber-scope)
PATH_OK = re.compile(rf"^/{SEG}(/{SEG}){{0,3}}$")
CHUNK = 64 * 1024
PART_MAX_AGE = 2 * 86400
FILE_MAX_AGE = 30 * 86400


# ---------------- unduhan yang bisa dilanjutkan ----------------


def parse_integrity(s):
    """'sha512-<base64>' atau 'sha1-<base64>' atau 40 heks (shasum) -> (algoritma, bytes) atau None."""
    if not s or not isinstance(s, str):
        return None
    m = re.match(r"^(sha512|sha384|sha256|sha1)-([A-Za-z0-9+/=]+)$", s.strip())
    if m:
        try:
            return m.group(1), base64.b64decode(m.group(2))
        except ValueError:
            return None
    if re.fullmatch(r"[0-9a-fA-F]{40}", s.strip()):
        return "sha1", bytes.fromhex(s.strip())
    return None


def file_digest(path, algo):
    h = hashlib.new(algo)
    with open(path, "rb") as f:
        while chunk := f.read(1 << 20):
            h.update(chunk)
    return h.digest()


class Download:
    """Satu unduhan yang diulang dengan Range sampai utuh. Aman dibaca banyak thread (Condition)."""

    def __init__(self, url, dest, integrity=None):
        self.url, self.dest = url, Path(dest)
        self.part = self.dest.with_name(self.dest.name + ".part")
        self.integrity = parse_integrity(integrity)
        self.total = None
        self.have = 0
        self.done = False
        self.error = None
        self.gen = 0  # naik saat unduhan terpaksa mulai dari nol (server mengabaikan Range)
        self.attempts = 0
        self.cond = threading.Condition()

    def _set(self, **kw):
        with self.cond:
            for k, v in kw.items():
                setattr(self, k, v)
            self.cond.notify_all()

    def run(self, max_attempts=60, timeout=30, sleep=time.sleep, backoff=2.0, opener=None):
        opener = opener or urllib.request.build_opener()
        self.dest.parent.mkdir(parents=True, exist_ok=True)
        for attempt in range(1, max_attempts + 1):
            self.attempts = attempt
            have = self.part.stat().st_size if self.part.exists() else 0
            headers = {"User-Agent": "macpanel-regproxy", "Accept-Encoding": "identity"}
            if have:
                headers["Range"] = f"bytes={have}-"
            try:
                with opener.open(urllib.request.Request(self.url, headers=headers), timeout=timeout) as r:
                    status = r.status
                    clen = int(r.headers.get("Content-Length") or -1)
                    if have and status == 200:  # Range diabaikan: mulai dari nol
                        have = 0
                        self.part.unlink(missing_ok=True)
                        self._set(gen=self.gen + 1, have=0)
                    elif have and status == 206:
                        m = re.match(r"^bytes (\d+)-\d+/(\d+|\*)$", r.headers.get("Content-Range") or "")
                        if not m or int(m.group(1)) != have:
                            have = 0
                            self.part.unlink(missing_ok=True)
                            self._set(gen=self.gen + 1, have=0)
                            raise OSError("Content-Range tidak cocok, mulai ulang")
                        total = int(m.group(2)) if m.group(2).isdigit() else None
                        if total is not None:
                            self._set(total=total)
                    if status == 200 and clen >= 0:
                        self._set(total=clen)
                    elif status == 206 and self.total is None and clen >= 0:
                        self._set(total=have + clen)
                    with open(self.part, "ab" if have else "wb") as f:
                        broken = False
                        while True:
                            try:
                                chunk = r.read(CHUNK)
                            except http.client.IncompleteRead as e:
                                chunk, broken = e.partial, True
                            if chunk:
                                f.write(chunk)
                                f.flush()
                                have += len(chunk)
                                self._set(have=have)
                            if broken or not chunk:
                                break
            except urllib.error.HTTPError as e:
                if e.code == 416 and self.total is not None and have == self.total:
                    pass  # sudah lengkap: lanjut ke verifikasi
                elif e.code == 416:
                    self.part.unlink(missing_ok=True)
                    self._set(have=0, gen=self.gen + 1)
                    continue
                elif 400 <= e.code < 500 and e.code != 429:
                    self._set(error=f"upstream HTTP {e.code}")
                    return False
                else:
                    sleep(min(backoff * attempt, 15))
                    continue
            except (OSError, http.client.HTTPException):
                sleep(min(backoff * attempt, 15))
                continue
            have = self.part.stat().st_size if self.part.exists() else 0
            if self.total is not None and have == self.total:
                return self._finish()
            if self.total is not None and have > self.total:
                self.part.unlink(missing_ok=True)
                self._set(have=0, gen=self.gen + 1)
                continue
            if self.total is None and have > 0:  # server tidak menyebut ukuran dan tutup bersih: anggap utuh
                self._set(total=have)
                return self._finish()
            sleep(min(backoff * attempt, 15))
        self._set(error=f"gagal setelah {max_attempts} percobaan")
        return False

    def _finish(self):
        if self.integrity:
            algo, want = self.integrity
            if file_digest(self.part, algo) != want:
                self.part.unlink(missing_ok=True)
                self._set(error="integritas tidak cocok dengan registry; berkas dibuang", have=0)
                return False
        os.replace(self.part, self.dest)
        self._set(done=True)
        return True


class Manager:
    def __init__(self, cache, opener=None, **run_kw):
        self.cache = Path(cache)
        self.opener, self.run_kw = opener, run_kw
        self.lock = threading.Lock()
        self.active = {}
        self.starts = 0  # berapa unduhan upstream dimulai (untuk tes: banyak klien, satu unduhan)

    def dest_for(self, path):
        return self.cache / "files" / hashlib.sha256(path.encode()).hexdigest()

    def get_opener(self):
        return self.opener or urllib.request.build_opener()

    def start(self, url, path, integrity=None):
        dest = self.dest_for(path)
        with self.lock:
            d = self.active.get(path)
            if d and not d.error:
                return d
            d = Download(url, dest, integrity)
            self.active[path] = d
            self.starts += 1
        threading.Thread(target=d.run, kwargs={"opener": self.get_opener(), **self.run_kw}, daemon=True).start()
        return d

    def prune(self, now=None):
        now = now or time.time()
        n = 0
        d = self.cache / "files"
        if not d.is_dir():
            return 0
        for f in d.iterdir():
            age = now - f.stat().st_mtime
            if (f.suffix == ".part" and age > PART_MAX_AGE) or (f.suffix != ".part" and age > FILE_MAX_AGE):
                f.unlink(missing_ok=True)
                n += 1
        return n


# ---------------- metadata ----------------


def _rewrite_dist(dist, up, base, integ):
    if not isinstance(dist, dict) or not isinstance(dist.get("tarball"), str):
        return
    t = dist["tarball"]
    if t.startswith(up + "/"):
        path = t[len(up) :]
        integ[path] = dist.get("integrity") or dist.get("shasum")
        dist["tarball"] = base.rstrip("/") + path


def rewrite_packument(doc, upstream, base, integ):
    """Tulis ulang dist.tarball yang menunjuk ke upstream agar menunjuk ke `base`, dan catat integritasnya per path.
    Menangani dokumen paket penuh/ringkas (`versions`) dan dokumen satu versi (`dist` di tingkat atas)."""
    up = upstream.rstrip("/")
    if not isinstance(doc, dict):
        return doc
    versions = doc.get("versions")
    if isinstance(versions, dict):
        for v in versions.values():
            if isinstance(v, dict):
                _rewrite_dist(v.get("dist"), up, base, integ)
    _rewrite_dist(doc.get("dist"), up, base, integ)
    return doc


def is_tarball(path):
    return "/-/" in path and path.endswith(".tgz")


def path_ok(path):
    return bool(PATH_OK.match(path)) and ".." not in path and "//" not in path


# ---------------- server ----------------


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server_version = "macpanel-regproxy"

    @property
    def srv(self):
        return self.server  # bertipe Server (lihat make_server)

    def log_message(self, *a):  # log akses tidak dicatat: tidak ada yang berguna dan nama paket bukan rahasia tetapi bising
        pass

    def _send(self, code, body=b"", ctype="text/plain; charset=utf-8", extra=None):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        for k, v in (extra or {}).items():
            self.send_header(k, v)
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def do_GET(self):
        if self.path == "/_health":
            return self._send(200, b"ok")
        sp = urllib.parse.urlsplit(self.path)
        if sp.scheme or sp.netloc:
            return self._send(400, b"permintaan absolute-URI ditolak")
        if not path_ok(sp.path):
            return self._send(400, b"path tidak sah")
        try:
            if is_tarball(sp.path):
                return self._tarball(sp.path)
            return self._metadata(sp.path)
        except (BrokenPipeError, ConnectionResetError):
            return None

    do_HEAD = do_GET

    def _metadata(self, path):
        srv = self.srv
        url = srv.upstream.rstrip("/") + path
        accept = self.headers.get("Accept", "application/json")
        last = "tidak diketahui"
        for attempt in range(4):
            try:
                req = urllib.request.Request(url, headers={"Accept": accept, "User-Agent": "macpanel-regproxy", "Accept-Encoding": "gzip"})
                with srv.manager.get_opener().open(req, timeout=30) as r:
                    raw = r.read()
                    if r.headers.get("Content-Encoding") == "gzip":
                        raw = gzip.decompress(raw)
                    ctype = r.headers.get("Content-Type", "application/json")
                break
            except urllib.error.HTTPError as e:
                if 400 <= e.code < 500 and e.code != 429:
                    return self._send(e.code, f"upstream HTTP {e.code}".encode())
                last = f"HTTP {e.code}"
            except (OSError, http.client.HTTPException) as e:
                last = type(e).__name__
            time.sleep(min(1.5 * (attempt + 1), 5))
        else:
            return self._send(502, f"registry tidak terjangkau ({last})".encode())
        base = f"http://127.0.0.1:{srv.server_address[1]}"  # selalu alamat loopback kita, bukan Host dari klien
        try:
            doc = json.loads(raw)
        except ValueError:
            return self._send(200, raw, ctype)
        doc = rewrite_packument(doc, srv.upstream, base, srv.integ)
        self._send(200, json.dumps(doc, separators=(",", ":")).encode(), "application/json")

    def _tarball(self, path):
        srv = self.srv
        dest = srv.manager.dest_for(path)
        if dest.exists():
            return self._serve_file(dest)
        url = srv.upstream.rstrip("/") + path
        d = srv.manager.start(url, path, srv.integ.get(path))
        with d.cond:
            d.cond.wait_for(lambda: d.total is not None or d.error or d.done, timeout=90)
        if d.error or d.total is None:
            return self._send(502, (d.error or "upstream tidak menjawab ukuran berkas").encode())
        self.send_response(200)
        self.send_header("Content-Type", "application/octet-stream")
        self.send_header("Content-Length", str(d.total))
        self.end_headers()
        if self.command == "HEAD":
            return
        gen, sent = d.gen, 0
        hold = 1 if d.integrity else 0  # byte terakhir ditahan sampai verifikasi lulus: berkas yang salah tidak pernah tampak lengkap bagi klien
        while sent < d.total:
            path_now = d.dest if d.done else d.part
            try:
                f = open(path_now, "rb")
            except FileNotFoundError:
                f = open(d.dest, "rb") if d.dest.exists() else None
            if f is None:
                if d.error:
                    break
                with d.cond:
                    d.cond.wait(timeout=1)
                continue
            with f:
                f.seek(sent)
                while sent < d.total:
                    if d.gen != gen or d.error:
                        self.close_connection = True  # klien akan mengulang; potongan lama tidak boleh bercampur dengan yang baru
                        return
                    avail = (d.total if d.done else max(d.have - hold, 0)) - sent
                    if avail <= 0:
                        with d.cond:
                            d.cond.wait(timeout=2)
                        if d.done and sent < d.total:
                            break  # file dipindah (rename); buka ulang dari dest
                        continue
                    chunk = f.read(min(CHUNK, avail))
                    if not chunk:
                        with d.cond:
                            d.cond.wait(timeout=1)
                        continue
                    self.wfile.write(chunk)
                    sent += len(chunk)
        if sent < d.total:
            self.close_connection = True

    def _serve_file(self, dest):
        size = dest.stat().st_size
        self.send_response(200)
        self.send_header("Content-Type", "application/octet-stream")
        self.send_header("Content-Length", str(size))
        self.end_headers()
        if self.command == "HEAD":
            return
        with open(dest, "rb") as f:
            shutil.copyfileobj(f, self.wfile, CHUNK)

    def _no(self):
        try:
            n = min(int(self.headers.get("Content-Length") or 0), 1 << 20)
            if n > 0:
                self.rfile.read(n)
        except (ValueError, OSError):
            pass
        self.close_connection = True
        self._send(405, b"hanya GET", extra={"Allow": "GET, HEAD"})

    do_POST = do_PUT = do_DELETE = do_PATCH = do_OPTIONS = _no


class Server(ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = True

    def __init__(self, addr, upstream, cache, opener=None, **run_kw):
        super().__init__(addr, Handler)
        self.upstream = upstream
        self.integ = {}
        self.manager = Manager(cache, opener=opener, **run_kw)


def make_server(port=PORT, upstream=ALLOWED_UPSTREAMS[0], cache=CACHE_DIR, **kw):
    if upstream.rstrip("/") not in ALLOWED_UPSTREAMS and not kw.pop("_testing", False):
        raise ValueError("registry tidak diizinkan: " + upstream)
    kw.pop("_testing", None)
    Path(cache).mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(cache, 0o700)
    return Server(("127.0.0.1", port), upstream, cache, **kw)


# ---------------- konfigurasi dan perintah ----------------


def load_config():
    try:
        d = json.loads(CONFIG.read_text())
    except (OSError, ValueError):
        d = {}
    up = d.get("upstream") if d.get("upstream") in ALLOWED_UPSTREAMS else ALLOWED_UPSTREAMS[0]
    return {"enabled": d.get("enabled") is True, "upstream": up}


def healthy(port=PORT, timeout=1.5):
    try:
        with urllib.request.urlopen(f"http://127.0.0.1:{port}/_health", timeout=timeout) as r:
            return r.status == 200 and r.read() == b"ok"
    except (OSError, http.client.HTTPException):
        return False


def env_overrides():
    """Untuk subproses npm/pnpm milik lab: arahkan ke proxy BILA diaktifkan dan sehat; selain itu {} (langsung ke registry)."""
    if load_config()["enabled"] and healthy():
        return {"npm_config_registry": f"http://127.0.0.1:{PORT}/"}
    return {}


def cache_stats(cache=CACHE_DIR):
    d = Path(cache) / "files"
    files = [f for f in d.iterdir()] if d.is_dir() else []
    done = [f for f in files if f.suffix != ".part"]
    part = [f for f in files if f.suffix == ".part"]
    return {"files": len(done), "bytes": sum(f.stat().st_size for f in done), "partial": len(part), "partial_bytes": sum(f.stat().st_size for f in part)}


def _plist():
    return {
        "Label": LABEL,
        "ProgramArguments": [sys.executable, str(ROOT / "server/scripts/lab"), "proxy", "serve"],
        "RunAtLoad": True,
        "KeepAlive": True,
        "ThrottleInterval": 15,
        "EnvironmentVariables": {"PATH": ENV["PATH"], "HOME": str(Path.home())},
        "StandardOutPath": str(LOG),
        "StandardErrorPath": str(LOG),
    }


def _launchctl(*a):
    return subprocess.run(["/bin/launchctl", *a], capture_output=True, text=True, timeout=30)


def proxy_enable():
    PLIST.parent.mkdir(parents=True, exist_ok=True)
    LOG.parent.mkdir(parents=True, exist_ok=True)
    cfg = load_config()
    CONFIG.parent.mkdir(parents=True, exist_ok=True)
    CONFIG.write_text(json.dumps({"enabled": True, "upstream": cfg["upstream"]}))
    os.chmod(CONFIG, 0o600)
    PLIST.write_bytes(plistlib.dumps(_plist()))
    uid = os.getuid()
    _launchctl("bootout", f"gui/{uid}/{LABEL}")
    r = _launchctl("bootstrap", f"gui/{uid}", str(PLIST))
    if r.returncode:
        die("launchctl bootstrap gagal: " + (r.stderr or r.stdout).strip()[-200:])
    for _ in range(40):
        if healthy():
            return f"aktif di http://127.0.0.1:{PORT}/ (registry {cfg['upstream']}, cache {CACHE_DIR})"
        time.sleep(0.5)
    die("proxy tidak menjawab setelah dinyalakan; lihat " + str(LOG))


def proxy_disable():
    _launchctl("bootout", f"gui/{os.getuid()}/{LABEL}")
    PLIST.unlink(missing_ok=True)
    cfg = load_config()
    CONFIG.parent.mkdir(parents=True, exist_ok=True)
    CONFIG.write_text(json.dumps({"enabled": False, "upstream": cfg["upstream"]}))
    for _ in range(20):
        if not healthy(timeout=0.5):
            return "nonaktif (cache dipertahankan; hapus dengan: lab proxy purge)"
        time.sleep(0.3)
    die("proxy masih menjawab setelah dimatikan")


def proxy_purge():
    n = cache_stats()
    shutil.rmtree(CACHE_DIR / "files", ignore_errors=True)
    return f"cache dibersihkan: {n['files']} berkas ({n['bytes']} byte) dan {n['partial']} setengah-unduhan"


def proxy_status():
    cfg = load_config()
    return {"enabled": cfg["enabled"], "upstream": cfg["upstream"], "running": healthy(), "port": PORT, "cache": cache_stats(), "cache_dir": str(CACHE_DIR)}


def proxy_serve():
    cfg = load_config()
    srv = make_server(upstream=cfg["upstream"])
    n = srv.manager.prune()
    print(f"regproxy mendengar di 127.0.0.1:{PORT} -> {cfg['upstream']} (pangkas {n} berkas lama)", flush=True)
    try:
        srv.serve_forever()
    except KeyboardInterrupt:
        pass


def proxy_cmd(a):
    if a.mode == "serve":
        return proxy_serve()
    if a.mode == "status":
        print(json.dumps(proxy_status(), indent=2))
        return None
    msg = {"enable": proxy_enable, "disable": proxy_disable, "purge": proxy_purge}[a.mode]()
    print("OK:", msg)
    return None
