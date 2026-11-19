// Jadwal cron untuk form tugas: ekspresi dari pilihan, validasi, uraian bahasa Indonesia, dan 3 waktu jalan berikutnya.
// Aturannya sengaja sama dengan cli/labcli/cronspec.py (yang berwenang); di sini hanya untuk umpan balik cepat sebelum kirim.

export type Mode = "minutes" | "hours" | "hourly" | "daily" | "weekly" | "monthly" | "custom" | "boot";

export const MODES: { id: Mode; label: string }[] = [
  { id: "minutes", label: "Setiap N menit" },
  { id: "hours", label: "Setiap N jam" },
  { id: "hourly", label: "Setiap jam, pada menit tertentu" },
  { id: "daily", label: "Setiap hari" },
  { id: "weekly", label: "Setiap minggu (pilih hari)" },
  { id: "monthly", label: "Setiap bulan (pilih tanggal)" },
  { id: "custom", label: "Cron kustom (5 kolom)" },
  { id: "boot", label: "Saat Mac mini menyala (setelah login)" },
];

export const DAYS = ["Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"];
const DAY_LONG = ["Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"];
const MONTH_LONG = ["", "Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"];

export interface Pick {
  mode: Mode;
  n: string; // menit atau jam
  minute: string; // menit ke-
  time: string; // HH:MM
  days: number[]; // 0 = Minggu
  dom: string; // tanggal 1-31
  cron: string;
}

export const DEFAULT_PICK: Pick = { mode: "minutes", n: "5", minute: "0", time: "03:00", days: [1, 2, 3, 4, 5], dom: "1", cron: "*/15 * * * *" };

export type Body = { every?: string; at?: string; cron?: string };

const hm = (t: string): [number, number] | null => {
  const m = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(t);
  return m ? [Number(m[1]), Number(m[2])] : null;
};

/** Isi kolom jadwal untuk API, atau pesan galat. */
export function toBody(p: Pick): { body: Body } | { error: string } {
  const int = (s: string, lo: number, hi: number) => (/^\d{1,3}$/.test(s) && Number(s) >= lo && Number(s) <= hi ? Number(s) : null);
  switch (p.mode) {
    case "minutes": {
      const n = int(p.n, 1, 59);
      return n ? { body: { every: `${n}m` } } : { error: "Menit antara 1 dan 59 (untuk lebih dari satu jam pilih 'Setiap N jam')." };
    }
    case "hours": {
      const n = int(p.n, 1, 999);
      return n ? { body: { every: `${n}h` } } : { error: "Jam antara 1 dan 999." };
    }
    case "hourly": {
      const m = int(p.minute, 0, 59);
      return m !== null ? { body: { cron: `${m} * * * *` } } : { error: "Menit antara 0 dan 59." };
    }
    case "daily":
      return hm(p.time) ? { body: { at: p.time } } : { error: "Jam harus HH:MM." };
    case "weekly": {
      const t = hm(p.time);
      if (!t) return { error: "Jam harus HH:MM." };
      if (!p.days.length) return { error: "Pilih minimal satu hari." };
      return { body: { cron: `${t[1]} ${t[0]} * * ${[...p.days].sort((a, b) => a - b).join(",")}` } };
    }
    case "monthly": {
      const t = hm(p.time);
      const d = int(p.dom, 1, 31);
      if (!t) return { error: "Jam harus HH:MM." };
      if (!d) return { error: "Tanggal antara 1 dan 31 (bulan yang lebih pendek dilewati)." };
      return { body: { cron: `${t[1]} ${t[0]} ${d} * *` } };
    }
    case "boot":
      return { body: { cron: "@reboot" } };
    case "custom": {
      const e = p.cron.trim().replace(/\s+/g, " ");
      const r = parseCron(e);
      return "error" in r ? { error: r.error } : { body: { cron: e } };
    }
  }
}

// ---------- parser (cermin cronspec.py) ----------
const ALIAS: Record<string, string> = { "@hourly": "0 * * * *", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@weekly": "0 0 * * 0", "@monthly": "0 0 1 * *", "@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *" };
const MON: Record<string, number> = Object.fromEntries("jan feb mar apr may jun jul aug sep oct nov dec".split(" ").map((m, i) => [m, i + 1]));
const DOW: Record<string, number> = Object.fromEntries("sun mon tue wed thu fri sat".split(" ").map((d, i) => [d, i]));
const FIELDS: [string, number, number, Record<string, number>][] = [["menit", 0, 59, {}], ["jam", 0, 23, {}], ["tanggal", 1, 31, {}], ["bulan", 1, 12, MON], ["hari", 0, 7, DOW]];
export const MAX_ENTRIES = 500;

export type Parsed = { boot: true } | { boot: false; sets: number[][]; full: boolean[] };

function field(text: string, lo: number, hi: number, names: Record<string, number>, label: string): number[] | string {
  const out = new Set<number>();
  for (const part of text.split(",")) {
    if (!part) return `${label}: daftar kosong`;
    const [rng, step] = part.split("/") as [string, string | undefined];
    if (part.split("/").length > 2 || (step !== undefined && !/^[1-9]\d{0,2}$/.test(step))) return `${label}: langkah harus angka positif`;
    const n = step ? Number(step) : 1;
    const num = (t: string) => (names[t.toLowerCase()] ?? (/^\d{1,2}$/.test(t) ? Number(t) : NaN));
    let a: number, b: number;
    if (rng === "*") [a, b] = [lo, label === "hari" ? 6 : hi];
    else if (rng.includes("-")) {
      const [x, y] = rng.split("-");
      [a, b] = [num(x), num(y)];
    } else {
      a = num(rng);
      b = step ? hi : a;
    }
    if (Number.isNaN(a) || Number.isNaN(b)) return `${label}: '${rng}' bukan angka${Object.keys(names).length ? " atau nama singkat" : ""}`;
    if (a < lo || a > hi || b < lo || b > hi) return `${label}: di luar rentang ${lo}-${hi}`;
    if (a > b) return `${label}: rentang terbalik`;
    for (let v = a; v <= b; v += n) out.add(label === "hari" && v === 7 ? 0 : v);
  }
  return [...out].sort((x, y) => x - y);
}

export function parseCron(expr: string): Parsed | { error: string } {
  const e = expr.trim().replace(/\s+/g, " ");
  if (e === "@reboot") return { boot: true };
  const parts = (ALIAS[e] ?? e).split(" ");
  if (parts.length !== 5) return { error: "Harus 5 kolom: menit jam tanggal bulan hari (mis. */15 8-17 * * 1-5)." };
  const sets: number[][] = [];
  for (let i = 0; i < 5; i++) {
    const [label, lo, hi, names] = FIELDS[i];
    const r = field(parts[i], lo, hi, names, label);
    if (typeof r === "string") return { error: r };
    sets.push(r);
  }
  const full = [sets[0].length === 60, sets[1].length === 24, sets[2].length === 31, sets[3].length === 12, sets[4].length === 7];
  if (!full[2] && !full[4]) return { error: "Tanggal dan hari-dalam-minggu tidak bisa sama-sama dibatasi (cron memakai OR, launchd AND). Pilih salah satu." };
  const total = sets.reduce((acc, s, i) => (full[i] ? acc : acc * s.length), 1);
  if (!full.every(Boolean) && total > MAX_ENTRIES) return { error: `Jadwal terlalu rapat (${total} entri, maks ${MAX_ENTRIES}). Perjarang menit atau jam.` };
  return { boot: false, sets, full };
}

const pad = (n: number) => String(n).padStart(2, "0");

function list(v: number[], fmt: (n: number) => string, join = ", "): string {
  return v.map(fmt).join(join);
}

function ranges(v: number[], fmt: (n: number) => string): string {
  const out: string[] = [];
  for (let i = 0; i < v.length; ) {
    let j = i;
    while (j + 1 < v.length && v[j + 1] === v[j] + 1) j++;
    out.push(j - i >= 2 ? `${fmt(v[i])}–${fmt(v[j])}` : v.slice(i, j + 1).map(fmt).join(", "));
    i = j + 1;
  }
  return out.join(", ");
}

/** Uraian singkat untuk ekspresi cron, "every Nm" dari lab, atau "@reboot". Tidak pernah melempar. */
export function describe(spec: string): string {
  const s = spec.trim();
  const ev = /^every (\d+)([mh])$/.exec(s);
  if (ev) return `Setiap ${ev[1]} ${ev[2] === "m" ? "menit" : "jam"}`;
  const r = parseCron(s);
  if ("error" in r) return s;
  if (r.boot) return "Saat Mac mini menyala (setelah login)";
  const [mi, ho, dom, mo, dw] = r.sets;
  const f = r.full;
  if (f.every(Boolean)) return "Setiap menit";
  let when: string;
  if (f[0] && f[1]) when = "Setiap menit";
  else if (!f[0] && f[1] && mi.length === 1) when = mi[0] === 0 ? "Setiap jam tepat" : `Setiap jam pada menit ${mi[0]}`;
  else if (!f[0] && f[1]) when = mi.length > 1 && mi.every((m, i) => i === 0 || m - mi[i - 1] === mi[1] - mi[0]) && mi[0] === 0 && 60 % mi[1] === 0 ? `Setiap ${mi[1]} menit` : `Setiap jam pada menit ${list(mi, String)}`;
  else if (f[0] && !f[1]) when = `Setiap menit pada jam ${ranges(ho, (n) => pad(n))}`;
  else if (mi.length === 1 && ho.length === 1) when = `Pukul ${pad(ho[0])}:${pad(mi[0])}`;
  else if (mi.length === 1) when = `Pukul ${list(ho, (h) => `${pad(h)}:${pad(mi[0])}`)}`;
  else when = `Menit ${list(mi, String)} pada jam ${ranges(ho, pad)}`;
  const parts = [when];
  if (!f[4]) parts.push(`hari ${ranges(dw, (d) => DAY_LONG[d])}`);
  else if (!f[2]) parts.push(`tanggal ${ranges(dom, String)}`);
  else if (when.startsWith("Pukul")) parts[0] = `Setiap hari p${when.slice(1)}`;
  if (!f[3]) parts.push(`bulan ${ranges(mo, (m) => MONTH_LONG[m])}`);
  return parts.join(", ");
}

/** n waktu jalan berikutnya (waktu lokal browser; zona waktu Mac mini bisa berbeda). Kosong untuk @reboot atau galat. */
export function nextRuns(spec: string, n = 3, from = new Date()): Date[] {
  const r = parseCron(spec);
  if ("error" in r || r.boot) return [];
  const [mi, ho, dom, mo, dw] = r.sets;
  const out: Date[] = [];
  const t = new Date(from);
  t.setSeconds(0, 0);
  for (let i = 0; i < 366 * 24 * 60 && out.length < n; i++) {
    t.setMinutes(t.getMinutes() + 1);
    if (mi.includes(t.getMinutes()) && ho.includes(t.getHours()) && mo.includes(t.getMonth() + 1) && dom.includes(t.getDate()) && dw.includes(t.getDay())) out.push(new Date(t));
  }
  return out;
}
