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
  thresholds: { cpu_warn: number; cpu_bad: number; ssd_warn: number; ssd_bad: number; press_warn: number; press_bad: number };
}

export type Tone = "ok" | "warn" | "bad" | "mute";
export interface Status {
  tone: Tone;
  text: string;
}

/** Koma desimal Indonesia. null atau NaN tampil "n/a": tidak ada angka karangan. */
export function num(v: number | null | undefined, digits = 0): string {
  if (v == null || !Number.isFinite(v)) return "n/a";
  return v.toFixed(digits).replace(".", ",");
}

export const fmtTemp = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? "n/a" : `${num(v, 0)} °C`);

export function levelStatus(v: number | null | undefined, warn: number, bad: number, labels = { ok: "Normal", warn: "Panas", bad: "Sangat panas" }): Status {
  if (v == null || !Number.isFinite(v)) return { tone: "mute", text: "Tidak terbaca" };
  if (v >= bad) return { tone: "bad", text: labels.bad };
  if (v >= warn) return { tone: "warn", text: labels.warn };
  return { tone: "ok", text: labels.ok };
}

export const tempStatus = (v: number | null | undefined, warn: number, bad: number) => levelStatus(v, warn, bad);

export const pressureStatus = (v: number | null | undefined, warn: number, bad: number) => levelStatus(v, warn, bad, { ok: "Normal", warn: "Tinggi", bad: "Sangat tinggi" });

export function smartStatus(smart: string | null | undefined): Status {
  switch (smart) {
    case "Verified":
      return { tone: "ok", text: "Sehat" };
    case "Not Supported":
      return { tone: "mute", text: "Tidak didukung" };
    case "":
    case null:
    case undefined:
      return { tone: "mute", text: "Tidak diketahui" };
    default:
      return { tone: "bad", text: "Bermasalah" };
  }
}

export function throttleStatus(t: HwSnapshot["thermal"]): Status {
  if (!t) return { tone: "mute", text: "Tidak terbaca" };
  return t.throttled ? { tone: "bad", text: `Dibatasi ${t.speed_limit}%` } : { tone: "ok", text: "Tidak dibatasi" };
}

/** Kipas: persen dari batas atas kalau batas diketahui. 0 rpm tetap angka nyata (kipas Mac mini boleh berhenti saat dingin). */
export function fanText(f: HwSnapshot["fan"]): string {
  if (!f) return "n/a";
  if (f.max && f.max > 0) return `${f.rpm} rpm (${Math.round((f.rpm / f.max) * 100)}% dari maks ${f.max})`;
  return `${f.rpm} rpm`;
}

/** Keadaan keseluruhan data: yang ditampilkan di atas kartu, supaya angka basi atau sumber cadangan tidak terbaca sebagai normal. */
export function freshness(d: HwData | null, nowSec: number): { tone: Tone; text: string } {
  if (!d || !d.now) return { tone: "mute", text: "Belum ada sampel. Sampel pertama muncul sekitar 30 detik setelah panel menyala." };
  const age = Math.max(0, nowSec - d.now.at);
  if (d.stale) return { tone: "warn", text: `Data basi: sampel terakhir ${Math.floor(age / 60)} menit lalu. Pemantauan perangkat keras mungkin berhenti.` };
  if (d.now.source === "none") return { tone: "warn", text: d.now.errors[d.now.errors.length - 1] || "Tidak ada sumber suhu." };
  if (d.now.source === "netdata") return { tone: "warn", text: "Memakai Netdata (macmon tidak tersedia). Skala suhu berbeda dari macmon, jadi jangan dibandingkan dengan riwayat lama." };
  return { tone: "ok", text: `Sumber: macmon. Diperbarui ${age < 60 ? `${Math.floor(age)} detik` : `${Math.floor(age / 60)} menit`} lalu.` };
}

/** Kalimat prakiraan disk. Tidak pernah menampilkan angka bila datanya belum cukup. */
export function forecastText(f: HwForecast): { tone: Tone; text: string } {
  if (!f.ok) return { tone: "mute", text: f.reason || "Data belum cukup." };
  const rate = `${num(f.pct_per_day, 2)}%/hari${f.gb_per_day != null ? ` (sekitar ${num(f.gb_per_day, 1)} GB/hari)` : ""}`;
  if (f.days_to_full == null) return { tone: "ok", text: f.reason ? `Pemakaian ${f.reason}. Tren ${rate}.` : `Tren ${rate}.` };
  const low = f.r2 < 0.5 ? " Tren naik-turun tidak beraturan, jadi perkiraan ini kasar." : "";
  const tone: Tone = f.r2 >= 0.5 && f.days_to_full < 3 ? "bad" : f.r2 >= 0.5 && f.days_to_full < 14 ? "warn" : "ok";
  return { tone, text: `Bila tren ${rate} berlanjut, disk penuh sekitar ${num(f.days_to_full, 0)} hari lagi.${low}` };
}

/** Kesimpulan satu kalimat dari beberapa status. Status "tidak terbaca" tidak dihitung sehat. */
export function verdict(rows: Status[]): Status {
  const bad = rows.filter((r) => r.tone === "bad").length;
  const warn = rows.filter((r) => r.tone === "warn").length;
  const ok = rows.filter((r) => r.tone === "ok").length;
  if (bad) return { tone: "bad", text: `${bad} komponen perlu dicek sekarang` };
  if (warn) return { tone: "warn", text: `${warn} komponen perlu diperhatikan` };
  if (ok === 0) return { tone: "mute", text: "Belum ada data komponen" };
  return { tone: "ok", text: rows.some((r) => r.tone === "mute") ? "Yang terbaca normal, sebagian belum terbaca" : "Semua komponen normal" };
}
