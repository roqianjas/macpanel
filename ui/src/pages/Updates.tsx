import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { errText, fmtTime, get, post } from "../api";
import { trackJob } from "../jobs";
import { Alert, Card, Chip, ConfirmDialog, PageHead, State, useLoad } from "../ui";
import { useT } from "../i18n";

interface U {
  source: string;
  name: string;
  from: string;
  to: string;
  pinned: boolean;
  major: boolean;
}

export default function Updates() {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<{ updates: U[]; pins: string[]; checked: number }>("/api/updates"));
  const [pick, setPick] = useState<U | null>(null);
  const [note, setNote] = useState("");
  const target = (u: U) => (u.source === "mise" ? `mise:${u.name}` : u.name);
  return (
    <>
      <PageHead
        title={t("nav.updates")}
        desc={t("ops.up.desc")}
        action={
          <button className="btn" onClick={reload} disabled={loading}>
            <RefreshCw size={18} aria-hidden="true" className={loading ? "spin" : ""} />
            {loading ? t("ops.checking") : t("ops.up.recheck")}
          </button>
        }
      />
      {note && <Alert tone="warn">{note}</Alert>}
      <Card title={t("ops.up.available")}>
        <State loading={loading && !data} error={error} empty={!!data && data.updates.length === 0} emptyText={t("ops.up.empty")} onRetry={reload}>
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">{t("ops.col.package")}</th>
                  <th scope="col">{t("ops.col.source")}</th>
                  <th scope="col">{t("apps.col.version")}</th>
                  <th scope="col">{t("ops.col.notes")}</th>
                  <th scope="col">
                    <span className="sr">{t("ops.col.actions")}</span>
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
                        {u.pinned && <Chip tone="warn">{t("ops.up.pin")}</Chip>}
                        {u.major && <Chip tone="bad">{t("ops.up.major")}</Chip>}
                      </div>
                    </td>
                    <td className="cell-end">
                      <button className="btn" onClick={() => setPick(u)} disabled={u.major}>
                        {t("ops.up.upgrade")}<span className="sr"> {u.name}</span>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {data && <p className="small muted">{t("ops.up.checked", { time: fmtTime(data.checked), pins: data.pins.length ? data.pins.join(", ") : t("ops.none") })}</p>}
        </State>
      </Card>
      {pick && (
        <ConfirmDialog
          title={t("ops.up.title", { name: pick.name })}
          word={target(pick)}
          verb={t("ops.up.upgrade")}
          body={
            <>
              <p>
                {t("ops.up.body", { from: pick.from, to: pick.to })}
              </p>
              {pick.pinned && <p className="alert alert-warn">{t("ops.up.pinned")}</p>}
            </>
          }
          onConfirm={async () => {
            try {
              const r = await post<{ job: number }>("/api/updates/apply", { target: target(pick), confirm: target(pick), allow_pinned: pick.pinned });
              trackJob(r.job, t("ops.up.title", { name: pick.name }), reload);
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
