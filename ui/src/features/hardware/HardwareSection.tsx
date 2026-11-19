import { Fan, HardDrive, MemoryStick, Plug, Thermometer } from "lucide-react";
import { LineChart } from "../../charts";
import { useT } from "../../i18n";
import { fmtBytes, niceTop, RANGES, type RangeKey } from "../../lib";
import { Card, Chip, State } from "../../ui";
import { fanText, forecastText, fmtTemp, freshness, num, smartStatus } from "./model";
import { useHw } from "./useHw";

// Ikon relevan dengan isi: Thermometer (suhu), Fan (kipas), Plug (daya listrik), MemoryStick (memori), HardDrive (SSD).
// Tiap grafik berjudul pertanyaan yang dijawabnya; grafik tanpa data tampil sebagai keterangan, bukan garis datar.

export default function HardwareSection({ range }: { range: RangeKey }) {
  const t = useT();
  const hw = useHw(range);
  const d = hw.data;
  const label = RANGES.find((r) => r.key === range)?.label ?? "";
  const now = Math.floor(Date.now() / 1000);
  const fresh = freshness(d, now);
  const pk = d?.peak ?? {};
  const ser = d?.series ?? {};
  const fanTop = niceTop(Math.max(pk.fan ?? 0, d?.now?.fan?.max ?? 0), 2000);
  const powTop = niceTop(pk.power ?? 0, 20);
  const smart = smartStatus(d?.ssd?.smart);
  const fc = d ? forecastText(d.forecast) : null;

  return (
    <>
      <h2 className="card-title" style={{ marginTop: 8 }}>
        <Thermometer size={18} className="icon" aria-hidden="true" /> {t("system.hw.title")}
      </h2>
      <State loading={hw.loading && !d} error={hw.error} onRetry={hw.reload}>
        {d && (
          <>
            <p className="small muted">
              {fresh.tone !== "ok" && <Chip tone={fresh.tone === "mute" ? "mute" : "warn"}>{fresh.tone === "mute" ? t("system.hw.waiting") : t("system.hw.attention")}</Chip>} {fresh.text}
            </p>

            <Card title={t("system.hw.temp.title", { label })}>
              <LineChart
                title={t("system.hw.temp.chart")}
                rangeKey={range}
                domain={[20, 100]}
                tickFmt={(v) => `${Math.round(v)}°`}
                series={[
                  { key: "cpu", label: "CPU", pts: ser.cpu ?? [], fmt: (v) => `${num(v, 1)} °C` },
                  { key: "gpu", label: "GPU", pts: ser.gpu ?? [], fmt: (v) => `${num(v, 1)} °C` },
                  { key: "ssd", label: "SSD", pts: ser.ssd ?? [], fmt: (v) => `${num(v, 1)} °C` },
                ]}
              />
              <p className="small muted">
                {t("system.hw.temp.peak", { cpu: fmtTemp(pk.cpu), gpu: fmtTemp(pk.gpu), ssd: fmtTemp(pk.ssd) })}
                {d.throttled_samples > 0 && ` ${t("system.hw.temp.throttled", { n: d.throttled_samples, samples: d.samples })}`}
              </p>
            </Card>

            <div className="grid2">
              <Card title={<><Fan size={18} className="icon" aria-hidden="true" /> {t("system.hw.fan.title")}</>}>
                <p className="small muted">{t("system.hw.now", { v: fanText(d.now?.fan ?? null) })}</p>
                <LineChart title={t("system.hw.fan.chart")} rangeKey={range} domain={[0, fanTop]} tickFmt={(v) => `${Math.round(v)}`} series={[{ key: "fan", label: t("system.hw.fan.series"), pts: ser.fan ?? [], fmt: (v) => `${Math.round(v)} rpm` }]} />
              </Card>
              <Card title={<><Plug size={18} className="icon" aria-hidden="true" /> {t("system.hw.pow.title")}</>}>
                <p className="small muted">{t("system.hw.pow.now", { v: d.now?.power?.system != null ? `${num(d.now.power.system, 1)} W` : t("fmt.na") })}</p>
                <LineChart title={t("system.hw.pow.chart")} rangeKey={range} domain={[0, powTop]} tickFmt={(v) => `${Math.round(v)} W`} series={[{ key: "power", label: t("system.hw.pow.series"), pts: ser.power ?? [], fmt: (v) => `${num(v, 1)} W` }]} />
              </Card>
            </div>

            <div className="grid2">
              <Card title={<><MemoryStick size={18} className="icon" aria-hidden="true" /> {t("system.hw.mem.title")}</>}>
                <p className="small muted">{t("system.hw.mem.hint")}{d.now?.mem?.swap_used_mb != null && ` ${t("system.hw.mem.swap", { used: num(d.now.mem.swap_used_mb, 1), total: num(d.now.mem.swap_total_mb) })}`}</p>
                <LineChart title={t("system.hw.mem.chart")} rangeKey={range} domain={[0, 100]} tickFmt={(v) => `${Math.round(v)}%`} series={[{ key: "pressure", label: t("system.hw.mem.series"), pts: ser.pressure ?? [], fmt: (v) => `${num(v, 0)}%` }]} />
              </Card>
              <Card title={<><HardDrive size={18} className="icon" aria-hidden="true" /> {t("system.hw.ssd.title")}</>}>
                {d.ssd ? (
                  <dl className="kv">
                    <div>
                      <dt>{t("system.hw.ssd.device")}</dt>
                      <dd>{d.ssd.name || t("fmt.na")}</dd>
                    </div>
                    <div>
                      <dt>{t("system.hw.ssd.smart")}</dt>
                      <dd>
                        <Chip tone={smart.tone}>{smart.text}</Chip> <span className="small muted">{d.ssd.smart || t("system.hw.ssd.noStatus")}</span>
                      </dd>
                    </div>
                    <div>
                      <dt>{t("system.hw.ssd.space")}</dt>
                      <dd>
                        {t("system.hw.ssd.spaceVal", { free: fmtBytes(d.ssd.free), total: fmtBytes(d.ssd.total) })}
                      </dd>
                    </div>
                    <div>
                      <dt>{t("system.hw.ssd.wear")}</dt>
                      <dd className="small muted">{t("system.hw.ssd.wearNote")}</dd>
                    </div>
                  </dl>
                ) : (
                  <p className="small muted">{t("system.hw.ssd.none")}</p>
                )}
                {fc && (
                  <p className="small" style={{ marginTop: 10 }}>
                    <strong>{t("system.hw.fc")}</strong> {fc.text}
                  </p>
                )}
              </Card>
            </div>
          </>
        )}
      </State>
    </>
  );
}
