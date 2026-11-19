import { useState } from "react";
import { errText, get, post } from "../api";
import type { PostUrl } from "../api.gen";
import { trackJob } from "../jobs";
import { Alert, Field, Modal, useLoad } from "../ui";
import { useT } from "../i18n";

const NAME = /^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$/;
const APP_NAME = /^[a-z][a-z0-9-]{0,28}[a-z0-9]$/;
type Kind = "laravel" | "wordpress" | "go" | "python" | "nextjs";
const KINDS: { id: Kind; label: string; tool?: string }[] = [
  { id: "laravel", label: "Laravel" },
  { id: "wordpress", label: "WordPress" },
  { id: "go", label: "Go", tool: "go" },
  { id: "python", label: "Python", tool: "python" },
  { id: "nextjs", label: "Next.js", tool: "node" },
];
const APP_HINT: Record<string, string> = {
  go: "ops.in.hint.go",
  python: "ops.in.hint.python",
  nextjs: "ops.in.hint.nextjs",
};

function useRun(onDone: (name: string) => void) {
  const t = useT();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const run = async (path: PostUrl, body: object, label: string, name: string) => {
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ job: number }>(path, body);
      trackJob(r.job, label, (j) => {
        setBusy(false);
        if (j.status === "ok") onDone(name);
        else setErr(j.output || t("ops.failed"));
      });
    } catch (x) {
      setBusy(false);
      setErr(errText(x));
    }
  };
  return { busy, err, run };
}

export function InstallModal({ onClose, onDone }: { onClose: () => void; onDone: (name: string) => void }) {
  const t = useT();
  const [kind, setKind] = useState<Kind>("laravel");
  const [runtime, setRuntime] = useState("");
  const [port, setPort] = useState("");
  const rts = useLoad(() => get<{ runtimes: { tool: string; version: string; default: boolean; pruned: boolean }[] }>("/api/runtime"));
  const [name, setName] = useState("");
  const [php, setPhp] = useState("8.4");
  const [db, setDb] = useState("mariadb");
  const { busy, err, run } = useRun(onDone);
  const meta = KINDS.find((k) => k.id === kind)!;
  const isApp = !!meta.tool;
  const versions = (rts.data?.runtimes ?? []).filter((r) => r.tool === meta.tool && !r.pruned);
  const portNum = port === "" ? 0 : Number(port);
  const ok = (isApp ? APP_NAME.test(name) : NAME.test(name)) && (!isApp || port === "" || (Number.isInteger(portNum) && portNum >= 1024 && portNum <= 65535));
  return (
    <Modal title={t("ops.in.title")} onClose={onClose}>
      <form className="stack" onSubmit={(e) => { e.preventDefault(); if (ok) run("/api/installer", isApp ? { kind, name, ...(runtime ? { runtime } : {}), ...(portNum ? { port: portNum } : {}) } : { kind, name, php, db }, t("ops.in.job", { kind, name }), name); }}>
        <p className="small muted">{isApp ? t(APP_HINT[kind]) : <>{t("ops.in.A")} <span className="mono">{t("ops.in.dirPath")}</span>{t("ops.in.B")} <span className="mono">{t("ops.in.siteHost")}</span>{t("ops.in.C")}</>} {t("ops.in.D")}</p>
        <div className="seg" role="group" aria-label={t("ops.in.kindAria")} style={{ flexWrap: "wrap" }}>
          {KINDS.map((k) => (
            <button key={k.id} type="button" className={`seg-btn ${kind === k.id ? "on" : ""}`} aria-pressed={kind === k.id} onClick={() => { setKind(k.id); setRuntime(""); }}>{k.label}</button>
          ))}
        </div>
        <Field label={t("ops.in.siteName")} hint={t("ops.in.siteNameHint")}>{(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder={t("ops.in.siteNamePh")} pattern="[a-z0-9]([a-z0-9\-]{0,28}[a-z0-9])?" title={t("ops.in.siteNameTitle")} value={name} onChange={(e) => setName(e.target.value.toLowerCase())} maxLength={30} required autoComplete="off" spellCheck={false} />}</Field>
        {isApp ? (
          <>
            <Field label={t("ops.in.version", { tool: meta.id === "nextjs" ? "Node" : meta.label })} hint={t("ops.in.versionHint")}>
              {(id, dd) => (
                <select id={id} aria-describedby={dd} value={runtime} onChange={(e) => setRuntime(e.target.value)}>
                  <option value="">{t("ops.in.default")}</option>
                  {versions.map((v) => <option key={v.version} value={v.version}>{v.version}</option>)}
                </select>
              )}
            </Field>
            <Field label={t("ops.in.port")} hint={t("ops.in.portHint")}>
              {(id, dd) => <input id={id} aria-describedby={dd} className="mono" inputMode="numeric" placeholder={t("ops.in.portPh")} pattern="[0-9]{4,5}" title={t("ops.in.portTitle")} value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))} maxLength={5} />}
            </Field>
          </>
        ) : (
          <>
        <Field label={t("ops.in.database")}>{(id) => <select id={id} value={db} onChange={(e) => setDb(e.target.value)}><option value="mariadb">{t("ops.in.mariadb")}</option><option value="mysql">{t("ops.in.mysql")}</option></select>}</Field>
        <Field label={t("ops.in.phpVersion")}>{(id) => <select id={id} value={php} onChange={(e) => setPhp(e.target.value)}>{["8.2", "8.3", "8.4", "8.5"].map((v) => <option key={v}>{v}</option>)}</select>}</Field>
          </>
        )}
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn btn-primary" disabled={busy || !ok}>{busy ? t("ops.in.installing") : t("apps.install")}</button>
        </div>
      </form>
    </Modal>
  );
}

export function CloneModal({ src, onClose, onDone }: { src: string; onClose: () => void; onDone: (name: string) => void }) {
  const t = useT();
  const [dst, setDst] = useState(`${src}-staging`.slice(0, 30));
  const [withDb, setWithDb] = useState(true);
  const { busy, err, run } = useRun(onDone);
  const ok = NAME.test(dst) && dst !== src;
  return (
    <Modal title={t("ops.in.cloneTitle", { src })} onClose={onClose}>
      <form className="stack" onSubmit={(e) => { e.preventDefault(); if (ok) run(`/api/sites/${src}/clone`, { dst, with_db: withDb }, t("ops.in.cloneJob", { src, dst }), dst); }}>
        <p className="small muted">{t("ops.in.cloneA")} <span className="mono">.git</span>, <span className="mono">node_modules</span>{t("ops.in.cloneB")} <span className="mono">{t("ops.in.dirPath")}</span>{t("ops.in.cloneC")} <span className="mono">.env</span> {t("ops.in.cloneD")}</p>
        <Field label={t("ops.in.newName")}>{(id) => <input id={id} className="mono" placeholder={t("ops.in.newNamePh")} pattern="[a-z0-9]([a-z0-9\-]{0,28}[a-z0-9])?" title={t("ops.in.newNameTitle")} value={dst} onChange={(e) => setDst(e.target.value.toLowerCase())} maxLength={30} required autoComplete="off" spellCheck={false} />}</Field>
        <div className="checks">
          <label className="check"><input type="checkbox" checked={withDb} onChange={(e) => setWithDb(e.target.checked)} />{t("ops.in.withDb")}</label>
        </div>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn btn-primary" disabled={busy || !ok}>{busy ? t("ops.in.cloning") : t("ops.in.clone")}</button>
        </div>
      </form>
    </Modal>
  );
}
