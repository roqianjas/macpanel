import { useState } from "react";
import { errText, post } from "../api";
import { Alert, Card, Field } from "../ui";

export default function Account() {
  const [cur, setCur] = useState("");
  const [nw, setNw] = useState("");
  const [again, setAgain] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const mismatch = again !== "" && again !== nw;
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      await post("/api/me/password", { current: cur, new: nw });
      setMsg({ tone: "ok", text: "Kata sandi diganti. Sesi lain di perangkat lain sudah dikeluarkan." });
      setCur("");
      setNw("");
      setAgain("");
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Ganti kata sandi">
      <form onSubmit={submit} className="stack" autoComplete="off">
        <p className="small muted">Meminta kode TOTP ulang. Semua sesi lain dikeluarkan setelah berhasil. Kode TOTP dan recovery code tidak berubah; untuk mengganti perangkat autentikator, buat ulang akun dari terminal (<span className="mono">panel user</span>).</p>
        <div className="grid2">
          <Field label="Kata sandi saat ini">{(id) => <input id={id} type="password" value={cur} onChange={(e) => setCur(e.target.value)} autoComplete="current-password" required />}</Field>
          <span />
          <Field label="Kata sandi baru" hint="Minimal 12 karakter, berbeda dari yang lama">{(id, dd) => <input id={id} aria-describedby={dd} type="password" value={nw} onChange={(e) => setNw(e.target.value)} autoComplete="new-password" minLength={12} maxLength={256} required />}</Field>
          <Field label="Ulangi kata sandi baru" hint={mismatch ? "Tidak sama dengan kata sandi baru" : undefined}>{(id, dd) => <input id={id} aria-describedby={dd} aria-invalid={mismatch} type="password" value={again} onChange={(e) => setAgain(e.target.value)} autoComplete="new-password" required />}</Field>
        </div>
        {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
        <div>
          <button className="btn" disabled={busy || !cur || nw.length < 12 || nw !== again}>{busy ? "Menyimpan…" : "Ganti kata sandi"}</button>
        </div>
      </form>
    </Card>
  );
}
