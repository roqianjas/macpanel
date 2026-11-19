import { Thermometer } from "lucide-react";
import { get } from "../../api";
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
          { key: "throttle", label: "Kecepatan CPU", value: live ? (s!.thermal?.throttled ? `dibatasi ${s!.thermal.speed_limit}%` : "penuh") : "n/a", hint: "macOS memperlambat CPU sendiri bila terlalu panas", st: live ? throttleStatus(s!.thermal) : { tone: "mute", text: "Tidak terbaca" } },
          { key: "cpu", label: "Suhu CPU", value: live ? fmtTemp(s!.temps.cpu) : "n/a", st: live ? tempStatus(s!.temps.cpu, th.cpu_warn, th.cpu_bad) : { tone: "mute", text: "Tidak terbaca" } },
          { key: "ssd", label: "Suhu SSD", value: live ? fmtTemp(s!.temps.ssd) : "n/a", hint: s?.temps.ssd == null && live ? "Suhu SSD hanya tersedia dari Netdata" : undefined, st: live ? tempStatus(s!.temps.ssd, th.ssd_warn, th.ssd_bad) : { tone: "mute", text: "Tidak terbaca" } },
          { key: "mem", label: "Tekanan memori", value: live && s!.mem?.pressure_pct != null ? `${num(s!.mem!.pressure_pct)}%` : "n/a", hint: live && s!.mem?.swap_used_mb != null ? `swap terpakai ${num(s!.mem!.swap_used_mb, 1)} MB` : undefined, st: live ? pressureStatus(s!.mem?.pressure_pct, th.press_warn, th.press_bad) : { tone: "mute", text: "Tidak terbaca" } },
          { key: "smart", label: "Kesehatan SSD", value: d.ssd?.smart || "n/a", hint: d.ssd ? `${d.ssd.name || "SSD"}, status SMART (persen keausan tidak tersedia tanpa sudo)` : undefined, st: smartStatus(d.ssd?.smart) },
          { key: "fan", label: "Kipas", value: live ? fanText(s!.fan) : "n/a", st: { tone: live && s!.fan ? "ok" : "mute", text: live && s!.fan ? "Berputar" : "Tidak terbaca" } },
        ]
      : [];
  // Sertifikat dan backup: bukan perangkat keras, tetapi menjawab pertanyaan yang sama ("apakah mesin ini sehat?")
  const sslList = (sites.data?.sites ?? []).filter((x) => !x.system && x.enabled !== false && x.ssl_days != null).map((x) => x.ssl_days as number);
  const minSsl = sslList.length ? Math.min(...sslList) : null;
  const bk = backupStatus(sys.data?.backup, now);
  if (d && sys.data) {
    rows.push({ key: "cert", label: "Sertifikat situs", value: minSsl == null ? "n/a" : `${minSsl} hari`, hint: minSsl == null ? "Belum ada situs ber-SSL" : "sisa terpendek di antara situs aktif", st: certStatus(minSsl) });
    rows.push({ key: "backup", label: "Backup terakhir", value: sys.data.backup ? `${Math.max(0, Math.floor((now - sys.data.backup.at) / 3600))} jam lalu` : "n/a", hint: bk.text, st: bk.st });
  }
  const nd = net.data;
  if (d && nd) {
    const nlive = !!nd.now && !nd.stale;
    const tsS = nlive ? tailscaleStatus(nd.now!.tailscale) : { tone: "mute" as const, text: "Tidak terbaca" };
    rows.unshift({ key: "ts", label: "Tailscale", value: nlive && nd.now!.tailscale ? `${nd.now!.tailscale.peers_online} perangkat online` : "n/a", hint: "jalur akses jarak jauh ke mesin ini", st: tsS });
    const inS = nlive ? internetStatus(nd.now!.internet, nd.thresholds.slow_ms) : { tone: "mute" as const, text: "Tidak terbaca" };
    rows.splice(1, 0, { key: "inet", label: "Internet", value: nlive ? fmtMs(nd.now!.internet.ms) : "n/a", hint: "waktu sambung TCP keluar", st: inS });
  }
  const v = verdict(rows.map((r) => r.st));
  const fc = d ? forecastText(d.forecast) : null;

  return (
    <Card
      title={
        <>
          <Thermometer size={18} className="icon" aria-hidden="true" /> Kesehatan mesin
          {d && (
            <span className="small" style={{ marginLeft: 8, fontWeight: 600 }}>
              <Mark tone={v.tone} /> {v.text}
            </span>
          )}
        </>
      }
      action={
        <a className="btn btn-sm" href="#/pantau">
          Grafik suhu<span className="sr"> dan perangkat keras</span>
        </a>
      }
    >
      <State loading={hw.loading && !d} error={hw.error} onRetry={hw.reload}>
        {d && (
          <>
            <p className="small muted">
              {fresh.tone !== "ok" && <Chip tone={fresh.tone === "mute" ? "mute" : "warn"}>{fresh.tone === "mute" ? "Menunggu" : "Perhatian"}</Chip>} {fresh.text}
            </p>
            {rows.length > 0 && (
              <ul className="health-grid" aria-label="Rincian kesehatan mesin">
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
                    <span className="health-label">Perkiraan disk penuh</span>
                    <span className="health-value">{d.forecast.ok && d.forecast.days_to_full != null ? `${num(d.forecast.days_to_full, 0)} hari` : d.forecast.ok ? "stabil" : "n/a"}</span>
                    <Chip tone={fc.tone}>{fc.tone === "mute" ? "Belum cukup data" : fc.tone === "ok" ? "Aman" : fc.tone === "warn" ? "Perhatian" : "Mendesak"}</Chip>
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
