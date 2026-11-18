import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import { ChevronLeft, ChevronRight, Search } from "lucide-react";
import { filterItems, pageOf, summary, PAGE_SIZE, type PageView } from "./paging";

// Ikon: Search untuk kolom cari, ChevronLeft/Right untuk halaman sebelumnya/berikutnya. Arah panah menunjukkan arah, jadi relevan.

export interface ListView<T> {
  q: string;
  setQ: (s: string) => void;
  view: PageView<T>;
  setPage: (p: number) => void;
  filteredCount: number;
  allCount: number;
  searching: boolean;
}

/**
 * Pencarian + halaman untuk daftar yang sudah dimuat penuh di browser (situs, database, worker, backup).
 * Halaman kembali ke 1 saat kueri berubah; bila item berkurang (mis. dihapus) halaman dijepit otomatis oleh pageOf.
 */
export function useListView<T>(items: readonly T[], text: (t: T) => string, size: number = PAGE_SIZE): ListView<T> {
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const filtered = useMemo(() => filterItems(items, q, text), [items, q]); // eslint-disable-line react-hooks/exhaustive-deps
  const view = pageOf(filtered, page, size);
  useEffect(() => setPage(1), [q]);
  return { q, setQ, view, setPage, filteredCount: filtered.length, allCount: items.length, searching: q.trim() !== "" };
}

/** Kolom cari dengan label terlihat (R-39) dan ringkasan hasil yang dibacakan pembaca layar. Tidak tampil bila daftar kecil dan belum dicari. */
export function ListSearch<T>({ lv, label = "Cari", hint, minItems = 8 }: { lv: ListView<T>; label?: string; hint?: string; minItems?: number }) {
  const id = useId();
  if (lv.allCount < minItems && !lv.searching) return null;
  return (
    <div className="list-tools">
      <div className="field list-search">
        <label htmlFor={id}>{label}</label>
        <div className="search-wrap">
          <Search size={18} className="icon" aria-hidden="true" />
          <input id={id} type="search" value={lv.q} onChange={(e) => lv.setQ(e.target.value)} maxLength={80} autoComplete="off" placeholder={hint ?? "Ketik sebagian nama"} />
        </div>
      </div>
      <p className="small muted" role="status" aria-live="polite">
        {summary(lv.view, lv.allCount, lv.searching)}
      </p>
    </div>
  );
}

/**
 * Tombol halaman. Tersembunyi bila hanya satu halaman.
 * Saat tombol yang sedang fokus menjadi nonaktif (sampai di halaman pertama/terakhir), fokus dipindah ke tombol lawannya;
 * tanpa ini fokus keyboard jatuh ke <body> dan pengguna keyboard kehilangan posisi (R-32).
 */
export function Pager<T>({ lv }: { lv: ListView<T> }) {
  const { page, pages } = lv.view;
  const prev = useRef<HTMLButtonElement>(null);
  const next = useRef<HTMLButtonElement>(null);
  const refocus = useRef<typeof prev | null>(null);
  // Setelah render: bila tombol yang tadi ditekan kini nonaktif, pindahkan fokus ke tombol lawannya.
  useEffect(() => {
    if (refocus.current) {
      refocus.current.current?.focus();
      refocus.current = null;
    }
  }, [page]);
  if (pages <= 1) return null;
  const go = (to: number, other: typeof prev) => {
    if (to <= 1 || to >= pages) refocus.current = other;
    lv.setPage(to);
  };
  return (
    <nav className="pager" aria-label="Halaman daftar">
      <button type="button" ref={prev} className="btn btn-sm" disabled={page <= 1} onClick={() => go(page - 1, next)}>
        <ChevronLeft size={16} aria-hidden="true" /> Sebelumnya
      </button>
      <span className="small muted" aria-current="page">
        Halaman {page} dari {pages}
      </span>
      <button type="button" ref={next} className="btn btn-sm" disabled={page >= pages} onClick={() => go(page + 1, prev)}>
        Berikutnya <ChevronRight size={16} aria-hidden="true" />
      </button>
    </nav>
  );
}

/** Daftar `<ul className="list">` dengan cari + halaman. `render` mengembalikan `<li>`. */
export function PagedList<T>({ items, text, render, label, hint, size }: { items: readonly T[]; text: (t: T) => string; render: (t: T) => ReactNode; label?: string; hint?: string; size?: number }) {
  const lv = useListView(items, text, size);
  return (
    <>
      <ListSearch lv={lv} label={label} hint={hint} />
      <ul className="list">{lv.view.rows.map(render)}</ul>
      <Pager lv={lv} />
    </>
  );
}
