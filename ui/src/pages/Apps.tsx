import { useEffect, useState } from "react";
import { Download, Play, RotateCw, ScrollText, Square, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { stateText, stateTone, useServices, type ServiceInfo } from "../data";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, ConfirmDialog, Mark, Modal, PageHead, State } from "../ui";

const ORDER = ["Web server", "PHP", "Database", "Cache dan antrean", "Monitoring", "Jaringan", "Alat pengembangan", "Sistem"];

export default function Apps() {
  const { data, error, loading, reload } = useServices();
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [busy, setBusy] = useState("");
  const [confirm, setConfirm] = useState<{ s: ServiceInfo; act: "stop" | "restart" | "remove" } | null>(null);
  const [log, setLog] = useState<ServiceInfo | null>(null);

  const services = data?.services ?? [];
  const groups = ORDER.map((g) => ({ g, items: services.filter((s) => s.group === g) })).filter((x) => x.items.length);

  const start = async (s: ServiceInfo) => {
    setBusy(s.id);
    setNote(null);
    try {
      await post(`/api/services/${s.id}/start`, {});
      setNote({ tone: "ok", text: `${s.label} berjalan.` });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      setBusy("");
      void reload();
    }
  };
  const install = async (s: ServiceInfo) => {
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/apps/${s.id}/install`, {});
      trackJob(r.job, `Pasang ${s.label}`, () => void reload());
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  const bad = services.filter((s) => s.installed && !s.optional && s.state !== "running");

  return (
    <>
      <PageHead title="Aplikasi" desc="Server dan layanan yang berjalan di Mac mini: nyalakan, hentikan, mulai ulang, dan baca lognya. Layanan opsional bisa dipasang dari sini." />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      {bad.length > 0 && <Alert tone="warn">{`Tidak berjalan: ${bad.map((b) => b.label).join(", ")}.`}</Alert>}
      <State loading={loading && !data} error={error} empty={!!data && groups.length === 0} emptyText="Daftar layanan kosong." onRetry={reload}>
        {groups.map(({ g, items }) => (
          <Card key={g} title={g}>
            <div className="tbl-wrap">
              <table className="tbl tbl-fixed">
                <colgroup><col style={{ width: "30%" }} /><col style={{ width: "128px" }} /><col style={{ width: "96px" }} /><col style={{ width: "64px" }} /><col /></colgroup>
                <thead>
                  <tr>
                    <th scope="col">Layanan</th>
                    <th scope="col">Status</th>
                    <th scope="col">Versi</th>
                    <th scope="col">Port</th>
                    <th scope="col"><span className="sr">Aksi</span></th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((s) => (
                    <tr key={s.id} className={s.installed ? "" : "row-off"}>
                      <th scope="row">
                        <span className="row"><Mark tone={stateTone(s.state)} /><span>{s.label}</span></span>
                        <span className="small muted" style={{ fontWeight: 400, display: "block" }}>{s.desc}</span>
                      </th>
                      <td><Chip tone={stateTone(s.state)}>{stateText[s.state]}</Chip></td>
                      <td className="mono">{s.version || "-"}</td>
                      <td className="mono">{s.port ?? "-"}</td>
                      <td className="cell-end">
                        <div className="row">
                          {!s.installed && s.optional && (
                            <button className="btn btn-sm" onClick={() => install(s)}>
                              <Download size={16} aria-hidden="true" /> Pasang<span className="sr"> {s.label}</span>
                            </button>
                          )}
                          {s.installed && s.ctl && s.state !== "running" && (
                            <button className="btn btn-sm" disabled={busy === s.id} onClick={() => start(s)}>
                              <Play size={16} aria-hidden="true" /> {busy === s.id ? "Memulai…" : "Mulai"}<span className="sr"> {s.label}</span>
                            </button>
                          )}
                          {s.installed && s.ctl && s.state === "running" && (
                            <>
                              <button className="btn btn-sm" onClick={() => setConfirm({ s, act: "restart" })}>
                                <RotateCw size={16} aria-hidden="true" /> Mulai ulang<span className="sr"> {s.label}</span>
                              </button>
                              <button className="btn btn-sm btn-danger-ghost" onClick={() => setConfirm({ s, act: "stop" })}>
                                <Square size={16} aria-hidden="true" /> Hentikan<span className="sr"> {s.label}</span>
                              </button>
                            </>
                          )}
                          {s.installed && !s.ctl && <span className="small muted">Dikelola di luar panel</span>}
                          {s.installed && s.has_log && (
                            <button className="btn btn-sm" onClick={() => setLog(s)}>
                              <ScrollText size={16} aria-hidden="true" /> Log<span className="sr"> {s.label}</span>
                            </button>
                          )}
                          {s.installed && s.optional && (
                            <button className="btn btn-sm btn-danger-ghost" onClick={() => setConfirm({ s, act: "remove" })}>
                              <Trash2 size={16} aria-hidden="true" /> Hapus<span className="sr"> {s.label}</span>
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        ))}
        <p className="small muted">Status diperiksa langsung ke port atau socket layanan, bukan dari catatan, dan menyegarkan diri tiap 30 detik. dnsmasq dan panel ini dikelola di luar panel: dnsmasq berjalan sebagai root, dan panel tidak boleh menghentikan dirinya sendiri.</p>
      </State>

      {confirm && (
        <ConfirmDialog
          title={confirm.act === "stop" ? `Hentikan ${confirm.s.label}` : confirm.act === "restart" ? `Mulai ulang ${confirm.s.label}` : `Hapus ${confirm.s.label}`}
          word={confirm.s.id}
          verb={confirm.act === "stop" ? "Hentikan" : confirm.act === "restart" ? "Mulai ulang" : "Hapus aplikasi"}
          body={
            <p>
              {confirm.act === "remove"
                ? "Layanan dihentikan lalu paketnya dihapus dari Homebrew. Datanya tidak dicadangkan."
                : confirm.s.group === "Database" || confirm.s.id === "nginx" || confirm.s.group === "PHP"
                  ? "Situs atau aplikasi yang memakainya akan terputus sebentar."
                  : "Layanan akan terputus sebentar."}{" "}
              Aksi ini meminta kode TOTP ulang.
            </p>
          }
          onConfirm={async () => {
            if (confirm.act === "remove") {
              const r = await post<{ job: number }>(`/api/apps/${confirm.s.id}/remove`, { confirm: confirm.s.id });
              trackJob(r.job, `Hapus ${confirm.s.label}`, () => void reload());
            } else {
              await post(`/api/services/${confirm.s.id}/${confirm.act}`, {});
              setNote({ tone: "ok", text: `${confirm.s.label} ${confirm.act === "stop" ? "dihentikan" : "dimulai ulang"}.` });
              void reload();
            }
          }}
          onClose={() => setConfirm(null)}
        />
      )}
      {log && <LogDialog s={log} onClose={() => setLog(null)} />}
    </>
  );
}

function LogDialog({ s, onClose }: { s: ServiceInfo; onClose: () => void }) {
  const [data, setData] = useState<{ log: string } | null>(null);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(true);
  const load = async () => {
    setLoading(true);
    try {
      setData(await get<{ log: string }>(`/api/services/${s.id}/logs?n=120`));
      setErr("");
    } catch (x) {
      setErr(errText(x));
    } finally {
      setLoading(false);
    }
  };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => void load(), []);
  return (
    <Modal title={`Log ${s.label}`} onClose={onClose} wide>
      <div className="stack">
        {loading && !data && <p className="muted" role="status">Memuat…</p>}
        {err && <Alert>{err}</Alert>}
        {data && <pre className="logbox mono">{data.log || "Log kosong."}</pre>}
        <div className="row-end">
          <button className="btn" onClick={load} disabled={loading}>Muat ulang</button>
          <button className="btn" onClick={onClose}>Tutup</button>
        </div>
      </div>
    </Modal>
  );
}
