import { useEffect, useState } from "react";
import { Plus, RotateCw, Trash2 } from "lucide-react";
import { errText, get, post } from "../api";
import { FilterTabs, ListSearch, Pager, useListView } from "../features/list/ListTools";
import AddTask, { TypeIcon } from "../features/tasks/AddTask";
import { describe } from "../features/tasks/schedule";
import { Alert, Card, Chip, ConfirmDialog, Modal, PageHead, State, Tabs, statusTone, useLoad } from "../ui";

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
  const [kind, setKind] = useState<"worker" | "job">(sub === "job" ? "job" : "worker");
  useEffect(() => { if (sub === "job" || sub === "worker") setKind(sub); }, [sub]);
  const { data, error, loading, reload } = useLoad(() => get<{ tasks: Task[] }>(`/api/tasks/${kind}`), [kind]);
  const tasks = data?.tasks ?? [];
  const statusFacets = [...new Set(tasks.map((t) => t.status).filter(Boolean))].sort().map((st) => ({ id: st, label: st[0].toUpperCase() + st.slice(1), test: (t: Task) => t.status === st }));
  const lv = useListView(tasks, (t) => `${t.name} ${t.status} ${t.schedule ?? ""} ${t.cmd ?? ""} ${describe(t.schedule ?? "")}`, undefined, statusFacets);
  const [adding, setAdding] = useState(false);
  const [rm, setRm] = useState<string | null>(null);
  const [log, setLog] = useState<string | null>(null);
  const [note, setNote] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const noun = kind === "worker" ? "worker" : "cron job";

  const restart = async (n: string) => {
    setNote(null);
    try {
      await post(`/api/tasks/${kind}/${n}/restart`, {});
      setNote({ tone: "ok", text: `${n} dimulai ulang.` });
      setTimeout(reload, 600);
    } catch (x) {
      setNote({ tone: "bad", text: errText(x) });
    }
  };
  return (
    <>
      <PageHead
        title="Worker dan cron"
        desc="Pengganti Supervisor dan crontab, dijalankan launchd. Tipe shell, PHP, Node, Python, dan URL; jadwal dari tiap N menit sampai cron kustom. Berjalan setelah Anda login di Mac mini (FileVault berhenti di layar unlock sehabis reboot)."
        action={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Plus size={18} aria-hidden="true" />
            Tambah {noun}
          </button>
        }
      />
      <Tabs
        label="Jenis tugas"
        value={kind}
        onChange={(k) => {
          setKind(k as "worker" | "job");
          setNote(null);
        }}
        tabs={[
          { id: "worker", label: "Worker (queue, daemon)" },
          { id: "job", label: "Cron (terjadwal)" },
        ]}
      />
      {note && <Alert tone={note.tone}>{note.text}</Alert>}
      <Card>
        <State loading={loading && !data} error={error} empty={!!data && data.tasks.length === 0} emptyText={`Belum ada ${noun}.`} onRetry={reload}>
          <FilterTabs lv={lv} label="Filter status" />
          <ListSearch lv={lv} label="Cari" hint="Nama, status, atau jadwal" />
          <div className="tbl-wrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th scope="col">Nama</th>
                  <th scope="col">Status</th>
                  <th scope="col">PID</th>
                  <th scope="col">Exit terakhir</th>
                  <th scope="col">{kind === "job" ? "Jadwal" : "Mode"}</th>
                  <th scope="col" className="col-low">Tipe dan perintah</th>
                  <th scope="col">
                    <span className="sr">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {lv.view.rows.map((t) => (
                  <tr key={t.name}>
                    <td className="mono">{t.name}</td>
                    <td>
                      <Chip tone={statusTone(t.status)}>{t.status}</Chip>
                    </td>
                    <td className="mono">{t.pid}</td>
                    <td className="mono">{t.exit}</td>
                    <td title={t.schedule}>
                      {kind === "job" ? describe(t.schedule) : "Selalu berjalan"}
                      {kind === "job" && <span className="small muted mono block">{t.schedule}</span>}
                    </td>
                    <td className="mono cell-trunc col-low" title={t.cmd ?? ""}>
                      <TypeIcon t={typeOf(t.cmd ?? "")} /> {(t.cmd ?? "").replace(/^\[(shell|url)\] /, "")}
                    </td>
                    <td className="cell-end">
                      <div className="row">
                        <button className="btn" onClick={() => setLog(t.name)}>
                          Log<span className="sr"> {t.name}</span>
                        </button>
                        <button className="btn btn-icon" onClick={() => restart(t.name)} aria-label={`Mulai ulang ${t.name}`}>
                          <RotateCw size={18} aria-hidden="true" />
                        </button>
                        <button className="btn btn-danger-ghost btn-icon" onClick={() => setRm(t.name)} aria-label={`Hapus ${t.name}`}>
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
          title={`Hapus ${noun} ${rm}`}
          word={rm}
          verb="Hapus"
          body={<p>Proses dihentikan dan LaunchAgent dihapus. Log tetap ada.</p>}
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
  const { data, error, loading, reload } = useLoad(() => get<{ log: string }>(`/api/tasks/${kind}/${name}/logs?n=100`));
  return (
    <Modal title={`Log ${name}`} onClose={onClose} wide>
      <State loading={loading && !data} error={error} empty={!!data && !data.log.trim()} emptyText="Log masih kosong." onRetry={reload}>
        <pre className="logbox mono">{data?.log}</pre>
      </State>
      <div className="row-end">
        <button className="btn" onClick={reload}>
          Muat ulang
        </button>
        <button className="btn" onClick={onClose}>
          Tutup
        </button>
      </div>
    </Modal>
  );
}
