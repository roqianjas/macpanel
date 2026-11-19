import { useState } from "react";
import { errText, get, post } from "../api";
import { fmtBytes } from "../lib";
import { Alert, Card, ConfirmDialog, Field, State, useLoad } from "../ui";
import { useT } from "../i18n";

interface Plan {
  keep: number;
  days: number;
  remove: { name: string; reason: string; kb: number }[];
  kept: number;
}

export function CleanupCard({ onChanged }: { onChanged: () => void }) {
  const t = useT();
  const cfg = useLoad(() => get<{ keep: number; days: number }>("/api/backups/config"));
  const [keep, setKeep] = useState("");
  const [days, setDays] = useState("");
  const k = keep || String(cfg.data?.keep ?? 7);
  const dd = days || String(cfg.data?.days ?? 0);
  const [plan, setPlan] = useState<Plan | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const preview = async () => {
    setBusy(true);
    setMsg(null);
    try {
      setPlan(await get<Plan>(`/api/backups/prune-preview?keep=${encodeURIComponent(k)}&days=${encodeURIComponent(dd)}`));
    } catch (x) {
      setPlan(null);
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const total = (plan?.remove ?? []).reduce((a, r) => a + r.kb * 1024, 0);
  return (
    <Card title={t("ops.bc.title")}>
      <State loading={cfg.loading && !cfg.data} error={cfg.error} onRetry={cfg.reload}>
        <p className="small muted">{t("ops.bc.desc")}</p>
        <div className="grid2">
          <Field label={t("ops.bk.keep")} hint="1-60">{(id, h) => <input id={id} aria-describedby={h} type="number" min={1} max={60} placeholder="10" required value={k} onChange={(e) => { setKeep(e.target.value); setPlan(null); }} />}</Field>
          <Field label={t("ops.bc.olderThan")} hint={t("ops.bc.noAgeLimit")}>{(id, h) => <input id={id} aria-describedby={h} type="number" min={0} max={3650} placeholder="30" required value={dd} onChange={(e) => { setDays(e.target.value); setPlan(null); }} />}</Field>
        </div>
        {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={preview} disabled={busy}>{busy ? t("ops.bc.calculating") : t("ops.bc.preview")}</button>
        </div>
        {plan && (
          plan.remove.length === 0 ? (
            <p className="muted">{t("ops.bc.nothing", { n: plan.kept })}</p>
          ) : (
            <div className="stack">
              <ul className="list">
                {plan.remove.map((r) => (
                  <li key={r.name} className="list-row"><div><p className="mono">{r.name}</p><p className="small muted">{r.reason}</p></div><span className="mono">{fmtBytes(r.kb * 1024)}</span></li>
                ))}
              </ul>
              <div className="row-end">
                <span className="small muted">{t("ops.bc.summary", { n: plan.remove.length, size: fmtBytes(total), kept: plan.kept })}</span>
                <button className="btn btn-danger-ghost" onClick={() => setConfirm(true)}>{t("ops.bk.delN", { n: plan.remove.length })}</button>
              </div>
            </div>
          )
        )}
      </State>
      {confirm && plan && (
        <ConfirmDialog
          title={t("ops.bc.confirmTitle")}
          word="hapus"
          verb={t("ops.bk.delN", { n: plan.remove.length })}
          body={<p>{t("ops.bc.confirmBody", { n: plan.remove.length, size: fmtBytes(total) })}</p>}
          onConfirm={async () => {
            await post("/api/backups/prune", { keep: Number(k), days: Number(dd) });
            setPlan(null);
            setMsg({ tone: "ok", text: t("ops.bc.deleted") });
            onChanged();
          }}
          onClose={() => setConfirm(false)}
        />
      )}
    </Card>
  );
}

interface Extras {
  db: { count: number; kb: number; oldest: number };
  restore: { count: number; kb: number; oldest: number };
  retired: { kb: number; note: string };
}

export function ExtrasCard() {
  const t = useT();
  const q = useLoad(() => get<Extras>("/api/backups/extras"));
  return (
    <Card title={t("ops.bc.extrasTitle")}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {q.data && (
          <div className="stack">
            <p className="small muted">{t("ops.bc.extrasDesc")}</p>
            <Row kind="db" title={t("ops.bc.dumps")} info={q.data.db} onDone={q.reload} />
            <Row kind="restore" title={t("ops.bc.extractions")} info={q.data.restore} onDone={q.reload} />
            <div className="list-row"><div><p>{t("ops.bc.legacy")}</p><p className="small muted">{q.data.retired.note}</p></div><span className="mono">{fmtBytes(q.data.retired.kb * 1024)}</span></div>
          </div>
        )}
      </State>
    </Card>
  );
}

function Row({ kind, title, info, onDone }: { kind: "db" | "restore"; title: string; info: { count: number; kb: number }; onDone: () => void }) {
  const t = useT();
  const [days, setDays] = useState("30");
  const [pv, setPv] = useState<{ count: number; kb: number } | null>(null);
  const [confirm, setConfirm] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const check = async () => {
    setMsg(null);
    try {
      setPv(await get(`/api/backups/extras-preview?kind=${kind}&days=${encodeURIComponent(days)}`));
    } catch (x) {
      setPv(null);
      setMsg({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <div className="stack">
      <div className="list-row"><div><p>{title}</p><p className="small muted">{t("ops.items", { n: info.count })}</p></div><span className="mono">{fmtBytes(info.kb * 1024)}</span></div>
      {info.count > 0 && (
        <div className="row-form">
          <Field label={t("ops.bc.olderThan")} hint={t("ops.bc.all")}>{(id, h) => <input id={id} aria-describedby={h} type="number" min={0} max={3650} placeholder="30" required value={days} onChange={(e) => { setDays(e.target.value); setPv(null); }} />}</Field>
          <button className="btn" onClick={check}>{t("ops.bc.count")}</button>
          {pv && pv.count > 0 && <button className="btn btn-danger-ghost" onClick={() => setConfirm(true)}>{t("ops.bc.delItems", { n: pv.count, size: fmtBytes(pv.kb * 1024) })}</button>}
        </div>
      )}
      {pv && pv.count === 0 && <p className="small muted">{t("ops.noMatch")}</p>}
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      {confirm && pv && (
        <ConfirmDialog
          title={t("ops.bc.delWhat", { what: title.toLowerCase() })}
          word="hapus"
          verb={t("ops.bc.delItemsVerb", { n: pv.count })}
          body={<p>{t("ops.bc.delItemsBody", { n: pv.count, size: fmtBytes(pv.kb * 1024) })}</p>}
          onConfirm={async () => {
            await post("/api/backups/extras", { kind, days: Number(days) });
            setPv(null);
            setMsg({ tone: "ok", text: t("ops.deleted") });
            onDone();
          }}
          onClose={() => setConfirm(false)}
        />
      )}
    </div>
  );
}
