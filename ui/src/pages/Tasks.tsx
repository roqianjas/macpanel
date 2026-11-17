import { useState } from "react";
import { Plus, RotateCw, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, Tabs, statusTone, useLoad } from "../ui";

interface Task {
  name: string;
  status: string;
  pid: string;
  exit: string;
  schedule: string;
}

export default function Tasks() {
  const [kind, setKind] = useState<"worker" | "job">("worker");
  const { data, error, loading, reload } = useLoad(() => get<{ tasks: Task[] }>(`/api/tasks/${kind}`), [kind]);
  const [adding, setAdding] = useState(false);
  const [rm, setRm] = useState<string | null>(null);
  const [log, setLog] = useState<string | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const noun = kind === "worker" ? "worker" : "cron job";

  const restart = async (n: string) => {
    setNote(null);
    try {
      await post(`/api/tasks/${kind}/${n}/restart`, {});
      setNote({ tone: "ok", text: `${n} dimulai ulang.` });
      setTimeout(reload, 600);
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <>
      <PageHead
        title="Worker dan cron"
        desc="Pengganti Supervisor dan crontab, dijalankan launchd. Berjalan setelah Anda login di Mac mini (FileVault berhenti di layar unlock sehabis reboot)."
        action={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            Tambah {noun}
          </button>
        }
      />
      <Tabs
        label="Jenis tugas"
        value={kind}
        onChange={(k) => {
          setKind(k as "worker" | "job");
          setNote(null);
        }}
        tabs={[
          { id: "worker", label: "Worker (queue, daemon)" },
          { id: "job", label: "Cron (terjadwal)" },
        ]}
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Card>
        <State loading={loading && !data} error={error} empty={!!data && data.tasks.length === 0} emptyText={`Belum ada ${noun}.`} onRetry={reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Nama</th>
                  <th scope="col">Status</th>
                  <th scope="col">PID</th>
                  <th scope="col">Exit terakhir</th>
                  <th scope="col">Jadwal dan perintah</th>
                  <th scope="col">
                    <span className="sr">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {data?.tasks.map((t) => (
                  <tr key={t.name}>
                    <td className="mono">{t.name}</td>
                    <td>
                      <Chip tone={statusTone(t.status)}>{t.status}</Chip>
                    </td>
                    <td className="mono">{t.pid}</td>
                    <td className="mono">{t.exit}</td>
                    <td className="mono cell-trunc" title={t.schedule}>
                      {t.schedule}
                    </td>
                    <td className="cell-end">
                      <div className="row">
                        <button className="btn" onClick={() => setLog(t.name)}>
                          Log<span className="sr"> {t.name}</span>
                        </button>
                        <button className="btn btn-icon" onClick={() => restart(t.name)} aria-label={`Mulai ulang ${t.name}`}>
                          <RotateCw size={18} aria-hidden="true" />
                        </button>
                        <button className="btn btn-danger-ghost btn-icon" onClick={() => setRm(t.name)} aria-label={`Hapus ${t.name}`}>
                          <Trash2 size={18} aria-hidden="true" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </State>
      </Card>
      {adding && <AddTask kind={kind} onClose={() => setAdding(false)} onDone={() => { setAdding(false); reload(); }} />}
      {rm && (
        <ConfirmDialog
          title={`Hapus ${noun} ${rm}`}
          word={rm}
          verb="Hapus"
          body={<p>Proses dihentikan dan LaunchAgent dihapus. Log tetap ada.</p>}
          onConfirm={async () => {
            await post(`/api/tasks/${kind}/${rm}/remove`, { confirm: rm });
            reload();
          }}
          onClose={() => setRm(null)}
        />
      )}
      {log && <LogDialog kind={kind} name={log} onClose={() => setLog(null)} />}
    </>
  );
}

function LogDialog({ kind, name, onClose }: { kind: string; name: string; onClose: () => void }) {
  const { data, error, loading, reload } = useLoad(() => get<{ log: string }>(`/api/tasks/${kind}/${name}/logs?n=100`));
  return (
    <Modal title={`Log ${name}`} onClose={onClose} wide>
      <State loading={loading && !data} error={error} empty={!!data && !data.log.trim()} emptyText="Log masih kosong." onRetry={reload}>
        <pre className="logbox mono">{data?.log}</pre>
      </State>
      <div className="row-end">
        <button className="btn" onClick={reload}>
          Muat ulang
        </button>
        <button className="btn" onClick={onClose}>
          Tutup
        </button>
      </div>
    </Modal>
  );
}

function AddTask({ kind, onClose, onDone }: { kind: "worker" | "job"; onClose: () => void; onDone: () => void }) {
  const [f, setF] = useState({ name: "", dir: "", cmd: "", php: "", procs: "1", when: "every", every: "5m", at: "03:00" });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const up = (k: keyof typeof f, v: string) => setF((p) => ({ ...p, [k]: v }));
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const body: Record<string, unknown> = { name: f.name, dir: f.dir, cmd: f.cmd };
      if (f.php) body.php = f.php;
      if (kind === "worker") body.procs = Number(f.procs);
      else if (f.when === "every") body.every = f.every;
      else body.at = f.at;
      await post(`/api/tasks/${kind}`, body);
      onDone();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={kind === "worker" ? "Tambah worker" : "Tambah cron job"} onClose={onClose} wide>
      <form onSubmit={submit} className="stack">
        <div className="grid2">
          <Field label="Nama" hint="Huruf kecil, angka, tanda hubung.">
            {(id, d) => <input id={id} aria-describedby={d} data-autofocus className="mono" value={f.name} onChange={(e) => up("name", e.target.value.toLowerCase())} required pattern="[a-z][a-z0-9\-]{0,39}" />}
          </Field>
          <Field label="Folder kerja" hint="Di bawah ~/MacPanel/wwwroot.">
            {(id, d) => <input id={id} aria-describedby={d} className="mono" value={f.dir} onChange={(e) => up("dir", e.target.value)} required placeholder="~/MacPanel/wwwroot/org/proyek" />}
          </Field>
        </div>
        <Field label="Perintah" hint="Program yang diizinkan: php, node, python3, npm run, pnpm run, composer run-script. Tanpa kutip, pipa, atau $; kode sebaris seperti php -r ditolak.">
          {(id, d) => <input id={id} aria-describedby={d} className="mono" value={f.cmd} onChange={(e) => up("cmd", e.target.value)} required placeholder="php artisan queue:work --tries=3" />}
        </Field>
        <div className="grid2">
          <Field label="Versi PHP (opsional)" hint="Untuk perintah php, memakai biner versi ini.">
            {(id, d) => (
              <select id={id} aria-describedby={d} value={f.php} onChange={(e) => up("php", e.target.value)}>
                <option value="">(bawaan)</option>
                {["8.2", "8.3", "8.4", "8.5"].map((v) => (
                  <option key={v}>{v}</option>
                ))}
              </select>
            )}
          </Field>
          {kind === "worker" ? (
            <Field label="Jumlah proses">{(id) => <input id={id} type="number" min={1} max={8} value={f.procs} onChange={(e) => up("procs", e.target.value)} />}</Field>
          ) : (
            <>
              <Field label="Jadwal">
                {(id) => (
                  <select id={id} value={f.when} onChange={(e) => up("when", e.target.value)}>
                    <option value="every">Setiap beberapa menit/jam</option>
                    <option value="at">Setiap hari pada jam tertentu</option>
                  </select>
                )}
              </Field>
              {f.when === "every" ? (
                <Field label="Interval" hint="Contoh: 5m (menit) atau 2h (jam).">
                  {(id, d) => <input id={id} aria-describedby={d} className="mono" value={f.every} onChange={(e) => up("every", e.target.value)} required pattern="[1-9][0-9]{0,2}[mh]" />}
                </Field>
              ) : (
                <Field label="Jam (24 jam)" hint="Zona waktu Mac mini.">
                  {(id, d) => <input id={id} aria-describedby={d} type="time" value={f.at} onChange={(e) => up("at", e.target.value)} required />}
                </Field>
              )}
            </>
          )}
        </div>
        {err && <Alert>{err}</Alert>}
        <p className="small muted">Aksi ini meminta kata sandi ulang karena menjalankan kode proyek terus-menerus.</p>
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || !f.name || !f.dir || !f.cmd}>
            {busy ? "Membuat…" : "Buat"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
