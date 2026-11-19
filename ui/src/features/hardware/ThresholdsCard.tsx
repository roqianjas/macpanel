import { useEffect, useState } from "react";
import { SlidersHorizontal } from "lucide-react";
import { errText, get, post } from "../../api";
import { useT } from "../../i18n";
import { Alert, Card, Field, State, useLoad } from "../../ui";
import { changed, isDefault, parseNum, THR_FIELDS, thrError, type Thr } from "./thresholds";

// Ikon SlidersHorizontal: kartu ini tentang ambang yang digeser, jadi relevan dengan isinya.
// Ambang bawaan adalah tebakan beralasan; kartu ini menampilkan bawaan di samping nilai aktif agar mudah dibandingkan.

interface Resp {
  thresholds: Thr;
  defaults: Thr;
  can_edit: boolean;
}

type Note = { tone: "ok" | "bad"; text: string } | null;

export default function ThresholdsCard() {
  const t = useT();
  const q = useLoad(() => get<Resp>("/api/hw/thresholds"));
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const r = q.data;

  useEffect(() => {
    if (r) setDraft(Object.fromEntries(THR_FIELDS.map((f) => [f.key, String(r.thresholds[f.key]).replace(".", ",")])));
  }, [r]);

  const parsed = r ? (Object.fromEntries(THR_FIELDS.map((f) => [f.key, parseNum(draft[f.key] ?? "")])) as unknown as Thr) : null;
  const err = parsed ? thrError(parsed) : "";
  const diff = r && parsed ? changed(parsed, r.thresholds) : {};
  const dirty = Object.keys(diff).length > 0;

  const run = async (body: unknown, done: string) => {
    setBusy(true);
    setNote(null);
    try {
      await post("/api/hw/thresholds", body);
      await q.reload();
      setNote({ tone: "ok", text: done });
    } catch (e) {
      setNote({ tone: "bad", text: errText(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card title={<><SlidersHorizontal size={18} className="icon" aria-hidden="true" /> {t("system.thr.title")}</>}>
      <State loading={q.loading && !r} error={q.error} onRetry={q.reload}>
        {r && parsed && (
          <div className="stack">
            {note && <Alert tone={note.tone}>{note.text}</Alert>}
            <p className="small muted">
              {t("system.thr.hint")}
            </p>
            <form
              className="stack"
              onSubmit={(e) => {
                e.preventDefault();
                if (!err && dirty) run(diff, t("system.thr.saved"));
              }}
            >
              {THR_FIELDS.map((f) => (
                <Field key={f.key} label={t("system.thr.fieldLabel", { label: f.label, unit: f.unit })} hint={t("system.thr.default", { v: r.defaults[f.key], unit: f.unit })}>
                  {(id, dd) => (
                    <input id={id} aria-describedby={dd} placeholder={String(r.defaults[f.key])} inputMode="decimal" value={draft[f.key] ?? ""} disabled={!r.can_edit || busy} aria-invalid={!!err && !Number.isFinite(parsed[f.key])} onChange={(e) => setDraft({ ...draft, [f.key]: e.target.value })} />
                  )}
                </Field>
              ))}
              {err && dirty && <Alert tone="bad">{err}</Alert>}
              {r.can_edit ? (
                <div className="row-end">
                  <button type="button" className="btn btn-ghost" disabled={busy || isDefault(r.thresholds, r.defaults)} onClick={() => run({ reset: true }, t("system.thr.resetDone"))}>
                    {t("system.thr.reset")}
                  </button>
                  <button className="btn btn-primary" disabled={busy || !dirty || !!err}>
                    {busy ? t("system.thr.saving") : t("system.thr.save")}
                  </button>
                </div>
              ) : (
                <p className="small muted">{t("system.thr.adminOnly")}</p>
              )}
            </form>
          </div>
        )}
      </State>
    </Card>
  );
}
