import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, useLoad } from "../ui";

interface U {
  source: string;
  name: string;
  from: string;
  to: string;
  pinned: boolean;
  major: boolean;
}

export default function Updates() {
  const { data, error, loading, reload } = useLoad(() => get<{ updates: U[]; pins: string[]; checked: number }>("/api/updates"));
  const [pick, setPick] = useState<U | null>(null);
  const [note, setNote] = useState("");
  const target = (u: U) => (u.source === "mise" ? `mise:${u.name}` : u.name);
  return (
    <>
      <PageHead
        title="Pembaruan"
        desc="Upgrade dilakukan satu per satu dengan backup terlebih dulu. Upgrade versi mayor ditolak; database diberi pin."
        action={
          <button className="btn" onClick={reload} disabled={loading}>
            <RefreshCw size={18} aria-hidden="true" className={loading ? "spin" : ""} />
            {loading ? "Memeriksa…" : "Periksa lagi"}
          </button>
        }
      />
      {note && <Alert tone="warn">{note}</Alert>}
      <Card title="Tersedia">
        <State loading={loading && !data} error={error} empty={!!data && data.updates.length === 0} emptyText="Semua paket sudah versi terbaru." onRetry={reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Paket</th>
                  <th scope="col">Sumber</th>
                  <th scope="col">Versi</th>
                  <th scope="col">Catatan</th>
                  <th scope="col">
                    <span className="sr">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {data?.updates.map((u) => (
                  <tr key={`${u.source}:${u.name}`}>
                    <td className="mono">{u.name}</td>
                    <td>{u.source === "mise" ? "mise" : "Homebrew"}</td>
                    <td className="mono">
                      {u.from} → {u.to}
                    </td>
                    <td>
                      <div className="row">
                        {u.pinned && <Chip tone="warn">Pin</Chip>}
                        {u.major && <Chip tone="bad">Mayor: butuh dump dan restore manual</Chip>}
                      </div>
                    </td>
                    <td className="cell-end">
                      <button className="btn" onClick={() => setPick(u)} disabled={u.major}>
                        Upgrade<span className="sr"> {u.name}</span>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {data && <p className="small muted">Diperiksa {fmtTime(data.checked)}. Pin aktif: {data.pins.length ? data.pins.join(", ") : "tidak ada"}.</p>}
        </State>
      </Card>
      {pick && (
        <ConfirmDialog
          title={`Upgrade ${pick.name}`}
          word={target(pick)}
          verb="Upgrade"
          body={
            <>
              <p>
                {pick.from} → {pick.to}. Backup dibuat dulu; bila backup gagal, upgrade dibatalkan.
              </p>
              {pick.pinned && <p className="alert alert-warn">Paket ini diberi pin (database). Upgrade patch tetap perlu dicek; layanan akan restart sebentar. Pin dipasang ulang setelahnya.</p>}
            </>
          }
          onConfirm={async () => {
            try {
              const r = await post<{ job: number }>("/api/updates/apply", { target: target(pick), confirm: target(pick), allow_pinned: pick.pinned });
              trackJob(r.job, `Upgrade ${pick.name}`, reload);
              setNote("");
            } catch (x) {
              throw new Error(errText(x));
            }
          }}
          onClose={() => setPick(null)}
        />
      )}
    </>
  );
}
