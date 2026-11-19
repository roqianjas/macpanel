import { useState } from "react";
import { Check, Palette, RotateCcw } from "lucide-react";
import { Card } from "../../ui";
import { setAccent, setDevicePref, setSkin, useAccent, useSkin, useThemePref, type Pref } from "../../settings";
import { ACCENTS, isHex, SKINS, type Mode, type Skin } from "../../skins";

// Ikon: Palette = tampilan (relevan: pemilih gaya dan warna), Check = pilihan aktif, RotateCcw = kembali ke bawaan gaya.

const MODES: { id: Pref; label: string }[] = [
  { id: "system", label: "Ikuti sistem" },
  { id: "light", label: "Terang" },
  { id: "dark", label: "Gelap" },
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
  const skin = useSkin();
  const accent = useAccent();
  const pref = useThemePref();
  const [custom, setCustom] = useState(accent ?? "#0f766e");
  const dark = typeof document !== "undefined" && document.documentElement.getAttribute("data-theme") === "dark";
  const mode: Mode = dark ? "dark" : "light";

  return (
    <Card title={<span className="with-icon"><Palette aria-hidden="true" size={18} /> Tampilan</span>}>
      <div className="stack-l">
        <div className="stack">
          <span className="field-label" id="skin-label">Gaya</span>
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
          <span className="field-label" id="accent-label">Warna aksen</span>
          <div className="accent-row" role="group" aria-labelledby="accent-label">
            {ACCENTS.map((a) => (
              <button key={a.id} type="button" className={`accent-dot ${accent === a.hex ? "on" : ""}`} style={{ background: a.hex }} aria-label={a.label} aria-pressed={accent === a.hex} title={a.label} onClick={() => { setAccent(a.hex); setCustom(a.hex); }} />
            ))}
            <label className="accent-custom">
              <span className="sr">Warna sendiri</span>
              <input type="color" value={isHex(custom) ? custom : "#0f766e"} onChange={(e) => { setCustom(e.target.value); setAccent(e.target.value); }} aria-label="Pilih warna aksen sendiri" />
            </label>
            <button type="button" className="btn btn-sm" disabled={accent === null} onClick={() => setAccent(null)}>
              <RotateCcw size={16} aria-hidden="true" /> Warna bawaan gaya
            </button>
          </div>
          <p className="small muted">
            {accent === null ? "Memakai warna bawaan gaya." : `Aksen ${accent}.`} Warna disesuaikan otomatis (lebih gelap atau lebih terang) supaya teks tetap terbaca di gaya dan mode yang dipilih.
          </p>
        </div>

        <div className="stack">
          <span className="field-label" id="mode-label">Terang atau gelap</span>
          <div className="seg" role="group" aria-labelledby="mode-label">
            {MODES.map((m) => (
              <button key={m.id} type="button" className={`seg-btn ${pref === m.id ? "on" : ""}`} aria-pressed={pref === m.id} onClick={() => setDevicePref(m.id)}>
                {m.label}
              </button>
            ))}
          </div>
          <p className="small muted">Gaya, aksen, dan mode disimpan di browser ini saja.</p>
        </div>
      </div>
    </Card>
  );
}
