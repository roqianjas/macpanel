import { useEffect, useState } from "react";
import { Download, Settings2, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, useLoad } from "../ui";
import PhpManage from "./PhpManage";
import { PhpVersions } from "../features/php/PhpVersions";
import { useT } from "../i18n";

interface PHP {
  version: string;
  installed: boolean;
  full?: string;
  fpm: boolean;
}
interface RT {
  tool: string;
  version: string;
  default: boolean;
  pruned: boolean;
}

const TOOLS = [
  { id: "node", label: "Node.js", sample: "24" },
  { id: "go", label: "Go", sample: "1.27" },
  { id: "python", label: "Python", sample: "3.13" },
  { id: "pnpm", label: "pnpm", sample: "12" },
];

export default function Runtimes() {
  const t = useT();
  const php = useLoad(() => get<{ php: PHP[] }>("/api/php"));
  const rt = useLoad(() => get<{ runtimes: RT[] }>("/api/runtime"));
  // FPM butuh beberapa detik untuk naik setelah pemasangan; periksa ulang sampai aktif agar tidak menampilkan "mati" yang basi.
  const fpmDown = !!php.data?.php.some((p) => p.installed && !p.fpm);
  useEffect(() => {
    if (!fpmDown) return;
    const iv = setInterval(php.reload, 4000);
    return () => clearInterval(iv);
  }, [fpmDown, php.reload]);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [rmPhp, setRmPhp] = useState<string | null>(null);
  const [manage, setManage] = useState<string | null>(null);
  const comp = useLoad(() => get<{ installed: boolean; version: string; php: string; outdated: boolean; latest: string }>("/api/composer"));
  const [rmRt, setRmRt] = useState<RT | null>(null);
  const [ver, setVer] = useState<Record<string, string>>({});

  const installPhp = async (v: string) => {
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/php/${v}/install`, {});
      trackJob(r.job, t("dev.rt.installPhp", { v }), php.reload);
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const composerAct = async (a: "install" | "update") => {
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/composer/${a}`, {});
      trackJob(r.job, a === "install" ? t("dev.rt.composerInstall") : t("dev.rt.composerUpdate"), comp.reload);
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const installRt = async (tool: string) => {
    const v = (ver[tool] ?? "").trim();
    if (!v) return;
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/runtime/${tool}/${v}/install`, {});
      trackJob(r.job, t("dev.rt.installTool", { tool, v }), rt.reload);
      setVer((p) => ({ ...p, [tool]: "" }));
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const useRt = async (r: RT) => {
    setNote(null);
    try {
      await post(`/api/runtime/${r.tool}/${r.version}/use`, {});
      setNote({ tone: "ok", text: t("dev.rt.madeDefault", { tool: r.tool, v: r.version }) });
      rt.reload();
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <>
      <PageHead title={t("nav.runtimes")} desc={t("dev.rt.desc")} />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <PhpVersions />
      <Card title="PHP">
        <State loading={php.loading && !php.data} error={php.error} onRetry={php.reload}>
          <ul className="list">
            {php.data?.php.map((p) => (
              <li key={p.version} className="list-row">
                <div>
                  <p>
                    <strong>PHP {p.version}</strong> {p.installed ? <span className="mono muted">{p.full}</span> : null}
                  </p>
                  <p className="small muted">{p.installed ? (p.fpm ? t("dev.rt.fpmSock") : t("dev.rt.fpmNotYet")) : t("svc.state.not_installed")}</p>
                </div>
                <div className="row">
                  {p.installed ? (
                    <>
                      <Chip tone={p.fpm ? "ok" : "warn"}>{p.fpm ? t("dev.rt.installed") : t("dev.rt.fpmOff")}</Chip>
                      <button className="btn" onClick={() => setManage(p.version)}>
                        <Settings2 size={18} aria-hidden="true" />
                        {t("dev.rt.manage")}<span className="sr"> PHP {p.version}</span>
                      </button>
                      <button className="btn btn-danger-ghost" onClick={() => setRmPhp(p.version)}>
                        <Trash2 size={18} aria-hidden="true" />
                        {t("apps.remove")}
                        <span className="sr"> PHP {p.version}</span>
                      </button>
                    </>
                  ) : (
                    <button className="btn" onClick={() => installPhp(p.version)}>
                      <Download size={18} aria-hidden="true" />
                      {t("apps.install")}
                      <span className="sr"> PHP {p.version}</span>
                    </button>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </State>
      </Card>
      <Card title="Composer">
        <State loading={comp.loading && !comp.data} error={comp.error} onRetry={comp.reload}>
          <div className="list-row">
            <div>
              <p>
                <strong>Composer</strong> {comp.data?.installed && <span className="mono muted">{comp.data.version}</span>}
              </p>
              <p className="small muted">
                {!comp.data?.installed ? t("dev.rt.notInstalledDot") : comp.data.outdated ? t("dev.rt.verAvail", { v: comp.data.latest }) : t("dev.rt.latest")} {t("dev.rt.composerNote")}
              </p>
            </div>
            <div className="row">
              {!comp.data?.installed ? (
                <button className="btn" onClick={() => composerAct("install")}>
                  <Download size={18} aria-hidden="true" />
                  {t("dev.rt.composerInstall")}
                </button>
              ) : (
                <button className="btn" disabled={!comp.data.outdated} onClick={() => composerAct("update")}>
                  <Download size={18} aria-hidden="true" />
                  {t("dev.rt.composerUpdate")}
                </button>
              )}
            </div>
          </div>
        </State>
      </Card>
      {TOOLS.map((tl) => {
        const items = rt.data?.runtimes.filter((r) => r.tool === tl.id) ?? [];
        return (
          <Card key={tl.id} title={tl.label}>
            <State loading={rt.loading && !rt.data} error={rt.error} empty={!!rt.data && items.length === 0} emptyText={t("dev.rt.noneManaged", { label: tl.label })} onRetry={rt.reload}>
              <ul className="list">
                {items.map((r) => (
                  <li key={r.version} className="list-row">
                    <span className="mono">{r.version}</span>
                    <div className="row">
                      {r.default && (
                        <Chip tone="info">{t("dev.php.builtin")}</Chip>
                      )}
                      {r.pruned && <Chip tone="warn">{t("dev.rt.willPrune")}</Chip>}
                      {!r.default && (
                        <button className="btn" onClick={() => useRt(r)}>
                          {t("dev.rt.makeDefault")}
                          <span className="sr">
                            {" "}
                            {tl.label} {r.version}
                          </span>
                        </button>
                      )}
                      <button className="btn btn-danger-ghost" onClick={() => setRmRt(r)}>
                        <Trash2 size={18} aria-hidden="true" />
                        {t("apps.remove")}
                        <span className="sr">
                          {" "}
                          {tl.label} {r.version}
                        </span>
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            </State>
            <form
              className="inline-form"
              onSubmit={(e) => {
                e.preventDefault();
                installRt(tl.id);
              }}
            >
              <label className="sr" htmlFor={`v-${tl.id}`}>
                {t("dev.rt.verInstalled", { label: tl.label })}
              </label>
              <input id={`v-${tl.id}`} className="mono" placeholder={t("dev.rt.verPh", { sample: tl.sample })} value={ver[tl.id] ?? ""} onChange={(e) => setVer((p) => ({ ...p, [tl.id]: e.target.value }))} pattern="(latest|lts|[0-9]+(\.[0-9]+){0,2})" />
              <button className="btn" disabled={!(ver[tl.id] ?? "").trim()}>
                <Download size={18} aria-hidden="true" />
                {t("dev.rt.installVer")}
              </button>
            </form>
          </Card>
        );
      })}
      {manage && <PhpManage version={manage} onClose={() => { setManage(null); php.reload(); }} />}
      {rmPhp && (
        <ConfirmDialog
          title={t("dev.rt.rmPhpTitle", { v: rmPhp })}
          word={rmPhp}
          verb={t("dev.rt.rmPhp")}
          body={<p>{t("dev.rt.rmPhpBody")}</p>}
          onConfirm={async () => {
            const r = await post<{ job: number }>(`/api/php/${rmPhp}/remove`, { confirm: rmPhp });
            trackJob(r.job, t("dev.rt.rmPhpTitle", { v: rmPhp }), php.reload);
          }}
          onClose={() => setRmPhp(null)}
        />
      )}
      {rmRt && (
        <ConfirmDialog
          title={t("dev.rt.rmRtTitle", { tool: rmRt.tool, v: rmRt.version })}
          word={`${rmRt.tool}@${rmRt.version}`}
          verb={t("dev.rt.rmVer")}
          body={<p>{t("dev.rt.rmRtBody")}</p>}
          onConfirm={async () => {
            await post(`/api/runtime/${rmRt.tool}/${rmRt.version}/remove`, { confirm: `${rmRt.tool}@${rmRt.version}` });
            rt.reload();
          }}
          onClose={() => setRmRt(null)}
        />
      )}
    </>
  );
}
