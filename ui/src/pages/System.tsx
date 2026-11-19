import { useEffect, useState } from "react";
import { Skull } from "lucide-react";
import { get, post } from "../api";
import { fmtBytes } from "../lib";
import CleanSection from "../features/clean/CleanSection";
import MemSection from "../features/clean/MemSection";
import TailscaleSection from "../features/tailscale/TailscaleSection";
import UninstallGuide from "../features/uninstall/UninstallGuide";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, Tabs, useLoad } from "../ui";
import { t as tr, useT } from "../i18n";

// Label dibaca saat render (getter) supaya mengikuti bahasa aktif.
const TABS = [
  { id: "keamanan", get label() { return tr("system.tab.sec"); } },
  { id: "proses", get label() { return tr("system.tab.proc"); } },
  { id: "disk", get label() { return tr("system.tab.disk"); } },
  { id: "dns", label: "DNS" },
  { id: "tailscale", label: "Tailscale" },
  { id: "docker", label: "Docker" },
  { id: "uninstall", label: "Uninstall" },
];

export default function System({ sub = "" }: { sub?: string }) {
  const t = useT();
  const [tab, setTab] = useState(TABS.some((x) => x.id === sub) ? sub : "keamanan");
  useEffect(() => { if (TABS.some((x) => x.id === sub)) setTab(sub); }, [sub]);
  return (
    <>
      <PageHead title={t("nav.system")} desc={t("system.desc")} />
      <Tabs tabs={TABS} value={tab} onChange={setTab} label={t("system.tabs.aria")} />
      <div role="tabpanel" aria-labelledby={`tab-${tab}`} className="tabpanel">
        {tab === "keamanan" ? <Security /> : tab === "proses" ? <Procs /> : tab === "disk" ? <SpaceMemory /> : tab === "dns" ? <Dns /> : tab === "tailscale" ? <TailscaleSection /> : tab === "uninstall" ? <UninstallGuide /> : <Docker />}
      </div>
    </>
  );
}

interface Port {
  addr: string;
  port: number;
  proc: string;
  exposed: boolean;
}

function Security() {
  const t = useT();
  const q = useLoad(() => get<{ firewall: string; stealth: string; ssh: { password_auth: string; root_login: string; kbd_interactive: string }; ports: Port[] }>("/api/sys/security"));
  const d = q.data;
  const fwOn = !!d && /enabled/i.test(d.firewall);
  const sshOk = !!d && d.ssh.password_auth === "no" && !["yes"].includes(d.ssh.root_login);
  return (
    <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
      {d && (
        <div className="stack">
          <Card title={t("system.sec.hardening")}>
            <ul className="list">
              <li className="list-row"><span>{t("system.sec.fw")}</span><Chip tone={fwOn ? "ok" : "warn"}>{fwOn ? t("system.sec.on") : t("system.sec.off")}</Chip></li>
              <li className="list-row"><span>{t("system.sec.pw")}</span><Chip tone={d.ssh.password_auth === "no" ? "ok" : "bad"}>{d.ssh.password_auth === "no" ? t("system.sec.pwOff") : d.ssh.password_auth}</Chip></li>
              <li className="list-row"><span>{t("system.sec.root")}</span><Chip tone={d.ssh.root_login === "no" ? "ok" : "warn"}>{d.ssh.root_login}</Chip></li>
              <li className="list-row"><span>{t("system.sec.kbd")}</span><Chip tone={d.ssh.kbd_interactive === "no" ? "ok" : "warn"}>{d.ssh.kbd_interactive}</Chip></li>
            </ul>
            <p className="small muted">{t("system.sec.note1")} <span className="mono">sudo</span>{t("system.sec.note2")} {sshOk ? "" : t("system.sec.check")}</p>
          </Card>
          <Card title={t("system.sec.ports")}>
            <p className="small muted">{t("system.sec.portsHint")}</p>
            <div className="tbl-wrap">
              <table className="tbl">
                <caption className="sr">{t("system.sec.caption")}</caption>
                <thead><tr><th scope="col">{t("apps.col.port")}</th><th scope="col">{t("system.address")}</th><th scope="col">{t("system.tab.proc")}</th><th scope="col">{t("apps.col.status")}</th></tr></thead>
                <tbody>
                  {d.ports.map((p, i) => (
                    <tr key={i}>
                      <td className="mono">{p.port}</td>
                      <td className="mono">{p.addr}</td>
                      <td>{p.proc}</td>
                      <td><Chip tone={p.exposed ? "warn" : "ok"}>{p.exposed ? t("system.sec.open") : t("system.sec.local")}</Chip></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </div>
      )}
    </State>
  );
}

interface Proc {
  pid: number;
  name: string;
  cpu: number;
  mem: number;
  rss_mb: number;
  etime: string;
  killable: boolean;
}

function Procs() {
  const t = useT();
  const q = useLoad(() => get<{ procs: Proc[]; total: number }>("/api/sys/procs"));
  const [kill, setKill] = useState<Proc | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  return (
    <Card title={t("system.proc.title")} action={<button className="btn" onClick={q.reload}>{t("system.proc.refresh")}</button>}>
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="tbl-wrap">
          <table className="tbl">
            <caption className="sr">{t("system.proc.caption")}</caption>
            <thead><tr><th scope="col">PID</th><th scope="col">{t("system.name")}</th><th scope="col">CPU %</th><th scope="col">RAM</th><th scope="col">{t("system.proc.run")}</th><th scope="col"><span className="sr">{t("system.action")}</span></th></tr></thead>
            <tbody>
              {q.data?.procs.map((p) => (
                <tr key={p.pid}>
                  <td className="mono">{p.pid}</td>
                  <td>{p.name}</td>
                  <td className="mono">{p.cpu.toFixed(1)}</td>
                  <td className="mono">{fmtBytes(p.rss_mb * 1048576)}</td>
                  <td className="mono">{p.etime}</td>
                  <td className="cell-end">
                    {p.killable ? (
                      <button className="btn btn-danger-ghost btn-sm" onClick={() => setKill(p)}>
                        <Skull size={16} aria-hidden="true" />{t("apps.stop")}<span className="sr"> {p.name} {p.pid}</span>
                      </button>
                    ) : (
                      <span className="small muted">{t("system.proc.managed")}</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="small muted">{t("system.proc.note")}</p>
      </State>
      {kill && (
        <ConfirmDialog
          title={t("system.proc.stopTitle", { name: kill.name, pid: kill.pid })}
          word={kill.name}
          verb={t("system.proc.stopVerb")}
          body={<p>{t("system.proc.stopBody")}</p>}
          onConfirm={async () => {
            const r = await post<{ job?: number; output?: string }>("/api/sys/kill", { pid: kill.pid });
            setNote({ tone: "ok", text: r.output ?? t("system.proc.sent") });
            setTimeout(q.reload, 800);
          }}
          onClose={() => setKill(null)}
        />
      )}
    </Card>
  );
}

function SpaceMemory() {
  return (
    <div className="stack">
      <CleanSection />
      <MemSection />
      <Disk />
    </div>
  );
}

function Disk() {
  const t = useT();
  const q = useLoad(() => get<{ rows: { name: string; kb: number }[]; total: number; free: number }>("/api/sys/disk"));
  const max = Math.max(1, ...(q.data?.rows.map((r) => r.kb) ?? [1]));
  return (
    <Card title={t("system.disk.title")} action={<button className="btn" onClick={q.reload}>{t("system.disk.recount")}</button>}>
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {q.data && (
          <>
            <p className="small muted">{t("system.disk.d1", { free: fmtBytes(q.data.free), total: fmtBytes(q.data.total) })} <span className="mono">du</span> {t("system.disk.d2")}</p>
            <ul className="bars">
              {q.data.rows.map((r) => (
                <li key={r.name} className="bar-row">
                  <span className="mono bar-name">{r.name}</span>
                  <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (r.kb / max) * 100)}%` }} /></span>
                  <span className="mono bar-val">{fmtBytes(r.kb * 1024)}</span>
                </li>
              ))}
            </ul>
          </>
        )}
      </State>
    </Card>
  );
}

function Docker() {
  const t = useT();
  const q = useLoad(() => get<{ installed: boolean; running?: boolean; error?: string; containers?: { name: string; image: string; status: string; ports: string }[] }>("/api/sys/docker"));
  const d = q.data;
  return (
    <Card title="Docker">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {d && !d.installed && <p className="muted">{t("system.dk.none")}</p>}
        {d?.installed && !d.running && <Alert>{t("system.dk.down")}{d.error ? `: ${d.error}` : "."}</Alert>}
        {d?.running && ((d.containers ?? []).length === 0 ? <p className="muted">{t("system.dk.empty")}</p> : (
          <ul className="list">{d.containers!.map((c) => <li key={c.name} className="list-row"><div><p className="mono">{c.name}</p><p className="small muted">{c.image} · {c.status}{c.ports ? ` · ${c.ports}` : ""}</p></div></li>)}</ul>
        ))}
        <p className="small muted">{t("system.dk.ro")}</p>
      </State>
    </Card>
  );
}

function Dns() {
  const t = useT();
  const q = useLoad(() => get<{ tld: string; config: string[]; resolver: string | null; wildcard: string | null; tailscale_ip: string | null; wildcard_matches_tailscale: boolean | null; responding: boolean; rows: { name: string; ip: string | null; ok: boolean }[] }>("/api/sys/dns"));
  const d = q.data;
  return (
    <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
      {d && (
        <div className="stack">
          <Card title={`DNS *.${d.tld}`} action={<button className="btn" onClick={q.reload}>{t("system.dns.recheck")}</button>}>
            <ul className="list">
              <li className="list-row"><span>{t("system.dns.answers")}</span><Chip tone={d.responding ? "ok" : "bad"}>{d.responding ? t("system.yes") : t("system.no")}</Chip></li>
              <li className="list-row"><span>{t("system.dns.points")}</span><span className="mono">{d.wildcard ?? t("system.dns.none")}</span></li>
              <li className="list-row"><span>{t("system.dns.tsip")}</span><span className="mono">{d.tailscale_ip ?? t("system.dns.unreadable")}</span></li>
              <li className="list-row"><span>{t("system.dns.same")}</span>{d.wildcard_matches_tailscale === null ? <Chip tone="mute">{t("system.dns.unknown")}</Chip> : <Chip tone={d.wildcard_matches_tailscale ? "ok" : "warn"}>{d.wildcard_matches_tailscale ? t("system.yes") : t("system.dns.differs")}</Chip>}</li>
              <li className="list-row"><span>/etc/resolver/{d.tld}</span><span className="mono">{d.resolver ?? t("system.dns.none")}</span></li>
            </ul>
            <p className="small muted">{t("system.dns.note1")} <span className="mono">/opt/homebrew/etc/dnsmasq.conf</span> {t("system.dns.note2")}</p>
            {d.wildcard_matches_tailscale === false && <Alert tone="warn">{t("system.dns.mismatch")}</Alert>}
          </Card>
          <Card title={t("system.dns.perSite")}>
            <ul className="list">
              {d.rows.map((r) => (
                <li key={r.name} className="list-row"><span className="mono">{r.name}</span><span className="mono">{r.ip ?? t("system.dns.noAnswer")}</span><Chip tone={r.ok ? "ok" : "bad"}>{r.ok ? t("system.dns.ok") : t("system.dns.wrong")}</Chip></li>
              ))}
            </ul>
          </Card>
        </div>
      )}
    </State>
  );
}
