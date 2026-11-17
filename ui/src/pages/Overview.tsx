import { Activity, Boxes, Cpu, Database, Download, Globe, HardDrive, MemoryStick, Network, Server, Timer } from "lucide-react";
import { get } from "../api";
import { CountUp, LineChart, RangePicker, Ring, Sparkline, toneOf } from "../charts";
import { stateText, stateTone, useAlerts, useMetrics, useServices, useSystem } from "../data";
import { fmtAgo, fmtBytes, fmtRate, fmtUptime, type RangeKey } from "../lib";
import { Card, Chip, Mark, State, usePoll } from "../ui";
import { useState } from "react";

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

  const hero = (
    <section className={`hero ${bad.length ? "hero-bad" : ""}`} aria-label="Status keseluruhan">
      <div>
        <h1 className="hero-title">
          <Mark tone={bad.length ? "bad" : "ok"} />
          {svc.loading && !svc.data ? "Memeriksa layanan…" : bad.length ? `${bad.length} layanan perlu dicek` : "Semua layanan berjalan"}
        </h1>
        <p className="hero-sub">
          {bad.length ? `Tidak berjalan: ${bad.map((b) => b.label).join(", ")}. Buka Aplikasi untuk menyalakan atau melihat lognya.` : `${running} dari ${total} layanan terpasang berjalan di ${si?.host ?? "Mac mini"}.`}
        </p>
      </div>
      <dl className="hero-facts">
        <div>
          <dt>Aktif sejak boot</dt>
          <dd>{si?.boot ? fmtUptime(now - si.boot) : "n/a"}</dd>
        </div>
        <div>
          <dt>IP tailnet</dt>
          <dd className="mono">{si?.tailnet_ip || "tidak tersambung"}</dd>
        </div>
        <div>
          <dt>Beban (1 mnt)</dt>
          <dd>{hasSample ? si!.load[0].toFixed(2).replace(".", ",") : "n/a"}</dd>
        </div>
      </dl>
    </section>
  );

  const tiles = [
    { href: "#/website", icon: Globe, label: "Website", ds: "Situs .lab, rewrite, SSL, config nginx", chip: counts.data ? (counts.data.sites ? `${counts.data.sitesOn} dari ${counts.data.sites} situs aktif` : "Belum ada situs Anda") : "" },
    { href: "#/aplikasi", icon: Server, label: "Aplikasi", ds: "nginx, PHP-FPM, database, cache", chip: svc.data ? `${running} dari ${total} berjalan` : "" },
    { href: "#/database", icon: Database, label: "Database", ds: "MariaDB, MySQL dan PostgreSQL per proyek", chip: counts.data ? `${counts.data.db} proyek` : "" },
    { href: "#/tugas", icon: Timer, label: "Worker dan cron", ds: "Pengganti Supervisor dan crontab", chip: counts.data ? `${counts.data.workers} worker, ${counts.data.jobs} cron` : "" },
    { href: "#/runtime", icon: Boxes, label: "PHP dan runtime", ds: "Versi berdampingan, Node, Go, Python", chip: counts.data ? `${counts.data.php} PHP terpasang` : "" },
    { href: "#/pembaruan", icon: Download, label: "Pembaruan", ds: "Upgrade aman dengan backup", chip: "" },
  ];

  return (
    <>
      {hero}
      {alerts.data && alerts.data.alerts.length > 0 && (
        <Card title={`Peringatan (${alerts.data.alerts.length})`}>
          <ul className="list">
            {alerts.data.alerts.map((a) => (
              <li key={a.id} className="list-row">
                <div>
                  <p>
                    <Chip tone={a.level === "bad" ? "bad" : "warn"}>{a.level === "bad" ? "Penting" : "Perhatian"}</Chip> <strong>{a.title}</strong>
                  </p>
                  <p className="small muted">{a.detail}</p>
                </div>
                <a className="btn btn-sm" href={a.link}>
                  Lihat<span className="sr"> {a.title}</span>
                </a>
              </li>
            ))}
          </ul>
          <p className="small muted">Peringatan hanya tampil di panel ini; tidak ada pesan keluar (email atau chat).</p>
        </Card>
      )}
      <ul className="tiles" aria-label="Akses cepat">
        {tiles.map((t) => (
          <li key={t.href}>
            <a className="tile" href={t.href}>
              <span className="tile-ic">
                <t.icon size={22} aria-hidden="true" />
              </span>
              <b>{t.label}</b>
              <span className="tile-ds">{t.ds}</span>
              {t.chip && <span className="chip chip-mute">{t.chip}</span>}
            </a>
          </li>
        ))}
      </ul>

      <State loading={sys.loading && !sys.data} error={sys.error} onRetry={sys.reload}>
        {si && (
          <ul className="kpis" aria-label="Sumber daya">
            <li className="card kpi">
              <h2 className="kpi-h"><Cpu size={16} aria-hidden="true" /> CPU</h2>
              <div className="kpi-body">
                <Ring pct={hasSample ? si.cpu : null} label="CPU terpakai" tone={toneOf(hasSample ? si.cpu : null)} />
                <div>
                  <p className="big">{hasSample ? <CountUp value={si.cpu} decimals={0} suffix="%" /> : "n/a"}</p>
                  <p className="small muted">{si.cores} inti</p>
                </div>
              </div>
              <Sparkline pts={spark.data?.cpu ?? []} label="CPU satu jam terakhir" />
            </li>
            <li className="card kpi">
              <h2 className="kpi-h"><MemoryStick size={16} aria-hidden="true" /> Memori</h2>
              <div className="kpi-body">
                <Ring pct={hasSample ? si.mem : null} label="Memori terpakai" tone={toneOf(hasSample ? si.mem : null)} />
                <div>
                  <p className="big">{hasSample ? <CountUp value={si.mem} decimals={0} suffix="%" /> : "n/a"}</p>
                  <p className="small muted">dari {fmtBytes(si.mem_total)}</p>
                </div>
              </div>
              <Sparkline pts={spark.data?.mem ?? []} label="Memori satu jam terakhir" />
            </li>
            <li className="card kpi">
              <h2 className="kpi-h"><HardDrive size={16} aria-hidden="true" /> Disk</h2>
              <div className="kpi-body">
                <Ring pct={si.disk_total ? si.disk : null} label="Disk terpakai" tone={toneOf(si.disk)} />
                <div>
                  <p className="big">{si.disk_total ? <CountUp value={si.disk} decimals={0} suffix="%" /> : "n/a"}</p>
                  <p className="small muted">{fmtBytes(si.disk_free)} bebas</p>
                </div>
              </div>
              {si.disk >= 85 && <p className="callout">Disk hampir penuh. Bersihkan backup lama atau cache.</p>}
            </li>
            <li className="card kpi">
              <h2 className="kpi-h"><Network size={16} aria-hidden="true" /> Jaringan</h2>
              {nets.length === 0 ? (
                <p className="small muted">Belum ada laju terukur. Muncul setelah dua sampel (sekitar 30 detik).</p>
              ) : (
                <>
                  <p className="big">{fmtRate(rx)}</p>
                  <p className="small muted">masuk sekarang, keluar {fmtRate(tx)} (kartu jaringan fisik)</p>
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
              <a className="small" href="#/pantau">Lihat riwayat bandwidth</a>
            </li>
          </ul>
        )}
      </State>

      <div className="split">
        <div className="stackc">
          <Card title={<><Activity size={18} className="icon" aria-hidden="true" /> Seberapa sibuk CPU dan memori?</>} action={<RangePicker value={range} onChange={setRange} />}>
          <State loading={met.loading && !met.data} error={met.error} onRetry={met.reload}>
            {met.data && (
              <LineChart
                title="CPU dan memori"
                rangeKey={range}
                domain={[0, 100]}
                tickFmt={(v) => `${Math.round(v)}%`}
                series={[
                  { key: "cpu", label: "CPU", pts: met.data.cpu, fmt: (v) => `${v.toFixed(0)}%` },
                  { key: "mem", label: "Memori", pts: met.data.mem, fmt: (v) => `${v.toFixed(0)}%` },
                ]}
              />
            )}
          </State>
          </Card>
          {si && (
            <Card title="Sistem">
              <dl className="sysinfo">
                <div><dt>Host</dt><dd>{si.host}</dd></div>
                <div><dt>Sistem</dt><dd>{si.os}</dd></div>
                <div><dt>Prosesor</dt><dd>{si.cores} inti</dd></div>
                <div><dt>Memori</dt><dd>{fmtBytes(si.mem_total)}</dd></div>
                <div><dt>Disk</dt><dd>{fmtBytes(si.disk_total)}</dd></div>
                <div>
                  <dt>Backup terakhir</dt>
                  <dd>{si.backup ? `${fmtAgo(now - si.backup.at)}${si.backup.local_only ? ", hanya lokal" : ""}` : "belum ada"}</dd>
                </div>
              </dl>
              {si.backup?.local_only && <p className="callout">Backup masih di disk yang sama, belum offsite.</p>}
            </Card>
          )}
        </div>
        <div className="stackc">
          <Card title="Status layanan" action={<a className="btn btn-sm" href="#/aplikasi">Kelola</a>}>
            <State loading={svc.loading && !svc.data} error={svc.error} onRetry={svc.reload}>
              <ul className="svc-list">
                {services.filter((s) => s.installed).map((s) => (
                  <li key={s.id}>
                    <a href="#/aplikasi">{s.label}</a>
                    <Chip tone={stateTone(s.state)}>{stateText[s.state]}</Chip>
                  </li>
                ))}
              </ul>
            </State>
          </Card>
        </div>
      </div>
    </>
  );
}
