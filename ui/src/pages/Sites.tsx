import { useEffect, useState } from "react";
import { ExternalLink, Pause, Play, Plus, Trash2 } from "lucide-react";
import { fmtBytes } from "../lib";
import { CloneModal, InstallModal } from "./Installer";
import { errText, fmtTime, get, post } from "../api";
import Deploy from "./Deploy";
import { Alert, Card, Chip, ConfirmDialog, Field, Modal, PageHead, State, Tabs, useLoad, type Tone } from "../ui";

interface Site {
  name: string;
  host: string;
  mode: string;
  target: string;
  php?: string;
  system: boolean;
  enabled: boolean;
  http: number;
  ms: number;
  ssl_days: number | null;
}
interface Rewrite {
  from: string;
  to: string;
  flag: string;
}
interface Model {
  mode: string;
  target: string;
  php?: string;
  preset?: string;
  rewrites: Rewrite[];
  max_body_mb: number;
  read_timeout_s: number;
  force_https: boolean;
  gzip: boolean;
  aliases?: string[];
  auth?: boolean;
  allow_ips?: string[];
}

const MODE_LABEL: Record<string, string> = { php: "PHP", proxy: "Proxy", statis: "Statis", kustom: "Kustom" };
const PHP_VERSIONS = ["8.2", "8.3", "8.4", "8.5"];

function health(s: Site): { tone: Tone; text: string } {
  if (!s.enabled) return { tone: "mute", text: "Dijeda" };
  if (s.http === 0) return { tone: "bad", text: "Tidak menjawab" };
  if (s.http >= 500) return { tone: "bad", text: `Galat ${s.http}` };
  if (s.http >= 400) return { tone: "warn", text: `${s.http}` };
  return { tone: "ok", text: `${s.http} · ${s.ms} ms` };
}

export default function Sites({ sub }: { sub: string }) {
  return sub ? <SiteDetail name={sub} /> : <SiteList />;
}

// ---------- daftar ----------

function SiteList() {
  const { data, error, loading, reload } = useLoad(() => get<{ sites: Site[] }>("/api/sites"));
  const [adding, setAdding] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [pause, setPause] = useState<Site | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const sites = data?.sites ?? [];
  const mine = sites.filter((s) => !s.system);

  const resume = async (s: Site) => {
    setNote(null);
    try {
      await post(`/api/sites/${s.name}/enable`, {});
      setNote({ tone: "ok", text: `${s.host} diaktifkan.` });
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    } finally {
      reload();
      setTimeout(reload, 1500); // nginx butuh sekitar 0,2-0,4 detik untuk memuat ulang; periksa lagi setelahnya
    }
  };

  return (
    <>
      <PageHead
        title="Website"
        desc={`Situs berdomain .lab, hanya terbuka dari tailnet. ${mine.length ? `${mine.filter((s) => s.enabled).length} dari ${mine.length} situs Anda aktif.` : "Belum ada situs Anda sendiri."}`}
        action={
          <>
            <button className="btn" onClick={() => setInstalling(true)}>Pasang cepat</button>
            <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            Tambah situs
          </button>
          </>
        }
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Card>
        <State loading={loading && !data} error={error} empty={!!data && sites.length === 0} emptyText="Belum ada situs. Pilih “Tambah situs” untuk membuat yang pertama." onRetry={reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Situs</th>
                  <th scope="col">Status</th>
                  <th scope="col">Jenis</th>
                  <th scope="col">PHP</th>
                  <th scope="col">Sertifikat</th>
                  <th scope="col">Sumber</th>
                  <th scope="col"><span className="sr">Aksi</span></th>
                </tr>
              </thead>
              <tbody>
                {sites.map((s) => {
                  const h = health(s);
                  return (
                    <tr key={s.name} className={s.enabled ? "" : "row-off"}>
                      <th scope="row">
                        <a className="row-link mono" href={`#/website/${s.name}`}>{s.host}</a>
                        {s.system && <> <Chip tone="mute">Sistem</Chip></>}
                      </th>
                      <td><Chip tone={h.tone}>{h.text}</Chip></td>
                      <td>{MODE_LABEL[s.mode] ?? s.mode}</td>
                      <td className="mono">{s.php ?? "-"}</td>
                      <td>
                        {s.ssl_days == null ? <span className="muted">-</span> : <Chip tone={s.ssl_days < 0 ? "bad" : s.ssl_days < 30 ? "warn" : "ok"}>{s.ssl_days < 0 ? "Kedaluwarsa" : `${s.ssl_days} hari`}</Chip>}
                      </td>
                      <td className="mono cell-trunc" title={s.target}>{s.mode === "proxy" ? `127.0.0.1:${s.target}` : s.target || "-"}</td>
                      <td className="cell-end">
                        <div className="row">
                          <a className="btn btn-sm" href={`#/website/${s.name}`}>{s.system ? "Lihat" : "Kelola"}<span className="sr"> {s.host}</span></a>
                          {!s.system && s.enabled && (
                            <button className="btn btn-sm" onClick={() => setPause(s)}>
                              <Pause size={16} aria-hidden="true" /> Jeda<span className="sr"> {s.host}</span>
                            </button>
                          )}
                          {!s.system && !s.enabled && (
                            <button className="btn btn-sm" onClick={() => resume(s)}>
                              <Play size={16} aria-hidden="true" /> Aktifkan<span className="sr"> {s.host}</span>
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="small muted">Status diperiksa lewat loopback ke nginx (HEAD /), bukan dari catatan. Situs yang dijeda tidak dilayani sampai diaktifkan lagi; konfigurasinya tetap tersimpan.</p>
        </State>
      </Card>
      {installing && <InstallModal onClose={() => setInstalling(false)} onDone={(n) => { setInstalling(false); location.hash = `#/website/${n}`; }} />}
      {adding && <AddSite onClose={() => setAdding(false)} onDone={(n) => { setAdding(false); location.hash = `#/website/${n}`; }} />}
      {pause && (
        <ConfirmDialog
          title={`Jeda ${pause.host}`}
          word={pause.name}
          verb="Jeda situs"
          body={<p>Situs berhenti dilayani (nginx tidak lagi memuat konfigurasinya). Folder proyek dan konfigurasi tidak dihapus. Meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            await post(`/api/sites/${pause.name}/disable`, {});
            setNote({ tone: "ok", text: `${pause.host} dijeda.` });
            reload();
            setTimeout(reload, 1500);
          }}
          onClose={() => setPause(null)}
        />
      )}
    </>
  );
}

function AddSite({ onClose, onDone }: { onClose: () => void; onDone: (name: string) => void }) {
  const [name, setName] = useState("");
  const [dbEngine, setDbEngine] = useState("");
  const [mode, setMode] = useState("php");
  const [target, setTarget] = useState("");
  const [php, setPhp] = useState("8.4");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const body: Record<string, string> = { name, target };
      if (mode === "php") body.php = php;
      await post("/api/sites", body);
      if (dbEngine) {
        try {
          await post("/api/db", { name: name.replace(/-/g, "_"), engine: dbEngine });
        } catch (x) {
          setErr(`Situs dibuat, tetapi database gagal dibuat: ${errText(x)}`);
          setBusy(false);
          return;
        }
      }
      onDone(name);
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  return (
    <Modal title="Tambah situs" onClose={onClose}>
      <form onSubmit={submit} className="stack">
        <Field label="Nama" hint={`Menjadi alamat https://${name || "nama"}.lab. Huruf kecil, angka, dan tanda hubung.`}>
          {(id, d) => <input id={id} aria-describedby={d} data-autofocus value={name} onChange={(e) => setName(e.target.value.toLowerCase())} required pattern="[a-z0-9]([a-z0-9\-]{0,38}[a-z0-9])?" />}
        </Field>
        <Field label="Jenis situs">
          {(id) => (
            <select id={id} value={mode} onChange={(e) => { setMode(e.target.value); setTarget(""); }}>
              <option value="php">PHP atau Laravel (folder public)</option>
              <option value="statis">Statis (folder berisi index.html)</option>
              <option value="proxy">Proxy ke aplikasi yang sudah berjalan (port)</option>
            </select>
          )}
        </Field>
        <Field label={mode === "proxy" ? "Port aplikasi" : "Folder"} hint={mode === "proxy" ? "1024 sampai 65535. Aplikasi harus mendengarkan di 127.0.0.1." : "Path absolut atau diawali ~/ dan harus di bawah ~/MacPanel/wwwroot."}>
          {(id, d) => <input id={id} aria-describedby={d} className="mono" value={target} onChange={(e) => setTarget(e.target.value)} required placeholder={mode === "proxy" ? "3000" : "~/MacPanel/wwwroot/org/proyek/public"} />}
        </Field>
        {mode === "php" && (
          <Field label="Versi PHP">
            {(id) => (
              <select id={id} value={php} onChange={(e) => setPhp(e.target.value)}>
                {PHP_VERSIONS.map((v) => <option key={v}>{v}</option>)}
              </select>
            )}
          </Field>
        )}
        {mode !== "statis" && (
          <Field label="Database (opsional)" hint={dbEngine ? `Membuat database dan user "${name.replace(/-/g, "_") || "nama"}" dengan kata sandi acak. Kredensial disimpan di ~/.config/macminiops/projects/ (tidak ditampilkan di sini).` : "Buat database kosong untuk proyek ini sekaligus."}>
            {(id, dd) => (
              <select id={id} aria-describedby={dd} value={dbEngine} onChange={(e) => setDbEngine(e.target.value)}>
                <option value="">Tanpa database</option>
                <option value="mysql">MySQL</option>
                <option value="pgsql">PostgreSQL</option>
              </select>
            )}
          </Field>
        )}
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>Batal</button>
          <button className="btn btn-primary" disabled={busy || !name || !target}>{busy ? "Membuat…" : "Buat situs"}</button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- detail ----------

const TABS = [
  { id: "umum", label: "Umum" },
  { id: "domain", label: "Domain" },
  { id: "akses", label: "Akses" },
  { id: "env", label: ".env" },
  { id: "trafik", label: "Trafik" },
  { id: "deploy", label: "Deploy" },
  { id: "rewrite", label: "Rewrite" },
  { id: "ssl", label: "SSL" },
  { id: "config", label: "Config nginx" },
  { id: "log", label: "Log" },
];

function SiteDetail({ name }: { name: string }) {
  const [tab, setTab] = useState("umum");
  const settings = useLoad(() => get<{ kind: string; model: Model | null; system: boolean }>(`/api/sites/${name}/settings`), [name]);
  const [removing, setRemoving] = useState(false);
  const [cloning, setCloning] = useState(false);
  const host = `${name}.lab`;
  const system = settings.data?.system ?? false;
  const missing = settings.error.includes("tidak ada");
  return (
    <>
      <PageHead
        title={host}
        desc={system ? "Situs sistem: hanya bisa dilihat dari panel." : missing ? "Situs ini tidak aktif (dijeda) atau tidak ada. Aktifkan dari daftar Website untuk mengelolanya." : "Atur situs, rewrite, sertifikat, dan config nginx."}
        action={
          <>
            <a className="btn" href="#/website">Semua situs</a>
            {!system && !missing && <button className="btn" onClick={() => setCloning(true)}>Kloning</button>}
            <a className="btn" href={`https://${host}`} target="_blank" rel="noopener noreferrer">
              <ExternalLink size={18} aria-hidden="true" />
              Buka situs<span className="sr"> (tab baru)</span>
            </a>
          </>
        }
      />
      {cloning && <CloneModal src={name} onClose={() => setCloning(false)} onDone={(n) => { setCloning(false); location.hash = `#/website/${n}`; }} />}
      {!missing && <Tabs tabs={system ? TABS.filter((t) => t.id !== "deploy") : TABS} value={tab} onChange={setTab} label="Bagian situs" />}
      {!missing && (
        <div role="tabpanel" aria-labelledby={`tab-${tab}`} className="tabpanel">
          {tab === "umum" || tab === "rewrite" || tab === "domain" || tab === "akses" ? (
            <State loading={settings.loading && !settings.data} error={settings.error} onRetry={settings.reload}>
              {settings.data &&
                (settings.data.model ? (
                  <>
                    <SettingsForm key={tab} name={name} kind={settings.data.kind} initial={settings.data.model} system={system} section={tab} onSaved={settings.reload} />
                    {tab === "akses" && !system && <AuthUsers name={name} enabled={!!settings.data.model.auth} />}
                  </>
                ) : (
                  <Card><p>Situs ini memakai config kustom yang tidak bisa diubah lewat formulir. Gunakan tab <strong>Config nginx</strong>.</p></Card>
                ))}
            </State>
          ) : tab === "trafik" ? (
            <Traffic name={name} />
          ) : tab === "env" ? (
            <EnvEditor name={name} />
          ) : tab === "deploy" ? (
            <Deploy name={name} />
          ) : tab === "ssl" ? (
            <Ssl name={name} />
          ) : tab === "config" ? (
            <ConfigEditor name={name} system={system} onSaved={settings.reload} />
          ) : (
            <Logs name={name} />
          )}
        </div>
      )}
      {missing && <Alert tone="warn">{settings.error}</Alert>}
      {!system && !missing && tab === "umum" && (
        <Card title="Hapus situs" className="danger-zone">
          <p className="muted small">Menghapus config dan sertifikat situs ini. Folder proyek tidak disentuh.</p>
          <div>
            <button className="btn btn-danger" onClick={() => setRemoving(true)}>
              <Trash2 size={18} aria-hidden="true" />
              Hapus {host}
            </button>
          </div>
        </Card>
      )}
      {removing && (
        <ConfirmDialog
          title={`Hapus ${host}`}
          word={name}
          verb="Hapus situs"
          body={<p>Situs akan berhenti dilayani. Aksi ini meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            await post(`/api/sites/${name}/remove`, { confirm: name });
            location.hash = "#/website";
          }}
          onClose={() => setRemoving(false)}
        />
      )}
    </>
  );
}

function SettingsForm({ name, kind, initial, system, section, onSaved }: { name: string; kind: string; initial: Model; system: boolean; section: string; onSaved: () => void }) {
  const [m, setM] = useState<Model>({ ...initial, rewrites: initial.rewrites ?? [] });
  const [replace, setReplace] = useState(false);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const set = <K extends keyof Model>(k: K, v: Model[K]) => setM((p) => ({ ...p, [k]: v }));
  const presets = m.mode === "php" ? ["laravel", "wordpress"] : m.mode === "statis" ? ["static", "spa"] : [];
  const kindLabel: Record<string, string> = { template: "Dibuat panel", legacy: "Dibuat addsite.sh (belum dikelola panel)", custom: "Diedit manual lewat editor config" };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      const body: Record<string, unknown> = { ...m, max_body_mb: Number(m.max_body_mb), read_timeout_s: Number(m.read_timeout_s) };
      if (replace) body.replace = true;
      await post(`/api/sites/${name}/settings`, body);
      setMsg({ tone: "ok", text: "Tersimpan. nginx diuji lalu dimuat ulang; perubahan aktif dalam sekitar 1 detik." });
      setTimeout(onSaved, 700);
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const addRule = () => set("rewrites", [...m.rewrites, { from: "", to: "", flag: "" }]);
  const editRule = (i: number, k: keyof Rewrite, v: string) => set("rewrites", m.rewrites.map((r, j) => (j === i ? { ...r, [k]: v } : r)));
  const dropRule = (i: number) => set("rewrites", m.rewrites.filter((_, j) => j !== i));

  return (
    <form onSubmit={save} className="stack-l">
      <p className="small muted">
        Status config: <Chip tone={kind === "custom" ? "warn" : "info"}>{kindLabel[kind] ?? kind}</Chip>
        {kind === "legacy" && " Menyimpan akan menggantikan config dengan template panel; versi lama dicadangkan."}
      </p>
      {kind === "custom" && !system && (
        <label className="check">
          <input type="checkbox" checked={replace} onChange={(e) => setReplace(e.target.checked)} />
          Timpa config yang diedit manual (versi lama tetap tersimpan di tab Config nginx)
        </label>
      )}
      <fieldset className="fs" disabled={system || busy}>
        {section === "domain" ? (
          <Card title="Domain tambahan">
            <p className="small muted">
              Satu situs bisa dijangkau lewat beberapa nama, mis. <span className="mono">www.lab</span>. Satu nama per baris, berbentuk <span className="mono">nama.lab</span> (maks 10). Sertifikat HTTPS dibuat ulang otomatis agar memuat semua nama. Nama situs inti dan nama yang sudah dipakai situs lain ditolak.
            </p>
            <Field label="Domain tambahan" hint={`Nama utama tetap ${name}.lab`}>
              {(id, dd) => (
                <textarea id={id} aria-describedby={dd} className="mono" rows={4} value={(m.aliases ?? []).join("\n")} onChange={(e) => set("aliases", e.target.value.split("\n").map((x) => x.trim().toLowerCase()).filter(Boolean))} spellCheck={false} />
              )}
            </Field>
          </Card>
        ) : section === "akses" ? (
          <Card title="Pembatasan akses">
            <p className="small muted">Akses ke semua situs sudah dibatasi ke jaringan tailnet. Di sini Anda bisa mempersempitnya lagi. Aturan hanya boleh berisi alamat di dalam tailnet atau loopback; alamat publik ditolak.</p>
            <Field label="Hanya izinkan IP berikut (opsional)" hint="Satu IP atau CIDR per baris, mis. 100.110.21.93 atau 100.64.0.0/16. Kosong = semua perangkat tailnet.">
              {(id, dd) => (
                <textarea id={id} aria-describedby={dd} className="mono" rows={3} value={(m.allow_ips ?? []).join("\n")} onChange={(e) => set("allow_ips", e.target.value.split("\n").map((x) => x.trim()).filter(Boolean))} spellCheck={false} />
              )}
            </Field>
            <div className="checks">
              <label className="check">
                <input type="checkbox" checked={!!m.auth} onChange={(e) => set("auth", e.target.checked)} />
                Wajib password dasar (HTTP Basic Auth). Tambahkan pengguna di bawah sebelum mengaktifkan.
              </label>
            </div>
          </Card>
        ) : section === "umum" ? (
          <Card title="Pengaturan umum">
            <div className="grid2">
              <Field label="Jenis situs">
                {(id) => (
                  <select id={id} value={m.mode} onChange={(e) => setM({ ...m, mode: e.target.value, preset: e.target.value === "php" ? "laravel" : e.target.value === "statis" ? "static" : "", php: e.target.value === "php" ? m.php || "8.4" : "" })}>
                    <option value="php">PHP</option>
                    <option value="statis">Statis</option>
                    <option value="proxy">Proxy</option>
                  </select>
                )}
              </Field>
              <Field label={m.mode === "proxy" ? "Port aplikasi" : "Folder"}>
                {(id) => <input id={id} className="mono" value={m.target} onChange={(e) => set("target", e.target.value)} required />}
              </Field>
              {m.mode === "php" && (
                <Field label="Versi PHP" hint="Ganti versi tanpa membuat ulang situs. Versi harus terpasang.">
                  {(id, d) => (
                    <select id={id} aria-describedby={d} value={m.php} onChange={(e) => set("php", e.target.value)}>
                      {PHP_VERSIONS.map((v) => <option key={v}>{v}</option>)}
                    </select>
                  )}
                </Field>
              )}
              {presets.length > 0 && (
                <Field label="Preset routing" hint={m.mode === "php" ? "Laravel: hanya index.php yang dijalankan. WordPress: semua file .php." : "Statis: rute tak dikenal 404. SPA: dialihkan ke index.html."}>
                  {(id, d) => (
                    <select id={id} aria-describedby={d} value={m.preset} onChange={(e) => set("preset", e.target.value)}>
                      {presets.map((p) => <option key={p}>{p}</option>)}
                    </select>
                  )}
                </Field>
              )}
              <Field label="Batas unggah (MB)">{(id) => <input id={id} type="number" min={1} max={1024} value={m.max_body_mb} onChange={(e) => set("max_body_mb", Number(e.target.value))} required />}</Field>
              <Field label="Timeout baca (detik)">{(id) => <input id={id} type="number" min={10} max={600} value={m.read_timeout_s} onChange={(e) => set("read_timeout_s", Number(e.target.value))} required />}</Field>
            </div>
            <div className="checks">
              <label className="check"><input type="checkbox" checked={m.force_https} onChange={(e) => set("force_https", e.target.checked)} />Paksa HTTPS (alihkan port 80 ke 443)</label>
              <label className="check"><input type="checkbox" checked={m.gzip} onChange={(e) => set("gzip", e.target.checked)} />Kompres respons (gzip)</label>
            </div>
          </Card>
        ) : (
          <Card
            title="Aturan rewrite"
            action={
              <button type="button" className="btn" onClick={addRule} disabled={m.rewrites.length >= 50}>
                <Plus size={18} aria-hidden="true" />
                Tambah aturan
              </button>
            }
          >
            <p className="small muted">Dijalankan berurutan sebelum routing preset. Pola regex tanpa spasi; tujuan diawali / atau http(s)://. Contoh: dari <span className="mono">^/lama/(.*)$</span> ke <span className="mono">/baru/$1</span> dengan flag permanent.</p>
            {m.rewrites.length === 0 ? (
              <div className="state"><p className="muted">Belum ada aturan rewrite.</p></div>
            ) : (
              <div className="tbl-wrap">
                <table className="tbl">
                  <thead>
                    <tr>
                      <th scope="col">Dari (regex)</th>
                      <th scope="col">Ke</th>
                      <th scope="col">Flag</th>
                      <th scope="col"><span className="sr">Hapus</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    {m.rewrites.map((r, i) => (
                      <tr key={i}>
                        <td><input aria-label={`Aturan ${i + 1}: dari`} className="mono" value={r.from} onChange={(e) => editRule(i, "from", e.target.value)} required /></td>
                        <td><input aria-label={`Aturan ${i + 1}: ke`} className="mono" value={r.to} onChange={(e) => editRule(i, "to", e.target.value)} required /></td>
                        <td>
                          <select aria-label={`Aturan ${i + 1}: flag`} value={r.flag} onChange={(e) => editRule(i, "flag", e.target.value)}>
                            <option value="">(tanpa)</option>
                            {["last", "break", "redirect", "permanent"].map((f) => <option key={f}>{f}</option>)}
                          </select>
                        </td>
                        <td className="cell-end">
                          <button type="button" className="btn btn-ghost btn-icon" onClick={() => dropRule(i)} aria-label={`Hapus aturan ${i + 1}`}>
                            <Trash2 size={18} aria-hidden="true" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        )}
      </fieldset>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      {!system && (
        <div>
          <button className="btn btn-primary" disabled={busy || (kind === "custom" && !replace)}>{busy ? "Menyimpan…" : "Simpan dan terapkan"}</button>
        </div>
      )}
    </form>
  );
}

function Ssl({ name }: { name: string }) {
  const { data, error, loading, reload } = useLoad(() => get<any>(`/api/sites/${name}/ssl`), [name]);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const renew = async () => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/ssl/renew`, {});
      setMsg({ tone: "ok", text: "Sertifikat baru diterbitkan dan nginx dimuat ulang." });
      reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Sertifikat HTTPS">
      <State loading={loading && !data} error={error} onRetry={reload}>
        {data && !data.exists && <p className="muted">Belum ada sertifikat untuk {data.host}.</p>}
        {data?.exists && (
          <dl className="kv">
            <div><dt>Domain</dt><dd className="mono">{(data.dns as string[]).join(", ")}</dd></div>
            <div><dt>Penerbit</dt><dd>{data.issuer}</dd></div>
            <div>
              <dt>Berlaku sampai</dt>
              <dd>{fmtTime(data.not_after)} <Chip tone={data.expired ? "bad" : data.days_left < 30 ? "warn" : "ok"}>{data.expired ? "Kedaluwarsa" : `${data.days_left} hari lagi`}</Chip></dd>
            </div>
          </dl>
        )}
        {data?.note && <p className="small muted">{data.note}</p>}
      </State>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      <div>
        <button className="btn" onClick={renew} disabled={busy}>{busy ? "Memperbarui…" : "Terbitkan ulang sertifikat"}</button>
      </div>
    </Card>
  );
}

function ConfigEditor({ name, system, onSaved }: { name: string; system: boolean; onSaved: () => void }) {
  const { data, error, loading, reload } = useLoad(() => get<{ content: string; sha: string; versions: { id: number; at: number; user: string; note: string }[] }>(`/api/sites/${name}/config`), [name]);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  useEffect(() => {
    if (data) setText(data.content);
  }, [data]);
  const save = async () => {
    if (!data) return;
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/config`, { content: text, sha: data.sha });
      setMsg({ tone: "ok", text: "Config diterapkan dan nginx dimuat ulang." });
      reload();
      onSaved();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const restore = async (id: number) => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/restore`, { id });
      setMsg({ tone: "ok", text: `Versi #${id} dipulihkan.` });
      reload();
      onSaved();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="stack-l">
      <Card title="Config nginx">
        <State loading={loading && !data} error={error} onRetry={reload}>
          <p className="small muted">
            Editor lanjutan. Hanya direktif yang diizinkan diterima, dan setiap blok server wajib mempertahankan <span className="mono">include .../tailnet_only.conf</span>. Config diuji dengan <span className="mono">nginx -t</span>; bila gagal, config lama dikembalikan otomatis.
          </p>
          <label className="sr" htmlFor="conf-text">Isi config nginx</label>
          <textarea id="conf-text" className="mono editor" value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} readOnly={system} rows={22} />
          {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
          {!system && (
            <div>
              <button className="btn btn-primary" onClick={save} disabled={busy || !data || text === data.content}>{busy ? "Menerapkan…" : "Simpan dan terapkan"}</button>
            </div>
          )}
        </State>
      </Card>
      {data && (
        <Card title="Riwayat versi">
          {data.versions.length === 0 ? (
            <div className="state"><p className="muted">Belum ada versi tersimpan. Versi dibuat setiap config diubah lewat panel.</p></div>
          ) : (
            <ul className="list">
              {data.versions.map((v) => (
                <li key={v.id} className="list-row">
                  <div>
                    <p><span className="mono">#{v.id}</span> {v.note}</p>
                    <p className="small muted">{fmtTime(v.at)} oleh {v.user}</p>
                  </div>
                  {!system && (
                    <button className="btn" onClick={() => restore(v.id)} disabled={busy}>Pulihkan<span className="sr"> versi {v.id}</span></button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </Card>
      )}
    </div>
  );
}

function Logs({ name }: { name: string }) {
  const [kind, setKind] = useState("access");
  const { data, error, loading, reload } = useLoad(() => get<{ lines: string[]; note: string }>(`/api/sites/${name}/logs?kind=${kind}&n=100`), [name, kind]);
  return (
    <Card
      title="Log"
      action={
        <div className="row">
          <select aria-label="Jenis log" value={kind} onChange={(e) => setKind(e.target.value)} style={{ width: "auto" }}>
            <option value="access">Akses</option>
            <option value="error">Error</option>
          </select>
          <button className="btn" onClick={reload}>Muat ulang</button>
        </div>
      }
    >
      <State loading={loading && !data} error={error} empty={!!data && data.lines.length === 0} emptyText={kind === "access" ? "Belum ada log akses. Log per situs aktif setelah pengaturan disimpan lewat panel." : "Tidak ada error untuk situs ini."} onRetry={reload}>
        <pre className="logbox mono">{data?.lines.join("\n")}</pre>
      </State>
    </Card>
  );
}


function AuthUsers({ name, enabled }: { name: string; enabled: boolean }) {
  const q = useLoad(() => get<{ users: string[] }>(`/api/sites/${name}/auth`), [name]);
  const [user, setUser] = useState("");
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [rm, setRm] = useState<string | null>(null);
  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await post(`/api/sites/${name}/auth/users`, { user, password: pw });
      setUser("");
      setPw("");
      q.reload();
    } catch (x) {
      setErr(errText(x));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Pengguna password dasar">
      <p className="small muted">{enabled ? "Password dasar aktif untuk situs ini." : "Password dasar belum aktif; aktifkan di kartu di atas setelah menambah pengguna."} Kata sandi disimpan sebagai hash dan tidak bisa dilihat lagi. Menambah dengan nama yang sudah ada mengganti kata sandinya.</p>
      <State loading={q.loading && !q.data} error={q.error} empty={!!q.data && q.data.users.length === 0} emptyText="Belum ada pengguna." onRetry={q.reload}>
        <ul className="list">
          {q.data?.users.map((u) => (
            <li key={u} className="list-row">
              <span className="mono">{u}</span>
              <button className="btn btn-danger-ghost" onClick={() => setRm(u)}>
                <Trash2 size={18} aria-hidden="true" />
                Hapus<span className="sr"> {u}</span>
              </button>
            </li>
          ))}
        </ul>
      </State>
      <form onSubmit={add} className="stack">
        <div className="grid2">
          <Field label="Nama pengguna">{(id) => <input id={id} className="mono" value={user} onChange={(e) => setUser(e.target.value)} autoComplete="off" required pattern="[A-Za-z0-9._\-]{1,32}" />}</Field>
          <Field label="Kata sandi" hint="8-128 karakter">{(id, dd) => <input id={id} aria-describedby={dd} type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="new-password" minLength={8} maxLength={128} required />}</Field>
        </div>
        {err && <Alert>{err}</Alert>}
        <div>
          <button className="btn" disabled={busy || !user || pw.length < 8}>{busy ? "Menyimpan…" : "Tambah atau ganti"}</button>
        </div>
      </form>
      {rm && (
        <ConfirmDialog
          title={`Hapus pengguna ${rm}`}
          word={rm}
          verb="Hapus pengguna"
          body={<p>Pengguna ini tidak bisa masuk lagi. Meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            await post(`/api/sites/${name}/auth/users/${rm}/remove`, {});
            q.reload();
          }}
          onClose={() => setRm(null)}
        />
      )}
    </Card>
  );
}

function EnvEditor({ name }: { name: string }) {
  const [state, setState] = useState<"closed" | "loading" | "open">("closed");
  const [content, setContent] = useState("");
  const [path, setPath] = useState("");
  const [exists, setExists] = useState(false);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const open = async () => {
    setState("loading");
    setMsg(null);
    try {
      const r = await get<{ content: string; exists: boolean; path: string }>(`/api/sites/${name}/env`);
      setContent(r.content);
      setExists(r.exists);
      setPath(r.path);
      setState("open");
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
      setState("closed");
    }
  };
  const save = async () => {
    setBusy(true);
    setMsg(null);
    try {
      await post(`/api/sites/${name}/env`, { content });
      setExists(true);
      setMsg({ tone: "ok", text: "Tersimpan. Cadangan sebelumnya disimpan sebagai .env.bak.<waktu> (3 terakhir). Aplikasi yang menyimpan config di cache (mis. php artisan config:cache) perlu di-refresh di tab Deploy." });
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Berkas .env proyek">
      <p className="small muted">Berisi rahasia (kata sandi database, kunci aplikasi). Membukanya meminta kata sandi ulang dan dicatat di audit; isinya tidak pernah dicatat. Akun hanya-baca tidak bisa membukanya. Berkas disimpan dengan izin 0600.</p>
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      {state !== "open" ? (
        <div>
          <button className="btn" onClick={open} disabled={state === "loading"}>{state === "loading" ? "Membuka…" : "Buka .env"}</button>
        </div>
      ) : (
        <div className="stack">
          <p className="small mono">{path}{exists ? "" : " (belum ada, akan dibuat)"}</p>
          <label className="sr" htmlFor="env-text">Isi .env</label>
          <textarea id="env-text" className="mono env-text" rows={16} value={content} onChange={(e) => setContent(e.target.value)} spellCheck={false} autoComplete="off" />
          <div className="row-end">
            <button className="btn" onClick={() => { setState("closed"); setContent(""); }}>Tutup</button>
            <button className="btn btn-primary" onClick={save} disabled={busy || content.length > 65536}>{busy ? "Menyimpan…" : "Simpan"}</button>
          </div>
        </div>
      )}
    </Card>
  );
}

interface Stats {
  exists: boolean;
  requests: number;
  bytes: number;
  days: { day: string; n: number }[];
  status: Record<string, number>;
  paths: { path: string; n: number }[];
  ips: { ip: string; n: number }[];
  note: string;
}

function Traffic({ name }: { name: string }) {
  const q = useLoad(() => get<Stats>(`/api/sites/${name}/stats`), [name]);
  const d = q.data;
  const max = Math.max(1, ...(d?.days?.map((x) => x.n) ?? [1]));
  return (
    <Card title="Statistik trafik" action={<button className="btn" onClick={q.reload}>Segarkan</button>}>
      <State loading={q.loading && !q.data} error={q.error} empty={!!d && !d.exists} emptyText="Belum ada log akses untuk situs ini." onRetry={q.reload}>
        {d?.exists && (
          <div className="stack">
            <p className="small muted">Dihitung dari log akses nginx ({d.note}). Bukan analitik pengunjung: hanya permintaan HTTP.</p>
            <dl className="kpis">
              <div><dt>Permintaan</dt><dd>{d.requests.toLocaleString("id-ID")}</dd></div>
              <div><dt>Data terkirim</dt><dd>{fmtBytes(d.bytes)}</dd></div>
              {["2xx", "3xx", "4xx", "5xx"].map((c) => <div key={c}><dt>{c}</dt><dd>{((d.status ?? {})[c] ?? 0).toLocaleString("id-ID")}</dd></div>)}
            </dl>
            <h3 className="sec-title">Permintaan per hari</h3>
            <ul className="bars">
              {(d.days ?? []).map((x) => (
                <li key={x.day} className="bar-row">
                  <span className="mono bar-name">{x.day}</span>
                  <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (x.n / max) * 100)}%` }} /></span>
                  <span className="mono bar-val">{x.n}</span>
                </li>
              ))}
            </ul>
            <div className="grid2">
              <div><h3 className="sec-title">Halaman teratas</h3><ul className="list">{(d.paths ?? []).map((x) => <li key={x.path} className="list-row"><span className="mono ellip">{x.path}</span><span className="mono">{x.n}</span></li>)}</ul></div>
              <div><h3 className="sec-title">IP teratas</h3><ul className="list">{(d.ips ?? []).map((x) => <li key={x.ip} className="list-row"><span className="mono">{x.ip}</span><span className="mono">{x.n}</span></li>)}</ul></div>
            </div>
          </div>
        )}
      </State>
    </Card>
  );
}
