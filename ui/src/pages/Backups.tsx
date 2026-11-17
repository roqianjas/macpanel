import { useState } from "react";
import { DatabaseBackup } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { trackJob } from "../jobs";
import { fmtBytes } from "../lib";
import { Alert, Card, Field, Modal, PageHead, State, useLoad } from "../ui";

interface Row {
  name: string;
  at: number;
  size: number;
  files: string[];
}

export default function Backups() {
  const q = useLoad(() => get<{ backups: Row[]; offsite: boolean }>("/api/backups"));
  const [busy, setBusy] = useState(false);
  const [browse, setBrowse] = useState<string | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const run = async () => {
    setBusy(true);
    setNote(null);
    try {
      const r = await post<{ job: number }>("/api/backups/run", {});
      trackJob(r.job, "Backup lokal", (j) => {
        setBusy(false);
        setNote(j.status === "ok" ? { tone: "ok", text: "Backup selesai." } : { tone: "bad", text: j.output || "Backup gagal." });
        q.reload();
      });
    } catch (x) {
      setBusy(false);
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const rows = q.data?.backups ?? [];
  return (
    <>
      <PageHead
        title="Backup"
        desc="Berjalan otomatis tiap hari (jam dan jumlah simpanan diatur di kartu Jadwal di bawah). Isi: dump semua database, folder ~/Developer (tanpa node_modules dan .env), dotfile, dan database panel."
        action={
          <button className="btn btn-primary" disabled={busy} onClick={run}>
            <DatabaseBackup size={18} aria-hidden="true" />
            {busy ? "Berjalan…" : "Jalankan sekarang"}
          </button>
        }
      />
      {q.data && !q.data.offsite && (
        <Alert tone="warn">
          Semua backup ini tersimpan di disk yang sama dengan Mac mini. Jika disk rusak atau Mac hilang, backup ikut hilang. Salinan ke disk eksternal atau cloud belum disiapkan. Private key SSH dan kunci TOTP panel sengaja tidak ikut.
        </Alert>
      )}
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Schedule />
      <Card title="Backup terakhir (14)">
        <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && rows.length === 0} emptyText="Belum ada backup. Jalankan yang pertama." onRetry={q.reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Waktu</th>
                  <th scope="col">Ukuran</th>
                  <th scope="col">Isi</th>
                  <th scope="col"><span className="sr">Aksi</span></th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.name}>
                    <td className="nowrap">{fmtTime(r.at)}</td>
                    <td className="nowrap">{fmtBytes(r.size)}</td>
                    <td className="mono small cell-trunc" title={r.files.join(", ")}>
                      {r.files.join(", ")}
                    </td>
                    <td className="cell-end">
                      {r.files.includes("developer.tar.gz") && (
                        <button className="btn btn-sm" onClick={() => setBrowse(r.name)}>
                          Jelajahi<span className="sr"> backup {r.name}</span>
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </State>
      </Card>
      {browse && <Browse name={browse} onClose={() => setBrowse(null)} />}
      <Card title="Memulihkan">
        <p className="small">Database proyek: halaman Database, tombol Dump dan Pulihkan (hanya ke database kosong). Berkas dan konfigurasi: ekstrak dari berkas di <span className="mono">~/Backups/staging/&lt;waktu&gt;/</span> lewat terminal; panel sengaja tidak menimpa berkas.</p>
      </Card>
    </>
  );
}

function Schedule() {
  const q = useLoad(() => get<{ time: string; keep: number }>("/api/backups/config"));
  const [time, setTime] = useState("");
  const [keep, setKeep] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const t = time || q.data?.time || "02:30";
  const k = keep || String(q.data?.keep ?? 7);
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      await post("/api/backups/config", { time: t, keep: Number(k) });
      setMsg({ tone: "ok", text: "Jadwal diperbarui di launchd." });
      setTime("");
      setKeep("");
      q.reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Jadwal dan retensi">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <form onSubmit={save} className="stack">
          <div className="grid2">
            <Field label="Jam backup harian" hint="Zona waktu Mac mini">{(id, dd) => <input id={id} aria-describedby={dd} type="time" value={t} onChange={(e) => setTime(e.target.value)} required />}</Field>
            <Field label="Simpan backup terakhir" hint="1-60. Yang lebih lama dihapus otomatis.">{(id, dd) => <input id={id} aria-describedby={dd} type="number" min={1} max={60} value={k} onChange={(e) => setKeep(e.target.value)} required />}</Field>
          </div>
          {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
          <div>
            <button className="btn" disabled={busy}>{busy ? "Menyimpan…" : "Simpan jadwal"}</button>
          </div>
        </form>
      </State>
    </Card>
  );
}

function Browse({ name, onClose }: { name: string; onClose: () => void }) {
  const [part, setPart] = useState<"developer" | "dotfiles">("developer");
  const [prefix, setPrefix] = useState("");
  const q = useLoad(() => get<{ entries: { name: string; dir: boolean }[] }>(`/api/backups/ls?name=${name}&part=${part}&prefix=${encodeURIComponent(prefix)}`), [name, part, prefix]);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const extract = async (path: string) => {
    setMsg(null);
    try {
      const r = await post<{ job: number }>("/api/backups/extract", { name, part, path });
      trackJob(r.job, `Ekstrak ${path}`, (j) => setMsg(j.status === "ok" ? { tone: "ok", text: j.output.split("\n").filter(Boolean).pop() ?? "Selesai." } : { tone: "bad", text: j.output || "Gagal." }));
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    }
  };
  const crumbs = prefix ? prefix.split("/") : [];
  return (
    <Modal title={`Jelajahi backup ${name}`} onClose={onClose} wide>
      <div className="stack">
        <div className="seg" role="group" aria-label="Bagian backup">
          {(["developer", "dotfiles"] as const).map((p) => (
            <button key={p} type="button" className={`seg-btn ${part === p ? "on" : ""}`} aria-pressed={part === p} onClick={() => { setPart(p); setPrefix(""); }}>
              {p === "developer" ? "Folder Developer" : "Dotfile"}
            </button>
          ))}
        </div>
        <p className="small mono">
          <button type="button" className="linklike" onClick={() => setPrefix("")}>/</button>
          {crumbs.map((c, i) => (
            <span key={i}> / <button type="button" className="linklike" onClick={() => setPrefix(crumbs.slice(0, i + 1).join("/"))}>{c}</button></span>
          ))}
        </p>
        {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
        <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && q.data.entries.length === 0} emptyText="Kosong." onRetry={q.reload}>
          <ul className="list ext-list">
            {q.data?.entries.map((e) => {
              const full = prefix ? `${prefix}/${e.name}` : e.name;
              return (
                <li key={e.name} className="list-row">
                  {e.dir ? (
                    <button type="button" className="linklike mono" onClick={() => setPrefix(full)}>{e.name}/</button>
                  ) : (
                    <span className="mono">{e.name}</span>
                  )}
                  <button className="btn btn-sm" onClick={() => extract(full)}>
                    Ekstrak<span className="sr"> {full}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        </State>
        <p className="small muted">Ekstrak menyalin ke <span className="mono">~/Backups/restore/&lt;waktu&gt;/</span>. Berkas asli tidak pernah ditimpa; salin manual bila perlu. Meminta kode TOTP ulang.</p>
      </div>
    </Modal>
  );
}
