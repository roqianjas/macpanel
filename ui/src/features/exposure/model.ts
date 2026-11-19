// Model paparan layanan: tipe, label status, dan pengurutan. Keputusan "perlu perhatian" dibuat server (status), UI hanya menampilkan.
import type { Tone } from "../../ui";
import { t } from "../../i18n";

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
      return { label: t("system.ex.st.flagged"), tone: "bad", hint: t("system.ex.st.flagged.hint") };
    case "allowed":
      return { label: t("system.ex.st.allowed"), tone: "ok", hint: t("system.ex.st.allowed.hint") };
    case "system":
      return { label: t("system.ex.st.system"), tone: "mute", hint: t("system.ex.st.system.hint") };
    case "tailnet":
      return { label: t("system.ex.st.tailnet"), tone: "ok", hint: t("system.ex.st.tailnet.hint") };
    case "guarded":
      return { label: t("system.ex.st.guarded"), tone: "warn", hint: t("system.ex.st.guarded.hint") };
    case "blocked":
      return { label: t("system.ex.st.blocked"), tone: "ok", hint: t("system.ex.st.blocked.hint") };
    case "unknown":
      return { label: t("system.ex.st.unknown"), tone: "warn", hint: t("system.ex.st.unknown.hint") };
    default:
      return { label: t("system.ex.st.loopback"), tone: "ok", hint: t("system.ex.st.loopback.hint") };
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
  if (i.hidden) return i.service ? t("system.ex.hiddenSvc", { service: i.service }) : t("system.ex.hidden");
  return i.proc;
}

/** Kalimat ringkasan di atas tabel. */
export function summary(v: Pick<ExView, "now" | "stale" | "findings" | "items">): string {
  if (!v.now) return t("system.ex.sum.none");
  if (v.stale) return t("system.ex.sum.stale");
  if (v.now.lan_ip === null) return t("system.ex.sum.noip");
  const flagged = v.items.filter((i) => i.status === "flagged").length;
  if (flagged > 0) return t("system.ex.sum.flagged", { n: flagged });
  if (v.findings > 0) return t("system.ex.sum.pending", { n: v.findings });
  return t("system.ex.sum.ok");
}

/** Entri daftar izin untuk satu item ("mailpit:8025"); sama dengan Key di server. */
export const entryOf = (i: Pick<ExItem, "key">): string => i.key;

export const isDefaultEntry = (e: string, defaults: readonly string[]): boolean => defaults.includes(e);

/** Perintah terminal untuk menutup port dari LAN lewat pf. Panel tidak menjalankannya (butuh sudo); null bila tidak masuk akal. */
export function closeCommand(i: Pick<ExItem, "port" | "status">): string | null {
  if (!Number.isInteger(i.port) || i.port < 1 || i.port > 65535 || i.port === 22) return null;
  if (i.status !== "flagged" && i.status !== "unknown") return null;
  return `lab firewall block ${i.port}`;
}
