import { useState } from "react";
import { useT } from "../../i18n";
import { Copy, ShieldAlert, ShieldCheck, Terminal } from "lucide-react";
import { errText, fmtTime, get, post } from "../../api";
import { Alert, Card, Chip, ConfirmDialog, State, usePoll } from "../../ui";
import { closeCommand, displayName, isDefaultEntry, loopbackCount, statusInfo, summary, visibleItems, type ExItem, type ExView } from "./model";

// Ikon: ShieldAlert bila ada layanan yang perlu perhatian, ShieldCheck bila tidak. Keduanya menandai keadaan nyata (ada/tidak ada paparan),
// bukan hiasan; bentuknya berbeda sehingga status tidak hanya bergantung pada warna.

type Note = { tone: "ok" | "bad"; text: string } | null;

export default function ExposureSection() {
  // Data berubah tiap 5 menit di server; 60 detik cukup untuk melihat hasil setelah mengubah daftar izin.
  const t = useT();
  const q = usePoll(() => get<ExView>("/api/exposure"), 60000);
  const [ask, setAsk] = useState<ExItem | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState("");
  const [howto, setHowto] = useState<ExItem | null>(null);
  const [copied, setCopied] = useState("");
  const v = q.data;
  const flagged = v ? v.items.filter((i) => i.status === "flagged").length : 0;

  const revoke = async (entry: string) => {
    setBusy(entry);
    setNote(null);
    try {
      await post("/api/exposure/revoke", { entry });
      await q.reload();
      setNote({ tone: "ok", text: t("system.ex.revoked", { entry }) });
    } catch (e) {
      setNote({ tone: "bad", text: errText(e) });
    } finally {
      setBusy("");
    }
  };

  return (
    <Card title={<>{flagged > 0 ? <ShieldAlert size={18} className="icon" aria-hidden="true" /> : <ShieldCheck size={18} className="icon" aria-hidden="true" />} {t("system.ex.title")}</>}>
      <State loading={q.loading && !v} error={q.error} onRetry={q.reload}>
        {v && (
          <div className="stack">
            {note && <Alert tone={note.tone}>{note.text}</Alert>}
            <p role="status" aria-live="polite" className={flagged > 0 ? "" : "muted"}>
              {summary(v)}
            </p>
            {v.now && (
              <p className="small muted">
                {t("system.ex.checked", { at: fmtTime(v.now.at), ip: v.now.lan_ip ?? t("system.ex.noip"), min: Math.round(v.interval / 60), samples: v.confirm_samples, count: loopbackCount(v.items) })}
              </p>
            )}
            {v.now && v.now.errors.length > 0 && <Alert tone="bad">{v.now.errors.join(" ")}</Alert>}
            {visibleItems(v.items).length > 0 && (
              <div className="tbl-wrap">
                <table className="tbl">
                  <caption className="sr">{t("system.ex.caption")}</caption>
                  <thead>
                    <tr>
                      <th scope="col">{t("apps.col.service")}</th>
                      <th scope="col">{t("apps.col.port")}</th>
                      <th scope="col">{t("system.address")}</th>
                      <th scope="col">{t("apps.col.status")}</th>
                      <th scope="col">
                        <span className="sr">{t("system.action")}</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleItems(v.items).map((i) => {
                      const s = statusInfo(i.status);
                      return (
                        <tr key={`${i.key}@${i.bind}`}>
                          <th scope="row" className="mono">
                            {displayName(i)}
                            {i.system && <span className="muted small"> (macOS)</span>}
                          </th>
                          <td className="mono">{i.port}</td>
                          <td className="mono">{i.bind === "?" ? t("system.ex.bindAll") : i.bind === "*" ? t("system.ex.bindAny") : i.bind}</td>
                          <td>
                            <Chip tone={s.tone}>{s.label}</Chip>
                            <span className="sr"> {s.hint}</span>
                          </td>
                          <td className="cell-end">
                            <div className="row">
                            {closeCommand(i) && (
                              <button className="btn btn-sm" aria-expanded={howto?.key === i.key} onClick={() => setHowto(howto?.key === i.key ? null : i)}>
                                <Terminal size={16} aria-hidden="true" /> {t("system.ex.howto")}<span className="sr"> {t("system.ex.srPort", { name: displayName(i), port: i.port })}</span>
                              </button>
                            )}
                            {i.status === "flagged" && v.can_edit && (
                              <button className="btn btn-sm" onClick={() => setAsk(i)}>
                                {t("system.ex.mark")}<span className="sr"> {t("system.ex.srPort", { name: displayName(i), port: i.port })}</span>
                              </button>
                            )}
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
            {howto && closeCommand(howto) && (
              <div className="howto" role="region" aria-label={t("system.ex.howtoAria", { port: howto.port })}>
                <p className="small">
                  <strong>{t("system.ex.closeHead", { port: howto.port })}</strong> {t("system.ex.run1")}{" "}
                  <span className="mono">sudo</span> {t("system.ex.run2")}
                </p>
                <div className="howto-row">
                  <code className="mono howto-cmd">{closeCommand(howto)}</code>
                  <button
                    className="btn btn-sm"
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(closeCommand(howto) || "");
                        setCopied(t("system.ex.copiedCmd"));
                      } catch {
                        setCopied(t("system.clipFail"));
                      }
                    }}
                  >
                    <Copy size={16} aria-hidden="true" /> {t("system.ex.copyCmd")}
                  </button>
                </div>
                <p className="small muted" role="status">
                  {copied || t("system.ex.tailnetOk")} {t("system.ex.checkRules")} <span className="mono">lab firewall verify</span>{t("system.ex.undoWith")}{" "}
                  <span className="mono">lab firewall unblock {howto.port}</span>.
                </p>
              </div>
            )}
            <div className="stack">
              <h3 className="h3">{t("system.ex.allowTitle")}</h3>
              <p className="small muted">{t("system.ex.allowHint")}</p>
              {v.allowed.length === 0 ? (
                <p className="small muted">{t("list.empty")}</p>
              ) : (
                <ul className="list">
                  {v.allowed.map((e) => (
                    <li key={e} className="list-row">
                      <span className="mono">
                        {e}
                        {isDefaultEntry(e, v.defaults) && <span className="muted small"> {t("system.ex.default")}</span>}
                      </span>
                      {v.can_edit && (
                        <button className="btn btn-sm" disabled={busy === e} onClick={() => revoke(e)}>
                          {busy === e ? t("system.ex.revoking") : t("system.ex.revoke")}
                          <span className="sr"> {e}</span>
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              )}
              {!v.can_edit && <p className="small muted">{t("system.ex.adminOnly")}</p>}
            </div>
          </div>
        )}
      </State>
      {ask && (
        <ConfirmDialog
          title={t("system.ex.confirmTitle", { name: displayName(ask), port: ask.port })}
          word={t("system.ex.word")}
          verb={t("system.ex.mark")}
          body={
            <p>
              {t("system.ex.confirm1")} <span className="mono">{ask.key}</span> {t("system.ex.confirm2")}
            </p>
          }
          onConfirm={async () => {
            await post("/api/exposure/allow", { entry: ask.key });
            await q.reload();
            setNote({ tone: "ok", text: t("system.ex.allowed", { entry: ask.key }) });
          }}
          onClose={() => setAsk(null)}
        />
      )}
    </Card>
  );
}
