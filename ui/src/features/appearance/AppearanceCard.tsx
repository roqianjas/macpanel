import { useRef, useState } from "react";
import { fileToLogo, LOGOS, setLogo, useLogo } from "./logo";
import { Check, ImageUp, Palette, RotateCcw } from "lucide-react";
import { Card } from "../../ui";
import { useT } from "../../i18n";
import { setAccent, setDevicePref, setSkin, useAccent, useSkin, useThemePref, type Pref } from "../../settings";
import { ACCENTS, isHex, SKINS, type Mode, type Skin } from "../../skins";

// Ikon: Palette = tampilan (relevan: pemilih gaya dan warna), Check = pilihan aktif, RotateCcw = kembali ke bawaan gaya.

const MODES: { id: Pref; key: string }[] = [
  { id: "system", key: "theme.system" },
  { id: "light", key: "theme.light" },
  { id: "dark", key: "theme.dark" },
];

function Swatch({ skin, mode }: { skin: Skin; mode: Mode }) {
  const s = SKINS.find((x) => x.id === skin)!.preview[mode];
  // Pratinjau kecil dari warna asli gaya (bukan gambar): latar, kartu, dua garis teks, dan satu tombol aksen.
  return (
    <span className="skin-prev" style={{ background: s.bg, borderColor: s.ink }} aria-hidden="true">
      <span className="skin-prev-card" style={{ background: s.surface, borderColor: s.ink }}>
        <i style={{ background: s.ink }} />
        <i style={{ background: s.ink, width: "55%", opacity: 0.45 }} />
        <b style={{ background: s.accent }} />
      </span>
    </span>
  );
}

export default function AppearanceCard() {
  const t = useT();
  const skin = useSkin();
  const accent = useAccent();
  const pref = useThemePref();
  const [custom, setCustom] = useState(accent ?? "#0f766e");
  const logo = useLogo();
  const file = useRef<HTMLInputElement>(null);
  const [logoErr, setLogoErr] = useState("");
  const dark = typeof document !== "undefined" && document.documentElement.getAttribute("data-theme") === "dark";
  const mode: Mode = dark ? "dark" : "light";

  return (
    <Card title={<span className="with-icon"><Palette aria-hidden="true" size={18} /> {t("settings.look.title")}</span>}>
      <div className="stack-l">
        <div className="stack">
          <span className="field-label" id="skin-label">{t("settings.look.skin")}</span>
          <ul className="skin-grid" role="group" aria-labelledby="skin-label">
            {SKINS.map((s) => (
              <li key={s.id}>
                <button type="button" className={`skin-opt ${skin === s.id ? "on" : ""}`} aria-pressed={skin === s.id} onClick={() => setSkin(s.id)}>
                  <Swatch skin={s.id} mode={mode} />
                  <span className="skin-name">
                    {s.label}
                    {skin === s.id && <Check size={16} aria-hidden="true" />}
                  </span>
                  <span className="small muted">{s.desc}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>

        <div className="stack">
          <span className="field-label" id="accent-label">{t("settings.look.accent")}</span>
          <div className="accent-row" role="group" aria-labelledby="accent-label">
            {ACCENTS.map((a) => (
              <button key={a.id} type="button" className={`accent-dot ${accent === a.hex ? "on" : ""}`} style={{ background: a.hex }} aria-label={a.label} aria-pressed={accent === a.hex} title={a.label} onClick={() => { setAccent(a.hex); setCustom(a.hex); }} />
            ))}
            <label className="accent-custom">
              <span className="sr">{t("settings.look.customSr")}</span>
              <input type="color" value={isHex(custom) ? custom : "#0f766e"} onChange={(e) => { setCustom(e.target.value); setAccent(e.target.value); }} aria-label={t("settings.look.customAria")} />
            </label>
            <button type="button" className="btn btn-sm" disabled={accent === null} onClick={() => setAccent(null)}>
              <RotateCcw size={16} aria-hidden="true" /> {t("settings.look.accentReset")}
            </button>
          </div>
          <p className="small muted">
            {accent === null ? t("settings.look.accentDefault") : t("settings.look.accentIs", { accent })} {t("settings.look.accentNote")}
          </p>
        </div>

        <div className="stack">
          <span className="field-label" id="logo-label">{t("settings.look.logo")}</span>
          <div className="logo-row" role="group" aria-labelledby="logo-label">
            {LOGOS.map(({ id, label, Icon }) => (
              <button key={id} type="button" className={`logo-opt ${logo.kind === "icon" && logo.id === id ? "on" : ""}`} aria-label={label} title={label} aria-pressed={logo.kind === "icon" && logo.id === id} onClick={() => { setLogoErr(""); setLogo({ kind: "icon", id }); }}>
                <Icon size={20} aria-hidden="true" />
              </button>
            ))}
            <button type="button" className={`logo-opt ${logo.kind === "img" ? "on" : ""}`} aria-label={t("settings.look.upload")} title={t("settings.look.upload")} aria-pressed={logo.kind === "img"} onClick={() => file.current?.click()}>
              {logo.kind === "img" ? <img src={logo.src} width={28} height={28} alt="" /> : <ImageUp size={20} aria-hidden="true" />}
            </button>
            <input ref={file} type="file" accept="image/png,image/jpeg,image/webp" className="sr" tabIndex={-1} aria-hidden="true" onChange={async (e) => {
              const f = e.target.files?.[0]; e.target.value = "";
              if (!f) return;
              try { setLogoErr(""); setLogo(await fileToLogo(f)); } catch (x) { setLogoErr(x instanceof Error ? x.message : t("settings.look.imgFail")); }
            }} />
          </div>
          {logoErr && <p className="small field-err" role="alert">{logoErr}</p>}
          <p className="small muted">{t("settings.look.logoNote")}</p>
        </div>

        <div className="stack">
          <span className="field-label" id="mode-label">{t("settings.look.mode")}</span>
          <div className="seg" role="group" aria-labelledby="mode-label">
            {MODES.map((m) => (
              <button key={m.id} type="button" className={`seg-btn ${pref === m.id ? "on" : ""}`} aria-pressed={pref === m.id} onClick={() => setDevicePref(m.id)}>
                {t(m.key)}
              </button>
            ))}
          </div>
          <p className="small muted">{t("settings.look.saved")}</p>
        </div>
      </div>
    </Card>
  );
}
