import { useState } from "react";
import { Cpu, Gauge, HardDrive, Network } from "lucide-react";
import { LineChart, RangePicker } from "../charts";
import { useMetrics, useSystem } from "../data";
import { byteUnit, fmtBytes, fmtMem, fmtRate, niceTop, RANGES, type RangeKey } from "../lib";
import { Card, PageHead, State } from "../ui";
import HardwareSection from "../features/hardware/HardwareSection";
import NetSection from "../features/netmon/NetSection";
import ExposureSection from "../features/exposure/ExposureSection";
import { decimalSep, useT } from "../i18n";

export default function Monitoring() {
  const t = useT();
  const [range, setRange] = useState<RangeKey>("1h");
  const met = useMetrics(range);
  const sys = useSystem();
  const m = met.data;
  const label = RANGES.find((r) => r.key === range)?.label ?? "";
  const loadMax = m ? Math.max(1, ...m.load.map((p) => p[1])) : 1;

  return (
    <>
      <PageHead title={t("nav.monitor")} desc={t("system.mon.desc")} action={<RangePicker value={range} onChange={setRange} />} />
      <State loading={met.loading && !m} skeleton="chart" error={met.error} onRetry={met.reload}>
        {m && (
          <>
            <Card title={<><Cpu size={18} className="icon" aria-hidden="true" /> {t("system.mon.cpuTitle", { label })}</>}>
              <LineChart
                title={t("ov.chart.title")}
                rangeKey={range}
                domain={[0, 100]}
                tickFmt={(v) => `${Math.round(v)}%`}
                series={[
                  { key: "cpu", label: "CPU", pts: m.cpu, fmt: (v) => `${v.toFixed(0)}%` },
                  { key: "mem", label: t("ov.mem"), pts: m.mem, fmt: (v) => `${v.toFixed(0)}%` },
                ]}
              />
            </Card>

            <div className="grid2">
              <Card title={<><Gauge size={18} className="icon" aria-hidden="true" /> {t("system.mon.load")}</>}>
                <p className="small muted">{t("system.mon.loadHint", { cores: sys.data?.cores ?? "?" })}</p>
                <LineChart title={t("system.mon.loadChart")} rangeKey={range} domain={[0, niceTop(loadMax, 4)]} tickFmt={(v) => String(Math.round(v * 10) / 10).replace(".", decimalSep())} series={[{ key: "load", label: t("system.mon.loadSeries"), pts: m.load, fmt: (v) => v.toFixed(2).replace(".", decimalSep()) }]} />
              </Card>
              <Card title={<><HardDrive size={18} className="icon" aria-hidden="true" /> {t("system.mon.disk")}</>}>
                <p className="small muted">{sys.data?.disk_total ? t("system.mon.diskFree", { free: fmtBytes(sys.data.disk_free), total: fmtBytes(sys.data.disk_total) }) : t("system.mon.diskUnknown")}</p>
                <LineChart title={t("system.mon.disk")} rangeKey={range} domain={[0, 100]} tickFmt={(v) => `${Math.round(v)}%`} series={[{ key: "disk", label: t("ov.disk"), pts: m.disk, fmt: (v) => `${v.toFixed(1).replace(".", decimalSep())}%` }]} />
              </Card>
            </div>

            <HardwareSection range={range} />

            <NetSection range={range} />
            <ExposureSection />

            <h2 className="card-title" style={{ marginTop: 8 }}>
              <Network size={18} className="icon" aria-hidden="true" /> {t("system.mon.bw")}
            </h2>
            {m.nets.length === 0 ? (
              <Card>
                <div className="state" role="status">
                  <p className="state-title">{t("system.mon.bwNone")}</p>
                  <div className="state-body">
                    <p>{t("system.mon.bwNoneBody")}</p>
                  </div>
                </div>
              </Card>
            ) : (
              <>
                {m.nets.map((n) => {
                  const top = Math.max(1, ...n.rx.map((p) => p[1]), ...n.tx.map((p) => p[1]));
                  const hi = niceTop(top, 4096, byteUnit(Math.max(top, 4096) / 4));
                  return (
                    <Card key={n.iface} title={t("system.mon.ifTitle", { label: n.label })}>
                      <LineChart
                        title={t("system.mon.ifChart", { label: n.label })}
                        rangeKey={range}
                        domain={[0, hi]}
                        tickFmt={(v) => fmtRate(v)}
                        series={[
                          { key: "rx", label: t("system.mon.rx"), pts: n.rx, fmt: fmtRate },
                          { key: "tx", label: t("system.mon.tx"), pts: n.tx, fmt: fmtRate },
                        ]}
                      />
                    </Card>
                  );
                })}
                <Card title={t("system.mon.totalTitle", { label })}>
                  <div className="tbl-wrap">
                    <table className="tbl">
                      <thead>
                        <tr>
                          <th scope="col">Interface</th>
                          <th scope="col" className="num">{t("system.mon.rxNow")}</th>
                          <th scope="col" className="num">{t("system.mon.txNow")}</th>
                          <th scope="col" className="num">{t("system.mon.rxTotal")}</th>
                          <th scope="col" className="num">{t("system.mon.txTotal")}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {m.nets.map((n) => {
                          const now = sys.data?.nets?.find((x) => x.iface === n.iface);
                          return (
                            <tr key={n.iface}>
                              <th scope="row" className="mono">{n.label}</th>
                              <td className="num">{now ? fmtRate(now.rx_bps) : t("fmt.na")}</td>
                              <td className="num">{now ? fmtRate(now.tx_bps) : t("fmt.na")}</td>
                              <td className="num">{fmtBytes(n.total_rx)}</td>
                              <td className="num">{fmtBytes(n.total_tx)}</td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                  <p className="small muted">{t("system.mon.totalNote")}</p>
                </Card>
              </>
            )}
          </>
        )}
      </State>

      <Card title={t("system.mon.top")}>
        <State loading={sys.loading && !sys.data} error={sys.error} empty={!!sys.data && !(sys.data.top ?? []).length} emptyText={t("system.mon.topEmpty")} onRetry={sys.reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">{t("system.mon.topProc")}</th>
                  <th scope="col" className="num">CPU</th>
                  <th scope="col" className="num">{t("system.mon.topMem")}</th>
                </tr>
              </thead>
              <tbody>
                {(sys.data?.top ?? []).map((p, i) => (
                  <tr key={`${p.name}-${i}`}>
                    <th scope="row" className="mono">{p.name}</th>
                    <td className="num">{p.cpu.toFixed(1).replace(".", decimalSep())}%</td>
                    <td className="num">{fmtMem(p.rss_mb * 1024 * 1024)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="small muted">{t("system.mon.topNote")}</p>
        </State>
      </Card>
    </>
  );
}
