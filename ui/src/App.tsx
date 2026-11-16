import { useCallback, useEffect, useState } from "react";
import { Boxes, Database, Download, Globe, History, KeyRound, LayoutDashboard, LogOut, Menu, Monitor, Moon, PanelLeftClose, PanelLeftOpen, Sun, Timer, X } from "lucide-react";
import { UNAUTH, api, errText, get, post, setCsrf, setStepUpHandler } from "./api";
import { Alert, Chip, Field, Modal, statusTone } from "./ui";
import { dismissJob, useTrackedJobs } from "./jobs";
import Overview from "./pages/Overview";
import Sites from "./pages/Sites";
import Databases from "./pages/Databases";
import Runtimes from "./pages/Runtimes";
import Tasks from "./pages/Tasks";
import Updates from "./pages/Updates";
import Activity from "./pages/Activity";

type Pref = "system" | "light" | "dark";

function applyTheme(p: Pref) {
  const dark = p === "dark" || (p === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  document.documentElement.setAttribute("data-pref", p);
}

function useTheme() {
  const [pref, setPref] = useState<Pref>(() => (localStorage.getItem("panel-theme") as Pref) || "system");
  useEffect(() => {
    applyTheme(pref);
    localStorage.setItem("panel-theme", pref);
    if (pref !== "system") return;
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const h = () => applyTheme("system");
    mq.addEventListener("change", h);
    return () => mq.removeEventListener("change", h);
  }, [pref]);
  const next = () => setPref((p) => (p === "system" ? "light" : p === "light" ? "dark" : "system"));
  return { pref, next };
}

// ---------- login ----------

function Login({ onDone }: { onDone: (user: string) => void }) {
  const [step, setStep] = useState<"pwd" | "totp">("pwd");
  const [name, setName] = useState("");
  const [pw, setPw] = useState("");
  const [code, setCode] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const submitPwd = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ csrf: string }>("/api/auth/login", { name, password: pw });
      setCsrf(r.csrf);
      setPw("");
      setStep("totp");
    } catch (x) {
      setErr(errText(x));
    } finally {
      setBusy(false);
    }
  };
  const submitCode = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ csrf: string }>("/api/auth/totp", { code: code.trim() });
      setCsrf(r.csrf);
      onDone(name.toLowerCase());
    } catch (x) {
      setErr(errText(x));
      setCode("");
      setBusy(false);
    }
  };
  return (
    <main className="login" id="main">
      <div className="login-card card">
        <div className="brand login-brand">
          <span className="brand-ic">
            <KeyRound size={20} aria-hidden="true" />
          </span>
          <span>Panel Mac mini</span>
        </div>
        <h1>{step === "pwd" ? "Masuk" : "Verifikasi dua langkah"}</h1>
        {step === "pwd" ? (
          <form onSubmit={submitPwd} className="stack">
            <Field label="Nama pengguna">{(id) => <input id={id} autoFocus autoComplete="username" value={name} onChange={(e) => setName(e.target.value)} required />}</Field>
            <Field label="Kata sandi">{(id) => <input id={id} type="password" autoComplete="current-password" value={pw} onChange={(e) => setPw(e.target.value)} required />}</Field>
            {err && <Alert>{err}</Alert>}
            <button className="btn btn-primary" disabled={busy || !name || !pw}>
              {busy ? "Memeriksa…" : "Lanjut ke kode"}
            </button>
          </form>
        ) : (
          <form onSubmit={submitCode} className="stack">
            <p className="muted">Buka Google Authenticator (atau aplikasi TOTP Anda) dan masukkan kode 6 digit untuk akun ini. Kode recovery juga diterima.</p>
            <Field label="Kode 6 digit atau kode recovery">{(id) => <input id={id} autoFocus inputMode="text" autoComplete="one-time-code" className="mono code-input" value={code} onChange={(e) => setCode(e.target.value)} required spellCheck={false} />}</Field>
            {err && <Alert>{err}</Alert>}
            <div className="row-end">
              <button type="button" className="btn" onClick={() => { setStep("pwd"); setErr(""); }}>
                Kembali
              </button>
              <button className="btn btn-primary" disabled={busy || code.trim().length < 6}>
                {busy ? "Memeriksa…" : "Masuk"}
              </button>
            </div>
          </form>
        )}
        <p className="small muted">Akses hanya dari jaringan tailnet. Percobaan gagal dicatat dan dibatasi.</p>
      </div>
    </main>
  );
}

// ---------- step-up ----------

function StepUpDialog({ resolve }: { resolve: (ok: boolean) => void }) {
  const [code, setCode] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await post("/api/auth/stepup", { code: code.trim() });
      resolve(true);
    } catch (x) {
      setErr(errText(x));
      setCode("");
      setBusy(false);
    }
  };
  return (
    <Modal title="Verifikasi ulang" onClose={() => resolve(false)}>
      <form onSubmit={submit} className="stack">
        <p>Aksi ini penting. Masukkan kode TOTP yang baru (tunggu kode berikutnya bila baru dipakai untuk masuk). Berlaku 5 menit.</p>
        <Field label="Kode 6 digit atau kode recovery">{(id) => <input id={id} data-autofocus className="mono code-input" value={code} onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code" spellCheck={false} required />}</Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={() => resolve(false)}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || code.trim().length < 6}>
            {busy ? "Memeriksa…" : "Verifikasi"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ---------- penampil job ----------

function JobTray() {
  const jobs = useTrackedJobs().filter((j) => !j.dismissed);
  if (!jobs.length) return null;
  return (
    <div className="tray" aria-live="polite" aria-label="Aksi yang berjalan">
      {jobs.slice(0, 4).map((j) => (
        <div key={j.id} className="tray-item">
          <div className="tray-top">
            <Chip tone={statusTone(j.status)} busy={j.status === "berjalan"}>
              {j.status === "berjalan" ? "Berjalan" : j.status === "ok" ? "Selesai" : j.status}
            </Chip>
            <span className="tray-label">{j.label}</span>
            <button className="btn btn-ghost btn-icon" onClick={() => dismissJob(j.id)} aria-label={`Sembunyikan ${j.label}`}>
              <X size={16} aria-hidden="true" />
            </button>
          </div>
          {j.status !== "berjalan" && j.output && <pre className="tray-out">{j.output.slice(-600)}</pre>}
        </div>
      ))}
    </div>
  );
}

// ---------- kerangka ----------

const NAV = [
  { id: "ringkasan", label: "Ringkasan", icon: LayoutDashboard },
  { id: "website", label: "Website", icon: Globe },
  { id: "database", label: "Database", icon: Database },
  { id: "runtime", label: "PHP dan runtime", icon: Boxes },
  { id: "tugas", label: "Worker dan cron", icon: Timer },
  { id: "pembaruan", label: "Pembaruan", icon: Download },
  { id: "aktivitas", label: "Aktivitas", icon: History },
];

function parseHash(): { page: string; sub: string } {
  const h = location.hash.replace(/^#\/?/, "");
  const [page, sub = ""] = h.split("/");
  return { page: NAV.some((n) => n.id === page) ? page : "ringkasan", sub: decodeURIComponent(sub) };
}

function Shell({ user, onLogout }: { user: string; onLogout: () => void }) {
  const theme = useTheme();
  const [route, setRoute] = useState(parseHash());
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem("panel-collapsed") === "1");
  const [sheet, setSheet] = useState(false);
  useEffect(() => {
    const h = () => {
      setRoute(parseHash());
      setSheet(false);
      document.getElementById("main")?.focus();
      window.scrollTo(0, 0);
    };
    addEventListener("hashchange", h);
    return () => removeEventListener("hashchange", h);
  }, []);
  useEffect(() => {
    localStorage.setItem("panel-collapsed", collapsed ? "1" : "0");
  }, [collapsed]);

  const ThemeIcon = theme.pref === "dark" ? Moon : theme.pref === "light" ? Sun : Monitor;
  const themeLabel = theme.pref === "dark" ? "Gelap" : theme.pref === "light" ? "Terang" : "Ikuti sistem";
  const page = (() => {
    switch (route.page) {
      case "website":
        return <Sites sub={route.sub} />;
      case "database":
        return <Databases />;
      case "runtime":
        return <Runtimes />;
      case "tugas":
        return <Tasks />;
      case "pembaruan":
        return <Updates />;
      case "aktivitas":
        return <Activity />;
      default:
        return <Overview />;
    }
  })();

  return (
    <div className={`app ${collapsed ? "is-collapsed" : ""} ${sheet ? "sheet-open" : ""}`}>
      <a href="#main" className="skip" onClick={(e) => { e.preventDefault(); document.getElementById("main")?.focus(); }}>
        Lompat ke konten
      </a>
      <nav className="side" aria-label="Navigasi utama">
        <div className="side-top">
          <span className="brand">
            <span className="brand-ic">
              <KeyRound size={18} aria-hidden="true" />
            </span>
            <span className="side-text">Panel Mac mini</span>
          </span>
          <button className="btn btn-ghost btn-icon sheet-close" onClick={() => setSheet(false)} aria-label="Tutup menu">
            <X size={18} aria-hidden="true" />
          </button>
        </div>
        <ul>
          {NAV.map((n) => (
            <li key={n.id}>
              <a href={`#/${n.id}`} className={route.page === n.id ? "on" : ""} aria-current={route.page === n.id ? "page" : undefined}>
                <n.icon size={20} aria-hidden="true" />
                <span className="side-text">{n.label}</span>
              </a>
            </li>
          ))}
        </ul>
        <div className="side-bottom">
          <button className="side-btn" onClick={() => setCollapsed((c) => !c)} aria-expanded={!collapsed} aria-label={collapsed ? "Perlebar menu" : "Ciutkan menu"}>
            {collapsed ? <PanelLeftOpen size={20} aria-hidden="true" /> : <PanelLeftClose size={20} aria-hidden="true" />}
            <span className="side-text">Ciutkan</span>
          </button>
        </div>
      </nav>
      <div className="col">
        <header className="top">
          <button className="btn btn-ghost btn-icon menu-btn" onClick={() => setSheet(true)} aria-label="Buka menu" aria-expanded={sheet}>
            <Menu size={20} aria-hidden="true" />
          </button>
          <span className="top-user muted small">
            Masuk sebagai <strong className="mono">{user}</strong>
          </span>
          <div className="top-actions">
            <button className="btn" onClick={theme.next} aria-label={`Tema: ${themeLabel}. Klik untuk mengganti`}>
              <ThemeIcon size={18} aria-hidden="true" />
              <span className="btn-text">{themeLabel}</span>
            </button>
            <button className="btn" onClick={onLogout}>
              <LogOut size={18} aria-hidden="true" />
              <span className="btn-text">Keluar</span>
            </button>
          </div>
        </header>
        <main id="main" className="main" tabIndex={-1}>
          {page}
        </main>
      </div>
      <button className="scrim-side" aria-label="Tutup menu" tabIndex={-1} onClick={() => setSheet(false)} />
      <JobTray />
    </div>
  );
}

export default function App() {
  const [user, setUser] = useState<string | null | undefined>(undefined);
  const [stepUp, setStepUp] = useState<null | ((ok: boolean) => void)>(null);

  useEffect(() => {
    setStepUpHandler(
      () =>
        new Promise<boolean>((resolve) => {
          setStepUp(() => (ok: boolean) => {
            setStepUp(null);
            resolve(ok);
          });
        }),
    );
    const un = () => setUser(null);
    addEventListener(UNAUTH, un);
    get<{ user: string; csrf: string }>("/api/me")
      .then((r) => {
        setCsrf(r.csrf);
        setUser(r.user);
      })
      .catch(() => setUser(null));
    return () => removeEventListener(UNAUTH, un);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api("POST", "/api/auth/logout", {});
    } catch {
      /* sesi mungkin sudah habis */
    }
    setUser(null);
  }, []);

  if (user === undefined)
    return (
      <main className="login" id="main">
        <p className="muted" role="status">
          Memuat…
        </p>
      </main>
    );
  return (
    <>
      {user ? <Shell user={user} onLogout={logout} /> : <Login onDone={setUser} />}
      {stepUp && <StepUpDialog resolve={stepUp} />}
    </>
  );
}
