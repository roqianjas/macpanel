import { useState } from "react";
import { errText, get, post } from "../../api";
import { Alert, Card } from "../../ui";

export function EnvEditor({ name }: { name: string }) {
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
          <textarea id="env-text" className="mono env-text" rows={16} placeholder={"APP_ENV=production\nAPP_DEBUG=false"} value={content} onChange={(e) => setContent(e.target.value)} spellCheck={false} autoComplete="off" />
          <div className="row-end">
            <button className="btn" onClick={() => { setState("closed"); setContent(""); }}>Tutup</button>
            <button className="btn btn-primary" onClick={save} disabled={busy || content.length > 65536}>{busy ? "Menyimpan…" : "Simpan"}</button>
          </div>
        </div>
      )}
    </Card>
  );
}
