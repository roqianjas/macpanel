import { Search } from "lucide-react";
import { fmtTime } from "../api";
import { FacetBar } from "../features/list/ListTools";
import { useCursorList } from "../features/list/useCursorList";
import { Card, Chip, PageHead, State, statusTone } from "../ui";

interface Row {
  id: number;
  at: number;
  user: string;
  action: string;
  detail: string;
  ip: string;
}
interface Job {
  id: number;
  at: number;
  action: string;
  status: string;
}

const PAGE = 25;

// Pintasan pencarian ke server (bukan memfilter baris yang sudah dimuat, yang akan menyesatkan pada daftar berhalaman).
// Tanpa garis bawah: "_" adalah wildcard pada pencarian LIKE.
const JOB_SHORTCUTS = [
  { id: "gagal", label: "Gagal" },
  { id: "berjalan", label: "Berjalan" },
  { id: "timeout", label: "Timeout" },
];
const AUDIT_SHORTCUTS = [
  { id: "login", label: "Login" },
  { id: "fail", label: "Gagal" },
  { id: "backup", label: "Backup" },
  { id: "clean", label: "Pembersihan" },
  { id: "token", label: "Token" },
];

function Shortcuts({ items, q, setQ, label }: { items: { id: string; label: string }[]; q: string; setQ: (s: string) => void; label: string }) {
  const value = items.some((i) => i.id === q.trim()) ? q.trim() : "all";
  return <FacetBar label={label} value={value} onChange={(id) => setQ(id === "all" ? "" : id)} facets={[{ id: "all", label: "Semua", count: 0 }, ...items.map((i) => ({ ...i, count: 0 }))]} counts={false} />;
}

function Tools({ id, label, hint, q, setQ, shown, total, searching }: { id: string; label: string; hint: string; q: string; setQ: (s: string) => void; shown: number; total: number; searching: boolean }) {
  return (
    <div className="list-tools">
      <div className="field list-search">
        <label htmlFor={id}>{label}</label>
        <div className="search-wrap">
          <Search size={18} className="icon" aria-hidden="true" />
          <input id={id} type="search" value={q} onChange={(e) => setQ(e.target.value)} maxLength={80} autoComplete="off" placeholder={hint} />
        </div>
      </div>
      <p className="small muted" role="status" aria-live="polite">
        {total === 0 ? (searching ? "Tidak ada yang cocok." : "Kosong.") : shown >= total ? `${total} ${searching ? "cocok" : "catatan"}.` : `Menampilkan ${shown} dari ${total}${searching ? " yang cocok" : ""}.`}
      </p>
    </div>
  );
}

function More({ hasMore, busy, onMore, what }: { hasMore: boolean; busy: boolean; onMore: () => void; what: string }) {
  if (!hasMore) return null;
  return (
    <div className="pager">
      <button type="button" className="btn" disabled={busy} onClick={onMore}>
        {busy ? "Memuat…" : `Muat ${what} lebih lama`}
      </button>
    </div>
  );
}

export default function Activity() {
  const audit = useCursorList<Row>("/api/audit", PAGE);
  const jobs = useCursorList<Job>("/api/jobs", PAGE);
  return (
    <>
      <PageHead title="Aktivitas" desc="Jejak audit berantai: setiap login, percobaan gagal, dan perubahan tercatat. Verifikasi integritas dengan: panel audit verify." />
      <Card title="Audit">
        <Shortcuts items={AUDIT_SHORTCUTS} q={audit.q} setQ={audit.setQ} label="Pintasan jenis catatan" />
        <Tools id="audit-q" label="Cari audit" hint="Pengguna, aksi, atau detail" q={audit.q} setQ={audit.setQ} shown={audit.rows.length} total={audit.total} searching={audit.searching} />
        <State loading={audit.loading && audit.rows.length === 0} error={audit.error} empty={!audit.loading && audit.rows.length === 0} emptyText={audit.searching ? "Tidak ada catatan yang cocok dengan pencarian ini." : "Belum ada catatan."} onRetry={audit.reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Waktu</th>
                  <th scope="col">Pengguna</th>
                  <th scope="col">Aksi</th>
                  <th scope="col">Detail</th>
                  <th scope="col">IP</th>
                </tr>
              </thead>
              <tbody>
                {audit.rows.map((r) => (
                  <tr key={r.id}>
                    <td className="nowrap">{fmtTime(r.at)}</td>
                    <td className="mono">{r.user || "-"}</td>
                    <td className="mono">{r.action}</td>
                    <td className="mono cell-trunc" title={r.detail}>
                      {r.detail}
                    </td>
                    <td className="mono">{r.ip}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <More hasMore={audit.hasMore} busy={audit.loadingMore} onMore={audit.more} what="catatan" />
        </State>
      </Card>
      <Card title="Aksi terakhir">
        <Shortcuts items={JOB_SHORTCUTS} q={jobs.q} setQ={jobs.setQ} label="Pintasan status aksi" />
        <Tools id="jobs-q" label="Cari aksi" hint="Nama aksi atau status" q={jobs.q} setQ={jobs.setQ} shown={jobs.rows.length} total={jobs.total} searching={jobs.searching} />
        <State loading={jobs.loading && jobs.rows.length === 0} error={jobs.error} empty={!jobs.loading && jobs.rows.length === 0} emptyText={jobs.searching ? "Tidak ada aksi yang cocok dengan pencarian ini." : "Belum ada aksi."} onRetry={jobs.reload}>
          <ul className="list">
            {jobs.rows.map((j) => (
              <li key={j.id} className="list-row">
                <div>
                  <p className="mono">{j.action}</p>
                  <p className="small muted">{fmtTime(j.at)}</p>
                </div>
                <Chip tone={statusTone(j.status)} busy={j.status === "berjalan"}>
                  {j.status}
                </Chip>
              </li>
            ))}
          </ul>
          <More hasMore={jobs.hasMore} busy={jobs.loadingMore} onMore={jobs.more} what="aksi" />
        </State>
      </Card>
    </>
  );
}
