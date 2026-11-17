import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from "react";
import { LoaderCircle, TriangleAlert, X } from "lucide-react";
import { errText } from "./api";

/** useLoad: muat data dengan status memuat/galat/kosong yang konsisten (R-27). */
export function useLoad<T>(fn: () => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const fnRef = useRef(fn);
  fnRef.current = fn;
  const reload = useCallback(async () => {
    setLoading(true);
    try {
      setData(await fnRef.current());
      setError("");
    } catch (e) {
      setError(errText(e));
    } finally {
      setLoading(false);
    }
  }, []);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => void reload(), deps);
  return { data, error, loading, reload };
}

export function State({ loading, error, empty, emptyText, onRetry, children }: { loading: boolean; error: string; empty?: boolean; emptyText?: string; onRetry?: () => void; children: ReactNode }) {
  if (error)
    return (
      <div className="state state-bad" role="alert">
        <TriangleAlert className="icon" size={18} aria-hidden="true" />
        <div>
          <p className="state-title">Gagal memuat</p>
          <p className="small muted pre">{error}</p>
          {onRetry && (
            <button className="btn" onClick={onRetry}>
              Coba lagi
            </button>
          )}
        </div>
      </div>
    );
  if (loading && !children) return null;
  if (loading)
    return (
      <div className="state" role="status">
        <LoaderCircle className="icon spin" size={18} aria-hidden="true" />
        <p className="muted">Memuat…</p>
      </div>
    );
  if (empty) return <div className="state"><p className="muted">{emptyText ?? "Belum ada data."}</p></div>;
  return <>{children}</>;
}

export type Tone = "ok" | "warn" | "bad" | "info" | "mute";

/** Bentuk status: lingkaran = hidup, kotak = mati, cincin = belum aktif, segitiga = peringatan. Selalu bersama teks. */
export function Mark({ tone }: { tone: Tone }) {
  return (
    <svg className={`mark mark-${tone === "mute" ? "off" : tone}`} width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" focusable="false">
      {(tone === "ok" || tone === "info") && <circle cx="5" cy="5" r="4.2" fill="currentColor" />}
      {tone === "bad" && <rect x="1" y="1" width="8" height="8" fill="currentColor" />}
      {tone === "mute" && <circle cx="5" cy="5" r="3.7" fill="none" stroke="currentColor" strokeWidth="1.4" />}
      {tone === "warn" && <path d="M5 .8 9.4 9H.6z" fill="currentColor" />}
    </svg>
  );
}

export function Chip({ tone = "mute", children, busy }: { tone?: Tone; children: ReactNode; busy?: boolean }) {
  return (
    <span className={`chip chip-${tone}`}>
      {busy ? <LoaderCircle className="icon spin" size={12} aria-hidden="true" /> : <Mark tone={tone} />}
      {children}
    </span>
  );
}

/** Muat data berkala (mis. status layanan). Berhenti saat tab tersembunyi. */
export function usePoll<T>(fn: () => Promise<T>, everyMs: number, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const fnRef = useRef(fn);
  fnRef.current = fn;
  const reload = useCallback(async () => {
    try {
      setData(await fnRef.current());
      setError("");
    } catch (e) {
      setError(errText(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    setLoading(true);
    void reload();
    const t = setInterval(() => {
      if (document.visibilityState === "visible") void reload();
    }, everyMs);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [everyMs, ...deps]);
  return { data, error, loading, reload };
}

export function statusTone(s: string): Tone {
  if (["ok", "jalan", "terpasang", "aktif", "running"].includes(s)) return "ok";
  if (["gagal", "timeout", "terputus", "error"].includes(s)) return "bad";
  if (["terjadwal"].includes(s)) return "info";
  if (["berjalan", "tidak dimuat"].includes(s)) return "warn";
  return "mute";
}

export function Card({ title, action, children, className = "" }: { title?: ReactNode; action?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`card ${className}`}>
      {(title || action) && (
        <header className="card-head">
          {title && <h2 className="card-title">{title}</h2>}
          {action}
        </header>
      )}
      {children}
    </section>
  );
}

export function PageHead({ title, desc, action }: { title: string; desc?: string; action?: ReactNode }) {
  return (
    <div className="page-head">
      <div>
        <h1>{title}</h1>
        {desc && <p className="muted">{desc}</p>}
      </div>
      {action && <div className="page-actions">{action}</div>}
    </div>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: (id: string, describedBy?: string) => ReactNode }) {
  const id = useId();
  const hid = hint ? `${id}-h` : undefined;
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {children(id, hid)}
      {hint && (
        <p id={hid} className="small muted">
          {hint}
        </p>
      )}
    </div>
  );
}

/** Dialog: fokus masuk dan terkunci di dalam, Escape menutup, fokus kembali ke pemicu. */
export function Modal({ title, onClose, children, wide }: { title: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const tid = useId();
  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    const node = ref.current!;
    const focusable = () => Array.from(node.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'));
    (node.querySelector<HTMLElement>("[data-autofocus]") ?? focusable()[0] ?? node).focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
      } else if (e.key === "Tab") {
        const f = focusable();
        if (!f.length) return;
        const first = f[0],
          last = f[f.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    node.addEventListener("keydown", onKey);
    document.body.classList.add("modal-open");
    return () => {
      node.removeEventListener("keydown", onKey);
      document.body.classList.remove("modal-open");
      prev?.focus?.();
    };
  }, [onClose]);
  return (
    <div className="scrim" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} className={`modal ${wide ? "modal-wide" : ""}`} role="dialog" aria-modal="true" aria-labelledby={tid} tabIndex={-1}>
        <header className="modal-head">
          <h2 id={tid}>{title}</h2>
          <button className="btn btn-ghost btn-icon" onClick={onClose} aria-label="Tutup dialog">
            <X size={18} aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body">{children}</div>
      </div>
    </div>
  );
}

/** Konfirmasi aksi merusak: pengguna mengetik ulang kata kunci. Kata sandi ulang diminta otomatis oleh api(). */
export function ConfirmDialog({ title, body, word, verb, onConfirm, onClose }: { title: string; body: ReactNode; word: string; verb: string; onConfirm: () => Promise<void>; onClose: () => void }) {
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const submit = async () => {
    setBusy(true);
    setErr("");
    try {
      await onConfirm();
      onClose();
    } catch (e) {
      setErr(errText(e));
      setBusy(false);
    }
  };
  return (
    <Modal title={title} onClose={onClose}>
      <div className="stack">
        <div>{body}</div>
        <Field label={`Ketik “${word}” untuk melanjutkan`}>
          {(id) => <input id={id} data-autofocus className="mono" value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" spellCheck={false} />}
        </Field>
        {err && (
          <p className="alert alert-bad pre" role="alert">
            {err}
          </p>
        )}
        <div className="row-end">
          <button className="btn" onClick={onClose}>
            Batal
          </button>
          <button className="btn btn-danger" disabled={typed !== word || busy} onClick={submit}>
            {busy ? "Memproses…" : verb}
          </button>
        </div>
      </div>
    </Modal>
  );
}

export function Alert({ tone = "bad", children }: { tone?: "bad" | "ok" | "warn"; children: ReactNode }) {
  return (
    <p className={`alert alert-${tone} pre`} role={tone === "bad" ? "alert" : "status"}>
      {children}
    </p>
  );
}

export function Tabs({ tabs, value, onChange, label }: { tabs: { id: string; label: string }[]; value: string; onChange: (id: string) => void; label: string }) {
  const onKey = (e: React.KeyboardEvent, i: number) => {
    let n = i;
    if (e.key === "ArrowRight") n = (i + 1) % tabs.length;
    else if (e.key === "ArrowLeft") n = (i - 1 + tabs.length) % tabs.length;
    else if (e.key === "Home") n = 0;
    else if (e.key === "End") n = tabs.length - 1;
    else return;
    e.preventDefault();
    onChange(tabs[n].id);
    document.getElementById(`tab-${tabs[n].id}`)?.focus();
  };
  return (
    <div className="tabs" role="tablist" aria-label={label}>
      {tabs.map((t, i) => (
        <button key={t.id} id={`tab-${t.id}`} role="tab" aria-selected={value === t.id} tabIndex={value === t.id ? 0 : -1} className={`tab ${value === t.id ? "on" : ""}`} onClick={() => onChange(t.id)} onKeyDown={(e) => onKey(e, i)}>
          {t.label}
        </button>
      ))}
    </div>
  );
}
