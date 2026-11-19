import { useEffect, useState } from "react";
import { Plus, RotateCw, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { FilterTabs, ListSearch, Pager, useListView } from "../features/list/ListTools";
import AddTask, { TypeIcon } from "../features/tasks/AddTask";
import { describe } from "../features/tasks/schedule";
import { Alert, Card, Chip, ConfirmDialog, Modal, PageHead, State, Tabs, statusTone, useLoad } from "../ui";
import { useT } from "../i18n";

const typeOf = (cmd: string) => (cmd.startsWith("[shell]") ? "shell" : cmd.startsWith("[url]") ? "url" : /^(php|composer)\b/.test(cmd) ? "php" : /^(node|npm|pnpm)\b/.test(cmd) ? "node" : /^python/.test(cmd) ? "python" : "cmd");

interface Task {
  name: string;
  status: string;
  pid: string;
  exit: string;
  schedule: string;
  cmd?: string;
}

export default function Tasks({ sub = "" }: { sub?: string }) {
  const t = useT();
  const [kind, setKind] = useState<"worker" | "job">(sub === "job" ? "job" : "worker");
  useEffect(() => { if (sub === "job" || sub === "worker") setKind(sub); }, [sub]);
  const { data, error, loading, reload } = useLoad(() => get<{ tasks: Task[] }>(`/api/tasks/${kind}`), [kind]);
  const tasks = data?.tasks ?? [];
  const statusFacets = [...new Set(tasks.map((x) => x.status).filter(Boolean))].sort().map((st) => ({ id: st, label: st[0].toUpperCase() + st.slice(1), test: (x: Task) => x.status === st }));
  const lv = useListView(tasks, (x) => `${x.name} ${x.status} ${x.schedule ?? ""} ${x.cmd ?? ""} ${describe(x.schedule ?? "")}`, undefined, statusFacets);
  const [adding, setAdding] = useState(false);
  const [rm, setRm] = useState<string | null>(null);
  const [log, setLog] = useState<string | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const noun = t(kind === "worker" ? "dev.task.noun.worker" : "dev.task.noun.job");

  const restart = async (n: string) => {
    setNote(null);
    try {
      await post(`/api/tasks/${kind}/${n}/restart`, {});
      setNote({ tone: "ok", text: t("dev.task.restarted", { name: n }) });
      setTimeout(reload, 600);
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <>
      <PageHead
        title={t("nav.tasks")}
        desc={t("dev.task.desc")}
        action={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            {t("dev.task.add", { noun })}
          </button>
        }
      />
      <Tabs
        label={t("dev.task.kinds")}
        value={kind}
        onChange={(k) => {
          setKind(k as "worker" | "job");
          setNote(null);
        }}
        tabs={[
          { id: "worker", label: t("dev.task.tab.worker") },
          { id: "job", label: t("dev.task.tab.job") },
        ]}
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Card>
        <State loading={loading && !data} error={error} empty={!!data && data.tasks.length === 0} emptyText={t("dev.task.none", { noun })} onRetry={reload}>
          <FilterTabs lv={lv} label={t("dev.task.filterStatus")} />
          <ListSearch lv={lv} label={t("list.search")} hint={t("dev.task.searchHint")} />
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">{t("dev.col.name")}</th>
                  <th scope="col">{t("apps.col.status")}</th>
                  <th scope="col">PID</th>
                  <th scope="col">{t("dev.task.lastExit")}</th>
                  <th scope="col">{kind === "job" ? t("dev.task.schedule") : t("dev.task.mode")}</th>
                  <th scope="col" className="col-low">{t("dev.task.typeCmd")}</th>
                  <th scope="col">
                    <span className="sr">{t("dev.col.actions")}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {lv.view.rows.map((x) => (
                  <tr key={x.name}>
                    <td className="mono">{x.name}</td>
                    <td>
                      <Chip tone={statusTone(x.status)}>{x.status}</Chip>
                    </td>
                    <td className="mono">{x.pid}</td>
                    <td className="mono">{x.exit}</td>
                    <td title={x.schedule}>
                      {kind === "job" ? describe(x.schedule) : t("dev.task.always")}
                      {kind === "job" && <span className="small muted mono block">{x.schedule}</span>}
                    </td>
                    <td className="mono cell-trunc col-low" title={x.cmd ?? ""}>
                      <TypeIcon t={typeOf(x.cmd ?? "")} /> {(x.cmd ?? "").replace(/^\[(shell|url)\] /, "")}
                    </td>
                    <td className="cell-end">
                      <div className="row">
                        <button className="btn" onClick={() => setLog(x.name)}>
                          {t("apps.log")}<span className="sr"> {x.name}</span>
                        </button>
                        <button className="btn btn-icon" onClick={() => restart(x.name)} aria-label={t("dev.docker.restartAria", { name: x.name })}>
                          <RotateCw size={18} aria-hidden="true" />
                        </button>
                        <button className="btn btn-danger-ghost btn-icon" onClick={() => setRm(x.name)} aria-label={t("dev.docker.rmAria", { name: x.name })}>
                          <Trash2 size={18} aria-hidden="true" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pager lv={lv} />
        </State>
      </Card>
      {adding && <AddTask kind={kind} onClose={() => setAdding(false)} onDone={() => { setAdding(false); reload(); }} />}
      {rm && (
        <ConfirmDialog
          title={t("dev.task.rmTitle", { noun, name: rm })}
          word={rm}
          verb={t("apps.remove")}
          body={<p>{t("dev.task.rmBody")}</p>}
          onConfirm={async () => {
            await post(`/api/tasks/${kind}/${rm}/remove`, { confirm: rm });
            reload();
          }}
          onClose={() => setRm(null)}
        />
      )}
      {log && <LogDialog kind={kind} name={log} onClose={() => setLog(null)} />}
    </>
  );
}

function LogDialog({ kind, name, onClose }: { kind: string; name: string; onClose: () => void }) {
  const t = useT();
  const { data, error, loading, reload } = useLoad(() => get<{ log: string }>(`/api/tasks/${kind}/${name}/logs?n=100`));
  return (
    <Modal title={t("apps.logTitle", { name })} onClose={onClose} wide>
      <State loading={loading && !data} error={error} empty={!!data && !data.log.trim()} emptyText={t("dev.docker.logEmpty")} onRetry={reload}>
        <pre className="logbox mono">{data?.log}</pre>
      </State>
      <div className="row-end">
        <button className="btn" onClick={reload}>
          {t("common.reload")}
        </button>
        <button className="btn" onClick={onClose}>
          {t("common.close")}
        </button>
      </div>
    </Modal>
  );
}
