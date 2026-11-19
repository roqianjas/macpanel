import { t } from "../../i18n";
// Logika daftar: pencarian dan halaman untuk data yang sudah ada di browser, serta penggabungan halaman cursor dari server.
// Murni (tanpa React) agar bisa diuji langsung.

export const PAGE_SIZE = 10;

/** Huruf kecil, spasi dirapikan. Dipakai bersama oleh pencarian dan perbandingan. */
export function norm(s: string): string {
  return s.toLowerCase().replace(/\s+/g, " ").trim();
}

/** Setiap kata dalam kueri harus muncul (AND). Kueri kosong mengembalikan semua. Tidak ada wildcard/regex: karakter apa pun dicari apa adanya. */
export function filterItems<T>(items: readonly T[], query: string, text: (t: T) => string): T[] {
  const words = norm(query).split(" ").filter(Boolean);
  if (words.length === 0) return items.slice();
  return items.filter((it) => {
    const hay = norm(text(it));
    return words.every((w) => hay.includes(w));
  });
}

export interface Facet<T> {
  id: string;
  label: string;
  test: (t: T) => boolean;
}

export interface FacetCount {
  id: string;
  label: string;
  count: number;
}

/** Jumlah item per facet, dihitung dari SELURUH daftar (bukan hasil pencarian) agar angka tab tidak melompat saat mengetik. "Semua" selalu ada di depan. */
export function countFacets<T>(items: readonly T[], facets: readonly Facet<T>[]): FacetCount[] {
  return [{ id: "all", label: t("list.all"), count: items.length }, ...facets.map((f) => ({ id: f.id, label: f.label, count: items.filter(f.test).length }))];
}

/** Terapkan facet terpilih. Id tak dikenal (mis. facet hilang setelah data berubah) diperlakukan sebagai "Semua", bukan daftar kosong. */
export function applyFacet<T>(items: readonly T[], facets: readonly Facet<T>[], id: string): T[] {
  const f = facets.find((x) => x.id === id);
  return f ? items.filter(f.test) : items.slice();
}

export interface PageView<T> {
  rows: T[];
  page: number; // 1-based, sudah dijepit ke rentang sah
  pages: number; // minimal 1
  total: number;
  from: number; // 1-based; 0 bila kosong
  to: number;
}

/** Potong satu halaman. Nomor halaman di luar rentang dijepit (mis. setelah item dihapus), tidak pernah menghasilkan halaman kosong palsu. */
export function pageOf<T>(items: readonly T[], page: number, size: number = PAGE_SIZE): PageView<T> {
  const sz = size >= 1 ? Math.floor(size) : PAGE_SIZE; // NaN, 0, dan negatif jatuh ke bawaan
  const total = items.length;
  const pages = Math.max(1, Math.ceil(total / sz));
  const p = Math.min(Math.max(1, Math.floor(page) || 1), pages);
  const start = (p - 1) * sz;
  const rows = items.slice(start, start + sz);
  return { rows, page: p, pages, total, from: total === 0 ? 0 : start + 1, to: start + rows.length };
}

/** Kalimat ringkas untuk pembaca layar dan mata: "11 sampai 20 dari 47" / "3 dari 47 cocok dengan pencarian". */
export function summary(v: { from: number; to: number; total: number }, allCount: number, searching: boolean): string {
  if (v.total === 0) return searching ? t("list.none", { all: allCount }) : t("list.empty");
  if (v.total <= v.to - v.from + 1) return searching ? t("list.matchOf", { total: v.total, all: allCount }) : t("list.items", { total: v.total });
  const base = t("list.range", { from: v.from, to: v.to, total: v.total });
  return searching ? t("list.rangeSearch", { base, all: allCount }) : base;
}

/** Nama item dengan `at` terbesar (yang terbaru). Seri: yang lebih dulu di daftar. Kosong: undefined. Dipakai agar aturan "yang terbaru tidak boleh dihapus" tidak bergantung pada posisi baris di halaman. */
export function newestName(items: readonly { name: string; at: number }[]): string | undefined {
  let best: { name: string; at: number } | undefined;
  for (const it of items) if (!best || it.at > best.at) best = it;
  return best?.name;
}

// ---- cursor dari server ----

export interface CursorPage<T> {
  rows: T[];
  next: number; // 0 = habis
  total: number;
}

export interface CursorState<T extends { id: number }> {
  rows: T[];
  next: number;
  total: number;
}

export const emptyCursor = <T extends { id: number }>(): CursorState<T> => ({ rows: [], next: 0, total: 0 });

/** Gabungkan halaman berikutnya. Baris dengan id yang sudah ada dibuang (aman bila halaman tumpang tindih karena data baru masuk). */
export function appendPage<T extends { id: number }>(cur: CursorState<T>, pg: CursorPage<T>): CursorState<T> {
  const seen = new Set(cur.rows.map((r) => r.id));
  const fresh = (pg.rows ?? []).filter((r) => !seen.has(r.id));
  return { rows: [...cur.rows, ...fresh], next: pg.next ?? 0, total: pg.total ?? cur.total };
}

/** Bangun path dengan limit/cursor/q. q dipangkas; kosong tidak dikirim. */
export function pagePath<B extends string>(base: B, o: { limit: number; cursor?: number; q?: string }): `${B}?${string}` {
  const p = new URLSearchParams();
  p.set("limit", String(o.limit));
  if (o.cursor && o.cursor > 0) p.set("cursor", String(o.cursor));
  const q = (o.q ?? "").trim();
  if (q) p.set("q", q);
  return `${base}?${p.toString()}`;
}
