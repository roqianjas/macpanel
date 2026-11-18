import { Globe, Network } from "lucide-react";
import { LineChart } from "../../charts";
import { niceTop, RANGES, type RangeKey } from "../../lib";
import { Card, Chip, State } from "../../ui";
import { dnsStatus, fmtMs, internetStatus, netFreshness, tailscaleStatus, uptimePct } from "./model";
import { useNetmon } from "./useNetmon";

// Ikon: Network (Tailscale dan jalur ke mesin) dan Globe (internet dan DNS), relevan dengan isinya.
// Pertanyaan tiap grafik ada di judulnya. Persen ketersediaan dihitung dari sampel yang ada, bukan dari waktu.

export default function NetSection({ range }: { range: RangeKey }) {
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
        <Network size={18} className="icon" aria-hidden="true" /> Konektivitas
      </h2>
      <State loading={nm.loading && !d} error={nm.error} onRetry={nm.reload}>
        {d && (
          <>
            <p className="small muted">
              {fresh.tone !== "ok" && <Chip tone={fresh.tone === "mute" ? "mute" : "warn"}>{fresh.tone === "mute" ? "Menunggu" : "Perhatian"}</Chip>} {fresh.text}
            </p>
            <div className="grid2">
              <Card title={<><Network size={18} className="icon" aria-hidden="true" /> Tailscale: apakah mesin ini bisa dijangkau dari luar?</>}>
                <dl className="kv">
                  <div>
                    <dt>Status</dt>
                    <dd>
                      <Chip tone={ts.tone}>{ts.text}</Chip>
                    </dd>
                  </div>
                  <div>
                    <dt>Perangkat online</dt>
                    <dd>{s?.tailscale ? `${s.tailscale.peers_online} dari ${s.tailscale.peers} perangkat lain` : "n/a"}</dd>
                  </div>
                  <div>
                    <dt>Putus dalam {label}</dt>
                    <dd>{d.ts_down === 0 ? "tidak ada" : `${d.ts_down} dari ${d.samples} sampel`}</dd>
                  </div>
                  {s?.tailscale && s.tailscale.health.length > 0 && (
                    <div>
                      <dt>Peringatan</dt>
                      <dd className="small">{s.tailscale.health.join("; ")}</dd>
                    </div>
                  )}
                </dl>
              </Card>
              <Card title={<><Globe size={18} className="icon" aria-hidden="true" /> Internet dan DNS: apakah keluar dan nama terjawab?</>}>
                <dl className="kv">
                  <div>
                    <dt>Internet</dt>
                    <dd>
                      <Chip tone={inet.tone}>{inet.text}</Chip> <span className="small muted">{fmtMs(s?.internet.ms)} (sambung TCP)</span>
                    </dd>
                  </div>
                  <div>
                    <dt>DNS lokal</dt>
                    <dd>
                      <Chip tone={dl.tone}>{dl.text}</Chip> <span className="small muted">{s?.dns_local ? `${s.dns_local.name}, ${fmtMs(s.dns_local.ms)}` : "tidak dites"}</span>
                    </dd>
                  </div>
                  <div>
                    <dt>Ketersediaan internet</dt>
                    <dd>
                      {uptimePct(d.inet_down, d.samples)} <span className="small muted">dari {d.samples} sampel dalam {label}</span>
                    </dd>
                  </div>
                </dl>
              </Card>
            </div>
            <Card title={<>Latensi internet: seberapa cepat koneksi keluar? ({label})</>}>
              <LineChart
                title="Latensi internet"
                rangeKey={range}
                domain={[0, top]}
                tickFmt={(v) => `${Math.round(v)} ms`}
                series={[
                  { key: "internet", label: "Internet", pts: d.series.internet ?? [], fmt: (v) => fmtMs(v) },
                  { key: "dns_local", label: "DNS lokal", pts: d.series.dns_local ?? [], fmt: (v) => fmtMs(v) },
                ]}
              />
              <p className="small muted">Diukur dengan waktu sambung TCP ke dua server publik, bukan ping. Titik yang gagal diukur dibiarkan kosong, bukan ditulis 0 ms. Puncak dalam rentang ini: {fmtMs(d.inet_max_ms || null)}.</p>
            </Card>
          </>
        )}
      </State>
    </>
  );
}
