import { normalizeHosts } from "./hosts";
import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { errText, post } from "../../api";
import { Alert, Card, Chip, Field } from "../../ui";
import { Rewrite, Model, PHP_VERSIONS } from "./types";
import { useT } from "../../i18n";

export function SettingsForm({ name, kind, initial, system, section, onSaved }: { name: string; kind: string; initial: Model; system: boolean; section: string; onSaved: () => void }) {
  const t = useT();
  const [m, setM] = useState<Model>({ ...initial, rewrites: initial.rewrites ?? [] });
  const [replace, setReplace] = useState(false);
  const [busy, setBusy] = useState(false);
  const [aliasText, setAliasText] = useState((initial.aliases ?? []).join("\n"));
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const set = <K extends keyof Model>(k: K, v: Model[K]) => setM((p) => ({ ...p, [k]: v }));
  const presets = m.mode === "php" ? ["laravel", "wordpress"] : m.mode === "statis" ? ["static", "spa"] : [];
  const kindLabel: Record<string, string> = { template: t("dev.set.kind.template"), legacy: t("dev.set.kind.legacy"), custom: t("dev.set.kind.custom") };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      const body: Record<string, unknown> = { ...m, aliases: normalizeHosts((m.aliases ?? []).join("\n")), max_body_mb: Number(m.max_body_mb), read_timeout_s: Number(m.read_timeout_s) };
      if (replace) body.replace = true;
      await post(`/api/sites/${name}/settings`, body);
      setMsg({ tone: "ok", text: t("dev.set.saved") });
      setTimeout(onSaved, 700);
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const addRule = () => set("rewrites", [...m.rewrites, { from: "", to: "", flag: "" }]);
  const editRule = (i: number, k: keyof Rewrite, v: string) => set("rewrites", m.rewrites.map((r, j) => (j === i ? { ...r, [k]: v } : r)));
  const dropRule = (i: number) => set("rewrites", m.rewrites.filter((_, j) => j !== i));

  return (
    <form onSubmit={save} className="stack-l">
      <p className="small muted">
        {t("dev.set.status")}: <Chip tone={kind === "custom" ? "warn" : "info"}>{kindLabel[kind] ?? kind}</Chip>
        {kind === "legacy" && ` ${t("dev.set.legacyNote")}`}
      </p>
      {kind === "custom" && !system && (
        <label className="check">
          <input type="checkbox" checked={replace} onChange={(e) => setReplace(e.target.checked)} />
          {t("dev.set.override")}
        </label>
      )}
      <fieldset className="fs" disabled={system || busy}>
        {section === "domain" ? (
          <Card title={t("dev.set.domains")}>
            <p className="small muted">
              {t("dev.set.dom1")} <span className="mono">www.lab</span>. {t("dev.set.dom2")} <span className="mono">{t("dev.set.domForm")}</span> {t("dev.set.dom3")}
            </p>
            <Field label={t("dev.set.domains")} hint={t("dev.set.mainName", { name })}>
              {(id, dd) => (
                <textarea id={id} aria-describedby={dd} className="mono" rows={4} placeholder={t("dev.set.domPh")} value={aliasText} onChange={(e) => { setAliasText(e.target.value); set("aliases", e.target.value.split("\n").map((x) => x.trim().toLowerCase()).filter(Boolean)); }} onBlur={(e) => { const h = normalizeHosts(e.target.value); setAliasText(h.join("\n")); set("aliases", h); }} spellCheck={false} />
              )}
            </Field>
            <p className="small muted">{t("dev.set.domPublicNote")}</p>
            <p className="small muted">{t("dev.set.domTunnelNote")}</p>
            {(m.aliases ?? []).some((x) => !x.endsWith(".lab")) && (
              <>
                <div className="alert alert-warn" role="alert">{t("dev.set.domPublicWarn")}</div>
                <label className="check">
                  <input type="checkbox" checked={!!m.public_ack} onChange={(e) => set("public_ack", e.target.checked)} />
                  {t("dev.set.domPublicAck")}
                </label>
              </>
            )}
          </Card>
        ) : section === "akses" ? (
          <Card title={t("dev.set.access")}>
            <p className="small muted">{t("dev.set.accessNote")}</p>
            <Field label={t("dev.set.allowIps")} hint={t("dev.set.allowIpsHint")}>
              {(id, dd) => (
                <textarea id={id} aria-describedby={dd} className="mono" rows={3} placeholder="100.64.0.10" value={(m.allow_ips ?? []).join("\n")} onChange={(e) => set("allow_ips", e.target.value.split("\n").map((x) => x.trim()).filter(Boolean))} spellCheck={false} />
              )}
            </Field>
            <div className="checks">
              <label className="check">
                <input type="checkbox" checked={!!m.auth} onChange={(e) => set("auth", e.target.checked)} />
                {t("dev.set.basicAuth")}
              </label>
            </div>
          </Card>
        ) : section === "umum" ? (
          <Card title={t("dev.set.general")}>
            <div className="grid2">
              <Field label={t("dev.site.kind")}>
                {(id) => (
                  <select id={id} value={m.mode} onChange={(e) => setM({ ...m, mode: e.target.value, preset: e.target.value === "php" ? "laravel" : e.target.value === "statis" ? "static" : "", php: e.target.value === "php" ? m.php || "8.4" : "" })}>
                    <option value="php">PHP</option>
                    <option value="statis">{t("dev.site.mode.statis")}</option>
                    <option value="proxy">Proxy</option>
                  </select>
                )}
              </Field>
              <Field label={m.mode === "proxy" ? t("dev.site.appPort") : t("dev.site.folder")}>
                {(id) => <input id={id} className="mono" placeholder={m.mode === "proxy" ? "3000" : t("dev.set.folderPh")} pattern={m.mode === "proxy" ? "[0-9]{4,5}" : undefined} title={m.mode === "proxy" ? t("dev.set.portTitle") : undefined} value={m.target} onChange={(e) => set("target", e.target.value)} required />}
              </Field>
              {m.mode === "php" && (
                <Field label={t("dev.site.phpVer")} hint={t("dev.set.phpHint")}>
                  {(id, d) => (
                    <select id={id} aria-describedby={d} value={m.php} onChange={(e) => set("php", e.target.value)}>
                      {PHP_VERSIONS.map((v) => <option key={v}>{v}</option>)}
                    </select>
                  )}
                </Field>
              )}
              {presets.length > 0 && (
                <Field label={t("dev.set.preset")} hint={m.mode === "php" ? t("dev.set.presetPhp") : t("dev.set.presetStatic")}>
                  {(id, d) => (
                    <select id={id} aria-describedby={d} value={m.preset} onChange={(e) => set("preset", e.target.value)}>
                      {presets.map((p) => <option key={p}>{p}</option>)}
                    </select>
                  )}
                </Field>
              )}
              <Field label={t("dev.set.upload")}>{(id) => <input id={id} type="number" min={1} max={1024} placeholder="50" value={m.max_body_mb} onChange={(e) => set("max_body_mb", Number(e.target.value))} required />}</Field>
              <Field label={t("dev.set.timeout")}>{(id) => <input id={id} type="number" min={10} max={600} placeholder="60" value={m.read_timeout_s} onChange={(e) => set("read_timeout_s", Number(e.target.value))} required />}</Field>
            </div>
            <div className="checks">
              <label className="check"><input type="checkbox" checked={m.force_https} onChange={(e) => set("force_https", e.target.checked)} />{t("dev.set.forceHttps")}</label>
              <label className="check"><input type="checkbox" checked={m.gzip} onChange={(e) => set("gzip", e.target.checked)} />{t("dev.set.gzip")}</label>
            </div>
          </Card>
        ) : (
          <Card
            title={t("dev.set.rewrite")}
            action={
              <button type="button" className="btn" onClick={addRule} disabled={m.rewrites.length >= 50}>
                <Plus size={18} aria-hidden="true" />
                {t("dev.set.addRule")}
              </button>
            }
          >
            <p className="small muted">{t("dev.set.rw1")} <span className="mono">^/lama/(.*)$</span> {t("dev.set.rw2")} <span className="mono">/baru/$1</span> {t("dev.set.rw3")}</p>
            {m.rewrites.length === 0 ? (
              <div className="state"><p className="muted">{t("dev.set.noRules")}</p></div>
            ) : (
              <div className="tbl-wrap">
                <table className="tbl">
                  <thead>
                    <tr>
                      <th scope="col">{t("dev.set.fromRegex")}</th>
                      <th scope="col">{t("dev.set.to")}</th>
                      <th scope="col">Flag</th>
                      <th scope="col"><span className="sr">{t("apps.remove")}</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    {m.rewrites.map((r, i) => (
                      <tr key={i}>
                        <td><input aria-label={t("dev.set.ruleFrom", { n: i + 1 })} className="mono" placeholder="^/lama/(.*)$" maxLength={200} value={r.from} onChange={(e) => editRule(i, "from", e.target.value)} required /></td>
                        <td><input aria-label={t("dev.set.ruleTo", { n: i + 1 })} className="mono" placeholder="/baru/$1" maxLength={200} value={r.to} onChange={(e) => editRule(i, "to", e.target.value)} required /></td>
                        <td>
                          <select aria-label={t("dev.set.ruleFlag", { n: i + 1 })} value={r.flag} onChange={(e) => editRule(i, "flag", e.target.value)}>
                            <option value="">{t("dev.set.noFlag")}</option>
                            {["last", "break", "redirect", "permanent"].map((f) => <option key={f}>{f}</option>)}
                          </select>
                        </td>
                        <td className="cell-end">
                          <button type="button" className="btn btn-ghost btn-icon" onClick={() => dropRule(i)} aria-label={t("dev.set.rmRule", { n: i + 1 })}>
                            <Trash2 size={18} aria-hidden="true" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        )}
      </fieldset>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      {!system && (
        <div>
          <button className="btn btn-primary" disabled={busy || (kind === "custom" && !replace)}>{busy ? t("dev.php.saving") : t("dev.conf.saveApply")}</button>
        </div>
      )}
    </form>
  );
}
