import { useState } from "react";
import { Pause, Play, Plus } from "lucide-react";
import { InstallModal } from "../../pages/Installer";
import { errText, get, post } from "../../api";
import { FilterTabs, ListSearch, Pager, useListView } from "../list/ListTools";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, useLoad } from "../../ui";
import { Site, MODE_LABEL, health } from "./types";
import { SITE_FACETS } from "./facets";
import { AddSite } from "./AddSite";
import { useT } from "../../i18n";

export function SiteList() {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<{ sites: Site[] }>("/api/sites"));
  const [adding, setAdding] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [pause, setPause] = useState<Site | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const sites = data?.sites ?? [];
  const mine = sites.filter((s) => !s.system);
  const lv = useListView(sites, (s) => `${s.host} ${s.name} ${s.mode} ${s.php ?? ""} ${s.target ?? ""} ${s.system ? "sistem system" : ""} ${s.enabled ? "aktif active" : "jeda paused"}`, undefined, SITE_FACETS);

  const resume = async (s: Site) => {
    setNote(null);
    try {
      await post(`/api/sites/${s.name}/enable`, {});
      setNote({ tone: "ok", text: t("dev.sl.resumed", { host: s.host }) });
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
        title={t("nav.websites")}
        desc={`${t("dev.sl.desc")} ${mine.length ? t("dev.sl.activeOf", { on: mine.filter((s) => s.enabled).length, n: mine.length }) : t("dev.sl.noneOwn")}`}
        action={
          <>
            <button className="btn" onClick={() => setInstalling(true)}>{t("dev.sl.quick")}</button>
            <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            {t("dev.site.add")}
          </button>
          </>
        }
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Card>
        <State loading={loading && !data} error={error} empty={!!data && sites.length === 0} emptyText={t("dev.sl.empty", { add: t("dev.site.add") })} onRetry={reload}>
          <FilterTabs lv={lv} label={t("dev.sl.filterKind")} />
          <ListSearch lv={lv} label={t("dev.sl.search")} hint={t("dev.sl.searchHint")} />
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">{t("dev.sl.col.site")}</th>
                  <th scope="col">{t("apps.col.status")}</th>
                  <th scope="col">{t("dev.sl.col.kind")}</th>
                  <th scope="col">PHP</th>
                  <th scope="col">{t("dev.sl.col.cert")}</th>
                  <th scope="col" className="col-low">{t("dev.sl.col.source")}</th>
                  <th scope="col"><span className="sr">{t("dev.col.actions")}</span></th>
                </tr>
              </thead>
              <tbody>
                {lv.view.rows.map((s) => {
                  const h = health(s);
                  return (
                    <tr key={s.name} className={s.enabled ? "" : "row-off"}>
                      <th scope="row">
                        <a className="row-link mono" href={`#/website/${s.name}`}>{s.host}</a>
                        {s.system && <> <Chip tone="mute">{t("nav.system")}</Chip></>}
                      </th>
                      <td><Chip tone={h.tone}>{h.text}</Chip></td>
                      <td>{MODE_LABEL[s.mode] ?? s.mode}</td>
                      <td className="mono">{s.php ?? "-"}</td>
                      <td>
                        {s.ssl_days == null ? <span className="muted">-</span> : <Chip tone={s.ssl_days < 0 ? "bad" : s.ssl_days < 30 ? "warn" : "ok"}>{s.ssl_days < 0 ? t("dev.ssl.expired") : t("fmt.day", { n: s.ssl_days })}</Chip>}
                      </td>
                      <td className="mono cell-trunc col-low" title={s.target}>{s.mode === "proxy" ? `127.0.0.1:${s.target}` : s.target || "-"}</td>
                      <td className="cell-end">
                        <div className="row">
                          <a className="btn btn-sm" href={`#/website/${s.name}`}>{s.system ? t("dev.sl.view") : t("dev.rt.manage")}<span className="sr"> {s.host}</span></a>
                          {!s.system && s.enabled && (
                            <button className="btn btn-sm" onClick={() => setPause(s)}>
                              <Pause size={16} aria-hidden="true" /> {t("dev.sl.pause")}<span className="sr"> {s.host}</span>
                            </button>
                          )}
                          {!s.system && !s.enabled && (
                            <button className="btn btn-sm" onClick={() => resume(s)}>
                              <Play size={16} aria-hidden="true" /> {t("dev.sl.resume")}<span className="sr"> {s.host}</span>
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
          <p className="small muted">{t("dev.sl.note")}</p>
        </State>
      </Card>
      {installing && <InstallModal onClose={() => setInstalling(false)} onDone={(n) => { setInstalling(false); location.hash = `#/website/${n}`; }} />}
      {adding && <AddSite onClose={() => setAdding(false)} onDone={(n) => { setAdding(false); location.hash = `#/website/${n}`; }} />}
      {pause && (
        <ConfirmDialog
          title={t("dev.sl.pauseTitle", { host: pause.host })}
          word={pause.name}
          verb={t("dev.sl.pauseVerb")}
          body={<p>{t("dev.sl.pauseBody")}</p>}
          onConfirm={async () => {
            await post(`/api/sites/${pause.name}/disable`, {});
            setNote({ tone: "ok", text: t("dev.sl.paused", { host: pause.host }) });
            reload();
            setTimeout(reload, 1500);
          }}
          onClose={() => setPause(null)}
        />
      )}
    </>
  );
}
