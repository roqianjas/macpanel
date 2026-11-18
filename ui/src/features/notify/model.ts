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

export const topicError = (t: string): string => (TOPIC_RE.test(t) ? "" : "Topik 16-64 karakter: huruf, angka, _ atau -. Pilih yang sulit ditebak, topik berfungsi seperti kata sandi.");

/** Alamat server: harus https dan bukan alamat internal. Server tetap memeriksa ulang; ini hanya agar galat muncul sebelum menyimpan. */
export function serverError(raw: string): string {
  let u: URL;
  try {
    u = new URL(raw.trim());
  } catch {
    return "Alamat server tidak valid.";
  }
  if (u.protocol !== "https:") return "Alamat server harus diawali https://";
  if (u.username || u.password) return "Alamat server tidak boleh memuat nama pengguna atau kata sandi.";
  if (u.search || u.hash) return "Alamat server tidak boleh memuat query atau fragment.";
  const h = u.hostname.toLowerCase();
  if (h === "localhost" || h.endsWith(".local") || h.endsWith(".lab") || h.endsWith(".internal")) return "Alamat server tidak boleh nama internal.";
  if (/^(127\.|10\.|192\.168\.|169\.254\.|0\.)/.test(h) || /^172\.(1[6-9]|2\d|3[01])\./.test(h) || h === "[::1]") return "Alamat server tidak boleh alamat internal.";
  return "";
}

const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-";

/** Topik acak 24 karakter (144 bit) dari sumber acak aman. Alfabet 64 karakter, jadi pemetaan byte tanpa bias (256 habis dibagi 64). */
export function genTopic(rand: (n: number) => Uint8Array = (n) => crypto.getRandomValues(new Uint8Array(n))): string {
  return Array.from(rand(24), (b) => ALPHABET[b % 64]).join("");
}

export function lastText(l: NotifyCfg["last"] | undefined, nowSec: number): string {
  if (!l || !l.at) return "Belum pernah mengirim.";
  const ago = Math.max(0, nowSec - l.at);
  const when = ago < 60 ? `${Math.floor(ago)} detik lalu` : ago < 3600 ? `${Math.floor(ago / 60)} menit lalu` : `${Math.floor(ago / 3600)} jam lalu`;
  return l.ok ? `Pengiriman terakhir berhasil, ${when}.` : `Pengiriman terakhir gagal, ${when}: ${l.error || "alasan tidak diketahui"}`;
}
