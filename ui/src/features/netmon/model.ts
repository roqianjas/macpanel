// Tipe dan logika murni untuk konektivitas (Tailscale, internet, DNS).
import type { Pt, Status } from "../hardware/model";
import { decimalSep, t } from "../../i18n";

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

const ms = (v: number | null | undefined) => (v == null || !Number.isFinite(v) ? t("fmt.na") : `${v.toFixed(v < 10 ? 1 : 0).replace(".", decimalSep())} ms`);
export const fmtMs = ms;

export function tailscaleStatus(ts: NetSnapshot["tailscale"]): Status {
  if (!ts) return { tone: "mute", text: t("system.hw.unreadable") };
  if (ts.state !== "Running") return { tone: "bad", text: ts.state === "Stopped" ? t("system.net.stopped") : ts.state === "NeedsLogin" ? t("system.net.login") : ts.state };
  if (!ts.self_online) return { tone: "bad", text: t("system.net.notOnline") };
  return { tone: "ok", text: t("system.net.connected") };
}

export function internetStatus(i: NetSnapshot["internet"] | undefined, slowMs: number): Status {
  if (!i) return { tone: "mute", text: t("system.hw.unreadable") };
  if (!i.ok) return { tone: "bad", text: t("system.net.cut") };
  if (i.ms != null && i.ms > slowMs) return { tone: "warn", text: t("system.net.slow") };
  return { tone: "ok", text: t("system.net.connected") };
}

export function dnsStatus(d: NetSnapshot["dns_local"] | undefined): Status {
  if (!d) return { tone: "mute", text: t("system.net.notTestedS") };
  return d.ok ? { tone: "ok", text: t("system.net.answered") } : { tone: "bad", text: t("system.net.failed") };
}

/** Ketersediaan dari sampel yang ada (bukan dari waktu): panel yang mati tidak dihitung "internet turun". */
export function uptimePct(bad: number, total: number): string {
  if (total <= 0) return t("fmt.na");
  const up = ((total - bad) / total) * 100;
  // 99,99 yang dibulatkan ke 100 menyesatkan bila masih ada kegagalan
  const r = bad > 0 && up > 99.9 ? 99.9 : Math.round(up * 10) / 10;
  return `${String(r).replace(".", decimalSep())}%`;
}

export function netFreshness(d: NetData | null, nowSec: number): { tone: "ok" | "warn" | "mute"; text: string } {
  if (!d || !d.now) return { tone: "mute", text: t("system.net.fr.none") };
  const age = Math.max(0, nowSec - d.now.at);
  if (d.stale) return { tone: "warn", text: t("system.net.fr.stale", { ago: t("fmt.ago", { span: t("fmt.min", { n: Math.floor(age / 60) }) }) }) };
  if (d.now.errors.length) return { tone: "warn", text: d.now.errors[d.now.errors.length - 1] };
  return { tone: "ok", text: t("system.net.fr.ok", { ago: age < 60 ? t("fmt.ago", { span: t("fmt.sec", { n: Math.floor(age) }) }) : t("fmt.ago", { span: t("fmt.min", { n: Math.floor(age / 60) }) }) }) };
}
