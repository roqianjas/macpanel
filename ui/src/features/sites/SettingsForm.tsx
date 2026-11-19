import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { errText, post } from "../../api";
import { Alert, Card, Chip, Field } from "../../ui";
import { Rewrite, Model, PHP_VERSIONS } from "./types";

export function SettingsForm({ name, kind, initial, system, section, onSaved }: { name: string; kind: string; initial: Model; system: boolean; section: string; onSaved: () => void }) {
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
                <textarea id={id} aria-describedby={dd} className="mono" rows={4} placeholder="www.contoh.lab" value={(m.aliases ?? []).join("\n")} onChange={(e) => set("aliases", e.target.value.split("\n").map((x) => x.trim().toLowerCase()).filter(Boolean))} spellCheck={false} />
              )}
            </Field>
          </Card>
        ) : section === "akses" ? (
          <Card title="Pembatasan akses">
            <p className="small muted">Akses ke semua situs sudah dibatasi ke jaringan tailnet. Di sini Anda bisa mempersempitnya lagi. Aturan hanya boleh berisi alamat di dalam tailnet atau loopback; alamat publik ditolak.</p>
            <Field label="Hanya izinkan IP berikut (opsional)" hint="Satu IP atau CIDR per baris, mis. 100.64.0.10 atau 100.64.0.0/16. Kosong = semua perangkat tailnet.">
              {(id, dd) => (
                <textarea id={id} aria-describedby={dd} className="mono" rows={3} placeholder="100.64.0.10" value={(m.allow_ips ?? []).join("\n")} onChange={(e) => set("allow_ips", e.target.value.split("\n").map((x) => x.trim()).filter(Boolean))} spellCheck={false} />
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
                {(id) => <input id={id} className="mono" placeholder={m.mode === "proxy" ? "3000" : "wwwroot/nama-situs/public"} pattern={m.mode === "proxy" ? "[0-9]{4,5}" : undefined} title={m.mode === "proxy" ? "Port 1024-65535" : undefined} value={m.target} onChange={(e) => set("target", e.target.value)} required />}
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
              <Field label="Batas unggah (MB)">{(id) => <input id={id} type="number" min={1} max={1024} placeholder="50" value={m.max_body_mb} onChange={(e) => set("max_body_mb", Number(e.target.value))} required />}</Field>
              <Field label="Timeout baca (detik)">{(id) => <input id={id} type="number" min={10} max={600} placeholder="60" value={m.read_timeout_s} onChange={(e) => set("read_timeout_s", Number(e.target.value))} required />}</Field>
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
                        <td><input aria-label={`Aturan ${i + 1}: dari`} className="mono" placeholder="^/lama/(.*)$" maxLength={200} value={r.from} onChange={(e) => editRule(i, "from", e.target.value)} required /></td>
                        <td><input aria-label={`Aturan ${i + 1}: ke`} className="mono" placeholder="/baru/$1" maxLength={200} value={r.to} onChange={(e) => editRule(i, "to", e.target.value)} required /></td>
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
