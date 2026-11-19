import { t } from "./i18n";
// Gaya (skin) panel dan warna aksen. Murni (tanpa React/DOM) agar bisa diuji langsung.
//
// Gaya mengganti token warna, huruf, bentuk garis, dan bayangan lewat atribut data-skin di <html>. Tata letak dan fungsi tidak berubah.
// Warna aksen pilihan pengguna diubah menjadi token yang AMAN KONTRAS: apa pun warnanya, teks di atas tombol aksen dan teks aksen di atas
// latar gaya terpilih selalu mencapai 4,5:1 (WCAG AA). Warna diubah terang/gelapnya sampai lolos, bukan ditolak.

export type Skin = "kertas" | "bengkel" | "mesin" | "kaca";
export type Mode = "light" | "dark";

export interface SkinInfo {
  id: Skin;
  label: string;
  desc: string;
  /** Warna contoh untuk pratinjau kartu pilihan (tiap mode). Harus sama dengan styles.css (dijaga tes). */
  preview: Record<Mode, { bg: string; surface: string; ink: string; accent: string }>;
}

export const DEFAULT_SKIN: Skin = "kertas";

export const SKINS: SkinInfo[] = [
  {
    id: "kertas",
    label: "Kertas Teknis",
    get desc() {
      return t("settings.skin.kertas");
    },
    preview: {
      light: { bg: "#f6f1e7", surface: "#fffaf0", ink: "#1c1a16", accent: "#c2410c" },
      dark: { bg: "#1b1813", surface: "#232018", ink: "#efe7d6", accent: "#fb923c" },
    },
  },
  {
    id: "bengkel",
    label: "Bengkel",
    get desc() {
      return t("settings.skin.bengkel");
    },
    preview: {
      light: { bg: "#fffdf5", surface: "#ffffff", ink: "#111111", accent: "#c6f432" },
      dark: { bg: "#14140f", surface: "#1e1e17", ink: "#f5f3e6", accent: "#c6f432" },
    },
  },
  {
    id: "mesin",
    label: "Ruang Mesin",
    get desc() {
      return t("settings.skin.mesin");
    },
    preview: {
      light: { bg: "#efeae0", surface: "#faf7f0", ink: "#25211a", accent: "#b45309" },
      dark: { bg: "#15130f", surface: "#1c1913", ink: "#ece6d8", accent: "#f2b13c" },
    },
  },
  {
    id: "kaca",
    label: "Kaca Dingin",
    get desc() {
      return t("settings.skin.kaca");
    },
    preview: {
      light: { bg: "#eef3f1", surface: "#ffffff", ink: "#10302c", accent: "#0f766e" },
      dark: { bg: "#0c1a18", surface: "#132624", ink: "#e3f2ef", accent: "#2dd4bf" },
    },
  },
];

export const isSkin = (v: unknown): v is Skin => SKINS.some((s) => s.id === v);

export const ACCENTS: { id: string; label: string; hex: string }[] = [
  { id: "jingga", get label() { return t("settings.accent.jingga"); }, hex: "#c2410c" },
  { id: "limau", get label() { return t("settings.accent.limau"); }, hex: "#c6f432" },
  { id: "ambar", get label() { return t("settings.accent.ambar"); }, hex: "#f2b13c" },
  { id: "teal", get label() { return t("settings.accent.teal"); }, hex: "#0f766e" },
  { id: "biru", get label() { return t("settings.accent.biru"); }, hex: "#1d4ed8" },
  { id: "hijau", get label() { return t("settings.accent.hijau"); }, hex: "#15803d" },
  { id: "magenta", get label() { return t("settings.accent.magenta"); }, hex: "#be185d" },
  { id: "grafit", get label() { return t("settings.accent.grafit"); }, hex: "#374151" },
];

// ---------------------------------------------------------------- warna

const HEX = /^#[0-9a-f]{6}$/i;
export const isHex = (v: unknown): v is string => typeof v === "string" && HEX.test(v);
export const normHex = (v: string) => v.toLowerCase();

type RGB = [number, number, number];

export function toRgb(hex: string): RGB {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

export function toHex([r, g, b]: RGB): string {
  const c = (x: number) => Math.max(0, Math.min(255, Math.round(x))).toString(16).padStart(2, "0");
  return `#${c(r)}${c(g)}${c(b)}`;
}

export function luminance(hex: string): number {
  const f = (v: number) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  };
  const [r, g, b] = toRgb(hex);
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
}

export function contrast(a: string, b: string): number {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** Campur dua warna: t = bagian `b` (0..1). */
export function mix(a: string, b: string, t: number): string {
  const x = toRgb(a);
  const y = toRgb(b);
  return toHex([x[0] + (y[0] - x[0]) * t, x[1] + (y[1] - x[1]) * t, x[2] + (y[2] - x[2]) * t]);
}

function toHsl(hex: string): [number, number, number] {
  const [r, g, b] = toRgb(hex).map((v) => v / 255) as RGB;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;
  if (max === min) return [0, 0, l];
  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  const h = max === r ? (g - b) / d + (g < b ? 6 : 0) : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
  return [h * 60, s, l];
}

function fromHsl(h: number, s: number, l: number): string {
  const k = (n: number) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return toHex([f(0) * 255, f(8) * 255, f(4) * 255]);
}

/** Warna paling mirip `hex` yang kontrasnya >= 4,5 terhadap semua latar. Mode gelap: dicerahkan; terang: digelapkan. */
export function readableOn(hex: string, backs: string[], dark: boolean): string {
  const ok = (c: string) => backs.every((b) => contrast(c, b) >= 4.5);
  if (ok(hex)) return hex;
  const [h, s, l] = toHsl(hex);
  for (let i = 1; i <= 50; i++) {
    const c = fromHsl(h, s, Math.max(0, Math.min(1, l + (dark ? 0.02 : -0.02) * i)));
    if (ok(c)) return c;
  }
  return dark ? "#ffffff" : "#111111";
}

export interface AccentTokens {
  accent: string;
  onAccent: string;
  primary: string;
  tint: string;
}

/** Token aksen aman kontras untuk satu latar gaya. `bg` dan `surface` adalah warna latar halaman dan kartu gaya terpilih. */
export function accentTokens(hex: string, bg: string, surface: string, dark: boolean): AccentTokens {
  const accent = normHex(hex);
  // Tinta lembut dulu; untuk warna tengah (luminans ~0,18) tinta lembut dan putih sama-sama di bawah 4,5, jadi hitam murni (>= 4,58 untuk warna apa pun).
  const onAccent = (contrast(accent, "#111111") >= 4.5 ? "#111111" : contrast(accent, "#ffffff") >= 4.5 ? "#ffffff" : "#000000") as string;
  const tint = mix(surface, accent, dark ? 0.22 : 0.14);
  const primary = readableOn(accent, [bg, surface, tint], dark);
  return { accent, onAccent, primary, tint };
}

// ---------------------------------------------------------------- penerapan ke DOM

/** Variabel CSS yang diisi dari warna aksen pilihan. Dihapus bila pengguna memakai warna bawaan gaya. */
export function accentVars(t: AccentTokens): Record<string, string> {
  return { "--accent": t.accent, "--on-accent": t.onAccent, "--primary": t.primary, "--on-tint": t.primary, "--focus": t.primary, "--tint": t.tint };
}

export const ACCENT_VAR_NAMES = Object.keys(accentVars({ accent: "", onAccent: "", primary: "", tint: "" }));

/** Muat huruf gaya (dibundel lokal, bukan CDN). Hanya huruf gaya yang dipilih yang diunduh browser. */
export async function loadSkinFonts(skin: Skin): Promise<void> {
  try {
    if (skin === "kertas") {
      await Promise.all([import("@fontsource/fraunces/latin-600.css"), import("@fontsource/fraunces/latin-700.css")]);
    } else if (skin === "bengkel") {
      await Promise.all([import("@fontsource/archivo/latin-400.css"), import("@fontsource/archivo/latin-600.css"), import("@fontsource/archivo/latin-800.css")]);
    } else if (skin === "mesin") {
      await Promise.all([import("@fontsource/ibm-plex-sans/latin-400.css"), import("@fontsource/ibm-plex-sans/latin-600.css")]);
    }
  } catch {
    /* huruf cadangan sistem tetap dipakai */
  }
}
