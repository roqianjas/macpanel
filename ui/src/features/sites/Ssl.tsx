import { useState } from "react";
import { errText, fmtTime, get, post } from "../../api";
import { Alert, Card, Chip, State, useLoad } from "../../ui";
import { useT } from "../../i18n";

export function Ssl({ name }: { name: string }) {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<any>(`/api/sites/${name}/ssl`), [name]);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const renew = async () => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/ssl/renew`, {});
      setMsg({ tone: "ok", text: t("dev.ssl.renewed") });
      reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title={t("dev.ssl.title")}>
      <State loading={loading && !data} error={error} onRetry={reload}>
        {data && !data.exists && <p className="muted">{t("dev.ssl.none", { host: data.host })}</p>}
        {data?.exists && (
          <dl className="kv">
            <div><dt>{t("dev.ssl.domain")}</dt><dd className="mono">{(data.dns as string[]).join(", ")}</dd></div>
            <div><dt>{t("dev.ssl.issuer")}</dt><dd>{data.issuer}</dd></div>
            <div>
              <dt>{t("dev.ssl.validUntil")}</dt>
              <dd>{fmtTime(data.not_after)} <Chip tone={data.expired ? "bad" : data.days_left < 30 ? "warn" : "ok"}>{data.expired ? t("dev.ssl.expired") : t("dev.ssl.daysLeft", { n: data.days_left })}</Chip></dd>
            </div>
          </dl>
        )}
        {data?.note && <p className="small muted">{data.note}</p>}
      </State>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      <div>
        <button className="btn" onClick={renew} disabled={busy}>{busy ? t("dev.ssl.renewing") : t("dev.ssl.renew")}</button>
      </div>
    </Card>
  );
}
