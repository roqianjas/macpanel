import { useState } from "react";
import { Trash2 } from "lucide-react";
import { errText, get, post } from "../../api";
import { Alert, Card, ConfirmDialog, Field, State, useLoad } from "../../ui";
import { useT } from "../../i18n";

export function AuthUsers({ name, enabled }: { name: string; enabled: boolean }) {
  const t = useT();
  const q = useLoad(() => get<{ users: string[] }>(`/api/sites/${name}/auth`), [name]);
  const [user, setUser] = useState("");
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [rm, setRm] = useState<string | null>(null);
  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await post(`/api/sites/${name}/auth/users`, { user, password: pw });
      setUser("");
      setPw("");
      q.reload();
    } catch (x) {
      setErr(errText(x));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title={t("dev.auth.title")}>
      <p className="small muted">{enabled ? t("dev.auth.on") : t("dev.auth.off")} {t("dev.auth.note")}</p>
      <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && q.data.users.length === 0} emptyText={t("dev.auth.none")} onRetry={q.reload}>
        <ul className="list">
          {q.data?.users.map((u) => (
            <li key={u} className="list-row">
              <span className="mono">{u}</span>
              <button className="btn btn-danger-ghost" onClick={() => setRm(u)}>
                <Trash2 size={18} aria-hidden="true" />
                {t("apps.remove")}<span className="sr"> {u}</span>
              </button>
            </li>
          ))}
        </ul>
      </State>
      <form onSubmit={add} className="stack">
        <div className="grid2">
          <Field label={t("login.user")}>{(id) => <input id={id} className="mono" placeholder={t("login.user.ph")} maxLength={32} value={user} onChange={(e) => setUser(e.target.value)} autoComplete="off" required pattern="[A-Za-z0-9._\-]{1,32}" />}</Field>
          <Field label={t("login.pw")} hint={t("dev.auth.pwHint")}>{(id, dd) => <input id={id} aria-describedby={dd} type="password" placeholder={t("dev.auth.pwHint")} value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="new-password" minLength={8} maxLength={128} required />}</Field>
        </div>
        {err && <Alert>{err}</Alert>}
        <div>
          <button className="btn" disabled={busy || !user || pw.length < 8}>{busy ? t("dev.php.saving") : t("dev.auth.addOrReplace")}</button>
        </div>
      </form>
      {rm && (
        <ConfirmDialog
          title={t("dev.auth.rmTitle", { name: rm })}
          word={rm}
          verb={t("dev.auth.rmVerb")}
          body={<p>{t("dev.auth.rmBody")}</p>}
          onConfirm={async () => {
            await post(`/api/sites/${name}/auth/users/${rm}/remove`, {});
            q.reload();
          }}
          onClose={() => setRm(null)}
        />
      )}
    </Card>
  );
}
