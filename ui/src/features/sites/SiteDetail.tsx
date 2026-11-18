import { useState } from "react";
import { ExternalLink, Trash2 } from "lucide-react";
import { CloneModal } from "../../pages/Installer";
import { get, post } from "../../api";
import Deploy from "../../pages/Deploy";
import { Alert, Card, ConfirmDialog, PageHead, State, Tabs, useLoad } from "../../ui";
import { Model } from "./types";
import { SettingsForm } from "./SettingsForm";
import { Ssl } from "./Ssl";
import { ConfigEditor } from "./ConfigEditor";
import { Logs } from "./Logs";
import { AuthUsers } from "./AuthUsers";
import { EnvEditor } from "./EnvEditor";
import { Traffic } from "./Traffic";

export const TABS = [
  { id: "umum", label: "Umum" },
  { id: "domain", label: "Domain" },
  { id: "akses", label: "Akses" },
  { id: "env", label: ".env" },
  { id: "trafik", label: "Trafik" },
  { id: "deploy", label: "Deploy" },
  { id: "rewrite", label: "Rewrite" },
  { id: "ssl", label: "SSL" },
  { id: "config", label: "Config nginx" },
  { id: "log", label: "Log" },
];

export function SiteDetail({ name }: { name: string }) {
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
        desc={system ? "Situs sistem: hanya bisa dilihat dari panel." : missing ? "Situs ini tidak aktif (dijeda) atau tidak ada. Aktifkan dari daftar Website untuk mengelolanya." : "Atur situs, rewrite, sertifikat, dan config nginx."}
        action={
          <>
            <a className="btn" href="#/website">Semua situs</a>
            {!system && !missing && <button className="btn" onClick={() => setCloning(true)}>Kloning</button>}
            <a className="btn" href={`https://${host}`} target="_blank" rel="noopener noreferrer">
              <ExternalLink size={18} aria-hidden="true" />
              Buka situs<span className="sr"> (tab baru)</span>
            </a>
          </>
        }
      />
      {cloning && <CloneModal src={name} onClose={() => setCloning(false)} onDone={(n) => { setCloning(false); location.hash = `#/website/${n}`; }} />}
      {!missing && <Tabs tabs={system ? TABS.filter((t) => t.id !== "deploy") : TABS} value={tab} onChange={setTab} label="Bagian situs" />}
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
                  <Card><p>Situs ini memakai config kustom yang tidak bisa diubah lewat formulir. Gunakan tab <strong>Config nginx</strong>.</p></Card>
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
          ) : tab === "config" ? (
            <ConfigEditor name={name} system={system} onSaved={settings.reload} />
          ) : (
            <Logs name={name} />
          )}
        </div>
      )}
      {missing && <Alert tone="warn">{settings.error}</Alert>}
      {!system && !missing && tab === "umum" && (
        <Card title="Hapus situs" className="danger-zone">
          <p className="muted small">Menghapus config dan sertifikat situs ini. Folder proyek tidak disentuh.</p>
          <div>
            <button className="btn btn-danger" onClick={() => setRemoving(true)}>
              <Trash2 size={18} aria-hidden="true" />
              Hapus {host}
            </button>
          </div>
        </Card>
      )}
      {removing && (
        <ConfirmDialog
          title={`Hapus ${host}`}
          word={name}
          verb="Hapus situs"
          body={<p>Situs akan berhenti dilayani. Aksi ini meminta kata sandi ulang.</p>}
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
