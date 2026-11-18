import { useState } from "react";
import { get } from "../../api";
import { Card, State, useLoad } from "../../ui";

export function Logs({ name }: { name: string }) {
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
