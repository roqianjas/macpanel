import { useState } from "react";
import { get } from "../../api";
import { Card, State, useLoad } from "../../ui";
import { useT } from "../../i18n";

export function Logs({ name }: { name: string }) {
  const t = useT();
  const [kind, setKind] = useState("access");
  const { data, error, loading, reload } = useLoad(() => get<{ lines: string[]; note: string }>(`/api/sites/${name}/logs?kind=${kind}&n=100`), [name, kind]);
  return (
    <Card
      title={t("apps.log")}
      action={
        <div className="row">
          <select aria-label={t("dev.log.kind")} value={kind} onChange={(e) => setKind(e.target.value)} style={{ width: "auto" }}>
            <option value="access">{t("dev.log.access")}</option>
            <option value="error">Error</option>
          </select>
          <button className="btn" onClick={reload}>{t("common.reload")}</button>
        </div>
      }
    >
      <State loading={loading && !data} error={error} empty={!!data && data.lines.length === 0} emptyText={kind === "access" ? t("dev.log.noAccess") : t("dev.log.noError")} onRetry={reload}>
        <pre className="logbox mono">{data?.lines.join("\n")}</pre>
      </State>
    </Card>
  );
}
