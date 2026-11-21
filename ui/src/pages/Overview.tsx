import { Activity, Boxes, Cpu, Database, Download, Globe, HardDrive, MemoryStick, Network, Server, Timer } from "lucide-react";
import { get } from "../api";
import { CountUp, LineChart, RangePicker, Ring, Sparkline, toneOf } from "../charts";
import { serviceView, useAlerts, useMetrics, useServices, useSystem } from "../data";
import { fmtAgo, fmtBytes, fmtMem, fmtRate, fmtUptime, type RangeKey } from "../lib";
import { Card, Chip, Mark, State, usePoll } from "../ui";
import HealthCard from "../features/hardware/HealthCard";
import { useState } from "react";
import { decimalSep, useT } from "../i18n";
import { alertDetail, alertTitle } from "../serverText";

interface Counts {
  sites: number;
  sitesOn: number;
  db: number;
  workers: number;
  jobs: number;
  php: number;
}

async function loadCounts(): Promise<Counts> {
  const [s, d, p, w, j] = await Promise.allSettled([get<any>("/api/sites"), get<any>("/api/db"), get<any>("/api/php"), get<any>("/api/tasks/worker"), get<any>("/api/tasks/job")]);
  const v = <T,>(r: PromiseSettledResult<T>): any => (r.status === "fulfilled" ? r.value : null);
  const sites = (v(s)?.sites ?? []).filter((x: any) => !x.system);
  const db = v(d)?.databases;
  return {
    sites: sites.length,
    sitesOn: sites.filter((x: any) => x.enabled).length,
    db: db ? (db.mariadb?.length ?? 0) + (db.mysql?.length ?? 0) + (db.pgsql?.length ?? 0) : 0,
    workers: v(w)?.tasks?.length ?? 0,
    jobs: v(j)?.tasks?.length ?? 0,
    php: (v(p)?.php ?? []).filter((x: any) => x.installed).length,
  };
}

export default function Overview() {
  const t = useT();
  const sys = useSystem();
  const svc = useServices();
  const [range, setRange] = useState<RangeKey>("1h");
  const met = useMetrics(range);
  const spark = useMetrics("1h");
  const counts = usePoll(loadCounts, 60000);
  const alerts = useAlerts();

  const services = svc.data?.services ?? [];
  const core = services.filter((s) => s.installed && !s.optional);
  const bad = core.filter((s) => s.state !== "running");
  const running = services.filter((s) => s.installed && s.state === "running").length;
  const total = services.filter((s) => s.installed).length;
  const si = sys.data;
  const hasSample = !!si && si.sample_at > 0;
  const nets = si?.nets ?? [];
  // Angka utama hanya dari kartu jaringan fisik: lalu lintas Tailscale juga lewat sana (terenkripsi), jadi menjumlahkan utun akan menghitung dua kali.
  const phys = nets.filter((n) => !n.ip.startsWith("100."));
  const rx = phys.reduce((a, n) => a + n.rx_bps, 0);
  const tx = phys.reduce((a, n) => a + n.tx_bps, 0);
  const now = Math.floor(Date.now() / 1000);

  const notifyLine = (code?: string, text?: string) =>
    code?.startsWith("ntfy:") ? t("ov.alert.footerNtfy", { host: code.slice(5) }) : code === "panel" || !text ? t("ov.alert.footer") : t("ov.alert.footerCustom", { x: text });
  const heroLoading = (svc.loading && !svc.data) || (sys.loading && !sys.data);
  const hero = heroLoading ? (
    <section className="hero" aria-label={t("ov.hero.aria")} role="status" aria-busy="true">
      <span className="sr">{t("state.loading")}</span>
      <div aria-hidden="true" className="sk-hero">
        <span className="sk sk-title" />
        <span className="sk sk-line" style={{ width: "70%" }} />
      </div>
      <div aria-hidden="true" className="sk-hero-facts">
        {[0, 1, 2].map((i) => <span key={i} className="sk sk-fact" />)}
      </div>
    </section>
  ) : (
    <section className={`hero ${bad.length ? "hero-bad" : ""}`} aria-label={t("ov.hero.aria")}>
      <div>
        <h1 className="hero-title">
          <Mark tone={bad.length ? "bad" : "ok"} />
          {svc.loading && !svc.data ? t("ov.checking") : bad.length ? t("ov.attention", { n: bad.length }) : t("ov.allok")}
        </h1>
        <p className="hero-sub">
          {bad.length ? t("ov.down", { names: bad.map((b) => b.label).join(", ") }) : t("ov.running", { running, total, host: si?.host ?? "Mac mini" })}
        </p>
      </div>
      <dl className="hero-facts">
        <div>
          <dt>{t("ov.uptime")}</dt>
          <dd>{si?.boot ? fmtUptime(now - si.boot) : t("fmt.na")}</dd>
        </div>
        <div>
          <dt>{t("ov.tailnet")}</dt>
          <dd className="mono">{si?.tailnet_ip || t("ov.notconn")}</dd>
        </div>
        <div>
          <dt>{t("ov.load")}</dt>
          <dd>{hasSample ? si!.load[0].toFixed(2).replace(".", decimalSep()) : t("fmt.na")}</dd>
        </div>
      </dl>
    </section>
  );

  const tiles = [
    { href: "#/website", icon: Globe, label: t("nav.websites"), ds: t("ov.tile.web"), chip: counts.data ? (counts.data.sites ? t("ov.chip.sites", { on: counts.data.sitesOn, n: counts.data.sites }) : t("ov.chip.nosites")) : "" },
    { href: "#/aplikasi", icon: Server, label: t("nav.apps"), ds: t("ov.tile.apps"), chip: svc.data ? t("ov.chip.running", { r: running, t: total }) : "" },
    { href: "#/database", icon: Database, label: t("nav.databases"), ds: t("ov.tile.db"), chip: counts.data ? t("ov.chip.projects", { n: counts.data.db }) : "" },
    { href: "#/tugas", icon: Timer, label: t("nav.tasks"), ds: t("ov.tile.tasks"), chip: counts.data ? t("ov.chip.tasks", { w: counts.data.workers, j: counts.data.jobs }) : "" },
    { href: "#/runtime", icon: Boxes, label: t("nav.runtimes"), ds: t("ov.tile.rt"), chip: counts.data ? t("ov.chip.php", { n: counts.data.php }) : "" },
    { href: "#/pembaruan", icon: Download, label: t("nav.updates"), ds: t("ov.tile.upd"), chip: "" },
  ];

  return (
    <>
      {hero}
      {alerts.data && alerts.data.alerts.length > 0 && (
        <Card title={t("ov.alerts", { n: alerts.data.alerts.length })}>
          <ul className="list">
            {alerts.data.alerts.map((a) => (
              <li key={a.id} className="list-row">
                <div>
                  <p>
                    <Chip tone={a.level === "bad" ? "bad" : "warn"}>{a.level === "bad" ? t("ov.alert.bad") : t("ov.alert.warn")}</Chip> <strong>{alertTitle(a)}</strong>
                  </p>
                  <p className="small muted">{alertDetail(a)}</p>
                </div>
                <a className="btn btn-sm" href={a.link}>
                  {t("ov.alert.view")}<span className="sr"> {a.title}</span>
                </a>
              </li>
            ))}
          </ul>
          <p className="small muted">{notifyLine(alerts.data.notify_code, alerts.data.notify)}</p>
        </Card>
      )}
      <State loading={sys.loading && !sys.data} skeleton="cards" error={sys.error} onRetry={sys.reload}>
        {si && (
          <ul className="kpis" aria-label={t("ov.res")}>
            <li className="card kpi">
              <h2 className="kpi-h"><Cpu size={16} aria-hidden="true" /> CPU</h2>
              <div className="kpi-body">
                <Ring pct={hasSample ? si.cpu : null} label={t("ov.cpu.used")} tone={toneOf(hasSample ? si.cpu : null)} />
                <div>
                  <p className="big">{hasSample ? <CountUp value={si.cpu} decimals={0} suffix="%" /> : t("fmt.na")}</p>
                  <p className="small muted">{t("ov.cores", { n: si.cores })}</p>
                </div>
              </div>
              <Sparkline pts={spark.data?.cpu ?? []} label={t("ov.cpu.hour")} />
            </li>
            <li className="card kpi">
              <h2 className="kpi-h"><MemoryStick size={16} aria-hidden="true" /> {t("ov.mem")}</h2>
              <div className="kpi-body">
                <Ring pct={hasSample ? si.mem : null} label={t("ov.mem.used")} tone={toneOf(hasSample ? si.mem : null)} />
                <div>
                  <p className="big">{hasSample ? <CountUp value={si.mem} decimals={0} suffix="%" /> : t("fmt.na")}</p>
                  <p className="small muted">{t("ov.mem.of", { size: fmtMem(si.mem_total) })}</p>
                </div>
              </div>
              <Sparkline pts={spark.data?.mem ?? []} label={t("ov.mem.hour")} />
            </li>
            <li className="card kpi">
              <h2 className="kpi-h"><HardDrive size={16} aria-hidden="true" /> {t("ov.disk")}</h2>
              <div className="kpi-body">
                <Ring pct={si.disk_total ? si.disk : null} label={t("ov.disk.used")} tone={toneOf(si.disk)} />
                <div>
                  <p className="big">{si.disk_total ? <CountUp value={si.disk} decimals={0} suffix="%" /> : t("fmt.na")}</p>
                  <p className="small muted">{t("ov.disk.free", { size: fmtBytes(si.disk_free) })}</p>
                  {(si.disk_purgeable ?? 0) > 100e6 && (
                    <p className="small muted">{t("ov.disk.purgeable", { size: fmtBytes(si.disk_purgeable ?? 0), total: fmtBytes(si.disk_free + (si.disk_purgeable ?? 0)) })}</p>
                  )}
                </div>
              </div>
              {si.disk >= 85 && <p className="callout">{t("system.ov.diskFull")}</p>}
            </li>
            <li className="card kpi">
              <h2 className="kpi-h"><Network size={16} aria-hidden="true" /> {t("ov.net")}</h2>
              {nets.length === 0 ? (
                <p className="small muted">{t("ov.net.none")}</p>
              ) : (
                <>
                  <p className="big">{fmtRate(rx)}</p>
                  <p className="small muted">{t("ov.net.now", { tx: fmtRate(tx) })}</p>
                  <ul className="kpi-rows">
                    {nets.map((n) => (
                      <li key={n.iface}>
                        <span>{n.label}</span>
                        <span className="mono">↓{fmtRate(n.rx_bps)} ↑{fmtRate(n.tx_bps)}</span>
                      </li>
                    ))}
                  </ul>
                </>
              )}
              <a className="small" href="#/pantau">{t("ov.net.hist")}</a>
            </li>
          </ul>
        )}
      </State>

      <div className="split">
        <div className="stackc">
          <Card title={<><Activity size={18} className="icon" aria-hidden="true" /> {t("ov.busy")}</>} action={<RangePicker value={range} onChange={setRange} />}>
          <State loading={met.loading && !met.data} skeleton="chart" error={met.error} onRetry={met.reload}>
            {met.data && (
              <LineChart
                title={t("ov.chart.title")}
                rangeKey={range}
                domain={[0, 100]}
                tickFmt={(v) => `${Math.round(v)}%`}
                series={[
                  { key: "cpu", label: "CPU", pts: met.data.cpu, fmt: (v) => `${v.toFixed(0)}%` },
                  { key: "mem", label: t("ov.mem"), pts: met.data.mem, fmt: (v) => `${v.toFixed(0)}%` },
                ]}
              />
            )}
          </State>
          </Card>
          {si && (
            <Card title={t("ov.sys")}>
              <dl className="sysinfo">
                <div><dt>{t("ov.sys.host")}</dt><dd>{si.host}</dd></div>
                <div><dt>{t("ov.sys.os")}</dt><dd>{si.os}</dd></div>
                <div><dt>{t("ov.sys.cpu")}</dt><dd>{t("ov.cores", { n: si.cores })}</dd></div>
                <div><dt>{t("ov.mem")}</dt><dd>{fmtMem(si.mem_total)}</dd></div>
                <div><dt>{t("ov.disk")}</dt><dd>{fmtBytes(si.disk_total)}</dd></div>
                <div>
                  <dt>{t("ov.sys.backup")}</dt>
                  <dd>{si.backup ? `${fmtAgo(now - si.backup.at)}${si.backup.local_only ? t("ov.sys.local") : ""}` : t("ov.sys.nobackup")}</dd>
                </div>
              </dl>
              {si.backup?.local_only && <p className="callout">{t("system.ov.bkLocal")}</p>}
            </Card>
          )}
        </div>
        <div className="stackc">
          <Card title={t("ov.svc")} action={<a className="btn btn-sm" href="#/aplikasi">{t("ov.svc.manage")}</a>}>
            <State loading={svc.loading && !svc.data} error={svc.error} onRetry={svc.reload}>
              <ul className="svc-list">
                {services.filter((s) => s.installed).map((s) => (
                  <li key={s.id}>
                    <a href="#/aplikasi">{s.label}</a>
                    <Chip tone={serviceView(s).tone}>{serviceView(s).text}</Chip>
                  </li>
                ))}
              </ul>
            </State>
          </Card>
        </div>
      </div>

      <HealthCard />
      <ul className="tiles" aria-label={t("ov.quick")}>
        {tiles.map((tile) => (
          <li key={tile.href}>
            <a className="tile" href={tile.href}>
              <span className="tile-ic">
                <tile.icon size={22} aria-hidden="true" />
              </span>
              <b>{tile.label}</b>
              <span className="tile-ds">{tile.ds}</span>
              {tile.chip && <span className="chip chip-mute">{tile.chip}</span>}
            </a>
          </li>
        ))}
      </ul>
    </>
  );
}
