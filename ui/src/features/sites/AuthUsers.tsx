import { useState } from "react";
import { Trash2 } from "lucide-react";
import { errText, get, post } from "../../api";
import { Alert, Card, ConfirmDialog, Field, State, useLoad } from "../../ui";

export function AuthUsers({ name, enabled }: { name: string; enabled: boolean }) {
  const q = useLoad(() => get<{ users: string[] }>(`/api/sites/${name}/auth`), [name]);
  const [user, setUser] = useState("");
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [rm, setRm] = useState<string | null>(null);
  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await post(`/api/sites/${name}/auth/users`, { user, password: pw });
      setUser("");
      setPw("");
      q.reload();
    } catch (x) {
      setErr(errText(x));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Pengguna password dasar">
      <p className="small muted">{enabled ? "Password dasar aktif untuk situs ini." : "Password dasar belum aktif; aktifkan di kartu di atas setelah menambah pengguna."} Kata sandi disimpan sebagai hash dan tidak bisa dilihat lagi. Menambah dengan nama yang sudah ada mengganti kata sandinya.</p>
      <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && q.data.users.length === 0} emptyText="Belum ada pengguna." onRetry={q.reload}>
        <ul className="list">
          {q.data?.users.map((u) => (
            <li key={u} className="list-row">
              <span className="mono">{u}</span>
              <button className="btn btn-danger-ghost" onClick={() => setRm(u)}>
                <Trash2 size={18} aria-hidden="true" />
                Hapus<span className="sr"> {u}</span>
              </button>
            </li>
          ))}
        </ul>
      </State>
      <form onSubmit={add} className="stack">
        <div className="grid2">
          <Field label="Nama pengguna">{(id) => <input id={id} className="mono" placeholder="nama pengguna" maxLength={32} value={user} onChange={(e) => setUser(e.target.value)} autoComplete="off" required pattern="[A-Za-z0-9._\-]{1,32}" />}</Field>
          <Field label="Kata sandi" hint="8-128 karakter">{(id, dd) => <input id={id} aria-describedby={dd} type="password" placeholder="8-128 karakter" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="new-password" minLength={8} maxLength={128} required />}</Field>
        </div>
        {err && <Alert>{err}</Alert>}
        <div>
          <button className="btn" disabled={busy || !user || pw.length < 8}>{busy ? "Menyimpan…" : "Tambah atau ganti"}</button>
        </div>
      </form>
      {rm && (
        <ConfirmDialog
          title={`Hapus pengguna ${rm}`}
          word={rm}
          verb="Hapus pengguna"
          body={<p>Pengguna ini tidak bisa masuk lagi. Meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            await post(`/api/sites/${name}/auth/users/${rm}/remove`, {});
            q.reload();
          }}
          onClose={() => setRm(null)}
        />
      )}
    </Card>
  );
}
