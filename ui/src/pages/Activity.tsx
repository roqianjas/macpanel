import { fmtTime, get } from "../api";
import { Card, Chip, PageHead, State, statusTone, useLoad } from "../ui";

interface Row {
  at: number;
  user: string;
  action: string;
  detail: string;
  ip: string;
}

export default function Activity() {
  const audit = useLoad(() => get<{ rows: Row[] }>("/api/audit?limit=100"));
  const jobs = useLoad(() => get<{ jobs: { id: number; at: number; action: string; status: string }[] }>("/api/jobs?limit=50"));
  return (
    <>
      <PageHead title="Aktivitas" desc="Jejak audit berantai: setiap login, percobaan gagal, dan perubahan tercatat. Verifikasi integritas dengan: panel audit verify." />
      <Card title="Audit (100 terakhir)">
        <State loading={audit.loading && !audit.data} error={audit.error} empty={!!audit.data && audit.data.rows.length === 0} emptyText="Belum ada catatan." onRetry={audit.reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Waktu</th>
                  <th scope="col">Pengguna</th>
                  <th scope="col">Aksi</th>
                  <th scope="col">Detail</th>
                  <th scope="col">IP</th>
                </tr>
              </thead>
              <tbody>
                {audit.data?.rows.map((r, i) => (
                  <tr key={i}>
                    <td className="nowrap">{fmtTime(r.at)}</td>
                    <td className="mono">{r.user || "-"}</td>
                    <td className="mono">{r.action}</td>
                    <td className="mono cell-trunc" title={r.detail}>
                      {r.detail}
                    </td>
                    <td className="mono">{r.ip}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </State>
      </Card>
      <Card title="Aksi terakhir">
        <State loading={jobs.loading && !jobs.data} error={jobs.error} empty={!!jobs.data && jobs.data.jobs.length === 0} emptyText="Belum ada aksi." onRetry={jobs.reload}>
          <ul className="list">
            {jobs.data?.jobs.map((j) => (
              <li key={j.id} className="list-row">
                <div>
                  <p className="mono">{j.action}</p>
                  <p className="small muted">{fmtTime(j.at)}</p>
                </div>
                <Chip tone={statusTone(j.status)} busy={j.status === "berjalan"}>
                  {j.status}
                </Chip>
              </li>
            ))}
          </ul>
        </State>
      </Card>
    </>
  );
}
