import { useState } from "react";
import { Archive, ArchiveRestore, KeyRound, Plus, Trash2 } from "lucide-react";
import { errText, fmtTime, get, post, upload } from "../api";
import { trackJob } from "../jobs";
import DbConfig from "./DbConfig";
import DbAccess from "./DbAccess";
import RedisBrowser from "./RedisBrowser";
import { fmtBytes } from "../lib";
import { FacetBar, PagedList } from "../features/list/ListTools";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";
import { RootPasswords } from "./DbUsers";
import { useT } from "../i18n";

type Engine = "mariadb" | "mysql" | "pgsql";
const ENGINE_LABEL: Record<Engine, string> = { mariadb: "MariaDB 11.8 (port 3306)", mysql: "MySQL 8.4 (port 3307)", pgsql: "PostgreSQL 17 (port 5432)" };

export default function Databases() {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<{ databases: Record<Engine, string[]>; system: string[] }>("/api/db"));
  const isSys = (n: string) => (data?.system ?? ["dev"]).includes(n);
  const [adding, setAdding] = useState(false);
  const [eng, setEng] = useState("all");
  const [dropping, setDropping] = useState<{ engine: Engine; name: string } | null>(null);
  const [restoring, setRestoring] = useState<{ engine: Engine; name: string } | null>(null);
  const [access, setAccess] = useState<{ engine: Engine; name: string } | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [busy, setBusy] = useState("");

  const dump = async (engine: Engine, name: string) => {
    setBusy(`${engine}/${name}`);
    setNote(null);
    try {
      await post(`/api/db/${engine}/${name}/dump`, {});
      setNote({ tone: "ok", text: t("db.dumped", { name }) });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy("");
    }
  };
  const total = data ? [...data.databases.mariadb, ...data.databases.mysql, ...data.databases.pgsql].filter((n) => !isSys(n)).length : 0;
  return (
    <>
      <PageHead
        title={t("db.title")}
        desc={t("db.desc")}
        action={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            {t("db.create")}
          </button>
        }
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <State loading={loading && !data} error={error} empty={!!data && total === 0} emptyText={t("db.empty")} onRetry={reload}>
        <FacetBar
          label={t("db.filter")}
          value={eng}
          onChange={setEng}
          facets={[
            { id: "all", label: t("list.all"), count: total },
            ...(["mariadb", "mysql", "pgsql"] as Engine[]).map((en) => ({ id: en, label: ENGINE_LABEL[en].split(" (")[0], count: (data?.databases[en] ?? []).filter((n) => !isSys(n)).length })),
          ]}
        />
        {(["mariadb", "mysql", "pgsql"] as Engine[]).filter((en) => eng === "all" || eng === en).map((en) => (
          <Card key={en} title={ENGINE_LABEL[en]}>
            {(data?.databases[en] ?? []).filter((n) => !isSys(n)).length === 0 && !(data?.databases[en] ?? []).some(isSys) ? (
              <div className="state">
                <p className="muted">{t("db.engineEmpty", { engine: ENGINE_LABEL[en] })}</p>
              </div>
            ) : (
              <PagedList
                items={data!.databases[en]}
                text={(n) => n}
                label={t("db.search", { engine: ENGINE_LABEL[en] })}
                hint={t("list.hint")}
                render={(n) => (
                  <li key={n} className="list-row">
                    <span className="mono">{n}</span>
                    {isSys(n) ? (
                      <Chip tone="mute">{t("db.system")}</Chip>
                    ) : (
                    <div className="row">
                      <button className="btn" disabled={busy === `${en}/${n}`} onClick={() => dump(en, n)}>
                        <Archive size={18} aria-hidden="true" />
                        {busy === `${en}/${n}` ? t("db.dumping") : "Dump"}
                        <span className="sr"> {n}</span>
                      </button>
                      <button className="btn" onClick={() => setAccess({ engine: en, name: n })}>
                        <KeyRound size={18} aria-hidden="true" />
                        {t("db.access")}
                        <span className="sr"> {n}</span>
                      </button>
                      <button className="btn" onClick={() => setRestoring({ engine: en, name: n })}>
                        <ArchiveRestore size={18} aria-hidden="true" />
                        {t("db.restore")}
                        <span className="sr"> {n}</span>
                      </button>
                      <button className="btn btn-danger-ghost" onClick={() => setDropping({ engine: en, name: n })}>
                        <Trash2 size={18} aria-hidden="true" />
                        {t("db.remove")}
                        <span className="sr"> {n}</span>
                      </button>
                    </div>
                    )}
                  </li>
                )}
              />
            )}
          </Card>
        ))}
        <p className="small muted">{t("db.tip")}</p>
      </State>
      <RootPasswords />
      <DbConfig />
      <RedisBrowser />
      {access && <DbAccess engine={access.engine} name={access.name} onClose={() => setAccess(null)} />}
      {restoring && (
        <RestoreDialog
          engine={restoring.engine}
          name={restoring.name}
          onClose={() => setRestoring(null)}
          onStarted={(job, file) => {
            trackJob(job, t("db.restoreJob", { name: restoring.name, file }), (j) => {
              setNote(j.status === "ok" ? { tone: "ok", text: t("db.restored", { name: restoring.name, file }) } : { tone: "bad", text: j.output || t("db.restoreFail") });
              void reload();
            });
            setRestoring(null);
          }}
        />
      )}
      {adding && <AddDB onClose={() => setAdding(false)} onDone={() => { setAdding(false); reload(); }} />}
      {dropping && (
        <ConfirmDialog
          title={t("db.dropTitle", { name: dropping.name })}
          word={dropping.name}
          verb={t("db.dropVerb")}
          body={<p>{t("db.dropBody")}</p>}
          onConfirm={async () => {
            await post(`/api/db/${dropping.engine}/${dropping.name}/drop`, { confirm: dropping.name });
            setNote({ tone: "ok", text: t("db.dropped", { name: dropping.name }) });
            reload();
          }}
          onClose={() => setDropping(null)}
        />
      )}
    </>
  );
}

function AddDB({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const t = useT();
  const [name, setName] = useState("");
  const [engine, setEngine] = useState<Engine>("mariadb");
  const [env, setEnv] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const body: Record<string, string> = { name, engine };
      if (env) body.env = env;
      await post("/api/db", body);
      onDone();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={t("db.create")} onClose={onClose}>
      <form onSubmit={submit} className="stack">
        <Field label={t("db.add.name")} hint={t("db.add.nameHint")}>
          {(id, d) => <input id={id} aria-describedby={d} data-autofocus className="mono" placeholder="app_db" maxLength={32} value={name} onChange={(e) => setName(e.target.value.toLowerCase())} required pattern="[a-z][a-z0-9_]{0,31}" />}
        </Field>
        <Field label={t("db.add.engine")}>
          {(id) => (
            <select id={id} value={engine} onChange={(e) => setEngine(e.target.value as Engine)}>
              <option value="mariadb">{ENGINE_LABEL.mariadb}</option>
              <option value="mysql">MySQL 8.4 (port 3307)</option>
              <option value="pgsql">PostgreSQL 17</option>
            </select>
          )}
        </Field>
        <Field label={t("db.add.env")} hint={t("db.add.envHint")}>
          {(id, d) => <input id={id} aria-describedby={d} className="mono" value={env} onChange={(e) => setEnv(e.target.value)} placeholder={t("db.add.envPh")} />}
        </Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-primary" disabled={busy || !name}>
            {busy ? t("db.creating") : t("db.create")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function RestoreDialog({ engine, name, onClose, onStarted }: { engine: Engine; name: string; onClose: () => void; onStarted: (job: number, file: string) => void }) {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<{ dumps: { file: string; at: number; size: number }[] }>(`/api/db/${engine}/${name}/dumps`));
  const [file, setFile] = useState("");
  const [up, setUp] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const dumps = data?.dumps ?? [];
  const pick = file || dumps[0]?.file || "";
  const go = async () => {
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ job: number }>(`/api/db/${engine}/${name}/restore`, { file: pick });
      onStarted(r.job, pick);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={t("db.rs.title", { name })} onClose={onClose}>
      <div className="stack">
        <p className="small">{t("db.rs.note1")}<strong>{t("db.rs.empty")}</strong>{t("db.rs.note2")}</p>
        <State loading={loading && !data} error={error} empty={!!data && dumps.length === 0} emptyText={t("db.rs.noDumps", { name })} onRetry={reload}>
          <Field label={t("db.rs.pick")}>
            {(id) => (
              <select id={id} data-autofocus value={pick} onChange={(e) => setFile(e.target.value)}>
                {dumps.map((d) => (
                  <option key={d.file} value={d.file}>
                    {fmtTime(d.at)} ({fmtBytes(d.size)})
                  </option>
                ))}
              </select>
            )}
          </Field>
        </State>
        <Field label={t("db.rs.upload")} hint={t("db.rs.uploadHint")}>
          {(id, dd) => (
            <input
              id={id}
              aria-describedby={dd}
              type="file"
              accept=".sql,.gz,application/gzip,application/sql,text/plain"
              disabled={up || busy}
              onChange={async (e) => {
                const f = e.target.files?.[0];
                if (!f) return;
                setUp(true);
                setErr("");
                try {
                  const r = await upload<{ file: string }>(`/api/db/${engine}/${name}/upload`, f);
                  setFile(r.file);
                  await reload();
                } catch (x) {
                  setErr(errText(x));
                } finally {
                  setUp(false);
                  e.target.value = "";
                }
              }}
            />
          )}
        </Field>
        {up && <p className="small muted" role="status">{t("db.rs.uploading")}</p>}
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn btn-primary" disabled={busy || up || !pick} onClick={go}>{busy ? t("db.rs.starting") : t("db.restore")}</button>
        </div>
      </div>
    </Modal>
  );
}
