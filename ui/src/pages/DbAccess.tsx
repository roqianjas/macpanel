import { useState } from "react";
import { Copy, Eye, KeyRound } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Modal, State, useLoad } from "../ui";
import { DbUsersSection, PasswordDialog } from "./DbUsers";
import { useT } from "../i18n";

type Creds = Record<string, string>;

export default function DbAccess({ engine, name, onClose }: { engine: string; name: string; onClose: () => void }) {
  const t = useT();
  const q = useLoad(() => get<{ users: { user: string; host: string }[]; cred: boolean; rotated: number }>(`/api/db/${engine}/${name}/users`), [engine, name]);
  const [creds, setCreds] = useState<Creds | null>(null);
  const [show, setShow] = useState(false);
  const [err, setErr] = useState("");
  const [note, setNote] = useState("");
  const [rotate, setRotate] = useState(false);
  const load = async () => {
    setErr("");
    try {
      setCreds(await post<Creds>(`/api/db/${engine}/${name}/creds`, {}));
    } catch (x) {
      setErr(errText(x));
    }
  };
  const copy = async (v: string, what: string) => {
    try {
      await navigator.clipboard.writeText(v);
      setNote(t("db.ac.copied", { what }));
    } catch {
      setNote(t("db.ac.noClip"));
    }
  };
  return (
    <Modal title={t("db.ac.title", { name, engine: { mariadb: "MariaDB", mysql: "MySQL", pgsql: "PostgreSQL" }[engine] ?? engine })} onClose={onClose} wide>
      <div className="stack">
        <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
          <ul className="list">
            {q.data?.users.map((u, i) => (
              <li key={i} className="list-row">
                <div>
                  <p className="mono">{u.user}{u.host ? `@${u.host}` : ""}</p>
                  <p className="small muted">{t("db.ac.primary")}</p>
                </div>
              </li>
            ))}
          </ul>
          <p className="small muted">{t("db.ac.rotated", { time: q.data?.rotated ? fmtTime(q.data.rotated) : "-", path: "~/.config/macminiops/projects/" })}</p>
        </State>
        {err && <Alert>{err}</Alert>}
        {note && <Alert tone="ok">{note}</Alert>}
        {creds ? (
          <dl className="kv">
            {Object.entries(creds).map(([k, v]) => (
              <div key={k} className="kv-row">
                <dt className="mono small">{k}</dt>
                <dd className="mono">{k === "DB_PASSWORD" && !show ? "••••••••••••" : v}</dd>
                <button className="btn btn-ghost btn-icon" onClick={() => copy(v, k)} aria-label={t("db.copy", { label: k })}>
                  <Copy size={16} aria-hidden="true" />
                </button>
              </div>
            ))}
          </dl>
        ) : null}
        <div className="row-end">
          {creds && (
            <button className="btn" onClick={() => setShow((s) => !s)}>
              <Eye size={18} aria-hidden="true" />
              {show ? t("db.ac.hide") : t("db.ac.show")}
            </button>
          )}
          {!creds && (
            <button className="btn" onClick={load}>
              <Eye size={18} aria-hidden="true" />
              {t("db.ac.view")}
            </button>
          )}
          <button className="btn btn-danger-ghost" onClick={() => setRotate(true)}>
            <KeyRound size={18} aria-hidden="true" />
            {t("db.pw.change")}
          </button>
        </div>
        <p className="small muted">{t("db.ac.audit", { env: ".env" })}</p>
        <hr className="sep" />
        <DbUsersSection engine={engine} name={name} />
      </div>
      {rotate && (
        <PasswordDialog
          title={t("db.ac.pwTitle", { name })}
          word={name}
          hint={t("db.ac.pwHint")}
          onClose={() => setRotate(false)}
          onSubmit={async (p) => {
            await post(`/api/db/${engine}/${name}/passwd`, { confirm: name, ...(p ? { password: p } : {}) });
            setRotate(false);
            setCreds(null);
            setShow(false);
            q.reload();
            setNote(t("db.ac.changed"));
          }}
        />
      )}
    </Modal>
  );
}
