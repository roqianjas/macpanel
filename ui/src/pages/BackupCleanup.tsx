import { useState } from "react";
import { errText, get, post } from "../api";
import { fmtBytes } from "../lib";
import { Alert, Card, ConfirmDialog, Field, State, useLoad } from "../ui";

interface Plan {
  keep: number;
  days: number;
  remove: { name: string; reason: string; kb: number }[];
  kept: number;
}

export function CleanupCard({ onChanged }: { onChanged: () => void }) {
  const cfg = useLoad(() => get<{ keep: number; days: number }>("/api/backups/config"));
  const [keep, setKeep] = useState("");
  const [days, setDays] = useState("");
  const k = keep || String(cfg.data?.keep ?? 7);
  const dd = days || String(cfg.data?.days ?? 0);
  const [plan, setPlan] = useState<Plan | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const preview = async () => {
    setBusy(true);
    setMsg(null);
    try {
      setPlan(await get<Plan>(`/api/backups/prune-preview?keep=${encodeURIComponent(k)}&days=${encodeURIComponent(dd)}`));
    } catch (x) {
      setPlan(null);
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  const total = (plan?.remove ?? []).reduce((a, r) => a + r.kb * 1024, 0);
  return (
    <Card title="Bersihkan backup lama">
      <State loading={cfg.loading && !cfg.data} error={cfg.error} onRetry={cfg.reload}>
        <p className="small muted">Hapus backup yang melebihi jumlah simpanan atau lebih tua dari batas umur. Backup terbaru tidak pernah dihapus. Lihat dulu daftar yang akan dihapus sebelum menjalankan. Pengaturan otomatis ada di kartu Jadwal; backup harian menjalankan pembersihan yang sama.</p>
        <div className="grid2">
          <Field label="Simpan backup terakhir" hint="1-60">{(id, h) => <input id={id} aria-describedby={h} type="number" min={1} max={60} value={k} onChange={(e) => { setKeep(e.target.value); setPlan(null); }} />}</Field>
          <Field label="Hapus yang lebih tua dari (hari)" hint="0 = tanpa batas umur">{(id, h) => <input id={id} aria-describedby={h} type="number" min={0} max={3650} value={dd} onChange={(e) => { setDays(e.target.value); setPlan(null); }} />}</Field>
        </div>
        {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
        <div className="row-end">
          <button className="btn" onClick={preview} disabled={busy}>{busy ? "Menghitung…" : "Lihat yang akan dihapus"}</button>
        </div>
        {plan && (
          plan.remove.length === 0 ? (
            <p className="muted">Tidak ada yang perlu dihapus. {plan.kept} backup tersimpan.</p>
          ) : (
            <div className="stack">
              <ul className="list">
                {plan.remove.map((r) => (
                  <li key={r.name} className="list-row"><div><p className="mono">{r.name}</p><p className="small muted">{r.reason}</p></div><span className="mono">{fmtBytes(r.kb * 1024)}</span></li>
                ))}
              </ul>
              <div className="row-end">
                <span className="small muted">{plan.remove.length} backup, {fmtBytes(total)} akan dibebaskan; {plan.kept} tersisa.</span>
                <button className="btn btn-danger-ghost" onClick={() => setConfirm(true)}>Hapus {plan.remove.length} backup</button>
              </div>
            </div>
          )
        )}
      </State>
      {confirm && plan && (
        <ConfirmDialog
          title="Hapus backup lama"
          word="hapus"
          verb={`Hapus ${plan.remove.length} backup`}
          body={<p>{plan.remove.length} backup ({fmtBytes(total)}) dihapus permanen dari disk ini. Backup terbaru tetap ada. Meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            await post("/api/backups/prune", { keep: Number(k), days: Number(dd) });
            setPlan(null);
            setMsg({ tone: "ok", text: "Backup lama dihapus." });
            onChanged();
          }}
          onClose={() => setConfirm(false)}
        />
      )}
    </Card>
  );
}

interface Extras {
  db: { count: number; kb: number; oldest: number };
  restore: { count: number; kb: number; oldest: number };
  retired: { kb: number; note: string };
}

export function ExtrasCard() {
  const q = useLoad(() => get<Extras>("/api/backups/extras"));
  return (
    <Card title="Dump database dan hasil ekstrak">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        {q.data && (
          <div className="stack">
            <p className="small muted">Berkas ini di luar backup harian: dump terakhir yang dibuat saat database dihapus atau di-dump manual, dan hasil ekstrak dari penjelajah backup. Hanya berkas yang dibuat panel yang dihapus; berkas lain di folder yang sama tidak disentuh.</p>
            <Row kind="db" title="Dump database" info={q.data.db} onDone={q.reload} />
            <Row kind="restore" title="Hasil ekstrak backup" info={q.data.restore} onDone={q.reload} />
            <div className="list-row"><div><p>Data lama hasil migrasi</p><p className="small muted">{q.data.retired.note}</p></div><span className="mono">{fmtBytes(q.data.retired.kb * 1024)}</span></div>
          </div>
        )}
      </State>
    </Card>
  );
}

function Row({ kind, title, info, onDone }: { kind: "db" | "restore"; title: string; info: { count: number; kb: number }; onDone: () => void }) {
  const [days, setDays] = useState("30");
  const [pv, setPv] = useState<{ count: number; kb: number } | null>(null);
  const [confirm, setConfirm] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const check = async () => {
    setMsg(null);
    try {
      setPv(await get(`/api/backups/extras-preview?kind=${kind}&days=${encodeURIComponent(days)}`));
    } catch (x) {
      setPv(null);
      setMsg({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <div className="stack">
      <div className="list-row"><div><p>{title}</p><p className="small muted">{info.count} item</p></div><span className="mono">{fmtBytes(info.kb * 1024)}</span></div>
      {info.count > 0 && (
        <div className="row-form">
          <Field label="Hapus yang lebih tua dari (hari)" hint="0 = semua">{(id, h) => <input id={id} aria-describedby={h} type="number" min={0} max={3650} value={days} onChange={(e) => { setDays(e.target.value); setPv(null); }} />}</Field>
          <button className="btn" onClick={check}>Hitung</button>
          {pv && pv.count > 0 && <button className="btn btn-danger-ghost" onClick={() => setConfirm(true)}>Hapus {pv.count} item ({fmtBytes(pv.kb * 1024)})</button>}
        </div>
      )}
      {pv && pv.count === 0 && <p className="small muted">Tidak ada yang cocok.</p>}
      {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
      {confirm && pv && (
        <ConfirmDialog
          title={`Hapus ${title.toLowerCase()}`}
          word="hapus"
          verb={`Hapus ${pv.count} item`}
          body={<p>{pv.count} item ({fmtBytes(pv.kb * 1024)}) dihapus permanen. Meminta kata sandi ulang.</p>}
          onConfirm={async () => {
            await post("/api/backups/extras", { kind, days: Number(days) });
            setPv(null);
            setMsg({ tone: "ok", text: "Dihapus." });
            onDone();
          }}
          onClose={() => setConfirm(false)}
        />
      )}
    </div>
  );
}
