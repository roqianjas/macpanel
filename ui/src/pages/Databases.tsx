import { useState } from "react";
import { Archive, Plus, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";

type Engine = "mysql" | "pgsql";
const ENGINE_LABEL: Record<Engine, string> = { mysql: "MySQL 8.4", pgsql: "PostgreSQL 17" };

export default function Databases() {
  const { data, error, loading, reload } = useLoad(() => get<{ databases: Record<Engine, string[]>; system: string[] }>("/api/db"));
  const isSys = (n: string) => (data?.system ?? ["dev"]).includes(n);
  const [adding, setAdding] = useState(false);
  const [dropping, setDropping] = useState<{ engine: Engine; name: string } | null>(null);
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
