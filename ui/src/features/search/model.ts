// Pencarian global: indeks (halaman, tab, situs, layanan, database, tugas, dokumen) dan peringkat. Murni, tanpa DOM.

export type Group = "Halaman" | "Website" | "Layanan" | "Database" | "Tugas" | "Dokumen";
export const GROUP_ORDER: Group[] = ["Halaman", "Website", "Layanan", "Database", "Tugas", "Dokumen"];

export interface Item {
  id: string;
  group: Group;
  label: string;
  hint?: string;
  hash: string; // tujuan, mis. "#/sistem/tailscale"
  keywords?: string; // kata lain yang boleh menemukan item ini (tanpa tampil)
  admin?: boolean;
}

// Halaman dan tab yang bisa dituju langsung. Kata kunci memuat istilah yang dicari orang, bukan hanya nama menunya.
export const PAGES: Item[] = [
  { id: "p-ringkasan", group: "Halaman", label: "Ringkasan", hash: "#/ringkasan", hint: "Kondisi mesin sekilas", keywords: "beranda home dashboard status" },
  { id: "p-pantau", group: "Halaman", label: "Pantau", hash: "#/pantau", hint: "Grafik CPU, memori, disk, jaringan", keywords: "monitoring grafik cpu ram suhu jaringan" },
  { id: "p-website", group: "Halaman", label: "Website", hash: "#/website", hint: "Situs .lab, SSL, rewrite", keywords: "situs domain nginx ssl proxy vhost" },
  { id: "p-aplikasi", group: "Halaman", label: "Aplikasi", hash: "#/aplikasi", hint: "Layanan: nyalakan, hentikan, log", keywords: "layanan service nginx redis php-fpm mysql start stop restart" },
  { id: "p-database", group: "Halaman", label: "Database", hash: "#/database", hint: "MySQL, PostgreSQL, Redis", keywords: "mysql mariadb postgres postgresql redis dump user sql" },
  { id: "p-runtime", group: "Halaman", label: "PHP dan runtime", hash: "#/runtime", hint: "Versi PHP, Node, Python, Composer", keywords: "php node python composer versi ekstensi fpm mise" },
  { id: "p-docker", group: "Halaman", label: "Docker", hash: "#/docker", hint: "Container, compose, image", keywords: "colima container image compose" },
  { id: "p-tugas", group: "Halaman", label: "Worker dan cron", hash: "#/tugas", hint: "Proses latar dan tugas terjadwal", keywords: "supervisor queue crontab jadwal schedule launchd" },
  { id: "p-tugas-job", group: "Halaman", label: "Cron job (terjadwal)", hash: "#/tugas/job", hint: "Worker dan cron", keywords: "crontab jadwal shell skrip url php" },
  { id: "p-tugas-worker", group: "Halaman", label: "Worker (queue, daemon)", hash: "#/tugas/worker", hint: "Worker dan cron", keywords: "supervisor queue horizon artisan" },
  { id: "p-sistem", group: "Halaman", label: "Sistem", hash: "#/sistem", hint: "Keamanan, proses, disk, DNS", keywords: "keamanan firewall ssh" },
  { id: "p-sistem-keamanan", group: "Halaman", label: "Keamanan", hash: "#/sistem/keamanan", hint: "Sistem", keywords: "firewall ssh port paparan lan" },
  { id: "p-sistem-proses", group: "Halaman", label: "Proses", hash: "#/sistem/proses", hint: "Sistem", keywords: "kill htop pid cpu" },
  { id: "p-sistem-disk", group: "Halaman", label: "Ruang dan memori", hash: "#/sistem/disk", hint: "Sistem: bersihkan cache, ukur disk, RAM", keywords: "disk ssd bersihkan clean cache sampah ram memori optimasi lega" },
  { id: "p-sistem-dns", group: "Halaman", label: "DNS", hash: "#/sistem/dns", hint: "Sistem: domain .lab", keywords: "dnsmasq resolver wildcard lab" },
  { id: "p-sistem-tailscale", group: "Halaman", label: "Tailscale", hash: "#/sistem/tailscale", hint: "Sistem: pasang, sambung, hentikan, lepas", keywords: "vpn tailnet install stop start uninstall login ip" },
  { id: "p-sistem-docker", group: "Halaman", label: "Docker (sistem)", hash: "#/sistem/docker", hint: "Sistem", keywords: "colima engine" },
  { id: "p-sistem-uninstall", group: "Halaman", label: "Uninstall panel", hash: "#/sistem/uninstall", hint: "Sistem: cara melepas panel", keywords: "hapus lepas copot remove" },
  { id: "p-backup", group: "Halaman", label: "Backup", hash: "#/backup", hint: "Cadangan dan jadwalnya", keywords: "cadangan restore jadwal harian" },
  { id: "p-pembaruan", group: "Halaman", label: "Pembaruan", hash: "#/pembaruan", hint: "Update paket Homebrew", keywords: "update upgrade brew outdated" },
  { id: "p-aktivitas", group: "Halaman", label: "Aktivitas", hash: "#/aktivitas", hint: "Catatan audit dan aksi", keywords: "audit log riwayat history login job" },
  { id: "p-pengaturan", group: "Halaman", label: "Pengaturan", hash: "#/pengaturan", hint: "Tampilan, notifikasi, akun, 2FA", keywords: "tema gaya aksen warna logo favicon nama panel kata sandi 2fa totp notifikasi" },
  { id: "p-pengguna", group: "Halaman", label: "Pengguna", hash: "#/pengguna", hint: "Akun panel dan peran", keywords: "user akun admin viewer token api", admin: true },
  { id: "p-dokumen", group: "Halaman", label: "Dokumentasi", hash: "#/dokumen", hint: "Panduan penggunaan panel", keywords: "docs bantuan panduan help" },
];

const norm = (s: string) => s.toLowerCase().normalize("NFKD").replace(/[\u0300-\u036f]/g, "");

/** 0 = tidak cocok. Awal label > awal kata > di dalam label > kata kunci > huruf berurutan (toleran salah ketik kecil). */
export function score(it: Item, query: string): number {
  const q = norm(query).trim();
  if (!q) return 1;
  const label = norm(it.label);
  const rest = norm(`${it.hint ?? ""} ${it.keywords ?? ""}`);
  const words = q.split(/\s+/);
  let total = 0;
  for (const w of words) {
    let s = 0;
    if (label === w) s = 100;
    else if (label.startsWith(w)) s = 80;
    else if (label.split(/[\s./:_-]+/).some((x) => x.startsWith(w))) s = 60;
    else if (label.includes(w)) s = 45;
    else if (rest.split(/[\s./:_,-]+/).some((x) => x.startsWith(w))) s = 25;
    else if (rest.includes(w)) s = 15;
    else if (w.length >= 3 && subseq(label, w)) s = 8;
    if (!s) return 0; // setiap kata harus cocok
    total += s;
  }
  return total / words.length;
}

function subseq(text: string, q: string): boolean {
  let i = 0;
  for (const c of text) if (c === q[i] && ++i === q.length) return true;
  return false;
}

export interface Hit {
  item: Item;
  score: number;
}

/** Urut per skor; tiap grup dibatasi agar satu jenis tidak menenggelamkan yang lain. Tanpa kata kunci: hanya halaman utama. */
export function rank(items: Item[], query: string, role = "admin", perGroup = 6): Hit[] {
  const q = query.trim();
  const visible = items.filter((i) => !i.admin || role === "admin");
  const pool = q ? visible : visible.filter((i) => i.group === "Halaman" && !i.id.includes("-", 2) && !/^p-(tugas|sistem)-/.test(i.id));
  const hits = pool.map((item) => ({ item, score: score(item, q) })).filter((h) => h.score > 0);
  const out: Hit[] = [];
  for (const g of GROUP_ORDER) {
    const inG = hits.filter((h) => h.item.group === g).sort((a, b) => b.score - a.score) // sort stabil: skor sama tetap urut menu;
    out.push(...inG.slice(0, q ? perGroup : 20));
  }
  return out;
}

// ---------- pembangun item dari data API ----------
interface SiteLike { name: string; host: string; mode: string; system: boolean; enabled: boolean }
interface SvcLike { id: string; label: string; group: string; state: string }
interface TaskLike { name: string; status: string }
interface DocLike { name: string; title: string }

export const fromSites = (l: SiteLike[]): Item[] => l.map((s) => ({ id: `s-${s.name}`, group: "Website", label: s.host, hint: `${s.mode}${s.system ? ", sistem" : ""}${s.enabled ? "" : ", dijeda"}`, hash: `#/website/${encodeURIComponent(s.name)}` }));
export const fromServices = (l: SvcLike[]): Item[] => l.map((s) => ({ id: `v-${s.id}`, group: "Layanan", label: s.label, hint: `${s.group}, ${s.state === "running" ? "berjalan" : s.state === "not_installed" ? "belum dipasang" : "tidak berjalan"}`, hash: "#/aplikasi", keywords: s.id }));
export const fromDatabases = (m: Record<string, string[]>): Item[] => Object.entries(m).flatMap(([engine, names]) => (names ?? []).map((n) => ({ id: `d-${engine}-${n}`, group: "Database" as Group, label: n, hint: engine, hash: "#/database" })));
export const fromTasks = (kind: "worker" | "job", l: TaskLike[]): Item[] => l.map((t) => ({ id: `t-${kind}-${t.name}`, group: "Tugas", label: t.name, hint: `${kind === "job" ? "cron job" : "worker"}, ${t.status}`, hash: `#/tugas/${kind}` }));
export const fromDocs = (l: DocLike[]): Item[] => l.map((d) => ({ id: `k-${d.name}`, group: "Dokumen", label: d.title, hint: "Dokumentasi", hash: `#/dokumen/${encodeURIComponent(d.name)}` }));
