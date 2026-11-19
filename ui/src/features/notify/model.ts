import { t } from "../../i18n";

// Logika murni untuk pengaturan notifikasi ponsel (ntfy). Aturan topik dan server sama dengan server.

export interface NotifyCfg {
  enabled: boolean;
  server: string;
  min: "bad" | "warn";
  topic_set: boolean;
  last: { at: number; ok: boolean; error?: string };
  default_server: string;
}

export const TOPIC_RE = /^[A-Za-z0-9_-]{16,64}$/;

export const topicError = (topic: string): string => (TOPIC_RE.test(topic) ? "" : t("settings.notify.topicErr"));

/** Alamat server: harus https dan bukan alamat internal. Server tetap memeriksa ulang; ini hanya agar galat muncul sebelum menyimpan. */
export function serverError(raw: string): string {
  let u: URL;
  try {
    u = new URL(raw.trim());
  } catch {
    return t("settings.notify.srvInvalid");
  }
  if (u.protocol !== "https:") return t("settings.notify.srvHttps");
  if (u.username || u.password) return t("settings.notify.srvCreds");
  if (u.search || u.hash) return t("settings.notify.srvQuery");
  const h = u.hostname.toLowerCase();
  if (h === "localhost" || h.endsWith(".local") || h.endsWith(".lab") || h.endsWith(".internal")) return t("settings.notify.srvInternalName");
  if (/^(127\.|10\.|192\.168\.|169\.254\.|0\.)/.test(h) || /^172\.(1[6-9]|2\d|3[01])\./.test(h) || h === "[::1]") return t("settings.notify.srvInternalIp");
  return "";
}

const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-";

/** Topik acak 24 karakter (144 bit) dari sumber acak aman. Alfabet 64 karakter, jadi pemetaan byte tanpa bias (256 habis dibagi 64). */
export function genTopic(rand: (n: number) => Uint8Array = (n) => crypto.getRandomValues(new Uint8Array(n))): string {
  return Array.from(rand(24), (b) => ALPHABET[b % 64]).join("");
}

export function lastText(l: NotifyCfg["last"] | undefined, nowSec: number): string {
  if (!l || !l.at) return t("settings.notify.never");
  const ago = Math.max(0, nowSec - l.at);
  const when = ago < 60 ? t("fmt.ago", { span: t("fmt.sec", { n: Math.floor(ago) }) }) : ago < 3600 ? t("fmt.ago", { span: t("fmt.min", { n: Math.floor(ago / 60) }) }) : t("fmt.ago", { span: t("fmt.hour", { n: Math.floor(ago / 3600) }) });
  return l.ok ? t("settings.notify.lastOk", { when }) : t("settings.notify.lastFail", { when, reason: l.error || t("settings.notify.unknown") });
}
