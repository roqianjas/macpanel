import { useState } from "react";
import { BellRing } from "lucide-react";
import { errText, get, post } from "../../api";
import { Alert, Card, Chip, ConfirmDialog, Field, State, useLoad } from "../../ui";
import { genTopic, lastText, serverError, topicError, type NotifyCfg } from "./model";

// Ikon BellRing: kartu ini tentang pesan ke ponsel, jadi lonceng relevan dengan isinya.
// Topik adalah rahasia: dikirim ke server sekali lalu tidak pernah dibaca balik; yang tampil hanya "sudah diisi".

type Note = { tone: "ok" | "bad" | "warn"; text: string } | null;

export default function NotifyCard() {
  const q = useLoad(() => get<NotifyCfg>("/api/notify"));
  const c = q.data;
  const [server, setServer] = useState<string | null>(null);
  const [topic, setTopic] = useState("");
  const [min, setMin] = useState<"bad" | "warn" | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [shown, setShown] = useState("");
  const [clearing, setClearing] = useState(false);
  const adminOnly = !!q.error && /admin/i.test(q.error);

  const srv = server ?? c?.server ?? "";
  const lvl = min ?? c?.min ?? "bad";
  const sErr = server !== null ? serverError(srv) : "";
  const tErr = topic !== "" ? topicError(topic) : "";
  const dirty = server !== null || topic !== "" || min !== null;

  const save = async (extra: Record<string, unknown>, done: string) => {
    setBusy(true);
    setNote(null);
    try {
      await post<NotifyCfg>("/api/notify", extra);
      if (typeof extra.topic === "string") setShown(extra.topic);
      setServer(null);
      setTopic("");
      setMin(null);
      await q.reload();
      setNote({ tone: "ok", text: done });
    } catch (e) {
      setNote({ tone: "bad", text: errText(e) });
    } finally {
      setBusy(false);
    }
  };

  const onSave = () => {
    const body: Record<string, unknown> = {};
    if (server !== null) body.server = srv.trim();
    if (topic !== "") body.topic = topic;
    if (min !== null) body.min = min;
    save(body, topic !== "" ? "Tersimpan. Salin topik ke aplikasi ntfy sekarang, panel tidak menampilkannya lagi." : "Tersimpan.");
  };

  const test = async () => {
    setBusy(true);
    setNote(null);
    try {
      const r = await post<{ ok: boolean; error?: string }>("/api/notify/test", {});
      setNote(r.ok ? { tone: "ok", text: "Pesan uji terkirim. Periksa ponsel Anda." } : { tone: "bad", text: `Pesan uji gagal: ${r.error ?? "alasan tidak diketahui"}` });
      await q.reload();
    } catch (e) {
      setNote({ tone: "bad", text: errText(e) });
    } finally {
      setBusy(false);
    }
  };

  if (adminOnly)
    return (
      <Card title={<><BellRing size={18} className="icon" aria-hidden="true" /> Notifikasi ponsel</>}>
        <p className="small muted">Hanya admin yang dapat melihat dan mengubah pengaturan notifikasi.</p>
      </Card>
    );

  return (
    <Card title={<><BellRing size={18} className="icon" aria-hidden="true" /> Notifikasi ponsel</>}>
      <State loading={q.loading && !c} error={q.error} onRetry={q.reload}>
        {c && (
          <div className="stack">
            {note && <Alert tone={note.tone}>{note.text}</Alert>}
            {shown && (
              <Alert tone="warn">
                {`Topik Anda: ${shown}\nBuka aplikasi ntfy di ponsel, pilih langganan topik, lalu tempel topik ini. Panel tidak akan menampilkannya lagi.`}{" "}
                <button type="button" className="btn btn-sm" onClick={() => navigator.clipboard.writeText(shown).then(() => setNote({ tone: "ok", text: "Topik disalin." }), () => setNote({ tone: "bad", text: "Gagal menyalin; salin manual." }))}>
                  Salin topik
                </button>{" "}
                <button type="button" className="btn btn-sm btn-ghost" onClick={() => setShown("")}>
                  Sudah saya salin
                </button>
              </Alert>
            )}
            <p>
              <Chip tone={c.enabled ? "ok" : "mute"}>{c.enabled ? "Aktif" : "Nonaktif"}</Chip> <span className="small muted">{lastText(c.last, Math.floor(Date.now() / 1000))}</span>
            </p>
            <p className="small muted">
              Saat aktif, peringatan penting dikirim ke ponsel lewat ntfy, termasuk pesan "pulih" saat masalah selesai. Pesan hanya berisi judul dan detail peringatan, tanpa kata sandi atau jalur file. Pesan melewati server ntfy yang Anda pilih; di ntfy.sh publik, siapa pun yang tahu topik dapat membaca pesannya, jadi topik harus sulit ditebak.
            </p>
            <form
              className="stack"
              onSubmit={(e) => {
                e.preventDefault();
                onSave();
              }}
            >
              <Field label="Server ntfy" hint={sErr || "Harus https. Bawaan ntfy.sh; bisa diganti ke server ntfy milik sendiri."}>
                {(id, dd) => <input id={id} aria-describedby={dd} aria-invalid={!!sErr} value={srv} onChange={(e) => setServer(e.target.value)} placeholder={c.default_server} autoComplete="off" spellCheck={false} />}
              </Field>
              <Field label="Topik (rahasia)" hint={tErr || (c.topic_set ? "Sudah diisi. Kosongkan untuk tidak mengubah, atau isi untuk menggantinya." : "16-64 karakter. Gunakan tombol di bawah untuk membuat yang acak.")}>
                {(id, dd) => <input id={id} className="mono" aria-describedby={dd} aria-invalid={!!tErr} value={topic} onChange={(e) => setTopic(e.target.value)} placeholder={c.topic_set ? "tidak diubah" : "isi topik"} autoComplete="off" spellCheck={false} />}
              </Field>
              <div className="row-end">
                <button type="button" className="btn btn-sm" onClick={() => setTopic(genTopic())}>
                  Buat topik acak
                </button>
              </div>
              <Field label="Kirim peringatan" hint="Penting saja menjaga ponsel tidak berisik; 'Penting dan perhatian' juga mengirim peringatan tingkat perhatian.">
                {(id, dd) => (
                  <select id={id} aria-describedby={dd} value={lvl} onChange={(e) => setMin(e.target.value as "bad" | "warn")}>
                    <option value="bad">Hanya yang penting</option>
                    <option value="warn">Penting dan perhatian</option>
                  </select>
                )}
              </Field>
              <div className="row-end">
                <button className="btn btn-primary" disabled={busy || !dirty || !!sErr || !!tErr}>
                  {busy ? "Menyimpan…" : "Simpan"}
                </button>
              </div>
            </form>
            <div className="row-end">
              {c.topic_set && (
                <>
                  <button type="button" className="btn" disabled={busy} onClick={() => save({ enabled: !c.enabled }, c.enabled ? "Notifikasi dinonaktifkan." : "Notifikasi diaktifkan. Peringatan yang sudah ada saat ini tidak dikirim; hanya yang baru.")}>
                    {c.enabled ? "Nonaktifkan" : "Aktifkan"}
                  </button>
                  <button type="button" className="btn" disabled={busy} onClick={test}>
                    Kirim pesan uji
                  </button>
                  <button type="button" className="btn btn-ghost" disabled={busy} onClick={() => setClearing(true)}>
                    Hapus topik
                  </button>
                </>
              )}
            </div>
          </div>
        )}
      </State>
      {clearing && (
        <ConfirmDialog
          title="Hapus topik notifikasi?"
          body="Notifikasi dinonaktifkan dan topik dihapus dari panel. Untuk mengaktifkan lagi, Anda perlu mengisi topik baru dan berlangganan ulang di ponsel."
          word="hapus"
          verb="Hapus topik"
          onConfirm={async () => {
            await post("/api/notify", { clear_topic: true });
            setClearing(false);
            setNote({ tone: "ok", text: "Topik dihapus dan notifikasi dinonaktifkan." });
            await q.reload();
          }}
          onClose={() => setClearing(false)}
        />
      )}
    </Card>
  );
}
