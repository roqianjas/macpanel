import { useCallback, useEffect, useState } from "react";
import { Activity, BookOpen, Bell, Boxes, Database, DatabaseBackup, Download, Globe, History, KeyRound, LayoutDashboard, LogOut, Menu, Monitor, Moon, PanelLeftClose, PanelLeftOpen, Server, Settings as SettingsIcon, ShieldCheck, Sun, Timer, X, type LucideIcon } from "lucide-react";
import { UNAUTH, api, errText, get, post, setCsrf, setRole, setStepUpHandler } from "./api";
import { useAlerts } from "./data";
import { Alert, Chip, Field, Modal, statusTone } from "./ui";
import ErrorBoundary from "./ErrorBoundary";
import { setDevicePref, usePanelName, useThemePref } from "./settings";
import Settings from "./pages/Settings";
import Docs from "./pages/Docs";
import { dismissJob, useTrackedJobs } from "./jobs";
import Overview from "./pages/Overview";
import Monitoring from "./pages/Monitoring";
import Sites from "./pages/Sites";
import Apps from "./pages/Apps";
import Databases from "./pages/Databases";
import Runtimes from "./pages/Runtimes";
import Tasks from "./pages/Tasks";
import Updates from "./pages/Updates";
import ActivityPage from "./pages/Activity";
import Backups from "./pages/Backups";
import System from "./pages/System";

function useTheme() {
  const pref = useThemePref();
  return { pref, next: () => setDevicePref(pref === "system" ? "light" : pref === "light" ? "dark" : "system") };
}

// ---------- masuk ----------

function Login({ onDone }: { onDone: (user: string) => void }) {
  const panelName = usePanelName();
  const [step, setStep] = useState<"pwd" | "totp">("pwd");
  const [name, setName] = useState("");
  const [pw, setPw] = useState("");
  const [code, setCode] = useState("");
  const [trust, setTrust] = useState(false);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const submitPwd = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const r = await post<{ csrf: string; trusted?: boolean; no2fa?: boolean }>("/api/auth/login", { name, password: pw });
      setCsrf(r.csrf);
      setPw("");
      if (r.trusted || r.no2fa) {
        onDone(name.toLowerCase());
        return;
      }
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
      const r = await post<{ csrf: string }>("/api/auth/totp", { code: code.trim(), trust });
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
        <div className="brand">
          <span className="brand-ic">
            <KeyRound size={20} aria-hidden="true" />
          </span>
          <span>{panelName}</span>
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
            <p className="muted small">Masukkan kode 6 digit dari Google Authenticator (atau aplikasi TOTP Anda). Satu kode recovery juga diterima.</p>
            <Field label="Kode 6 digit atau kode recovery">{(id) => <input id={id} autoFocus autoComplete="one-time-code" className="mono code-input" value={code} onChange={(e) => setCode(e.target.value)} required spellCheck={false} />}</Field>
            <label className="check"><input type="checkbox" checked={trust} onChange={(e) => setTrust(e.target.checked)} />Percayai perangkat ini selama 30 hari (berikutnya cukup kata sandi). Jangan dicentang di komputer umum.</label>
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

function StepUpDialog({ resolve }: { resolve: (ok: boolean) => void }) {
  const [pw, setPw] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await post("/api/auth/stepup", { password: pw });
      resolve(true);
    } catch (x) {
      setErr(errText(x));
      setPw("");
      setBusy(false);
    }
  };
  return (
    <Modal title="Verifikasi ulang" onClose={() => resolve(false)}>
      <form onSubmit={submit} className="stack">
        <p className="small">Aksi ini penting. Masukkan kata sandi Anda. Setelah benar, aksi penting lain tidak bertanya lagi selama 15 menit.</p>
        <Field label="Kata sandi">{(id) => <input id={id} data-autofocus type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="current-password" maxLength={256} required />}</Field>
        {err && <Alert>{err}</Alert>}
        <div className="row-end">
          <button type="button" className="btn" onClick={() => resolve(false)}>
            Batal
          </button>
          <button className="btn btn-primary" disabled={busy || !pw}>
            {busy ? "Memeriksa…" : "Verifikasi"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

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
            <button className="btn btn-ghost btn-icon btn-sm" onClick={() => dismissJob(j.id)} aria-label={`Sembunyikan ${j.label}`}>
              <X size={16} aria-hidden="true" />
            </button>
          </div>
          {j.status !== "berjalan" && j.status !== "ok" && j.output && <pre className="tray-out">{j.output.slice(-600)}</pre>}
        </div>
      ))}
    </div>
  );
}

// ---------- kerangka ----------

interface NavItem {
  id: string;
  label: string;
  icon: LucideIcon;
}
const GROUPS: { label: string; items: NavItem[] }[] = [
  { label: "Gambaran", items: [{ id: "ringkasan", label: "Ringkasan", icon: LayoutDashboard }, { id: "pantau", label: "Pantau", icon: Activity }] },
  { label: "Kelola", items: [{ id: "website", label: "Website", icon: Globe }, { id: "aplikasi", label: "Aplikasi", icon: Server }, { id: "database", label: "Database", icon: Database }, { id: "runtime", label: "PHP dan runtime", icon: Boxes }, { id: "tugas", label: "Worker dan cron", icon: Timer }] },
  { label: "Sistem", items: [{ id: "sistem", label: "Sistem", icon: ShieldCheck }, { id: "backup", label: "Backup", icon: DatabaseBackup }, { id: "pembaruan", label: "Pembaruan", icon: Download }, { id: "aktivitas", label: "Aktivitas", icon: History }, { id: "pengaturan", label: "Pengaturan", icon: SettingsIcon }, { id: "dokumen", label: "Dokumentasi", icon: BookOpen }] },
];
const ALL = GROUPS.flatMap((g) => g.items);

function parseHash(): { page: string; sub: string } {
  const [page, sub = ""] = location.hash.replace(/^#\/?/, "").split("/");
  return { page: ALL.some((n) => n.id === page) ? page : "ringkasan", sub: decodeURIComponent(sub) };
}

function Shell({ user, role, onLogout }: { user: string; role: string; onLogout: () => void }) {
  const panelName = usePanelName();
  const theme = useTheme();
  const alerts = useAlerts();
  const nAlerts = alerts.data?.alerts.length ?? 0;
  const nBad = alerts.data?.alerts.filter((a) => a.level === "bad").length ?? 0;
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
  useEffect(() => localStorage.setItem("panel-collapsed", collapsed ? "1" : "0"), [collapsed]);

  const ThemeIcon = theme.pref === "dark" ? Moon : theme.pref === "light" ? Sun : Monitor;
  const themeLabel = theme.pref === "dark" ? "Gelap" : theme.pref === "light" ? "Terang" : "Ikuti sistem";
  const page = (() => {
    switch (route.page) {
      case "pantau": return <Monitoring />;
      case "website": return <Sites sub={route.sub} />;
      case "aplikasi": return <Apps />;
      case "database": return <Databases />;
      case "runtime": return <Runtimes />;
      case "tugas": return <Tasks />;
      case "sistem": return <System />;
      case "backup": return <Backups />;
      case "pembaruan": return <Updates />;
      case "aktivitas": return <ActivityPage />;
      case "pengaturan": return <Settings />;
      case "dokumen": return <Docs sub={route.sub} />;
      default: return <Overview />;
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
            <span className="brand-text">{panelName}</span>
          </span>
          <button className="btn btn-ghost btn-icon sheet-close" onClick={() => setSheet(false)} aria-label="Tutup menu">
            <X size={18} aria-hidden="true" />
          </button>
        </div>
        {GROUPS.map((g) => (
          <div key={g.label} className="side-group">
            <p className="side-label">{g.label}</p>
            <ul>
              {g.items.map((n) => (
                <li key={n.id}>
                  <a href={`#/${n.id}`} className={route.page === n.id ? "on" : ""} aria-current={route.page === n.id ? "page" : undefined}>
                    <n.icon size={20} aria-hidden="true" />
                    <span className="side-text">{n.label}</span>
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
        <div className="side-bottom">
          <button className="side-collapse" onClick={() => setCollapsed((c) => !c)} aria-expanded={!collapsed} aria-label={collapsed ? "Perlebar menu" : "Ciutkan menu"}>
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
          <span className="top-info">
            Masuk sebagai <strong className="mono">{user}</strong> {role === "viewer" && <Chip tone="info">Hanya-baca</Chip>}
          </span>
          <div className="top-actions">
            {nAlerts > 0 && (
              <a className={`btn ${nBad ? "btn-danger-ghost" : ""}`} href="#/ringkasan" aria-label={`${nAlerts} peringatan${nBad ? `, ${nBad} penting` : ""}. Buka Ringkasan`}>
                <Bell size={18} aria-hidden="true" />
                <span className="btn-text">{nAlerts} peringatan</span>
              </a>
            )}
            <button className="btn" onClick={theme.next} aria-label={`Tema: ${themeLabel}. Klik untuk mengganti`}>
              <ThemeIcon size={18} aria-hidden="true" />
              <span className="btn-text">{themeLabel}</span>
            </button>
            <button className="btn" onClick={onLogout} aria-label="Keluar">
              <LogOut size={18} aria-hidden="true" />
              <span className="btn-text">Keluar</span>
            </button>
          </div>
        </header>
        <main id="main" className="main" tabIndex={-1}>
          <ErrorBoundary resetKey={`${route.page}/${route.sub ?? ""}`}>{page}</ErrorBoundary>
        </main>
      </div>
      <button className="scrim-side" aria-label="Tutup menu" tabIndex={-1} onClick={() => setSheet(false)} />
      <JobTray />
    </div>
  );
}

export default function App() {
  const [user, setUser] = useState<string | null | undefined>(undefined);
  const [role, setRoleState] = useState("admin");
  const [stepUp, setStepUp] = useState<null | ((ok: boolean) => void)>(null);
  useEffect(() => {
    setStepUpHandler(() => new Promise<boolean>((resolve) => setStepUp(() => (ok: boolean) => { setStepUp(null); resolve(ok); })));
    const un = () => setUser(null);
    addEventListener(UNAUTH, un);
    get<{ user: string; csrf: string; role?: string }>("/api/me")
      .then((r) => { setCsrf(r.csrf); setRole(r.role ?? "admin"); setRoleState(r.role ?? "admin"); setUser(r.user); })
      .catch(() => setUser(null));
    return () => removeEventListener(UNAUTH, un);
  }, []);
  const logout = useCallback(async () => {
    try { await api("POST", "/api/auth/logout", {}); } catch { /* sesi mungkin sudah habis */ }
    setUser(null);
  }, []);
  if (user === undefined)
    return (
      <main className="login" id="main">
        <p className="muted" role="status">Memuat…</p>
      </main>
    );
  return (
    <>
      {user ? <Shell user={user} role={role} onLogout={logout} /> : <Login onDone={(u) => { get<{ role?: string }>("/api/me").then((r) => { setRole(r.role ?? "admin"); setRoleState(r.role ?? "admin"); }).catch(() => {}); setUser(u); }} />}
      {stepUp && <StepUpDialog resolve={stepUp} />}
    </>
  );
}
