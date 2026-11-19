import { Thermometer } from "lucide-react";
import { get } from "../../api";
import { useT } from "../../i18n";
import { useSystem } from "../../data";
import { Card, Chip, Mark, State, usePoll } from "../../ui";
import { backupStatus, certStatus, fanText, forecastText, fmtTemp, freshness, num, pressureStatus, smartStatus, tempStatus, throttleStatus, verdict, type Status } from "./model";
import { useHw } from "./useHw";
import { useNetmon } from "../netmon/useNetmon";
import { fmtMs, internetStatus, tailscaleStatus } from "../netmon/model";

// Ikon Thermometer: kartu ini tentang suhu dan kondisi komponen, jadi ikonnya relevan dengan isinya.
// Satu fokus per layar: judul kartu berisi kesimpulan (normal / perlu dicek); baris di bawahnya rincian.

interface Row {
  key: string;
  label: string;
  value: string;
  hint?: string;
  st: Status;
}

export default function HealthCard() {
  const t = useT();
  const hw = useHw("1h");
  const sys = useSystem();
  const net = useNetmon("1h");
  const sites = usePoll(() => get<{ sites: { system?: boolean; enabled?: boolean; ssl_days: number | null }[] }>("/api/sites"), 120000);
  const d = hw.data;
  const now = Math.floor(Date.now() / 1000);
  const th = d?.thresholds;
  const s = d?.now;
  const fresh = freshness(d, now);
  // Angka basi tidak boleh tampil sebagai kondisi sekarang
  const live = !!s && !d?.stale && s.source !== "none";

  const rows: Row[] =
    d && th
      ? [
          { key: "throttle", label: t("system.hw.h.throttle"), value: live ? (s!.thermal?.throttled ? t("system.hw.h.limited", { n: s!.thermal.speed_limit }) : t("system.hw.h.full")) : t("fmt.na"), hint: t("system.hw.h.throttleHint"), st: live ? throttleStatus(s!.thermal) : { tone: "mute", text: t("system.hw.unreadable") } },
          { key: "cpu", label: t("system.hw.h.cpu"), value: live ? fmtTemp(s!.temps.cpu) : t("fmt.na"), st: live ? tempStatus(s!.temps.cpu, th.cpu_warn, th.cpu_bad) : { tone: "mute", text: t("system.hw.unreadable") } },
          { key: "ssd", label: t("system.hw.h.ssdT"), value: live ? fmtTemp(s!.temps.ssd) : t("fmt.na"), hint: s?.temps.ssd == null && live ? t("system.hw.h.ssdHint") : undefined, st: live ? tempStatus(s!.temps.ssd, th.ssd_warn, th.ssd_bad) : { tone: "mute", text: t("system.hw.unreadable") } },
          { key: "mem", label: t("system.hw.h.mem"), value: live && s!.mem?.pressure_pct != null ? `${num(s!.mem!.pressure_pct)}%` : t("fmt.na"), hint: live && s!.mem?.swap_used_mb != null ? t("system.hw.h.swap", { n: num(s!.mem!.swap_used_mb, 1) }) : undefined, st: live ? pressureStatus(s!.mem?.pressure_pct, th.press_warn, th.press_bad) : { tone: "mute", text: t("system.hw.unreadable") } },
          { key: "smart", label: t("system.hw.h.smart"), value: d.ssd?.smart || t("fmt.na"), hint: d.ssd ? t("system.hw.h.smartHint", { name: d.ssd.name || "SSD" }) : undefined, st: smartStatus(d.ssd?.smart) },
          { key: "fan", label: t("system.hw.h.fan"), value: live ? fanText(s!.fan) : t("fmt.na"), st: { tone: live && s!.fan ? "ok" : "mute", text: live && s!.fan ? t("system.hw.h.spinning") : t("system.hw.unreadable") } },
        ]
      : [];
  // Sertifikat dan backup: bukan perangkat keras, tetapi menjawab pertanyaan yang sama ("apakah mesin ini sehat?")
  const sslList = (sites.data?.sites ?? []).filter((x) => !x.system && x.enabled !== false && x.ssl_days != null).map((x) => x.ssl_days as number);
  const minSsl = sslList.length ? Math.min(...sslList) : null;
  const bk = backupStatus(sys.data?.backup, now);
  if (d && sys.data) {
    rows.push({ key: "cert", label: t("system.hw.h.cert"), value: minSsl == null ? t("fmt.na") : t("fmt.day", { n: minSsl }), hint: minSsl == null ? t("system.hw.h.certNone") : t("system.hw.h.certHint"), st: certStatus(minSsl) });
    rows.push({ key: "backup", label: t("system.hw.h.backup"), value: sys.data.backup ? t("fmt.ago", { span: t("fmt.hour", { n: Math.max(0, Math.floor((now - sys.data.backup.at) / 3600)) }) }) : t("fmt.na"), hint: bk.text, st: bk.st });
  }
  const nd = net.data;
  if (d && nd) {
    const nlive = !!nd.now && !nd.stale;
    const tsS = nlive ? tailscaleStatus(nd.now!.tailscale) : { tone: "mute" as const, text: t("system.hw.unreadable") };
    rows.unshift({ key: "ts", label: "Tailscale", value: nlive && nd.now!.tailscale ? t("system.hw.h.tsPeers", { n: nd.now!.tailscale.peers_online }) : t("fmt.na"), hint: t("system.hw.h.tsHint"), st: tsS });
    const inS = nlive ? internetStatus(nd.now!.internet, nd.thresholds.slow_ms) : { tone: "mute" as const, text: t("system.hw.unreadable") };
    rows.splice(1, 0, { key: "inet", label: "Internet", value: nlive ? fmtMs(nd.now!.internet.ms) : t("fmt.na"), hint: t("system.hw.h.inetHint"), st: inS });
  }
  const v = verdict(rows.map((r) => r.st));
  const fc = d ? forecastText(d.forecast) : null;

  return (
    <Card
      title={
        <>
          <Thermometer size={18} className="icon" aria-hidden="true" /> {t("system.hw.h.title")}
          {d && (
            <span className="small" style={{ marginLeft: 8, fontWeight: 600 }}>
              <Mark tone={v.tone} /> {v.text}
            </span>
          )}
        </>
      }
      action={
        <a className="btn btn-sm" href="#/pantau">
          {t("system.hw.h.chart")}<span className="sr">{t("system.hw.h.chartSr")}</span>
        </a>
      }
    >
      <State loading={hw.loading && !d} error={hw.error} onRetry={hw.reload}>
        {d && (
          <>
            <p className="small muted">
              {fresh.tone !== "ok" && <Chip tone={fresh.tone === "mute" ? "mute" : "warn"}>{fresh.tone === "mute" ? t("system.hw.waiting") : t("system.hw.attention")}</Chip>} {fresh.text}
            </p>
            {rows.length > 0 && (
              <ul className="health-grid" aria-label={t("system.hw.h.aria")}>
                {rows.map((r) => (
                  <li key={r.key} className="health-item">
                    <span className="health-label">{r.label}</span>
                    <span className="health-value">{r.value}</span>
                    <Chip tone={r.st.tone}>{r.st.text}</Chip>
                    {r.hint && <span className="small muted health-hint">{r.hint}</span>}
                  </li>
                ))}
                {fc && (
                  <li className="health-item">
                    <span className="health-label">{t("system.hw.h.fcFull")}</span>
                    <span className="health-value">{d.forecast.ok && d.forecast.days_to_full != null ? t("fmt.day", { n: Math.round(d.forecast.days_to_full) }) : d.forecast.ok ? t("system.hw.h.stable") : t("fmt.na")}</span>
                    <Chip tone={fc.tone}>{fc.tone === "mute" ? t("system.hw.h.noData") : fc.tone === "ok" ? t("system.hw.safe") : fc.tone === "warn" ? t("system.hw.attention") : t("system.hw.h.urgent")}</Chip>
                    <span className="small muted health-hint">{fc.text}</span>
                  </li>
                )}
              </ul>
            )}
          </>
        )}
      </State>
    </Card>
  );
}
