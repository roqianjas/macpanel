import { useState } from "react";
import { Play, RotateCw, Square, Trash2, Download } from "lucide-react";
import { errText, get, post } from "../api";
import type { GetUrl } from "../api.gen";
import { trackJob } from "../jobs";
import { FacetBar } from "../features/list/ListTools";
import { applyFacet, countFacets, type Facet } from "../features/list/paging";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, Tabs, usePoll } from "../ui";
import { t, useT } from "../i18n";

interface Ctr { id: string; name: string; image: string; state: string; status: string; ports: string; project: string }
interface Img { id: string; repo: string; tag: string; size: string; created: string }
interface Proj { name: string; file: string }
interface St { installed: boolean; running: boolean; version: string; containers: number; running_containers: number; images: number }
interface Ov { status: St; containers: Ctr[]; images: Img[]; projects: Proj[] }
interface Stat { name: string; cpu: string; mem: string; memp: string }

const stateTone = (s: string) => (s === "running" ? "ok" : s === "exited" || s === "created" ? "mute" : "warn") as "ok" | "mute" | "warn";

const ctrFacets = (): Facet<Ctr>[] => [
  { id: "running", label: t("svc.state.running"), test: (c) => c.state === "running" },
  { id: "stopped", label: t("svc.state.stopped"), test: (c) => c.state !== "running" },
];

export default function Docker({ goto }: { goto?: (id: string) => void }) {
  const t = useT();
  const ov = usePoll(() => get<Ov>("/api/docker"), 6000);
  const stats = usePoll(() => (ov.data?.status.running ? get<{ stats: Stat[] }>("/api/docker/stats") : Promise.resolve({ stats: [] })), 15000, [ov.data?.status.running]);
  const [tab, setTab] = useState("container");
  const [cf, setCf] = useState("all");
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [logOf, setLogOf] = useState<{ title: string; url: GetUrl } | null>(null);
  const [rm, setRm] = useState<{ kind: "ctr" | "img"; ref: string } | null>(null);
  const [pull, setPull] = useState(false);
  const st = ov.data?.status;
  const statMap = new Map((stats.data?.stats ?? []).map((s) => [s.name, s]));

  const act = async (c: Ctr, a: "start" | "stop" | "restart") => {
    setNote(null);
    try {
      await post(`/api/docker/containers/${encodeURIComponent(c.name)}/${a}`, {});
      setNote({ tone: "ok", text: t("dev.docker.actOk", { name: c.name, a }) });
      ov.reload();
    } catch (x) { setNote({ tone: "bad", text: errText(x) }); }
  };
  const compose = async (p: Proj, a: "up" | "down") => {
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/docker/compose/${p.name}/${a}`, {});
      trackJob(r.job, t("dev.docker.composeJob", { a, name: p.name }), ov.reload);
    } catch (x) { setNote({ tone: "bad", text: errText(x) }); }
  };
  const prune = async () => {
    setNote(null);
    try {
      await post("/api/docker/images/prune", {});
      setNote({ tone: "ok", text: t("dev.docker.pruned") });
      ov.reload();
    } catch (x) { setNote({ tone: "bad", text: errText(x) }); }
  };

  return (
    <>
      <PageHead title="Docker" desc={t("dev.docker.desc")} />
      <State loading={ov.loading && !ov.data} error={ov.error} onRetry={ov.reload}>
        {st && !st.installed && (
          <Card title={t("dev.docker.notInstalled")}>
            <p className="muted">{t("dev.docker.colima")}</p>
            <div className="row-end"><button className="btn btn-primary" onClick={() => goto?.("aplikasi")}>{t("dev.docker.openAppsInstall")}</button></div>
          </Card>
        )}
        {st && st.installed && !st.running && (
          <Card title={t("dev.docker.notRunning")}>
            <p className="muted">{t("dev.docker.startHint")}</p>
            <div className="row-end"><button className="btn btn-primary" onClick={() => goto?.("aplikasi")}>{t("dev.docker.openApps")}</button></div>
          </Card>
        )}
        {st?.running && ov.data && (
          <>
            {note && <Alert tone={note.tone}>{note.text}</Alert>}
            <div className="stats" style={{ display: "flex", gap: 12, flexWrap: "wrap", margin: "0 0 12px" }}>
              <Chip tone="ok">{t("dev.docker.runningVer", { version: st.version })}</Chip>
              <Chip tone="mute">{t("dev.docker.activeCtr", { up: st.running_containers, total: st.containers })}</Chip>
              <Chip tone="mute">{t("dev.docker.imgCount", { n: st.images })}</Chip>
            </div>
            <Tabs label={t("dev.docker.sections")} value={tab} onChange={setTab} tabs={[{ id: "container", label: t("dev.docker.tab.ctr") }, { id: "compose", label: t("dev.docker.tab.compose") }, { id: "image", label: t("dev.docker.tab.img") }]} />
            {tab === "container" && (
              <Card>
                <FacetBar label={t("dev.docker.filterStatus")} value={cf} onChange={setCf} facets={countFacets(ov.data.containers, ctrFacets())} />
                {ov.data.containers.length === 0 ? <p className="muted">{t("dev.docker.noCtr")}</p> : (
                  <div className="tbl-wrap"><table className="tbl">
                    <thead><tr><th scope="col">{t("dev.col.name")}</th><th scope="col">{t("apps.col.status")}</th><th scope="col">{t("dev.docker.tab.img")}</th><th scope="col">{t("apps.col.port")}</th><th scope="col">CPU / RAM</th><th scope="col"><span className="sr">{t("dev.col.actions")}</span></th></tr></thead>
                    <tbody>
                      {applyFacet(ov.data.containers, ctrFacets(), cf).map((c) => {
                        const s = statMap.get(c.name);
                        return (
                          <tr key={c.id}>
                            <td><span className="mono">{c.name}</span>{c.project && <div className="muted small">{t("dev.docker.project", { name: c.project })}</div>}</td>
                            <td><Chip tone={stateTone(c.state)}>{c.state}</Chip><div className="muted small">{c.status}</div></td>
                            <td className="mono small">{c.image}</td>
                            <td className="mono small">{c.ports || "-"}</td>
                            <td className="mono small">{s ? `${s.cpu} · ${s.mem.split(" / ")[0]}` : "-"}</td>
                            <td><div className="row-end" style={{ flexWrap: "wrap" }}>
                              {c.state === "running" ? (
                                <button className="btn btn-icon" onClick={() => act(c, "stop")} aria-label={t("dev.docker.stopAria", { name: c.name })}><Square size={16} aria-hidden /></button>
                              ) : (
                                <button className="btn btn-icon" onClick={() => act(c, "start")} aria-label={t("dev.docker.runAria", { name: c.name })}><Play size={16} aria-hidden /></button>
                              )}
                              <button className="btn btn-icon" onClick={() => act(c, "restart")} aria-label={t("dev.docker.restartAria", { name: c.name })}><RotateCw size={16} aria-hidden /></button>
                              <button className="btn" onClick={() => setLogOf({ title: t("apps.logTitle", { name: c.name }), url: `/api/docker/containers/${encodeURIComponent(c.name)}/logs?n=200` })}>{t("apps.log")}<span className="sr"> {c.name}</span></button>
                              <button className="btn btn-danger-ghost btn-icon" onClick={() => setRm({ kind: "ctr", ref: c.name })} aria-label={t("dev.docker.rmAria", { name: c.name })}><Trash2 size={16} aria-hidden /></button>
                            </div></td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table></div>
                )}
              </Card>
            )}
            {tab === "compose" && (
              <Card>
                <p className="muted small">{t("dev.docker.composeNote1")} <span className="mono">compose.yaml</span> {t("dev.docker.composeOr")} <span className="mono">docker-compose.yml</span> {t("dev.docker.composeNote2")}</p>
                {ov.data.projects.length === 0 ? <p className="muted">{t("dev.docker.noProj")}</p> : (
                  <div className="tbl-wrap"><table className="tbl">
                    <thead><tr><th scope="col">{t("dev.docker.col.project")}</th><th scope="col">{t("dev.docker.col.file")}</th><th scope="col">{t("dev.docker.tab.ctr")}</th><th scope="col"><span className="sr">{t("dev.col.actions")}</span></th></tr></thead>
                    <tbody>
                      {ov.data.projects.map((p) => {
                        const cs = ov.data!.containers.filter((c) => c.project === p.name);
                        const up = cs.filter((c) => c.state === "running").length;
                        return (
                          <tr key={p.name}>
                            <td className="mono">{p.name}</td>
                            <td className="mono small">{p.file}</td>
                            <td>{cs.length ? <Chip tone={up === cs.length ? "ok" : "warn"}>{t("dev.docker.activeCtr", { up, total: cs.length })}</Chip> : <Chip tone="mute">{t("dev.docker.notStarted")}</Chip>}</td>
                            <td><div className="row-end" style={{ flexWrap: "wrap" }}>
                              <button className="btn" onClick={() => compose(p, "up")}>{t("dev.docker.run")}</button>
                              <button className="btn" onClick={() => compose(p, "down")}>{t("apps.stop")}</button>
                              <button className="btn" onClick={() => setLogOf({ title: t("dev.docker.projLog", { name: p.name }), url: `/api/docker/compose/${p.name}/logs?n=200` })}>{t("apps.log")}<span className="sr"> {p.name}</span></button>
                            </div></td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table></div>
                )}
              </Card>
            )}
            {tab === "image" && (
              <Card title={t("dev.docker.tab.img")} action={<div className="row-end"><button className="btn" onClick={prune}>{t("dev.docker.prune")}</button><button className="btn btn-primary" onClick={() => setPull(true)}><Download size={16} aria-hidden /> {t("dev.docker.pull")}</button></div>}>
                {ov.data.images.length === 0 ? <p className="muted">{t("dev.docker.noImg")}</p> : (
                  <div className="tbl-wrap"><table className="tbl">
                    <thead><tr><th scope="col">{t("dev.docker.tab.img")}</th><th scope="col">Tag</th><th scope="col">{t("dev.docker.col.size")}</th><th scope="col">{t("dev.docker.col.created")}</th><th scope="col"><span className="sr">{t("dev.col.actions")}</span></th></tr></thead>
                    <tbody>
                      {ov.data.images.map((i) => (
                        <tr key={i.id + i.tag}>
                          <td className="mono">{i.repo}</td><td className="mono small">{i.tag}</td><td>{i.size}</td><td className="muted small">{i.created}</td>
                          <td><div className="row-end"><button className="btn btn-danger-ghost btn-icon" onClick={() => setRm({ kind: "img", ref: i.tag && i.tag !== "<none>" ? `${i.repo}:${i.tag}` : i.id })} aria-label={t("dev.docker.rmImgAria", { name: i.repo })}><Trash2 size={16} aria-hidden /></button></div></td>
                        </tr>
                      ))}
                    </tbody>
                  </table></div>
                )}
              </Card>
            )}
          </>
        )}
      </State>
      {logOf && <LogDialog title={logOf.title} url={logOf.url} onClose={() => setLogOf(null)} />}
      {pull && <PullDialog onClose={() => setPull(false)} onDone={ov.reload} />}
      {rm && (
        <ConfirmDialog
          title={rm.kind === "ctr" ? t("dev.docker.rmCtrTitle") : t("dev.docker.rmImgTitle")}
          body={<p>{rm.kind === "ctr" ? t("dev.docker.rmCtrBody") : t("dev.docker.rmImgBody")} {t("dev.docker.typeBefore")} <span className="mono">{rm.ref}</span> {t("dev.docker.typeAfter")}</p>}
          word={rm.ref}
          verb={t("apps.remove")}
          onClose={() => setRm(null)}
          onConfirm={async () => {
            if (rm.kind === "ctr") await post(`/api/docker/containers/${encodeURIComponent(rm.ref)}/remove`, { confirm: rm.ref });
            else await post("/api/docker/images/remove", { ref: rm.ref, confirm: rm.ref });
            setRm(null);
            ov.reload();
          }}
        />
      )}
    </>
  );
}

function LogDialog({ title, url, onClose }: { title: string; url: GetUrl; onClose: () => void }) {
  const t = useT();
  const { data, error, loading, reload } = usePoll(() => get<{ log: string }>(url), 5000);
  return (
    <Modal title={title} onClose={onClose} wide>
      <State loading={loading && !data} error={error} empty={!!data && !data.log.trim()} emptyText={t("dev.docker.logEmpty")} onRetry={reload}>
        <pre className="logbox mono">{data?.log}</pre>
      </State>
      <div className="row-end"><button className="btn" onClick={onClose}>{t("common.close")}</button></div>
    </Modal>
  );
}

function PullDialog({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const t = useT();
  const [ref, setRef] = useState("");
  const [err, setErr] = useState("");
  const ok = /^[a-z0-9][a-z0-9._/:@-]{0,200}$/.test(ref);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr("");
    try {
      const r = await post<{ job: number }>("/api/docker/images/pull", { ref });
      trackJob(r.job, t("dev.docker.pullJob", { ref }), onDone);
      onClose();
    } catch (x) { setErr(errText(x)); }
  };
  return (
    <Modal title={t("dev.docker.pull")} onClose={onClose}>
      <form onSubmit={submit}>
        {err && <Alert>{err}</Alert>}
        <Field label={t("dev.docker.imgName")} hint={t("dev.docker.imgHint")}>
          {(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="nginx:1.27" required maxLength={200} pattern="[a-z0-9][a-z0-9._/:@\-]{0,200}" title={t("dev.docker.imgPattern")} value={ref} onChange={(e) => setRef(e.target.value.trim())} data-autofocus />}
        </Field>
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn btn-primary" disabled={!ok}>{t("dev.docker.pullBtn")}</button>
        </div>
      </form>
    </Modal>
  );
}
