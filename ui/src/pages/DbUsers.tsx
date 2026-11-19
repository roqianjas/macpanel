import { useState } from "react";
import { Copy } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import type { PostUrl } from "../api.gen";
import { t as tr, useT } from "../i18n";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, State, useLoad } from "../ui";

const PRIV_LABEL: Record<string, string> = {
  get ro() {
    return tr("db.priv.ro");
  },
  get rw() {
    return tr("db.priv.rw");
  },
  get all() {
    return tr("db.priv.all");
  },
};
const ENGINE_LABEL: Record<string, string> = { mariadb: "MariaDB", mysql: "MySQL", pgsql: "PostgreSQL" };

/** Tampilkan rahasia sekali (kata sandi acak). */
export function SecretDialog({ title, label, value, note, onClose }: { title: string; label: string; value: string; note?: string; onClose: () => void }) {
  const t = useT();
  const [msg, setMsg] = useState("");
  return (
    <Modal title={title} onClose={onClose}>
      <div className="stack">
        <Alert tone="warn">{t("db.secret.once")}</Alert>
        <span className="field-label">{label}</span>
        <div className="row">
          <code className="mono" style={{ wordBreak: "break-all" }}>{value}</code>
          <button
            className="btn btn-ghost btn-icon"
            aria-label={t("db.copy", { label })}
            onClick={async () => {
              try {
                await navigator.clipboard.writeText(value);
                setMsg(t("db.copied"));
              } catch {
                setMsg(t("db.copyFail"));
              }
            }}
          >
            <Copy size={16} aria-hidden="true" />
          </button>
        </div>
        {msg && <p className="small muted" role="status">{msg}</p>}
        {note && <p className="small muted">{note}</p>}
        <div className="row-end">
          <button className="btn btn-primary" onClick={onClose}>
            {t("db.saved")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

/** Dialog ganti kata sandi: acak atau pilihan sendiri, dengan ketik ulang nama. */
export function PasswordDialog({ title, word, hint, onClose, onSubmit }: { title: string; word: string; hint?: string; onClose: () => void; onSubmit: (password: string | undefined) => Promise<void> }) {
  const t = useT();
  const [typed, setTyped] = useState("");
  const [own, setOwn] = useState(false);
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const okPw = !own || /^[A-Za-z0-9._+=:,@%^-]{12,100}$/.test(pw);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await onSubmit(own ? pw : undefined);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={title} onClose={onClose}>
      <form className="stack" onSubmit={submit} autoComplete="off">
        {hint && <p className="small muted">{hint}</p>}
        <div className="seg" role="group" aria-label={t("db.pw.source")}>
          <button type="button" className={`seg-btn ${!own ? "on" : ""}`} aria-pressed={!own} onClick={() => setOwn(false)}>
            {t("db.pw.random")}
          </button>
          <button type="button" className={`seg-btn ${own ? "on" : ""}`} aria-pressed={own} onClick={() => setOwn(true)}>
            {t("db.pw.own")}
          </button>
        </div>
        {own && (
          <Field label={t("db.pw.new")} hint={t("db.pw.rule")}>
            {(id, dd) => <input id={id} aria-describedby={dd} type="password" autoComplete="new-password" placeholder={t("db.pw.ph")} maxLength={100} value={pw} onChange={(e) => setPw(e.target.value)} required />}
          </Field>
        )}
        <Field label={t("db.pw.confirm", { word })}>{(id) => <input id={id} className="mono" placeholder={word} value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" spellCheck={false} required />}</Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-danger" disabled={busy || typed !== word || !okPw}>
            {busy ? t("db.saving") : t("db.pw.change")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- user database per database ----------
interface DbUser {
  user: string;
  host: string;
  privs: string;
  primary: boolean;
  created?: number;
}
type Secret = { title: string; label: string; value: string; note?: string } | null;

export function DbUsersSection({ engine, name }: { engine: string; name: string }) {
  const t = useT();
  const base = `/api/db/${engine}/${name}/dbusers` as const;
  const q = useLoad(() => get<{ users: DbUser[]; port: number }>(base), [engine, name]);
  const [adding, setAdding] = useState(false);
  const [pwFor, setPwFor] = useState<string | null>(null);
  const [privFor, setPrivFor] = useState<DbUser | null>(null);
  const [delFor, setDelFor] = useState<string | null>(null);
  const [secret, setSecret] = useState<Secret>(null);
  const [note, setNote] = useState("");
  const port = q.data?.port ?? 0;
  return (
    <div className="stack">
      <div className="row-end" style={{ justifyContent: "space-between" }}>
        <h3 className="h3">{t("db.users.title")}</h3>
        <button className="btn" onClick={() => setAdding(true)}>
          {t("db.users.add")}
        </button>
      </div>
      <p className="small muted">
        {t("db.users.intro", { port: port ? `:${port}` : "" })}
      </p>
      {note && <Alert tone="ok">{note}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <ul className="list">
          {q.data?.users.map((u) => (
            <li key={u.user} className="list-row">
              <div>
                <p className="mono">{u.user}</p>
                <p className="small muted">
                  {u.primary ? t("db.users.primary") : t("db.users.created", { time: u.created ? fmtTime(u.created) : "-" })} · {u.host}
                </p>
              </div>
              <div className="row">
                <Chip tone={u.privs === "all" ? "info" : u.privs === "rw" ? "warn" : "ok"}>{PRIV_LABEL[u.privs] ?? u.privs}</Chip>
                {!u.primary && (
                  <>
                    <button className="btn" onClick={() => setPwFor(u.user)}>
                      {t("db.users.pwBtn")}<span className="sr"> {u.user}</span>
                    </button>
                    <button className="btn" onClick={() => setPrivFor(u)}>
                      {t("db.users.privBtn")}<span className="sr"> {u.user}</span>
                    </button>
                    <button className="btn btn-danger-ghost" onClick={() => setDelFor(u.user)}>
                      {t("db.users.delBtn")}<span className="sr"> {u.user}</span>
                    </button>
                  </>
                )}
              </div>
            </li>
          ))}
        </ul>
      </State>
      {adding && (
        <AddUser
          base={base}
          onClose={() => setAdding(false)}
          onDone={(r) => {
            setAdding(false);
            q.reload();
            if (r.password) setSecret({ title: t("db.users.createdTitle", { user: r.user }), label: t("db.pw.label"), value: r.password, note: t("db.users.createdNote", { port: r.port, name }) });
            else setNote(t("db.users.createdOwn", { user: r.user }));
          }}
        />
      )}
      {pwFor && (
        <PasswordDialog
          title={t("db.users.pwTitle", { user: pwFor })}
          word={pwFor}
          hint={t("db.users.pwHint")}
          onClose={() => setPwFor(null)}
          onSubmit={async (p) => {
            const r = await post<{ password?: string }>(`${base}/${pwFor}/passwd`, p ? { password: p } : {});
            const u = pwFor;
            setPwFor(null);
            if (r.password) setSecret({ title: t("db.users.pwChanged", { user: u }), label: t("db.pw.new"), value: r.password });
            else setNote(t("db.users.pwChangedNote", { user: u }));
          }}
        />
      )}
      {privFor && (
        <PrivDialog
          user={privFor}
          onClose={() => setPrivFor(null)}
          onSubmit={async (p) => {
            await post(`${base}/${privFor.user}/privs`, { privs: p });
            setNote(t("db.users.privSet", { user: privFor.user, priv: PRIV_LABEL[p] }));
            setPrivFor(null);
            q.reload();
          }}
        />
      )}
      {delFor && (
        <ConfirmDialog
          title={t("db.users.delTitle", { user: delFor })}
          word={delFor}
          verb={t("db.users.delVerb")}
          body={<p>{t("db.users.delBody")}</p>}
          onClose={() => setDelFor(null)}
          onConfirm={async () => {
            await post(`${base}/${delFor}/remove`, { confirm: delFor });
            setNote(t("db.users.deleted", { user: delFor }));
            setDelFor(null);
            q.reload();
          }}
        />
      )}
      {secret && <SecretDialog {...secret} onClose={() => setSecret(null)} />}
    </div>
  );
}

function AddUser({ base, onClose, onDone }: { base: PostUrl; onClose: () => void; onDone: (r: { user: string; password?: string; port: number }) => void }) {
  const t = useT();
  const [user, setUser] = useState("");
  const [privs, setPrivs] = useState("ro");
  const [own, setOwn] = useState(false);
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const okPw = !own || /^[A-Za-z0-9._+=:,@%^-]{12,100}$/.test(pw);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      onDone(await post(base, { user, privs, ...(own ? { password: pw } : {}) }));
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={t("db.users.addTitle")} onClose={onClose}>
      <form className="stack" onSubmit={submit} autoComplete="off">
        <Field label={t("db.users.name")} hint={t("db.users.nameHint")}>
          {(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="app_user" maxLength={32} value={user} onChange={(e) => setUser(e.target.value.toLowerCase())} pattern="[a-z][a-z0-9_]{2,31}" required data-autofocus spellCheck={false} />}
        </Field>
        <Field label={t("db.users.access")} hint={t("db.users.accessHint")}>
          {(id, dd) => (
            <select id={id} aria-describedby={dd} value={privs} onChange={(e) => setPrivs(e.target.value)}>
              <option value="ro">{t("db.users.optRo")}</option>
              <option value="rw">{t("db.users.optRw")}</option>
              <option value="all">{t("db.users.optAll")}</option>
            </select>
          )}
        </Field>
        <div className="seg" role="group" aria-label={t("db.pw.source")}>
          <button type="button" className={`seg-btn ${!own ? "on" : ""}`} aria-pressed={!own} onClick={() => setOwn(false)}>
            {t("db.pw.random")}
          </button>
          <button type="button" className={`seg-btn ${own ? "on" : ""}`} aria-pressed={own} onClick={() => setOwn(true)}>
            {t("db.pw.own")}
          </button>
        </div>
        {own && (
          <Field label={t("db.pw.label")} hint={t("db.pw.rule2")}>
            {(id, dd) => <input id={id} aria-describedby={dd} type="password" autoComplete="new-password" placeholder={t("db.pw.ph")} maxLength={100} value={pw} onChange={(e) => setPw(e.target.value)} required />}
          </Field>
        )}
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-primary" disabled={busy || !user || !okPw}>
            {busy ? t("db.creating") : t("db.users.create")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function PrivDialog({ user, onClose, onSubmit }: { user: DbUser; onClose: () => void; onSubmit: (p: string) => Promise<void> }) {
  const t = useT();
  const [p, setP] = useState(user.privs);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  return (
    <Modal title={t("db.users.privTitle", { user: user.user })} onClose={onClose}>
      <form
        className="stack"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setErr("");
          try {
            await onSubmit(p);
          } catch (x) {
            setErr(errText(x));
            setBusy(false);
          }
        }}
      >
        <Field label={t("db.users.priv")}>
          {(id) => (
            <select id={id} value={p} onChange={(e) => setP(e.target.value)}>
              <option value="ro">{t("db.priv.ro")}</option>
              <option value="rw">{t("db.priv.rw")}</option>
              <option value="all">{t("db.priv.all")}</option>
            </select>
          )}
        </Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-primary" disabled={busy || p === user.privs}>
            {busy ? t("db.saving") : t("common.save")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- kata sandi root ----------
export function RootPasswords() {
  const t = useT();
  const [eng, setEng] = useState<string | null>(null);
  const [secret, setSecret] = useState<Secret>(null);
  const [note, setNote] = useState("");
  return (
    <Card title={t("db.root.title")}>
      <div className="stack">
        <p className="small muted">
          {t("db.root.intro", { file: "db.env" })}
        </p>
        {note && <Alert tone="ok">{note}</Alert>}
        <ul className="list">
          {(["mariadb", "mysql", "pgsql"] as const).map((e) => (
            <li key={e} className="list-row">
              <div>
                <p>{ENGINE_LABEL[e]}</p>
                <p className="small muted">{e === "pgsql" ? t("db.root.devAcct") : t("db.root.rootAcct", { port: e === "mariadb" ? 3306 : 3307 })}</p>
              </div>
              <button className="btn btn-danger-ghost" onClick={() => setEng(e)}>
                {t("db.pw.change")}<span className="sr"> {ENGINE_LABEL[e]}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
      {eng && (
        <PasswordDialog
          title={t("db.root.pwTitle", { engine: ENGINE_LABEL[eng] })}
          word={eng}
          hint={t("db.root.pwHint")}
          onClose={() => setEng(null)}
          onSubmit={async (p) => {
            const r = await post<{ password?: string }>(`/api/dbroot/${eng}/passwd`, { confirm: eng, ...(p ? { password: p } : {}) });
            const label = ENGINE_LABEL[eng];
            setEng(null);
            if (r.password) setSecret({ title: t("db.root.changedTitle", { engine: label }), label: t("db.pw.new"), value: r.password, note: t("db.root.alsoSaved") });
            else setNote(t("db.root.changedNote", { engine: label }));
          }}
        />
      )}
      {secret && <SecretDialog {...secret} onClose={() => setSecret(null)} />}
    </Card>
  );
}
