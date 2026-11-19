"""Ekspresi cron 5 kolom -> jadwal launchd (murni, tanpa efek samping).

launchd tidak punya sintaks cron: ia menerima daftar kamus StartCalendarInterval (Minute/Hour/Day/Month/Weekday, kunci yang
tidak ada = semua nilai). Modul ini memperluas ekspresi ke daftar itu. Dua beda semantik yang tidak bisa dijembatani ditolak
terang-terangan, bukan diterjemahkan diam-diam:
  * hari-dalam-bulan DAN hari-dalam-minggu sama-sama dibatasi: cron memakai OR, launchd memakai AND.
  * hasil perluasan lebih dari MAX_ENTRIES entri.
"""

import re

MAX_ENTRIES = 500

ALIASES = {
    "@hourly": "0 * * * *",
    "@daily": "0 0 * * *",
    "@midnight": "0 0 * * *",
    "@weekly": "0 0 * * 0",
    "@monthly": "0 0 1 * *",
    "@yearly": "0 0 1 1 *",
    "@annually": "0 0 1 1 *",
}
REBOOT = "@reboot"

_MONTHS = {m: i + 1 for i, m in enumerate("jan feb mar apr may jun jul aug sep oct nov dec".split())}
_DOW = {d: i for i, d in enumerate("sun mon tue wed thu fri sat".split())}

# (nama, minimum, maksimum, nama-singkat)
FIELDS = [("menit", 0, 59, {}), ("jam", 0, 23, {}), ("tanggal", 1, 31, {}), ("bulan", 1, 12, _MONTHS), ("hari", 0, 7, _DOW)]


class CronError(ValueError):
    pass


def _num(tok, names, label):
    t = tok.lower()
    if t in names:
        return names[t]
    if re.fullmatch(r"\d{1,2}", t):
        return int(t)
    raise CronError(f"{label}: '{tok}' bukan angka" + (" atau nama singkat" if names else ""))


def _field(text, lo, hi, names, label):
    out = set()
    for part in text.split(","):
        if not part:
            raise CronError(f"{label}: daftar kosong")
        rng, _, step = part.partition("/")
        if _ and not re.fullmatch(r"[1-9]\d{0,2}", step):
            raise CronError(f"{label}: langkah '/{step}' harus angka positif")
        n = int(step) if step else 1
        if rng == "*":
            a, b = lo, hi
            if label == "hari":
                b = 6  # * = 0..6; 7 hanyalah nama lain Minggu
        elif "-" in rng:
            x, _, y = rng.partition("-")
            a, b = _num(x, names, label), _num(y, names, label)
        else:
            a = _num(rng, names, label)
            b = hi if step else a
        if not (lo <= a <= hi and lo <= b <= hi):
            raise CronError(f"{label}: di luar rentang {lo}-{hi}")
        if a > b:
            raise CronError(f"{label}: rentang terbalik ({a}-{b})")
        out.update(range(a, b + 1, n))
    if label == "hari":
        out = {0 if v == 7 else v for v in out}
    return sorted(out)


def parse(expr):
    """Kembalikan ("boot", None), ("interval", detik), atau ("calendar", [kamus, ...])."""
    e = " ".join(expr.split())
    if e == REBOOT:
        return "boot", None
    e = ALIASES.get(e, e)
    parts = e.split(" ")
    if len(parts) != 5:
        raise CronError("harus 5 kolom: menit jam tanggal bulan hari (mis. */15 8-17 * * 1-5)")
    vals = [_field(p, lo, hi, names, label) for p, (label, lo, hi, names) in zip(parts, FIELDS)]
    full = [len(vals[0]) == 60, len(vals[1]) == 24, len(vals[2]) == 31, len(vals[3]) == 12, len(vals[4]) == 7]
    if not full[2] and not full[4]:
        raise CronError("tanggal dan hari-dalam-minggu tidak boleh sama-sama dibatasi (cron memakai OR, launchd AND). Pilih salah satu")
    if all(full):
        return "interval", 60
    keys = ["Minute", "Hour", "Day", "Month", "Weekday"]
    use = [(k, v) for k, v, f in zip(keys, vals, full) if not f]
    total = 1
    for _, v in use:
        total *= len(v)
    if total > MAX_ENTRIES:
        raise CronError(f"jadwal terlalu rapat ({total} entri, maks {MAX_ENTRIES}). Perjarang menit atau jam")
    out = [{}]
    for k, v in use:
        out = [{**d, k: x} for d in out for x in v]
    return "calendar", out
