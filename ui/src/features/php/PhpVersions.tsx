import { useEffect, useState } from "react";
import { errText, get, post } from "../../api";
import { Alert, Card, Chip, Field, State, useLoad } from "../../ui";
import { useT } from "../../i18n";

type Info = {
  default: string;
  installed: string[];
  shims_ready: boolean;
  shell_ready: boolean;
  brew_linked: string;
  sites: { site: string; version: string | null }[];
};

/** Jumlah situs per versi PHP, diurutkan menurut versi. Tetap ringkas berapa pun jumlah situsnya. */
export function webSummary(sites: { site: string; version: string | null }[]): { version: string; n: number }[] {
  const m = new Map<string, number>();
  for (const s of sites) {
    if (s.version) {
      m.set(s.version, (m.get(s.version) ?? 0) + 1);
    }
  }
  return [...m.entries()].sort((a, b) => a[0].localeCompare(b[0], undefined, { numeric: true })).map(([version, n]) => ({ version, n }));
}

export function PhpVersions({ onChanged }: { onChanged?: () => void }) {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<Info>("/api/php/default"));
  const [pick, setPick] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  useEffect(() => {
    if (data) {
      setPick(data.default || data.brew_linked || data.installed[0] || "");
    }
  }, [data]);
  const save = async () => {
    setBusy(true);
    setMsg(null);
    try {
      await post("/api/php/default", { version: pick });
      setMsg({ tone: "ok", text: t("phpv.saved", { v: pick }) });
      reload();
      onChanged?.();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const web = webSummary(data?.sites ?? []);
  const effective = data?.default || data?.brew_linked || "";
  return (
    <Card title={t("phpv.title")}>
      <State loading={loading && !data} error={error} onRetry={reload}>
        {data && (
          <>
            <p className="small muted">{t("phpv.intro")}</p>
            <dl className="kv pub-kv">
              <div>
                <dt>{t("phpv.cli")}</dt>
                <dd className="row">
                  <Chip tone="info">PHP {effective || "-"}</Chip>
                  {!data.default && <span className="small muted">{t("phpv.cliBrew")}</span>}
                </dd>
              </div>
              <div>
                <dt>{t("phpv.web")}</dt>
                <dd className="row">
                  {web.length === 0 ? <span className="muted">-</span> : web.map((w) => <Chip key={w.version} tone="mute">{t("phpv.sites", { v: w.version, n: w.n })}</Chip>)}
                  <a className="small" href="#/website">{t("phpv.seeSites")}</a>
                </dd>
              </div>
            </dl>
            <Field label={t("phpv.pick")} hint={t("phpv.pickHint")}>
              {(id, dd) => (
                <select id={id} aria-describedby={dd} value={pick} onChange={(e) => setPick(e.target.value)} disabled={busy}>
                  {data.installed.map((v) => <option key={v} value={v}>PHP {v}</option>)}
                </select>
              )}
            </Field>
            <div className="row">
              <button type="button" className="btn btn-primary" disabled={busy || !pick || pick === data.default} onClick={save}>
                {busy ? t("common.processing") : t("phpv.save")}
              </button>
            </div>
            {!data.shims_ready || !data.shell_ready ? (
              <div className="alert alert-warn" role="status">
                {t("phpv.noShim")} <code className="mono">lab php shims</code>
              </div>
            ) : (
              <p className="small muted">{t("phpv.order")}</p>
            )}
          </>
        )}
      </State>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
    </Card>
  );
}
