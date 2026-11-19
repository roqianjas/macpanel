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
    title: "Panel saja",
    what: ["LaunchAgent panel, proxy npm, backup harian, dan logrotate", "firewall dari lab firewall (kunci RustDesk tetap terpasang)", "kunci enkripsi panel di Keychain, tautan nginx.conf, cache dan log panel"],
    keeps: "Situs, database, aplikasi, backup, dan pengaturan tetap ada; nginx dan database tetap berjalan.",
    danger: false,
  },
  {
    id: 2,
    title: "Panel + aplikasi dihentikan",
    what: ["semua isi tingkat 1", "nginx, PHP-FPM, MariaDB/MySQL/PostgreSQL, Redis, Mailpit, Netdata, dnsmasq, Colima, 9router, dan Kuma dihentikan dan dilepas dari login", "kunci firewall RustDesk dilepas"],
    keeps: "Paket Homebrew dan semua data (situs, database, backup) tetap ada di disk.",
    danger: false,
  },
  {
    id: 3,
    title: "Semuanya",
    what: ["semua isi tingkat 2", "backup akhir ke folder di luar ~/MacPanel, diverifikasi sebelum apa pun dihapus", "paket Homebrew bundel dihapus, data database dihapus, CA mkcert dilepas", "~/MacPanel dan ~/.config/macminiops dihapus"],
    keeps: "Hanya yang ada di folder backup akhir. Tailscale, RustDesk, Hermes, dan Homebrew tidak disentuh.",
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
