"""lab sys hw / ssd: parser dan logika sumber data (macmon utama, Netdata cadangan)."""

import json
import plistlib

import pytest
from labcli import hardware as hw

MACMON = {
    "temp": {"cpu_temp_avg": 42.66346, "gpu_temp_avg": 41.989582},
    "fans": [{"name": "fan0", "rpm": 1000, "max_rpm": 4900}],
    "chip_power": 2.048594,
    "dc_in_power": 7.087473,
    "sys_power": 6.470309,
    "memory": {"pressure_level": 0, "pressure_percent": 40, "ram_total": 17179869184, "ram_usage": 12549144576, "swap_total": 1073741824, "swap_usage": 19922944},
}


def line(d):
    return json.dumps(d) + "\n"


def test_parse_macmon_normal():
    r = hw.parse_macmon(line(MACMON))
    assert r["temps"] == {"cpu": 42.7, "gpu": 42.0}
    assert r["fan"] == {"rpm": 1000, "max": 4900}
    assert r["power"] == {"chip": 2.05, "dc_in": 7.09, "system": 6.47}
    assert r["mem"] == {"pressure_pct": 40.0, "swap_used_mb": 19.0, "swap_total_mb": 1024}


def test_parse_macmon_takes_last_line_and_skips_noise():
    first = dict(MACMON, temp={"cpu_temp_avg": 10.0, "gpu_temp_avg": 10.0})
    r = hw.parse_macmon("peringatan teks\n" + line(first) + "bukan json {\n" + line(MACMON))
    assert r["temps"]["cpu"] == 42.7


@pytest.mark.parametrize("bad", ["", "bukan json", "[1,2]", "{}", '{"temp": 5}', '{"temp": {}, "fans": []}'])
def test_parse_macmon_garbage_gives_none(bad):
    assert hw.parse_macmon(bad) is None


@pytest.mark.parametrize("v", [0, -5, 0.0, 4.9, 125.1, 500, 99999, float("nan"), True, None, "42", [42]])
def test_impossible_temperatures_are_dropped_not_invented(v):
    d = dict(MACMON, temp={"cpu_temp_avg": v, "gpu_temp_avg": 50.0})
    r = hw.parse_macmon(line(d))
    assert "cpu" not in r["temps"] and r["temps"]["gpu"] == 50.0


def test_temperature_range_edges_accepted():
    for v in (5.0, 125.0):
        assert hw.parse_macmon(line(dict(MACMON, temp={"cpu_temp_avg": v})))["temps"]["cpu"] == v


def test_missing_parts_become_null():
    r = hw.parse_macmon(line({"temp": {"cpu_temp_avg": 50.0}}))
    assert r["fan"] is None and r["power"] is None and r["mem"] is None
    r = hw.parse_macmon(line({"temp": {"cpu_temp_avg": 50.0}, "fans": [{"rpm": -1}], "memory": {"pressure_percent": 150}}))
    assert r["fan"] is None and r["mem"] is None  # nilai mustahil dibuang


def test_fan_at_zero_rpm_is_real_data_not_missing():
    r = hw.parse_macmon(line(dict(MACMON, fans=[{"rpm": 0, "max_rpm": 4900}])))
    assert r["fan"] == {"rpm": 0, "max": 4900}


def test_impossible_power_dropped():
    r = hw.parse_macmon(line(dict(MACMON, chip_power=-3, dc_in_power=5000, sys_power=12.0)))
    assert r["power"] == {"chip": None, "dc_in": None, "system": 12.0}


def nd(avg):
    return {"labels": ["time", "min", "avg", "max"], "data": [[1, 0, avg, 0]]}


def test_parse_netdata_temp():
    assert hw.parse_netdata_temp(nd(55.5)) == 55.5
    assert hw.parse_netdata_temp(nd(0)) is None  # sensor yang tidak ada dilaporkan 0
    assert hw.parse_netdata_temp({"labels": ["time", "x"], "data": [[1, 5]]}) is None
    assert hw.parse_netdata_temp({"labels": ["time", "avg"], "data": []}) is None
    assert hw.parse_netdata_temp({}) is None and hw.parse_netdata_temp(None) is None


def test_netdata_temps_collects_and_survives_failures():
    def fetch(url):
        if "storage" in url:
            raise OSError("netdata mati")
        if "gpu" in url:
            return nd(0)
        return nd(50.04)

    t = hw.netdata_temps(fetch)
    assert t == {"cpu": 50.0, "memory": 50.0, "ambient": 50.0}


def test_snapshot_prefers_macmon_and_adds_missing_sensors_from_netdata():
    s = hw.hw_snapshot(
        macmon=lambda: (hw.parse_macmon(line(MACMON)), None), netdata=lambda: {"cpu": 99.0, "ssd": 48.1, "ambient": 39.0}, therm={"speed_limit": 100, "throttled": False}, now=lambda: 1700000000
    )
    assert s["source"] == "macmon" and s["errors"] == []
    assert s["temps"]["cpu"] == 42.7  # bukan 99 dari Netdata: skala sensor berbeda tidak dicampur
    assert s["temps"]["ssd"] == 48.1 and s["temps"]["ambient"] == 39.0
    assert s["at"] == 1700000000


def test_snapshot_falls_back_to_netdata_and_says_so():
    s = hw.hw_snapshot(macmon=lambda: (None, "macmon tidak terpasang"), netdata=lambda: {"cpu": 55.0}, therm=None)
    assert s["source"] == "netdata" and s["temps"] == {"cpu": 55.0}
    assert s["fan"] is None and s["power"] is None  # daya Netdata terbukti salah: tidak dipakai
    assert any("macmon tidak terpasang" in e for e in s["errors"])
    assert any("skala" in e for e in s["errors"])


def test_snapshot_with_no_source_reports_none_honestly():
    s = hw.hw_snapshot(macmon=lambda: (None, "macmon gagal"), netdata=lambda: {}, therm=None)
    assert s["source"] == "none" and s["temps"] == {}
    assert any("brew install macmon" in e for e in s["errors"])


def test_parse_thermal():
    assert hw.parse_thermal("Note: No thermal warning level has been recorded\n") == {"speed_limit": 100, "throttled": False}
    assert hw.parse_thermal("CPU_Scheduler_Limit = 100\nCPU_Speed_Limit = 75\n") == {"speed_limit": 75, "throttled": True}


def plist(**kw):
    return plistlib.dumps(kw)


def test_parse_diskutil():
    r = hw.parse_diskutil(plist(MediaName="APPLE SSD AP0256Z", SolidState=True, SMARTStatus="Verified", TotalSize=245054767104, ParentWholeDisk="disk0"))
    assert r == {"parent": "disk0", "name": "APPLE SSD AP0256Z", "solid_state": True, "smart": "Verified", "size": 245054767104}
    assert hw.parse_diskutil(plist(SMARTStatus="Failing"))["smart"] == "Failing"
    assert hw.parse_diskutil(plist(MediaName="x"))["smart"] is None  # tidak ada status: tidak dikarang "Verified"
    assert hw.parse_diskutil(b"bukan plist") is None


def test_new_subcommands_are_registered():
    from labcli.cli import HANDLERS, build_parser

    a = build_parser().parse_args(["sys", "hw"])
    assert a.mod == "sys" and a.act == "hw" and "sys" in HANDLERS
    assert build_parser().parse_args(["sys", "ssd"]).act == "ssd"
