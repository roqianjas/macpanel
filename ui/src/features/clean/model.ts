// Model analisis disk dan memori: tipe, pengelompokan, perkiraan, dan ringkasan hasil job. Keputusan tingkat keamanan dibuat CLI; UI hanya menampilkan.
import { fmtBytes } from "../../lib";
import { t } from "../../i18n";

export type Tier = "safe" | "careful" | "show" | "detected";
export type DetectedClass = "aman" | "tinjau" | "terkunci";

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
  /** Hanya untuk tier "detected" (cache aplikasi yang ditemukan otomatis). */
  cls?: DetectedClass;
  src?: "apple" | "app" | "lainnya";
  orphan?: boolean;
  running?: boolean;
  loc?: string;
  /** Nama proses terkait yang sedang berjalan; baris ini dilewati panel sampai prosesnya ditutup. */
  busy?: string[];
}

export interface Scan {
  at: number;
  rows: ScanRow[];
  safe_bytes: number;
  careful_bytes: number;
  detected_bytes?: number;
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
  const out: Record<Tier, ScanRow[]> = { safe: [], careful: [], show: [], detected: [] };
  for (const r of scan?.rows ?? []) out[r.tier]?.push(r);
  return out;
}

/** Ukuran gabungan baris yang dipilih. Id yang tidak ada di analisis (kedaluwarsa) diabaikan. */
export function estimate(scan: Scan | null, ids: Iterable<string>): number {
  const by = new Map((scan?.rows ?? []).map((r) => [r.id, r]));
  let n = 0;
  for (const id of ids) {
    const r = by.get(id);
    if (r && r.tier !== "show" && !(r.tier === "detected" && r.cls === "terkunci")) n += r.bytes;
  }
  return n;
}

export function scanAge(at: number, now: number): string {
  const s = Math.max(0, now - at);
  if (s < 90) return t("system.cl.justNow");
  if (s < 3600) return t("fmt.ago", { span: t("fmt.min", { n: Math.round(s / 60) }) });
  if (s < 86400) return t("fmt.ago", { span: t("fmt.hour", { n: Math.round(s / 3600) }) });
  return t("fmt.ago", { span: t("fmt.day", { n: Math.round(s / 86400) }) });
}

/** Hasil analisis lebih dari 1 hari dianggap basi: ukuran bisa sudah berubah. */
export const isStale = (at: number, now: number) => now - at > 86400;

export function ageLabel(days: number | null | undefined): string {
  if (days == null) return t("system.cl.noAge");
  if (days < 1) return t("system.cl.today");
  return t("fmt.ago", { span: t("fmt.day", { n: days }) });
}

export function sizeLabel(r: Pick<ScanRow, "bytes" | "complete">): string {
  return r.complete ? fmtBytes(r.bytes) : `≥ ${t("system.cl.partial", { size: fmtBytes(r.bytes) })}`;
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
  if (!r) return { tone: "warn", text: t("system.cl.runUnreadable") };
  const errs = r.items.filter((i) => i.error).length;
  const done = r.items.length - errs;
  if (done === 0 && r.skipped.length && !errs) {
    return { tone: "warn", text: t("system.cl.nothing", { n: r.skipped.length, reasons: r.skipped.map((s) => s.reason).join("; ") }) };
  }
  const parts = [t("system.cl.freed", { size: fmtBytes(r.bytes), n: done })];
  if (errs) parts.push(t("system.cl.failed", { n: errs }));
  if (r.skipped.length) parts.push(t("system.cl.skippedN", { n: r.skipped.length, reasons: r.skipped.map((s) => s.reason).join("; ") }));
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
  if (freePct == null) return { label: t("system.mem.pr.unknown"), tone: "mute" };
  if (freePct >= 40) return { label: t("system.mem.pr.ok"), tone: "ok" };
  if (freePct >= 20) return { label: t("system.mem.pr.warn"), tone: "warn" };
  return { label: t("system.mem.pr.bad"), tone: "bad" };
}

/** Bagian yang dipakai aplikasi (bukan cache): wired + active + compressed. inactive adalah cache yang bisa dilepas macOS kapan saja. */
export function appUsedMb(m: MemView["mem"]): number | null {
  if (m.wired_mb == null || m.active_mb == null || m.compressed_mb == null) return null;
  return m.wired_mb + m.active_mb + m.compressed_mb;
}

export const PURGE_COMMAND = "sudo purge";

// ---- cache terdeteksi

/** Ambang tampilan awal: yang lebih kecil disembunyikan (sakelar menampilkannya). Ukuran basis 1000 seperti macOS. */
export const SMALL_BYTES = 50 * 1000 * 1000;

/** Kunci terjemahan untuk alasan sebuah baris terdeteksi. Diturunkan dari kelas, bukan dari teks server, supaya ikut bahasa UI. */
export function whyKey(r: ScanRow): string {
  if (r.cls === "terkunci") {
    return r.src === "apple" ? "apple" : r.running ? "running" : "unreadable";
  }
  return r.cls === "aman" ? "orphan" : "review";
}

export const isBusy = (r: ScanRow) => (r.busy?.length ?? 0) > 0;
export const canPick = (r: ScanRow) => r.tier === "detected" && r.cls !== "terkunci" && !isBusy(r);

export const DETECTED_FACETS: { id: string; test: (r: ScanRow) => boolean }[] = [
  { id: "aman", test: (r) => r.cls === "aman" },
  { id: "tinjau", test: (r) => r.cls === "tinjau" },
  { id: "terkunci", test: (r) => r.cls === "terkunci" },
  { id: "yatim", test: (r) => !!r.orphan },
  { id: "apple", test: (r) => r.src === "apple" },
];

/** Id yang boleh dikirim dari hasil filter: tidak pernah memuat baris Terkunci. */
export function pickableIds(rows: readonly ScanRow[]): string[] {
  return rows.filter(canPick).map((r) => r.id);
}

export function sumBytes(rows: readonly ScanRow[], ids: Iterable<string>): number {
  const by = new Map(rows.map((r) => [r.id, r]));
  let n = 0;
  for (const id of ids) {
    const r = by.get(id);
    if (r && canPick(r)) n += r.bytes;
  }
  return n;
}
