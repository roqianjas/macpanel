import { fmtTime, get } from "../api";
import Account from "./Account";
import { Card, Chip, PageHead, State, statusTone, useLoad } from "../ui";

interface Row {
  at: number;
  user: string;
  action: string;
  detail: string;
  ip: string;
}

export default function Activity() {
  const audit = useLoad(() => get<{ rows: Row[] }>("/api/audit"));
  const users = useLoad(() => get<{ users: { name: string; role: string; disabled: boolean; created: number }[]; howto: string }>("/api/users"));
  const jobs = useLoad(() => get<{ jobs: { id: number; at: number; action: string; status: string }[] }>("/api/jobs"));
  return (
    <>
      <PageHead title="Aktivitas" desc="Jejak audit berantai: setiap login, percobaan gagal, dan perubahan tercatat. Verifikasi integritas dengan: panel audit verify." />
      <Account />
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
      {!users.error && (
        <Card title="Pengguna">
          <State loading={users.loading && !users.data} error="" onRetry={users.reload}>
            <ul className="list">
              {users.data?.users.map((u) => (
                <li key={u.name} className="list-row">
                  <div>
                    <p className="mono">{u.name}</p>
                    <p className="small muted">Dibuat {fmtTime(u.created)}</p>
                  </div>
                  <div className="row">
                    {u.disabled && <Chip tone="warn">Nonaktif</Chip>}
                    <Chip tone={u.role === "admin" ? "info" : "mute"}>{u.role === "admin" ? "Admin" : "Hanya-baca"}</Chip>
                  </div>
                </li>
              ))}
            </ul>
            <p className="small muted">{users.data?.howto} Kata sandi dan kunci TOTP sengaja hanya dibuat di terminal, tidak lewat web.</p>
          </State>
        </Card>
      )}
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
