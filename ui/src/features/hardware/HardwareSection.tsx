import { Fan, HardDrive, MemoryStick, Plug, Thermometer } from "lucide-react";
import { LineChart } from "../../charts";
import { fmtBytes, niceTop, RANGES, type RangeKey } from "../../lib";
import { Card, Chip, State } from "../../ui";
import { fanText, forecastText, fmtTemp, freshness, num, smartStatus } from "./model";
import { useHw } from "./useHw";

// Ikon relevan dengan isi: Thermometer (suhu), Fan (kipas), Plug (daya listrik), MemoryStick (memori), HardDrive (SSD).
// Tiap grafik berjudul pertanyaan yang dijawabnya; grafik tanpa data tampil sebagai keterangan, bukan garis datar.

export default function HardwareSection({ range }: { range: RangeKey }) {
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
        <Thermometer size={18} className="icon" aria-hidden="true" /> Perangkat keras
      </h2>
      <State loading={hw.loading && !d} error={hw.error} onRetry={hw.reload}>
        {d && (
          <>
            <p className="small muted">
              {fresh.tone !== "ok" && <Chip tone={fresh.tone === "mute" ? "mute" : "warn"}>{fresh.tone === "mute" ? "Menunggu" : "Perhatian"}</Chip>} {fresh.text}
            </p>

            <Card title={<>Suhu komponen: kapan mesin paling panas? ({label})</>}>
              <LineChart
                title="Suhu komponen"
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
                Puncak dalam rentang ini: CPU {fmtTemp(pk.cpu)}, GPU {fmtTemp(pk.gpu)}, SSD {fmtTemp(pk.ssd)}. Grafik memakai rata-rata per ember waktu, jadi lonjakan singkat tampil lebih rendah daripada puncaknya.
                {d.throttled_samples > 0 && ` macOS membatasi kecepatan CPU pada ${d.throttled_samples} dari ${d.samples} sampel.`}
              </p>
            </Card>

            <div className="grid2">
              <Card title={<><Fan size={18} className="icon" aria-hidden="true" /> Kipas: seberapa keras bekerja?</>}>
                <p className="small muted">Sekarang: {fanText(d.now?.fan ?? null)}.</p>
                <LineChart title="Kecepatan kipas" rangeKey={range} domain={[0, fanTop]} tickFmt={(v) => `${Math.round(v)}`} series={[{ key: "fan", label: "Kipas", pts: ser.fan ?? [], fmt: (v) => `${Math.round(v)} rpm` }]} />
              </Card>
              <Card title={<><Plug size={18} className="icon" aria-hidden="true" /> Daya sistem: berapa watt dipakai?</>}>
                <p className="small muted">Sekarang: {d.now?.power?.system != null ? `${num(d.now.power.system, 1)} W` : "n/a"}. Dibaca dari macmon.</p>
                <LineChart title="Daya sistem" rangeKey={range} domain={[0, powTop]} tickFmt={(v) => `${Math.round(v)} W`} series={[{ key: "power", label: "Daya", pts: ser.power ?? [], fmt: (v) => `${num(v, 1)} W` }]} />
              </Card>
            </div>

            <div className="grid2">
              <Card title={<><MemoryStick size={18} className="icon" aria-hidden="true" /> Tekanan memori: apakah RAM cukup?</>}>
                <p className="small muted">Tekanan tinggi berarti macOS terpaksa memampatkan atau menukar isi memori ke disk, lebih berarti daripada persen memori terpakai.{d.now?.mem?.swap_used_mb != null && ` Swap terpakai ${num(d.now.mem.swap_used_mb, 1)} MB dari ${num(d.now.mem.swap_total_mb)} MB.`}</p>
                <LineChart title="Tekanan memori" rangeKey={range} domain={[0, 100]} tickFmt={(v) => `${Math.round(v)}%`} series={[{ key: "pressure", label: "Tekanan", pts: ser.pressure ?? [], fmt: (v) => `${num(v, 0)}%` }]} />
              </Card>
              <Card title={<><HardDrive size={18} className="icon" aria-hidden="true" /> SSD: sehat dan cukup lega?</>}>
                {d.ssd ? (
                  <dl className="kv">
                    <div>
                      <dt>Perangkat</dt>
                      <dd>{d.ssd.name || "n/a"}</dd>
                    </div>
                    <div>
                      <dt>Status SMART</dt>
                      <dd>
                        <Chip tone={smart.tone}>{smart.text}</Chip> <span className="small muted">{d.ssd.smart || "tidak ada status"}</span>
                      </dd>
                    </div>
                    <div>
                      <dt>Ruang</dt>
                      <dd>
                        {fmtBytes(d.ssd.free)} bebas dari {fmtBytes(d.ssd.total)}
                      </dd>
                    </div>
                    <div>
                      <dt>Keausan</dt>
                      <dd className="small muted">Persen keausan tidak tersedia tanpa sudo, jadi tidak ditampilkan.</dd>
                    </div>
                  </dl>
                ) : (
                  <p className="small muted">Info SSD belum terbaca. Panel membacanya saat start lalu tiap 10 menit.</p>
                )}
                {fc && (
                  <p className="small" style={{ marginTop: 10 }}>
                    <strong>Perkiraan penuh:</strong> {fc.text}
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
