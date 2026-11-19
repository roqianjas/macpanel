import { t } from "../../i18n";

// Panduan uninstall: panel hanya menampilkan perintah; tidak menjalankan apa pun (butuh sudo dan menghapus dirinya sendiri).
export interface Level {
  id: 1 | 2 | 3;
  title: string;
  what: string[];
  keeps: string;
  danger: boolean;
}

export const LEVELS: Level[] = [
  {
    id: 1,
    get title() { return t("system.un.L1.title"); },
    get what() { return [t("system.un.L1.w1"), t("system.un.L1.w2"), t("system.un.L1.w3")]; },
    get keeps() { return t("system.un.L1.keeps"); },
    danger: false,
  },
  {
    id: 2,
    get title() { return t("system.un.L2.title"); },
    get what() { return [t("system.un.L2.w1"), t("system.un.L2.w2"), t("system.un.L2.w3")]; },
    get keeps() { return t("system.un.L2.keeps"); },
    danger: false,
  },
  {
    id: 3,
    get title() { return t("system.un.L3.title"); },
    get what() { return [t("system.un.L3.w1"), t("system.un.L3.w2"), t("system.un.L3.w3"), t("system.un.L3.w4")]; },
    get keeps() { return t("system.un.L3.keeps"); },
    danger: true,
  },
];

/** Perintah: rencana dulu (aman, tanpa --apply), lalu menjalankan. */
export function planCommand(level: 1 | 2 | 3, backupDir = "~/Backups-macpanel"): string {
  return level === 3 ? `lab uninstall --level 3 --backup-dir ${backupDir}` : `lab uninstall --level ${level}`;
}

export function applyCommand(level: 1 | 2 | 3, backupDir = "~/Backups-macpanel"): string {
  return `${planCommand(level, backupDir)} --apply`;
}

// Tailscale berdiri sendiri (bukan bagian tingkat 1-3): panel dan SSH Anda terjangkau lewat tailnet, jadi melepasnya memutus akses jarak jauh.
export const TAILSCALE = {
  statusCmd: "lab tailscale status",
  installPlan: "lab tailscale install",
  installApply: "lab tailscale install --apply",
  loginCmd: "tailscale up",
  removePlan: "lab uninstall --level 4",
  removeApply: "lab uninstall --level 4 --apply",
  get removeWarn() {
    return [t("system.un.W1"), t("system.un.W2"), t("system.un.W3"), t("system.un.W4")];
  },
} as const;
