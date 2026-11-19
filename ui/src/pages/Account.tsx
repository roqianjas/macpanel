import { useState } from "react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Card, Field, State, useLoad } from "../ui";
import { useT } from "../i18n";

export default function Account({ forced, onDone }: { forced?: boolean; onDone?: () => void } = {}) {
  const t = useT();
  const [cur, setCur] = useState("");
  const [nw, setNw] = useState("");
  const [again, setAgain] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const mismatch = again !== "" && again !== nw;
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      await post("/api/me/password", { current: cur, new: nw });
      setMsg({ tone: "ok", text: t("settings.acct.pwDone") });
      setCur("");
      setNw("");
      setAgain("");
      onDone?.();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="stack">
    {!forced && <Devices />}
    <Card title={t("settings.acct.pwTitle")}>
      <form onSubmit={submit} className="stack" autoComplete="off">
        <p className="small muted">{t("settings.acct.pwHelp")}</p>
        <div className="grid2">
          <Field label={t("settings.acct.cur")}>{(id) => <input id={id} type="password" placeholder={t("settings.acct.cur.ph")} value={cur} onChange={(e) => setCur(e.target.value)} autoComplete="current-password" required />}</Field>
          <span />
          <Field label={t("settings.acct.new")} hint={t("settings.acct.new.hint")}>{(id, dd) => <input id={id} aria-describedby={dd} type="password" placeholder={t("settings.acct.new.ph")} value={nw} onChange={(e) => setNw(e.target.value)} autoComplete="new-password" minLength={12} maxLength={256} required />}</Field>
          <Field label={t("settings.acct.again")} hint={mismatch ? t("settings.acct.again.hint") : undefined}>{(id, dd) => <input id={id} aria-describedby={dd} aria-invalid={mismatch} type="password" placeholder={t("settings.acct.again.ph")} value={again} onChange={(e) => setAgain(e.target.value)} autoComplete="new-password" required />}</Field>
        </div>
        {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
        <div>
          <button className="btn" disabled={busy || !cur || nw.length < 12 || nw !== again}>{busy ? t("settings.saving") : t("settings.acct.pwTitle")}</button>
        </div>
      </form>
    </Card>
    </div>
  );
}

interface Device { id: number; label: string; created: number; last_used: number; expires: number; current: boolean }

function Devices() {
  const t = useT();
  const q = useLoad(() => get<{ devices: Device[]; days: number }>("/api/me/devices"));
  const [err, setErr] = useState("");
  const revoke = async (body: { id?: number; all?: boolean }) => {
    setErr("");
    try {
      await post("/api/me/devices/revoke", body);
      q.reload();
    } catch (x) {
      setErr(errText(x));
    }
  };
  const list = q.data?.devices ?? [];
  return (
    <Card title={t("settings.dev.title")} action={list.length > 0 ? <button className="btn" onClick={() => revoke({ all: true })}>{t("settings.dev.revokeAll")}</button> : undefined}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          <p className="small muted">{t("settings.dev.help", { days: q.data?.days ?? 30 })}</p>
          {err && <Alert tone="bad">{err}</Alert>}
          {list.length === 0 ? (
            <p className="small muted">{t("settings.dev.none")}</p>
          ) : (
            <ul className="list">
              {list.map((d) => (
                <li key={d.id} className="list-row">
                  <span>
                    {d.label}
                    {d.current ? t("settings.dev.current") : ""}
                    <br />
                    <span className="small muted">{t("settings.dev.times", { used: fmtTime(d.last_used), until: fmtTime(d.expires) })}</span>
                  </span>
                  <button className="btn" onClick={() => revoke({ id: d.id })}>{t("settings.revoke")}</button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </State>
    </Card>
  );
}
