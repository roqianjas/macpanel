import { useEffect, useState } from "react";
import { Download, Settings2, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, useLoad } from "../ui";
import PhpManage from "./PhpManage";

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
  const php = useLoad(() => get<{ php: PHP[] }>("/api/php"));
  const rt = useLoad(() => get<{ runtimes: RT[] }>("/api/runtime"));
  // FPM butuh beberapa detik untuk naik setelah pemasangan; periksa ulang sampai aktif agar tidak menampilkan "mati" yang basi.
  const fpmDown = !!php.data?.php.some((p) => p.installed && !p.fpm);
  useEffect(() => {
    if (!fpmDown) return;
    const t = setInterval(php.reload, 4000);
    return () => clearInterval(t);
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
      trackJob(r.job, `Pasang PHP ${v}`, php.reload);
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const composerAct = async (a: "install" | "update") => {
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/composer/${a}`, {});
      trackJob(r.job, a === "install" ? "Pasang Composer" : "Perbarui Composer", comp.reload);
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
      trackJob(r.job, `Pasang ${tool} ${v}`, rt.reload);
      setVer((p) => ({ ...p, [tool]: "" }));
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const useRt = async (r: RT) => {
    setNote(null);
    try {
      await post(`/api/runtime/${r.tool}/${r.version}/use`, {});
      setNote({ tone: "ok", text: `${r.tool} ${r.version} dijadikan versi bawaan.` });
      rt.reload();
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <>
      <PageHead title="PHP dan runtime" desc="Pasang beberapa versi berdampingan. Pemasangan berjalan di latar belakang; progresnya tampil di pojok kanan bawah." />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Card title="PHP">
        <State loading={php.loading && !php.data} error={php.error} onRetry={php.reload}>
          <ul className="list">
            {php.data?.php.map((p) => (
              <li key={p.version} className="list-row">
                <div>
                  <p>
                    <strong>PHP {p.version}</strong> {p.installed ? <span className="mono muted">{p.full}</span> : null}
                  </p>
                  <p className="small muted">{p.installed ? (p.fpm ? "FPM aktif lewat unix socket" : "Terpasang, FPM belum aktif") : "Belum terpasang"}</p>
                </div>
                <div className="row">
                  {p.installed ? (
                    <>
                      <Chip tone={p.fpm ? "ok" : "warn"}>{p.fpm ? "Terpasang" : "FPM mati"}</Chip>
                      <button className="btn" onClick={() => setManage(p.version)}>
                        <Settings2 size={18} aria-hidden="true" />
                        Kelola<span className="sr"> PHP {p.version}</span>
                      </button>
                      <button className="btn btn-danger-ghost" onClick={() => setRmPhp(p.version)}>
                        <Trash2 size={18} aria-hidden="true" />
                        Hapus
                        <span className="sr"> PHP {p.version}</span>
                      </button>
                    </>
                  ) : (
                    <button className="btn" onClick={() => installPhp(p.version)}>
                      <Download size={18} aria-hidden="true" />
                      Pasang
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
                {!comp.data?.installed ? "Belum terpasang." : comp.data.outdated ? `Versi ${comp.data.latest} tersedia.` : "Versi terbaru."} Dipasang lewat Homebrew; perintah composer per proyek ada di tab Deploy tiap situs.
              </p>
            </div>
            <div className="row">
              {!comp.data?.installed ? (
                <button className="btn" onClick={() => composerAct("install")}>
                  <Download size={18} aria-hidden="true" />
                  Pasang Composer
                </button>
              ) : (
                <button className="btn" disabled={!comp.data.outdated} onClick={() => composerAct("update")}>
                  <Download size={18} aria-hidden="true" />
                  Perbarui Composer
                </button>
              )}
            </div>
          </div>
        </State>
      </Card>
      {TOOLS.map((t) => {
        const items = rt.data?.runtimes.filter((r) => r.tool === t.id) ?? [];
        return (
          <Card key={t.id} title={t.label}>
            <State loading={rt.loading && !rt.data} error={rt.error} empty={!!rt.data && items.length === 0} emptyText={`Belum ada ${t.label} yang dikelola mise.`} onRetry={rt.reload}>
              <ul className="list">
                {items.map((r) => (
                  <li key={r.version} className="list-row">
                    <span className="mono">{r.version}</span>
                    <div className="row">
                      {r.default && (
                        <Chip tone="info">Bawaan</Chip>
                      )}
                      {r.pruned && <Chip tone="warn">Akan dibersihkan</Chip>}
                      {!r.default && (
                        <button className="btn" onClick={() => useRt(r)}>
                          Jadikan bawaan
                          <span className="sr">
                            {" "}
                            {t.label} {r.version}
                          </span>
                        </button>
                      )}
                      <button className="btn btn-danger-ghost" onClick={() => setRmRt(r)}>
                        <Trash2 size={18} aria-hidden="true" />
                        Hapus
                        <span className="sr">
                          {" "}
                          {t.label} {r.version}
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
                installRt(t.id);
              }}
            >
              <label className="sr" htmlFor={`v-${t.id}`}>
                Versi {t.label} yang dipasang
              </label>
              <input id={`v-${t.id}`} className="mono" placeholder={`mis. ${t.sample} atau latest`} value={ver[t.id] ?? ""} onChange={(e) => setVer((p) => ({ ...p, [t.id]: e.target.value }))} pattern="(latest|lts|[0-9]+(\.[0-9]+){0,2})" />
              <button className="btn" disabled={!(ver[t.id] ?? "").trim()}>
                <Download size={18} aria-hidden="true" />
                Pasang versi
              </button>
            </form>
          </Card>
        );
      })}
      {manage && <PhpManage version={manage} onClose={() => { setManage(null); php.reload(); }} />}
      {rmPhp && (
        <ConfirmDialog
          title={`Hapus PHP ${rmPhp}`}
          word={rmPhp}
          verb="Hapus PHP"
          body={<p>Situs yang memakai versi ini akan berhenti bekerja sampai dipindah ke versi lain. Meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            const r = await post<{ job: number }>(`/api/php/${rmPhp}/remove`, { confirm: rmPhp });
            trackJob(r.job, `Hapus PHP ${rmPhp}`, php.reload);
          }}
          onClose={() => setRmPhp(null)}
        />
      )}
      {rmRt && (
        <ConfirmDialog
          title={`Hapus ${rmRt.tool} ${rmRt.version}`}
          word={`${rmRt.tool}@${rmRt.version}`}
          verb="Hapus versi"
          body={<p>Proyek yang bergantung pada versi ini perlu dipindah. Meminta kata sandi ulang.</p>}
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
