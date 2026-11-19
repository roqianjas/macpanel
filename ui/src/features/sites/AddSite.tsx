import { useState } from "react";
import { errText, post } from "../../api";
import { Alert, Field, Modal } from "../../ui";
import { PHP_VERSIONS } from "./types";
import { useT } from "../../i18n";

export function AddSite({ onClose, onDone }: { onClose: () => void; onDone: (name: string) => void }) {
  const t = useT();
  const [name, setName] = useState("");
  const [dbEngine, setDbEngine] = useState("");
  const [mode, setMode] = useState("php");
  const [target, setTarget] = useState("");
  const [php, setPhp] = useState("8.4");
  const [apache, setApache] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const body: Record<string, string | boolean> = { name, target };
      if (mode === "php") body.php = php;
      if (apache && mode !== "proxy") body.apache = true;
      await post("/api/sites", body);
      if (dbEngine) {
        try {
          await post("/api/db", { name: name.replace(/-/g, "_"), engine: dbEngine });
        } catch (x) {
          setErr(t("dev.site.dbFail", { err: errText(x) }));
          setBusy(false);
          return;
        }
      }
      onDone(name);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={t("dev.site.add")} onClose={onClose}>
      <form onSubmit={submit} className="stack">
        <Field label={t("dev.col.name")} hint={t("dev.site.nameHint", { name: name || t("dev.site.nameWord") })}>
          {(id, d) => <input id={id} aria-describedby={d} data-autofocus placeholder={t("dev.site.namePh")} maxLength={40} value={name} onChange={(e) => setName(e.target.value.toLowerCase())} required pattern="[a-z0-9]([a-z0-9\-]{0,38}[a-z0-9])?" />}
        </Field>
        <Field label={t("dev.site.kind")}>
          {(id) => (
            <select id={id} value={mode} onChange={(e) => { setMode(e.target.value); setTarget(""); if (e.target.value === "proxy") setApache(false); }}>
              <option value="php">{t("dev.site.opt.php")}</option>
              <option value="statis">{t("dev.site.opt.static")}</option>
              <option value="proxy">{t("dev.site.opt.proxy")}</option>
            </select>
          )}
        </Field>
        <Field label={mode === "proxy" ? t("dev.site.appPort") : t("dev.site.folder")} hint={mode === "proxy" ? t("dev.site.portHint") : t("dev.site.pathHint")}>
          {(id, d) => <input id={id} aria-describedby={d} className="mono" value={target} onChange={(e) => setTarget(e.target.value)} required placeholder={mode === "proxy" ? "3000" : t("dev.site.folderPh")} />}
        </Field>
        {mode !== "proxy" && (
          <label className="check">
            <input type="checkbox" checked={apache} onChange={(e) => setApache(e.target.checked)} />
            {t("dev.site.apache")}
          </label>
        )}
        {mode === "php" && (
          <Field label={t("dev.site.phpVer")}>
            {(id) => (
              <select id={id} value={php} onChange={(e) => setPhp(e.target.value)}>
                {PHP_VERSIONS.map((v) => <option key={v}>{v}</option>)}
              </select>
            )}
          </Field>
        )}
        {mode !== "statis" && (
          <Field label={t("dev.site.dbOpt")} hint={dbEngine ? t("dev.site.dbHintOn", { name: name.replace(/-/g, "_") || t("dev.site.nameWord") }) : t("dev.site.dbHintOff")}>
            {(id, dd) => (
              <select id={id} aria-describedby={dd} value={dbEngine} onChange={(e) => setDbEngine(e.target.value)}>
                <option value="">{t("dev.site.noDb")}</option>
                <option value="mariadb">MariaDB (3306)</option>
                <option value="mysql">MySQL (3307)</option>
                <option value="pgsql">PostgreSQL</option>
              </select>
            )}
          </Field>
        )}
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn btn-primary" disabled={busy || !name || !target}>{busy ? t("dev.task.creating") : t("dev.site.create")}</button>
        </div>
      </form>
    </Modal>
  );
}
