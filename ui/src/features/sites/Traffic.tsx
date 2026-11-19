import { fmtBytes } from "../../lib";
import { get } from "../../api";
import { Card, State, useLoad } from "../../ui";
import { Stats } from "./types";
import { localeTag, useT } from "../../i18n";

export function Traffic({ name }: { name: string }) {
  const t = useT();
  const q = useLoad(() => get<Stats>(`/api/sites/${name}/stats`), [name]);
  const d = q.data;
  const max = Math.max(1, ...(d?.days?.map((x) => x.n) ?? [1]));
  return (
    <Card title={t("dev.traffic.title")} action={<button className="btn" onClick={q.reload}>{t("dev.traffic.refresh")}</button>}>
      <State loading={q.loading && !q.data} error={q.error} empty={!!d && !d.exists} emptyText={t("dev.traffic.none")} onRetry={q.reload}>
        {d?.exists && (
          <div className="stack">
            <p className="small muted">{t("dev.traffic.note", { note: d.note })}</p>
            <dl className="kpis">
              <div><dt>{t("dev.traffic.requests")}</dt><dd>{d.requests.toLocaleString(localeTag())}</dd></div>
              <div><dt>{t("dev.traffic.sent")}</dt><dd>{fmtBytes(d.bytes)}</dd></div>
              {["2xx", "3xx", "4xx", "5xx"].map((c) => <div key={c}><dt>{c}</dt><dd>{((d.status ?? {})[c] ?? 0).toLocaleString(localeTag())}</dd></div>)}
            </dl>
            <h3 className="sec-title">{t("dev.traffic.perDay")}</h3>
            <ul className="bars">
              {(d.days ?? []).map((x) => (
                <li key={x.day} className="bar-row">
                  <span className="mono bar-name">{x.day}</span>
                  <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (x.n / max) * 100)}%` }} /></span>
                  <span className="mono bar-val">{x.n}</span>
                </li>
              ))}
            </ul>
            <div className="grid2">
              <div><h3 className="sec-title">{t("dev.traffic.topPages")}</h3><ul className="list">{(d.paths ?? []).map((x) => <li key={x.path} className="list-row"><span className="mono ellip">{x.path}</span><span className="mono">{x.n}</span></li>)}</ul></div>
              <div><h3 className="sec-title">{t("dev.traffic.topIps")}</h3><ul className="list">{(d.ips ?? []).map((x) => <li key={x.ip} className="list-row"><span className="mono">{x.ip}</span><span className="mono">{x.n}</span></li>)}</ul></div>
            </div>
          </div>
        )}
      </State>
    </Card>
  );
}
