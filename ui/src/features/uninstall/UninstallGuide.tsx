import { useState } from "react";
import { Copy, Terminal, TriangleAlert } from "lucide-react";
import { Card, Chip } from "../../ui";
import { useT } from "../../i18n";
import { applyCommand, LEVELS, planCommand, TAILSCALE } from "./model";

// Ikon Terminal: perintah dijalankan di terminal, bukan oleh panel. TriangleAlert hanya pada tingkat yang menghapus data.

export default function UninstallGuide() {
  const t = useT();
  const [note, setNote] = useState("");
  const copy = async (v: string) => {
    try {
      await navigator.clipboard.writeText(v);
      setNote(t("system.copied", { v }));
    } catch {
      setNote(t("system.clipFail"));
    }
  };
  return (
    <div className="stack">
      <Card title={t("system.un.title")}>
        <p className="small muted">
          {t("system.un.i1")} <span className="mono">--apply</span> {t("system.un.i2")}
        </p>
        <p className="small muted">
          {t("system.un.i3")}
        </p>
      </Card>
      {LEVELS.map((l) => (
        <Card
          key={l.id}
          title={
            <>
              {l.danger ? <TriangleAlert size={18} className="icon" aria-hidden="true" /> : <Terminal size={18} className="icon" aria-hidden="true" />} {t("system.un.level", { id: l.id, title: l.title })}
              {l.danger && <span style={{ marginLeft: 8 }}><Chip tone="bad">{t("system.un.deletes")}</Chip></span>}
            </>
          }
        >
          <p className="small"><strong>{t("system.un.removed")}</strong></p>
          <ul className="small bullets">{l.what.map((w) => <li key={w}>{w}</li>)}</ul>
          <p className="small muted">{l.keeps}</p>
          {([[t("system.un.step1"), planCommand(l.id)], [t("system.un.step2"), applyCommand(l.id)]] as const).map(([label, cmd]) => (
            <div key={label} className="stack">
              <span className="small muted">{label}</span>
              <div className="howto-row">
                <code className="mono howto-cmd">{cmd}</code>
                <button className="btn btn-sm" onClick={() => copy(cmd)}>
                  <Copy size={16} aria-hidden="true" /> {t("system.copyBtn")}<span className="sr">{t("system.copyCmdSr", { label })}</span>
                </button>
              </div>
            </div>
          ))}
          {l.danger && <p className="callout">{t("system.un.l3")}</p>}
        </Card>
      ))}
      <Card title={<><Terminal size={18} className="icon" aria-hidden="true" /> {t("system.un.tsTitle")}</>}>
        <p className="small muted">{t("system.un.ts1")} <span className="mono">sudo</span> {t("system.un.ts2")}</p>
        {([[t("system.un.tsPlan"), TAILSCALE.installPlan], [t("system.un.tsRun"), TAILSCALE.installApply], [t("system.un.tsLogin"), TAILSCALE.loginCmd], [t("system.un.tsStatus"), TAILSCALE.statusCmd]] as const).map(([label, cmd]) => (
          <div key={label} className="stack">
            <span className="small muted">{label}</span>
            <div className="howto-row">
              <code className="mono howto-cmd">{cmd}</code>
              <button className="btn btn-sm" onClick={() => copy(cmd)}><Copy size={16} aria-hidden="true" /> {t("system.copyBtn")}<span className="sr">{t("system.copyCmdSr", { label })}</span></button>
            </div>
          </div>
        ))}
        <hr className="sep" />
        <p><TriangleAlert size={18} className="icon" aria-hidden="true" /> <strong>{t("system.un.rmTitle")}</strong> <Chip tone="bad">{t("system.un.rmChip")}</Chip></p>
        <ul className="small bullets">{TAILSCALE.removeWarn.map((w) => <li key={w}>{w}</li>)}</ul>
        {([[t("system.un.rmPlan"), TAILSCALE.removePlan], [t("system.un.rmRun"), TAILSCALE.removeApply]] as const).map(([label, cmd]) => (
          <div key={label} className="stack">
            <span className="small muted">{label}</span>
            <div className="howto-row">
              <code className="mono howto-cmd">{cmd}</code>
              <button className="btn btn-sm" onClick={() => copy(cmd)}><Copy size={16} aria-hidden="true" /> {t("system.copyBtn")}<span className="sr">{t("system.copyCmdSr", { label })}</span></button>
            </div>
          </div>
        ))}
      </Card>
      <p className="small muted" role="status">{note}</p>
    </div>
  );
}
