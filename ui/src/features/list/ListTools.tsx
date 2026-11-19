import { useT } from "../../i18n";
import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import { TabIcon, useScrollRow } from "../../tabicons";
import { ChevronLeft, ChevronRight, Search } from "lucide-react";
import { applyFacet, countFacets, filterItems, pageOf, summary, PAGE_SIZE, type Facet, type FacetCount, type PageView } from "./paging";

// Ikon: Search untuk kolom cari, ChevronLeft/Right untuk halaman sebelumnya/berikutnya. Arah panah menunjukkan arah, jadi relevan.

export interface ListView<T> {
  q: string;
  setQ: (s: string) => void;
  view: PageView<T>;
  setPage: (p: number) => void;
  filteredCount: number;
  allCount: number;
  searching: boolean;
  facets: FacetCount[];
  facet: string;
  setFacet: (id: string) => void;
}

/**
 * Pencarian + halaman untuk daftar yang sudah dimuat penuh di browser (situs, database, worker, backup).
 * Halaman kembali ke 1 saat kueri berubah; bila item berkurang (mis. dihapus) halaman dijepit otomatis oleh pageOf.
 */
export function useListView<T>(items: readonly T[], text: (t: T) => string, size: number = PAGE_SIZE, facetDefs: readonly Facet<T>[] = []): ListView<T> {
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const [facet, setFacet] = useState("all");
  const facets = useMemo(() => countFacets(items, facetDefs), [items]); // eslint-disable-line react-hooks/exhaustive-deps
  const active = facets.some((f) => f.id === facet) ? facet : "all";
  const filtered = useMemo(() => filterItems(applyFacet(items, facetDefs, active), q, text), [items, q, active]); // eslint-disable-line react-hooks/exhaustive-deps
  const view = pageOf(filtered, page, size);
  useEffect(() => {
    setPage(1);
  }, [q, active]);
  return { q, setQ, view, setPage, filteredCount: filtered.length, allCount: items.length, searching: q.trim() !== "" || active !== "all", facets, facet: active, setFacet };
}

/** Tab filter presentasional. Memakai aria-pressed (ini filter, bukan panel bertab). Tab berisi 0 disembunyikan kecuali "Semua" dan yang sedang dipilih. */
export function FacetBar({ facets, value, onChange, label, counts = true }: { facets: readonly FacetCount[]; value: string; onChange: (id: string) => void; label: string; counts?: boolean }) {
  const row = useScrollRow(value);
  const shown = counts ? facets.filter((f) => f.id === "all" || f.count > 0 || f.id === value) : facets;
  if (shown.length <= 2) return null;
  return (
    <div className="ftabs" role="group" aria-label={label} ref={row}>
      {shown.map((f) => (
        <button key={f.id} type="button" className={`ftab ${value === f.id ? "on" : ""}`} aria-pressed={value === f.id} onClick={() => onChange(f.id)}>
          <TabIcon id={f.id} />
          {f.label}{counts && <> <span className="ftab-n">{f.count}</span></>}
        </button>
      ))}
    </div>
  );
}

export function FilterTabs<T>({ lv, label }: { lv: ListView<T>; label: string }) {
  return <FacetBar facets={lv.facets} value={lv.facet} onChange={lv.setFacet} label={label} />;
}

/** Kolom cari dengan label terlihat (R-39) dan ringkasan hasil yang dibacakan pembaca layar. Tidak tampil bila daftar kecil dan belum dicari. */
export function ListSearch<T>({ lv, label, hint, minItems = 8 }: { lv: ListView<T>; label?: string; hint?: string; minItems?: number }) {
  const id = useId();
  const t = useT();
  if (lv.allCount < minItems && !lv.searching) return null;
  return (
    <div className="list-tools">
      <div className="field list-search">
        <label htmlFor={id}>{label ?? t("list.search")}</label>
        <div className="search-wrap">
          <Search size={18} className="icon" aria-hidden="true" />
          <input id={id} type="search" value={lv.q} onChange={(e) => lv.setQ(e.target.value)} maxLength={80} autoComplete="off" placeholder={hint ?? t("list.hint")} />
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
  const t = useT();
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
    <nav className="pager" aria-label={t("list.nav")}>
      <button type="button" ref={prev} className="btn btn-sm" disabled={page <= 1} onClick={() => go(page - 1, next)}>
        <ChevronLeft size={16} aria-hidden="true" /> {t("list.prev")}
      </button>
      <span className="small muted" aria-current="page">
        {t("list.pages", { page, pages })}
      </span>
      <button type="button" ref={next} className="btn btn-sm" disabled={page >= pages} onClick={() => go(page + 1, prev)}>
        {t("list.next")} <ChevronRight size={16} aria-hidden="true" />
      </button>
    </nav>
  );
}

/** Daftar `<ul className="list">` dengan cari + halaman. `render` mengembalikan `<li>`. */
export function PagedList<T>({ items, text, render, label, hint, size, facets, facetLabel }: { items: readonly T[]; text: (t: T) => string; render: (t: T) => ReactNode; label?: string; hint?: string; size?: number; facets?: readonly Facet<T>[]; facetLabel?: string }) {
  const lv = useListView(items, text, size, facets);
  const t = useT();
  return (
    <>
      <FilterTabs lv={lv} label={facetLabel ?? t("list.filter")} />
      <ListSearch lv={lv} label={label} hint={hint} />
      <ul className="list">{lv.view.rows.map(render)}</ul>
      <Pager lv={lv} />
    </>
  );
}
