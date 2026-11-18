import { useState } from "react";
import { errText, fmtTime, get, post } from "../../api";
import { Alert, Card, Chip, State, useLoad } from "../../ui";

export function Ssl({ name }: { name: string }) {
  const { data, error, loading, reload } = useLoad(() => get<any>(`/api/sites/${name}/ssl`), [name]);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const renew = async () => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/ssl/renew`, {});
      setMsg({ tone: "ok", text: "Sertifikat baru diterbitkan dan nginx dimuat ulang." });
      reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Sertifikat HTTPS">
      <State loading={loading && !data} error={error} onRetry={reload}>
        {data && !data.exists && <p className="muted">Belum ada sertifikat untuk {data.host}.</p>}
        {data?.exists && (
          <dl className="kv">
            <div><dt>Domain</dt><dd className="mono">{(data.dns as string[]).join(", ")}</dd></div>
            <div><dt>Penerbit</dt><dd>{data.issuer}</dd></div>
            <div>
              <dt>Berlaku sampai</dt>
              <dd>{fmtTime(data.not_after)} <Chip tone={data.expired ? "bad" : data.days_left < 30 ? "warn" : "ok"}>{data.expired ? "Kedaluwarsa" : `${data.days_left} hari lagi`}</Chip></dd>
            </div>
          </dl>
        )}
        {data?.note && <p className="small muted">{data.note}</p>}
      </State>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      <div>
        <button className="btn" onClick={renew} disabled={busy}>{busy ? "Memperbarui…" : "Terbitkan ulang sertifikat"}</button>
      </div>
    </Card>
  );
}
