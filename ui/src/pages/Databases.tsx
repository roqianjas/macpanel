import { useState } from "react";
import { Archive, ArchiveRestore, KeyRound, Plus, Trash2 } from "lucide-react";
import { errText, fmtTime, get, post, upload } from "../api";
import { trackJob } from "../jobs";
import DbConfig from "./DbConfig";
import DbAccess from "./DbAccess";
import RedisBrowser from "./RedisBrowser";
import { fmtBytes } from "../lib";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";

type Engine = "mysql" | "pgsql";
const ENGINE_LABEL: Record<Engine, string> = { mysql: "MySQL 8.4", pgsql: "PostgreSQL 17" };

export default function Databases() {
  const { data, error, loading, reload } = useLoad(() => get<{ databases: Record<Engine, string[]>; system: string[] }>("/api/db"));
  const isSys = (n: string) => (data?.system ?? ["dev"]).includes(n);
  const [adding, setAdding] = useState(false);
  const [dropping, setDropping] = useState<{ engine: Engine; name: string } | null>(null);
  const [restoring, setRestoring] = useState<{ engine: Engine; name: string } | null>(null);
  const [access, setAccess] = useState<{ engine: Engine; name: string } | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [busy, setBusy] = useState("");

  const dump = async (engine: Engine, name: string) => {
    setBusy(`${engine}/${name}`);
    setNote(null);
    try {
      await post(`/api/db/${engine}/${name}/dump`, {});
      setNote({ tone: "ok", text: `Dump ${name} dibuat di ~/Backups/db di Mac mini. Salinan ini masih di disk yang sama, bukan backup offsite.` });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy("");
    }
  };
  const total = data ? [...data.databases.mysql, ...data.databases.pgsql].filter((n) => !isSys(n)).length : 0;
  return (
    <>
      <PageHead
        title="Database"
        desc="Satu database dan satu pengguna per proyek. Kata sandi dibuat acak dan tidak pernah ditampilkan di panel."
        action={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            Buat database
          </button>
        }
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <State loading={loading && !data} error={error} empty={!!data && total === 0} emptyText="Belum ada database proyek. Pilih “Buat database”." onRetry={reload}>
        {(["mysql", "pgsql"] as Engine[]).map((en) => (
          <Card key={en} title={ENGINE_LABEL[en]}>
            {(data?.databases[en] ?? []).filter((n) => !isSys(n)).length === 0 && !(data?.databases[en] ?? []).some(isSys) ? (
              <div className="state">
                <p className="muted">Belum ada database {ENGINE_LABEL[en]}.</p>
              </div>
            ) : (
              <ul className="list">
                {data!.databases[en].map((n) => (
                  <li key={n} className="list-row">
                    <span className="mono">{n}</span>
                    {isSys(n) ? (
                      <Chip tone="mute">Sistem: dikelola server</Chip>
                    ) : (
                    <div className="row">
                      <button className="btn" disabled={busy === `${en}/${n}`} onClick={() => dump(en, n)}>
                        <Archive size={18} aria-hidden="true" />
                        {busy === `${en}/${n}` ? "Membuat dump…" : "Dump"}
                        <span className="sr"> {n}</span>
                      </button>
                      <button className="btn" onClick={() => setAccess({ engine: en, name: n })}>
                        <KeyRound size={18} aria-hidden="true" />
                        Akses
                        <span className="sr"> {n}</span>
                      </button>
                      <button className="btn" onClick={() => setRestoring({ engine: en, name: n })}>
                        <ArchiveRestore size={18} aria-hidden="true" />
                        Pulihkan
                        <span className="sr"> {n}</span>
                      </button>
                      <button className="btn btn-danger-ghost" onClick={() => setDropping({ engine: en, name: n })}>
                        <Trash2 size={18} aria-hidden="true" />
                        Hapus
                        <span className="sr"> {n}</span>
                      </button>
                    </div>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Card>
        ))}
        <p className="small muted">Untuk melihat isi tabel pakai Adminer di db.lab atau DBeaver lewat tailnet. Kata sandi proyek ada di berkas ~/.config/macminiops/projects di Mac mini.</p>
      </State>
      <DbConfig />
      <RedisBrowser />
      {access && <DbAccess engine={access.engine} name={access.name} onClose={() => setAccess(null)} />}
      {restoring && (
        <RestoreDialog
          engine={restoring.engine}
          name={restoring.name}
          onClose={() => setRestoring(null)}
          onStarted={(job, file) => {
            trackJob(job, `Pulihkan ${restoring.name} dari ${file}`, (j) => {
              setNote(j.status === "ok" ? { tone: "ok", text: `${restoring.name} dipulihkan dari ${file}.` } : { tone: "bad", text: j.output || "Pemulihan gagal." });
              void reload();
            });
            setRestoring(null);
          }}
        />
      )}
      {adding && <AddDB onClose={() => setAdding(false)} onDone={() => { setAdding(false); reload(); }} />}
      {dropping && (
        <ConfirmDialog
          title={`Hapus database ${dropping.name}`}
          word={dropping.name}
          verb="Hapus database"
          body={<p>Dump otomatis dibuat ke ~/Backups/db sebelum dihapus. Database dan penggunanya hilang dari server. Meminta kode TOTP ulang.</p>}
          onConfirm={async () => {
            await post(`/api/db/${dropping.engine}/${dropping.name}/drop`, { confirm: dropping.name });
            setNote({ tone: "ok", text: `${dropping.name} dihapus; dump terakhir tersimpan di ~/Backups/db.` });
            reload();
          }}
          onClose={() => setDropping(null)}
        />
      )}
    </>
  );
}

function AddDB({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const [name, setName] = useState("");
  const [engine, setEngine] = useState<Engine>("mysql");
  const [env, setEnv] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const body: Record<string, string> = { name, engine };
      if (env) body.env = env;
      await post("/api/db", body);
      onDone();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title="Buat database" onClose={onClose}>
      <form onSubmit={submit} className="stack">
        <Field label="Nama" hint="Dipakai untuk nama database dan pengguna. Huruf kecil, angka, dan garis bawah; diawali huruf.">
          {(id, d) => <input id={id} aria-describedby={d} data-autofocus className="mono" value={name} onChange={(e) => setName(e.target.value.toLowerCase())} required pattern="[a-z][a-z0-9_]{0,31}" />}
        </Field>
        <Field label="Mesin">
          {(id) => (
            <select id={id} value={engine} onChange={(e) => setEngine(e.target.value as Engine)}>
              <option value="mysql">MySQL 8.4</option>
              <option value="pgsql">PostgreSQL 17</option>
            </select>
          )}
        </Field>
        <Field label="Tulis ke berkas .env proyek (opsional)" hint="Path berkas .env di bawah ~/Developer. Kunci DB_* ditulis di sana, bukan ditampilkan.">
          {(id, d) => <input id={id} aria-describedby={d} className="mono" value={env} onChange={(e) => setEnv(e.target.value)} placeholder="~/Developer/org/proyek/.env" />}
        </Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || !name}>
            {busy ? "Membuat…" : "Buat database"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function RestoreDialog({ engine, name, onClose, onStarted }: { engine: Engine; name: string; onClose: () => void; onStarted: (job: number, file: string) => void }) {
  const { data, error, loading, reload } = useLoad(() => get<{ dumps: { file: string; at: number; size: number }[] }>(`/api/db/${engine}/${name}/dumps`));
  const [file, setFile] = useState("");
  const [up, setUp] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const dumps = data?.dumps ?? [];
  const pick = file || dumps[0]?.file || "";
  const go = async () => {
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ job: number }>(`/api/db/${engine}/${name}/restore`, { file: pick });
      onStarted(r.job, pick);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={`Pulihkan ${name}`} onClose={onClose}>
      <div className="stack">
        <p className="small">Pemulihan hanya berjalan ke database yang <strong>kosong</strong> dan tidak pernah menimpa data. Bila database ini sudah berisi, hapus isinya dulu atau pulihkan ke database baru dengan nama yang sama setelah dihapus. Meminta kode TOTP ulang.</p>
        <State loading={loading && !data} error={error} empty={!!data && dumps.length === 0} emptyText={`Belum ada dump untuk ${name}. Buat dulu lewat tombol Dump.`} onRetry={reload}>
          <Field label="Pilih dump">
            {(id) => (
              <select id={id} data-autofocus value={pick} onChange={(e) => setFile(e.target.value)}>
                {dumps.map((d) => (
                  <option key={d.file} value={d.file}>
                    {fmtTime(d.at)} ({fmtBytes(d.size)})
                  </option>
                ))}
              </select>
            )}
          </Field>
        </State>
        <Field label="Atau unggah dump baru" hint="Berkas .sql atau .sql.gz, maksimal 256 MB. Isinya diperiksa saat dipulihkan: perintah klien (shell, baca/tulis berkas) ditolak, dan dijalankan sebagai user proyek.">
          {(id, dd) => (
            <input
              id={id}
              aria-describedby={dd}
              type="file"
              accept=".sql,.gz,application/gzip,application/sql,text/plain"
              disabled={up || busy}
              onChange={async (e) => {
                const f = e.target.files?.[0];
                if (!f) return;
                setUp(true);
                setErr("");
                try {
                  const r = await upload<{ file: string }>(`/api/db/${engine}/${name}/upload`, f);
                  setFile(r.file);
                  await reload();
                } catch (x) {
                  setErr(errText(x));
                } finally {
                  setUp(false);
                  e.target.value = "";
                }
              }}
            />
          )}
        </Field>
        {up && <p className="small muted" role="status">Mengunggah…</p>}
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={onClose}>Batal</button>
          <button className="btn btn-primary" disabled={busy || up || !pick} onClick={go}>{busy ? "Memulai…" : "Pulihkan"}</button>
        </div>
      </div>
    </Modal>
  );
}
