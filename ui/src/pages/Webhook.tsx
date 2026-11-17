import { useState } from "react";
import { Copy } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { Alert, Card, ConfirmDialog, Field, Modal, State, useLoad } from "../ui";

interface Hook {
  configured: boolean;
  enabled?: boolean;
  branch?: string;
  steps?: string[];
  last_at?: number;
  last_result?: string;
}

export default function Webhook({ name, steps }: { name: string; steps: { id: string; label: string }[] }) {
  const q = useLoad(() => get<Hook>(`/api/sites/${name}/hook`), [name]);
  const h = q.data;
  const [branch, setBranch] = useState<string | null>(null);
  const [sel, setSel] = useState<Set<string> | null>(null);
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const [secret, setSecret] = useState<string | null>(null);
  const [rotate, setRotate] = useState(false);
  const [del, setDel] = useState(false);
  const br = branch ?? h?.branch ?? "main";
  const chosen = sel ?? new Set(h?.steps ?? ["git_pull"]);
  const en = enabled ?? h?.enabled ?? true;
  const toggle = (id: string) => {
    const n = new Set(chosen);
    if (n.has(id)) n.delete(id);
    else n.add(id);
    setSel(n);
  };
  const save = async (rot: boolean) => {
    setBusy(true);
    setMsg(null);
    try {
      const r = await post<{ secret?: string }>(`/api/sites/${name}/hook`, { enabled: en, branch: br, steps: steps.map((s) => s.id).filter((i) => chosen.has(i)), rotate: rot });
      if (r.secret) setSecret(r.secret);
      else setMsg({ tone: "ok", text: "Pengaturan webhook disimpan." });
      setBranch(null);
      setSel(null);
      setEnabled(null);
      q.reload();
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Webhook GitHub (deploy otomatis)">
      <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
        <div className="stack">
          <p className="small muted">
            Setiap push ke branch yang dipilih menjalankan langkah deploy di bawah. Langkah ditentukan di sini, bukan oleh payload GitHub. Penerima webhook berjalan di <span className="mono">127.0.0.1:9101</span> dan sengaja tidak terbuka ke internet. Agar GitHub bisa menjangkaunya, Anda perlu mengekspos hanya port itu (mis. Tailscale Funnel atau Cloudflare Tunnel); itu keputusan Anda. Tanda tangan HMAC diperiksa pada setiap permintaan; yang salah dijawab 404 dan IP-nya dikunci setelah beberapa kali.
          </p>
          {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
          <div className="checks">
            <label className="check"><input type="checkbox" checked={en} onChange={(e) => setEnabled(e.target.checked)} />Aktif</label>
          </div>
          <Field label="Branch yang memicu deploy" hint="Mis. main atau production.">{(id, dd) => <input id={id} aria-describedby={dd} className="mono" value={br} onChange={(e) => setBranch(e.target.value)} spellCheck={false} maxLength={100} />}</Field>
          <fieldset className="fieldset">
            <legend className="small">Langkah deploy</legend>
            <ul className="list">
              {steps.map((s) => (
                <li key={s.id} className="list-row">
                  <label className="check"><input type="checkbox" checked={chosen.has(s.id)} onChange={() => toggle(s.id)} />{s.label}</label>
                  <span className="mono small muted">{s.id}</span>
                </li>
              ))}
            </ul>
          </fieldset>
          {h?.configured && <p className="small muted">Pemicu terakhir: {h.last_at ? `${fmtTime(h.last_at)} (${h.last_result})` : "belum pernah"}.</p>}
          <div className="row-end">
            {h?.configured && <button className="btn btn-danger-ghost" onClick={() => setDel(true)}>Hapus webhook</button>}
            {h?.configured && <button className="btn" onClick={() => setRotate(true)}>Buat secret baru</button>}
            <button className="btn btn-primary" disabled={busy || chosen.size === 0} onClick={() => save(false)}>{busy ? "Menyimpan…" : h?.configured ? "Simpan" : "Aktifkan webhook"}</button>
          </div>
        </div>
      </State>
      {secret && (
        <Modal title="Secret webhook (hanya tampil sekali)" onClose={() => setSecret(null)}>
          <div className="stack">
            <Alert tone="warn">Salin sekarang. Panel hanya menyimpan versi terenkripsi dan tidak bisa menampilkannya lagi; bila hilang, buat secret baru.</Alert>
            <p className="small">Di GitHub: Settings, Webhooks, Add webhook.</p>
            <ul className="list">
              <li className="list-row"><span>Payload URL</span><span className="mono small">https://&lt;alamat-publik-Anda&gt;/hook/{name}</span></li>
              <li className="list-row"><span>Content type</span><span className="mono">application/json</span></li>
              <li className="list-row"><span>Events</span><span className="mono">Just the push event</span></li>
            </ul>
            <p className="mono ellip-wrap" aria-label="Secret">{secret}</p>
            <div className="row-end">
              <button className="btn" onClick={() => navigator.clipboard?.writeText(secret)}><Copy size={16} aria-hidden="true" />Salin secret</button>
              <button className="btn btn-primary" onClick={() => setSecret(null)}>Sudah saya salin</button>
            </div>
          </div>
        </Modal>
      )}
      {rotate && (
        <ConfirmDialog title="Buat secret baru" word={name} verb="Buat secret baru" body={<p>Secret lama langsung tidak berlaku; perbarui webhook di GitHub. Meminta kata sandi ulang.</p>} onConfirm={async () => { await save(true); }} onClose={() => setRotate(false)} />
      )}
      {del && (
        <ConfirmDialog title="Hapus webhook" word={name} verb="Hapus webhook" body={<p>Webhook dihapus dan secret dibuang. Meminta kata sandi ulang.</p>} onConfirm={async () => { await post(`/api/sites/${name}/hook/remove`, {}); q.reload(); }} onClose={() => setDel(false)} />
      )}
    </Card>
  );
}
