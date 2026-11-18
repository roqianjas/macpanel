import { useState } from "react";
import { errText, post } from "../../api";
import { Alert, Field, Modal } from "../../ui";
import { PHP_VERSIONS } from "./types";

export function AddSite({ onClose, onDone }: { onClose: () => void; onDone: (name: string) => void }) {
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
                <option value="mariadb">MariaDB (3306)</option>
                <option value="mysql">MySQL (3307)</option>
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
