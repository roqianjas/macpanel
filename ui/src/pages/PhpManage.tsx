import { useState } from "react";
import { Download, RefreshCw, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Chip, ConfirmDialog, Field, Modal, State, Tabs, useLoad } from "../ui";
import { t, useT } from "../i18n";

interface Ext {
  name: string;
  label: string;
  installed: boolean;
  enabled: boolean;
  loaded: boolean;
  zend: boolean;
}
interface IniKey {
  group?: string;
  key: string;
  value: string;
  default: string;
  hint: string;
}

const tabs = () => [
  { id: "ext", label: t("dev.php.tab.ext") },
  { id: "ini", label: t("dev.php.tab.ini") },
  { id: "fpm", label: t("dev.php.tab.fpm") },
];

export default function PhpManage({ version, onClose }: { version: string; onClose: () => void }) {
  const t = useT();
  const [tab, setTab] = useState("ext");
  return (
    <Modal title={t("dev.php.manage", { v: version })} onClose={onClose} wide>
      <Tabs tabs={tabs()} value={tab} onChange={setTab} label={t("dev.php.sections")} />
      <div role="tabpanel" aria-labelledby={`tab-${tab}`} className="tabpanel">
        {tab === "ext" ? <Extensions v={version} /> : tab === "ini" ? <Settings v={version} /> : <Fpm v={version} />}
      </div>
    </Modal>
  );
}

function Switch({ on, disabled, label, onChange }: { on: boolean; disabled?: boolean; label: string; onChange?: () => void }) {
  return (
    <button type="button" role="switch" aria-checked={on} aria-label={label} disabled={disabled} className={`switch ${on ? "on" : ""}`} onClick={onChange}>
      <span className="switch-knob" />
    </button>
  );
}

type Filter = "all" | "toggle" | "builtin";

function Extensions({ v }: { v: string }) {
  const t = useT();
  const q = useLoad(() => get<{ builtin: string[]; managed: Ext[] }>(`/api/php/${v}/ext`), [v]);
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const [err, setErr] = useState("");
  const [rm, setRm] = useState<Ext | null>(null);
  const [filter, setFilter] = useState<Filter>("all");
  const [term, setTerm] = useState("");
  const act = async (e: Ext, a: "install" | "remove" | "enable" | "disable") => {
    setErr("");
    setPending((p) => ({ ...p, [e.name]: true }));
    try {
      const r = await post<{ job: number }>(`/api/php/${v}/ext/${e.name}/${a}`, {});
      trackJob(r.job, t(`dev.php.extJob.${a}`, { label: e.label, v }), (j) => {
        setPending((p) => ({ ...p, [e.name]: false }));
        if (j.status !== "ok") setErr(j.output || t("dev.failed"));
        q.reload();
      });
    } catch (x) {
      setPending((p) => ({ ...p, [e.name]: false }));
      setErr(errText(x));
    }
  };
  const managed = (q.data?.managed ?? []).filter((m) => m.name);
  const builtin = q.data?.builtin ?? [];
  const q2 = term.trim().toLowerCase();
  const mShow = filter === "builtin" ? [] : managed.filter((m) => !q2 || m.name.includes(q2) || m.label.toLowerCase().includes(q2));
  const bShow = filter === "toggle" ? [] : builtin.filter((b) => !q2 || b.toLowerCase().includes(q2));
  const FILTERS: { id: Filter; label: string }[] = [
    { id: "all", label: t("dev.php.f.all", { n: managed.length + builtin.length }) },
    { id: "toggle", label: t("dev.php.f.toggle", { n: managed.length }) },
    { id: "builtin", label: t("dev.php.f.builtin", { n: builtin.length }) },
  ];
  return (
    <div className="stack">
      <div className="ext-bar">
        <label className="sr" htmlFor="ext-q">{t("dev.php.searchExt")}</label>
        <input id="ext-q" type="search" placeholder={t("dev.php.searchExtPh")} value={term} onChange={(e) => setTerm(e.target.value)} autoComplete="off" />
        <div className="seg" role="group" aria-label={t("dev.php.filterExt")}>
          {FILTERS.map((f) => (
            <button key={f.id} type="button" className={`seg-btn ${filter === f.id ? "on" : ""}`} aria-pressed={filter === f.id} onClick={() => setFilter(f.id)}>
              {f.label}
            </button>
          ))}
        </div>
      </div>
      {err && <Alert>{err}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && mShow.length + bShow.length === 0} emptyText={t("dev.php.noExt")} onRetry={q.reload}>
        <div className="ext-list" role="list">
          {mShow.map((e) => (
            <div key={e.name} role="listitem" className="ext-row">
              <div className="ext-name">
                <strong className="mono">{e.name}</strong>
                <span className="small muted">{e.label}</span>
              </div>
              <Chip tone="info">{t("dev.php.extra")}</Chip>
              <span className="small muted ext-state">{!e.installed ? t("svc.state.not_installed") : e.loaded ? t("dev.php.active") : t("svc.state.off")}</span>
              <div className="ext-act">
                {e.installed ? (
                  <>
                    <Switch on={e.loaded} disabled={pending[e.name]} label={t(e.loaded ? "dev.php.disable" : "dev.php.enable", { name: e.name })} onChange={() => act(e, e.loaded ? "disable" : "enable")} />
                    <button className="btn btn-icon btn-danger-ghost" disabled={pending[e.name]} onClick={() => setRm(e)} aria-label={t("dev.docker.rmAria", { name: e.name })} title={t("dev.php.rmExt")}>
                      <Trash2 size={18} aria-hidden="true" />
                    </button>
                  </>
                ) : (
                  <button className="btn btn-sm" disabled={pending[e.name]} onClick={() => act(e, "install")}>
                    <Download size={16} aria-hidden="true" />
                    {pending[e.name] ? t("dev.php.installing") : t("apps.install")}
                    <span className="sr"> {e.name}</span>
                  </button>
                )}
              </div>
            </div>
          ))}
          {bShow.map((b) => (
            <div key={b} role="listitem" className="ext-row">
              <div className="ext-name">
                <strong className="mono">{b}</strong>
              </div>
              <Chip tone="mute">{t("dev.php.builtin")}</Chip>
              <span className="small muted ext-state">{t("dev.php.permanent")}</span>
              <div className="ext-act">
                <Switch on disabled label={t("dev.php.builtinAria", { name: b })} />
              </div>
            </div>
          ))}
        </div>
        <p className="small muted">{t("dev.php.note1")} <strong>{t("dev.php.builtin")}</strong> {t("dev.php.note2")} <strong>{t("dev.php.extra")}</strong> {t("dev.php.note3")}</p>
      </State>
      {rm && (
        <ConfirmDialog
          title={t("apps.confirm.remove", { name: rm.label })}
          word={rm.name}
          verb={t("dev.php.rmExt")}
          body={<p>{t("dev.php.rmBody", { v })}</p>}
          onConfirm={async () => act(rm, "remove")}
          onClose={() => setRm(null)}
        />
      )}
    </div>
  );
}

function Settings({ v }: { v: string }) {
  const t = useT();
  const q = useLoad(() => get<{ keys: IniKey[] }>(`/api/php/${v}/ini`), [v]);
  const [edit, setEdit] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const changed = (q.data?.keys ?? []).filter((k) => edit[k.key] !== undefined && edit[k.key].trim() !== "" && edit[k.key].trim() !== k.value);
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const values: Record<string, string> = {};
      changed.forEach((k) => (values[k.key] = edit[k.key].trim()));
      const r = await post<{ job: number }>(`/api/php/${v}/ini`, { values });
      trackJob(r.job, t("dev.php.saveJob", { v }), (j) => {
        setBusy(false);
        setNote(j.status === "ok" ? { tone: "ok", text: t("dev.php.saved") } : { tone: "bad", text: j.output || t("dev.failed") });
        setEdit({});
        q.reload();
      });
    } catch (x) {
      setBusy(false);
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <form onSubmit={save} className="stack">
      <p className="small muted">{t("dev.php.iniNote", { v })}</p>
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {[
          { g: "php", t: t("dev.php.sec.limits") },
          { g: "fpm", t: t("dev.php.sec.pool") },
        ].map((sec) => (
          <section key={sec.g} className="stack" aria-label={sec.t}>
            <h3 className="sec-title">{sec.t}</h3>
            <div className="grid-form">
              {q.data?.keys
                .filter((k) => (k.group ?? "php") === sec.g)
                .map((k) => (
                  <Field key={k.key} label={k.key} hint={t("dev.php.iniHint", { hint: k.hint, cur: k.value || `${t("dev.php.default")}${k.default ? ` (${k.default})` : ""}` })}>
                    {(id, dd) => <input id={id} aria-describedby={dd} className="mono" value={edit[k.key] ?? ""} placeholder={t("dev.php.unchanged")} onChange={(e) => setEdit((p) => ({ ...p, [k.key]: e.target.value }))} spellCheck={false} />}
                  </Field>
                ))}
            </div>
          </section>
        ))}
        <div className="row-end">
          <button className="btn btn-primary" disabled={busy || changed.length === 0}>
            {busy ? t("dev.php.saving") : `${t("dev.php.saveRestart")}${changed.length ? ` (${changed.length})` : ""}`}
          </button>
        </div>
      </State>
    </form>
  );
}

function Fpm({ v }: { v: string }) {
  const t = useT();
  const log = useLoad(() => get<{ log: string }>(`/api/services/php-${v}/logs?n=80`), [v]);
  const [restart, setRestart] = useState(false);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  return (
    <div className="stack">
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <div className="row">
        <button className="btn" onClick={() => setRestart(true)}>
          <RefreshCw size={18} aria-hidden="true" />
          {t("dev.php.restartFpm", { v })}
        </button>
        <button className="btn" onClick={log.reload}>{t("dev.php.reloadLog")}</button>
      </div>
      <State loading={log.loading && !log.data} error={log.error} onRetry={log.reload}>
        <pre className="logbox" tabIndex={0} aria-label={t("dev.php.fpmLog", { v })}>{log.data?.log || t("dev.php.logEmpty")}</pre>
      </State>
      {restart && (
        <ConfirmDialog
          title={t("dev.php.restartFpm", { v })}
          word={`php-${v}`}
          verb={t("apps.restart")}
          body={<p>{t("dev.php.restartBody", { v })}</p>}
          onConfirm={async () => {
            await post(`/api/services/php-${v}/restart`, {});
            setNote({ tone: "ok", text: t("dev.php.restarted", { v }) });
            log.reload();
          }}
          onClose={() => setRestart(false)}
        />
      )}
    </div>
  );
}
