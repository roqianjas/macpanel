import { useState } from "react";
import { Rocket } from "lucide-react";
import { errText, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, State, useLoad } from "../ui";
import Webhook from "./Webhook";
import { useT } from "../i18n";

interface Info {
  dir: string;
  php: string | null;
  steps: { id: string; label: string }[];
  git: { branch: string; commit: string; dirty: boolean; remote: boolean } | null;
}

const DEFAULT_ON = new Set(["git_pull", "composer_install", "node_install", "node_build", "fpm_restart"]);
const EXCLUSIVE: Record<string, string> = { composer_install: "composer_install_nodev", composer_install_nodev: "composer_install" };

export default function Deploy({ name }: { name: string }) {
  const t = useT();
  const q = useLoad(() => get<Info>(`/api/sites/${name}/deploy`), [name]);
  const [sel, setSel] = useState<Set<string> | null>(null);
  const [busy, setBusy] = useState(false);
  const [out, setOut] = useState<{ ok: boolean; text: string } | null>(null);
  const [err, setErr] = useState("");
  const steps = q.data?.steps ?? [];
  const chosen = sel ?? new Set(steps.filter((s) => DEFAULT_ON.has(s.id)).map((s) => s.id));
  const toggle = (id: string) => {
    const n = new Set(chosen);
    if (n.has(id)) n.delete(id);
    else {
      n.add(id);
      if (EXCLUSIVE[id]) n.delete(EXCLUSIVE[id]);
    }
    setSel(n);
  };
  const run = async () => {
    setBusy(true);
    setErr("");
    setOut(null);
    try {
      const ordered = steps.filter((s) => chosen.has(s.id)).map((s) => s.id);
      const r = await post<{ job: number }>(`/api/sites/${name}/deploy`, { steps: ordered });
      trackJob(r.job, t("ops.dp.job", { name }), (j) => {
        setBusy(false);
        setOut({ ok: j.status === "ok", text: j.output });
        q.reload();
      });
    } catch (x) {
      setBusy(false);
      setErr(errText(x));
    }
  };
  if (q.error.includes("tidak punya root")) {
    return (
      <Card title={t("ops.dp.unavailable")}>
        <p className="small muted">{t("ops.dp.unavailableBody")}</p>
      </Card>
    );
  }
  return (
    <div className="stack">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {q.data && (
          <>
            <Card title={t("ops.dp.project")}>
              <p className="mono small">{q.data.dir}</p>
              {q.data.git ? (
                <p className="small">
                  {t("ops.dp.branch")} <strong className="mono">{q.data.git.branch}</strong>{t("ops.dp.lastCommit")} <span className="mono">{q.data.git.commit}</span>{" "}
                  {q.data.git.dirty && <Chip tone="warn">{t("ops.dp.dirty")}</Chip>} {!q.data.git.remote && <Chip tone="warn">{t("ops.dp.noRemote")}</Chip>}
                </p>
              ) : (
                <p className="small muted">{t("ops.dp.noGit")}</p>
              )}
              <p className="small muted">{t("ops.dp.stepsNote")}</p>
            </Card>
            <Card title={t("ops.dp.steps")}>
              {steps.length === 0 ? (
                <p className="muted">{t("ops.dp.noSteps")}</p>
              ) : (
                <ul className="list">
                  {steps.map((s) => (
                    <li key={s.id} className="list-row">
                      <label className="check">
                        <input type="checkbox" checked={chosen.has(s.id)} onChange={() => toggle(s.id)} />
                        <span>{s.label}</span>
                      </label>
                      <span className="mono small muted">{s.id}</span>
                    </li>
                  ))}
                </ul>
              )}
              {err && <Alert>{err}</Alert>}
              <div className="row-end">
                <button className="btn btn-primary" disabled={busy || chosen.size === 0} onClick={run}>
                  <Rocket size={18} aria-hidden="true" />
                  {busy ? t("ops.running") : t("ops.dp.run", { n: chosen.size })}
                </button>
              </div>
            </Card>
            <Webhook name={name} steps={steps} />
            {out && (
              <Card title={out.ok ? t("ops.dp.resultOk") : t("ops.dp.resultBad")}>
                <pre className="logbox mono" tabIndex={0} aria-label={t("ops.dp.outputAria")}>{out.text || t("ops.dp.noOutput")}</pre>
              </Card>
            )}
          </>
        )}
      </State>
    </div>
  );
}
