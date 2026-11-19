// Tipe dan logika murni untuk konektivitas (Tailscale, internet, DNS).
import type { Pt, Status } from "../hardware/model";
import { decimalSep } from "../../i18n";

export interface NetSnapshot {
  at: number;
  tailscale: { state: string; self_online: boolean; peers: number; peers_online: number; health: string[] } | null;
  internet: { ok: boolean; ms: number | null };
  dns_local: { name: string; ok: boolean; ms: number | null } | null;
  dns_public: { name: string; ok: boolean; ms: number | null } | null;
  errors: string[];
}

export interface NetData {
  now: NetSnapshot | null;
  stale: boolean;
  range: string;
  bucket: number;
  interval: number;
  series: Record<string, Pt[]>;
  samples: number;
  ts_down: number;
  inet_down: number;
  dns_local_down: number;
  dns_local_n: number;
  inet_max_ms: number;
  thresholds: { slow_ms: number };
}

const ms = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? "n/a" : `${v.toFixed(v < 10 ? 1 : 0).replace(".", decimalSep())} ms`);
export const fmtMs = ms;

export function tailscaleStatus(t: NetSnapshot["tailscale"]): Status {
  if (!t) return { tone: "mute", text: "Tidak terbaca" };
  if (t.state !== "Running") return { tone: "bad", text: t.state === "Stopped" ? "Berhenti" : t.state === "NeedsLogin" ? "Perlu login" : t.state };
  if (!t.self_online) return { tone: "bad", text: "Tidak online" };
  return { tone: "ok", text: "Tersambung" };
}

export function internetStatus(i: NetSnapshot["internet"] | undefined, slowMs: number): Status {
  if (!i) return { tone: "mute", text: "Tidak terbaca" };
  if (!i.ok) return { tone: "bad", text: "Terputus" };
  if (i.ms != null && i.ms > slowMs) return { tone: "warn", text: "Lambat" };
  return { tone: "ok", text: "Tersambung" };
}

export function dnsStatus(d: NetSnapshot["dns_local"] | undefined): Status {
  if (!d) return { tone: "mute", text: "Tidak dites" };
  return d.ok ? { tone: "ok", text: "Terjawab" } : { tone: "bad", text: "Gagal" };
}

/** Ketersediaan dari sampel yang ada (bukan dari waktu): panel yang mati tidak dihitung "internet turun". */
export function uptimePct(bad: number, total: number): string {
  if (total <= 0) return "n/a";
  const up = ((total - bad) / total) * 100;
  // 99,99 yang dibulatkan ke 100 menyesatkan bila masih ada kegagalan
  const r = bad > 0 && up > 99.9 ? 99.9 : Math.round(up * 10) / 10;
  return `${String(r).replace(".", decimalSep())}%`;
}

export function netFreshness(d: NetData | null, nowSec: number): { tone: "ok" | "warn" | "mute"; text: string } {
  if (!d || !d.now) return { tone: "mute", text: "Belum ada sampel. Sampel pertama muncul sekitar semenit setelah panel menyala." };
  const age = Math.max(0, nowSec - d.now.at);
  if (d.stale) return { tone: "warn", text: `Data basi: sampel terakhir ${Math.floor(age / 60)} menit lalu. Pemantauan konektivitas mungkin berhenti.` };
  if (d.now.errors.length) return { tone: "warn", text: d.now.errors[d.now.errors.length - 1] };
  return { tone: "ok", text: `Diperbarui ${age < 60 ? `${Math.floor(age)} detik` : `${Math.floor(age / 60)} menit`} lalu.` };
}
