import { useState } from "react";
import { Pause, Play, Plus } from "lucide-react";
import { InstallModal } from "../../pages/Installer";
import { errText, get, post } from "../../api";
import { FilterTabs, ListSearch, Pager, useListView } from "../list/ListTools";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, useLoad } from "../../ui";
import { Site, MODE_LABEL, health } from "./types";
import { SITE_FACETS } from "./facets";
import { AddSite } from "./AddSite";

export function SiteList() {
  const { data, error, loading, reload } = useLoad(() => get<{ sites: Site[] }>("/api/sites"));
  const [adding, setAdding] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [pause, setPause] = useState<Site | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const sites = data?.sites ?? [];
  const mine = sites.filter((s) => !s.system);
  const lv = useListView(sites, (s) => `${s.host} ${s.name} ${s.mode} ${s.php ?? ""} ${s.target ?? ""} ${s.system ? "sistem" : ""} ${s.enabled ? "aktif" : "jeda"}`, undefined, SITE_FACETS);

  const resume = async (s: Site) => {
    setNote(null);
    try {
      await post(`/api/sites/${s.name}/enable`, {});
      setNote({ tone: "ok", text: `${s.host} diaktifkan.` });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      reload();
      setTimeout(reload, 1500); // nginx butuh sekitar 0,2-0,4 detik untuk memuat ulang; periksa lagi setelahnya
    }
  };

  return (
    <>
      <PageHead
        title="Website"
        desc={`Situs berdomain .lab, hanya terbuka dari tailnet. ${mine.length ? `${mine.filter((s) => s.enabled).length} dari ${mine.length} situs Anda aktif.` : "Belum ada situs Anda sendiri."}`}
        action={
          <>
            <button className="btn" onClick={() => setInstalling(true)}>Pasang cepat</button>
            <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            Tambah situs
          </button>
          </>
        }
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Card>
        <State loading={loading && !data} error={error} empty={!!data && sites.length === 0} emptyText="Belum ada situs. Pilih “Tambah situs” untuk membuat yang pertama." onRetry={reload}>
          <FilterTabs lv={lv} label="Filter jenis situs" />
          <ListSearch lv={lv} label="Cari situs" hint="Nama, jenis, PHP, atau sumber" />
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Situs</th>
                  <th scope="col">Status</th>
                  <th scope="col">Jenis</th>
                  <th scope="col">PHP</th>
                  <th scope="col">Sertifikat</th>
                  <th scope="col" className="col-low">Sumber</th>
                  <th scope="col"><span className="sr">Aksi</span></th>
                </tr>
              </thead>
              <tbody>
                {lv.view.rows.map((s) => {
                  const h = health(s);
                  return (
                    <tr key={s.name} className={s.enabled ? "" : "row-off"}>
                      <th scope="row">
                        <a className="row-link mono" href={`#/website/${s.name}`}>{s.host}</a>
                        {s.system && <> <Chip tone="mute">Sistem</Chip></>}
                      </th>
                      <td><Chip tone={h.tone}>{h.text}</Chip></td>
                      <td>{MODE_LABEL[s.mode] ?? s.mode}</td>
                      <td className="mono">{s.php ?? "-"}</td>
                      <td>
                        {s.ssl_days == null ? <span className="muted">-</span> : <Chip tone={s.ssl_days < 0 ? "bad" : s.ssl_days < 30 ? "warn" : "ok"}>{s.ssl_days < 0 ? "Kedaluwarsa" : `${s.ssl_days} hari`}</Chip>}
                      </td>
                      <td className="mono cell-trunc col-low" title={s.target}>{s.mode === "proxy" ? `127.0.0.1:${s.target}` : s.target || "-"}</td>
                      <td className="cell-end">
                        <div className="row">
                          <a className="btn btn-sm" href={`#/website/${s.name}`}>{s.system ? "Lihat" : "Kelola"}<span className="sr"> {s.host}</span></a>
                          {!s.system && s.enabled && (
                            <button className="btn btn-sm" onClick={() => setPause(s)}>
                              <Pause size={16} aria-hidden="true" /> Jeda<span className="sr"> {s.host}</span>
                            </button>
                          )}
                          {!s.system && !s.enabled && (
                            <button className="btn btn-sm" onClick={() => resume(s)}>
                              <Play size={16} aria-hidden="true" /> Aktifkan<span className="sr"> {s.host}</span>
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <Pager lv={lv} />
          <p className="small muted">Status diperiksa lewat loopback ke nginx (HEAD /), bukan dari catatan. Situs yang dijeda tidak dilayani sampai diaktifkan lagi; konfigurasinya tetap tersimpan.</p>
        </State>
      </Card>
      {installing && <InstallModal onClose={() => setInstalling(false)} onDone={(n) => { setInstalling(false); location.hash = `#/website/${n}`; }} />}
      {adding && <AddSite onClose={() => setAdding(false)} onDone={(n) => { setAdding(false); location.hash = `#/website/${n}`; }} />}
      {pause && (
        <ConfirmDialog
          title={`Jeda ${pause.host}`}
          word={pause.name}
          verb="Jeda situs"
          body={<p>Situs berhenti dilayani (nginx tidak lagi memuat konfigurasinya). Folder proyek dan konfigurasi tidak dihapus. Meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            await post(`/api/sites/${pause.name}/disable`, {});
            setNote({ tone: "ok", text: `${pause.host} dijeda.` });
            reload();
            setTimeout(reload, 1500);
          }}
          onClose={() => setPause(null)}
        />
      )}
    </>
  );
}
