import { useEffect, useState } from "react";
import { Download, Play, RotateCw, ScrollText, Square, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { serviceView, useServices, type ServiceInfo } from "../data";
import { trackJob } from "../jobs";
import { FacetBar } from "../features/list/ListTools";
import { applyFacet, countFacets, type Facet } from "../features/list/paging";
import { useT, type Key } from "../i18n";
import { Alert, Card, Chip, ConfirmDialog, Mark, Modal, PageHead, Skeleton, State } from "../ui";

const ORDER = ["Web server", "PHP", "Database", "Cache dan antrean", "Monitoring", "Jaringan", "Alat pengembangan", "Sistem"];

const APP_FACETS: Facet<ServiceInfo>[] = [
  ...ORDER.map((g) => ({ id: g, label: `svc.group.${g}`, test: (s: ServiceInfo) => s.group === g })),
  { id: "_bermasalah", label: "apps.filter.down", test: (s: ServiceInfo) => s.installed && s.state !== "running" },
  { id: "_opsional", label: "apps.filter.optional", test: (s: ServiceInfo) => s.optional },
];

export default function Apps() {
  const t = useT();
  const { data, error, loading, reload } = useServices();
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [busy, setBusy] = useState("");
  const [confirm, setConfirm] = useState<{ s: ServiceInfo; act: "stop" | "restart" | "remove" } | null>(null);
  const [log, setLog] = useState<ServiceInfo | null>(null);

  const [facet, setFacet] = useState("all");
  const all = data?.services ?? [];
  const facets = countFacets(all, APP_FACETS);
  const active = facets.some((f) => f.id === facet) ? facet : "all";
  const services = applyFacet(all, APP_FACETS, active);
  const groups = ORDER.map((g) => ({ g, items: services.filter((s) => s.group === g) })).filter((x) => x.items.length);

  const start = async (s: ServiceInfo) => {
    setBusy(s.id);
    setNote(null);
    try {
      await post(`/api/services/${s.id}/start`, {});
      setNote({ tone: "ok", text: t("apps.started", { name: s.label }) });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy("");
      void reload();
    }
  };
  const install = async (s: ServiceInfo) => {
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/apps/${s.id}/install`, {});
      trackJob(r.job, t("apps.job.install", { name: s.label }), () => void reload());
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const bad = services.filter((s) => s.installed && !s.optional && s.state !== "running");

  return (
    <>
      <PageHead title={t("apps.title")} desc={t("apps.desc")} />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      {bad.length > 0 && <Alert tone="warn">{t("apps.down", { names: bad.map((b) => b.label).join(", ") })}</Alert>}
      <FacetBar facets={facets.map((f) => ({ ...f, label: t(f.label as Key) }))} value={active} onChange={setFacet} label={t("apps.filter.label")} />
      <State loading={loading && !data} error={error} empty={!!data && groups.length === 0} emptyText={t("apps.empty")} onRetry={reload}>
        {groups.map(({ g, items }) => (
          <Card key={g} title={t(`svc.group.${g}` as Key)}>
            <div className="tbl-wrap">
              <table className="tbl tbl-fixed svc-table">
                <colgroup><col style={{ width: "30%" }} /><col style={{ width: "128px" }} /><col style={{ width: "96px" }} /><col style={{ width: "64px" }} /><col /></colgroup>
                <thead>
                  <tr>
                    <th scope="col">{t("apps.col.service")}</th>
                    <th scope="col">{t("apps.col.status")}</th>
                    <th scope="col">{t("apps.col.version")}</th>
                    <th scope="col">{t("apps.col.port")}</th>
                    <th scope="col"><span className="sr">Aksi</span></th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((s) => (
                    <tr key={s.id} className={s.installed ? "" : "row-off"}>
                      <th scope="row">
                        <span className="row"><Mark tone={serviceView(s).tone} /><span>{s.label}</span></span>
                        <span className="small muted" style={{ fontWeight: 400, display: "block" }}>{s.desc}</span>
                      </th>
                      <td><Chip tone={serviceView(s).tone}>{serviceView(s).text}</Chip></td>
                      <td className="mono" data-label={t("apps.col.version")}>{s.version || "-"}</td>
                      <td className="mono" data-label={t("apps.col.port")}>{s.port ?? "-"}</td>
                      <td className="cell-end">
                        <div className="row">
                          {!s.installed && s.optional && (
                            <button className="btn btn-sm" onClick={() => install(s)}>
                              <Download size={16} aria-hidden="true" /> {t("apps.install")}<span className="sr"> {s.label}</span>
                            </button>
                          )}
                          {s.installed && s.ctl && s.state !== "running" && (
                            <button className="btn btn-sm" disabled={busy === s.id} onClick={() => start(s)}>
                              <Play size={16} aria-hidden="true" /> {busy === s.id ? t("apps.starting") : t("apps.start")}<span className="sr"> {s.label}</span>
                            </button>
                          )}
                          {s.installed && s.ctl && s.state === "running" && (
                            <>
                              <button className="btn btn-sm" onClick={() => setConfirm({ s, act: "restart" })}>
                                <RotateCw size={16} aria-hidden="true" /> {t("apps.restart")}<span className="sr"> {s.label}</span>
                              </button>
                              <button className="btn btn-sm btn-danger-ghost" onClick={() => setConfirm({ s, act: "stop" })}>
                                <Square size={16} aria-hidden="true" /> {t("apps.stop")}<span className="sr"> {s.label}</span>
                              </button>
                            </>
                          )}
                          {s.installed && !s.ctl && <span className="small muted">{t("apps.external")}</span>}
                          {s.installed && s.has_log && (
                            <button className="btn btn-sm" onClick={() => setLog(s)}>
                              <ScrollText size={16} aria-hidden="true" /> {t("apps.log")}<span className="sr"> {s.label}</span>
                            </button>
                          )}
                          {s.installed && s.optional && (
                            <button className="btn btn-sm btn-danger-ghost" onClick={() => setConfirm({ s, act: "remove" })}>
                              <Trash2 size={16} aria-hidden="true" /> {t("apps.remove")}<span className="sr"> {s.label}</span>
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        ))}
        <p className="small muted">{t("apps.note")}</p>
      </State>

      {confirm && (
        <ConfirmDialog
          title={t(`apps.confirm.${confirm.act}` as Key, { name: confirm.s.label })}
          word={confirm.s.id}
          verb={confirm.act === "stop" ? t("apps.stop") : confirm.act === "restart" ? t("apps.restart") : t("apps.confirm.removeVerb")}
          body={
            <p>
              {confirm.act === "remove"
                ? t("apps.confirm.removeBody")
                : confirm.s.group === "Database" || confirm.s.id === "nginx" || confirm.s.group === "PHP"
                  ? t("apps.confirm.dependents")
                  : t("apps.confirm.brief")}{" "}
              {t("common.reauth")}
            </p>
          }
          onConfirm={async () => {
            if (confirm.act === "remove") {
              const r = await post<{ job: number }>(`/api/apps/${confirm.s.id}/remove`, { confirm: confirm.s.id });
              trackJob(r.job, t("apps.job.remove", { name: confirm.s.label }), () => void reload());
            } else {
              await post(`/api/services/${confirm.s.id}/${confirm.act}`, {});
              setNote({ tone: "ok", text: t(confirm.act === "stop" ? "apps.stopped" : "apps.restarted", { name: confirm.s.label }) });
              void reload();
            }
          }}
          onClose={() => setConfirm(null)}
        />
      )}
      {log && <LogDialog s={log} onClose={() => setLog(null)} />}
    </>
  );
}

function LogDialog({ s, onClose }: { s: ServiceInfo; onClose: () => void }) {
  const t = useT();
  const [data, setData] = useState<{ log: string } | null>(null);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(true);
  const load = async () => {
    setLoading(true);
    try {
      setData(await get<{ log: string }>(`/api/services/${s.id}/logs?n=120`));
      setErr("");
    } catch (x) {
      setErr(errText(x));
    } finally {
      setLoading(false);
    }
  };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    void load();
  }, []);
  return (
    <Modal title={t("apps.logTitle", { name: s.label })} onClose={onClose} wide>
      <div className="stack">
        {loading && !data && <Skeleton />}
        {err && <Alert>{err}</Alert>}
        {data && <pre className="logbox mono">{data.log || t("apps.logEmpty")}</pre>}
        <div className="row-end">
          <button className="btn" onClick={load} disabled={loading}>{t("common.reload")}</button>
          <button className="btn" onClick={onClose}>{t("common.close")}</button>
        </div>
      </div>
    </Modal>
  );
}
