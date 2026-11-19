import { t } from "../../i18n";
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
const page = (id: string, hash: string, label: () => string, hint: () => string, keywords: string, admin?: boolean): Item => ({
  id, group: "Halaman", hash, keywords, admin,
  get label() { return label(); },
  get hint() { return hint(); },
});
// Label dan hint mengikuti bahasa saat dibaca (getter). Kata kunci memuat istilah Indonesia dan English agar pencarian bekerja di kedua bahasa.
export const PAGES: Item[] = [
  page("p-ringkasan", "#/ringkasan", () => t("nav.overview"), () => t("dev.search.h.ringkasan"), "beranda home dashboard status overview summary"),
  page("p-pantau", "#/pantau", () => t("nav.monitor"), () => t("dev.search.h.pantau"), "monitoring grafik cpu ram suhu jaringan monitor charts memory temperature network"),
  page("p-website", "#/website", () => t("nav.websites"), () => t("dev.search.h.website"), "situs domain nginx ssl proxy vhost website sites domain certificate"),
  page("p-aplikasi", "#/aplikasi", () => t("nav.apps"), () => t("dev.search.h.aplikasi"), "layanan service nginx redis php-fpm mysql start stop restart apps services"),
  page("p-database", "#/database", () => t("nav.databases"), () => t("dev.search.h.database"), "mysql mariadb postgres postgresql redis dump user sql databases"),
  page("p-runtime", "#/runtime", () => t("nav.runtimes"), () => t("dev.search.h.runtime"), "php node python composer versi ekstensi fpm mise runtimes version extensions"),
  page("p-docker", "#/docker", () => t("nav.docker"), () => t("dev.search.h.docker"), "colima container image compose"),
  page("p-tugas", "#/tugas", () => t("nav.tasks"), () => t("dev.search.h.tugas"), "supervisor queue crontab jadwal schedule launchd workers cron jobs tasks scheduled"),
  page("p-tugas-job", "#/tugas/job", () => t("dev.search.l.tugas-job"), () => t("nav.tasks"), "crontab jadwal shell skrip url php scheduled script"),
  page("p-tugas-worker", "#/tugas/worker", () => t("dev.task.tab.worker"), () => t("nav.tasks"), "supervisor queue horizon artisan"),
  page("p-sistem", "#/sistem", () => t("nav.system"), () => t("dev.search.h.sistem"), "keamanan firewall ssh system security"),
  page("p-sistem-keamanan", "#/sistem/keamanan", () => t("dev.search.l.sistem-keamanan"), () => t("nav.system"), "firewall ssh port paparan lan security exposure"),
  page("p-sistem-proses", "#/sistem/proses", () => t("dev.search.l.sistem-proses"), () => t("nav.system"), "kill htop pid cpu processes"),
  page("p-sistem-disk", "#/sistem/disk", () => t("dev.search.l.sistem-disk"), () => t("dev.search.h.sistem-disk"), "disk ssd bersihkan clean cache sampah ram memori optimasi lega space storage memory free up junk"),
  page("p-sistem-dns", "#/sistem/dns", () => "DNS", () => t("dev.search.h.sistem-dns"), "dnsmasq resolver wildcard lab"),
  page("p-sistem-tailscale", "#/sistem/tailscale", () => "Tailscale", () => t("dev.search.h.sistem-tailscale"), "vpn tailnet install stop start uninstall login ip"),
  page("p-sistem-docker", "#/sistem/docker", () => t("dev.search.l.sistem-docker"), () => t("nav.system"), "colima engine"),
  page("p-sistem-uninstall", "#/sistem/uninstall", () => t("dev.search.l.sistem-uninstall"), () => t("dev.search.h.sistem-uninstall"), "hapus lepas copot remove"),
  page("p-backup", "#/backup", () => t("nav.backup"), () => t("dev.search.h.backup"), "cadangan restore jadwal harian schedule daily"),
  page("p-pembaruan", "#/pembaruan", () => t("nav.updates"), () => t("dev.search.h.pembaruan"), "update upgrade brew outdated updates"),
  page("p-aktivitas", "#/aktivitas", () => t("nav.activity"), () => t("dev.search.h.aktivitas"), "audit log riwayat history login job activity"),
  page("p-pengaturan", "#/pengaturan", () => t("nav.settings"), () => t("dev.search.h.pengaturan"), "tema gaya aksen warna logo favicon nama panel kata sandi 2fa totp notifikasi settings theme password language bahasa"),
  page("p-pengguna", "#/pengguna", () => t("nav.users"), () => t("dev.search.h.pengguna"), "user akun admin viewer token api users accounts roles", true),
  page("p-dokumen", "#/dokumen", () => t("nav.docs"), () => t("dev.search.h.dokumen"), "docs bantuan panduan help documentation"),
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

export const fromSites = (l: SiteLike[]): Item[] => l.map((s) => ({ id: `s-${s.name}`, group: "Website", label: s.host, hint: `${s.mode}${s.system ? t("dev.search.sys") : ""}${s.enabled ? "" : t("dev.search.paused")}`, hash: `#/website/${encodeURIComponent(s.name)}` }));
export const fromServices = (l: SvcLike[]): Item[] => l.map((s) => ({ id: `v-${s.id}`, group: "Layanan", label: s.label, hint: `${s.group}, ${s.state === "running" ? t("dev.search.running") : s.state === "not_installed" ? t("dev.search.notInstalled") : t("dev.search.notRunning")}`, hash: "#/aplikasi", keywords: s.id }));
export const fromDatabases = (m: Record<string, string[]>): Item[] => Object.entries(m).flatMap(([engine, names]) => (names ?? []).map((n) => ({ id: `d-${engine}-${n}`, group: "Database" as Group, label: n, hint: engine, hash: "#/database" })));
export const fromTasks = (kind: "worker" | "job", l: TaskLike[]): Item[] => l.map((x) => ({ id: `t-${kind}-${x.name}`, group: "Tugas", label: x.name, hint: `${t(kind === "job" ? "dev.task.noun.job" : "dev.task.noun.worker")}, ${x.status}`, hash: `#/tugas/${kind}` }));
export const fromDocs = (l: DocLike[]): Item[] => l.map((d) => ({ id: `k-${d.name}`, group: "Dokumen", label: d.title, hint: t("nav.docs"), hash: `#/dokumen/${encodeURIComponent(d.name)}` }));
