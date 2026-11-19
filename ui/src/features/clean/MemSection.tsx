import { useState } from "react";
import { Copy, Gauge, MemoryStick, Power } from "lucide-react";
import { get, post } from "../../api";
import { fmtBytes } from "../../lib";
import { t as tr, useT } from "../../i18n";
import { Card, Chip, ConfirmDialog, State, useLoad } from "../../ui";
import { appUsedMb, pressure, PURGE_COMMAND, type MemSuggestion, type MemView } from "./model";

// Ikon: MemoryStick = memori, Gauge = tekanan, Power = matikan layanan opsional, Copy = salin perintah.

const mb = (v: number | undefined) => (v == null ? tr("fmt.na") : fmtBytes(v * 1048576));

export default function MemSection() {
  const t = useT();
  const q = useLoad(() => get<MemView>("/api/mem"));
  const [stop, setStop] = useState<MemSuggestion | null>(null);
  const [note, setNote] = useState("");
  const d = q.data;
  const p = pressure(d?.mem.pressure_free_pct);
  const used = d ? appUsedMb(d.mem) : null;
  const max = Math.max(1, ...(d?.groups.map((x) => x.mb) ?? [1]));
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(PURGE_COMMAND);
      setNote(t("system.copied", { v: PURGE_COMMAND }));
    } catch {
      setNote(t("system.clipFail"));
    }
  };
  return (
    <div className="stack">
      <Card
        title={
          <span className="with-icon">
            <MemoryStick aria-hidden="true" size={18} /> {t("system.mem.title")}
          </span>
        }
        action={<button className="btn" onClick={q.reload}>{t("system.refresh")}</button>}
      >
        <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
          {d && (
            <>
              <p className="with-icon">
                <Gauge aria-hidden="true" size={16} /> {t("system.mem.pressure")} <Chip tone={p.tone}>{p.label}{d.mem.pressure_free_pct != null ? ` ${t("system.mem.pctFree", { pct: d.mem.pressure_free_pct })}` : ""}</Chip>
              </p>
              <ul className="list">
                <li className="list-row"><span>{t("system.mem.total")}</span><span className="mono">{mb(d.mem.total_mb)}</span></li>
                <li className="list-row"><span>{t("system.mem.used")}</span><span className="mono">{used == null ? t("fmt.na") : mb(used)}</span></li>
                <li className="list-row"><span>{t("system.mem.cache")}</span><span className="mono">{mb(d.mem.inactive_mb)}</span></li>
                <li className="list-row"><span>{t("system.mem.swap")}</span><span className="mono">{d.mem.swap_used_mb == null ? t("fmt.na") : mb(d.mem.swap_used_mb)}</span></li>
              </ul>
              <p className="small muted">{t("system.mem.note")}</p>
            </>
          )}
        </State>
      </Card>

      {d && (
        <>
          <Card title={t("system.mem.top")}>
            <ul className="bars">
              {d.groups.map((x) => (
                <li key={x.label} className="bar-row">
                  <span className="bar-name">{x.label}{x.procs > 1 ? ` ${t("system.mem.procs", { n: x.procs })}` : ""}</span>
                  <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (x.mb / max) * 100)}%` }} /></span>
                  <span className="mono bar-val">{mb(x.mb)}</span>
                </li>
              ))}
            </ul>
            <p className="small muted">{t("system.mem.topNote")}</p>
          </Card>

          <Card title={t("system.mem.sugg")}>
            {d.suggestions.length === 0 ? (
              <p className="muted">{t("system.mem.suggNone")}</p>
            ) : (
              <ul className="list">
                {d.suggestions.map((s) => (
                  <li key={s.id} className="list-row clean-row">
                    <div>
                      <p>{s.label}{s.mb != null ? <span className="mono"> · ±{mb(s.mb)}</span> : null}</p>
                      <p className="small muted">{s.text}</p>
                    </div>
                    <button className="btn" onClick={() => setStop(s)}><Power aria-hidden="true" size={16} /> {t("system.mem.off")}</button>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card title={t("system.mem.purge")}>
            <p className="small muted">{d.purge_note} {t("system.mem.purgeNote")}</p>
            <p className="small muted">{t("system.mem.purgeRoot")}</p>
            <div className="row-end">
              <code className="mono">{PURGE_COMMAND}</code>
              <button className="btn" onClick={copy}><Copy aria-hidden="true" size={16} /> {t("system.copyBtn")}</button>
            </div>
            {note && <p className="small muted" role="status">{note}</p>}
          </Card>
        </>
      )}

      {stop && (
        <ConfirmDialog
          title={t("system.mem.offTitle", { name: stop.label })}
          word={stop.id}
          verb={t("system.mem.offVerb")}
          onClose={() => setStop(null)}
          body={<p>{stop.text} {t("system.mem.offBody")}</p>}
          onConfirm={async () => {
            await post(`/api/services/${stop.id}/stop`, {});
            q.reload();
          }}
        />
      )}
    </div>
  );
}
