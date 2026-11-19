import { useState } from "react";
import { Copy, Terminal, TriangleAlert } from "lucide-react";
import { Card, Chip } from "../../ui";
import { applyCommand, LEVELS, planCommand } from "./model";

// Ikon Terminal: perintah dijalankan di terminal, bukan oleh panel. TriangleAlert hanya pada tingkat yang menghapus data.

export default function UninstallGuide() {
  const [note, setNote] = useState("");
  const copy = async (v: string) => {
    try {
      await navigator.clipboard.writeText(v);
      setNote(`Disalin: ${v}`);
    } catch {
      setNote("Clipboard tidak tersedia di koneksi ini; salin manual.");
    }
  };
  return (
    <div className="stack">
      <Card title="Cara uninstall">
        <p className="small muted">
          Uninstall sengaja tidak dijalankan dari panel: panel tidak punya hak root, dan menghapus dirinya sendiri akan memutus akses Anda bila gagal di tengah.
          Jalankan di terminal Mac mini atau lewat SSH. Selalu lihat rencananya dulu; tanpa <span className="mono">--apply</span> tidak ada yang diubah.
        </p>
        <p className="small muted">
          Tidak pernah disentuh: Tailscale, aplikasi RustDesk, Hermes, Homebrew, dan toolchain (mise, go, node, composer).
        </p>
      </Card>
      {LEVELS.map((l) => (
        <Card
          key={l.id}
          title={
            <>
              {l.danger ? <TriangleAlert size={18} className="icon" aria-hidden="true" /> : <Terminal size={18} className="icon" aria-hidden="true" />} Tingkat {l.id}: {l.title}
              {l.danger && <span style={{ marginLeft: 8 }}><Chip tone="bad">Menghapus data</Chip></span>}
            </>
          }
        >
          <p className="small"><strong>Yang dilepas atau dihapus</strong></p>
          <ul className="small bullets">{l.what.map((w) => <li key={w}>{w}</li>)}</ul>
          <p className="small muted">{l.keeps}</p>
          {([["1. Lihat rencana (aman)", planCommand(l.id)], ["2. Jalankan", applyCommand(l.id)]] as const).map(([label, cmd]) => (
            <div key={label} className="stack">
              <span className="small muted">{label}</span>
              <div className="howto-row">
                <code className="mono howto-cmd">{cmd}</code>
                <button className="btn btn-sm" onClick={() => copy(cmd)}>
                  <Copy size={16} aria-hidden="true" /> Salin<span className="sr"> perintah {label}</span>
                </button>
              </div>
            </div>
          ))}
          {l.danger && <p className="callout">Tingkat 3 meminta Anda mengetik ulang nama mesin, dan menolak berjalan bila backup akhir gagal diverifikasi.</p>}
        </Card>
      ))}
      <p className="small muted" role="status">{note}</p>
    </div>
  );
}
