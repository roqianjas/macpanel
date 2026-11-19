import { Search } from "lucide-react";
import { fmtTime } from "../api";
import { FacetBar } from "../features/list/ListTools";
import { useCursorList } from "../features/list/useCursorList";
import { Card, Chip, PageHead, State, statusTone } from "../ui";
import { useT } from "../i18n";

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
  { id: "gagal", label: "ops.act.failed" },
  { id: "berjalan", label: "svc.state.running" },
  { id: "timeout", label: "ops.act.timeout" },
];
const AUDIT_SHORTCUTS = [
  { id: "login", label: "ops.act.login" },
  { id: "fail", label: "ops.act.failed" },
  { id: "backup", label: "nav.backup" },
  { id: "clean", label: "ops.act.cleanup" },
  { id: "token", label: "ops.act.token" },
];

function Shortcuts({ items, q, setQ, label }: { items: { id: string; label: string }[]; q: string; setQ: (s: string) => void; label: string }) {
  const t = useT();
  const value = items.some((i) => i.id === q.trim()) ? q.trim() : "all";
  return <FacetBar label={label} value={value} onChange={(id) => setQ(id === "all" ? "" : id)} facets={[{ id: "all", label: t("list.all"), count: 0 }, ...items.map((i) => ({ ...i, label: t(i.label), count: 0 }))]} counts={false} />;
}

function Tools({ id, label, hint, q, setQ, shown, total, searching }: { id: string; label: string; hint: string; q: string; setQ: (s: string) => void; shown: number; total: number; searching: boolean }) {
  const t = useT();
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
        {total === 0 ? (searching ? t("ops.noMatch") : t("list.empty")) : shown >= total ? t(searching ? "ops.act.countMatch" : "ops.act.countRec", { n: total }) : t(searching ? "ops.act.showingMatch" : "ops.act.showing", { shown, total })}
      </p>
    </div>
  );
}

function More({ hasMore, busy, onMore, what }: { hasMore: boolean; busy: boolean; onMore: () => void; what: string }) {
  const t = useT();
  if (!hasMore) return null;
  return (
    <div className="pager">
      <button type="button" className="btn" disabled={busy} onClick={onMore}>
        {busy ? t("state.loading") : t(what)}
      </button>
    </div>
  );
}

export default function Activity() {
  const t = useT();
  const audit = useCursorList<Row>("/api/audit", PAGE);
  const jobs = useCursorList<Job>("/api/jobs", PAGE);
  return (
    <>
      <PageHead title={t("nav.activity")} desc={t("ops.act.desc")} />
      <Card title={t("ops.act.audit")}>
        <Shortcuts items={AUDIT_SHORTCUTS} q={audit.q} setQ={audit.setQ} label={t("ops.act.auditShortcuts")} />
        <Tools id="audit-q" label={t("ops.act.auditSearch")} hint={t("ops.act.auditHint")} q={audit.q} setQ={audit.setQ} shown={audit.rows.length} total={audit.total} searching={audit.searching} />
        <State loading={audit.loading && audit.rows.length === 0} error={audit.error} empty={!audit.loading && audit.rows.length === 0} emptyText={audit.searching ? t("ops.act.auditNoMatch") : t("ops.act.auditEmpty")} onRetry={audit.reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">{t("ops.col.time")}</th>
                  <th scope="col">{t("ops.col.user")}</th>
                  <th scope="col">{t("ops.col.action")}</th>
                  <th scope="col">{t("ops.col.detail")}</th>
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
          <More hasMore={audit.hasMore} busy={audit.loadingMore} onMore={audit.more} what="ops.act.moreRec" />
        </State>
      </Card>
      <Card title={t("ops.act.jobs")}>
        <Shortcuts items={JOB_SHORTCUTS} q={jobs.q} setQ={jobs.setQ} label={t("ops.act.jobShortcuts")} />
        <Tools id="jobs-q" label={t("ops.act.jobSearch")} hint={t("ops.act.jobHint")} q={jobs.q} setQ={jobs.setQ} shown={jobs.rows.length} total={jobs.total} searching={jobs.searching} />
        <State loading={jobs.loading && jobs.rows.length === 0} error={jobs.error} empty={!jobs.loading && jobs.rows.length === 0} emptyText={jobs.searching ? t("ops.act.jobNoMatch") : t("ops.act.jobEmpty")} onRetry={jobs.reload}>
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
          <More hasMore={jobs.hasMore} busy={jobs.loadingMore} onMore={jobs.more} what="ops.act.moreJob" />
        </State>
      </Card>
    </>
  );
}
