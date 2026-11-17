import { useState } from "react";
import { Copy, Eye, KeyRound } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, ConfirmDialog, Modal, State, useLoad } from "../ui";

type Creds = Record<string, string>;

export default function DbAccess({ engine, name, onClose }: { engine: string; name: string; onClose: () => void }) {
  const q = useLoad(() => get<{ users: { user: string; host: string }[]; cred: boolean; rotated: number }>(`/api/db/${engine}/${name}/users`), [engine, name]);
  const [creds, setCreds] = useState<Creds | null>(null);
  const [show, setShow] = useState(false);
  const [err, setErr] = useState("");
  const [note, setNote] = useState("");
  const [rotate, setRotate] = useState(false);
  const load = async () => {
    setErr("");
    try {
      setCreds(await post<Creds>(`/api/db/${engine}/${name}/creds`, {}));
    } catch (x) {
      setErr(errText(x));
    }
  };
  const copy = async (v: string, what: string) => {
    try {
      await navigator.clipboard.writeText(v);
      setNote(`${what} disalin ke clipboard.`);
    } catch {
      setNote("Clipboard tidak tersedia di koneksi ini; tampilkan lalu salin manual.");
    }
  };
  return (
    <Modal title={`Akses ${name} (${engine === "mysql" ? "MySQL" : "PostgreSQL"})`} onClose={onClose} wide>
      <div className="stack">
        <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
          <ul className="list">
            {q.data?.users.map((u, i) => (
              <li key={i} className="list-row">
                <div>
                  <p className="mono">{u.user}{u.host ? `@${u.host}` : ""}</p>
                  <p className="small muted">User proyek (hak penuh hanya pada database ini)</p>
                </div>
              </li>
            ))}
          </ul>
          <p className="small muted">Kata sandi terakhir diganti: {q.data?.rotated ? fmtTime(q.data.rotated) : "-"}. Kredensial disimpan di <span className="mono">~/.config/macminiops/projects/</span> dengan izin 0600.</p>
        </State>
        {err && <Alert>{err}</Alert>}
        {note && <Alert tone="ok">{note}</Alert>}
        {creds ? (
          <dl className="kv">
            {Object.entries(creds).map(([k, v]) => (
              <div key={k} className="kv-row">
                <dt className="mono small">{k}</dt>
                <dd className="mono">{k === "DB_PASSWORD" && !show ? "••••••••••••" : v}</dd>
                <button className="btn btn-ghost btn-icon" onClick={() => copy(v, k)} aria-label={`Salin ${k}`}>
                  <Copy size={16} aria-hidden="true" />
                </button>
              </div>
            ))}
          </dl>
        ) : null}
        <div className="row-end">
          {creds && (
            <button className="btn" onClick={() => setShow((s) => !s)}>
              <Eye size={18} aria-hidden="true" />
              {show ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"}
            </button>
          )}
          {!creds && (
            <button className="btn" onClick={load}>
              <Eye size={18} aria-hidden="true" />
              Lihat kredensial
            </button>
          )}
          <button className="btn btn-danger-ghost" onClick={() => setRotate(true)}>
            <KeyRound size={18} aria-hidden="true" />
            Ganti kata sandi
          </button>
        </div>
        <p className="small muted">Melihat kredensial dan mengganti sandi meminta kata sandi ulang dan dicatat di audit (tanpa isi). Setelah sandi diganti, perbarui <span className="mono">.env</span> aplikasi yang memakainya.</p>
      </div>
      {rotate && (
        <ConfirmDialog
          title={`Ganti kata sandi ${name}`}
          word={name}
          verb="Ganti kata sandi"
          body={<p>Kata sandi lama langsung tidak berlaku; aplikasi yang memakainya akan gagal konek sampai .env diperbarui.</p>}
          onConfirm={async () => {
            await post(`/api/db/${engine}/${name}/passwd`, { confirm: name });
            setCreds(null);
            setShow(false);
            q.reload();
            setNote("Kata sandi diganti. Klik Lihat kredensial untuk melihat yang baru.");
          }}
          onClose={() => setRotate(false)}
        />
      )}
    </Modal>
  );
}
