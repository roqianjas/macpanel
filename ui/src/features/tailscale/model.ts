import { t } from "../../i18n";

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
  running: { tone: "ok", get text() { return t("system.ts.ph.running"); } },
  stopped: { tone: "warn", get text() { return t("system.ts.ph.stopped"); } },
  login: { tone: "warn", get text() { return t("system.ts.ph.login"); } },
  missing: { tone: "mute", get text() { return t("system.ts.ph.missing"); } },
  unknown: { tone: "warn", get text() { return t("system.ts.ph.unknown"); } },
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
  { id: "install", get title() { return t("system.ts.a.install"); }, get why() { return t("system.ts.a.install.why"); }, plan: "lab tailscale install", run: "lab tailscale install --apply", danger: false, sudo: true },
  { id: "login", get title() { return t("system.ts.a.login"); }, get why() { return t("system.ts.a.login.why"); }, run: "tailscale up", danger: false, sudo: false },
  { id: "start", get title() { return t("system.ts.a.start"); }, get why() { return t("system.ts.a.start.why"); }, plan: "lab tailscale start", run: "lab tailscale start --apply", danger: false, sudo: true },
  { id: "restart", get title() { return t("system.ts.a.restart"); }, get why() { return t("system.ts.a.restart.why"); }, plan: "lab tailscale restart", run: "lab tailscale restart --apply", danger: false, sudo: true },
  { id: "down", get title() { return t("system.ts.a.down"); }, get why() { return t("system.ts.a.down.why"); }, plan: "lab tailscale down", run: "lab tailscale down --apply", danger: true, sudo: false },
  { id: "stop", get title() { return t("system.ts.a.stop"); }, get why() { return t("system.ts.a.stop.why"); }, plan: "lab tailscale stop", run: "lab tailscale stop --apply", danger: true, sudo: true },
  { id: "uninstall", get title() { return t("system.ts.a.uninstall"); }, get why() { return t("system.ts.a.uninstall.why"); }, plan: "lab uninstall --level 4", run: "lab uninstall --level 4 --apply", danger: true, sudo: true },
];

/** Tindakan yang masuk akal untuk keadaan sekarang; yang lain tetap tampil tetapi diredupkan dengan alasannya. */
export function applicable(a: Action, p: Phase): { ok: boolean; reason: string } {
  const need = (ok: boolean, reason: string) => ({ ok, reason: ok ? "" : reason });
  switch (a.id) {
    case "install":
      return need(p === "missing", t("system.ts.r.installed"));
    case "login":
      return need(p !== "missing", t("system.ts.r.installFirst"));
    case "start":
      return need(p === "stopped" || p === "unknown", p === "missing" ? t("system.ts.r.installFirst") : t("system.ts.r.running"));
    case "restart":
      return need(p !== "missing", t("system.ts.r.installFirst"));
    case "down":
      return need(p === "running", p === "missing" ? t("system.ts.r.installFirst") : t("system.ts.r.notConnected"));
    case "stop":
    case "uninstall":
      return need(p !== "missing", t("system.ts.r.notBrew"));
    default:
      return { ok: true, reason: "" };
  }
}
