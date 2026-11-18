import { useEffect, useState } from "react";
import { errText, fmtTime, get, post } from "../../api";
import { Alert, Card, State, useLoad } from "../../ui";

export function ConfigEditor({ name, system, onSaved }: { name: string; system: boolean; onSaved: () => void }) {
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
