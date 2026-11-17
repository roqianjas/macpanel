import { useState } from "react";
import { DatabaseBackup } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { trackJob } from "../jobs";
import { fmtBytes } from "../lib";
import { Alert, Card, PageHead, State, useLoad } from "../ui";

interface Row {
  name: string;
  at: number;
  size: number;
  files: string[];
}

export default function Backups() {
  const q = useLoad(() => get<{ backups: Row[]; offsite: boolean }>("/api/backups"));
  const [busy, setBusy] = useState(false);
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
        desc="Berjalan otomatis tiap hari pukul 02:30. Isi: dump semua database, folder ~/Developer (tanpa node_modules dan .env), dotfile, dan database panel."
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
      <Card title="Backup terakhir (14)">
        <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && rows.length === 0} emptyText="Belum ada backup. Jalankan yang pertama." onRetry={q.reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Waktu</th>
                  <th scope="col">Ukuran</th>
                  <th scope="col">Isi</th>
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
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </State>
      </Card>
      <Card title="Memulihkan">
        <p className="small">Database proyek: halaman Database, tombol Dump dan Pulihkan (hanya ke database kosong). Berkas dan konfigurasi: ekstrak dari berkas di <span className="mono">~/Backups/staging/&lt;waktu&gt;/</span> lewat terminal; panel sengaja tidak menimpa berkas.</p>
      </Card>
    </>
  );
}
