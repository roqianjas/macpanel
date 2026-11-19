import { useState } from "react";
import { Play, RotateCw, Square, Trash2, Download } from "lucide-react";
import { errText, get, post } from "../api";
import type { GetUrl } from "../api.gen";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, Tabs, usePoll } from "../ui";

interface Ctr { id: string; name: string; image: string; state: string; status: string; ports: string; project: string }
interface Img { id: string; repo: string; tag: string; size: string; created: string }
interface Proj { name: string; file: string }
interface St { installed: boolean; running: boolean; version: string; containers: number; running_containers: number; images: number }
interface Ov { status: St; containers: Ctr[]; images: Img[]; projects: Proj[] }
interface Stat { name: string; cpu: string; mem: string; memp: string }

const stateTone = (s: string) => (s === "running" ? "ok" : s === "exited" || s === "created" ? "mute" : "warn") as "ok" | "mute" | "warn";

export default function Docker({ goto }: { goto?: (id: string) => void }) {
  const ov = usePoll(() => get<Ov>("/api/docker"), 6000);
  const stats = usePoll(() => (ov.data?.status.running ? get<{ stats: Stat[] }>("/api/docker/stats") : Promise.resolve({ stats: [] })), 15000, [ov.data?.status.running]);
  const [tab, setTab] = useState("container");
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [logOf, setLogOf] = useState<{ title: string; url: GetUrl } | null>(null);
  const [rm, setRm] = useState<{ kind: "ctr" | "img"; ref: string } | null>(null);
  const [pull, setPull] = useState(false);
  const st = ov.data?.status;
  const statMap = new Map((stats.data?.stats ?? []).map((s) => [s.name, s]));

  const act = async (c: Ctr, a: "start" | "stop" | "restart") => {
    setNote(null);
    try {
      await post(`/api/docker/containers/${encodeURIComponent(c.name)}/${a}`, {});
      setNote({ tone: "ok", text: `Container ${c.name}: ${a} berhasil.` });
      ov.reload();
    } catch (x) { setNote({ tone: "bad", text: errText(x) }); }
  };
  const compose = async (p: Proj, a: "up" | "down") => {
    setNote(null);
    try {
      const r = await post<{ job: number }>(`/api/docker/compose/${p.name}/${a}`, {});
      trackJob(r.job, `Compose ${a} ${p.name}`, ov.reload);
    } catch (x) { setNote({ tone: "bad", text: errText(x) }); }
  };
  const prune = async () => {
    setNote(null);
    try {
      await post("/api/docker/images/prune", {});
      setNote({ tone: "ok", text: "Image yang tidak terpakai sudah dibersihkan." });
      ov.reload();
    } catch (x) { setNote({ tone: "bad", text: errText(x) }); }
  };

  return (
    <>
      <PageHead title="Docker" desc="Kelola container, image, dan proyek Compose. Port selalu diikat ke 127.0.0.1 dan dibuka lewat situs nginx." />
      <State loading={ov.loading && !ov.data} error={ov.error} onRetry={ov.reload}>
        {st && !st.installed && (
          <Card title="Docker belum terpasang">
            <p className="muted">Docker dijalankan lewat Colima: VM ringan tanpa aplikasi GUI (2 CPU, 4 GB RAM, hanya folder wwwroot yang dibagikan). Tidak memakai RAM sampai dijalankan. Pemasangan memerlukan internet dan beberapa menit.</p>
            <div className="row-end"><button className="btn btn-primary" onClick={() => goto?.("aplikasi")}>Buka halaman Aplikasi untuk memasang</button></div>
          </Card>
        )}
        {st && st.installed && !st.running && (
          <Card title="Docker terpasang tetapi belum berjalan">
            <p className="muted">Jalankan dari halaman Aplikasi (Docker). Start pertama mengunduh image VM dan bisa memakan beberapa menit.</p>
            <div className="row-end"><button className="btn btn-primary" onClick={() => goto?.("aplikasi")}>Buka halaman Aplikasi</button></div>
          </Card>
        )}
        {st?.running && ov.data && (
          <>
            {note && <Alert tone={note.tone}>{note.text}</Alert>}
            <div className="stats" style={{ display: "flex", gap: 12, flexWrap: "wrap", margin: "0 0 12px" }}>
              <Chip tone="ok">Berjalan · Docker {st.version}</Chip>
              <Chip tone="mute">{st.running_containers} dari {st.containers} container aktif</Chip>
              <Chip tone="mute">{st.images} image</Chip>
            </div>
            <Tabs label="Bagian Docker" value={tab} onChange={setTab} tabs={[{ id: "container", label: "Container" }, { id: "compose", label: "Proyek Compose" }, { id: "image", label: "Image" }]} />
            {tab === "container" && (
              <Card>
                {ov.data.containers.length === 0 ? <p className="muted">Belum ada container. Jalankan proyek Compose dari tab Proyek Compose.</p> : (
                  <div className="tbl-wrap"><table className="tbl">
                    <thead><tr><th scope="col">Nama</th><th scope="col">Status</th><th scope="col">Image</th><th scope="col">Port</th><th scope="col">CPU / RAM</th><th scope="col"><span className="sr">Aksi</span></th></tr></thead>
                    <tbody>
                      {ov.data.containers.map((c) => {
                        const s = statMap.get(c.name);
                        return (
                          <tr key={c.id}>
                            <td><span className="mono">{c.name}</span>{c.project && <div className="muted small">proyek {c.project}</div>}</td>
                            <td><Chip tone={stateTone(c.state)}>{c.state}</Chip><div className="muted small">{c.status}</div></td>
                            <td className="mono small">{c.image}</td>
                            <td className="mono small">{c.ports || "-"}</td>
                            <td className="mono small">{s ? `${s.cpu} · ${s.mem.split(" / ")[0]}` : "-"}</td>
                            <td><div className="row-end" style={{ flexWrap: "wrap" }}>
                              {c.state === "running" ? (
                                <button className="btn btn-icon" onClick={() => act(c, "stop")} aria-label={`Hentikan ${c.name}`}><Square size={16} aria-hidden /></button>
                              ) : (
                                <button className="btn btn-icon" onClick={() => act(c, "start")} aria-label={`Jalankan ${c.name}`}><Play size={16} aria-hidden /></button>
                              )}
                              <button className="btn btn-icon" onClick={() => act(c, "restart")} aria-label={`Mulai ulang ${c.name}`}><RotateCw size={16} aria-hidden /></button>
                              <button className="btn" onClick={() => setLogOf({ title: `Log ${c.name}`, url: `/api/docker/containers/${encodeURIComponent(c.name)}/logs?n=200` })}>Log<span className="sr"> {c.name}</span></button>
                              <button className="btn btn-danger-ghost btn-icon" onClick={() => setRm({ kind: "ctr", ref: c.name })} aria-label={`Hapus ${c.name}`}><Trash2 size={16} aria-hidden /></button>
                            </div></td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table></div>
                )}
              </Card>
            )}
            {tab === "compose" && (
              <Card>
                <p className="muted small">Panel mencari <span className="mono">compose.yaml</span> atau <span className="mono">docker-compose.yml</span> di folder situs (wwwroot). Sebelum dijalankan, berkas diperiksa: privileged, network host, cap_add, devices, mount di luar wwwroot atau docker.sock, dan port yang tidak terikat ke 127.0.0.1 ditolak.</p>
                {ov.data.projects.length === 0 ? <p className="muted">Belum ada proyek Compose di wwwroot.</p> : (
                  <div className="tbl-wrap"><table className="tbl">
                    <thead><tr><th scope="col">Proyek</th><th scope="col">Berkas</th><th scope="col">Container</th><th scope="col"><span className="sr">Aksi</span></th></tr></thead>
                    <tbody>
                      {ov.data.projects.map((p) => {
                        const cs = ov.data!.containers.filter((c) => c.project === p.name);
                        const up = cs.filter((c) => c.state === "running").length;
                        return (
                          <tr key={p.name}>
                            <td className="mono">{p.name}</td>
                            <td className="mono small">{p.file}</td>
                            <td>{cs.length ? <Chip tone={up === cs.length ? "ok" : "warn"}>{up} dari {cs.length} aktif</Chip> : <Chip tone="mute">belum dijalankan</Chip>}</td>
                            <td><div className="row-end" style={{ flexWrap: "wrap" }}>
                              <button className="btn" onClick={() => compose(p, "up")}>Jalankan</button>
                              <button className="btn" onClick={() => compose(p, "down")}>Hentikan</button>
                              <button className="btn" onClick={() => setLogOf({ title: `Log proyek ${p.name}`, url: `/api/docker/compose/${p.name}/logs?n=200` })}>Log<span className="sr"> {p.name}</span></button>
                            </div></td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table></div>
                )}
              </Card>
            )}
            {tab === "image" && (
              <Card title="Image" action={<div className="row-end"><button className="btn" onClick={prune}>Bersihkan tak terpakai</button><button className="btn btn-primary" onClick={() => setPull(true)}><Download size={16} aria-hidden /> Tarik image</button></div>}>
                {ov.data.images.length === 0 ? <p className="muted">Belum ada image.</p> : (
                  <div className="tbl-wrap"><table className="tbl">
                    <thead><tr><th scope="col">Image</th><th scope="col">Tag</th><th scope="col">Ukuran</th><th scope="col">Dibuat</th><th scope="col"><span className="sr">Aksi</span></th></tr></thead>
                    <tbody>
                      {ov.data.images.map((i) => (
                        <tr key={i.id + i.tag}>
                          <td className="mono">{i.repo}</td><td className="mono small">{i.tag}</td><td>{i.size}</td><td className="muted small">{i.created}</td>
                          <td><div className="row-end"><button className="btn btn-danger-ghost btn-icon" onClick={() => setRm({ kind: "img", ref: i.tag && i.tag !== "<none>" ? `${i.repo}:${i.tag}` : i.id })} aria-label={`Hapus image ${i.repo}`}><Trash2 size={16} aria-hidden /></button></div></td>
                        </tr>
                      ))}
                    </tbody>
                  </table></div>
                )}
              </Card>
            )}
          </>
        )}
      </State>
      {logOf && <LogDialog title={logOf.title} url={logOf.url} onClose={() => setLogOf(null)} />}
      {pull && <PullDialog onClose={() => setPull(false)} onDone={ov.reload} />}
      {rm && (
        <ConfirmDialog
          title={rm.kind === "ctr" ? "Hapus container?" : "Hapus image?"}
          body={<p>{rm.kind === "ctr" ? "Container dihentikan dan dihapus. Data di volume tidak ikut terhapus." : "Image dihapus dari disk; dapat ditarik lagi nanti."} Ketik <span className="mono">{rm.ref}</span> untuk konfirmasi.</p>}
          word={rm.ref}
          verb="Hapus"
          onClose={() => setRm(null)}
          onConfirm={async () => {
            if (rm.kind === "ctr") await post(`/api/docker/containers/${encodeURIComponent(rm.ref)}/remove`, { confirm: rm.ref });
            else await post("/api/docker/images/remove", { ref: rm.ref, confirm: rm.ref });
            setRm(null);
            ov.reload();
          }}
        />
      )}
    </>
  );
}

function LogDialog({ title, url, onClose }: { title: string; url: GetUrl; onClose: () => void }) {
  const { data, error, loading, reload } = usePoll(() => get<{ log: string }>(url), 5000);
  return (
    <Modal title={title} onClose={onClose} wide>
      <State loading={loading && !data} error={error} empty={!!data && !data.log.trim()} emptyText="Log masih kosong." onRetry={reload}>
        <pre className="logbox mono">{data?.log}</pre>
      </State>
      <div className="row-end"><button className="btn" onClick={onClose}>Tutup</button></div>
    </Modal>
  );
}

function PullDialog({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const [ref, setRef] = useState("");
  const [err, setErr] = useState("");
  const ok = /^[a-z0-9][a-z0-9._/:@-]{0,200}$/.test(ref);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr("");
    try {
      const r = await post<{ job: number }>("/api/docker/images/pull", { ref });
      trackJob(r.job, `Tarik image ${ref}`, onDone);
      onClose();
    } catch (x) { setErr(errText(x)); }
  };
  return (
    <Modal title="Tarik image" onClose={onClose}>
      <form onSubmit={submit}>
        {err && <Alert>{err}</Alert>}
        <Field label="Nama image" hint="Contoh: nginx:1.27, postgres:17, redis:7. Huruf kecil saja.">
          {(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="nginx:1.27" required maxLength={200} pattern="[a-z0-9][a-z0-9._/:@\-]{0,200}" title="Huruf kecil, angka, dan . _ / : @ -" value={ref} onChange={(e) => setRef(e.target.value.trim())} data-autofocus />}
        </Field>
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>Batal</button>
          <button className="btn btn-primary" disabled={!ok}>Tarik</button>
        </div>
      </form>
    </Modal>
  );
}
