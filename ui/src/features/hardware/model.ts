import { decimalSep, t } from "../../i18n";

/** Umur dalam detik -> "N detik lalu" atau "N menit lalu". */
const agoShort = (age: number) => (age < 60 ? t("fmt.ago", { span: t("fmt.sec", { n: Math.floor(age) }) }) : t("fmt.ago", { span: t("fmt.min", { n: Math.floor(age / 60) }) }));
// Tipe dan logika murni untuk perangkat keras (suhu, kipas, daya, tekanan memori, SSD).
// Ambang datang dari server (/api/hw "thresholds"), jadi label di layar dan peringatan di server tidak bisa berbeda.

export interface HwSnapshot {
  at: number;
  source: "macmon" | "netdata" | "none";
  temps: Partial<Record<"cpu" | "gpu" | "ssd" | "memory" | "ambient", number>>;
  fan: { rpm: number; max: number | null } | null;
  power: { chip: number | null; dc_in: number | null; system: number | null } | null;
  mem: { pressure_pct: number | null; swap_used_mb: number | null; swap_total_mb: number | null } | null;
  thermal: { speed_limit: number; throttled: boolean } | null;
  errors: string[];
}

export interface HwSsd {
  name: string;
  solid_state: boolean;
  smart: string;
  total: number;
  free: number;
  wear: string;
  at: number;
}

export interface HwForecast {
  ok: boolean;
  reason?: string;
  pct_per_day: number;
  gb_per_day?: number;
  days_to_full?: number;
  r2: number;
  span_days: number;
}

export type Pt = [number, number];

export interface HwData {
  now: HwSnapshot | null;
  stale: boolean;
  ssd: HwSsd | null;
  forecast: HwForecast;
  range: string;
  bucket: number;
  interval: number;
  series: Record<string, Pt[]>;
  peak: Record<string, number>;
  samples: number;
  throttled_samples: number;
  thresholds: { cpu_warn: number; cpu_bad: number; ssd_warn: number; ssd_bad: number; press_warn: number; press_bad: number; swap_mb: number };
}

export type Tone = "ok" | "warn" | "bad" | "mute";
export interface Status {
  tone: Tone;
  text: string;
}

/** Koma desimal Indonesia. null atau NaN tampil "n/a": tidak ada angka karangan. */
export function num(v: number | null | undefined, digits = 0): string {
  if (v == null || !Number.isFinite(v)) return t("fmt.na");
  return v.toFixed(digits).replace(".", decimalSep());
}

export const fmtTemp = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? t("fmt.na") : `${num(v, 0)} °C`);

export function levelStatus(v: number | null | undefined, warn: number, bad: number, labels = { ok: t("system.hw.lv.ok"), warn: t("system.hw.lv.warn"), bad: t("system.hw.lv.bad") }): Status {
  if (v == null || !Number.isFinite(v)) return { tone: "mute", text: t("system.hw.unreadable") };
  if (v >= bad) return { tone: "bad", text: labels.bad };
  if (v >= warn) return { tone: "warn", text: labels.warn };
  return { tone: "ok", text: labels.ok };
}

export const tempStatus = (v: number | null | undefined, warn: number, bad: number) => levelStatus(v, warn, bad);

export const pressureStatus = (v: number | null | undefined, warn: number, bad: number) => levelStatus(v, warn, bad, { ok: t("system.hw.lv.ok"), warn: t("system.hw.lv.high"), bad: t("system.hw.lv.vhigh") });

export function smartStatus(smart: string | null | undefined): Status {
  switch (smart) {
    case "Verified":
      return { tone: "ok", text: t("system.hw.sm.ok") };
    case "Not Supported":
      return { tone: "mute", text: t("system.hw.sm.unsupported") };
    case "":
    case null:
    case undefined:
      return { tone: "mute", text: t("system.hw.sm.unknown") };
    default:
      return { tone: "bad", text: t("system.hw.sm.bad") };
  }
}

export function throttleStatus(th: HwSnapshot["thermal"]): Status {
  if (!th) return { tone: "mute", text: t("system.hw.unreadable") };
  return th.throttled ? { tone: "bad", text: t("system.hw.th.limited", { n: th.speed_limit }) } : { tone: "ok", text: t("system.hw.th.free") };
}

/** Kipas: persen dari batas atas kalau batas diketahui. 0 rpm tetap angka nyata (kipas Mac mini boleh berhenti saat dingin). */
export function fanText(f: HwSnapshot["fan"]): string {
  if (!f) return t("fmt.na");
  if (f.max && f.max > 0) return t("system.hw.fanPct", { rpm: f.rpm, pct: Math.round((f.rpm / f.max) * 100), max: f.max });
  return `${f.rpm} rpm`;
}

/** Keadaan keseluruhan data: yang ditampilkan di atas kartu, supaya angka basi atau sumber cadangan tidak terbaca sebagai normal. */
export function freshness(d: HwData | null, nowSec: number): { tone: Tone; text: string } {
  if (!d || !d.now) return { tone: "mute", text: t("system.hw.fr.none") };
  const age = Math.max(0, nowSec - d.now.at);
  if (d.stale) return { tone: "warn", text: t("system.hw.fr.stale", { ago: t("fmt.ago", { span: t("fmt.min", { n: Math.floor(age / 60) }) }) }) };
  if (d.now.source === "none") return { tone: "warn", text: d.now.errors[d.now.errors.length - 1] || t("system.hw.fr.nosrc") };
  if (d.now.source === "netdata") return { tone: "warn", text: t("system.hw.fr.netdata") };
  return { tone: "ok", text: t("system.hw.fr.ok", { ago: agoShort(age) }) };
}

/** Kalimat prakiraan disk. Tidak pernah menampilkan angka bila datanya belum cukup. */
export function forecastText(f: HwForecast): { tone: Tone; text: string } {
  if (!f.ok) return { tone: "mute", text: f.reason || t("system.hw.fc.nodata") };
  const rate = t("system.hw.fc.rate", { pct: num(f.pct_per_day, 2) }) + (f.gb_per_day != null ? t("system.hw.fc.rateGb", { gb: num(f.gb_per_day, 1) }) : "");
  if (f.days_to_full == null) return { tone: "ok", text: f.reason ? t("system.hw.fc.usage", { reason: f.reason, rate }) : t("system.hw.fc.trend", { rate }) };
  const low = f.r2 < 0.5 ? t("system.hw.fc.low") : "";
  const tone: Tone = f.r2 >= 0.5 && f.days_to_full < 3 ? "bad" : f.r2 >= 0.5 && f.days_to_full < 14 ? "warn" : "ok";
  return { tone, text: t("system.hw.fc.full", { rate, n: Math.round(f.days_to_full) }) + low };
}

/** Kesimpulan satu kalimat dari beberapa status. Status "tidak terbaca" tidak dihitung sehat. */
export function verdict(rows: Status[]): Status {
  const bad = rows.filter((r) => r.tone === "bad").length;
  const warn = rows.filter((r) => r.tone === "warn").length;
  const ok = rows.filter((r) => r.tone === "ok").length;
  if (bad) return { tone: "bad", text: t("system.hw.vd.bad", { n: bad }) };
  if (warn) return { tone: "warn", text: t("system.hw.vd.warn", { n: warn }) };
  if (ok === 0) return { tone: "mute", text: t("system.hw.vd.none") };
  return { tone: "ok", text: rows.some((r) => r.tone === "mute") ? t("system.hw.vd.partial") : t("system.hw.vd.ok") };
}

/** Sertifikat situs: hari tersisa terkecil. Ambang sama dengan alert server (di bawah 14 hari = perhatian, 0 atau kurang = penting). */
export function certStatus(minDays: number | null | undefined): Status {
  if (minDays == null || !Number.isFinite(minDays)) return { tone: "mute", text: t("system.hw.cert.none") };
  if (minDays <= 0) return { tone: "bad", text: t("system.hw.cert.expired") };
  if (minDays < 14) return { tone: "warn", text: t("system.hw.cert.soon") };
  return { tone: "ok", text: t("system.hw.safe") };
}

export interface BackupInfo {
  at: number;
  count: number;
  local_only: boolean;
  name: string;
}

/** Backup: umur dan kejujuran lokasi. Salinan di disk yang sama bukan backup penuh, jadi local_only tidak pernah tampil "Aman" begitu saja. */
export function backupStatus(b: BackupInfo | null | undefined, nowSec: number): { st: Status; text: string } {
  if (!b) return { st: { tone: "warn", text: t("system.hw.bk.none") }, text: t("system.hw.bk.noneText") };
  const hours = Math.max(0, Math.floor((nowSec - b.at) / 3600));
  const age = hours < 1 ? t("system.hw.bk.lt1") : hours < 48 ? t("fmt.ago", { span: t("fmt.hour", { n: hours }) }) : t("fmt.ago", { span: t("fmt.day", { n: Math.floor(hours / 24) }) });
  const where = b.local_only ? ` ${t("system.hw.bk.localNote")}` : "";
  if (hours > 36) return { st: { tone: "warn", text: t("system.hw.bk.old") }, text: t("system.hw.bk.last", { age }) + where };
  if (b.local_only) return { st: { tone: "warn", text: t("system.hw.bk.localOnly") }, text: t("system.hw.bk.last", { age }) + where };
  return { st: { tone: "ok", text: t("system.hw.safe") }, text: t("system.hw.bk.last", { age }) };
}
