import { useState } from "react";
import { t, useT } from "../i18n";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";
import { hasDevicePref, setDevicePref, setPublicSettings, useServerTheme, useThemePref, usePanelName, type Pref } from "../settings";
import AppearanceCard from "../features/appearance/AppearanceCard";
import NotifyCard from "../features/notify/NotifyCard";
import ThresholdsCard from "../features/hardware/ThresholdsCard";

const THEMES: { id: Pref; key: string }[] = [
  { id: "system", key: "theme.system" },
  { id: "light", key: "theme.light" },
  { id: "dark", key: "theme.dark" },
];

type Note = { tone: "ok" | "bad"; text: string } | null;

async function copy(v: string, set: (n: Note) => void, what: string) {
  try {
    await navigator.clipboard.writeText(v);
    set({ tone: "ok", text: t("settings.copied", { what }) });
  } catch {
    set({ tone: "bad", text: t("settings.copyFail") });
  }
}

// ---------- Panel: nama dan tema ----------
function PanelCard() {
  const t = useT();
  const q = useLoad(() => get<{ name: string; theme: Pref; can_edit: boolean }>("/api/settings"));
  const name = usePanelName();
  const pref = useThemePref();
  const serverTheme = useServerTheme();
  const [draft, setDraft] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const canEdit = q.data?.can_edit ?? false;
  const save = async (body: { name?: string; theme?: string }, done: string) => {
    setBusy(true);
    setNote(null);
    try {
      setPublicSettings(await post<{ name: string; theme: string }>("/api/settings", body));
      setDraft(null);
      setNote({ tone: "ok", text: done });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const shown = draft ?? name;
  return (
    <Card title={t("settings.panel.title")}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
          <form
            className="stack"
            onSubmit={(e) => {
              e.preventDefault();
              save({ name: shown }, t("settings.name.saved"));
            }}
          >
            <Field label={t("settings.name.label")} hint={canEdit ? t("settings.name.hint.edit") : t("settings.name.hint.ro")}>
              {(id, dd) => <input id={id} aria-describedby={dd} placeholder="MacPanel" value={shown} maxLength={40} disabled={!canEdit} onChange={(e) => setDraft(e.target.value)} required />}
            </Field>
            {canEdit && (
              <div className="row-end">
                <button className="btn btn-primary" disabled={busy || draft === null || draft.trim() === "" || draft === name}>
                  {busy ? t("settings.saving") : t("settings.name.save")}
                </button>
              </div>
            )}
          </form>
          <div className="stack">
            <span className="field-label">{t("settings.theme.device")}</span>
            <div className="seg" role="group" aria-label={t("settings.theme.device")}>
              {THEMES.map((th) => (
                <button key={th.id} type="button" className={`seg-btn ${pref === th.id ? "on" : ""}`} aria-pressed={pref === th.id} onClick={() => setDevicePref(th.id)}>
                  {t(th.key)}
                </button>
              ))}
            </div>
            <p className="small muted">
              {t("settings.theme.local")}{" "}
              {hasDevicePref() && (
                <button type="button" className="btn btn-ghost" onClick={() => setDevicePref(null)}>
                  {t("settings.theme.reset")}
                </button>
              )}
            </p>
          </div>
          {canEdit && (
            <Field label={t("settings.theme.default")} hint={t("settings.theme.defaultHint")}>
              {(id, dd) => (
                <select id={id} aria-describedby={dd} value={serverTheme} disabled={busy} onChange={(e) => save({ theme: e.target.value }, t("settings.theme.defaultSaved"))}>
                  {THEMES.map((th) => (
                    <option key={th.id} value={th.id}>
                      {t(th.key)}
                    </option>
                  ))}
                </select>
              )}
            </Field>
          )}
        </div>
      </State>
    </Card>
  );
}

// ---------- 2FA ----------
export function TwoFactor() {
  const t = useT();
  const q = useLoad(() => get<{ enabled: boolean; recovery_left: number }>("/api/me/2fa"));
  const [note, setNote] = useState<Note>(null);
  const [setup, setSetup] = useState<{ secret: string; uri: string } | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [off, setOff] = useState(false);
  const [busy, setBusy] = useState(false);
  const start = async () => {
    setBusy(true);
    setNote(null);
    try {
      setSetup(await post<{ secret: string; uri: string }>("/api/me/2fa/setup", {}));
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const on = q.data?.enabled;
  return (
    <Card title={t("settings.2fa.title")} action={q.data ? <Chip tone={on ? "ok" : "warn"}>{on ? t("settings.on") : t("settings.off")}</Chip> : undefined}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
          {on ? (
            <>
              <p className="small muted">{t("settings.2fa.onText", { left: q.data?.recovery_left ?? "" })}</p>
              <div className="row-end">
                <button className="btn btn-danger-ghost" onClick={() => setOff(true)}>
                  {t("settings.2fa.disable")}
                </button>
              </div>
            </>
          ) : (
            <>
              <Alert tone="warn">{t("settings.2fa.offWarn")}</Alert>
              <div className="row-end">
                <button className="btn btn-primary" disabled={busy} onClick={start}>
                  {busy ? t("settings.preparing") : t("settings.2fa.enable")}
                </button>
              </div>
            </>
          )}
        </div>
      </State>
      {setup && <EnableDialog setup={setup} onClose={() => setSetup(null)} onDone={(c) => { setSetup(null); setCodes(c); q.reload(); }} />}
      {codes && <CodesDialog codes={codes} onClose={() => setCodes(null)} />}
      {off && <DisableDialog onClose={() => setOff(false)} onDone={() => { setOff(false); q.reload(); setNote({ tone: "ok", text: t("settings.2fa.disabledDone") }); }} />}
    </Card>
  );
}

function EnableDialog({ setup, onClose, onDone }: { setup: { secret: string; uri: string }; onClose: () => void; onDone: (codes: string[]) => void }) {
  const t = useT();
  const [code, setCode] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ recovery_codes: string[] }>("/api/me/2fa/enable", { code: code.trim() });
      onDone(r.recovery_codes);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={t("settings.2fa.enable")} onClose={onClose}>
      <form className="stack" onSubmit={submit}>
        <p className="small muted">{t("settings.2fa.setupHelp")}</p>
        <div className="row">
          <code className="mono" style={{ wordBreak: "break-all" }}>{setup.secret}</code>
          <button type="button" className="btn" onClick={() => copy(setup.secret, setNote, t("settings.copy.key"))}>
            {t("settings.copy")}
          </button>
        </div>
        {note && <Alert tone={note.tone}>{note.text}</Alert>}
        <Field label={t("settings.2fa.code")} hint={t("settings.2fa.codeHint")}>
          {(id, dd) => <input id={id} aria-describedby={dd} data-autofocus inputMode="numeric" autoComplete="one-time-code" className="mono code-input" placeholder="123456" value={code} onChange={(e) => setCode(e.target.value)} required />}
        </Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-primary" disabled={busy || code.trim().length < 6}>
            {busy ? t("login.checking") : t("settings.activate")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function CodesDialog({ codes, onClose }: { codes: string[]; onClose: () => void }) {
  const t = useT();
  const [note, setNote] = useState<Note>(null);
  return (
    <Modal title={t("settings.codes.title")} onClose={onClose}>
      <div className="stack">
        <Alert tone="warn">{t("settings.codes.warn")}</Alert>
        <ul className="list mono">
          {codes.map((c) => (
            <li key={c} className="list-row">
              {c}
            </li>
          ))}
        </ul>
        {note && <Alert tone={note.tone}>{note.text}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={() => copy(codes.join("\n"), setNote, t("settings.codes.name"))}>
            {t("settings.copyAll")}
          </button>
          <button className="btn btn-primary" onClick={onClose}>
            {t("settings.savedIt")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function DisableDialog({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const t = useT();
  const [pw, setPw] = useState("");
  const [code, setCode] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await post("/api/me/2fa/disable", { password: pw, code: code.trim() });
      onDone();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={t("settings.2fa.disable")} onClose={onClose}>
      <form className="stack" onSubmit={submit} autoComplete="off">
        <p className="small muted">{t("settings.2fa.disableHelp")}</p>
        <Field label={t("login.pw")}>{(id) => <input id={id} type="password" data-autofocus autoComplete="current-password" placeholder={t("login.pw.ph")} value={pw} onChange={(e) => setPw(e.target.value)} required />}</Field>
        <Field label={t("settings.2fa.codeOrRecovery")}>{(id) => <input id={id} autoComplete="one-time-code" className="mono code-input" placeholder="123456" value={code} onChange={(e) => setCode(e.target.value)} required />}</Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-danger" disabled={busy || !pw || code.trim().length < 6}>
            {busy ? t("login.checking") : t("settings.2fa.disable")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- Token API ----------
interface Tok {
  id: number;
  name: string;
  prefix: string;
  scope: string;
  created: number;
  expires: number;
  last_used: number;
  last_ip: string;
}
const SCOPE_KEY: Record<string, string> = { read: "settings.scope.read", operate: "settings.scope.operate", full: "settings.scope.full" };
const scopeLabel = (s: string) => (SCOPE_KEY[s] ? t(SCOPE_KEY[s]) : s);
const SCOPE_TONE: Record<string, "ok" | "warn" | "bad"> = { read: "ok", operate: "warn", full: "bad" };

function Tokens() {
  const t = useT();
  const q = useLoad(() => get<{ tokens: Tok[]; max: number }>("/api/me/tokens"));
  const [name, setName] = useState("");
  const [scope, setScope] = useState("operate");
  const [days, setDays] = useState("90");
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [made, setMade] = useState<{ token: string; scope: string } | null>(null);
  const [rev, setRev] = useState<Tok | null>(null);
  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const r = await post<{ token: string; scope: string }>("/api/me/tokens", { name: name.trim(), scope, days: Number(days) });
      setMade(r);
      setName("");
      q.reload();
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const list = q.data?.tokens ?? [];
  return (
    <Card title={t("settings.tok.title")}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          <p className="small muted">
            {t("settings.tok.i1")}<span className="mono">panelctl</span>{t("settings.tok.i2")}<span className="mono">Authorization: Bearer</span>{t("settings.tok.i3")}
          </p>
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
          <form className="stack" onSubmit={create}>
            <div className="grid2">
              <Field label={t("settings.tok.name")} hint={t("settings.tok.nameHint")}>
                {(id, dd) => <input id={id} aria-describedby={dd} placeholder={t("settings.tok.namePh")} value={name} maxLength={40} onChange={(e) => setName(e.target.value)} required autoComplete="off" />}
              </Field>
              <Field label={t("settings.tok.level")}>
                {(id) => (
                  <select id={id} value={scope} onChange={(e) => setScope(e.target.value)}>
                    <option value="read">{t("settings.scope.read")}</option>
                    <option value="operate">{t("settings.scope.operate")}</option>
                    <option value="full">{t("settings.scope.full")}</option>
                  </select>
                )}
              </Field>
              <Field label={t("settings.tok.validity")}>
                {(id) => (
                  <select id={id} value={days} onChange={(e) => setDays(e.target.value)}>
                    <option value="7">{t("fmt.day", { n: 7 })}</option>
                    <option value="30">{t("fmt.day", { n: 30 })}</option>
                    <option value="90">{t("fmt.day", { n: 90 })}</option>
                    <option value="365">{t("fmt.day", { n: 365 })}</option>
                  </select>
                )}
              </Field>
            </div>
            <div className="row-end">
              <button className="btn btn-primary" disabled={busy || name.trim() === "" || list.length >= (q.data?.max ?? 10)}>
                {busy ? t("settings.creating") : t("settings.tok.create")}
              </button>
            </div>
          </form>
          {list.length === 0 ? (
            <p className="small muted">{t("settings.tok.none")}</p>
          ) : (
            <ul className="list">
              {list.map((tk) => (
                <li key={tk.id} className="list-row">
                  <span>
                    {tk.name} <Chip tone={SCOPE_TONE[tk.scope] ?? "mute"}>{scopeLabel(tk.scope)}</Chip>
                    <br />
                    <span className="small muted">
                      <span className="mono">{tk.prefix}…</span>{t("settings.tok.until", { time: fmtTime(tk.expires) })}{tk.last_used ? `${t("settings.tok.lastUsed", { time: fmtTime(tk.last_used) })}${tk.last_ip ? t("settings.tok.lastFrom", { ip: tk.last_ip }) : ""}` : t("settings.tok.never")}
                    </span>
                  </span>
                  <button className="btn btn-danger-ghost" onClick={() => setRev(tk)}>
                    {t("settings.revoke")}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </State>
      {made && <MadeDialog made={made} onClose={() => setMade(null)} />}
      {rev && (
        <ConfirmDialog
          title={t("settings.tok.revokeTitle")}
          body={<>{t("settings.tok.revokeBody", { name: rev.name })}</>}
          word={rev.name}
          verb={t("settings.revoke")}
          onClose={() => setRev(null)}
          onConfirm={async () => {
            await post("/api/me/tokens/revoke", { id: rev.id });
            setRev(null);
            q.reload();
          }}
        />
      )}
    </Card>
  );
}

function MadeDialog({ made, onClose }: { made: { token: string; scope: string }; onClose: () => void }) {
  const t = useT();
  const [note, setNote] = useState<Note>(null);
  return (
    <Modal title={t("settings.tok.made")} onClose={onClose} wide>
      <div className="stack">
        <Alert tone="warn">{t("settings.tok.madeWarn")}</Alert>
        <div className="row">
          <code className="mono" style={{ wordBreak: "break-all" }}>{made.token}</code>
        </div>
        <p className="small muted">
          {t("settings.tok.m1")}<span className="mono">~/.config/macpanel/token</span>{t("settings.tok.m2")}<span className="mono">panelctl whoami</span>{t("settings.tok.m3", { scope: scopeLabel(made.scope) })}
        </p>
        {note && <Alert tone={note.tone}>{note.text}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={() => copy(made.token, setNote, t("settings.tok.name2"))}>
            {t("settings.tok.copyToken")}
          </button>
          <button className="btn btn-primary" onClick={onClose}>
            {t("settings.savedIt")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

export default function Settings() {
  const t = useT();
  return (
    <>
      <PageHead title={t("nav.settings")} desc={t("settings.page.desc")} />
      <PanelCard />
      <AppearanceCard />
      <NotifyCard />
      <ThresholdsCard />
      <Card title={t("settings.me.title")}><p className="small muted">{t("settings.me.text")}<a href="#/profil">{t("profile.mine")}</a>.</p></Card>
      <Tokens />
    </>
  );
}
