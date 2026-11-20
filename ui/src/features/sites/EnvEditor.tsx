import { useState } from "react";
import { errText, get, post } from "../../api";
import { Alert, Card, OUTSIDE_ROOT, OutsideNotice } from "../../ui";
import { useT } from "../../i18n";

export function EnvEditor({ name }: { name: string }) {
  const t = useT();
  const [state, setState] = useState<"closed" | "loading" | "open">("closed");
  const [content, setContent] = useState("");
  const [path, setPath] = useState("");
  const [exists, setExists] = useState(false);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [outside, setOutside] = useState(false);
  const open = async () => {
    setState("loading");
    setMsg(null);
    try {
      const r = await get<{ content: string; exists: boolean; path: string }>(`/api/sites/${name}/env`);
      setContent(r.content);
      setExists(r.exists);
      setPath(r.path);
      setState("open");
    } catch (x) {
      const e = errText(x);
      if (OUTSIDE_ROOT.test(e)) {
        setOutside(true);
      } else {
        setMsg({ tone: "bad", text: e });
      }
      setState("closed");
    }
  };
  const save = async () => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/env`, { content });
      setExists(true);
      setMsg({ tone: "ok", text: t("dev.env.saved") });
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title={t("dev.env.title")}>
      <p className="small muted">{t("dev.env.note")}</p>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      {outside ? (
        <OutsideNotice />
      ) : state !== "open" ? (
        <div>
          <button className="btn" onClick={open} disabled={state === "loading"}>{state === "loading" ? t("dev.env.opening") : t("dev.env.open")}</button>
        </div>
      ) : (
        <div className="stack">
          <p className="small mono">{path}{exists ? "" : t("dev.env.willCreate")}</p>
          <label className="sr" htmlFor="env-text">{t("dev.env.content")}</label>
          <textarea id="env-text" className="mono env-text" rows={16} placeholder={"APP_ENV=production\nAPP_DEBUG=false"} value={content} onChange={(e) => setContent(e.target.value)} spellCheck={false} autoComplete="off" />
          <div className="row-end">
            <button className="btn" onClick={() => { setState("closed"); setContent(""); }}>{t("common.close")}</button>
            <button className="btn btn-primary" onClick={save} disabled={busy || content.length > 65536}>{busy ? t("dev.php.saving") : t("common.save")}</button>
          </div>
        </div>
      )}
    </Card>
  );
}
