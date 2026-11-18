"""lab appinstall / siteclone: pemasang satu klik (Laravel, WordPress) dan kloning situs."""

import json
import os
import re
import secrets
import shutil
import subprocess
import time
from pathlib import Path

from labcli.core import (
    APPS_DIR,
    ENV,
    PHPS,
    SITE_NAME_RE,
    die,
    lab,
    need,
    site_exists,
    stream,
)
from labcli.dbcfg import DBPORT
from labcli.deploy import project
from labcli.phpenv import COMPOSER, php_binary, php_home
from labcli.projects import PROJ_KINDS, app_project


def set_env_kv(text, kv):
    """Ganti/aktifkan baris KEY=... (termasuk yang dikomentari); tambahkan bila tidak ada."""
    for k, v in kv.items():
        val = v if re.fullmatch(r"[A-Za-z0-9_./:@+-]*", v) else '"' + v.replace("\\", "\\\\").replace('"', '\\"') + '"'
        pat = re.compile(rf"^#?\s*{re.escape(k)}=.*$", re.MULTILINE)
        text = pat.sub(lambda m: f"{k}={val}", text, count=1) if pat.search(text) else text.rstrip("\n") + f"\n{k}={val}\n"
    return text


def rollback(dest, dbs):
    if dest and dest.exists() and APPS_DIR in dest.parents:
        shutil.rmtree(dest, ignore_errors=True)
    for n, eng in dbs:
        lab("db", "drop", n, "--engine", eng, inp=n + "\n")


def app_install(a):
    if a.kind in PROJ_KINDS:
        return app_project(a)
    n = a.name
    if not SITE_NAME_RE.fullmatch(n or ""):
        die("nama: huruf kecil, angka, tanda hubung (maks 30)")
    if n in ("home", "panel", "db", "nginx", "kuma", "stats", "dev", "mysql", "postgres", "root"):
        die("nama itu dicadangkan")
    v = a.php
    if v not in PHPS:
        die(f"versi PHP: {', '.join(PHPS)}")
    if not php_home(v).exists():
        die(f"PHP {v} belum terpasang")
    dest = APPS_DIR / n
    if dest.exists() or site_exists(n):
        die("nama sudah dipakai (folder atau situs)")
    APPS_DIR.mkdir(parents=True, exist_ok=True)
    dbs = []
    try:
        print("== database ==", flush=True)
        deng = a.db
        need(lab("db", "create", n, "--engine", deng), "buat database")
        dbs.append((n, deng))
        import json as _j

        cred = _j.loads(need(lab("db", "creds", n, "--engine", deng), "baca kredensial"))
        kv = {"DB_HOST": "127.0.0.1", "DB_PORT": str(DBPORT[deng]), "DB_DATABASE": n, "DB_USERNAME": cred["DB_USERNAME"], "DB_PASSWORD": cred["DB_PASSWORD"]}
        php = php_binary(v)
        env = dict(ENV, COMPOSER_NO_INTERACTION="1", COMPOSER_ALLOW_SUPERUSER="0")
        if a.kind == "laravel":
            print("== composer create-project laravel/laravel (bisa beberapa menit) ==", flush=True)
            for attempt in range(3):
                if not stream([php, COMPOSER, "create-project", "laravel/laravel", str(dest), "--no-interaction", "--prefer-dist"], env=env):
                    break
                shutil.rmtree(dest, ignore_errors=True)
                if attempt == 2:
                    die("composer create-project gagal setelah 3 percobaan (cek koneksi internet)")
                print(f"percobaan {attempt + 1} gagal, mengulang…", flush=True)
                time.sleep(4)
            ef = dest / ".env"
            ef.write_text(set_env_kv(ef.read_text(), dict(kv, DB_CONNECTION="mysql", APP_URL=f"https://{n}.lab", SESSION_DRIVER="file", CACHE_STORE="file", QUEUE_CONNECTION="sync")))
            os.chmod(ef, 0o600)
            print("== artisan migrate ==", flush=True)
            if stream([php, "artisan", "migrate", "--force"], cwd=str(dest), env=env):
                die("artisan migrate gagal")
            root = dest / "public"
        else:
            print("== unduh WordPress dari wordpress.org ==", flush=True)
            tgz = Path(f"/tmp/wp-{os.getpid()}.tar.gz")
            try:
                r = subprocess.run(["/usr/bin/curl", "-fsSL", "--retry", "4", "--retry-all-errors", "-o", str(tgz), "https://wordpress.org/latest.tar.gz"], env=ENV, text=True, capture_output=True)
                if r.returncode:
                    die("unduh gagal: " + (r.stderr or "").strip()[-200:])
                dest.mkdir(mode=0o755)
                need(subprocess.run(["/usr/bin/tar", "-xzf", str(tgz), "-C", str(dest), "--strip-components=1"], text=True, capture_output=True), "ekstrak")
            finally:
                tgz.unlink(missing_ok=True)
            sample = (dest / "wp-config-sample.php").read_text()

            def q(x):
                return x.replace("\\", "\\\\").replace("'", "\\'")

            for key, val in (("DB_NAME", n), ("DB_USER", kv["DB_USERNAME"]), ("DB_PASSWORD", kv["DB_PASSWORD"]), ("DB_HOST", "127.0.0.1")):
                sample = re.sub(rf"define\(\s*'{key}',\s*'[^']*'\s*\);", lambda m: f"define( '{key}', '{q(val)}' );", sample)
            sample = re.sub(r"'put your unique phrase here'", lambda m: "'" + secrets.token_urlsafe(48) + "'", sample)
            wc = dest / "wp-config.php"
            wc.write_text(sample)
            os.chmod(wc, 0o600)
            root = dest
        print("== daftarkan situs ==", flush=True)
        need(lab("site", "add", n, str(root), "--php", v, *(["--wordpress"] if a.kind == "wordpress" else [])), "tambah situs")
    except SystemExit:
        rollback(dest, dbs)
        print("dibatalkan: folder dan database yang baru dibuat dibersihkan")
        raise
    print(f"OK: {a.kind} terpasang di {dest}. Buka https://{n}.lab" + (" dan selesaikan wizard WordPress (kredensial database sudah terisi)." if a.kind == "wordpress" else "."))


def site_clone(a):
    src, dst = a.src, a.dst
    if not SITE_NAME_RE.fullmatch(dst or ""):
        die("nama tujuan: huruf kecil, angka, tanda hubung (maks 30)")
    if dst in ("home", "panel", "db", "nginx", "kuma", "stats", "dev", "mysql", "postgres", "root"):
        die("nama itu dicadangkan")
    d, v = project(src)
    if dst == src or site_exists(dst) or (APPS_DIR / dst).exists():
        die("nama tujuan sudah dipakai")
    if APPS_DIR in d.parents or d.parent == APPS_DIR or True:
        pass
    out = APPS_DIR / dst
    APPS_DIR.mkdir(parents=True, exist_ok=True)
    dbs = []
    try:
        print("== salin berkas (tanpa .git, node_modules, log) ==", flush=True)
        r = subprocess.run(
            ["/usr/bin/rsync", "-a", "--exclude=.git", "--exclude=node_modules", "--exclude=storage/logs/*", "--exclude=.env.bak.*", "--exclude=*.log", f"{d}/", f"{out}/"],
            env=ENV,
            text=True,
            capture_output=True,
        )
        if r.returncode:
            die("salin gagal: " + (r.stderr or "").strip()[-200:])
        envf = out / ".env"
        if envf.exists():
            text = envf.read_text()
            m = re.search(r"^DB_DATABASE=(.*)$", text, re.MULTILINE)
            conn = (re.search(r"^DB_CONNECTION=(\S+)", text, re.MULTILINE) or [None, "mysql"])[1]
            pm = re.search(r"^DB_PORT=(\d+)", text, re.MULTILINE)
            eng = "pgsql" if conn in ("pgsql", "postgres", "postgresql") else ("mysql" if pm and pm.group(1) == str(DBPORT["mysql"]) else "mariadb")
            if m and m.group(1).strip().strip("\"'"):
                srcdb = m.group(1).strip().strip("\"'")
                print("== database baru untuk kloning ==", flush=True)
                need(lab("db", "create", dst, "--engine", eng), "buat database")
                dbs.append((dst, eng))
                cred = json.loads(need(lab("db", "creds", dst, "--engine", eng), "baca kredensial"))
                if a.with_db:
                    if SITE_NAME_RE.fullmatch(srcdb) and lab("db", "users", srcdb, "--engine", eng).returncode == 0:
                        o = need(lab("db", "dump", srcdb, "--engine", eng), "dump database sumber")
                        f = o.split("OK:")[1].split("(")[0].strip()
                        nf = str(Path(f).with_name(Path(f).name.replace(f"{srcdb}-{eng}-", f"{dst}-{eng}-", 1)))
                        os.rename(f, nf)
                        f = nf  # restore hanya menerima dump bernama sesuai database tujuan
                        need(lab("db", "restore", dst, "--engine", eng, "--file", f), "pulihkan ke database baru")
                        try:
                            os.remove(f)
                        except OSError:
                            pass
                    else:
                        print("catatan: database sumber bukan database proyek lab; hanya database kosong dibuat")
                kv = {
                    "DB_DATABASE": dst,
                    "DB_USERNAME": cred["DB_USERNAME"],
                    "DB_PASSWORD": cred["DB_PASSWORD"],
                    "DB_HOST": "127.0.0.1",
                    "DB_PORT": str(cred.get("DB_PORT") or DBPORT.get(eng, 5432)),
                    "APP_URL": f"https://{dst}.lab",
                }
                envf.write_text(set_env_kv(text, kv))
                os.chmod(envf, 0o600)
            else:
                envf.write_text(set_env_kv(text, {"APP_URL": f"https://{dst}.lab"})) if "APP_URL=" in text else None
        root = out / "public" if (d / "public").is_dir() else out
        args = ["site", "add", dst, str(root)] + (["--php", v] if v else [])
        need(lab(*args), "tambah situs")
    except SystemExit:
        rollback(out, dbs)
        print("dibatalkan: salinan dan database yang baru dibuat dibersihkan")
        raise
    print(f"OK: {src} dikloning ke {dst} ({out}). Buka https://{dst}.lab. Database kloning terpisah dari sumber (.env sudah diarahkan ke database baru).")
