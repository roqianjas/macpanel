import { useState } from "react";
import { Download, RefreshCw, Trash2 } from "lucide-react";
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
  group?: string;
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

function Switch({ on, disabled, label, onChange }: { on: boolean; disabled?: boolean; label: string; onChange?: () => void }) {
  return (
    <button type="button" role="switch" aria-checked={on} aria-label={label} disabled={disabled} className={`switch ${on ? "on" : ""}`} onClick={onChange}>
      <span className="switch-knob" />
    </button>
  );
}

type Filter = "all" | "toggle" | "builtin";

function Extensions({ v }: { v: string }) {
  const q = useLoad(() => get<{ builtin: string[]; managed: Ext[] }>(`/api/php/${v}/ext`), [v]);
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const [err, setErr] = useState("");
  const [rm, setRm] = useState<Ext | null>(null);
  const [filter, setFilter] = useState<Filter>("all");
  const [term, setTerm] = useState("");
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
  const managed = (q.data?.managed ?? []).filter((m) => m.name);
  const builtin = q.data?.builtin ?? [];
  const t = term.trim().toLowerCase();
  const mShow = filter === "builtin" ? [] : managed.filter((m) => !t || m.name.includes(t) || m.label.toLowerCase().includes(t));
  const bShow = filter === "toggle" ? [] : builtin.filter((b) => !t || b.toLowerCase().includes(t));
  const FILTERS: { id: Filter; label: string }[] = [
    { id: "all", label: `Semua (${managed.length + builtin.length})` },
    { id: "toggle", label: `Bisa diatur (${managed.length})` },
    { id: "builtin", label: `Bawaan (${builtin.length})` },
  ];
  return (
    <div className="stack">
      <div className="ext-bar">
        <label className="sr" htmlFor="ext-q">Cari ekstensi</label>
        <input id="ext-q" type="search" placeholder="Cari ekstensi…" value={term} onChange={(e) => setTerm(e.target.value)} autoComplete="off" />
        <div className="seg" role="group" aria-label="Filter ekstensi">
          {FILTERS.map((f) => (
            <button key={f.id} type="button" className={`seg-btn ${filter === f.id ? "on" : ""}`} aria-pressed={filter === f.id} onClick={() => setFilter(f.id)}>
              {f.label}
            </button>
          ))}
        </div>
      </div>
      {err && <Alert>{err}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && mShow.length + bShow.length === 0} emptyText="Tidak ada ekstensi yang cocok." onRetry={q.reload}>
        <div className="ext-list" role="list">
          {mShow.map((e) => (
            <div key={e.name} role="listitem" className="ext-row">
              <div className="ext-name">
                <strong className="mono">{e.name}</strong>
                <span className="small muted">{e.label}</span>
              </div>
              <Chip tone="info">Tambahan</Chip>
              <span className="small muted ext-state">{!e.installed ? "Belum terpasang" : e.loaded ? "Aktif" : "Dimatikan"}</span>
              <div className="ext-act">
                {e.installed ? (
                  <>
                    <Switch on={e.loaded} disabled={pending[e.name]} label={`${e.loaded ? "Matikan" : "Aktifkan"} ${e.name}`} onChange={() => act(e, e.loaded ? "disable" : "enable")} />
                    <button className="btn btn-icon btn-danger-ghost" disabled={pending[e.name]} onClick={() => setRm(e)} aria-label={`Hapus ${e.name}`} title="Hapus ekstensi">
                      <Trash2 size={18} aria-hidden="true" />
                    </button>
                  </>
                ) : (
                  <button className="btn btn-sm" disabled={pending[e.name]} onClick={() => act(e, "install")}>
                    <Download size={16} aria-hidden="true" />
                    {pending[e.name] ? "Memasang…" : "Pasang"}
                    <span className="sr"> {e.name}</span>
                  </button>
                )}
              </div>
            </div>
          ))}
          {bShow.map((b) => (
            <div key={b} role="listitem" className="ext-row">
              <div className="ext-name">
                <strong className="mono">{b}</strong>
              </div>
              <Chip tone="mute">Bawaan</Chip>
              <span className="small muted ext-state">Aktif, permanen</span>
              <div className="ext-act">
                <Switch on disabled label={`${b}: bawaan PHP, tidak bisa dimatikan`} />
              </div>
            </div>
          ))}
        </div>
        <p className="small muted">Ekstensi <strong>Bawaan</strong> dikompilasi permanen ke PHP versi Homebrew ini, jadi tidak bisa dimatikan satu per satu (berbeda dengan cPanel/aaPanel yang memakai modul terpisah). Ekstensi <strong>Tambahan</strong> dipasang lewat PECL per versi PHP dan bisa diaktifkan, dimatikan, atau dihapus. FPM dimulai ulang otomatis; pemasangan bisa beberapa menit.</p>
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
        {[
          { g: "php", t: "Batas PHP" },
          { g: "fpm", t: "Pool FPM (jumlah proses)" },
        ].map((sec) => (
          <section key={sec.g} className="stack" aria-label={sec.t}>
            <h3 className="sec-title">{sec.t}</h3>
            <div className="grid-form">
              {q.data?.keys
                .filter((k) => (k.group ?? "php") === sec.g)
                .map((k) => (
                  <Field key={k.key} label={k.key} hint={`${k.hint}. Saat ini: ${k.value || `bawaan${k.default ? ` (${k.default})` : ""}`}`}>
                    {(id, dd) => <input id={id} aria-describedby={dd} className="mono" value={edit[k.key] ?? ""} placeholder="(tidak diubah)" onChange={(e) => setEdit((p) => ({ ...p, [k.key]: e.target.value }))} spellCheck={false} />}
                  </Field>
                ))}
            </div>
          </section>
        ))}
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
