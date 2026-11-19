import { useState } from "react";
import { DatabaseBackup } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { trackJob } from "../jobs";
import { fmtBytes } from "../lib";
import { ListSearch, Pager, useListView, FilterTabs } from "../features/list/ListTools";
import type { Facet } from "../features/list/paging";
import { newestName } from "../features/list/paging";
import { Alert, Card, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";
import { CleanupCard, ExtrasCard } from "./BackupCleanup";
import { useT } from "../i18n";

interface Row {
  name: string;
  at: number;
  size: number;
  files: string[];
}

const DAY = 86400;
const age = (at: number) => Date.now() / 1000 - at;
const AGE_FACETS: Facet<{ at: number }>[] = [
  { id: "baru", label: "ops.bk.age.recent", test: (r) => age(r.at) <= 7 * DAY },
  { id: "bulan", label: "ops.bk.age.month", test: (r) => age(r.at) > 7 * DAY && age(r.at) <= 30 * DAY },
  { id: "lama", label: "ops.bk.age.old", test: (r) => age(r.at) > 30 * DAY },
];

export default function Backups() {
  const t = useT();
  const q = useLoad(() => get<{ backups: Row[]; offsite: boolean }>("/api/backups"));
  const [busy, setBusy] = useState(false);
  const [browse, setBrowse] = useState<string | null>(null);
  const [sel, setSel] = useState<string[]>([]);
  const [delAsk, setDelAsk] = useState(false);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const run = async () => {
    setBusy(true);
    setNote(null);
    try {
      const r = await post<{ job: number }>("/api/backups/run", {});
      trackJob(r.job, t("ops.bk.job"), (j) => {
        setBusy(false);
        setNote(j.status === "ok" ? { tone: "ok", text: t("ops.bk.finished") } : { tone: "bad", text: j.output || t("ops.bk.failed") });
        q.reload();
      });
    } catch (x) {
      setBusy(false);
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const rows = q.data?.backups ?? [];
  // Yang terbaru ditentukan dari waktu, bukan dari posisi baris: setelah dicari/dipaginasi posisi 0 di halaman bukan lagi yang terbaru.
  const newest = newestName(rows);
  const lv = useListView(rows, (r) => `${r.name} ${r.files.join(" ")} ${fmtTime(r.at)}`, undefined, AGE_FACETS);
  return (
    <>
      <PageHead
        title={t("nav.backup")}
        desc={t("ops.bk.desc")}
        action={
          <button className="btn btn-primary" disabled={busy} onClick={run}>
            <DatabaseBackup size={18} aria-hidden="true" />
            {busy ? t("ops.running") : t("ops.bk.runNow")}
          </button>
        }
      />
      {q.data && !q.data.offsite && (
        <Alert tone="warn">
          {t("ops.bk.offsite")}
        </Alert>
      )}
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Schedule />
      <Card title={t("ops.bk.list")} action={sel.length > 0 ? <button className="btn btn-danger-ghost" onClick={() => setDelAsk(true)}>{t("ops.bk.delSel", { n: sel.length })}</button> : undefined}>
        <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && rows.length === 0} emptyText={t("ops.bk.empty")} onRetry={q.reload}>
          <FilterTabs lv={{ ...lv, facets: lv.facets.map((f) => ({ ...f, label: f.id === "all" ? t("list.all") : t(f.label) })) }} label={t("ops.bk.filter")} />
          <ListSearch lv={lv} label={t("ops.bk.search")} hint={t("ops.bk.searchHint")} />
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col"><span className="sr">{t("ops.col.select")}</span></th>
                  <th scope="col">{t("ops.col.time")}</th>
                  <th scope="col">{t("ops.col.size")}</th>
                  <th scope="col">{t("ops.col.contents")}</th>
                  <th scope="col"><span className="sr">{t("ops.col.actions")}</span></th>
                </tr>
              </thead>
              <tbody>
                {lv.view.rows.map((r) => (
                  <tr key={r.name}>
                    <td>
                      <input type="checkbox" className="rowcheck" disabled={r.name === newest} checked={sel.includes(r.name)} onChange={(e) => setSel((p) => (e.target.checked ? [...p, r.name] : p.filter((x) => x !== r.name)))} aria-label={r.name === newest ? t("ops.bk.newestAria", { name: r.name }) : t("ops.bk.pick", { name: r.name })} title={r.name === newest ? t("ops.bk.newestNoDel") : undefined} />
                    </td>
                    <td className="nowrap">{fmtTime(r.at)}</td>
                    <td className="nowrap">{fmtBytes(r.size)}</td>
                    <td className="mono small cell-trunc" title={r.files.join(", ")}>
                      {r.files.join(", ")}
                    </td>
                    <td className="cell-end">
                      {r.files.includes("developer.tar.gz") && (
                        <button className="btn btn-sm" onClick={() => setBrowse(r.name)}>
                          {t("ops.bk.browse")}<span className="sr"> {t("ops.bk.ofBackup", { name: r.name })}</span>
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pager lv={lv} />
        </State>
      </Card>
      <CleanupCard onChanged={() => { setSel([]); q.reload(); }} />
      <ExtrasCard />
      {delAsk && (
        <ConfirmDialog
          title={t("ops.bk.delN", { n: sel.length })}
          word="hapus"
          verb={t("ops.bk.delN", { n: sel.length })}
          body={<p>{t("ops.bk.delBody")}</p>}
          onConfirm={async () => {
            await post("/api/backups/delete", { names: sel });
            setSel([]);
            q.reload();
          }}
          onClose={() => setDelAsk(false)}
        />
      )}
      {browse && <Browse name={browse} onClose={() => setBrowse(null)} />}
      <Card title={t("ops.bk.restore")}>
        <p className="small">{t("ops.bk.restoreA")} <span className="mono">~/MacPanel/backup/restore/</span>{t("ops.bk.restoreB")}</p>
      </Card>
    </>
  );
}

function Schedule() {
  const t = useT();
  const q = useLoad(() => get<{ time: string; keep: number; days: number; extra_days: number; enabled: boolean; every_days: number }>("/api/backups/config"));
  const [time, setTime] = useState("");
  const [keep, setKeep] = useState("");
  const [days, setDays] = useState("");
  const [extra, setExtra] = useState("");
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [every, setEvery] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const on = enabled ?? q.data?.enabled ?? true;
  const ev = every || String(q.data?.every_days ?? 1);
  const tm = time || q.data?.time || "02:30";
  const k = keep || String(q.data?.keep ?? 7);
  const dy = days || String(q.data?.days ?? 0);
  const ex = extra || String(q.data?.extra_days ?? 0);
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      await post("/api/backups/config", { time: tm, keep: Number(k), days: Number(dy), extra_days: Number(ex), enabled: on, every_days: Number(ev) });
      setMsg({ tone: "ok", text: on ? t("ops.bk.sched.on") : t("ops.bk.sched.off") });
      setTime("");
      setEnabled(null);
      setEvery("");
      setKeep("");
      setDays("");
      setExtra("");
      q.reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title={t("ops.bk.sched.title")}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <form onSubmit={save} className="stack">
          <label className="check">
            <input type="checkbox" checked={on} onChange={(e) => setEnabled(e.target.checked)} />
            {t("ops.bk.sched.enable")}
          </label>
          {!on && <p className="callout">{t("ops.bk.sched.offNote")}</p>}
          <div className="grid2">
            <Field label={t("ops.bk.every")} hint={t("ops.bk.everyHint")}>{(id, dd) => <input id={id} aria-describedby={dd} type="number" min={1} max={30} placeholder="1" value={ev} onChange={(e) => setEvery(e.target.value)} disabled={!on} required />}</Field>
            <Field label={t("ops.bk.hour")} hint={t("ops.bk.hourHint")}>{(id, dd) => <input id={id} aria-describedby={dd} type="time" value={tm} onChange={(e) => setTime(e.target.value)} required />}</Field>
            <Field label={t("ops.bk.keep")} hint={t("ops.bk.keepHint")}>{(id, dd) => <input id={id} aria-describedby={dd} type="number" min={1} max={60} placeholder="10" value={k} onChange={(e) => setKeep(e.target.value)} required />}</Field>
            <Field label={t("ops.bk.autoDel")} hint={t("ops.bk.autoDelHint")}>{(id, dd) => <input id={id} aria-describedby={dd} type="number" min={0} max={3650} placeholder="30" value={dy} onChange={(e) => setDays(e.target.value)} required />}</Field>
            <Field label={t("ops.bk.autoDelExtra")} hint={t("ops.bk.autoDelExtraHint")}>{(id, dd) => <input id={id} aria-describedby={dd} type="number" min={0} max={3650} placeholder="7" value={ex} onChange={(e) => setExtra(e.target.value)} required />}</Field>
          </div>
          {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
          <div>
            <button className="btn" disabled={busy}>{busy ? t("ops.saving") : t("ops.bk.saveSched")}</button>
          </div>
        </form>
      </State>
    </Card>
  );
}

function Browse({ name, onClose }: { name: string; onClose: () => void }) {
  const t = useT();
  const [part, setPart] = useState<"developer" | "dotfiles">("developer");
  const [prefix, setPrefix] = useState("");
  const q = useLoad(() => get<{ entries: { name: string; dir: boolean }[] }>(`/api/backups/ls?name=${name}&part=${part}&prefix=${encodeURIComponent(prefix)}`), [name, part, prefix]);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const extract = async (path: string) => {
    setMsg(null);
    try {
      const r = await post<{ job: number }>("/api/backups/extract", { name, part, path });
      trackJob(r.job, t("ops.bk.extractJob", { path }), (j) => setMsg(j.status === "ok" ? { tone: "ok", text: j.output.split("\n").filter(Boolean).pop() ?? t("ops.done") } : { tone: "bad", text: j.output || t("ops.failed") }));
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    }
  };
  const crumbs = prefix ? prefix.split("/") : [];
  return (
    <Modal title={t("ops.bk.browseTitle", { name })} onClose={onClose} wide>
      <div className="stack">
        <div className="seg" role="group" aria-label={t("ops.bk.section")}>
          {(["developer", "dotfiles"] as const).map((p) => (
            <button key={p} type="button" className={`seg-btn ${part === p ? "on" : ""}`} aria-pressed={part === p} onClick={() => { setPart(p); setPrefix(""); }}>
              {p === "developer" ? t("ops.bk.devFolder") : t("ops.bk.dotfiles")}
            </button>
          ))}
        </div>
        <p className="small mono">
          <button type="button" className="linklike" onClick={() => setPrefix("")}>/</button>
          {crumbs.map((c, i) => (
            <span key={i}> / <button type="button" className="linklike" onClick={() => setPrefix(crumbs.slice(0, i + 1).join("/"))}>{c}</button></span>
          ))}
        </p>
        {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
        <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && q.data.entries.length === 0} emptyText={t("list.empty")} onRetry={q.reload}>
          <ul className="list ext-list">
            {q.data?.entries.map((e) => {
              const full = prefix ? `${prefix}/${e.name}` : e.name;
              return (
                <li key={e.name} className="list-row">
                  {e.dir ? (
                    <button type="button" className="linklike mono" onClick={() => setPrefix(full)}>{e.name}/</button>
                  ) : (
                    <span className="mono">{e.name}</span>
                  )}
                  <button className="btn btn-sm" onClick={() => extract(full)}>
                    {t("ops.bk.extract")}<span className="sr"> {full}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        </State>
        <p className="small muted">{t("ops.bk.extractA")} <span className="mono">{t("ops.bk.restorePath")}</span>{t("ops.bk.extractB")}</p>
      </div>
    </Modal>
  );
}
