import { useState } from "react";
import { errText, get, post } from "../api";
import { trackJob } from "../jobs";
import { useT } from "../i18n";
import { Alert, Card, Field, Modal, State, Tabs, useLoad } from "../ui";

interface Key {
  key: string;
  value: string;
  live: string;
  hint: string;
}

const ENGINES = [
  { id: "mariadb", label: "MariaDB" },
  { id: "mysql", label: "MySQL" },
  { id: "redis", label: "Redis" },
];

export default function DbConfig() {
  const t = useT();
  const [eng, setEng] = useState("mariadb");
  return (
    <Card title={t("db.cfg.title")}>
      <Tabs tabs={ENGINES} value={eng} onChange={setEng} label={t("db.cfg.servers")} />
      <div role="tabpanel" aria-labelledby={`tab-${eng}`} className="tabpanel">
        <Form key={eng} engine={eng} />
      </div>
    </Card>
  );
}

function Form({ engine }: { engine: string }) {
  const t = useT();
  const q = useLoad(() => get<{ keys: Key[] }>(`/api/dbcfg/${engine}`), [engine]);
  const [edit, setEdit] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [slow, setSlow] = useState<string | null>(null);
  const isMy = engine === "mysql" || engine === "mariadb";
  const changed = (q.data?.keys ?? []).filter((k) => (edit[k.key] ?? "").trim() !== "" && edit[k.key].trim() !== k.value);
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const values: Record<string, string> = {};
      changed.forEach((k) => (values[k.key] = edit[k.key].trim()));
      const r = await post<{ job: number }>(`/api/dbcfg/${engine}`, { values });
      trackJob(r.job, t("db.cfg.job", { engine }), (j) => {
        setBusy(false);
        setNote(j.status === "ok" ? { tone: "ok", text: t("db.cfg.ok") } : { tone: "bad", text: j.output || t("db.cfg.fail") });
        setEdit({});
        q.reload();
      });
    } catch (x) {
      setBusy(false);
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const openSlow = async () => {
    try {
      setSlow((await get<{ log: string }>(`/api/dbcfg/${engine}/slow`)).log || t("db.cfg.emptyLog"));
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <form onSubmit={save} className="stack">
      <p className="small muted">
        {t("db.cfg.intro", { file: isMy ? "my.cnf" : "redis.conf", engine: { mariadb: "MariaDB", mysql: "MySQL" }[engine] ?? "Redis" })}
      </p>
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="grid-form">
          {q.data?.keys.map((k) => (
            <Field key={k.key} label={k.key} hint={t("db.cfg.live", { hint: k.hint, live: k.live || "?" })}>
              {(id, dd) => <input id={id} aria-describedby={dd} className="mono" value={edit[k.key] ?? ""} placeholder={t("db.cfg.unchanged")} onChange={(e) => setEdit((p) => ({ ...p, [k.key]: e.target.value }))} spellCheck={false} />}
            </Field>
          ))}
        </div>
        <div className="row-end">
          {isMy && (
            <button type="button" className="btn" onClick={openSlow}>
              {t("db.cfg.slowBtn")}
            </button>
          )}
          <button className="btn btn-primary" disabled={busy || changed.length === 0}>
            {busy ? t("db.saving") : changed.length ? t("db.cfg.saveRestartN", { count: changed.length }) : t("db.cfg.saveRestart")}
          </button>
        </div>
      </State>
      {slow !== null && (
        <Modal title={t("db.cfg.slowTitle", { engine: engine === "mariadb" ? "MariaDB" : "MySQL" })} onClose={() => setSlow(null)} wide>
          <pre className="logbox mono" tabIndex={0} aria-label={t("db.cfg.slowAria")}>{slow}</pre>
        </Modal>
      )}
    </form>
  );
}
