import { useState } from "react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";
import { SecretDialog } from "./DbUsers";
import { Avatar } from "../features/profile/avatar";

interface Acc {
  name: string;
  role: string;
  disabled: boolean;
  totp_on: boolean;
  must_change: boolean;
  created: number;
  self: boolean;
  display_name: string;
  avatar: string;
}
type Secret = { title: string; label: string; value: string; note?: string } | null;

export default function Users() {
  const q = useLoad(() => get<{ accounts: Acc[] }>("/api/accounts"));
  const [adding, setAdding] = useState(false);
  const [sel, setSel] = useState<Acc | null>(null);
  const [secret, setSecret] = useState<Secret>(null);
  const [note, setNote] = useState("");
  const list = q.data?.accounts ?? [];
  return (
    <>
      <PageHead
        title="Pengguna"
        desc="Akun yang dapat masuk ke panel. Akun baru mendapat kata sandi sementara yang wajib diganti saat masuk pertama; 2FA dinyalakan sendiri oleh pemiliknya di Pengaturan."
        action={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            Tambah pengguna
          </button>
        }
      />
      {note && <Alert tone="ok">{note}</Alert>}
      <Card title="Daftar pengguna">
        <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && list.length === 0} emptyText="Belum ada pengguna." onRetry={q.reload}>
          <ul className="list">
            {list.map((u) => (
              <li key={u.name} className="list-row user-row">
                <Avatar name={u.name} display={u.display_name} avatar={u.avatar} size={44} />
                <div>
                  {u.display_name && <p className="profile-name">{u.display_name}</p>}
                  <p className="mono">
                    @{u.name}
                    {u.self ? " (Anda)" : ""}
                  </p>
                  <p className="small muted">Dibuat {fmtTime(u.created)}</p>
                  <div className="row" style={{ flexWrap: "wrap" }}>
                    <Chip tone={u.role === "admin" ? "info" : "mute"}>{u.role === "admin" ? "Admin" : "Hanya-baca"}</Chip>
                    <Chip tone={u.totp_on ? "ok" : "warn"}>{u.totp_on ? "2FA aktif" : "2FA mati"}</Chip>
                    {u.disabled && <Chip tone="bad">Nonaktif</Chip>}
                    {u.must_change && <Chip tone="warn">Belum ganti sandi sementara</Chip>}
                  </div>
                </div>
                {u.self ? (
                  <a className="btn" href="#/profil">
                    Profil saya<span className="sr"> (nama, avatar, kata sandi, 2FA)</span>
                  </a>
                ) : (
                  <button className="btn" onClick={() => setSel(u)}>
                    Kelola<span className="sr"> {u.name}</span>
                  </button>
                )}
              </li>
            ))}
          </ul>
        </State>
      </Card>
      {adding && (
        <AddAccount
          onClose={() => setAdding(false)}
          onDone={(r) => {
            setAdding(false);
            q.reload();
            if (!r.temp_password) return setNote(`Akun ${r.name} dibuat dengan kata sandi sementara pilihan Anda. Pemilik wajib menggantinya saat masuk pertama.`);
            setSecret({ title: `Akun ${r.name} dibuat`, label: "Kata sandi sementara", value: r.temp_password ?? "", note: "Berikan kepada pemiliknya lewat jalur aman. Ia wajib menggantinya saat masuk pertama." });
          }}
        />
      )}
      {sel && (
        <Manage
          acc={sel}
          onClose={() => setSel(null)}
          onChanged={(msg, sec) => {
            setSel(null);
            q.reload();
            if (sec) setSecret(sec);
            else setNote(msg);
          }}
        />
      )}
      {secret && <SecretDialog {...secret} onClose={() => setSecret(null)} />}
    </>
  );
}

function AddAccount({ onClose, onDone }: { onClose: () => void; onDone: (r: { name: string; temp_password?: string }) => void }) {
  const [name, setName] = useState("");
  const [role, setRole] = useState("viewer");
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  return (
    <Modal title="Tambah pengguna" onClose={onClose}>
      <form
        className="stack"
        autoComplete="off"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setErr("");
          try {
            onDone(await post("/api/accounts", pw ? { name, role, password: pw } : { name, role }));
          } catch (x) {
            setErr(errText(x));
            setBusy(false);
          }
        }}
      >
        <Field label="Nama pengguna" hint="Huruf kecil, angka, _ atau -; 3-32 karakter; diawali huruf.">
          {(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="nama-pengguna" maxLength={32} value={name} onChange={(e) => setName(e.target.value.toLowerCase())} pattern="[a-z][a-z0-9_-]{2,31}" required data-autofocus spellCheck={false} />}
        </Field>
        <Field label="Peran" hint="Hanya-baca: melihat saja, tidak dapat mengubah apa pun dan tidak melihat kredensial.">
          {(id, dd) => (
            <select id={id} aria-describedby={dd} value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="viewer">Hanya-baca</option>
              <option value="admin">Admin (semua aksi)</option>
            </select>
          )}
        </Field>
        <PasswordField value={pw} onChange={setPw} />
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || !name || (pw !== "" && pw.length < 12)}>
            {busy ? "Membuat…" : "Buat akun"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function Manage({ acc, onClose, onChanged }: { acc: Acc; onClose: () => void; onChanged: (msg: string, secret?: Secret) => void }) {
  const [role, setRole] = useState(acc.role);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [del, setDel] = useState(false);
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setErr("");
    try {
      await fn();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  const base = `/api/accounts/${acc.name}` as const;
  return (
    <Modal title={`Kelola ${acc.name}`} onClose={onClose}>
      <div className="stack">
        {err && <Alert>{err}</Alert>}
        <Field label="Peran" hint="Mengubah peran mengeluarkan semua sesi dan mencabut token akun itu.">
          {(id, dd) => (
            <div className="row">
              <select id={id} aria-describedby={dd} value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="viewer">Hanya-baca</option>
                <option value="admin">Admin</option>
              </select>
              <button className="btn" disabled={busy || role === acc.role} onClick={() => run(async () => { await post(`${base}/role`, { role }); onChanged(`Peran ${acc.name} diubah.`); })}>
                Simpan peran
              </button>
            </div>
          )}
        </Field>
        <div className="stack">
          <button className="btn" disabled={busy} onClick={() => run(async () => { await post(`${base}/${acc.disabled ? "enable" : "disable"}`, {}); onChanged(`${acc.name} ${acc.disabled ? "diaktifkan" : "dinonaktifkan"}.`); })}>
            {acc.disabled ? "Aktifkan akun" : "Nonaktifkan akun (keluarkan semua sesi)"}
          </button>
          <ResetPassword busy={busy} run={run} base={base} acc={acc} onChanged={onChanged} />
          {!acc.totp_on && <p className="small muted">2FA mati. Hanya pemilik akun yang bisa menyalakannya (butuh aplikasi autentikator di ponselnya) lewat Profil saya.</p>}
          {acc.totp_on && (
            <button className="btn" disabled={busy} onClick={() => run(async () => { await post(`${base}/2fa-off`, {}); onChanged(`2FA ${acc.name} dimatikan; ia dapat menyalakannya lagi di Pengaturan.`); })}>
              Matikan 2FA (mis. ponsel hilang)
            </button>
          )}
          <button className="btn btn-danger-ghost" disabled={busy} onClick={() => setDel(true)}>
            Hapus akun
          </button>
        </div>
      </div>
      {del && (
        <ConfirmDialog
          title={`Hapus akun ${acc.name}`}
          word={acc.name}
          verb="Hapus akun"
          body={<p>Akun, sesi, perangkat tepercaya, dan tokennya dihapus permanen. Riwayat di Aktivitas tetap ada.</p>}
          onClose={() => setDel(false)}
          onConfirm={async () => {
            await post(`${base}/remove`, { confirm: acc.name });
            setDel(false);
            onChanged(`Akun ${acc.name} dihapus.`);
          }}
        />
      )}
    </Modal>
  );
}

function PasswordField({ value, onChange, label = "Kata sandi sementara (opsional)" }: { value: string; onChange: (v: string) => void; label?: string }) {
  const [show, setShow] = useState(false);
  const gen = () => {
    const a = new Uint8Array(18);
    crypto.getRandomValues(a);
    onChange(btoa(String.fromCharCode(...a)).replace(/[+/=]/g, "x").slice(0, 20));
    setShow(true);
  };
  return (
    <Field label={label} hint={value ? (value.length < 12 ? `Minimal 12 karakter (sekarang ${value.length}).` : "Pemilik tetap wajib menggantinya saat masuk pertama.") : "Kosongkan agar panel membuat kata sandi acak dan menampilkannya sekali. Atau isi sendiri, minimal 12 karakter."}>
      {(id, dd) => (
        <div className="row">
          <input id={id} aria-describedby={dd} className="mono" type={show ? "text" : "password"} autoComplete="new-password" placeholder="kosong = dibuat otomatis" maxLength={256} value={value} onChange={(e) => onChange(e.target.value)} />
          <button type="button" className="btn" onClick={() => setShow((v) => !v)} aria-pressed={show}>{show ? "Sembunyikan" : "Tampilkan"}</button>
          <button type="button" className="btn" onClick={gen}>Buat acak</button>
        </div>
      )}
    </Field>
  );
}

function ResetPassword({ busy, run, base, acc, onChanged }: { busy: boolean; run: (fn: () => Promise<void>) => void; base: string; acc: Acc; onChanged: (msg: string, secret?: Secret) => void }) {
  const [open, setOpen] = useState(false);
  const [pw, setPw] = useState("");
  if (!open) return <button className="btn" disabled={busy} onClick={() => setOpen(true)}>Reset kata sandi</button>;
  return (
    <div className="stack sched">
      <PasswordField value={pw} onChange={setPw} label="Kata sandi sementara baru (opsional)" />
      <div className="row">
        <button className="btn" onClick={() => { setOpen(false); setPw(""); }}>Batal</button>
        <button className="btn btn-primary" disabled={busy || (pw !== "" && pw.length < 12)} onClick={() => run(async () => {
          const r = await post<{ temp_password?: string }>(`${base}/reset` as `/api/accounts/${string}/reset`, pw ? { password: pw } : {});
          if (!r.temp_password) return onChanged(`Kata sandi ${acc.name} direset ke pilihan Anda; semua sesi dan token dicabut. Ia wajib menggantinya saat masuk.`);
          onChanged("", { title: `Kata sandi ${acc.name} direset`, label: "Kata sandi sementara", value: r.temp_password, note: "Semua sesi, perangkat tepercaya, dan token akun itu dicabut. Ia wajib menggantinya saat masuk." });
        })}>Reset kata sandi</button>
      </div>
    </div>
  );
}
