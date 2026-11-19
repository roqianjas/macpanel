import { useState } from "react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, useLoad } from "../ui";
import { hasDevicePref, setDevicePref, setPublicSettings, useServerTheme, useThemePref, usePanelName, type Pref } from "../settings";
import Account from "./Account";
import AppearanceCard from "../features/appearance/AppearanceCard";
import NotifyCard from "../features/notify/NotifyCard";
import ThresholdsCard from "../features/hardware/ThresholdsCard";

const THEMES: { id: Pref; label: string }[] = [
  { id: "system", label: "Ikuti sistem" },
  { id: "light", label: "Terang" },
  { id: "dark", label: "Gelap" },
];

type Note = { tone: "ok" | "bad"; text: string } | null;

async function copy(v: string, set: (n: Note) => void, what: string) {
  try {
    await navigator.clipboard.writeText(v);
    set({ tone: "ok", text: `${what} disalin.` });
  } catch {
    set({ tone: "bad", text: "Gagal menyalin; salin manual." });
  }
}

// ---------- Panel: nama dan tema ----------
function PanelCard() {
  const q = useLoad(() => get<{ name: string; theme: Pref; can_edit: boolean }>("/api/settings"));
  const name = usePanelName();
  const pref = useThemePref();
  const serverTheme = useServerTheme();
  const [draft, setDraft] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const canEdit = q.data?.can_edit ?? false;
  const save = async (body: { name?: string; theme?: string }, done: string) => {
    setBusy(true);
    setNote(null);
    try {
      setPublicSettings(await post<{ name: string; theme: string }>("/api/settings", body));
      setDraft(null);
      setNote({ tone: "ok", text: done });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const shown = draft ?? name;
  return (
    <Card title="Panel">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
          <form
            className="stack"
            onSubmit={(e) => {
              e.preventDefault();
              save({ name: shown }, "Nama panel diganti.");
            }}
          >
            <Field label="Nama panel" hint={canEdit ? "Tampil di halaman masuk, menu samping, dan judul tab. 1-40 karakter." : "Hanya admin yang dapat mengganti nama panel."}>
              {(id, dd) => <input id={id} aria-describedby={dd} placeholder="MacPanel" value={shown} maxLength={40} disabled={!canEdit} onChange={(e) => setDraft(e.target.value)} required />}
            </Field>
            {canEdit && (
              <div className="row-end">
                <button className="btn btn-primary" disabled={busy || draft === null || draft.trim() === "" || draft === name}>
                  {busy ? "Menyimpan…" : "Simpan nama"}
                </button>
              </div>
            )}
          </form>
          <div className="stack">
            <span className="field-label">Tema perangkat ini</span>
            <div className="seg" role="group" aria-label="Tema perangkat ini">
              {THEMES.map((t) => (
                <button key={t.id} type="button" className={`seg-btn ${pref === t.id ? "on" : ""}`} aria-pressed={pref === t.id} onClick={() => setDevicePref(t.id)}>
                  {t.label}
                </button>
              ))}
            </div>
            <p className="small muted">
              Disimpan di browser ini saja.{" "}
              {hasDevicePref() && (
                <button type="button" className="btn btn-ghost" onClick={() => setDevicePref(null)}>
                  Kembali ke bawaan panel
                </button>
              )}
            </p>
          </div>
          {canEdit && (
            <Field label="Tema bawaan panel" hint="Dipakai perangkat yang belum memilih tema sendiri.">
              {(id, dd) => (
                <select id={id} aria-describedby={dd} value={serverTheme} disabled={busy} onChange={(e) => save({ theme: e.target.value }, "Tema bawaan diganti.")}>
                  {THEMES.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.label}
                    </option>
                  ))}
                </select>
              )}
            </Field>
          )}
        </div>
      </State>
    </Card>
  );
}

// ---------- 2FA ----------
export function TwoFactor() {
  const q = useLoad(() => get<{ enabled: boolean; recovery_left: number }>("/api/me/2fa"));
  const [note, setNote] = useState<Note>(null);
  const [setup, setSetup] = useState<{ secret: string; uri: string } | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [off, setOff] = useState(false);
  const [busy, setBusy] = useState(false);
  const start = async () => {
    setBusy(true);
    setNote(null);
    try {
      setSetup(await post<{ secret: string; uri: string }>("/api/me/2fa/setup", {}));
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const on = q.data?.enabled;
  return (
    <Card title="Verifikasi dua langkah (2FA)" action={q.data ? <Chip tone={on ? "ok" : "warn"}>{on ? "Aktif" : "Mati"}</Chip> : undefined}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
          {on ? (
            <>
              <p className="small muted">Login meminta kode dari aplikasi autentikator. Recovery code tersisa: {q.data?.recovery_left}. Mematikan 2FA membutuhkan kata sandi dan kode saat ini, lalu mengeluarkan semua sesi lain dan mencabut semua perangkat tepercaya.</p>
              <div className="row-end">
                <button className="btn btn-danger-ghost" onClick={() => setOff(true)}>
                  Matikan 2FA
                </button>
              </div>
            </>
          ) : (
            <>
              <Alert tone="warn">2FA mati: akun ini masuk hanya dengan nama pengguna dan kata sandi. Nyalakan kembali agar login lebih aman.</Alert>
              <div className="row-end">
                <button className="btn btn-primary" disabled={busy} onClick={start}>
                  {busy ? "Menyiapkan…" : "Nyalakan 2FA"}
                </button>
              </div>
            </>
          )}
        </div>
      </State>
      {setup && <EnableDialog setup={setup} onClose={() => setSetup(null)} onDone={(c) => { setSetup(null); setCodes(c); q.reload(); }} />}
      {codes && <CodesDialog codes={codes} onClose={() => setCodes(null)} />}
      {off && <DisableDialog onClose={() => setOff(false)} onDone={() => { setOff(false); q.reload(); setNote({ tone: "ok", text: "2FA dimatikan. Sesi lain dan perangkat tepercaya sudah dicabut." }); }} />}
    </Card>
  );
}

function EnableDialog({ setup, onClose, onDone }: { setup: { secret: string; uri: string }; onClose: () => void; onDone: (codes: string[]) => void }) {
  const [code, setCode] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ recovery_codes: string[] }>("/api/me/2fa/enable", { code: code.trim() });
      onDone(r.recovery_codes);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title="Nyalakan 2FA" onClose={onClose}>
      <form className="stack" onSubmit={submit}>
        <p className="small muted">Di aplikasi autentikator (Google Authenticator, Authy, 1Password), pilih tambah akun lalu "Enter a setup key" (jenis berbasis waktu), dan masukkan kunci ini:</p>
        <div className="row">
          <code className="mono" style={{ wordBreak: "break-all" }}>{setup.secret}</code>
          <button type="button" className="btn" onClick={() => copy(setup.secret, setNote, "Kunci")}>
            Salin
          </button>
        </div>
        {note && <Alert tone={note.tone}>{note.text}</Alert>}
        <Field label="Kode 6 digit dari aplikasi" hint="Memastikan kunci sudah benar sebelum 2FA diaktifkan.">
          {(id, dd) => <input id={id} aria-describedby={dd} data-autofocus inputMode="numeric" autoComplete="one-time-code" className="mono code-input" placeholder="123456" value={code} onChange={(e) => setCode(e.target.value)} required />}
        </Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || code.trim().length < 6}>
            {busy ? "Memeriksa…" : "Aktifkan"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function CodesDialog({ codes, onClose }: { codes: string[]; onClose: () => void }) {
  const [note, setNote] = useState<Note>(null);
  return (
    <Modal title="Simpan recovery code" onClose={onClose}>
      <div className="stack">
        <Alert tone="warn">Kode ini hanya tampil sekali. Tiap kode dapat dipakai satu kali bila ponsel hilang. Simpan di pengelola kata sandi.</Alert>
        <ul className="list mono">
          {codes.map((c) => (
            <li key={c} className="list-row">
              {c}
            </li>
          ))}
        </ul>
        {note && <Alert tone={note.tone}>{note.text}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={() => copy(codes.join("\n"), setNote, "Recovery code")}>
            Salin semua
          </button>
          <button className="btn btn-primary" onClick={onClose}>
            Sudah saya simpan
          </button>
        </div>
      </div>
    </Modal>
  );
}

function DisableDialog({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const [pw, setPw] = useState("");
  const [code, setCode] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await post("/api/me/2fa/disable", { password: pw, code: code.trim() });
      onDone();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title="Matikan 2FA" onClose={onClose}>
      <form className="stack" onSubmit={submit} autoComplete="off">
        <p className="small muted">Masukkan kata sandi dan kode 6 digit (atau satu recovery code) saat ini.</p>
        <Field label="Kata sandi">{(id) => <input id={id} type="password" data-autofocus autoComplete="current-password" placeholder="kata sandi" value={pw} onChange={(e) => setPw(e.target.value)} required />}</Field>
        <Field label="Kode 6 digit atau recovery code">{(id) => <input id={id} autoComplete="one-time-code" className="mono code-input" placeholder="123456" value={code} onChange={(e) => setCode(e.target.value)} required />}</Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-danger" disabled={busy || !pw || code.trim().length < 6}>
            {busy ? "Memeriksa…" : "Matikan 2FA"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- Token API ----------
interface Tok {
  id: number;
  name: string;
  prefix: string;
  scope: string;
  created: number;
  expires: number;
  last_used: number;
  last_ip: string;
}
const SCOPE_LABEL: Record<string, string> = { read: "Baca-saja", operate: "Operasi", full: "Penuh" };
const SCOPE_TONE: Record<string, "ok" | "warn" | "bad"> = { read: "ok", operate: "warn", full: "bad" };

function Tokens() {
  const q = useLoad(() => get<{ tokens: Tok[]; max: number }>("/api/me/tokens"));
  const [name, setName] = useState("");
  const [scope, setScope] = useState("operate");
  const [days, setDays] = useState("90");
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [made, setMade] = useState<{ token: string; scope: string } | null>(null);
  const [rev, setRev] = useState<Tok | null>(null);
  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const r = await post<{ token: string; scope: string }>("/api/me/tokens", { name: name.trim(), scope, days: Number(days) });
      setMade(r);
      setName("");
      q.reload();
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const list = q.data?.tokens ?? [];
  return (
    <Card title="Token API (agen AI dan otomasi)">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          <p className="small muted">
            Token memberi akses API tanpa login manusia, dipakai lewat perintah <span className="mono">panelctl</span> atau header <span className="mono">Authorization: Bearer</span>. Hanya hash yang disimpan, berlaku terbatas, dan dapat dicabut kapan saja. Baca-saja: hanya melihat. Operasi: tambah dan ubah (situs, database, deploy), tanpa aksi yang butuh verifikasi ulang. Penuh: juga hapus, config, dan pasang (penghapusan wajib mengetik nama target). Login, 2FA, kata sandi, kredensial database, dan isi .env tidak pernah tersedia lewat token.
          </p>
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
          <form className="stack" onSubmit={create}>
            <div className="grid2">
              <Field label="Nama token" hint="Mis. nama agen atau mesin.">
                {(id, dd) => <input id={id} aria-describedby={dd} placeholder="mis. laptop-hp" value={name} maxLength={40} onChange={(e) => setName(e.target.value)} required autoComplete="off" />}
              </Field>
              <Field label="Tingkat">
                {(id) => (
                  <select id={id} value={scope} onChange={(e) => setScope(e.target.value)}>
                    <option value="read">Baca-saja</option>
                    <option value="operate">Operasi</option>
                    <option value="full">Penuh</option>
                  </select>
                )}
              </Field>
              <Field label="Masa berlaku">
                {(id) => (
                  <select id={id} value={days} onChange={(e) => setDays(e.target.value)}>
                    <option value="7">7 hari</option>
                    <option value="30">30 hari</option>
                    <option value="90">90 hari</option>
                    <option value="365">365 hari</option>
                  </select>
                )}
              </Field>
            </div>
            <div className="row-end">
              <button className="btn btn-primary" disabled={busy || name.trim() === "" || list.length >= (q.data?.max ?? 10)}>
                {busy ? "Membuat…" : "Buat token"}
              </button>
            </div>
          </form>
          {list.length === 0 ? (
            <p className="small muted">Belum ada token.</p>
          ) : (
            <ul className="list">
              {list.map((t) => (
                <li key={t.id} className="list-row">
                  <span>
                    {t.name} <Chip tone={SCOPE_TONE[t.scope] ?? "mute"}>{SCOPE_LABEL[t.scope] ?? t.scope}</Chip>
                    <br />
                    <span className="small muted">
                      <span className="mono">{t.prefix}…</span> · berlaku sampai {fmtTime(t.expires)} · {t.last_used ? `terakhir dipakai ${fmtTime(t.last_used)}${t.last_ip ? ` dari ${t.last_ip}` : ""}` : "belum pernah dipakai"}
                    </span>
                  </span>
                  <button className="btn btn-danger-ghost" onClick={() => setRev(t)}>
                    Cabut
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </State>
      {made && <MadeDialog made={made} onClose={() => setMade(null)} />}
      {rev && (
        <ConfirmDialog
          title="Cabut token"
          body={<>Token "{rev.name}" langsung berhenti bekerja. Agen yang memakainya akan ditolak.</>}
          word={rev.name}
          verb="Cabut"
          onClose={() => setRev(null)}
          onConfirm={async () => {
            await post("/api/me/tokens/revoke", { id: rev.id });
            setRev(null);
            q.reload();
          }}
        />
      )}
    </Card>
  );
}

function MadeDialog({ made, onClose }: { made: { token: string; scope: string }; onClose: () => void }) {
  const [note, setNote] = useState<Note>(null);
  return (
    <Modal title="Token dibuat" onClose={onClose} wide>
      <div className="stack">
        <Alert tone="warn">Token ini hanya tampil sekali. Simpan sekarang di tempat aman; jangan ditempel ke chat atau repo.</Alert>
        <div className="row">
          <code className="mono" style={{ wordBreak: "break-all" }}>{made.token}</code>
        </div>
        <p className="small muted">
          Pakai di mesin klien: simpan ke <span className="mono">~/.config/macpanel/token</span> (chmod 600), lalu <span className="mono">panelctl whoami</span>. Tingkat: {SCOPE_LABEL[made.scope]}.
        </p>
        {note && <Alert tone={note.tone}>{note.text}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={() => copy(made.token, setNote, "Token")}>
            Salin token
          </button>
          <button className="btn btn-primary" onClick={onClose}>
            Sudah saya simpan
          </button>
        </div>
      </div>
    </Modal>
  );
}

export default function Settings() {
  return (
    <>
      <PageHead title="Pengaturan" desc="Nama dan tema panel, kata sandi, verifikasi dua langkah, perangkat tepercaya, dan token API." />
      <PanelCard />
      <AppearanceCard />
      <NotifyCard />
      <ThresholdsCard />
      <Account />
      <TwoFactor />
      <Tokens />
    </>
  );
}
