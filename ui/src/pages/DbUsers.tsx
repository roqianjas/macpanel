import { useState } from "react";
import { Copy } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import type { PostUrl } from "../api.gen";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, State, useLoad } from "../ui";

const PRIV_LABEL: Record<string, string> = { ro: "Baca-saja", rw: "Baca-tulis", all: "Penuh" };
const ENGINE_LABEL: Record<string, string> = { mariadb: "MariaDB", mysql: "MySQL", pgsql: "PostgreSQL" };

/** Tampilkan rahasia sekali (kata sandi acak). */
export function SecretDialog({ title, label, value, note, onClose }: { title: string; label: string; value: string; note?: string; onClose: () => void }) {
  const [msg, setMsg] = useState("");
  return (
    <Modal title={title} onClose={onClose}>
      <div className="stack">
        <Alert tone="warn">Hanya tampil sekali. Simpan sekarang di pengelola kata sandi; jangan ditempel ke chat atau repo.</Alert>
        <span className="field-label">{label}</span>
        <div className="row">
          <code className="mono" style={{ wordBreak: "break-all" }}>{value}</code>
          <button
            className="btn btn-ghost btn-icon"
            aria-label={`Salin ${label}`}
            onClick={async () => {
              try {
                await navigator.clipboard.writeText(value);
                setMsg("Disalin.");
              } catch {
                setMsg("Gagal menyalin; salin manual.");
              }
            }}
          >
            <Copy size={16} aria-hidden="true" />
          </button>
        </div>
        {msg && <p className="small muted" role="status">{msg}</p>}
        {note && <p className="small muted">{note}</p>}
        <div className="row-end">
          <button className="btn btn-primary" onClick={onClose}>
            Sudah saya simpan
          </button>
        </div>
      </div>
    </Modal>
  );
}

/** Dialog ganti kata sandi: acak atau pilihan sendiri, dengan ketik ulang nama. */
export function PasswordDialog({ title, word, hint, onClose, onSubmit }: { title: string; word: string; hint?: string; onClose: () => void; onSubmit: (password: string | undefined) => Promise<void> }) {
  const [typed, setTyped] = useState("");
  const [own, setOwn] = useState(false);
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const okPw = !own || /^[A-Za-z0-9._+=:,@%^-]{12,100}$/.test(pw);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await onSubmit(own ? pw : undefined);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title={title} onClose={onClose}>
      <form className="stack" onSubmit={submit} autoComplete="off">
        {hint && <p className="small muted">{hint}</p>}
        <div className="seg" role="group" aria-label="Asal kata sandi">
          <button type="button" className={`seg-btn ${!own ? "on" : ""}`} aria-pressed={!own} onClick={() => setOwn(false)}>
            Acak (disarankan)
          </button>
          <button type="button" className={`seg-btn ${own ? "on" : ""}`} aria-pressed={own} onClick={() => setOwn(true)}>
            Pilih sendiri
          </button>
        </div>
        {own && (
          <Field label="Kata sandi baru" hint="12-100 karakter: huruf, angka, dan . _ + = : , @ % ^ - (tanpa spasi dan tanda kutip).">
            {(id, dd) => <input id={id} aria-describedby={dd} type="password" autoComplete="new-password" placeholder="kata sandi" maxLength={100} value={pw} onChange={(e) => setPw(e.target.value)} required />}
          </Field>
        )}
        <Field label={`Ketik "${word}" untuk konfirmasi`}>{(id) => <input id={id} className="mono" placeholder={word} value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" spellCheck={false} required />}</Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-danger" disabled={busy || typed !== word || !okPw}>
            {busy ? "Menyimpan…" : "Ganti kata sandi"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- user database per database ----------
interface DbUser {
  user: string;
  host: string;
  privs: string;
  primary: boolean;
  created?: number;
}
type Secret = { title: string; label: string; value: string; note?: string } | null;

export function DbUsersSection({ engine, name }: { engine: string; name: string }) {
  const base = `/api/db/${engine}/${name}/dbusers` as const;
  const q = useLoad(() => get<{ users: DbUser[]; port: number }>(base), [engine, name]);
  const [adding, setAdding] = useState(false);
  const [pwFor, setPwFor] = useState<string | null>(null);
  const [privFor, setPrivFor] = useState<DbUser | null>(null);
  const [delFor, setDelFor] = useState<string | null>(null);
  const [secret, setSecret] = useState<Secret>(null);
  const [note, setNote] = useState("");
  const port = q.data?.port ?? 0;
  return (
    <div className="stack">
      <div className="row-end" style={{ justifyContent: "space-between" }}>
        <h3 className="h3">User database</h3>
        <button className="btn" onClick={() => setAdding(true)}>
          Tambah user
        </button>
      </div>
      <p className="small muted">
        Selain user proyek, Anda dapat membuat user lain dengan hak terbatas pada database ini. Hanya dapat terhubung dari mesin ini (127.0.0.1{port ? `:${port}` : ""}). Hanya user yang dibuat dari panel yang dapat diubah atau dihapus di sini.
      </p>
      {note && <Alert tone="ok">{note}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <ul className="list">
          {q.data?.users.map((u) => (
            <li key={u.user} className="list-row">
              <div>
                <p className="mono">{u.user}</p>
                <p className="small muted">
                  {u.primary ? "User proyek" : `Dibuat ${u.created ? fmtTime(u.created) : "-"}`} · {u.host}
                </p>
              </div>
              <div className="row">
                <Chip tone={u.privs === "all" ? "info" : u.privs === "rw" ? "warn" : "ok"}>{PRIV_LABEL[u.privs] ?? u.privs}</Chip>
                {!u.primary && (
                  <>
                    <button className="btn" onClick={() => setPwFor(u.user)}>
                      Sandi<span className="sr"> {u.user}</span>
                    </button>
                    <button className="btn" onClick={() => setPrivFor(u)}>
                      Hak<span className="sr"> {u.user}</span>
                    </button>
                    <button className="btn btn-danger-ghost" onClick={() => setDelFor(u.user)}>
                      Hapus<span className="sr"> {u.user}</span>
                    </button>
                  </>
                )}
              </div>
            </li>
          ))}
        </ul>
      </State>
      {adding && (
        <AddUser
          base={base}
          onClose={() => setAdding(false)}
          onDone={(r) => {
            setAdding(false);
            q.reload();
            if (r.password) setSecret({ title: `User ${r.user} dibuat`, label: "Kata sandi", value: r.password, note: `Server 127.0.0.1, port ${r.port}, database ${name}.` });
            else setNote(`User ${r.user} dibuat dengan kata sandi pilihan Anda.`);
          }}
        />
      )}
      {pwFor && (
        <PasswordDialog
          title={`Ganti kata sandi ${pwFor}`}
          word={pwFor}
          hint="Kata sandi lama langsung tidak berlaku; aplikasi yang memakainya akan gagal konek."
          onClose={() => setPwFor(null)}
          onSubmit={async (p) => {
            const r = await post<{ password?: string }>(`${base}/${pwFor}/passwd`, p ? { password: p } : {});
            const u = pwFor;
            setPwFor(null);
            if (r.password) setSecret({ title: `Kata sandi ${u} diganti`, label: "Kata sandi baru", value: r.password });
            else setNote(`Kata sandi ${u} diganti.`);
          }}
        />
      )}
      {privFor && (
        <PrivDialog
          user={privFor}
          onClose={() => setPrivFor(null)}
          onSubmit={async (p) => {
            await post(`${base}/${privFor.user}/privs`, { privs: p });
            setNote(`Hak ${privFor.user} menjadi ${PRIV_LABEL[p]}.`);
            setPrivFor(null);
            q.reload();
          }}
        />
      )}
      {delFor && (
        <ConfirmDialog
          title={`Hapus user ${delFor}`}
          word={delFor}
          verb="Hapus user"
          body={<p>User dihapus permanen; aplikasi yang memakainya akan gagal konek. Data di database tidak terhapus.</p>}
          onClose={() => setDelFor(null)}
          onConfirm={async () => {
            await post(`${base}/${delFor}/remove`, { confirm: delFor });
            setNote(`User ${delFor} dihapus.`);
            setDelFor(null);
            q.reload();
          }}
        />
      )}
      {secret && <SecretDialog {...secret} onClose={() => setSecret(null)} />}
    </div>
  );
}

function AddUser({ base, onClose, onDone }: { base: PostUrl; onClose: () => void; onDone: (r: { user: string; password?: string; port: number }) => void }) {
  const [user, setUser] = useState("");
  const [privs, setPrivs] = useState("ro");
  const [own, setOwn] = useState(false);
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const okPw = !own || /^[A-Za-z0-9._+=:,@%^-]{12,100}$/.test(pw);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      onDone(await post(base, { user, privs, ...(own ? { password: pw } : {}) }));
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title="Tambah user database" onClose={onClose}>
      <form className="stack" onSubmit={submit} autoComplete="off">
        <Field label="Nama user" hint="Huruf kecil, angka, garis bawah; diawali huruf; 3-32 karakter.">
          {(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="app_user" maxLength={32} value={user} onChange={(e) => setUser(e.target.value.toLowerCase())} pattern="[a-z][a-z0-9_]{2,31}" required autoFocus spellCheck={false} />}
        </Field>
        <Field label="Hak akses" hint="Berlaku hanya pada database ini; tidak ada hak global.">
          {(id, dd) => (
            <select id={id} aria-describedby={dd} value={privs} onChange={(e) => setPrivs(e.target.value)}>
              <option value="ro">Baca-saja (SELECT)</option>
              <option value="rw">Baca-tulis (SELECT, INSERT, UPDATE, DELETE)</option>
              <option value="all">Penuh (termasuk ubah struktur tabel)</option>
            </select>
          )}
        </Field>
        <div className="seg" role="group" aria-label="Asal kata sandi">
          <button type="button" className={`seg-btn ${!own ? "on" : ""}`} aria-pressed={!own} onClick={() => setOwn(false)}>
            Acak (disarankan)
          </button>
          <button type="button" className={`seg-btn ${own ? "on" : ""}`} aria-pressed={own} onClick={() => setOwn(true)}>
            Pilih sendiri
          </button>
        </div>
        {own && (
          <Field label="Kata sandi" hint="12-100 karakter: huruf, angka, dan . _ + = : , @ % ^ -">
            {(id, dd) => <input id={id} aria-describedby={dd} type="password" autoComplete="new-password" placeholder="kata sandi" maxLength={100} value={pw} onChange={(e) => setPw(e.target.value)} required />}
          </Field>
        )}
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || !user || !okPw}>
            {busy ? "Membuat…" : "Buat user"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function PrivDialog({ user, onClose, onSubmit }: { user: DbUser; onClose: () => void; onSubmit: (p: string) => Promise<void> }) {
  const [p, setP] = useState(user.privs);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  return (
    <Modal title={`Hak akses ${user.user}`} onClose={onClose}>
      <form
        className="stack"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setErr("");
          try {
            await onSubmit(p);
          } catch (x) {
            setErr(errText(x));
            setBusy(false);
          }
        }}
      >
        <Field label="Hak">
          {(id) => (
            <select id={id} value={p} onChange={(e) => setP(e.target.value)}>
              <option value="ro">Baca-saja</option>
              <option value="rw">Baca-tulis</option>
              <option value="all">Penuh</option>
            </select>
          )}
        </Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || p === user.privs}>
            {busy ? "Menyimpan…" : "Simpan"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- kata sandi root ----------
export function RootPasswords() {
  const [eng, setEng] = useState<string | null>(null);
  const [secret, setSecret] = useState<Secret>(null);
  const [note, setNote] = useState("");
  return (
    <Card title="Kata sandi root server database">
      <div className="stack">
        <p className="small muted">
          Akun administrator tiap server database. Kata sandi baru diverifikasi dulu sebelum disimpan di berkas rahasia milik pemilik mesin (<span className="mono">db.env</span>, izin 0600); bila gagal, tidak ada yang berubah. Setelah diganti, perbarui koneksi di aplikasi klien (mis. DBeaver). Tidak tersedia lewat token API.
        </p>
        {note && <Alert tone="ok">{note}</Alert>}
        <ul className="list">
          {(["mariadb", "mysql", "pgsql"] as const).map((e) => (
            <li key={e} className="list-row">
              <div>
                <p>{ENGINE_LABEL[e]}</p>
                <p className="small muted">{e === "pgsql" ? "Akun dev, port 5432" : `Akun root, port ${e === "mariadb" ? 3306 : 3307}`}</p>
              </div>
              <button className="btn btn-danger-ghost" onClick={() => setEng(e)}>
                Ganti kata sandi<span className="sr"> {ENGINE_LABEL[e]}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
      {eng && (
        <PasswordDialog
          title={`Ganti kata sandi root ${ENGINE_LABEL[eng]}`}
          word={eng}
          hint="Aplikasi yang memakai akun root akan gagal konek sampai diperbarui. Proyek memakai user sendiri, jadi tidak terpengaruh."
          onClose={() => setEng(null)}
          onSubmit={async (p) => {
            const r = await post<{ password?: string }>(`/api/dbroot/${eng}/passwd`, { confirm: eng, ...(p ? { password: p } : {}) });
            const label = ENGINE_LABEL[eng];
            setEng(null);
            if (r.password) setSecret({ title: `Kata sandi root ${label} diganti`, label: "Kata sandi baru", value: r.password, note: "Juga tersimpan di db.env." });
            else setNote(`Kata sandi root ${label} diganti dan diverifikasi.`);
          }}
        />
      )}
      {secret && <SecretDialog {...secret} onClose={() => setSecret(null)} />}
    </Card>
  );
}
