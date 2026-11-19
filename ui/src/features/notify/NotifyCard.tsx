import { useState } from "react";
import { BellRing } from "lucide-react";
import { errText, get, post } from "../../api";
import { Alert, Card, Chip, ConfirmDialog, Field, State, useLoad } from "../../ui";
import { useT } from "../../i18n";
import { genTopic, lastText, serverError, topicError, type NotifyCfg } from "./model";

// Ikon BellRing: kartu ini tentang pesan ke ponsel, jadi lonceng relevan dengan isinya.
// Topik adalah rahasia: dikirim ke server sekali lalu tidak pernah dibaca balik; yang tampil hanya "sudah diisi".

type Note = { tone: "ok" | "bad" | "warn"; text: string } | null;

export default function NotifyCard() {
  const t = useT();
  const q = useLoad(() => get<NotifyCfg>("/api/notify"));
  const c = q.data;
  const [server, setServer] = useState<string | null>(null);
  const [topic, setTopic] = useState("");
  const [min, setMin] = useState<"bad" | "warn" | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [shown, setShown] = useState("");
  const [clearing, setClearing] = useState(false);
  const adminOnly = !!q.error && /admin/i.test(q.error);

  const srv = server ?? c?.server ?? "";
  const lvl = min ?? c?.min ?? "bad";
  const sErr = server !== null ? serverError(srv) : "";
  const tErr = topic !== "" ? topicError(topic) : "";
  const dirty = server !== null || topic !== "" || min !== null;

  const save = async (extra: Record<string, unknown>, done: string) => {
    setBusy(true);
    setNote(null);
    try {
      await post<NotifyCfg>("/api/notify", extra);
      if (typeof extra.topic === "string") setShown(extra.topic);
      setServer(null);
      setTopic("");
      setMin(null);
      await q.reload();
      setNote({ tone: "ok", text: done });
    } catch (e) {
      setNote({ tone: "bad", text: errText(e) });
    } finally {
      setBusy(false);
    }
  };

  const onSave = () => {
    const body: Record<string, unknown> = {};
    if (server !== null) body.server = srv.trim();
    if (topic !== "") body.topic = topic;
    if (min !== null) body.min = min;
    save(body, topic !== "" ? t("settings.notify.savedTopic") : t("settings.notify.saved"));
  };

  const test = async () => {
    setBusy(true);
    setNote(null);
    try {
      const r = await post<{ ok: boolean; error?: string }>("/api/notify/test", {});
      setNote(r.ok ? { tone: "ok", text: t("settings.notify.testOk") } : { tone: "bad", text: t("settings.notify.testFail", { reason: r.error ?? t("settings.notify.unknown") }) });
      await q.reload();
    } catch (e) {
      setNote({ tone: "bad", text: errText(e) });
    } finally {
      setBusy(false);
    }
  };

  if (adminOnly)
    return (
      <Card title={<><BellRing size={18} className="icon" aria-hidden="true" /> {t("settings.notify.title")}</>}>
        <p className="small muted">{t("settings.notify.adminOnly")}</p>
      </Card>
    );

  return (
    <Card title={<><BellRing size={18} className="icon" aria-hidden="true" /> {t("settings.notify.title")}</>}>
      <State loading={q.loading && !c} error={q.error} onRetry={q.reload}>
        {c && (
          <div className="stack">
            {note && <Alert tone={note.tone}>{note.text}</Alert>}
            {shown && (
              <Alert tone="warn">
                {t("settings.notify.yourTopic", { topic: shown })}{" "}
                <button type="button" className="btn btn-sm" onClick={() => navigator.clipboard.writeText(shown).then(() => setNote({ tone: "ok", text: t("settings.notify.topicCopied") }), () => setNote({ tone: "bad", text: t("settings.copyFail") }))}>
                  {t("settings.notify.copyTopic")}
                </button>{" "}
                <button type="button" className="btn btn-sm btn-ghost" onClick={() => setShown("")}>
                  {t("settings.notify.copiedIt")}
                </button>
              </Alert>
            )}
            <p>
              <Chip tone={c.enabled ? "ok" : "mute"}>{c.enabled ? t("settings.enabled") : t("settings.disabled")}</Chip> <span className="small muted">{lastText(c.last, Math.floor(Date.now() / 1000))}</span>
            </p>
            <p className="small muted">
              {t("settings.notify.help")}
            </p>
            <form
              className="stack"
              onSubmit={(e) => {
                e.preventDefault();
                onSave();
              }}
            >
              <Field label={t("settings.notify.server")} hint={sErr || t("settings.notify.serverHint")}>
                {(id, dd) => <input id={id} aria-describedby={dd} aria-invalid={!!sErr} value={srv} onChange={(e) => setServer(e.target.value)} placeholder={c.default_server} autoComplete="off" spellCheck={false} />}
              </Field>
              <Field label={t("settings.notify.topic")} hint={tErr || (c.topic_set ? t("settings.notify.topicSetHint") : t("settings.notify.topicHint"))}>
                {(id, dd) => <input id={id} className="mono" aria-describedby={dd} aria-invalid={!!tErr} value={topic} onChange={(e) => setTopic(e.target.value)} placeholder={c.topic_set ? t("settings.notify.topicUnchanged") : t("settings.notify.topicFill")} autoComplete="off" spellCheck={false} />}
              </Field>
              <div className="row-end">
                <button type="button" className="btn btn-sm" onClick={() => setTopic(genTopic())}>
                  {t("settings.notify.genTopic")}
                </button>
              </div>
              <Field label={t("settings.notify.level")} hint={t("settings.notify.levelHint")}>
                {(id, dd) => (
                  <select id={id} aria-describedby={dd} value={lvl} onChange={(e) => setMin(e.target.value as "bad" | "warn")}>
                    <option value="bad">{t("settings.notify.levelBad")}</option>
                    <option value="warn">{t("settings.notify.levelWarn")}</option>
                  </select>
                )}
              </Field>
              <div className="row-end">
                <button className="btn btn-primary" disabled={busy || !dirty || !!sErr || !!tErr}>
                  {busy ? t("settings.saving") : t("common.save")}
                </button>
              </div>
            </form>
            <div className="row-end">
              {c.topic_set && (
                <>
                  <button type="button" className="btn" disabled={busy} onClick={() => save({ enabled: !c.enabled }, c.enabled ? t("settings.notify.disabledDone") : t("settings.notify.enabledDone"))}>
                    {c.enabled ? t("settings.notify.disable") : t("settings.notify.enable")}
                  </button>
                  <button type="button" className="btn" disabled={busy} onClick={test}>
                    {t("settings.notify.sendTest")}
                  </button>
                  <button type="button" className="btn btn-ghost" disabled={busy} onClick={() => setClearing(true)}>
                    {t("settings.notify.clear")}
                  </button>
                </>
              )}
            </div>
          </div>
        )}
      </State>
      {clearing && (
        <ConfirmDialog
          title={t("settings.notify.clearTitle")}
          body={t("settings.notify.clearBody")}
          word={t("settings.notify.clearWord")}
          verb={t("settings.notify.clear")}
          onConfirm={async () => {
            await post("/api/notify", { clear_topic: true });
            setClearing(false);
            setNote({ tone: "ok", text: t("settings.notify.clearDone") });
            await q.reload();
          }}
          onClose={() => setClearing(false)}
        />
      )}
    </Card>
  );
}
