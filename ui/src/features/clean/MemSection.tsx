import { useState } from "react";
import { Copy, Gauge, MemoryStick, Power } from "lucide-react";
import { get, post } from "../../api";
import { fmtBytes } from "../../lib";
import { Card, Chip, ConfirmDialog, State, useLoad } from "../../ui";
import { appUsedMb, pressure, PURGE_COMMAND, type MemSuggestion, type MemView } from "./model";

// Ikon: MemoryStick = memori, Gauge = tekanan, Power = matikan layanan opsional, Copy = salin perintah.

const mb = (v: number | undefined) => (v == null ? "n/a" : fmtBytes(v * 1048576));

export default function MemSection() {
  const q = useLoad(() => get<MemView>("/api/mem"));
  const [stop, setStop] = useState<MemSuggestion | null>(null);
  const [note, setNote] = useState("");
  const d = q.data;
  const p = pressure(d?.mem.pressure_free_pct);
  const used = d ? appUsedMb(d.mem) : null;
  const max = Math.max(1, ...(d?.groups.map((x) => x.mb) ?? [1]));
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(PURGE_COMMAND);
      setNote(`Disalin: ${PURGE_COMMAND}`);
    } catch {
      setNote("Clipboard tidak tersedia di koneksi ini; salin manual.");
    }
  };
  return (
    <div className="stack">
      <Card
        title={
          <span className="with-icon">
            <MemoryStick aria-hidden="true" size={18} /> Memori
          </span>
        }
        action={<button className="btn" onClick={q.reload}>Perbarui</button>}
      >
        <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
          {d && (
            <>
              <p className="with-icon">
                <Gauge aria-hidden="true" size={16} /> Tekanan memori: <Chip tone={p.tone}>{p.label}{d.mem.pressure_free_pct != null ? ` (${d.mem.pressure_free_pct}% bebas)` : ""}</Chip>
              </p>
              <ul className="list">
                <li className="list-row"><span>Total</span><span className="mono">{mb(d.mem.total_mb)}</span></li>
                <li className="list-row"><span>Dipakai aplikasi dan sistem (wired + aktif + terkompresi)</span><span className="mono">{used == null ? "n/a" : mb(used)}</span></li>
                <li className="list-row"><span>Cache file yang bisa dilepas macOS kapan saja</span><span className="mono">{mb(d.mem.inactive_mb)}</span></li>
                <li className="list-row"><span>Swap terpakai</span><span className="mono">{d.mem.swap_used_mb == null ? "n/a" : mb(d.mem.swap_used_mb)}</span></li>
              </ul>
              <p className="small muted">Angka “memori terpakai” di panel lain sering memasukkan cache file, jadi terlihat penuh padahal sehat. Yang perlu diwaspadai adalah tekanan “Tertekan” dan swap yang terus naik.</p>
            </>
          )}
        </State>
      </Card>

      {d && (
        <>
          <Card title="Pemakai memori terbesar per layanan">
            <ul className="bars">
              {d.groups.map((x) => (
                <li key={x.label} className="bar-row">
                  <span className="bar-name">{x.label}{x.procs > 1 ? ` (${x.procs} proses)` : ""}</span>
                  <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (x.mb / max) * 100)}%` }} /></span>
                  <span className="mono bar-val">{mb(x.mb)}</span>
                </li>
              ))}
            </ul>
            <p className="small muted">Ukuran resident per kelompok proses; layanan yang berbagi memori bisa terhitung dobel. Untuk menghentikan satu proses pengembangan, pakai tab Proses.</p>
          </Card>

          <Card title="Saran penghematan">
            {d.suggestions.length === 0 ? (
              <p className="muted">Tidak ada layanan opsional atau PHP-FPM tak terpakai yang sedang berjalan.</p>
            ) : (
              <ul className="list">
                {d.suggestions.map((s) => (
                  <li key={s.id} className="list-row clean-row">
                    <div>
                      <p>{s.label}{s.mb != null ? <span className="mono"> · ±{mb(s.mb)}</span> : null}</p>
                      <p className="small muted">{s.text}</p>
                    </div>
                    <button className="btn" onClick={() => setStop(s)}><Power aria-hidden="true" size={16} /> Matikan…</button>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card title="Lepas cache memori (sudo purge)">
            <p className="small muted">{d.purge_note} Hasilnya angka “terpakai” turun sebentar, lalu naik lagi saat aplikasi membaca ulang dari SSD. Tidak mengurangi pemakaian aplikasi. Menurut saya tidak perlu, tetapi tersedia bila Anda mau.</p>
            <p className="small muted">Panel tidak punya hak root, jadi jalankan sendiri di terminal Mac mini:</p>
            <div className="row-end">
              <code className="mono">{PURGE_COMMAND}</code>
              <button className="btn" onClick={copy}><Copy aria-hidden="true" size={16} /> Salin</button>
            </div>
            {note && <p className="small muted" role="status">{note}</p>}
          </Card>
        </>
      )}

      {stop && (
        <ConfirmDialog
          title={`Matikan ${stop.label}?`}
          word={stop.id}
          verb="Matikan"
          onClose={() => setStop(null)}
          body={<p>{stop.text} Layanan bisa dinyalakan lagi dari halaman Aplikasi.</p>}
          onConfirm={async () => {
            await post(`/api/services/${stop.id}/stop`, {});
            q.reload();
          }}
        />
      )}
    </div>
  );
}
