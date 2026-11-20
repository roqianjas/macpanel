import { useEffect, useState } from "react";
import { errText, get, post } from "../../api";
import { Alert, Card, Chip, Field, State, useLoad } from "../../ui";
import { useT } from "../../i18n";

type Tunnel = { site: string; url: string; expires: number; seconds_left: number };
type PublicInfo = { installed: boolean; ttl_default: number; ttl_min: number; ttl_max: number; tunnel: Tunnel | null; active_total: number };

const TTLS = [30, 60, 120, 240, 480, 1440];

function dur(m: number, t: ReturnType<typeof useT>) {
  if (m < 60) {
    return t("pub.min", { n: m });
  }
  return m % 60 === 0 ? t("pub.h", { h: m / 60 }) : t("pub.hm", { h: Math.floor(m / 60), m: m % 60 });
}

function left(sec: number, t: ReturnType<typeof useT>) {
  const m = Math.max(0, Math.ceil(sec / 60));
  return dur(m, t);
}

export function PublicAccess({ name }: { name: string }) {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<PublicInfo>(`/api/sites/${name}/public`), [name]);
  const [ttl, setTtl] = useState(120);
  const [ack, setAck] = useState(false);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [copied, setCopied] = useState(false);
  const active = !!data?.tunnel;
  useEffect(() => {
    if (!active) {
      return;
    }
    const id = window.setInterval(reload, 15000);
    return () => window.clearInterval(id);
  }, [active, reload]);
  const act = async (path: string, body: object, ok: string) => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/public/${path}`, body);
      setMsg({ tone: "ok", text: ok });
      setAck(false);
      reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const copy = (u: string) => {
    navigator.clipboard.writeText(u).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    });
  };
  return (
    <Card title={t("pub.title")}>
      <State loading={loading && !data} error={error} onRetry={reload}>
        {data && !data.installed && (
          <p>
            {t("pub.notInstalled")} <a href="#/aplikasi">{t("pub.openApps")}</a>
          </p>
        )}
        {data?.installed && active && data.tunnel && (
          <>
            <div className="alert alert-warn" role="alert">{t("pub.warnActive")}</div>
            <dl className="kv pub-kv">
              <div>
                <dt>{t("pub.address")}</dt>
                <dd className="mono">
                  {data.tunnel.url ? <a href={data.tunnel.url} target="_blank" rel="noopener noreferrer">{data.tunnel.url}</a> : t("pub.waiting")}
                </dd>
              </div>
              <div>
                <dt>{t("pub.remaining")}</dt>
                <dd className="row">
                  <Chip tone="warn">{t("pub.public")}</Chip>
                  <span>{left(data.tunnel.seconds_left, t)}</span>
                </dd>
              </div>
            </dl>
            <div className="row">
              {data.tunnel.url && (
                <button type="button" className="btn" onClick={() => copy(data.tunnel!.url)}>
                  {copied ? t("pub.copied") : t("pub.copy")}
                </button>
              )}
              <button type="button" className="btn btn-danger" disabled={busy} onClick={() => act("off", {}, t("pub.closed"))}>
                {busy ? t("common.processing") : t("pub.close")}
              </button>
            </div>
          </>
        )}
        {data?.installed && !active && (
          <>
            <p className="small muted">{t("pub.intro")}</p>
            <ul className="pub-list small muted">
              <li>{t("pub.pt1")}</li>
              <li>{t("pub.pt2")}</li>
              <li>{t("pub.pt3")}</li>
            </ul>
            <Field label={t("pub.ttl")} hint={t("pub.ttlHint")}>
              {(id, dd) => (
                <select id={id} aria-describedby={dd} value={ttl} onChange={(e) => setTtl(Number(e.target.value))}>
                  {TTLS.filter((m) => m >= data.ttl_min && m <= data.ttl_max).map((m) => (
                    <option key={m} value={m}>{dur(m, t)}</option>
                  ))}
                </select>
              )}
            </Field>
            <label className="check">
              <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} />
              {t("pub.ack")}
            </label>
            <div>
              <button type="button" className="btn btn-primary" disabled={busy || !ack} onClick={() => act("on", { ttl_min: ttl, ack: true }, t("pub.opened"))}>
                {busy ? t("pub.starting") : t("pub.open")}
              </button>
            </div>
            {data.active_total > 0 && <p className="small muted">{t("pub.others", { n: data.active_total })}</p>}
          </>
        )}
      </State>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
    </Card>
  );
}
