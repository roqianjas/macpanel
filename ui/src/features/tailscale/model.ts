// Kelola Tailscale: panel hanya membaca status dan menampilkan perintah; sudo -S -p '' dijalankan pemilik di terminal.
export interface Tailnet {
  state: string;
  self_online: boolean;
  peers: number;
  peers_online: number;
  health: string[];
  ip: string | null;
  name: string | null;
  version: string | null;
}
export interface TsInfo {
  formula: boolean;
  daemon_plist: boolean;
  tailnet: Tailnet | null;
  error: string | null;
}

export type Phase = "missing" | "running" | "stopped" | "login" | "unknown";

export function phase(i: TsInfo): Phase {
  if (!i.formula) return "missing";
  const s = i.tailnet?.state;
  if (s === "Running") return "running";
  if (s === "NeedsLogin" || s === "NeedsMachineAuth") return "login";
  if (s === "Stopped" || s === "Starting" || s === "NoState") return "stopped";
  return "unknown";
}

export const PHASE_TEXT: Record<Phase, { tone: "ok" | "warn" | "bad" | "mute"; text: string }> = {
  running: { tone: "ok", text: "Tersambung" },
  stopped: { tone: "warn", text: "Terputus atau daemon berhenti" },
  login: { tone: "warn", text: "Perlu login" },
  missing: { tone: "mute", text: "Belum terpasang (lewat Homebrew)" },
  unknown: { tone: "warn", text: "Status tidak terbaca" },
};

export interface Action {
  id: string;
  title: string;
  why: string;
  plan?: string; // tanpa --apply: hanya rencana
  run: string;
  danger: boolean; // memutus akses jarak jauh
  sudo: boolean;
}

export const ACTIONS: Action[] = [
  { id: "install", title: "Pasang", why: "Memasang paket Homebrew dan mendaftarkan daemon sistem (hidup sejak boot). Login ke tailnet dilakukan terpisah.", plan: "lab tailscale install", run: "lab tailscale install --apply", danger: false, sudo: true },
  { id: "login", title: "Login atau sambungkan", why: "Membuka browser untuk masuk ke tailnet Anda. Hanya Anda yang bisa; tidak pernah dijalankan otomatis. Dipakai juga untuk menyambung lagi setelah 'putuskan'.", run: "tailscale up", danger: false, sudo: false },
  { id: "start", title: "Jalankan daemon", why: "Menyalakan tailscaled bila berhenti.", plan: "lab tailscale start", run: "lab tailscale start --apply", danger: false, sudo: true },
  { id: "restart", title: "Mulai ulang daemon", why: "Berguna bila macet. Tailnet putus beberapa detik, lalu kembali sendiri.", plan: "lab tailscale restart", run: "lab tailscale restart --apply", danger: false, sudo: true },
  { id: "down", title: "Putuskan sambungan (daemon tetap hidup)", why: "Mac mini keluar dari tailnet sementara. Sambung lagi dengan 'tailscale up'.", plan: "lab tailscale down", run: "lab tailscale down --apply", danger: true, sudo: false },
  { id: "stop", title: "Hentikan daemon", why: "Mematikan tailscaled sepenuhnya.", plan: "lab tailscale stop", run: "lab tailscale stop --apply", danger: true, sudo: true },
  { id: "uninstall", title: "Lepas (uninstall)", why: "Menghentikan daemon dan menghapus paket. Berdiri sendiri, tidak memengaruhi panel, situs, atau database.", plan: "lab uninstall --level 4", run: "lab uninstall --level 4 --apply", danger: true, sudo: true },
];

/** Tindakan yang masuk akal untuk keadaan sekarang; yang lain tetap tampil tetapi diredupkan dengan alasannya. */
export function applicable(a: Action, p: Phase): { ok: boolean; reason: string } {
  const need = (ok: boolean, reason: string) => ({ ok, reason: ok ? "" : reason });
  switch (a.id) {
    case "install":
      return need(p === "missing", "Sudah terpasang.");
    case "login":
      return need(p !== "missing", "Pasang dulu.");
    case "start":
      return need(p === "stopped" || p === "unknown", p === "missing" ? "Pasang dulu." : "Sudah berjalan.");
    case "restart":
      return need(p !== "missing", "Pasang dulu.");
    case "down":
      return need(p === "running", p === "missing" ? "Pasang dulu." : "Tidak sedang tersambung.");
    case "stop":
    case "uninstall":
      return need(p !== "missing", "Tidak terpasang lewat Homebrew.");
    default:
      return { ok: true, reason: "" };
  }
}
