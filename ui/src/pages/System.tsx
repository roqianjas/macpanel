import { useState } from "react";
import { Skull } from "lucide-react";
import { get, post } from "../api";
import { fmtBytes } from "../lib";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, Tabs, useLoad } from "../ui";

const TABS = [
  { id: "keamanan", label: "Keamanan" },
  { id: "proses", label: "Proses" },
  { id: "disk", label: "Disk" },
  { id: "docker", label: "Docker" },
];

export default function System() {
  const [tab, setTab] = useState("keamanan");
  return (
    <>
      <PageHead title="Sistem" desc="Status keamanan, proses, pemakaian disk, dan Docker. Halaman ini hanya membaca, kecuali menghentikan proses pengembangan." />
      <Tabs tabs={TABS} value={tab} onChange={setTab} label="Bagian sistem" />
      <div role="tabpanel" aria-labelledby={`tab-${tab}`} className="tabpanel">
        {tab === "keamanan" ? <Security /> : tab === "proses" ? <Procs /> : tab === "disk" ? <Disk /> : <Docker />}
      </div>
    </>
  );
}

interface Port {
  addr: string;
  port: number;
  proc: string;
  exposed: boolean;
}

function Security() {
  const q = useLoad(() => get<{ firewall: string; stealth: string; ssh: { password_auth: string; root_login: string; kbd_interactive: string }; ports: Port[] }>("/api/sys/security"));
  const d = q.data;
  const fwOn = !!d && /enabled/i.test(d.firewall);
  const sshOk = !!d && d.ssh.password_auth === "no" && !["yes"].includes(d.ssh.root_login);
  return (
    <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
      {d && (
        <div className="stack">
          <Card title="Pengerasan">
            <ul className="list">
              <li className="list-row"><span>Firewall aplikasi macOS</span><Chip tone={fwOn ? "ok" : "warn"}>{fwOn ? "Aktif" : "Mati"}</Chip></li>
              <li className="list-row"><span>Login SSH dengan kata sandi</span><Chip tone={d.ssh.password_auth === "no" ? "ok" : "bad"}>{d.ssh.password_auth === "no" ? "Dimatikan (hanya kunci)" : d.ssh.password_auth}</Chip></li>
              <li className="list-row"><span>Login SSH sebagai root</span><Chip tone={d.ssh.root_login === "no" ? "ok" : "warn"}>{d.ssh.root_login}</Chip></li>
              <li className="list-row"><span>Keyboard-interactive SSH</span><Chip tone={d.ssh.kbd_interactive === "no" ? "ok" : "warn"}>{d.ssh.kbd_interactive}</Chip></li>
            </ul>
            <p className="small muted">Mengubah firewall atau SSH butuh <span className="mono">sudo</span>, jadi sengaja tidak bisa dilakukan dari panel. Jalankan sendiri di Terminal. {sshOk ? "" : "Periksa baris yang bertanda kuning atau merah."}</p>
          </Card>
          <Card title="Port yang mendengarkan">
            <p className="small muted">Baris "terbuka" artinya tidak terikat ke loopback. Untuk nginx itu wajar karena akses dibatasi tailnet lewat konfigurasi; layanan lain sebaiknya hanya di 127.0.0.1.</p>
            <div className="table-wrap">
              <table className="table">
                <caption className="sr">Port TCP yang mendengarkan</caption>
                <thead><tr><th scope="col">Port</th><th scope="col">Alamat</th><th scope="col">Proses</th><th scope="col">Status</th></tr></thead>
                <tbody>
                  {d.ports.map((p, i) => (
                    <tr key={i}>
                      <td className="mono">{p.port}</td>
                      <td className="mono">{p.addr}</td>
                      <td>{p.proc}</td>
                      <td><Chip tone={p.exposed ? "warn" : "ok"}>{p.exposed ? "Terbuka" : "Lokal"}</Chip></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </div>
      )}
    </State>
  );
}

interface Proc {
  pid: number;
  name: string;
  cpu: number;
  mem: number;
  rss_mb: number;
  etime: string;
  killable: boolean;
}

function Procs() {
  const q = useLoad(() => get<{ procs: Proc[]; total: number }>("/api/sys/procs"));
  const [kill, setKill] = useState<Proc | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  return (
    <Card title="Proses milik Anda (40 teratas menurut CPU)" action={<button className="btn" onClick={q.reload}>Segarkan</button>}>
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="table-wrap">
          <table className="table">
            <caption className="sr">Proses</caption>
            <thead><tr><th scope="col">PID</th><th scope="col">Nama</th><th scope="col">CPU %</th><th scope="col">RAM</th><th scope="col">Jalan</th><th scope="col"><span className="sr">Aksi</span></th></tr></thead>
            <tbody>
              {q.data?.procs.map((p) => (
                <tr key={p.pid}>
                  <td className="mono">{p.pid}</td>
                  <td>{p.name}</td>
                  <td className="mono">{p.cpu.toFixed(1)}</td>
                  <td className="mono">{fmtBytes(p.rss_mb * 1048576)}</td>
                  <td className="mono">{p.etime}</td>
                  <td className="cell-end">
                    {p.killable ? (
                      <button className="btn btn-danger-ghost btn-sm" onClick={() => setKill(p)}>
                        <Skull size={16} aria-hidden="true" />Hentikan<span className="sr"> {p.name} {p.pid}</span>
                      </button>
                    ) : (
                      <span className="small muted">dikelola sistem</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="small muted">Hanya proses pengembangan (node, php, python, go, composer, dan sejenisnya) yang bisa dihentikan, dengan SIGTERM. nginx, database, panel, dan proses sistem dikecualikan; kelola layanan lewat halaman Aplikasi.</p>
      </State>
      {kill && (
        <ConfirmDialog
          title={`Hentikan ${kill.name} (${kill.pid})`}
          word={kill.name}
          verb="Hentikan proses"
          body={<p>Mengirim SIGTERM. Pekerjaan yang belum disimpan oleh proses itu bisa hilang.</p>}
          onConfirm={async () => {
            const r = await post<{ job?: number; output?: string }>("/api/sys/kill", { pid: kill.pid });
            setNote({ tone: "ok", text: r.output ?? "Perintah dikirim." });
            setTimeout(q.reload, 800);
          }}
          onClose={() => setKill(null)}
        />
      )}
    </Card>
  );
}

function Disk() {
  const q = useLoad(() => get<{ rows: { name: string; kb: number }[]; total: number; free: number }>("/api/sys/disk"));
  const max = Math.max(1, ...(q.data?.rows.map((r) => r.kb) ?? [1]));
  return (
    <Card title="Pemakaian disk per folder" action={<button className="btn" onClick={q.reload}>Hitung ulang</button>}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {q.data && (
          <>
            <p className="small muted">Bebas {fmtBytes(q.data.free)} dari {fmtBytes(q.data.total)}. Perhitungan memakai <span className="mono">du</span> dan bisa memakan waktu beberapa detik.</p>
            <ul className="bars">
              {q.data.rows.map((r) => (
                <li key={r.name} className="bar-row">
                  <span className="mono bar-name">{r.name}</span>
                  <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (r.kb / max) * 100)}%` }} /></span>
                  <span className="mono bar-val">{fmtBytes(r.kb * 1024)}</span>
                </li>
              ))}
            </ul>
          </>
        )}
      </State>
    </Card>
  );
}

function Docker() {
  const q = useLoad(() => get<{ installed: boolean; running?: boolean; error?: string; containers?: { name: string; image: string; status: string; ports: string }[] }>("/api/sys/docker"));
  const d = q.data;
  return (
    <Card title="Docker">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {d && !d.installed && <p className="muted">Docker belum terpasang. Sesuai kebijakan, Docker hanya dipasang bila sebuah proyek memerlukannya; semua layanan lain berjalan native.</p>}
        {d?.installed && !d.running && <Alert>Docker terpasang tetapi tidak berjalan{d.error ? `: ${d.error}` : "."}</Alert>}
        {d?.running && ((d.containers ?? []).length === 0 ? <p className="muted">Tidak ada kontainer yang berjalan.</p> : (
          <ul className="list">{d.containers!.map((c) => <li key={c.name} className="list-row"><div><p className="mono">{c.name}</p><p className="small muted">{c.image} · {c.status}{c.ports ? ` · ${c.ports}` : ""}</p></div></li>)}</ul>
        ))}
        <p className="small muted">Panel hanya menampilkan status Docker (baca saja).</p>
      </State>
    </Card>
  );
}
