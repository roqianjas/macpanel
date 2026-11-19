import { useEffect, useState } from "react";
import { errText, fmtTime, get, post } from "../../api";
import { Alert, Card, State, useLoad } from "../../ui";
import { useT } from "../../i18n";

export function ConfigEditor({ name, system, onSaved }: { name: string; system: boolean; onSaved: () => void }) {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<{ content: string; sha: string; versions: { id: number; at: number; user: string; note: string }[] }>(`/api/sites/${name}/config`), [name]);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  useEffect(() => {
    if (data) setText(data.content);
  }, [data]);
  const save = async () => {
    if (!data) return;
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/config`, { content: text, sha: data.sha });
      setMsg({ tone: "ok", text: t("dev.conf.applied") });
      reload();
      onSaved();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const restore = async (id: number) => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/restore`, { id });
      setMsg({ tone: "ok", text: t("dev.conf.restored", { id }) });
      reload();
      onSaved();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="stack-l">
      <Card title={t("dev.conf.title")}>
        <State loading={loading && !data} error={error} onRetry={reload}>
          <p className="small muted">
            {t("dev.conf.note1")} <span className="mono">include .../tailnet_only.conf</span>. {t("dev.conf.note2")} <span className="mono">nginx -t</span>{t("dev.conf.note3")}
          </p>
          <label className="sr" htmlFor="conf-text">{t("dev.conf.content")}</label>
          <textarea id="conf-text" className="mono editor" value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} readOnly={system} rows={22} />
          {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
          {!system && (
            <div>
              <button className="btn btn-primary" onClick={save} disabled={busy || !data || text === data.content}>{busy ? t("dev.conf.applying") : t("dev.conf.saveApply")}</button>
            </div>
          )}
        </State>
      </Card>
      {data && (
        <Card title={t("dev.conf.history")}>
          {data.versions.length === 0 ? (
            <div className="state"><p className="muted">{t("dev.conf.noHistory")}</p></div>
          ) : (
            <ul className="list">
              {data.versions.map((v) => (
                <li key={v.id} className="list-row">
                  <div>
                    <p><span className="mono">#{v.id}</span> {v.note}</p>
                    <p className="small muted">{t("dev.conf.by", { time: fmtTime(v.at), user: v.user })}</p>
                  </div>
                  {!system && (
                    <button className="btn" onClick={() => restore(v.id)} disabled={busy}>{t("dev.conf.restore")}<span className="sr"> {t("dev.conf.versionN", { id: v.id })}</span></button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </Card>
      )}
    </div>
  );
}
