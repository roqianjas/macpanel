import { useState } from "react";
import { Copy } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Card, ConfirmDialog, Field, Modal, State, useLoad } from "../ui";
import { useT } from "../i18n";

interface Hook {
  configured: boolean;
  enabled?: boolean;
  branch?: string;
  steps?: string[];
  last_at?: number;
  last_result?: string;
}

export default function Webhook({ name, steps }: { name: string; steps: { id: string; label: string }[] }) {
  const t = useT();
  const q = useLoad(() => get<Hook>(`/api/sites/${name}/hook`), [name]);
  const h = q.data;
  const [branch, setBranch] = useState<string | null>(null);
  const [sel, setSel] = useState<Set<string> | null>(null);
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [secret, setSecret] = useState<string | null>(null);
  const [rotate, setRotate] = useState(false);
  const [del, setDel] = useState(false);
  const br = branch ?? h?.branch ?? "main";
  const chosen = sel ?? new Set(h?.steps ?? ["git_pull"]);
  const en = enabled ?? h?.enabled ?? true;
  const toggle = (id: string) => {
    const n = new Set(chosen);
    if (n.has(id)) n.delete(id);
    else n.add(id);
    setSel(n);
  };
  const save = async (rot: boolean) => {
    setBusy(true);
    setMsg(null);
    try {
      const r = await post<{ secret?: string }>(`/api/sites/${name}/hook`, { enabled: en, branch: br, steps: steps.map((s) => s.id).filter((i) => chosen.has(i)), rotate: rot });
      if (r.secret) setSecret(r.secret);
      else setMsg({ tone: "ok", text: t("ops.wh.saved") });
      setBranch(null);
      setSel(null);
      setEnabled(null);
      q.reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title={t("ops.wh.title")}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          <p className="small muted">
            {t("ops.wh.A")} <span className="mono">127.0.0.1:9101</span> {t("ops.wh.B")}
          </p>
          {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
          <div className="checks">
            <label className="check"><input type="checkbox" checked={en} onChange={(e) => setEnabled(e.target.checked)} />{t("ops.enabled")}</label>
          </div>
          <Field label={t("ops.wh.branch")} hint={t("ops.wh.branchHint")}>{(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="main" required pattern="[A-Za-z0-9._/\-]{1,100}" title={t("ops.wh.branchTitle")} value={br} onChange={(e) => setBranch(e.target.value)} spellCheck={false} maxLength={100} />}</Field>
          <fieldset className="fieldset">
            <legend className="small">{t("ops.dp.steps")}</legend>
            <ul className="list">
              {steps.map((s) => (
                <li key={s.id} className="list-row">
                  <label className="check"><input type="checkbox" checked={chosen.has(s.id)} onChange={() => toggle(s.id)} />{s.label}</label>
                  <span className="mono small muted">{s.id}</span>
                </li>
              ))}
            </ul>
          </fieldset>
          {h?.configured && <p className="small muted">{t("ops.wh.last", { value: h.last_at ? `${fmtTime(h.last_at)} (${h.last_result})` : t("ops.wh.never") })}</p>}
          <div className="row-end">
            {h?.configured && <button className="btn btn-danger-ghost" onClick={() => setDel(true)}>{t("ops.wh.delete")}</button>}
            {h?.configured && <button className="btn" onClick={() => setRotate(true)}>{t("ops.wh.rotate")}</button>}
            <button className="btn btn-primary" disabled={busy || chosen.size === 0} onClick={() => save(false)}>{busy ? t("ops.saving") : h?.configured ? t("common.save") : t("ops.wh.enable")}</button>
          </div>
        </div>
      </State>
      {secret && (
        <Modal title={t("ops.wh.secretTitle")} onClose={() => setSecret(null)}>
          <div className="stack">
            <Alert tone="warn">{t("ops.wh.secretWarn")}</Alert>
            <p className="small">{t("ops.wh.ghSteps")}</p>
            <ul className="list">
              <li className="list-row"><span>Payload URL</span><span className="mono small">{t("ops.wh.payloadUrl", { name })}</span></li>
              <li className="list-row"><span>Content type</span><span className="mono">application/json</span></li>
              <li className="list-row"><span>Events</span><span className="mono">Just the push event</span></li>
            </ul>
            <p className="mono ellip-wrap" aria-label="Secret">{secret}</p>
            <div className="row-end">
              <button className="btn" onClick={() => navigator.clipboard?.writeText(secret)}><Copy size={16} aria-hidden="true" />{t("ops.wh.copy")}</button>
              <button className="btn btn-primary" onClick={() => setSecret(null)}>{t("ops.wh.copied")}</button>
            </div>
          </div>
        </Modal>
      )}
      {rotate && (
        <ConfirmDialog title={t("ops.wh.rotate")} word={name} verb={t("ops.wh.rotate")} body={<p>{t("ops.wh.rotateBody")}</p>} onConfirm={async () => { await save(true); }} onClose={() => setRotate(false)} />
      )}
      {del && (
        <ConfirmDialog title={t("ops.wh.delete")} word={name} verb={t("ops.wh.delete")} body={<p>{t("ops.wh.deleteBody")}</p>} onConfirm={async () => { await post(`/api/sites/${name}/hook/remove`, {}); q.reload(); }} onClose={() => setDel(false)} />
      )}
    </Card>
  );
}
