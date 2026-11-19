import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { BookOpen, Compass, Cog, Database, Globe, Search as SearchIcon, Server, CornerDownLeft } from "lucide-react";
import { get } from "../../api";
import { Modal, Skeleton } from "../../ui";
import { useT } from "../../i18n";
import ErrorBoundary from "../../ErrorBoundary";
import { fromDatabases, fromDocs, fromServices, fromSites, fromTasks, GROUP_ORDER, PAGES, rank, type Group, type Item } from "./model";

// Ikon per jenis hasil, dipilih sesuai isinya: Compass = halaman, Globe = situs, Server = layanan, Database, Cog = tugas, BookOpen = dokumen.
const ICON: Record<Group, typeof Globe> = { Halaman: Compass, Website: Globe, Layanan: Server, Database: Database, Tugas: Cog, Dokumen: BookOpen };

const TTL = 60_000;
let cache: { at: number; items: Item[]; failed: number } | null = null;

async function loadIndex(): Promise<{ items: Item[]; failed: number }> {
  if (cache && Date.now() - cache.at < TTL) return cache;
  const src = await Promise.allSettled([
    get<{ sites: Parameters<typeof fromSites>[0] }>("/api/sites").then((d) => fromSites(d.sites)),
    get<{ services: Parameters<typeof fromServices>[0] }>("/api/services").then((d) => fromServices(d.services)),
    get<{ databases: Record<string, string[]> }>("/api/db").then((d) => fromDatabases(d.databases)),
    get<{ tasks: Parameters<typeof fromTasks>[1] }>("/api/tasks/worker").then((d) => fromTasks("worker", d.tasks)),
    get<{ tasks: Parameters<typeof fromTasks>[1] }>("/api/tasks/job").then((d) => fromTasks("job", d.tasks)),
    get<{ docs: Parameters<typeof fromDocs>[0] }>("/api/docs").then((d) => fromDocs(d.docs)),
  ]);
  const items = src.flatMap((r) => (r.status === "fulfilled" ? r.value : []));
  cache = { at: Date.now(), items, failed: src.filter((r) => r.status === "rejected").length };
  return cache;
}

const typing = (t: EventTarget | null) => t instanceof HTMLElement && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));

/** Tombol di bilah atas + dialog pencarian. Ctrl/Cmd+K membuka dari mana saja; "/" membuka bila tidak sedang mengetik. */
export default function GlobalSearch({ role }: { role: string }) {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const k = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      } else if (e.key === "/" && !e.ctrlKey && !e.metaKey && !e.altKey && !typing(e.target) && !document.body.classList.contains("modal-open")) {
        e.preventDefault();
        setOpen(true);
      }
    };
    addEventListener("keydown", k);
    return () => removeEventListener("keydown", k);
  }, []);
  const t = useT();
  const mac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);
  return (
    <>
      <button className="top-search" onClick={() => setOpen(true)} aria-label={t("common.search.aria")} aria-haspopup="dialog">
        <SearchIcon size={18} aria-hidden="true" />
        <span className="top-search-text">{t("common.search.short")}</span>
        <kbd className="kbd" aria-hidden="true">{mac ? "⌘K" : "Ctrl K"}</kbd>
      </button>
      {open && (
        <ErrorBoundary resetKey={String(open)}>
          <SearchDialog role={role} onClose={() => setOpen(false)} />
        </ErrorBoundary>
      )}
    </>
  );
}

function SearchDialog({ role, onClose }: { role: string; onClose: () => void }) {
  const [q, setQ] = useState("");
  const [data, setData] = useState<{ items: Item[]; failed: number } | null>(cache);
  const [active, setActive] = useState(0);
  const listId = useId();
  const list = useRef<HTMLUListElement>(null);
  useEffect(() => {
    let alive = true;
    void loadIndex().then((d) => alive && setData(d));
    return () => void (alive = false);
  }, []);
  const hits = useMemo(() => rank([...PAGES, ...(data?.items ?? [])], q, role), [q, data, role]);
  useEffect(() => {
    setActive(0);
  }, [q]);
  useEffect(() => {
    list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  }, [active]);
  const go = useCallback((h: Item) => {
    onClose();
    location.hash = h.hash;
  }, [onClose]);
  const onKey = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") (e.preventDefault(), setActive((a) => Math.min(a + 1, hits.length - 1)));
    else if (e.key === "ArrowUp") (e.preventDefault(), setActive((a) => Math.max(a - 1, 0)));
    else if (e.key === "Home") (e.preventDefault(), setActive(0));
    else if (e.key === "End") (e.preventDefault(), setActive(hits.length - 1));
    else if (e.key === "Enter" && hits[active]) (e.preventDefault(), go(hits[active].item));
  };
  let last: Group | null = null;
  return (
    <Modal title="Cari" onClose={onClose} wide>
      <div className="stack">
        <div className="search-box">
          <SearchIcon size={18} aria-hidden="true" />
          <input
            data-autofocus
            type="search"
            role="combobox"
            aria-expanded="true"
            aria-controls={listId}
            aria-activedescendant={hits[active] ? `${listId}-${active}` : undefined}
            aria-label="Cari halaman, situs, layanan, database, atau tugas"
            placeholder="Cari halaman, situs, layanan, database, tugas…"
            autoComplete="off"
            spellCheck={false}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={onKey}
          />
        </div>
        <ul id={listId} ref={list} className="search-list" role="listbox" aria-label="Hasil pencarian">
          {hits.map((h, i) => {
            const Ic = ICON[h.item.group];
            const head = h.item.group !== last ? ((last = h.item.group), h.item.group) : null;
            return (
              <li key={h.item.id} role="presentation">
                {head && <p className="search-head" aria-hidden="true">{head}</p>}
                <div id={`${listId}-${i}`} role="option" aria-selected={i === active} className={`search-opt ${i === active ? "on" : ""}`} onMouseMove={() => i !== active && setActive(i)} onClick={() => go(h.item)}>
                  <Ic size={18} aria-hidden="true" className="icon" />
                  <span className="search-label">{h.item.label}</span>
                  {h.item.hint && <span className="small muted search-hint">{h.item.hint}</span>}
                  {i === active && <CornerDownLeft size={16} aria-hidden="true" className="icon search-enter" />}
                </div>
              </li>
            );
          })}
        </ul>
        {hits.length === 0 && <p className="state muted" role="status">Tidak ada hasil untuk “{q}”. Coba nama situs, layanan, atau menu.</p>}
        {!data && <Skeleton />}
        {data && data.failed > 0 && <p className="small muted" role="status">{data.failed} sumber gagal dimuat; hasil mungkin tidak lengkap.</p>}
        <p className="small muted">Panah atas/bawah memilih, Enter membuka, Esc menutup. Kelompok: {GROUP_ORDER.join(", ")}.</p>
      </div>
    </Modal>
  );
}
