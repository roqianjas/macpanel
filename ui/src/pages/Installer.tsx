import { useState } from "react";
import { errText, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Field, Modal } from "../ui";

const NAME = /^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$/;

function useRun(onDone: (name: string) => void) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const run = async (path: string, body: object, label: string, name: string) => {
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ job: number }>(path, body);
      trackJob(r.job, label, (j) => {
        setBusy(false);
        if (j.status === "ok") onDone(name);
        else setErr(j.output || "Gagal.");
      });
    } catch (x) {
      setBusy(false);
      setErr(errText(x));
    }
  };
  return { busy, err, run };
}

export function InstallModal({ onClose, onDone }: { onClose: () => void; onDone: (name: string) => void }) {
  const [kind, setKind] = useState<"laravel" | "wordpress">("laravel");
  const [name, setName] = useState("");
  const [php, setPhp] = useState("8.4");
  const { busy, err, run } = useRun(onDone);
  const ok = NAME.test(name);
  return (
    <Modal title="Pasang cepat" onClose={onClose}>
      <form className="stack" onSubmit={(e) => { e.preventDefault(); if (ok) run("/api/installer", { kind, name, php }, `Pasang ${kind} ${name}`, name); }}>
        <p className="small muted">Membuat folder di <span className="mono">~/MacPanel/wwwroot/&lt;nama&gt;</span>, database MySQL baru, kredensial, dan situs <span className="mono">&lt;nama&gt;.lab</span>. Butuh internet dan beberapa menit. Bila gagal di tengah, semua yang baru dibuat dibersihkan. Meminta kode TOTP ulang.</p>
        <div className="seg" role="group" aria-label="Jenis aplikasi">
          {(["laravel", "wordpress"] as const).map((k) => (
            <button key={k} type="button" className={`seg-btn ${kind === k ? "on" : ""}`} aria-pressed={kind === k} onClick={() => setKind(k)}>{k === "laravel" ? "Laravel" : "WordPress"}</button>
          ))}
        </div>
        <Field label="Nama situs" hint="Huruf kecil, angka, tanda hubung. Jadi nama.lab.">{(id, dd) => <input id={id} aria-describedby={dd} className="mono" value={name} onChange={(e) => setName(e.target.value.toLowerCase())} maxLength={30} required autoComplete="off" spellCheck={false} />}</Field>
        <Field label="Versi PHP">{(id) => <select id={id} value={php} onChange={(e) => setPhp(e.target.value)}>{["8.2", "8.3", "8.4", "8.5"].map((v) => <option key={v}>{v}</option>)}</select>}</Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>Batal</button>
          <button className="btn btn-primary" disabled={busy || !ok}>{busy ? "Memasang… (lihat tray kerja)" : "Pasang"}</button>
        </div>
      </form>
    </Modal>
  );
}

export function CloneModal({ src, onClose, onDone }: { src: string; onClose: () => void; onDone: (name: string) => void }) {
  const [dst, setDst] = useState(`${src}-staging`.slice(0, 30));
  const [withDb, setWithDb] = useState(true);
  const { busy, err, run } = useRun(onDone);
  const ok = NAME.test(dst) && dst !== src;
  return (
    <Modal title={`Kloning ${src}.lab`} onClose={onClose}>
      <form className="stack" onSubmit={(e) => { e.preventDefault(); if (ok) run(`/api/sites/${src}/clone`, { dst, with_db: withDb }, `Kloning ${src} ke ${dst}`, dst); }}>
        <p className="small muted">Menyalin folder proyek (tanpa <span className="mono">.git</span>, <span className="mono">node_modules</span>, dan log) ke <span className="mono">~/MacPanel/wwwroot/&lt;nama&gt;</span>, membuat situs baru, dan database baru terpisah dengan kredensial sendiri; <span className="mono">.env</span> salinan diarahkan ke database baru sehingga staging tidak menyentuh data sumber. Meminta kode TOTP ulang.</p>
        <Field label="Nama situs baru">{(id) => <input id={id} className="mono" value={dst} onChange={(e) => setDst(e.target.value.toLowerCase())} maxLength={30} required autoComplete="off" spellCheck={false} />}</Field>
        <div className="checks">
          <label className="check"><input type="checkbox" checked={withDb} onChange={(e) => setWithDb(e.target.checked)} />Salin juga isi database (bila proyek memakai database lab dengan nama sama)</label>
        </div>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>Batal</button>
          <button className="btn btn-primary" disabled={busy || !ok}>{busy ? "Mengkloning… (lihat tray kerja)" : "Kloning"}</button>
        </div>
      </form>
    </Modal>
  );
}
