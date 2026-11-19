import { useState } from "react";
import { Eraser, Eye, HardDrive, ScanSearch, ShieldCheck, Trash2, TriangleAlert } from "lucide-react";
import { errText, get, post } from "../../api";
import { fmtBytes } from "../../lib";
import { useT } from "../../i18n";
import { trackJob } from "../../jobs";
import { Alert, Card, ConfirmDialog, State, useLoad } from "../../ui";
import { ageLabel, estimate, group, isStale, parseRun, runSummary, scanAge, sizeLabel, type RunResult, type Scan, type ScanRow } from "./model";

// Ikon: ScanSearch = analisis, ShieldCheck = aman, TriangleAlert = perlu persetujuan, Eye = hanya ditampilkan, Trash2 = hapus, Eraser = bersihkan pilihan.

const shortId = (id: string) => id.replace(/^(bk|cx):/, "");

interface Review {
  ids: string[];
  preview: RunResult;
}

export default function CleanSection() {
  const t = useT();
  const q = useLoad(() => get<{ scan: Scan | null }>("/api/clean"));
  const scan = q.data?.scan ?? null;
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [note, setNote] = useState<{ tone: "ok" | "warn" | "bad"; text: string } | null>(null);
  const [review, setReview] = useState<Review | null>(null);
  const [careful, setCareful] = useState<ScanRow | null>(null);
  const g = group(scan);
  const now = Math.floor(Date.now() / 1000);

  const rescan = () => {
    setBusy(true);
    setErr("");
    post("/api/clean/scan", {})
      .then((r: { job: number }) =>
        trackJob(r.job, t("system.cl.job.scan"), () => {
          setBusy(false);
          setPicked(new Set());
          q.reload();
        }),
      )
      .catch((e) => {
        setErr(errText(e));
        setBusy(false);
      });
  };

  const afterRun = (output: string) => {
    setNote(runSummary(parseRun(output)));
    rescan();
  };

  const toggle = (id: string) =>
    setPicked((p) => {
      const n = new Set(p);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });

  const openReview = async () => {
    setErr("");
    setBusy(true);
    try {
      const r = await post<{ preview: RunResult }>("/api/clean/preview", { ids: [...picked] });
      setReview({ ids: [...picked], preview: r.preview });
    } catch (e) {
      setErr(errText(e));
    } finally {
      setBusy(false);
    }
  };

  const est = estimate(scan, picked);

  return (
    <div className="stack">
      <Card
        title={
          <span className="with-icon">
            <ScanSearch aria-hidden="true" size={18} /> {t("system.cl.title")}
          </span>
        }
        action={
          <button className="btn btn-primary" onClick={rescan} disabled={busy}>
            {busy ? t("system.cl.working") : scan ? t("system.cl.rescan") : t("system.cl.scanNow")}
          </button>
        }
      >
        <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
          {!scan ? (
            <p className="muted">{t("system.cl.none")}</p>
          ) : (
            <>
              <ul className="list">
                <li className="list-row"><span>{t("system.cl.free")}</span><span className="mono">{fmtBytes(scan.disk_free)}</span></li>
                <li className="list-row"><span>{t("system.cl.safeTotal")}</span><span className="mono">{fmtBytes(scan.safe_bytes)}</span></li>
                <li className="list-row"><span>{t("system.cl.carefulTotal")}</span><span className="mono">{fmtBytes(scan.careful_bytes)}</span></li>
              </ul>
              <p className="small muted">{t("system.cl.analyzed", { age: scanAge(scan.at, now) })}</p>
              {isStale(scan.at, now) && <Alert tone="warn">{t("system.cl.stale")}</Alert>}
            </>
          )}
          {err && <Alert>{err}</Alert>}
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
        </State>
      </Card>

      {scan && (
        <>
          <Card
            title={
              <span className="with-icon">
                <ShieldCheck aria-hidden="true" size={18} /> {t("system.cl.safe")}
              </span>
            }
          >
            <p className="small muted">{t("system.cl.safeHint")}</p>
            {(() => {
              const filled = g.safe.filter((r) => r.bytes > 0).sort((x, y) => y.bytes - x.bytes);
              const empty = g.safe.filter((r) => r.bytes === 0);
              return (
                <>
                  <ul className="cgrid">
                    {filled.map((r) => (
                      <li key={r.id} className="crow">
                        <label className="clean-pick">
                          <input type="checkbox" checked={picked.has(r.id)} onChange={() => toggle(r.id)} aria-label={t("system.cl.pick", { label: r.label })} />
                          <span>
                            <span className="clean-name">{r.label}</span>
                            <span className="small muted block">{r.why} {r.cost}</span>
                          </span>
                        </label>
                        <span className="mono csize">{sizeLabel(r)}</span>
                      </li>
                    ))}
                  </ul>
                  {filled.length === 0 && <p className="muted">{t("system.cl.allEmpty")}</p>}
                  {empty.length > 0 && (
                    <details className="small muted">
                      <summary>{t("system.cl.emptyN", { n: empty.length })}</summary>
                      <p>{empty.map((r) => r.label).join(", ")}.</p>
                    </details>
                  )}
                </>
              );
            })()}
            <div className="sticky-bar">
              <span className="small" aria-live="polite">{picked.size === 0 ? t("system.cl.noneSel") : t("system.cl.sel", { n: picked.size, size: fmtBytes(est) })}</span>
              <span className="row-end">
                <button className="btn" onClick={() => setPicked(new Set(g.safe.filter((r) => r.bytes > 0).map((r) => r.id)))} disabled={busy}>
                  {t("system.cl.selAll")}
                </button>
                <button className="btn btn-primary" onClick={openReview} disabled={busy || picked.size === 0}>
                  <Eraser aria-hidden="true" size={16} /> {t("system.cl.review")}
                </button>
              </span>
            </div>
          </Card>

          <Card
            title={
              <span className="with-icon">
                <TriangleAlert aria-hidden="true" size={18} /> {t("system.cl.careful")}
              </span>
            }
          >
            <p className="small muted">{t("system.cl.carefulHint")}</p>
            {g.careful.length === 0 ? (
              <p className="muted">{t("system.cl.noMatch")}</p>
            ) : (
              <ul className="cgrid">
                {[...g.careful].sort((x, y) => y.bytes - x.bytes).map((r) => {
                  const it = r.items ?? [];
                  return (
                    <li key={r.id} className="crow crow-act">
                      <div>
                        <p className="mono clean-name">{shortId(r.id)}</p>
                        <p className="small muted">{ageLabel(r.age_days)} · {it.length ? `${it.slice(0, 3).join(", ")}${it.length > 3 ? ` ${t("system.cl.more", { n: it.length - 3 })}` : ""}` : t("system.cl.emptyWord")}</p>
                      </div>
                      <span className="mono csize">{sizeLabel(r)}</span>
                      <button className="btn btn-danger" onClick={() => setCareful(r)} disabled={busy}>
                        <Trash2 aria-hidden="true" size={16} /> {t("system.cl.del")}
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </Card>

          <Card
            title={
              <span className="with-icon">
                <Eye aria-hidden="true" size={18} /> {t("system.cl.show")}
              </span>
            }
          >
            <p className="small muted">{t("system.cl.showHint")}</p>
            <ul className="bars">
              {(() => {
                const max = Math.max(1, ...g.show.map((r) => r.bytes));
                return g.show.map((r) => (
                  <li key={r.id} className="bar-row" title={r.why}>
                    <span className="bar-name"><HardDrive aria-hidden="true" size={14} /> {r.label}</span>
                    <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (r.bytes / max) * 100)}%` }} /></span>
                    <span className="mono bar-val">{r.complete ? fmtBytes(r.bytes) : `≥ ${fmtBytes(r.bytes)}`}</span>
                  </li>
                ));
              })()}
            </ul>
            <p className="small muted">{t("system.cl.fda")}</p>
          </Card>
        </>
      )}

      {review && (
        <ConfirmDialog
          title={t("system.cl.review")}
          word={t("system.cl.word")}
          verb={t("system.cl.verb")}
          onClose={() => setReview(null)}
          body={
            <div className="stack">
              <p>{t("system.cl.about1")} <strong>{fmtBytes(review.preview.bytes)}</strong>{t("system.cl.about2")}</p>
              <ul className="bullets">
                {review.preview.items.map((i) => <li key={i.id}>{i.label}: {fmtBytes(i.bytes)}</li>)}
              </ul>
              {review.preview.skipped.length > 0 && (
                <Alert tone="warn">{t("system.cl.skipped", { list: review.preview.skipped.map((s) => `${s.id} (${s.reason})`).join("; ") })}</Alert>
              )}
              <p className="small muted">{t("system.cl.reauth")}</p>
            </div>
          }
          onConfirm={async () => {
            const r = await post<{ job: number }>("/api/clean/run", { ids: review.ids });
            setBusy(true);
            trackJob(r.job, t("system.cl.job.clean"), (j) => {
              setBusy(false);
              afterRun(j.output);
            });
          }}
        />
      )}

      {careful && (
        <ConfirmDialog
          title={t("system.cl.delTitle", { name: shortId(careful.id) })}
          word={shortId(careful.id)}
          verb={t("system.cl.delVerb")}
          onClose={() => setCareful(null)}
          body={
            <div className="stack">
              <p>{t("system.cl.delBody", { size: sizeLabel(careful), age: ageLabel(careful.age_days) })}</p>
              <p className="small">{careful.cost}</p>
              <p className="small muted">{t("system.cl.contents", { list: (careful.items ?? []).join(", ") || t("system.cl.emptyWord") })}</p>
            </div>
          }
          onConfirm={async () => {
            const name = shortId(careful.id);
            const r = await post<{ job: number }>("/api/clean/run", { ids: [careful.id], typed: name });
            setBusy(true);
            trackJob(r.job, t("apps.job.remove", { name }), (j) => {
              setBusy(false);
              afterRun(j.output);
            });
          }}
        />
      )}
    </div>
  );
}
