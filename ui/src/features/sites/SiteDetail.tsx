import { useState } from "react";
import { ExternalLink, Trash2 } from "lucide-react";
import { CloneModal } from "../../pages/Installer";
import { get, post } from "../../api";
import Deploy from "../../pages/Deploy";
import { Alert, Card, ConfirmDialog, PageHead, State, Tabs, useLoad } from "../../ui";
import { Model } from "./types";
import { SettingsForm } from "./SettingsForm";
import { Ssl } from "./Ssl";
import { PublicAccess } from "./Public";
import { ConfigEditor } from "./ConfigEditor";
import { Logs } from "./Logs";
import { AuthUsers } from "./AuthUsers";
import { EnvEditor } from "./EnvEditor";
import { Traffic } from "./Traffic";
import { t, useT } from "../../i18n";

export const tabs = () => [
  { id: "umum", label: t("dev.sd.tab.general") },
  { id: "domain", label: t("dev.sd.tab.domain") },
  { id: "akses", label: t("dev.sd.tab.access") },
  { id: "env", label: ".env" },
  { id: "trafik", label: t("dev.sd.tab.traffic") },
  { id: "deploy", label: "Deploy" },
  { id: "rewrite", label: "Rewrite" },
  { id: "ssl", label: "SSL" },
  { id: "publik", label: t("pub.tab") },
  { id: "config", label: t("dev.conf.title") },
  { id: "log", label: t("apps.log") },
];

export function SiteDetail({ name }: { name: string }) {
  const t = useT();
  const [tab, setTab] = useState("umum");
  const settings = useLoad(() => get<{ kind: string; model: Model | null; system: boolean }>(`/api/sites/${name}/settings`), [name]);
  const [removing, setRemoving] = useState(false);
  const [cloning, setCloning] = useState(false);
  const host = `${name}.lab`;
  const system = settings.data?.system ?? false;
  const missing = settings.error.includes("tidak ada");
  return (
    <>
      <PageHead
        title={host}
        desc={system ? t("dev.sd.systemDesc") : missing ? t("dev.sd.missingDesc") : t("dev.sd.desc")}
        action={
          <>
            <a className="btn" href="#/website">{t("dev.sd.allSites")}</a>
            {!system && !missing && <button className="btn" onClick={() => setCloning(true)}>{t("dev.sd.clone")}</button>}
            <a className="btn" href={`https://${host}`} target="_blank" rel="noopener noreferrer">
              <ExternalLink size={18} aria-hidden="true" />
              {t("dev.sd.open")}<span className="sr"> {t("dev.sd.newTab")}</span>
            </a>
          </>
        }
      />
      {cloning && <CloneModal src={name} onClose={() => setCloning(false)} onDone={(n) => { setCloning(false); location.hash = `#/website/${n}`; }} />}
      {!missing && <Tabs tabs={system ? tabs().filter((x) => x.id !== "deploy" && x.id !== "publik") : tabs()} value={tab} onChange={setTab} label={t("dev.sd.sections")} />}
      {!missing && (
        <div role="tabpanel" aria-labelledby={`tab-${tab}`} className="tabpanel">
          {tab === "umum" || tab === "rewrite" || tab === "domain" || tab === "akses" ? (
            <State loading={settings.loading && !settings.data} error={settings.error} onRetry={settings.reload}>
              {settings.data &&
                (settings.data.model ? (
                  <>
                    <SettingsForm key={tab} name={name} kind={settings.data.kind} initial={settings.data.model} system={system} section={tab} onSaved={settings.reload} />
                    {tab === "akses" && !system && <AuthUsers name={name} enabled={!!settings.data.model.auth} />}
                  </>
                ) : (
                  <Card><p>{t("dev.sd.customNote1")} <strong>{t("dev.conf.title")}</strong>{t("dev.sd.customNote2")}</p></Card>
                ))}
            </State>
          ) : tab === "trafik" ? (
            <Traffic name={name} />
          ) : tab === "env" ? (
            <EnvEditor name={name} />
          ) : tab === "deploy" ? (
            <Deploy name={name} />
          ) : tab === "ssl" ? (
            <Ssl name={name} />
          ) : tab === "publik" ? (
            <PublicAccess name={name} />
          ) : tab === "config" ? (
            <ConfigEditor name={name} system={system} onSaved={settings.reload} />
          ) : (
            <Logs name={name} />
          )}
        </div>
      )}
      {missing && <Alert tone="warn">{settings.error}</Alert>}
      {!system && !missing && tab === "umum" && (
        <Card title={t("dev.sd.rmSite")} className="danger-zone">
          <p className="muted small">{t("dev.sd.rmNote")}</p>
          <div>
            <button className="btn btn-danger" onClick={() => setRemoving(true)}>
              <Trash2 size={18} aria-hidden="true" />
              {t("dev.sd.rmHost", { host })}
            </button>
          </div>
        </Card>
      )}
      {removing && (
        <ConfirmDialog
          title={t("dev.sd.rmHost", { host })}
          word={name}
          verb={t("dev.sd.rmSite")}
          body={<p>{t("dev.sd.rmBody")}</p>}
          onConfirm={async () => {
            await post(`/api/sites/${name}/remove`, { confirm: name });
            location.hash = "#/website";
          }}
          onClose={() => setRemoving(false)}
        />
      )}
    </>
  );
}
