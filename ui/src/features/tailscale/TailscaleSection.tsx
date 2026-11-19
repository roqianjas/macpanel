import { useState } from "react";
import { Copy, Power, RefreshCw, ShieldAlert, Terminal, Waypoints } from "lucide-react";
import { get } from "../../api";
import { Alert, Card, Chip, State, useLoad } from "../../ui";
import { ACTIONS, applicable, phase, PHASE_TEXT, type TsInfo } from "./model";

// Ikon: Waypoints = jalur tailnet, Power = daemon, RefreshCw = muat ulang status, ShieldAlert = tindakan yang memutus akses, Terminal = perintah.

export default function TailscaleSection() {
  const q = useLoad(() => get<TsInfo>("/api/tailscale"));
  const [note, setNote] = useState("");
  const [ack, setAck] = useState(false);
  const copy = async (v: string) => {
    try {
      await navigator.clipboard.writeText(v);
      setNote(`Disalin: ${v}`);
    } catch {
      setNote("Clipboard tidak tersedia di koneksi ini; salin manual.");
    }
  };
  const d = q.data;
  const p = d ? phase(d) : "unknown";
  const pt = PHASE_TEXT[p];
  return (
    <div className="stack">
      <Card
        title={<span className="with-icon"><Waypoints size={18} aria-hidden="true" /> Status Tailscale</span>}
        action={<button className="btn btn-sm" onClick={q.reload} disabled={q.loading}><RefreshCw size={16} aria-hidden="true" /> Muat ulang</button>}
      >
        <State loading={q.loading && !d} error={q.error} onRetry={q.reload} skeleton="cards">
          {d && (
            <>
              <ul className="list">
                <li className="list-row"><span>Status</span><Chip tone={pt.tone}>{pt.text}</Chip></li>
                <li className="list-row"><span>Paket Homebrew</span><Chip tone={d.formula ? "ok" : "mute"}>{d.formula ? "Terpasang" : "Tidak ada"}</Chip></li>
                <li className="list-row"><span>Daemon sistem (jalan sejak boot)</span><Chip tone={d.daemon_plist ? "ok" : "mute"}>{d.daemon_plist ? "Terdaftar" : "Belum terdaftar"}</Chip></li>
                {d.tailnet?.ip && <li className="list-row"><span>IP Tailscale</span><span className="mono">{d.tailnet.ip}</span></li>}
                {d.tailnet?.name && <li className="list-row"><span>Nama di tailnet</span><span className="mono">{d.tailnet.name}</span></li>}
                {d.tailnet?.version && <li className="list-row"><span>Versi</span><span className="mono">{d.tailnet.version}</span></li>}
                {d.tailnet && <li className="list-row"><span>Perangkat lain</span><span>{d.tailnet.peers_online} online dari {d.tailnet.peers}</span></li>}
              </ul>
              {d.tailnet && d.tailnet.health.length > 0 && <Alert tone="warn">{d.tailnet.health.join("; ")}</Alert>}
              {d.error && !d.tailnet && <Alert tone="warn">{d.error}</Alert>}
              {d.tailnet && !d.tailnet.self_online && p === "running" && <Alert tone="warn">Daemon berjalan tetapi mesin ini belum online di tailnet.</Alert>}
            </>
          )}
        </State>
      </Card>

      <Card title={<span className="with-icon"><Power size={18} aria-hidden="true" /> Kelola</span>}>
        <p className="small muted">
          Panel hanya menampilkan perintah. Semua yang mengubah Tailscale butuh <span className="mono">sudo</span> atau mengubah akses, jadi dijalankan di terminal Mac mini (layar atau SSH). Tanpa <span className="mono">--apply</span> perintah <span className="mono">lab</span> hanya menampilkan rencana.
        </p>
        <Alert tone="warn">
          <ShieldAlert size={16} className="icon" aria-hidden="true" /> Panel dan SSH Anda terjangkau lewat tailnet. Tindakan bertanda <strong>memutus akses</strong> membuat Anda tidak bisa masuk dari jauh sampai Tailscale dinyalakan lagi dari layar Mac mini. <span className="mono">lab</span> menolak menjalankannya dari sesi SSH yang lewat tailnet kecuali dengan <span className="mono">--allow-remote-cut</span>.
        </Alert>
        <label className="check check-block">
          <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} />
          <span>Saya paham dan sedang di depan Mac mini atau punya jalur masuk lain (tampilkan perintah yang memutus akses)</span>
        </label>
        <ul className="ts-actions">
          {ACTIONS.map((a) => {
            const ap = applicable(a, p);
            const locked = a.danger && !ack;
            return (
              <li key={a.id} className={`ts-act ${ap.ok ? "" : "off"}`}>
                <div className="ts-act-head">
                  <strong>{a.title}</strong>
                  {a.danger && <Chip tone="bad">Memutus akses</Chip>}
                  {a.sudo && <Chip tone="mute">sudo</Chip>}
                  {!ap.ok && <span className="small muted">{ap.reason}</span>}
                </div>
                <p className="small muted">{a.why}</p>
                {ap.ok && !locked && (
                  <div className="stack">
                    {a.plan && <CmdRow label={`Lihat rencana ${a.title}`} cmd={a.plan} onCopy={copy} />}
                    <CmdRow label={`Jalankan ${a.title}`} cmd={a.run} onCopy={copy} primary />
                  </div>
                )}
                {ap.ok && locked && <p className="small muted">Centang persetujuan di atas untuk menampilkan perintahnya.</p>}
              </li>
            );
          })}
        </ul>
        <p className="small muted" role="status">{note}</p>
      </Card>
    </div>
  );
}

function CmdRow({ label, cmd, onCopy, primary }: { label: string; cmd: string; onCopy: (v: string) => void; primary?: boolean }) {
  return (
    <div className="howto-row">
      <Terminal size={16} className="icon" aria-hidden="true" />
      <code className="mono howto-cmd">{cmd}</code>
      <button className={`btn btn-sm ${primary ? "" : "btn-ghost"}`} onClick={() => onCopy(cmd)}>
        <Copy size={16} aria-hidden="true" /> Salin<span className="sr"> perintah: {label}</span>
      </button>
    </div>
  );
}
