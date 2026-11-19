// Model analisis disk dan memori: tipe, pengelompokan, perkiraan, dan ringkasan hasil job. Keputusan tingkat keamanan dibuat CLI; UI hanya menampilkan.
import { fmtBytes } from "../../lib";

export type Tier = "safe" | "careful" | "show";

export interface ScanRow {
  id: string;
  label: string;
  tier: Tier;
  bytes: number;
  complete: boolean;
  why: string;
  cost: string;
  paths: string[];
  items?: string[];
  age_days?: number | null;
}

export interface Scan {
  at: number;
  rows: ScanRow[];
  safe_bytes: number;
  careful_bytes: number;
  disk_free: number;
}

export interface RunItem {
  id: string;
  label: string;
  tier: Tier;
  bytes: number;
  error: string | null;
}

export interface RunResult {
  dry_run: boolean;
  items: RunItem[];
  skipped: { id: string; reason: string }[];
  bytes: number;
}

export function group(scan: Scan | null): Record<Tier, ScanRow[]> {
  const out: Record<Tier, ScanRow[]> = { safe: [], careful: [], show: [] };
  for (const r of scan?.rows ?? []) out[r.tier]?.push(r);
  return out;
}

/** Ukuran gabungan baris yang dipilih. Id yang tidak ada di analisis (kedaluwarsa) diabaikan. */
export function estimate(scan: Scan | null, ids: Iterable<string>): number {
  const by = new Map((scan?.rows ?? []).map((r) => [r.id, r]));
  let n = 0;
  for (const id of ids) {
    const r = by.get(id);
    if (r && r.tier !== "show") n += r.bytes;
  }
  return n;
}

export function scanAge(at: number, now: number): string {
  const s = Math.max(0, now - at);
  if (s < 90) return "baru saja";
  if (s < 3600) return `${Math.round(s / 60)} menit lalu`;
  if (s < 86400) return `${Math.round(s / 3600)} jam lalu`;
  return `${Math.round(s / 86400)} hari lalu`;
}

/** Hasil analisis lebih dari 1 hari dianggap basi: ukuran bisa sudah berubah. */
export const isStale = (at: number, now: number) => now - at > 86400;

export function ageLabel(days: number | null | undefined): string {
  if (days == null) return "umur tidak terbaca";
  if (days < 1) return "dibuat hari ini";
  return `${days} hari lalu`;
}

export function sizeLabel(r: Pick<ScanRow, "bytes" | "complete">): string {
  return r.complete ? fmtBytes(r.bytes) : `≥ ${fmtBytes(r.bytes)} (sebagian tidak terbaca)`;
}

/** Keluaran job `lab clean run`: JSON di baris terakhir. Null bila tidak terbaca (jangan menebak). */
export function parseRun(output: string): RunResult | null {
  const lines = output.trim().split("\n").filter(Boolean);
  for (let i = lines.length - 1; i >= 0; i--) {
    try {
      const v = JSON.parse(lines[i]);
      if (v && Array.isArray(v.items) && Array.isArray(v.skipped)) return v as RunResult;
    } catch {
      /* bukan JSON: coba baris sebelumnya */
    }
  }
  return null;
}

export function runSummary(r: RunResult | null): { tone: "ok" | "warn" | "bad"; text: string } {
  if (!r) return { tone: "warn", text: "Selesai, tetapi hasilnya tidak terbaca. Jalankan analisis ulang untuk melihat keadaan sebenarnya." };
  const errs = r.items.filter((i) => i.error).length;
  const done = r.items.length - errs;
  const parts = [`${fmtBytes(r.bytes)} dibebaskan dari ${done} entri`];
  if (errs) parts.push(`${errs} entri sebagian gagal`);
  if (r.skipped.length) parts.push(`${r.skipped.length} dilewati (${r.skipped.map((s) => s.reason).join("; ")})`);
  return { tone: errs || r.skipped.length ? "warn" : "ok", text: parts.join(", ") + "." };
}

// ---- memori

export interface MemGroup {
  label: string;
  mb: number;
  procs: number;
  pids: number[];
}

export interface MemSuggestion {
  id: string;
  label: string;
  action: "stop";
  mb: number | null;
  text: string;
}

export interface MemView {
  mem: { total_mb?: number; wired_mb?: number; active_mb?: number; inactive_mb?: number; compressed_mb?: number; free_mb?: number; swap_used_mb?: number; pressure_free_pct?: number };
  groups: MemGroup[];
  suggestions: MemSuggestion[];
  purge_note: string;
}

/** Tekanan memori dari "persentase memori bebas" macOS (makin tinggi makin lega). Ambang adalah pedoman, bukan angka Apple. */
export function pressure(freePct: number | undefined): { label: string; tone: "ok" | "warn" | "bad" | "mute" } {
  if (freePct == null) return { label: "Tidak terbaca", tone: "mute" };
  if (freePct >= 40) return { label: "Lega", tone: "ok" };
  if (freePct >= 20) return { label: "Sedang", tone: "warn" };
  return { label: "Tertekan", tone: "bad" };
}

/** Bagian yang dipakai aplikasi (bukan cache): wired + active + compressed. inactive adalah cache yang bisa dilepas macOS kapan saja. */
export function appUsedMb(m: MemView["mem"]): number | null {
  if (m.wired_mb == null || m.active_mb == null || m.compressed_mb == null) return null;
  return m.wired_mb + m.active_mb + m.compressed_mb;
}

export const PURGE_COMMAND = "sudo purge";
