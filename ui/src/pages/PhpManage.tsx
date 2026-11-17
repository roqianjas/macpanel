import { useState } from "react";
import { Download, Power, PowerOff, RefreshCw, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Chip, ConfirmDialog, Field, Modal, State, Tabs, useLoad } from "../ui";

interface Ext {
  name: string;
  label: string;
  installed: boolean;
  enabled: boolean;
  loaded: boolean;
  zend: boolean;
}
interface IniKey {
  key: string;
  value: string;
  default: string;
  hint: string;
}

const TABS = [
  { id: "ext", label: "Ekstensi" },
  { id: "ini", label: "Pengaturan" },
  { id: "fpm", label: "FPM dan log" },
];

export default function PhpManage({ version, onClose }: { version: string; onClose: () => void }) {
  const [tab, setTab] = useState("ext");
  return (
    <Modal title={`Kelola PHP ${version}`} onClose={onClose} wide>
      <Tabs tabs={TABS} value={tab} onChange={setTab} label="Bagian PHP" />
      <div role="tabpanel" aria-labelledby={`tab-${tab}`} className="tabpanel">
        {tab === "ext" ? <Extensions v={version} /> : tab === "ini" ? <Settings v={version} /> : <Fpm v={version} />}
      </div>
    </Modal>
  );
}

function Extensions({ v }: { v: string }) {
  const q = useLoad(() => get<{ builtin: string[]; managed: Ext[] }>(`/api/php/${v}/ext`), [v]);
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const [err, setErr] = useState("");
  const [rm, setRm] = useState<Ext | null>(null);
  const act = async (e: Ext, a: "install" | "remove" | "enable" | "disable") => {
    setErr("");
    setPending((p) => ({ ...p, [e.name]: true }));
    try {
      const r = await post<{ job: number }>(`/api/php/${v}/ext/${e.name}/${a}`, {});
      trackJob(r.job, `${a === "install" ? "Pasang" : a === "remove" ? "Hapus" : a === "enable" ? "Aktifkan" : "Matikan"} ${e.label} (PHP ${v})`, (j) => {
        setPending((p) => ({ ...p, [e.name]: false }));
        if (j.status !== "ok") setErr(j.output || "Gagal.");
        q.reload();
      });
    } catch (x) {
      setPending((p) => ({ ...p, [e.name]: false }));
      setErr(errText(x));
    }
  };
  const managed = q.data?.managed.filter((m) => m.name) ?? [];
  return (
    <div className="stack">
      <p className="small muted">Ekstensi tambahan dipasang lewat PECL dan hanya berlaku untuk PHP {v}. FPM dimulai ulang otomatis; pemasangan bisa memakan beberapa menit (progres di pojok kanan bawah).</p>
      {err && <Alert>{err}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <ul className="list">
          {managed.map((e) => (
            <li key={e.name} className="list-row">
              <div>
                <p>
                  <strong>{e.label}</strong> <span className="mono muted">{e.name}</span>
                </p>
                <p className="small muted">{!e.installed ? "Belum terpasang" : e.loaded ? "Terpasang dan termuat" : e.enabled ? "Terpasang, belum termuat (mulai ulang FPM)" : "Terpasang, dimatikan"}</p>
              </div>
              <div className="row">
                <Chip tone={e.loaded ? "ok" : e.installed ? "warn" : "mute"} busy={pending[e.name]}>
                  {pending[e.name] ? "Memproses" : e.loaded ? "Aktif" : e.installed ? "Nonaktif" : "Tidak ada"}
                </Chip>
                {!e.installed ? (
                  <button className="btn" disabled={pending[e.name]} onClick={() => act(e, "install")}>
                    <Download size={18} aria-hidden="true" />
                    Pasang<span className="sr"> {e.label}</span>
                  </button>
                ) : (
                  <>
                    <button className="btn" disabled={pending[e.name]} onClick={() => act(e, e.loaded ? "disable" : "enable")}>
                      {e.loaded ? <PowerOff size={18} aria-hidden="true" /> : <Power size={18} aria-hidden="true" />}
                      {e.loaded ? "Matikan" : "Aktifkan"}
                      <span className="sr"> {e.label}</span>
                    </button>
                    <button className="btn btn-danger-ghost" disabled={pending[e.name]} onClick={() => setRm(e)}>
                      <Trash2 size={18} aria-hidden="true" />
                      Hapus<span className="sr"> {e.label}</span>
                    </button>
                  </>
                )}
              </div>
            </li>
          ))}
        </ul>
        <details className="builtin">
          <summary>Ekstensi bawaan ({q.data?.builtin.length ?? 0}), sudah termasuk di PHP dan tidak perlu dipasang</summary>
          <p className="mono small chips-wrap">{q.data?.builtin.join(", ")}</p>
        </details>
      </State>
      {rm && (
        <ConfirmDialog
          title={`Hapus ${rm.label}`}
          word={rm.name}
          verb="Hapus ekstensi"
          body={<p>Ekstensi dihapus dari PHP {v}. Aplikasi yang memakainya akan error sampai dipasang lagi. Meminta kode TOTP ulang.</p>}
          onConfirm={async () => act(rm, "remove")}
          onClose={() => setRm(null)}
        />
      )}
    </div>
  );
}

function Settings({ v }: { v: string }) {
  const q = useLoad(() => get<{ keys: IniKey[] }>(`/api/php/${v}/ini`), [v]);
  const [edit, setEdit] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const changed = (q.data?.keys ?? []).filter((k) => edit[k.key] !== undefined && edit[k.key].trim() !== "" && edit[k.key].trim() !== k.value);
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const values: Record<string, string> = {};
      changed.forEach((k) => (values[k.key] = edit[k.key].trim()));
      const r = await post<{ job: number }>(`/api/php/${v}/ini`, { values });
      trackJob(r.job, `Simpan pengaturan PHP ${v}`, (j) => {
        setBusy(false);
        setNote(j.status === "ok" ? { tone: "ok", text: "Tersimpan dan FPM dimulai ulang." } : { tone: "bad", text: j.output || "Gagal." });
        setEdit({});
        q.reload();
      });
    } catch (x) {
      setBusy(false);
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <form onSubmit={save} className="stack">
      <p className="small muted">Berlaku untuk situs web (FPM) PHP {v}. Perintah di terminal (artisan, composer) memakai pengaturan PHP bawaan. Kolom kosong berarti tidak diubah; nilai bawaan PHP ditampilkan sebagai petunjuk.</p>
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="grid-form">
          {q.data?.keys.map((k) => (
            <Field key={k.key} label={k.key} hint={`${k.hint}. Saat ini: ${k.value || `bawaan (${k.default || "kosong"})`}`}>
              {(id, d) => <input id={id} aria-describedby={d} className="mono" value={edit[k.key] ?? ""} placeholder="(tidak diubah)" onChange={(e) => setEdit((p) => ({ ...p, [k.key]: e.target.value }))} spellCheck={false} />}
            </Field>
          ))}
        </div>
        <div className="row-end">
          <button className="btn btn-primary" disabled={busy || changed.length === 0}>
            {busy ? "Menyimpan…" : `Simpan dan mulai ulang FPM${changed.length ? ` (${changed.length})` : ""}`}
          </button>
        </div>
      </State>
    </form>
  );
}

function Fpm({ v }: { v: string }) {
  const log = useLoad(() => get<{ log: string }>(`/api/services/php-${v}/logs?n=80`), [v]);
  const [restart, setRestart] = useState(false);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  return (
    <div className="stack">
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <div className="row">
        <button className="btn" onClick={() => setRestart(true)}>
          <RefreshCw size={18} aria-hidden="true" />
          Mulai ulang FPM {v}
        </button>
        <button className="btn" onClick={log.reload}>Muat ulang log</button>
      </div>
      <State loading={log.loading && !log.data} error={log.error} onRetry={log.reload}>
        <pre className="logbox" tabIndex={0} aria-label={`Log FPM ${v}`}>{log.data?.log || "(log kosong)"}</pre>
      </State>
      {restart && (
        <ConfirmDialog
          title={`Mulai ulang FPM ${v}`}
          word={`php-${v}`}
          verb="Mulai ulang"
          body={<p>Situs PHP {v} berhenti menjawab beberapa detik. Meminta kode TOTP ulang.</p>}
          onConfirm={async () => {
            await post(`/api/services/php-${v}/restart`, {});
            setNote({ tone: "ok", text: `FPM ${v} dimulai ulang.` });
            log.reload();
          }}
          onClose={() => setRestart(false)}
        />
      )}
    </div>
  );
}
