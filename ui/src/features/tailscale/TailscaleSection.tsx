import { useState } from "react";
import { Copy, Power, RefreshCw, ShieldAlert, Terminal, Waypoints } from "lucide-react";
import { get } from "../../api";
import { useT } from "../../i18n";
import { Alert, Card, Chip, State, useLoad } from "../../ui";
import { ACTIONS, applicable, phase, PHASE_TEXT, type TsInfo } from "./model";

// Ikon: Waypoints = jalur tailnet, Power = daemon, RefreshCw = muat ulang status, ShieldAlert = tindakan yang memutus akses, Terminal = perintah.

export default function TailscaleSection() {
  const t = useT();
  const q = useLoad(() => get<TsInfo>("/api/tailscale"));
  const [note, setNote] = useState("");
  const [ack, setAck] = useState(false);
  const copy = async (v: string) => {
    try {
      await navigator.clipboard.writeText(v);
      setNote(t("system.copied", { v }));
    } catch {
      setNote(t("system.clipFail"));
    }
  };
  const d = q.data;
  const p = d ? phase(d) : "unknown";
  const pt = PHASE_TEXT[p];
  return (
    <div className="stack">
      <Card
        title={<span className="with-icon"><Waypoints size={18} aria-hidden="true" /> {t("system.ts.title")}</span>}
        action={<button className="btn btn-sm" onClick={q.reload} disabled={q.loading}><RefreshCw size={16} aria-hidden="true" /> {t("common.reload")}</button>}
      >
        <State loading={q.loading && !d} error={q.error} onRetry={q.reload} skeleton="cards">
          {d && (
            <>
              <ul className="list">
                <li className="list-row"><span>{t("apps.col.status")}</span><Chip tone={pt.tone}>{pt.text}</Chip></li>
                <li className="list-row"><span>{t("system.ts.brew")}</span><Chip tone={d.formula ? "ok" : "mute"}>{d.formula ? t("system.ts.installed") : t("system.ts.absent")}</Chip></li>
                <li className="list-row"><span>{t("system.ts.daemon")}</span><Chip tone={d.daemon_plist ? "ok" : "mute"}>{d.daemon_plist ? t("system.ts.registered") : t("system.ts.unregistered")}</Chip></li>
                {d.tailnet?.ip && <li className="list-row"><span>{t("system.ts.ip")}</span><span className="mono">{d.tailnet.ip}</span></li>}
                {d.tailnet?.name && <li className="list-row"><span>{t("system.ts.name")}</span><span className="mono">{d.tailnet.name}</span></li>}
                {d.tailnet?.version && <li className="list-row"><span>{t("apps.col.version")}</span><span className="mono">{d.tailnet.version}</span></li>}
                {d.tailnet && <li className="list-row"><span>{t("system.ts.peers")}</span><span>{t("system.ts.peersVal", { online: d.tailnet.peers_online, total: d.tailnet.peers })}</span></li>}
              </ul>
              {d.tailnet && d.tailnet.health.length > 0 && <Alert tone="warn">{d.tailnet.health.join("; ")}</Alert>}
              {d.error && !d.tailnet && <Alert tone="warn">{d.error}</Alert>}
              {d.tailnet && !d.tailnet.self_online && p === "running" && <Alert tone="warn">{t("system.ts.notOnline")}</Alert>}
            </>
          )}
        </State>
      </Card>

      <Card title={<span className="with-icon"><Power size={18} aria-hidden="true" /> {t("system.ts.manage")}</span>}>
        <p className="small muted">
          {t("system.ts.mg1")} <span className="mono">sudo</span> {t("system.ts.mg2")} <span className="mono">--apply</span> {t("system.ts.mg3")} <span className="mono">lab</span> {t("system.ts.mg4")}
        </p>
        <Alert tone="warn">
          <ShieldAlert size={16} className="icon" aria-hidden="true" /> {t("system.ts.warn1")} <strong>{t("system.ts.warn2")}</strong> {t("system.ts.warn3")} <span className="mono">lab</span> {t("system.ts.warn4")} <span className="mono">--allow-remote-cut</span>.
        </Alert>
        <label className="check check-block">
          <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} />
          <span>{t("system.ts.ack")}</span>
        </label>
        <ul className="ts-actions">
          {ACTIONS.map((a) => {
            const ap = applicable(a, p);
            const locked = a.danger && !ack;
            return (
              <li key={a.id} className={`ts-act ${ap.ok ? "" : "off"}`}>
                <div className="ts-act-head">
                  <strong>{a.title}</strong>
                  {a.danger && <Chip tone="bad">{t("system.ts.cuts")}</Chip>}
                  {a.sudo && <Chip tone="mute">sudo</Chip>}
                  {!ap.ok && <span className="small muted">{ap.reason}</span>}
                </div>
                <p className="small muted">{a.why}</p>
                {ap.ok && !locked && (
                  <div className="stack">
                    {a.plan && <CmdRow label={t("system.ts.plan", { title: a.title })} cmd={a.plan} onCopy={copy} />}
                    <CmdRow label={t("system.ts.run", { title: a.title })} cmd={a.run} onCopy={copy} primary />
                  </div>
                )}
                {ap.ok && locked && <p className="small muted">{t("system.ts.locked")}</p>}
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
  const t = useT();
  return (
    <div className="howto-row">
      <Terminal size={16} className="icon" aria-hidden="true" />
      <code className="mono howto-cmd">{cmd}</code>
      <button className={`btn btn-sm ${primary ? "" : "btn-ghost"}`} onClick={() => onCopy(cmd)}>
        <Copy size={16} aria-hidden="true" /> {t("system.copyBtn")}<span className="sr">{t("system.ts.copySr", { label })}</span>
      </button>
    </div>
  );
}
