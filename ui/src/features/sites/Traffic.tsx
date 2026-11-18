import { fmtBytes } from "../../lib";
import { get } from "../../api";
import { Card, State, useLoad } from "../../ui";
import { Stats } from "./types";

export function Traffic({ name }: { name: string }) {
  const q = useLoad(() => get<Stats>(`/api/sites/${name}/stats`), [name]);
  const d = q.data;
  const max = Math.max(1, ...(d?.days?.map((x) => x.n) ?? [1]));
  return (
    <Card title="Statistik trafik" action={<button className="btn" onClick={q.reload}>Segarkan</button>}>
      <State loading={q.loading && !q.data} error={q.error} empty={!!d && !d.exists} emptyText="Belum ada log akses untuk situs ini." onRetry={q.reload}>
        {d?.exists && (
          <div className="stack">
            <p className="small muted">Dihitung dari log akses nginx ({d.note}). Bukan analitik pengunjung: hanya permintaan HTTP.</p>
            <dl className="kpis">
              <div><dt>Permintaan</dt><dd>{d.requests.toLocaleString("id-ID")}</dd></div>
              <div><dt>Data terkirim</dt><dd>{fmtBytes(d.bytes)}</dd></div>
              {["2xx", "3xx", "4xx", "5xx"].map((c) => <div key={c}><dt>{c}</dt><dd>{((d.status ?? {})[c] ?? 0).toLocaleString("id-ID")}</dd></div>)}
            </dl>
            <h3 className="sec-title">Permintaan per hari</h3>
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
              <div><h3 className="sec-title">Halaman teratas</h3><ul className="list">{(d.paths ?? []).map((x) => <li key={x.path} className="list-row"><span className="mono ellip">{x.path}</span><span className="mono">{x.n}</span></li>)}</ul></div>
              <div><h3 className="sec-title">IP teratas</h3><ul className="list">{(d.ips ?? []).map((x) => <li key={x.ip} className="list-row"><span className="mono">{x.ip}</span><span className="mono">{x.n}</span></li>)}</ul></div>
            </div>
          </div>
        )}
      </State>
    </Card>
  );
}
