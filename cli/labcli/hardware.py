"""lab sys hw / ssd: suhu, kipas, daya, tekanan memori, dan kesehatan SSD (semua tanpa sudo).

Sumber utama macmon (Apple Silicon, IOReport/SMC). Netdata (API lokal) menjadi cadangan bila macmon tidak ada
atau gagal, dan melengkapi sensor yang macmon tidak punya (SSD, memori, suasana). Nilai yang tidak terbaca
dilaporkan null, tidak dikarang. Angka daya Netdata sengaja TIDAK dipakai: terbukti salah di mesin ini
(169,8 W vs 2,6 W dari macmon), jadi daya hanya berasal dari macmon.
"""

import json
import plistlib
import re
import shutil
import subprocess
import time
import urllib.request

from labcli.core import ENV, die, sh

NETDATA = "http://127.0.0.1:19999"
# Chart Netdata (sensor SMC) -> kunci suhu. Dimensi "avg" dipakai.
NETDATA_TEMPS = {
    "sensors.temperature_subsystem_cpu": "cpu",
    "sensors.temperature_subsystem_gpu": "gpu",
    "sensors.temperature_subsystem_storage": "ssd",
    "sensors.temperature_subsystem_memory": "memory",
    "sensors.temperature_subsystem_ambient": "ambient",
}
# Hanya suhu di rentang ini dianggap sensor sungguhan; di luarnya (0 = sensor tidak ada, ratusan = galat) dibuang.
TEMP_MIN, TEMP_MAX = 5.0, 125.0


def _num(v, lo=None, hi=None):
    """Angka berhingga dalam rentang, selain itu None (bool bukan angka)."""
    if isinstance(v, bool) or not isinstance(v, (int, float)):
        return None
    if v != v or v in (float("inf"), float("-inf")):
        return None
    if lo is not None and v < lo:
        return None
    if hi is not None and v > hi:
        return None
    return float(v)


def _r1(v):
    return None if v is None else round(v, 1)


def parse_macmon(text):
    """Keluaran `macmon pipe -s 1` (satu objek JSON per baris; ambil yang terakhir) -> dict ringkas, atau None."""
    d = None
    for ln in text.splitlines():
        ln = ln.strip()
        if ln.startswith("{"):
            try:
                d = json.loads(ln)
            except ValueError:
                continue
    if not isinstance(d, dict):
        return None
    t = d.get("temp") if isinstance(d.get("temp"), dict) else {}
    out = {
        "temps": {k: v for k, v in (("cpu", _r1(_num(t.get("cpu_temp_avg"), TEMP_MIN, TEMP_MAX))), ("gpu", _r1(_num(t.get("gpu_temp_avg"), TEMP_MIN, TEMP_MAX)))) if v is not None},
        "fan": None,
        "power": None,
        "mem": None,
    }
    fans = d.get("fans")
    if isinstance(fans, list) and fans and isinstance(fans[0], dict):
        rpm, mx = _num(fans[0].get("rpm"), 0, 20000), _num(fans[0].get("max_rpm"), 1, 20000)
        if rpm is not None:
            out["fan"] = {"rpm": int(rpm), "max": int(mx) if mx else None}
    p = {k: _num(d.get(src), 0, 1000) for k, src in (("chip", "chip_power"), ("dc_in", "dc_in_power"), ("system", "sys_power"))}
    if any(v is not None for v in p.values()):
        out["power"] = {k: (round(v, 2) if v is not None else None) for k, v in p.items()}
    m = d.get("memory") if isinstance(d.get("memory"), dict) else {}
    pp = _num(m.get("pressure_percent"), 0, 100)
    if pp is not None:
        mb = 1024 * 1024
        su, st = _num(m.get("swap_usage"), 0), _num(m.get("swap_total"), 0)
        out["mem"] = {"pressure_pct": round(pp, 1), "swap_used_mb": round(su / mb, 1) if su is not None else None, "swap_total_mb": round(st / mb) if st is not None else None}
    if not out["temps"] and out["fan"] is None and out["power"] is None and out["mem"] is None:
        return None
    return out


def parse_netdata_temp(r):
    """Respons /api/v1/data (format json) -> rata-rata dimensi 'avg' atau None."""
    try:
        labels, rows = r["labels"], r["data"]
        i = labels.index("avg")
        return _num(rows[0][i], TEMP_MIN, TEMP_MAX) if rows else None
    except (KeyError, ValueError, IndexError, TypeError):
        return None


def netdata_temps(fetch=None):
    """Suhu dari Netdata. fetch(url) -> objek JSON, bisa diganti di tes."""

    def real(url):
        with urllib.request.urlopen(url, timeout=3) as r:
            return json.load(r)

    fetch = fetch or real
    out = {}
    for chart, key in NETDATA_TEMPS.items():
        try:
            v = parse_netdata_temp(fetch(f"{NETDATA}/api/v1/data?chart={chart}&after=-15&points=1&format=json"))
        except Exception:
            v = None
        if v is not None:
            out[key] = round(v, 1)
    return out


def parse_thermal(text):
    """`pmset -g therm`: tanpa catatan pembatasan berarti tidak ada throttling."""
    m = re.search(r"CPU_Speed_Limit\s*=\s*(\d+)", text)
    limit = int(m.group(1)) if m else 100
    return {"speed_limit": limit, "throttled": limit < 100}


def _macmon():
    exe = shutil.which("macmon", path=ENV.get("PATH"))
    if not exe:
        return None, "macmon tidak terpasang"
    try:
        r = subprocess.run([exe, "pipe", "-s", "1", "-i", "500"], env=ENV, capture_output=True, text=True, timeout=8)
    except subprocess.TimeoutExpired:
        return None, "macmon timeout"
    if r.returncode:
        return None, "macmon gagal (kode %d)" % r.returncode
    d = parse_macmon(r.stdout)
    return d, None if d else "keluaran macmon tidak dikenali"


def hw_snapshot(macmon=_macmon, netdata=netdata_temps, therm=None, now=time.time):
    errors = []
    base, err = macmon()
    if err:
        errors.append(err)
    nd = netdata()
    if base:
        source = "macmon"
        temps = dict(base["temps"])
        for k in ("ssd", "memory", "ambient"):  # sensor yang macmon tidak punya
            if k in nd:
                temps[k] = nd[k]
        snap = dict(base, temps=temps)
    elif nd:
        source = "netdata"
        snap = {"temps": nd, "fan": None, "power": None, "mem": None}
        errors.append("memakai Netdata: skala suhu berbeda dari macmon, jangan dibandingkan langsung")
    else:
        source = "none"
        snap = {"temps": {}, "fan": None, "power": None, "mem": None}
        errors.append("tidak ada sumber suhu (pasang macmon: brew install macmon)")
    if therm is None:
        r = sh(["/usr/bin/pmset", "-g", "therm"])
        therm = parse_thermal(r.stdout) if r.returncode == 0 else None
    return dict(snap, at=int(now()), source=source, thermal=therm, errors=errors)


def parse_diskutil(raw):
    """Plist `diskutil info -plist` -> info SSD."""
    try:
        p = plistlib.loads(raw)
    except Exception:
        return None
    smart = p.get("SMARTStatus")
    return {
        "parent": p.get("ParentWholeDisk") if isinstance(p.get("ParentWholeDisk"), str) else None,
        "name": p.get("MediaName") or p.get("IORegistryEntryName") or "",
        "solid_state": bool(p.get("SolidState")),
        "smart": smart if isinstance(smart, str) else None,  # "Verified", "Failing", "Not Supported"
        "size": p.get("TotalSize") if isinstance(p.get("TotalSize"), int) else None,
    }


def _diskutil(target):
    r = subprocess.run(["/usr/sbin/diskutil", "info", "-plist", target], env=ENV, capture_output=True, timeout=15)
    return parse_diskutil(r.stdout) if r.returncode == 0 else None


def ssd_info():
    # "/" adalah snapshot sistem yang disegel; volume Data menunjuk ke disk fisik lewat ParentWholeDisk
    vol = _diskutil("/System/Volumes/Data")
    if vol is None:
        die("diskutil info gagal")
    info = (_diskutil(vol["parent"]) if vol.get("parent") else None) or vol
    du = shutil.disk_usage("/System/Volumes/Data")
    info.pop("parent", None)
    # catatan jujur: tanpa sudo hanya status SMART yang tersedia, bukan persen keausan
    info.update(total=du.total, free=du.free, wear="tidak tersedia tanpa sudo", at=int(time.time()))
    return info


def sys_hw():
    print(json.dumps(hw_snapshot()))


def sys_ssd():
    print(json.dumps(ssd_info()))
