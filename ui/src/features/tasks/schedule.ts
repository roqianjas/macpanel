// Jadwal cron untuk form tugas: ekspresi dari pilihan, validasi, uraian bahasa Indonesia, dan 3 waktu jalan berikutnya.
// Aturannya sengaja sama dengan cli/labcli/cronspec.py (yang berwenang); di sini hanya untuk umpan balik cepat sebelum kirim.
import { t } from "../../i18n";

export type Mode = "minutes" | "hours" | "hourly" | "daily" | "weekly" | "monthly" | "custom" | "boot";

export const modes = (): { id: Mode; label: string }[] => [
  { id: "minutes", label: t("dev.sch.mode.minutes") },
  { id: "hours", label: t("dev.sch.mode.hours") },
  { id: "hourly", label: t("dev.sch.mode.hourly") },
  { id: "daily", label: t("dev.sch.mode.daily") },
  { id: "weekly", label: t("dev.sch.mode.weekly") },
  { id: "monthly", label: t("dev.sch.mode.monthly") },
  { id: "custom", label: t("dev.sch.mode.custom") },
  { id: "boot", label: t("dev.sch.mode.boot") },
];

export const days = (): string[] => [0, 1, 2, 3, 4, 5, 6].map((i) => t(`dev.sch.dayS.${i}`));
const dayLong = (i: number): string => t(`dev.sch.dayL.${i}`);
const monthLong = (i: number): string => t(`dev.sch.month.${i}`);

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

const hm = (s: string): [number, number] | null => {
  const m = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(s);
  return m ? [Number(m[1]), Number(m[2])] : null;
};

/** Isi kolom jadwal untuk API, atau pesan galat. */
export function toBody(p: Pick): { body: Body } | { error: string } {
  const int = (s: string, lo: number, hi: number) => (/^\d{1,3}$/.test(s) && Number(s) >= lo && Number(s) <= hi ? Number(s) : null);
  switch (p.mode) {
    case "minutes": {
      const n = int(p.n, 1, 59);
      return n ? { body: { every: `${n}m` } } : { error: t("dev.sch.err.minutes") };
    }
    case "hours": {
      const n = int(p.n, 1, 999);
      return n ? { body: { every: `${n}h` } } : { error: t("dev.sch.err.hours") };
    }
    case "hourly": {
      const m = int(p.minute, 0, 59);
      return m !== null ? { body: { cron: `${m} * * * *` } } : { error: t("dev.sch.err.minute") };
    }
    case "daily":
      return hm(p.time) ? { body: { at: p.time } } : { error: t("dev.sch.err.time") };
    case "weekly": {
      const tm = hm(p.time);
      if (!tm) return { error: t("dev.sch.err.time") };
      if (!p.days.length) return { error: t("dev.sch.err.days") };
      return { body: { cron: `${tm[1]} ${tm[0]} * * ${[...p.days].sort((a, b) => a - b).join(",")}` } };
    }
    case "monthly": {
      const tm = hm(p.time);
      const d = int(p.dom, 1, 31);
      if (!tm) return { error: t("dev.sch.err.time") };
      if (!d) return { error: t("dev.sch.err.dom") };
      return { body: { cron: `${tm[1]} ${tm[0]} ${d} * *` } };
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
const FIELDS: [string, number, number, Record<string, number>][] = [["minute", 0, 59, {}], ["hour", 0, 23, {}], ["dom", 1, 31, {}], ["month", 1, 12, MON], ["dow", 0, 7, DOW]];
export const MAX_ENTRIES = 500;

export type Parsed = { boot: true } | { boot: false; sets: number[][]; full: boolean[] };

function field(text: string, lo: number, hi: number, names: Record<string, number>, fid: string): number[] | string {
  const label = t(`dev.sch.f.${fid}`);
  const out = new Set<number>();
  for (const part of text.split(",")) {
    if (!part) return t("dev.sch.err.empty", { label });
    const [rng, step] = part.split("/") as [string, string | undefined];
    if (part.split("/").length > 2 || (step !== undefined && !/^[1-9]\d{0,2}$/.test(step))) return t("dev.sch.err.step", { label });
    const n = step ? Number(step) : 1;
    const num = (s: string) => (names[s.toLowerCase()] ?? (/^\d{1,2}$/.test(s) ? Number(s) : NaN));
    let a: number, b: number;
    if (rng === "*") [a, b] = [lo, fid === "dow" ? 6 : hi];
    else if (rng.includes("-")) {
      const [x, y] = rng.split("-");
      [a, b] = [num(x), num(y)];
    } else {
      a = num(rng);
      b = step ? hi : a;
    }
    if (Number.isNaN(a) || Number.isNaN(b)) return t(Object.keys(names).length ? "dev.sch.err.notNumName" : "dev.sch.err.notNum", { label, rng });
    if (a < lo || a > hi || b < lo || b > hi) return t("dev.sch.err.range", { label, lo, hi });
    if (a > b) return t("dev.sch.err.reversed", { label });
    for (let v = a; v <= b; v += n) out.add(fid === "dow" && v === 7 ? 0 : v);
  }
  return [...out].sort((x, y) => x - y);
}

export function parseCron(expr: string): Parsed | { error: string } {
  const e = expr.trim().replace(/\s+/g, " ");
  if (e === "@reboot") return { boot: true };
  const parts = (ALIAS[e] ?? e).split(" ");
  if (parts.length !== 5) return { error: t("dev.sch.err.cols") };
  const sets: number[][] = [];
  for (let i = 0; i < 5; i++) {
    const [fid, lo, hi, names] = FIELDS[i];
    const r = field(parts[i], lo, hi, names, fid);
    if (typeof r === "string") return { error: r };
    sets.push(r);
  }
  const full = [sets[0].length === 60, sets[1].length === 24, sets[2].length === 31, sets[3].length === 12, sets[4].length === 7];
  if (!full[2] && !full[4]) return { error: t("dev.sch.err.domDow") };
  const total = sets.reduce((acc, s, i) => (full[i] ? acc : acc * s.length), 1);
  if (!full.every(Boolean) && total > MAX_ENTRIES) return { error: t("dev.sch.err.dense", { total, max: MAX_ENTRIES }) };
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
  if (ev) return t(ev[2] === "m" ? "dev.sch.everyMin" : "dev.sch.everyHour", { n: Number(ev[1]) });
  const r = parseCron(s);
  if ("error" in r) return s;
  if (r.boot) return t("dev.sch.mode.boot");
  const [mi, ho, dom, mo, dw] = r.sets;
  const f = r.full;
  if (f.every(Boolean)) return t("dev.sch.everyMinute");
  let when: string;
  let clock = "";
  if (f[0] && f[1]) when = t("dev.sch.everyMinute");
  else if (!f[0] && f[1] && mi.length === 1) when = mi[0] === 0 ? t("dev.sch.hourSharp") : t("dev.sch.hourAtMin", { m: mi[0] });
  else if (!f[0] && f[1]) when = mi.length > 1 && mi.every((m, i) => i === 0 || m - mi[i - 1] === mi[1] - mi[0]) && mi[0] === 0 && 60 % mi[1] === 0 ? t("dev.sch.everyMin", { n: mi[1] }) : t("dev.sch.hourAtMin", { m: list(mi, String) });
  else if (f[0] && !f[1]) when = t("dev.sch.minAtHours", { h: ranges(ho, (n) => pad(n)) });
  else if (mi.length === 1 && ho.length === 1) clock = `${pad(ho[0])}:${pad(mi[0])}`, when = t("dev.sch.at", { time: clock });
  else if (mi.length === 1) clock = list(ho, (h) => `${pad(h)}:${pad(mi[0])}`), when = t("dev.sch.at", { time: clock });
  else when = t("dev.sch.minutesAtHours", { m: list(mi, String), h: ranges(ho, pad) });
  const parts = [when];
  if (!f[4]) parts.push(t("dev.sch.onDays", { days: ranges(dw, dayLong) }));
  else if (!f[2]) parts.push(t("dev.sch.onDom", { d: ranges(dom, String) }));
  else if (clock) parts[0] = t("dev.sch.dailyAt", { time: clock });
  if (!f[3]) parts.push(t("dev.sch.inMonths", { m: ranges(mo, monthLong) }));
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
