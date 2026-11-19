import { useState } from "react";
import { CalendarClock, Code2, Globe, Link2, Terminal } from "lucide-react";
import { errText, post } from "../../api";
import { Alert, Field, Modal } from "../../ui";
import { DAYS, DEFAULT_PICK, describe, MODES, nextRuns, toBody, type Mode, type Pick } from "./schedule";

// Ikon tipe: Terminal = skrip shell, Code2 = bahasa pemrograman, Link2 = URL; dipakai di tabel dan form agar tipe terbaca sekilas.
export type TaskType = "shell" | "php" | "node" | "python" | "url";

export const TYPES: { id: TaskType; label: string; hint: string; worker: boolean }[] = [
  { id: "shell", label: "Shell (skrip)", hint: "Beberapa baris perintah zsh.", worker: true },
  { id: "php", label: "PHP", hint: "php atau composer; mis. php artisan schedule:run.", worker: true },
  { id: "node", label: "Node", hint: "node, npm run, atau pnpm run.", worker: true },
  { id: "python", label: "Python", hint: "python3 berkas.py.", worker: true },
  { id: "url", label: "Kunjungi URL", hint: "Meminta alamat http(s) lewat curl, mis. endpoint schedule atau webhook.", worker: false },
];

export const TypeIcon = ({ t, size = 16 }: { t: string; size?: number }) => {
  const I = t === "shell" ? Terminal : t === "url" ? Link2 : t === "php" || t === "node" || t === "python" ? Code2 : Globe;
  return <I size={size} className="icon" aria-hidden="true" />;
};

const DEFAULT_CMD: Record<string, string> = { php: "php artisan schedule:run", node: "npm run start", python: "python3 worker.py", shell: "", url: "" };
const MAX_SCRIPT = 20000;
const fmt = new Intl.DateTimeFormat("id-ID", { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

export function SchedulePicker({ pick, set }: { pick: Pick; set: (p: Pick) => void }) {
  const up = (k: keyof Pick, v: Pick[keyof Pick]) => set({ ...pick, [k]: v });
  const r = toBody(pick);
  const spec = "error" in r ? "" : (r.body.every ? `every ${r.body.every}` : r.body.at ? (() => { const [h, m] = r.body.at!.split(":"); return `${Number(m)} ${Number(h)} * * *`; })() : r.body.cron ?? "");
  const next = spec && !spec.startsWith("every") ? nextRuns(spec) : [];
  return (
    <fieldset className="stack sched">
      <legend className="field-label"><CalendarClock size={16} className="icon" aria-hidden="true" /> Jadwal</legend>
      <div className="grid2">
        <Field label="Pola">
          {(id) => (
            <select id={id} value={pick.mode} onChange={(e) => up("mode", e.target.value as Mode)}>
              {MODES.map((m) => <option key={m.id} value={m.id}>{m.label}</option>)}
            </select>
          )}
        </Field>
        {(pick.mode === "minutes" || pick.mode === "hours") && (
          <Field label={pick.mode === "minutes" ? "Setiap berapa menit" : "Setiap berapa jam"} hint={pick.mode === "minutes" ? "1 sampai 59." : "1 sampai 999."}>
            {(id, d) => <input id={id} aria-describedby={d} className="mono" inputMode="numeric" placeholder="5" value={pick.n} onChange={(e) => up("n", e.target.value)} required />}
          </Field>
        )}
        {pick.mode === "hourly" && (
          <Field label="Pada menit ke-" hint="0 sampai 59.">
            {(id, d) => <input id={id} aria-describedby={d} className="mono" inputMode="numeric" placeholder="0" value={pick.minute} onChange={(e) => up("minute", e.target.value)} required />}
          </Field>
        )}
        {(pick.mode === "daily" || pick.mode === "weekly" || pick.mode === "monthly") && (
          <Field label="Jam (24 jam)" hint="Zona waktu Mac mini.">
            {(id, d) => <input id={id} aria-describedby={d} type="time" value={pick.time} onChange={(e) => up("time", e.target.value)} required />}
          </Field>
        )}
        {pick.mode === "monthly" && (
          <Field label="Tanggal" hint="Bulan yang tidak punya tanggal itu dilewati.">
            {(id, d) => <input id={id} aria-describedby={d} className="mono" inputMode="numeric" placeholder="1" value={pick.dom} onChange={(e) => up("dom", e.target.value)} required />}
          </Field>
        )}
      </div>
      {pick.mode === "weekly" && (
        <div className="stack">
          <span className="field-label" id="days-label">Hari</span>
          <div className="row" role="group" aria-labelledby="days-label">
            {DAYS.map((d, i) => (
              <button key={d} type="button" className={`chip-btn ${pick.days.includes(i) ? "on" : ""}`} aria-pressed={pick.days.includes(i)} onClick={() => up("days", pick.days.includes(i) ? pick.days.filter((x) => x !== i) : [...pick.days, i])}>
                {d}
              </button>
            ))}
          </div>
        </div>
      )}
      {pick.mode === "custom" && (
        <div className="stack">
          <Field label="Ekspresi cron" hint="menit jam tanggal bulan hari. Dukung * , - / dan nama singkat (mon-fri, jan). Alias: @hourly @daily @weekly @monthly @reboot.">
            {(id, d) => <input id={id} aria-describedby={d} className="mono" placeholder="*/15 8-17 * * 1-5" value={pick.cron} onChange={(e) => up("cron", e.target.value)} required />}
          </Field>
          <div className="row">
            {[["Tiap menit", "* * * * *"], ["Tiap 5 menit", "*/5 * * * *"], ["Tiap 30 menit", "*/30 * * * *"], ["Tiap jam", "@hourly"], ["Tiap hari 00:00", "@daily"], ["Sen–Jum 08:00", "0 8 * * 1-5"], ["Tiap minggu", "@weekly"], ["Tiap bulan", "@monthly"]].map(([l, v]) => (
              <button key={v} type="button" className="btn btn-sm" onClick={() => up("cron", v)}>{l}</button>
            ))}
          </div>
        </div>
      )}
      <p className={`small ${"error" in r ? "field-err" : "muted"}`} role="status" aria-live="polite">
        {"error" in r ? r.error : <><strong>{describe(spec)}</strong>{next.length > 0 && <> · berikutnya: {next.map((d) => fmt.format(d)).join("; ")}</>}</>}
      </p>
    </fieldset>
  );
}

export default function AddTask({ kind, onClose, onDone }: { kind: "worker" | "job"; onClose: () => void; onDone: () => void }) {
  const [f, setF] = useState({ name: "", dir: "", type: "shell" as TaskType, cmd: "", script: "", url: "", php: "", procs: "1" });
  const [pick, setPick] = useState<Pick>(DEFAULT_PICK);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const up = (k: keyof typeof f, v: string) => setF((p) => ({ ...p, [k]: v }));
  const types = TYPES.filter((t) => kind === "job" || t.worker);
  const setType = (t: TaskType) => setF((p) => ({ ...p, type: t, cmd: p.cmd === "" || Object.values(DEFAULT_CMD).includes(p.cmd) ? DEFAULT_CMD[t] : p.cmd }));
  const sched = kind === "job" ? toBody(pick) : { body: {} };
  const bytes = new TextEncoder().encode(f.script).length;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if ("error" in sched) return setErr(sched.error);
    setBusy(true);
    setErr("");
    try {
      const body: Record<string, unknown> = { name: f.name, type: f.type };
      if (f.type !== "url" || f.dir) body.dir = f.dir;
      if (f.type === "shell") body.script = f.script;
      else if (f.type === "url") body.url = f.url;
      else body.cmd = f.cmd;
      if (f.php && f.type !== "url") body.php = f.php;
      if (kind === "worker") body.procs = Number(f.procs);
      else Object.assign(body, sched.body);
      await post(`/api/tasks/${kind}`, body);
      onDone();
    } catch (x) {
      setErr(errText(x));
      setBusy(false);
    }
  };
  const ready = f.name && (f.type === "url" ? f.url : f.dir && (f.type === "shell" ? f.script.trim() : f.cmd));

  return (
    <Modal title={kind === "worker" ? "Tambah worker" : "Tambah cron job"} onClose={onClose} wide>
      <form onSubmit={submit} className="stack">
        <div className="grid2">
          <Field label="Nama" hint="Huruf kecil, angka, tanda hubung.">
            {(id, d) => <input id={id} aria-describedby={d} data-autofocus className="mono" placeholder="queue-default" maxLength={40} value={f.name} onChange={(e) => up("name", e.target.value.toLowerCase())} required pattern="[a-z][a-z0-9\-]{0,39}" />}
          </Field>
          <Field label="Tipe" hint={types.find((t) => t.id === f.type)?.hint}>
            {(id, d) => (
              <select id={id} aria-describedby={d} value={f.type} onChange={(e) => setType(e.target.value as TaskType)}>
                {types.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
              </select>
            )}
          </Field>
        </div>
        <div className="grid2">
          <Field label={f.type === "url" ? "Folder kerja (opsional)" : "Folder kerja"} hint="Di bawah ~/MacPanel/wwwroot.">
            {(id, d) => <input id={id} aria-describedby={d} className="mono" value={f.dir} onChange={(e) => up("dir", e.target.value)} required={f.type !== "url"} placeholder="~/MacPanel/wwwroot/org/proyek" />}
          </Field>
          {f.type !== "url" && (
            <Field label="Versi PHP (opsional)" hint="Perintah php di skrip atau perintah memakai biner versi ini.">
              {(id, d) => (
                <select id={id} aria-describedby={d} value={f.php} onChange={(e) => up("php", e.target.value)}>
                  <option value="">(bawaan)</option>
                  {["8.3", "8.4", "8.5"].map((v) => <option key={v}>{v}</option>)}
                </select>
              )}
            </Field>
          )}
        </div>
        {f.type === "shell" && (
          <Field label="Skrip" hint={`Dijalankan zsh di folder kerja, sebagai pengguna Anda (${bytes} dari ${MAX_SCRIPT} byte). Disimpan di ~/MacPanel/server/tasks, hanya bisa dibaca Anda.`}>
            {(id, d) => <textarea id={id} aria-describedby={d} className="mono script-box" rows={9} spellCheck={false} maxLength={MAX_SCRIPT} placeholder={"cd ~/MacPanel/wwwroot/org/proyek\nphp artisan schedule:run >> storage/logs/cron.log 2>&1"} value={f.script} onChange={(e) => up("script", e.target.value)} required />}
          </Field>
        )}
        {(f.type === "php" || f.type === "node" || f.type === "python") && (
          <Field label="Perintah" hint="Tanpa kutip, pipa, atau $; kode sebaris seperti php -r ditolak. Perlu pipa atau pengalihan? Pakai tipe Shell.">
            {(id, d) => <input id={id} aria-describedby={d} className="mono" placeholder={DEFAULT_CMD[f.type]} value={f.cmd} onChange={(e) => up("cmd", e.target.value)} required />}
          </Field>
        )}
        {f.type === "url" && (
          <Field label="Alamat URL" hint="http:// atau https://, tanpa spasi atau kutip.">
            {(id, d) => <input id={id} aria-describedby={d} className="mono" type="url" placeholder="https://situs.lab/schedule/run" value={f.url} onChange={(e) => up("url", e.target.value)} required />}
          </Field>
        )}
        {kind === "worker" ? (
          <Field label="Jumlah proses">{(id) => <input id={id} type="number" min={1} max={8} placeholder="1" required value={f.procs} onChange={(e) => up("procs", e.target.value)} />}</Field>
        ) : (
          <SchedulePicker pick={pick} set={setPick} />
        )}
        {f.type === "shell" && <Alert tone="warn">Skrip shell bebas: apa pun yang ditulis dijalankan sebagai pengguna Anda di Mac mini, tanpa pembatasan program. Hanya tempel skrip yang Anda pahami.</Alert>}
        {err && <Alert>{err}</Alert>}
        <p className="small muted">Aksi ini meminta kata sandi ulang karena menjalankan kode terus-menerus. Tercatat di Aktivitas.</p>
        <div className="row-end">
          <button type="button" className="btn" onClick={onClose}>Batal</button>
          <button className="btn btn-primary" disabled={busy || !ready}>{busy ? "Membuat…" : "Buat"}</button>
        </div>
      </form>
    </Modal>
  );
}
