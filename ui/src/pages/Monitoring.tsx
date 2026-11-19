import { useState } from "react";
import { Cpu, Gauge, HardDrive, Network } from "lucide-react";
import { LineChart, RangePicker } from "../charts";
import { useMetrics, useSystem } from "../data";
import { byteUnit, fmtBytes, fmtRate, niceTop, RANGES, type RangeKey } from "../lib";
import { Card, PageHead, State } from "../ui";
import HardwareSection from "../features/hardware/HardwareSection";
import NetSection from "../features/netmon/NetSection";
import ExposureSection from "../features/exposure/ExposureSection";

export default function Monitoring() {
  const [range, setRange] = useState<RangeKey>("1h");
  const met = useMetrics(range);
  const sys = useSystem();
  const m = met.data;
  const label = RANGES.find((r) => r.key === range)?.label ?? "";
  const loadMax = m ? Math.max(1, ...m.load.map((p) => p[1])) : 1;

  return (
    <>
      <PageHead title="Pantau" desc="Sumber daya dan lalu lintas jaringan Mac mini. Satu titik dicatat tiap 30 detik, disimpan 7 hari, dan hanya berisi pengukuran nyata." action={<RangePicker value={range} onChange={setRange} />} />
      <State loading={met.loading && !m} error={met.error} onRetry={met.reload}>
        {m && (
          <>
            <Card title={<><Cpu size={18} className="icon" aria-hidden="true" /> CPU dan memori: kapan mesin paling sibuk? ({label})</>}>
              <LineChart
                title="CPU dan memori"
                rangeKey={range}
                domain={[0, 100]}
                tickFmt={(v) => `${Math.round(v)}%`}
                series={[
                  { key: "cpu", label: "CPU", pts: m.cpu, fmt: (v) => `${v.toFixed(0)}%` },
                  { key: "mem", label: "Memori", pts: m.mem, fmt: (v) => `${v.toFixed(0)}%` },
                ]}
              />
            </Card>

            <div className="grid2">
              <Card title={<><Gauge size={18} className="icon" aria-hidden="true" /> Beban rata-rata 1 menit</>}>
                <p className="small muted">Angka di atas jumlah inti ({sys.data?.cores ?? "?"}) berarti proses antre menunggu CPU.</p>
                <LineChart title="Beban rata-rata" rangeKey={range} domain={[0, niceTop(loadMax, 4)]} tickFmt={(v) => String(Math.round(v * 10) / 10).replace(".", ",")} series={[{ key: "load", label: "Beban", pts: m.load, fmt: (v) => v.toFixed(2).replace(".", ",") }]} />
              </Card>
              <Card title={<><HardDrive size={18} className="icon" aria-hidden="true" /> Pemakaian disk</>}>
                <p className="small muted">{sys.data?.disk_total ? `${fmtBytes(sys.data.disk_free)} bebas dari ${fmtBytes(sys.data.disk_total)}.` : "Ukuran disk belum terbaca."}</p>
                <LineChart title="Pemakaian disk" rangeKey={range} domain={[0, 100]} tickFmt={(v) => `${Math.round(v)}%`} series={[{ key: "disk", label: "Disk", pts: m.disk, fmt: (v) => `${v.toFixed(1).replace(".", ",")}%` }]} />
              </Card>
            </div>

            <HardwareSection range={range} />

            <NetSection range={range} />
            <ExposureSection />

            <h2 className="card-title" style={{ marginTop: 8 }}>
              <Network size={18} className="icon" aria-hidden="true" /> Bandwidth jaringan
            </h2>
            {m.nets.length === 0 ? (
              <Card>
                <div className="state" role="status">
                  <p className="state-title">Belum ada data bandwidth</p>
                  <div className="state-body">
                    <p>Laju dihitung dari selisih penghitung byte antar sampel, jadi grafik pertama muncul setelah dua sampel (sekitar 30 detik) dan hanya untuk interface yang punya alamat IPv4 (LAN dan Tailscale). Bila baru menyalakan Tailscale, interface-nya muncul di sampel berikutnya.</p>
                  </div>
                </div>
              </Card>
            ) : (
              <>
                {m.nets.map((n) => {
                  const top = Math.max(1, ...n.rx.map((p) => p[1]), ...n.tx.map((p) => p[1]));
                  const hi = niceTop(top, 4096, byteUnit(Math.max(top, 4096) / 4));
                  return (
                    <Card key={n.iface} title={`${n.label}: berapa banyak lalu lintas masuk dan keluar?`}>
                      <LineChart
                        title={`Bandwidth ${n.label}`}
                        rangeKey={range}
                        domain={[0, hi]}
                        tickFmt={(v) => fmtRate(v)}
                        series={[
                          { key: "rx", label: "Masuk", pts: n.rx, fmt: fmtRate },
                          { key: "tx", label: "Keluar", pts: n.tx, fmt: fmtRate },
                        ]}
                      />
                    </Card>
                  );
                })}
                <Card title={`Total lalu lintas (${label})`}>
                  <div className="tbl-wrap">
                    <table className="tbl">
                      <thead>
                        <tr>
                          <th scope="col">Interface</th>
                          <th scope="col" className="num">Masuk sekarang</th>
                          <th scope="col" className="num">Keluar sekarang</th>
                          <th scope="col" className="num">Total masuk</th>
                          <th scope="col" className="num">Total keluar</th>
                        </tr>
                      </thead>
                      <tbody>
                        {m.nets.map((n) => {
                          const now = sys.data?.nets?.find((x) => x.iface === n.iface);
                          return (
                            <tr key={n.iface}>
                              <th scope="row" className="mono">{n.label}</th>
                              <td className="num">{now ? fmtRate(now.rx_bps) : "n/a"}</td>
                              <td className="num">{now ? fmtRate(now.tx_bps) : "n/a"}</td>
                              <td className="num">{fmtBytes(n.total_rx)}</td>
                              <td className="num">{fmtBytes(n.total_tx)}</td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                  <p className="small muted">Total dijumlah dari selisih penghitung antar sampel. Lalu lintas saat panel mati tidak ikut terhitung.</p>
                </Card>
              </>
            )}
          </>
        )}
      </State>

      <Card title="Proses teratas berdasarkan CPU">
        <State loading={sys.loading && !sys.data} error={sys.error} empty={!!sys.data && !(sys.data.top ?? []).length} emptyText="Daftar proses belum terbaca." onRetry={sys.reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Proses</th>
                  <th scope="col" className="num">CPU</th>
                  <th scope="col" className="num">Memori (RSS)</th>
                </tr>
              </thead>
              <tbody>
                {(sys.data?.top ?? []).map((p, i) => (
                  <tr key={`${p.name}-${i}`}>
                    <th scope="row" className="mono">{p.name}</th>
                    <td className="num">{p.cpu.toFixed(1).replace(".", ",")}%</td>
                    <td className="num">{fmtBytes(p.rss_mb * 1024 * 1024)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="small muted">Hanya nama proses; argumen perintah tidak ditampilkan karena bisa memuat rahasia.</p>
        </State>
      </Card>
    </>
  );
}
