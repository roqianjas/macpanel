import { useState } from "react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";
import { SecretDialog } from "./DbUsers";
import { Avatar } from "../features/profile/avatar";
import { useT } from "../i18n";

interface Acc {
  name: string;
  role: string;
  disabled: boolean;
  totp_on: boolean;
  must_change: boolean;
  created: number;
  self: boolean;
  display_name: string;
  avatar: string;
}
type Secret = { title: string; label: string; value: string; note?: string } | null;

export default function Users() {
  const t = useT();
  const q = useLoad(() => get<{ accounts: Acc[] }>("/api/accounts"));
  const [adding, setAdding] = useState(false);
  const [sel, setSel] = useState<Acc | null>(null);
  const [secret, setSecret] = useState<Secret>(null);
  const [note, setNote] = useState("");
  const list = q.data?.accounts ?? [];
  return (
    <>
      <PageHead
        title={t("nav.users")}
        desc={t("settings.users.desc")}
        action={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            {t("settings.users.add")}
          </button>
        }
      />
      {note && <Alert tone="ok">{note}</Alert>}
      <Card title={t("settings.users.list")}>
        <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && list.length === 0} emptyText={t("settings.users.empty")} onRetry={q.reload}>
          <ul className="list">
            {list.map((u) => (
              <li key={u.name} className="list-row user-row">
                <Avatar name={u.name} display={u.display_name} avatar={u.avatar} size={44} />
                <div>
                  {u.display_name && <p className="profile-name">{u.display_name}</p>}
                  <p className="mono">
                    @{u.name}
                    {u.self ? t("settings.users.you") : ""}
                  </p>
                  <p className="small muted">{t("settings.users.created", { time: fmtTime(u.created) })}</p>
                  <div className="row" style={{ flexWrap: "wrap" }}>
                    <Chip tone={u.role === "admin" ? "info" : "mute"}>{u.role === "admin" ? t("role.admin") : t("role.viewer")}</Chip>
                    <Chip tone={u.totp_on ? "ok" : "warn"}>{u.totp_on ? t("settings.users.2faOn") : t("settings.users.2faOff")}</Chip>
                    {u.disabled && <Chip tone="bad">{t("settings.disabled")}</Chip>}
                    {u.must_change && <Chip tone="warn">{t("settings.users.mustChange")}</Chip>}
                  </div>
                </div>
                {u.self ? (
                  <a className="btn" href="#/profil">
                    {t("profile.mine")}<span className="sr">{t("settings.users.mineSr")}</span>
                  </a>
                ) : (
                  <button className="btn" onClick={() => setSel(u)}>
                    {t("settings.users.manage")}<span className="sr"> {u.name}</span>
                  </button>
                )}
              </li>
            ))}
          </ul>
        </State>
      </Card>
      {adding && (
        <AddAccount
          onClose={() => setAdding(false)}
          onDone={(r) => {
            setAdding(false);
            q.reload();
            if (!r.temp_password) return setNote(t("settings.users.createdOwn", { name: r.name }));
            setSecret({ title: t("settings.users.createdTitle", { name: r.name }), label: t("settings.users.tempPw"), value: r.temp_password ?? "", note: t("settings.users.createdNote") });
          }}
        />
      )}
      {sel && (
        <Manage
          acc={sel}
          onClose={() => setSel(null)}
          onChanged={(msg, sec) => {
            setSel(null);
            q.reload();
            if (sec) setSecret(sec);
            else setNote(msg);
          }}
        />
      )}
      {secret && <SecretDialog {...secret} onClose={() => setSecret(null)} />}
    </>
  );
}

function AddAccount({ onClose, onDone }: { onClose: () => void; onDone: (r: { name: string; temp_password?: string }) => void }) {
  const t = useT();
  const [name, setName] = useState("");
  const [role, setRole] = useState("viewer");
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  return (
    <Modal title={t("settings.users.add")} onClose={onClose}>
      <form
        className="stack"
        autoComplete="off"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setErr("");
          try {
            onDone(await post("/api/accounts", pw ? { name, role, password: pw } : { name, role }));
          } catch (x) {
            setErr(errText(x));
            setBusy(false);
          }
        }}
      >
        <Field label={t("login.user")} hint={t("settings.users.nameHint")}>
          {(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder={t("settings.users.namePh")} maxLength={32} value={name} onChange={(e) => setName(e.target.value.toLowerCase())} pattern="[a-z][a-z0-9_-]{2,31}" required data-autofocus spellCheck={false} />}
        </Field>
        <Field label={t("settings.users.role")} hint={t("settings.users.roleHint")}>
          {(id, dd) => (
            <select id={id} aria-describedby={dd} value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="viewer">{t("role.viewer")}</option>
              <option value="admin">{t("settings.users.adminAll")}</option>
            </select>
          )}
        </Field>
        <PasswordField value={pw} onChange={setPw} />
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-primary" disabled={busy || !name || (pw !== "" && pw.length < 12)}>
            {busy ? t("settings.creating") : t("settings.users.createAcc")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function Manage({ acc, onClose, onChanged }: { acc: Acc; onClose: () => void; onChanged: (msg: string, secret?: Secret) => void }) {
  const t = useT();
  const [role, setRole] = useState(acc.role);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [del, setDel] = useState(false);
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setErr("");
    try {
      await fn();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  const base = `/api/accounts/${acc.name}` as const;
  return (
    <Modal title={t("settings.users.manageTitle", { name: acc.name })} onClose={onClose}>
      <div className="stack">
        {err && <Alert>{err}</Alert>}
        <Field label={t("settings.users.role")} hint={t("settings.users.roleHint2")}>
          {(id, dd) => (
            <div className="row">
              <select id={id} aria-describedby={dd} value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="viewer">{t("role.viewer")}</option>
                <option value="admin">{t("role.admin")}</option>
              </select>
              <button className="btn" disabled={busy || role === acc.role} onClick={() => run(async () => { await post(`${base}/role`, { role }); onChanged(t("settings.users.roleChanged", { name: acc.name })); })}>
                {t("settings.users.saveRole")}
              </button>
            </div>
          )}
        </Field>
        <div className="stack">
          <button className="btn" disabled={busy} onClick={() => run(async () => { await post(`${base}/${acc.disabled ? "enable" : "disable"}`, {}); onChanged(t(acc.disabled ? "settings.users.nowEnabled" : "settings.users.nowDisabled", { name: acc.name })); })}>
            {acc.disabled ? t("settings.users.enableAcc") : t("settings.users.disableAcc")}
          </button>
          <ResetPassword busy={busy} run={run} base={base} acc={acc} onChanged={onChanged} />
          {!acc.totp_on && <p className="small muted">{t("settings.users.2faOffNote")}</p>}
          {acc.totp_on && (
            <button className="btn" disabled={busy} onClick={() => run(async () => { await post(`${base}/2fa-off`, {}); onChanged(t("settings.users.2faOffDone", { name: acc.name })); })}>
              {t("settings.users.2faOffBtn")}
            </button>
          )}
          <button className="btn btn-danger-ghost" disabled={busy} onClick={() => setDel(true)}>
            {t("settings.users.delete")}
          </button>
        </div>
      </div>
      {del && (
        <ConfirmDialog
          title={t("settings.users.deleteTitle", { name: acc.name })}
          word={acc.name}
          verb={t("settings.users.delete")}
          body={<p>{t("settings.users.deleteBody")}</p>}
          onClose={() => setDel(false)}
          onConfirm={async () => {
            await post(`${base}/remove`, { confirm: acc.name });
            setDel(false);
            onChanged(t("settings.users.deleted", { name: acc.name }));
          }}
        />
      )}
    </Modal>
  );
}

function PasswordField({ value, onChange, label }: { value: string; onChange: (v: string) => void; label?: string }) {
  const t = useT();
  const [show, setShow] = useState(false);
  const gen = () => {
    const a = new Uint8Array(18);
    crypto.getRandomValues(a);
    onChange(btoa(String.fromCharCode(...a)).replace(/[+/=]/g, "x").slice(0, 20));
    setShow(true);
  };
  return (
    <Field label={label ?? t("settings.users.tempOpt")} hint={value ? (value.length < 12 ? t("settings.users.pwHint.short", { len: value.length }) : t("settings.users.pwHint.ok")) : t("settings.users.pwHint.empty")}>
      {(id, dd) => (
        <div className="row">
          <input id={id} aria-describedby={dd} className="mono" type={show ? "text" : "password"} autoComplete="new-password" placeholder={t("settings.users.pwPh")} maxLength={256} value={value} onChange={(e) => onChange(e.target.value)} />
          <button type="button" className="btn" onClick={() => setShow((v) => !v)} aria-pressed={show}>{show ? t("settings.users.hide") : t("settings.users.show")}</button>
          <button type="button" className="btn" onClick={gen}>{t("settings.users.random")}</button>
        </div>
      )}
    </Field>
  );
}

function ResetPassword({ busy, run, base, acc, onChanged }: { busy: boolean; run: (fn: () => Promise<void>) => void; base: string; acc: Acc; onChanged: (msg: string, secret?: Secret) => void }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [pw, setPw] = useState("");
  if (!open) return <button className="btn" disabled={busy} onClick={() => setOpen(true)}>{t("settings.users.reset")}</button>;
  return (
    <div className="stack sched">
      <PasswordField value={pw} onChange={setPw} label={t("settings.users.tempOptNew")} />
      <div className="row">
        <button className="btn" onClick={() => { setOpen(false); setPw(""); }}>{t("common.cancel")}</button>
        <button className="btn btn-primary" disabled={busy || (pw !== "" && pw.length < 12)} onClick={() => run(async () => {
          const r = await post<{ temp_password?: string }>(`${base}/reset` as `/api/accounts/${string}/reset`, pw ? { password: pw } : {});
          if (!r.temp_password) return onChanged(t("settings.users.resetOwn", { name: acc.name }));
          onChanged("", { title: t("settings.users.resetTitle", { name: acc.name }), label: t("settings.users.tempPw"), value: r.temp_password, note: t("settings.users.resetNote") });
        })}>{t("settings.users.reset")}</button>
      </div>
    </div>
  );
}
