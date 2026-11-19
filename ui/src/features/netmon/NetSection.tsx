import { Globe, Network } from "lucide-react";
import { LineChart } from "../../charts";
import { useT } from "../../i18n";
import { niceTop, RANGES, type RangeKey } from "../../lib";
import { Card, Chip, State } from "../../ui";
import { dnsStatus, fmtMs, internetStatus, netFreshness, tailscaleStatus, uptimePct } from "./model";
import { useNetmon } from "./useNetmon";

// Ikon: Network (Tailscale dan jalur ke mesin) dan Globe (internet dan DNS), relevan dengan isinya.
// Pertanyaan tiap grafik ada di judulnya. Persen ketersediaan dihitung dari sampel yang ada, bukan dari waktu.

export default function NetSection({ range }: { range: RangeKey }) {
  const t = useT();
  const nm = useNetmon(range);
  const d = nm.data;
  const label = RANGES.find((r) => r.key === range)?.label ?? "";
  const now = Math.floor(Date.now() / 1000);
  const fresh = netFreshness(d, now);
  const s = d?.now;
  const ts = tailscaleStatus(s?.tailscale ?? null);
  const inet = internetStatus(s?.internet, d?.thresholds.slow_ms ?? 300);
  const dl = dnsStatus(s?.dns_local);
  const top = niceTop(Math.max(d?.inet_max_ms ?? 0, 50), 50);

  return (
    <>
      <h2 className="card-title" style={{ marginTop: 8 }}>
        <Network size={18} className="icon" aria-hidden="true" /> {t("system.net.title")}
      </h2>
      <State loading={nm.loading && !d} error={nm.error} onRetry={nm.reload}>
        {d && (
          <>
            <p className="small muted">
              {fresh.tone !== "ok" && <Chip tone={fresh.tone === "mute" ? "mute" : "warn"}>{fresh.tone === "mute" ? t("system.hw.waiting") : t("system.hw.attention")}</Chip>} {fresh.text}
            </p>
            <div className="grid2">
              <Card title={<><Network size={18} className="icon" aria-hidden="true" /> {t("system.net.ts.title")}</>}>
                <dl className="kv">
                  <div>
                    <dt>{t("apps.col.status")}</dt>
                    <dd>
                      <Chip tone={ts.tone}>{ts.text}</Chip>
                    </dd>
                  </div>
                  <div>
                    <dt>{t("system.net.peersOnline")}</dt>
                    <dd>{s?.tailscale ? t("system.net.peersVal", { online: s.tailscale.peers_online, peers: s.tailscale.peers }) : t("fmt.na")}</dd>
                  </div>
                  <div>
                    <dt>{t("system.net.down", { label })}</dt>
                    <dd>{d.ts_down === 0 ? t("system.net.none") : t("system.net.downVal", { n: d.ts_down, samples: d.samples })}</dd>
                  </div>
                  {s?.tailscale && s.tailscale.health.length > 0 && (
                    <div>
                      <dt>{t("system.net.warn")}</dt>
                      <dd className="small">{s.tailscale.health.join("; ")}</dd>
                    </div>
                  )}
                </dl>
              </Card>
              <Card title={<><Globe size={18} className="icon" aria-hidden="true" /> {t("system.net.inet.title")}</>}>
                <dl className="kv">
                  <div>
                    <dt>Internet</dt>
                    <dd>
                      <Chip tone={inet.tone}>{inet.text}</Chip> <span className="small muted">{t("system.net.tcp", { ms: fmtMs(s?.internet.ms) })}</span>
                    </dd>
                  </div>
                  <div>
                    <dt>{t("system.net.dns")}</dt>
                    <dd>
                      <Chip tone={dl.tone}>{dl.text}</Chip> <span className="small muted">{s?.dns_local ? `${s.dns_local.name}, ${fmtMs(s.dns_local.ms)}` : t("system.net.notTested")}</span>
                    </dd>
                  </div>
                  <div>
                    <dt>{t("system.net.avail")}</dt>
                    <dd>
                      {uptimePct(d.inet_down, d.samples)} <span className="small muted">{t("system.net.availOf", { samples: d.samples, label })}</span>
                    </dd>
                  </div>
                </dl>
              </Card>
            </div>
            <Card title={t("system.net.lat.title", { label })}>
              <LineChart
                title={t("system.net.lat.chart")}
                rangeKey={range}
                domain={[0, top]}
                tickFmt={(v) => `${Math.round(v)} ms`}
                series={[
                  { key: "internet", label: "Internet", pts: d.series.internet ?? [], fmt: (v) => fmtMs(v) },
                  { key: "dns_local", label: t("system.net.dns"), pts: d.series.dns_local ?? [], fmt: (v) => fmtMs(v) },
                ]}
              />
              <p className="small muted">{t("system.net.lat.note", { max: fmtMs(d.inet_max_ms || null) })}</p>
            </Card>
          </>
        )}
      </State>
    </>
  );
}
