import { useState } from "react";
import { Copy, Terminal, TriangleAlert } from "lucide-react";
import { Card, Chip } from "../../ui";
import { applyCommand, LEVELS, planCommand, TAILSCALE } from "./model";

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
          Tingkat 1-3 tidak pernah menyentuh Tailscale (lihat kartu Tailscale di bawah), aplikasi RustDesk, Hermes, Homebrew, dan toolchain (mise, go, node, composer).
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
      <Card title={<><Terminal size={18} className="icon" aria-hidden="true" /> Tailscale: pasang atau lepas</>}>
        <p className="small muted">Tailscale bukan bagian dari tingkat 1-3 karena akses jarak jauh ke panel dan SSH bergantung padanya. Panel hanya menampilkan perintahnya; pemasangan butuh <span className="mono">sudo</span> dan login lewat browser, keduanya oleh Anda.</p>
        {([["Pasang: lihat rencana (aman)", TAILSCALE.installPlan], ["Pasang: jalankan", TAILSCALE.installApply], ["Lalu login ke tailnet (browser)", TAILSCALE.loginCmd], ["Cek status", TAILSCALE.statusCmd]] as const).map(([label, cmd]) => (
          <div key={label} className="stack">
            <span className="small muted">{label}</span>
            <div className="howto-row">
              <code className="mono howto-cmd">{cmd}</code>
              <button className="btn btn-sm" onClick={() => copy(cmd)}><Copy size={16} aria-hidden="true" /> Salin<span className="sr"> perintah {label}</span></button>
            </div>
          </div>
        ))}
        <hr className="sep" />
        <p><TriangleAlert size={18} className="icon" aria-hidden="true" /> <strong>Lepas Tailscale (tingkat 4)</strong> <Chip tone="bad">Memutus akses jarak jauh</Chip></p>
        <ul className="small bullets">{TAILSCALE.removeWarn.map((w) => <li key={w}>{w}</li>)}</ul>
        {([["Lihat rencana (aman)", TAILSCALE.removePlan], ["Jalankan", TAILSCALE.removeApply]] as const).map(([label, cmd]) => (
          <div key={label} className="stack">
            <span className="small muted">{label}</span>
            <div className="howto-row">
              <code className="mono howto-cmd">{cmd}</code>
              <button className="btn btn-sm" onClick={() => copy(cmd)}><Copy size={16} aria-hidden="true" /> Salin<span className="sr"> perintah {label}</span></button>
            </div>
          </div>
        ))}
      </Card>
      <p className="small muted" role="status">{note}</p>
    </div>
  );
}
