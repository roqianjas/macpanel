import { useMemo, useState } from "react";
import { Eraser, Lock, ScanEye } from "lucide-react";
import { errText, post } from "../../api";
import { fmtBytes } from "../../lib";
import { useT } from "../../i18n";
import { trackJob } from "../../jobs";
import { Alert, Card, ConfirmDialog } from "../../ui";
import { FilterTabs, ListSearch, Pager, useListView } from "../list/ListTools";
import { DETECTED_FACETS, SMALL_BYTES, canPick, pickableIds, whyKey, sizeLabel, sumBytes, type RunResult, type ScanRow } from "./model";

const SRC: Record<string, "system.cl.det.srcApple" | "system.cl.det.srcApp" | "system.cl.det.srcOther"> = { apple: "system.cl.det.srcApple", app: "system.cl.det.srcApp", lainnya: "system.cl.det.srcOther" } as const;

interface Review {
  ids: string[];
  preview: RunResult;
}

export function DetectedSection({ rows, onDone }: { rows: ScanRow[]; onDone: (output: string) => void }) {
  const t = useT();
  const [showSmall, setShowSmall] = useState(false);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [review, setReview] = useState<Review | null>(null);
  const [ack, setAck] = useState(false);
  const small = rows.filter((r) => r.bytes < SMALL_BYTES);
  const shown = useMemo(() => (showSmall ? rows : rows.filter((r) => r.bytes >= SMALL_BYTES)), [rows, showSmall]);
  const facets = DETECTED_FACETS.map((f) => ({ ...f, label: t(`system.cl.det.f.${f.id}`) }));
  const lv = useListView(shown, (r) => `${r.label} ${r.loc ?? ""} ${r.cls ?? ""} ${r.src ?? ""}`, 8, facets);
  const visibleIds = new Set(shown.map((r) => r.id));
  const sel = [...picked].filter((id) => visibleIds.has(id));
  const selBytes = sumBytes(rows, sel);

  const toggle = (id: string) =>
    setPicked((p) => {
      const n = new Set(p);
      if (n.has(id)) {
        n.delete(id);
      } else {
        n.add(id);
      }
      return n;
    });

  const openReview = async () => {
    setErr("");
    setBusy(true);
    try {
      const r = await post<{ preview: RunResult }>("/api/clean/preview", { ids: sel });
      setAck(false);
      setReview({ ids: sel, preview: r.preview });
    } catch (e) {
      setErr(errText(e));
    } finally {
      setBusy(false);
    }
  };

  const needsAck = !!review && review.ids.some((id) => rows.find((r) => r.id === id)?.cls === "tinjau");

  return (
    <Card
      title={
        <span className="with-icon">
          <ScanEye aria-hidden="true" size={18} /> {t("system.cl.det.title")}
        </span>
      }
    >
      <p className="small muted">{t("system.cl.det.hint")}</p>
      <FilterTabs lv={lv} label={t("system.cl.det.filter")} />
      <ListSearch lv={lv} label={t("system.cl.det.search")} hint={t("system.cl.det.searchHint")} minItems={1} />
      {small.length > 0 && (
        <label className="check">
          <input type="checkbox" checked={showSmall} onChange={(e) => setShowSmall(e.target.checked)} />
          {t("system.cl.det.showSmall", { n: small.length, size: fmtBytes(small.reduce((a, r) => a + r.bytes, 0)) })}
        </label>
      )}
      {lv.filteredCount === 0 ? (
        <p className="muted">{lv.searching || !showSmall ? t("system.cl.det.noneMatch") : t("system.cl.det.none")}</p>
      ) : (
        <ul className="cgrid">
          {lv.view.rows.map((r) => {
            const locked = !canPick(r);
            return (
              <li key={r.id} className="crow">
                <label className={`clean-pick ${locked ? "is-locked" : ""}`}>
                  <input type="checkbox" checked={picked.has(r.id)} disabled={locked} onChange={() => toggle(r.id)} aria-label={t("system.cl.pick", { label: r.label })} />
                  <span>
                    <span className="clean-name mono">{r.label}</span>
                    <span className="small muted block">
                      {locked && <Lock aria-hidden="true" size={12} />} {t(`system.cl.det.c.${r.cls}`)} · {t(SRC[r.src ?? "lainnya"])} · {r.loc}
                    </span>
                    <span className="small muted block">{t(`system.cl.det.w.${whyKey(r)}`)}</span>
                  </span>
                </label>
                <span className="mono csize">{sizeLabel(r)}</span>
              </li>
            );
          })}
        </ul>
      )}
      <Pager lv={lv} />
      {err && <Alert>{err}</Alert>}
      <div className="sticky-bar">
        <span className="small" aria-live="polite">{sel.length === 0 ? t("system.cl.noneSel") : t("system.cl.sel", { n: sel.length, size: fmtBytes(selBytes) })}</span>
        <span className="row-end">
          <button className="btn" onClick={() => setPicked(new Set(pickableIds(lv.filtered)))} disabled={busy || pickableIds(lv.filtered).length === 0}>
            {t("system.cl.det.selFiltered", { n: pickableIds(lv.filtered).length })}
          </button>
          <button className="btn" onClick={() => setPicked(new Set())} disabled={busy || picked.size === 0}>{t("system.cl.det.clear")}</button>
          <button className="btn btn-primary" onClick={openReview} disabled={busy || sel.length === 0}>
            <Eraser aria-hidden="true" size={16} /> {t("system.cl.review")}
          </button>
        </span>
      </div>
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
              {needsAck && (
                <>
                  <div className="alert alert-warn" role="alert">{t("system.cl.det.warn")}</div>
                  <label className="check">
                    <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} />
                    {t("system.cl.det.ack")}
                  </label>
                </>
              )}
              <p className="small muted">{t("system.cl.reauth")}</p>
            </div>
          }
          onConfirm={async () => {
            if (needsAck && !ack) {
              throw new Error(t("system.cl.det.ackNeeded"));
            }
            const r = await post<{ job: number }>("/api/clean/run", { ids: review.ids, review_ack: needsAck ? ack : true });
            setBusy(true);
            trackJob(r.job, t("system.cl.job.clean"), (j) => {
              setBusy(false);
              setPicked(new Set());
              onDone(j.output);
            });
          }}
        />
      )}
    </Card>
  );
}
