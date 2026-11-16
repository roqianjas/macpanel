import { Boxes, Database, Download, Globe, Timer } from "lucide-react";
import { get, fmtTime } from "../api";
import { Card, Chip, PageHead, State, statusTone, useLoad } from "../ui";

interface Counts {
  sites: number;
  mysql: number;
  pgsql: number;
  php: number;
  workers: number;
  jobs: number;
}
interface JobRow {
  id: number;
  at: number;
  user: string;
  action: string;
  status: string;
}

export default function Overview() {
  const c = useLoad(async (): Promise<Counts> => {
    const [s, d, p, w, j] = await Promise.all([get<any>("/api/sites"), get<any>("/api/db"), get<any>("/api/php"), get<any>("/api/tasks/worker"), get<any>("/api/tasks/job")]);
    return {
      sites: s.sites.filter((x: any) => !x.system).length,
      mysql: d.databases.mysql.length,
      pgsql: d.databases.pgsql.length,
      php: p.php.filter((x: any) => x.installed).length,
      workers: w.tasks.length,
      jobs: j.tasks.length,
    };
  });
  const jobs = useLoad(() => get<{ jobs: JobRow[] }>("/api/jobs"));
  const cards = c.data
    ? [
        { href: "#/website", icon: Globe, label: "Situs Anda", value: c.data.sites, hint: "di luar situs sistem" },
        { href: "#/database", icon: Database, label: "Database", value: c.data.mysql + c.data.pgsql, hint: `${c.data.mysql} MySQL, ${c.data.pgsql} PostgreSQL` },
        { href: "#/runtime", icon: Boxes, label: "Versi PHP terpasang", value: c.data.php, hint: "dengan pool FPM" },
        { href: "#/tugas", icon: Timer, label: "Worker dan cron", value: c.data.workers + c.data.jobs, hint: `${c.data.workers} worker, ${c.data.jobs} cron` },
      ]
    : [];
  return (
    <>
      <PageHead title="Ringkasan" desc="Kondisi singkat Mac mini. Grafik sumber daya ada di portal home.lab dan stats.lab." />
      <State loading={c.loading && !c.data} error={c.error} onRetry={c.reload}>
        <ul className="stat-grid">
          {cards.map((k) => (
            <li key={k.label}>
              <a className="stat" href={k.href}>
                <k.icon size={22} aria-hidden="true" className="stat-ic" />
                <span className="stat-val">{k.value}</span>
                <span className="stat-label">{k.label}</span>
                <span className="small muted">{k.hint}</span>
              </a>
            </li>
          ))}
        </ul>
      </State>
      <Card
        title="Aksi terakhir"
        action={
          <a className="btn" href="#/aktivitas">
            <Download size={18} aria-hidden="true" />
            Semua aktivitas
          </a>
        }
      >
        <State loading={jobs.loading && !jobs.data} error={jobs.error} empty={!!jobs.data && jobs.data.jobs.length === 0} emptyText="Belum ada aksi yang dijalankan lewat panel." onRetry={jobs.reload}>
          <ul className="list">
            {jobs.data?.jobs.slice(0, 6).map((j) => (
              <li key={j.id} className="list-row">
                <div>
                  <p className="mono">{j.action}</p>
                  <p className="small muted">
                    {fmtTime(j.at)} oleh {j.user}
                  </p>
                </div>
                <Chip tone={statusTone(j.status)} busy={j.status === "berjalan"}>
                  {j.status}
                </Chip>
              </li>
            ))}
          </ul>
        </State>
      </Card>
    </>
  );
}
