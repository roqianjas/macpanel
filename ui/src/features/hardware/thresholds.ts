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
  { key: "cpu_warn", label: "Suhu CPU/GPU: perhatian", unit: "°C" },
  { key: "cpu_bad", label: "Suhu CPU/GPU: penting", unit: "°C" },
  { key: "ssd_warn", label: "Suhu SSD: perhatian", unit: "°C" },
  { key: "ssd_bad", label: "Suhu SSD: penting", unit: "°C" },
  { key: "press_warn", label: "Tekanan memori: perhatian", unit: "%" },
  { key: "press_bad", label: "Tekanan memori: penting", unit: "%" },
  { key: "swap_mb", label: "Swap terpakai: perhatian", unit: "MB" },
];

function pair(name: string, w: number, b: number, lo: number, hi: number): string {
  if (!(w >= lo) || !(b <= hi)) return `${name}: nilai harus di antara ${lo} dan ${hi}.`;
  if (!(w < b)) return `${name}: batas perhatian harus lebih rendah dari batas penting.`;
  return "";
}

/** "" bila sah; selain itu pesan galat pertama. Rentang sama dengan server. */
export function thrError(t: Thr): string {
  for (const k of Object.keys(t) as (keyof Thr)[]) if (typeof t[k] !== "number" || !Number.isFinite(t[k])) return "Semua kolom harus berupa angka.";
  return pair("Suhu CPU/GPU", t.cpu_warn, t.cpu_bad, 40, 120) || pair("Suhu SSD", t.ssd_warn, t.ssd_bad, 40, 110) || pair("Tekanan memori", t.press_warn, t.press_bad, 30, 100) || (t.swap_mb >= 256 && t.swap_mb <= 65536 ? "" : "Swap: antara 256 dan 65536 MB.");
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
