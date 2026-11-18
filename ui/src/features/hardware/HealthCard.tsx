import { Thermometer } from "lucide-react";
import { Card, Chip, Mark, State } from "../../ui";
import { fanText, forecastText, fmtTemp, freshness, num, pressureStatus, smartStatus, tempStatus, throttleStatus, verdict, type Status } from "./model";
import { useHw } from "./useHw";

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
              <ul className="list">
                {rows.map((r) => (
                  <li key={r.key} className="list-row">
                    <div>
                      <p>
                        <strong>{r.label}</strong>
                      </p>
                      {r.hint && <p className="small muted">{r.hint}</p>}
                    </div>
                    <div style={{ textAlign: "right" }}>
                      <p className="nowrap" style={{ fontVariantNumeric: "tabular-nums" }}>
                        {r.value}
                      </p>
                      <Chip tone={r.st.tone}>{r.st.text}</Chip>
                    </div>
                  </li>
                ))}
                {fc && (
                  <li className="list-row">
                    <div>
                      <p>
                        <strong>Perkiraan disk penuh</strong>
                      </p>
                      <p className="small muted">{fc.text}</p>
                    </div>
                    <Chip tone={fc.tone}>{fc.tone === "mute" ? "Belum cukup data" : fc.tone === "ok" ? "Aman" : fc.tone === "warn" ? "Perhatian" : "Mendesak"}</Chip>
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
