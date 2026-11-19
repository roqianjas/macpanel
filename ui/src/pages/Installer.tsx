import { useState } from "react";
import { errText, get, post } from "../api";
import type { PostUrl } from "../api.gen";
import { trackJob } from "../jobs";
import { Alert, Field, Modal, useLoad } from "../ui";

const NAME = /^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$/;
const APP_NAME = /^[a-z][a-z0-9-]{0,28}[a-z0-9]$/;
type Kind = "laravel" | "wordpress" | "go" | "python" | "nextjs";
const KINDS: { id: Kind; label: string; tool?: string }[] = [
  { id: "laravel", label: "Laravel" },
  { id: "wordpress", label: "WordPress" },
  { id: "go", label: "Go", tool: "go" },
  { id: "python", label: "Python", tool: "python" },
  { id: "nextjs", label: "Next.js", tool: "node" },
];
const APP_HINT: Record<string, string> = {
  go: "Membuat proyek Go sederhana (net/http), membangun biner, menjalankannya sebagai worker launchd di 127.0.0.1, dan membuat situs proxy <nama>.lab.",
  python: "Membuat proyek Python dengan virtualenv (.venv) dan aplikasi contoh tanpa dependensi, menjalankannya sebagai worker launchd di 127.0.0.1, dan membuat situs proxy <nama>.lab.",
  nextjs: "Menjalankan create-next-app (TypeScript, App Router, pnpm), membangun, menjalankannya sebagai worker launchd di 127.0.0.1, dan membuat situs proxy <nama>.lab. Butuh internet dan bisa beberapa menit.",
};

function useRun(onDone: (name: string) => void) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const run = async (path: PostUrl, body: object, label: string, name: string) => {
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
  const [kind, setKind] = useState<Kind>("laravel");
  const [runtime, setRuntime] = useState("");
  const [port, setPort] = useState("");
  const rts = useLoad(() => get<{ runtimes: { tool: string; version: string; default: boolean; pruned: boolean }[] }>("/api/runtime"));
  const [name, setName] = useState("");
  const [php, setPhp] = useState("8.4");
  const [db, setDb] = useState("mariadb");
  const { busy, err, run } = useRun(onDone);
  const meta = KINDS.find((k) => k.id === kind)!;
  const isApp = !!meta.tool;
  const versions = (rts.data?.runtimes ?? []).filter((r) => r.tool === meta.tool && !r.pruned);
  const portNum = port === "" ? 0 : Number(port);
  const ok = (isApp ? APP_NAME.test(name) : NAME.test(name)) && (!isApp || port === "" || (Number.isInteger(portNum) && portNum >= 1024 && portNum <= 65535));
  return (
    <Modal title="Pasang cepat" onClose={onClose}>
      <form className="stack" onSubmit={(e) => { e.preventDefault(); if (ok) run("/api/installer", isApp ? { kind, name, ...(runtime ? { runtime } : {}), ...(portNum ? { port: portNum } : {}) } : { kind, name, php, db }, `Pasang ${kind} ${name}`, name); }}>
        <p className="small muted">{isApp ? APP_HINT[kind] : <>Membuat folder di <span className="mono">~/MacPanel/wwwroot/&lt;nama&gt;</span>, database baru (MariaDB atau MySQL), kredensial, dan situs <span className="mono">&lt;nama&gt;.lab</span>. Butuh internet dan beberapa menit.</>} Bila gagal di tengah, semua yang baru dibuat dibersihkan. Meminta kata sandi ulang.</p>
        <div className="seg" role="group" aria-label="Jenis proyek" style={{ flexWrap: "wrap" }}>
          {KINDS.map((k) => (
            <button key={k.id} type="button" className={`seg-btn ${kind === k.id ? "on" : ""}`} aria-pressed={kind === k.id} onClick={() => { setKind(k.id); setRuntime(""); }}>{k.label}</button>
          ))}
        </div>
        <Field label="Nama situs" hint="Huruf kecil, angka, tanda hubung. Jadi nama.lab.">{(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="toko-saya" pattern="[a-z0-9]([a-z0-9\-]{0,28}[a-z0-9])?" title="Huruf kecil, angka, tanda hubung; tidak diawali atau diakhiri tanda hubung" value={name} onChange={(e) => setName(e.target.value.toLowerCase())} maxLength={30} required autoComplete="off" spellCheck={false} />}</Field>
        {isApp ? (
          <>
            <Field label={`Versi ${meta.id === "nextjs" ? "Node" : meta.label}`} hint="Kosong = versi bawaan. Versi lain dipasang di menu PHP dan runtime.">
              {(id, dd) => (
                <select id={id} aria-describedby={dd} value={runtime} onChange={(e) => setRuntime(e.target.value)}>
                  <option value="">Bawaan</option>
                  {versions.map((v) => <option key={v.version} value={v.version}>{v.version}</option>)}
                </select>
              )}
            </Field>
            <Field label="Port (opsional)" hint="Kosong = dipilihkan otomatis (3100-3999). Aplikasi hanya mendengarkan 127.0.0.1.">
              {(id, dd) => <input id={id} aria-describedby={dd} className="mono" inputMode="numeric" placeholder="otomatis" pattern="[0-9]{4,5}" title="Port 1024-65535 atau kosong" value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))} maxLength={5} />}
            </Field>
          </>
        ) : (
          <>
        <Field label="Database">{(id) => <select id={id} value={db} onChange={(e) => setDb(e.target.value)}><option value="mariadb">MariaDB (port 3306)</option><option value="mysql">MySQL (port 3307)</option></select>}</Field>
        <Field label="Versi PHP">{(id) => <select id={id} value={php} onChange={(e) => setPhp(e.target.value)}>{["8.2", "8.3", "8.4", "8.5"].map((v) => <option key={v}>{v}</option>)}</select>}</Field>
          </>
        )}
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
        <p className="small muted">Menyalin folder proyek (tanpa <span className="mono">.git</span>, <span className="mono">node_modules</span>, dan log) ke <span className="mono">~/MacPanel/wwwroot/&lt;nama&gt;</span>, membuat situs baru, dan database baru terpisah dengan kredensial sendiri; <span className="mono">.env</span> salinan diarahkan ke database baru sehingga staging tidak menyentuh data sumber. Meminta kata sandi ulang.</p>
        <Field label="Nama situs baru">{(id) => <input id={id} className="mono" placeholder="toko-saya-baru" pattern="[a-z0-9]([a-z0-9\-]{0,28}[a-z0-9])?" title="Huruf kecil, angka, tanda hubung" value={dst} onChange={(e) => setDst(e.target.value.toLowerCase())} maxLength={30} required autoComplete="off" spellCheck={false} />}</Field>
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
