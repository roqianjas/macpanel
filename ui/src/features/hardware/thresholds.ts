import { t } from "../../i18n";

// Ambang alert perangkat keras: tipe, validasi sisi klien (cermin server), dan perbandingan dengan bawaan.
// Server tetap memvalidasi ulang; ini hanya agar galat muncul sebelum menyimpan.

export interface Thr {
  cpu_warn: number;
  cpu_bad: number;
  ssd_warn: number;
  ssd_bad: number;
  press_warn: number;
  press_bad: number;
  swap_mb: number;
}

export const THR_FIELDS: { key: keyof Thr; label: string; unit: string }[] = [
  { key: "cpu_warn", get label() { return t("system.thr.f.cpu_warn"); }, unit: "°C" },
  { key: "cpu_bad", get label() { return t("system.thr.f.cpu_bad"); }, unit: "°C" },
  { key: "ssd_warn", get label() { return t("system.thr.f.ssd_warn"); }, unit: "°C" },
  { key: "ssd_bad", get label() { return t("system.thr.f.ssd_bad"); }, unit: "°C" },
  { key: "press_warn", get label() { return t("system.thr.f.press_warn"); }, unit: "%" },
  { key: "press_bad", get label() { return t("system.thr.f.press_bad"); }, unit: "%" },
  { key: "swap_mb", get label() { return t("system.thr.f.swap_mb"); }, unit: "MB" },
];

function pair(name: string, w: number, b: number, lo: number, hi: number): string {
  if (!(w >= lo) || !(b <= hi)) return t("system.thr.range", { name, lo, hi });
  if (!(w < b)) return t("system.thr.order", { name });
  return "";
}

/** "" bila sah; selain itu pesan galat pertama. Rentang sama dengan server. */
export function thrError(v: Thr): string {
  for (const k of Object.keys(v) as (keyof Thr)[]) if (typeof v[k] !== "number" || !Number.isFinite(v[k])) return t("system.thr.nan");
  return pair(t("system.thr.n.cpu"), v.cpu_warn, v.cpu_bad, 40, 120) || pair(t("system.thr.n.ssd"), v.ssd_warn, v.ssd_bad, 40, 110) || pair(t("system.thr.n.press"), v.press_warn, v.press_bad, 30, 100) || (v.swap_mb >= 256 && v.swap_mb <= 65536 ? "" : t("system.thr.swap"));
}

/** Teks kolom -> angka; kosong atau bukan angka menjadi NaN agar thrError menolaknya (bukan diam-diam 0). */
export function parseNum(s: string): number {
  const v = s.trim().replace(",", ".");
  return v === "" ? NaN : Number(v);
}

export const isDefault = (t: Thr, d: Thr): boolean => (Object.keys(d) as (keyof Thr)[]).every((k) => t[k] === d[k]);

/** Hanya field yang berubah dari nilai tersimpan, supaya server tidak menimpa field lain. */
export function changed(draft: Thr, saved: Thr): Partial<Thr> {
  const out: Partial<Thr> = {};
  for (const k of Object.keys(saved) as (keyof Thr)[]) if (draft[k] !== saved[k]) out[k] = draft[k];
  return out;
}
