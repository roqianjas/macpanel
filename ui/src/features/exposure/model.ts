// Model paparan layanan: tipe, label status, dan pengurutan. Keputusan "perlu perhatian" dibuat server (status), UI hanya menampilkan.
import type { Tone } from "../../ui";

export type Scope = "loopback" | "tailnet" | "all" | "lan";
export type Status = "loopback" | "tailnet" | "system" | "guarded" | "blocked" | "unknown" | "allowed" | "flagged";

export interface ExItem {
  key: string;
  proc: string;
  pid: number | null;
  port: number;
  bind: string;
  scope: Scope;
  system: boolean;
  lan_reachable: boolean | null;
  pf_guarded?: boolean;
  service: string;
  hidden: boolean;
  status: Status;
}

export interface ExView {
  now: { at: number; lan_ip: string | null; errors: string[] } | null;
  stale: boolean;
  items: ExItem[];
  allowed: string[];
  defaults: string[];
  findings: number;
  confirm_samples: number;
  interval: number;
  can_edit: boolean;
}

/** Teks status yang menyebut artinya, tidak hanya warna. */
export function statusInfo(s: Status): { label: string; tone: Tone; hint: string } {
  switch (s) {
    case "flagged":
      return { label: "Perlu perhatian", tone: "bad", hint: "Terbuka ke jaringan rumah dan tidak ada di daftar izin." };
    case "allowed":
      return { label: "Diizinkan", tone: "ok", hint: "Terbuka ke jaringan rumah, tetapi Anda menandainya disengaja." };
    case "system":
      return { label: "Sistem macOS", tone: "mute", hint: "Layanan bawaan macOS." };
    case "tailnet":
      return { label: "Hanya tailnet", tone: "ok", hint: "Hanya mendengar di alamat Tailscale." };
    case "guarded":
      return { label: "Dijaga pf", tone: "warn", hint: "Ada aturan firewall pf untuk port ini, tetapi uji dari mesin sendiri tidak bisa membuktikan aturannya aktif. Periksa dengan: sudo pfctl -s Anchors" };
    case "blocked":
      return { label: "Diblokir", tone: "ok", hint: "Mendengar di semua antarmuka, tetapi sambungan dari LAN gagal (mis. firewall)." };
    case "unknown":
      return { label: "Belum teruji", tone: "warn", hint: "Keterjangkauan dari LAN tidak dapat diuji." };
    default:
      return { label: "Loopback", tone: "ok", hint: "Hanya dari mesin ini." };
  }
}

const ORDER: Record<Status, number> = { flagged: 0, unknown: 1, guarded: 2, allowed: 3, blocked: 4, system: 5, tailnet: 6, loopback: 7 };

/** Yang perlu perhatian dulu, lalu yang terbuka tetapi diizinkan; urutan stabil menurut port. */
export function sortItems(items: readonly ExItem[]): ExItem[] {
  return items.slice().sort((a, b) => ORDER[a.status] - ORDER[b.status] || a.port - b.port || a.proc.localeCompare(b.proc));
}

/** Yang ditampilkan di tabel: semua selain loopback (yang hanya dari mesin ini cukup dihitung). */
export const visibleItems = (items: readonly ExItem[]): ExItem[] => sortItems(items.filter((i) => i.status !== "loopback"));
export const loopbackCount = (items: readonly ExItem[]): number => items.filter((i) => i.status === "loopback").length;

/** Nama yang dibaca manusia: proses, atau tebakan layanan bagi proses yang tidak terlihat. */
export function displayName(i: Pick<ExItem, "proc" | "hidden" | "service">): string {
  if (i.hidden) return i.service ? `Proses tidak terlihat (${i.service})` : "Proses tidak terlihat";
  return i.proc;
}

/** Kalimat ringkasan di atas tabel. */
export function summary(v: Pick<ExView, "now" | "stale" | "findings" | "items">): string {
  if (!v.now) return "Belum ada pemeriksaan. Pertama dijalankan beberapa menit setelah panel menyala.";
  if (v.stale) return "Pemeriksaan berhenti; data di bawah sudah lama.";
  if (v.now.lan_ip === null) return "IP LAN tidak terbaca, jadi keterjangkauan dari jaringan rumah tidak dapat diuji.";
  const flagged = v.items.filter((i) => i.status === "flagged").length;
  if (flagged > 0) return `${flagged} layanan terbuka ke jaringan rumah dan perlu perhatian.`;
  if (v.findings > 0) return `${v.findings} layanan terbuka ke jaringan rumah dan menunggu konfirmasi pemeriksaan berikutnya.`;
  return "Tidak ada layanan tak disengaja yang terbuka ke jaringan rumah.";
}

/** Entri daftar izin untuk satu item ("mailpit:8025"); sama dengan Key di server. */
export const entryOf = (i: Pick<ExItem, "key">): string => i.key;

export const isDefaultEntry = (e: string, defaults: readonly string[]): boolean => defaults.includes(e);
