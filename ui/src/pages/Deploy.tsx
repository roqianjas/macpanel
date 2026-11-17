import { useState } from "react";
import { Rocket } from "lucide-react";
import { errText, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, State, useLoad } from "../ui";
import Webhook from "./Webhook";

interface Info {
  dir: string;
  php: string | null;
  steps: { id: string; label: string }[];
  git: { branch: string; commit: string; dirty: boolean; remote: boolean } | null;
}

const DEFAULT_ON = new Set(["git_pull", "composer_install", "node_install", "node_build", "fpm_restart"]);
const EXCLUSIVE: Record<string, string> = { composer_install: "composer_install_nodev", composer_install_nodev: "composer_install" };

export default function Deploy({ name }: { name: string }) {
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
      trackJob(r.job, `Deploy ${name}`, (j) => {
        setBusy(false);
        setOut({ ok: j.status === "ok", text: j.output });
        q.reload();
      });
    } catch (x) {
      setBusy(false);
      setErr(errText(x));
    }
  };
  return (
    <div className="stack">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {q.data && (
          <>
            <Card title="Proyek">
              <p className="mono small">{q.data.dir}</p>
              {q.data.git ? (
                <p className="small">
                  Cabang <strong className="mono">{q.data.git.branch}</strong>, commit terakhir <span className="mono">{q.data.git.commit}</span>{" "}
                  {q.data.git.dirty && <Chip tone="warn">Ada perubahan lokal</Chip>} {!q.data.git.remote && <Chip tone="warn">Tanpa remote</Chip>}
                </p>
              ) : (
                <p className="small muted">Folder ini bukan repositori git, jadi langkah git pull tidak tersedia.</p>
              )}
              <p className="small muted">Langkah dijalankan berurutan dan berhenti di langkah pertama yang gagal. Git memakai SSH tanpa prompt: bila key butuh passphrase, tarik manual di terminal. Meminta kode TOTP ulang.</p>
            </Card>
            <Card title="Langkah deploy">
              {steps.length === 0 ? (
                <p className="muted">Tidak ada langkah yang cocok untuk proyek ini.</p>
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
                  {busy ? "Berjalan…" : `Jalankan ${chosen.size} langkah`}
                </button>
              </div>
            </Card>
            <Webhook name={name} steps={steps} />
            {out && (
              <Card title={out.ok ? "Hasil: selesai" : "Hasil: gagal"}>
                <pre className="logbox mono" tabIndex={0} aria-label="Keluaran deploy">{out.text || "(tanpa keluaran)"}</pre>
              </Card>
            )}
          </>
        )}
      </State>
    </div>
  );
}
