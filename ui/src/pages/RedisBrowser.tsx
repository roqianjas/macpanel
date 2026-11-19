import { useState } from "react";
import { Eye, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { useT } from "../i18n";
import { Alert, Card, ConfirmDialog, Field, Modal, State, useLoad } from "../ui";

interface KeyRow {
  key: string;
  type: string;
  ttl: number;
}
interface Val {
  key: string;
  type: string;
  ttl: number;
  size: number;
  value: string;
  truncated: boolean;
}

export default function RedisBrowser() {
  const t = useT();
  const [db, setDb] = useState("0");
  const [pattern, setPattern] = useState("*");
  const [applied, setApplied] = useState("*");
  const info = useLoad(() => get<{ dbsize: number; used_memory: string; maxmemory: string }>(`/api/redis/info?db=${db}`), [db]);
  const scan = useLoad(() => get<{ keys: KeyRow[]; more: boolean }>(`/api/redis/scan?db=${db}&pattern=${encodeURIComponent(applied)}`), [db, applied]);
  const [view, setView] = useState<Val | null>(null);
  const [del, setDel] = useState<string | null>(null);
  const [err, setErr] = useState("");
  const open = async (key: string) => {
    setErr("");
    try {
      setView(await post<Val>("/api/redis/get", { db, key }));
    } catch (x) {
      setErr(errText(x));
    }
  };
  return (
    <Card title={t("db.rd.title")}>
      <p className="small muted">
        {t("db.rd.intro", { info: info.data ? t("db.rd.info", { count: info.data.dbsize, db, mem: info.data.used_memory }) : "" })}
      </p>
      <form className="row-form" onSubmit={(e) => { e.preventDefault(); setApplied(pattern.trim() || "*"); }}>
        <Field label={t("db.rd.pattern")} hint={t("db.rd.patternHint")}>{(id, dd) => <input id={id} aria-describedby={dd} className="mono" placeholder="user:*" maxLength={100} pattern="[A-Za-z0-9:_.*?\-]{1,100}" title={t("db.rd.patternTitle")} value={pattern} onChange={(e) => setPattern(e.target.value)} spellCheck={false} />}</Field>
        <Field label={t("db.rd.database")}>{(id) => <select id={id} value={db} onChange={(e) => setDb(e.target.value)}>{Array.from({ length: 16 }, (_, i) => <option key={i} value={String(i)}>db {i}</option>)}</select>}</Field>
        <button className="btn" type="submit">{t("db.rd.search")}</button>
      </form>
      {err && <Alert>{err}</Alert>}
      <State loading={scan.loading && !scan.data} error={scan.error} empty={!!scan.data && scan.data.keys.length === 0} emptyText={t("db.rd.none")} onRetry={scan.reload}>
        <ul className="list">
          {scan.data?.keys.map((k) => (
            <li key={k.key} className="list-row">
              <div className="grow">
                <p className="mono ellip-wrap">{k.key}</p>
                <p className="small muted">{k.type} · {k.ttl < 0 ? t("db.rd.noTtl") : t("db.rd.ttl", { ttl: k.ttl })}</p>
              </div>
              <button className="btn btn-sm" onClick={() => open(k.key)}><Eye size={16} aria-hidden="true" />{t("db.rd.view")}<span className="sr"> {k.key}</span></button>
              <button className="btn btn-danger-ghost btn-sm" onClick={() => setDel(k.key)}><Trash2 size={16} aria-hidden="true" />{t("db.rd.del")}<span className="sr"> {k.key}</span></button>
            </li>
          ))}
        </ul>
        {scan.data?.more && <p className="small muted">{t("db.rd.more")}</p>}
      </State>
      {view && (
        <Modal title={view.key} onClose={() => setView(null)} wide>
          <p className="small muted">{view.type} · {view.size} {view.type === "string" ? t("db.rd.byte") : t("db.rd.elem")}{view.truncated ? t("db.rd.partial") : ""}</p>
          <pre className="logbox mono" tabIndex={0} aria-label={t("db.rd.valAria")}>{view.value}</pre>
        </Modal>
      )}
      {del && (
        <ConfirmDialog
          title={t("db.rd.delTitle")}
          word={del.length > 40 ? del.slice(0, 40) : del}
          verb={t("db.rd.delVerb")}
          body={<p className="mono ellip-wrap">{del}</p>}
          onConfirm={async () => {
            await post("/api/redis/del", { db, key: del });
            scan.reload();
            info.reload();
          }}
          onClose={() => setDel(null)}
        />
      )}
    </Card>
  );
}
