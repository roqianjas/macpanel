import { useEffect, useSyncExternalStore } from "react";
import { Atom, Boxes, Cloud, Command, Cpu, Flame, Gem, Hexagon, KeyRound, Layers, Leaf, Orbit, Rocket, Server, ShieldCheck, Terminal, Zap, type LucideIcon } from "lucide-react";

// Logo panel: ikon bawaan (mengikuti warna aksen) atau gambar sendiri. Disimpan per browser, sama seperti gaya dan aksen.
// Ikon lain dipilih karena relevan dengan server/ops; KeyRound tetap bawaan (kunci = akses).
export const LOGOS: { id: string; label: string; Icon: LucideIcon }[] = [
  { id: "key", label: "Kunci", Icon: KeyRound },
  { id: "server", label: "Server", Icon: Server },
  { id: "terminal", label: "Terminal", Icon: Terminal },
  { id: "cpu", label: "Prosesor", Icon: Cpu },
  { id: "rocket", label: "Roket", Icon: Rocket },
  { id: "shield", label: "Perisai", Icon: ShieldCheck },
  { id: "zap", label: "Petir", Icon: Zap },
  { id: "boxes", label: "Kotak", Icon: Boxes },
  { id: "cloud", label: "Awan", Icon: Cloud },
  { id: "hexagon", label: "Segi enam", Icon: Hexagon },
  { id: "layers", label: "Lapisan", Icon: Layers },
  { id: "command", label: "Command", Icon: Command },
  { id: "atom", label: "Atom", Icon: Atom },
  { id: "orbit", label: "Orbit", Icon: Orbit },
  { id: "flame", label: "Api", Icon: Flame },
  { id: "leaf", label: "Daun", Icon: Leaf },
  { id: "gem", label: "Permata", Icon: Gem },
];

const KEY = "panel-logo";
export const MAX_IMG_BYTES = 2 * 1024 * 1024;
export const IMG_TYPES = ["image/png", "image/jpeg", "image/webp"];

export type Logo = { kind: "icon"; id: string } | { kind: "img"; src: string };

const subs = new Set<() => void>();
const emit = () => subs.forEach((f) => f());
const subscribe = (f: () => void) => (subs.add(f), () => void subs.delete(f));

/** Hanya data:image/png yang kita buat sendiri yang diterima kembali dari penyimpanan (bukan URL sembarang). */
export const isImgSrc = (v: unknown): v is string => typeof v === "string" && /^data:image\/png;base64,[A-Za-z0-9+/=]{16,200000}$/.test(v);

export function parseLogo(raw: string | null): Logo {
  if (raw?.startsWith("img:") && isImgSrc(raw.slice(4))) return { kind: "img", src: raw.slice(4) };
  const id = raw?.startsWith("icon:") ? raw.slice(5) : "key";
  return { kind: "icon", id: LOGOS.some((l) => l.id === id) ? id : "key" };
}

let cache: { raw: string | null; logo: Logo } | null = null;
export function currentLogo(): Logo {
  const raw = localStorage.getItem(KEY);
  if (!cache || cache.raw !== raw) cache = { raw, logo: parseLogo(raw) }; // identitas stabil untuk useSyncExternalStore
  return cache.logo;
}

export function setLogo(l: Logo | null) {
  if (!l) localStorage.removeItem(KEY);
  else localStorage.setItem(KEY, l.kind === "icon" ? `icon:${l.id}` : `img:${l.src}`);
  emit();
}

export const useLogo = () => useSyncExternalStore(subscribe, currentLogo);

/** Gambar unggahan: dipotong persegi dari tengah, diperkecil 128 px, disimpan sebagai PNG baru (metadata asli dan isi aktif tidak ikut). */
export async function fileToLogo(file: File, S = 128): Promise<Logo> {
  if (!IMG_TYPES.includes(file.type)) throw new Error("Format harus PNG, JPG, atau WebP. SVG ditolak karena bisa memuat skrip.");
  if (file.size > MAX_IMG_BYTES) throw new Error("Ukuran gambar maksimal 2 MB.");
  const bmp = await createImageBitmap(file).catch(() => null);
  if (!bmp) throw new Error("Gambar tidak bisa dibaca.");
  const c = document.createElement("canvas");
  c.width = c.height = S;
  const g = c.getContext("2d");
  if (!g) throw new Error("Browser tidak mendukung canvas.");
  const side = Math.min(bmp.width, bmp.height);
  g.drawImage(bmp, (bmp.width - side) / 2, (bmp.height - side) / 2, side, side, 0, 0, S, S);
  bmp.close();
  const src = c.toDataURL("image/png");
  if (!isImgSrc(src)) throw new Error("Gambar terlalu rumit setelah diperkecil. Coba gambar lain.");
  return { kind: "img", src };
}

export function BrandMark({ size = 20 }: { size?: number }) {
  const l = useLogo();
  if (l.kind === "img") return <img className="brand-img" src={l.src} width={size + 8} height={size + 8} alt="" />;
  const I = LOGOS.find((x) => x.id === l.id)!.Icon;
  return <I size={size} aria-hidden="true" />;
}

function setFavicon(href: string, type: string) {
  let el = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!el) {
    el = document.createElement("link");
    el.rel = "icon";
    document.head.appendChild(el);
  }
  el.type = type;
  el.href = href;
}

/** Favicon mengikuti logo: gambar sendiri apa adanya; ikon digambar ulang dengan warna aksen aktif di atas kotak membulat. */
export function useFavicon(deps: string) {
  const l = useLogo();
  useEffect(() => {
    if (l.kind === "img") return setFavicon(l.src, "image/png");
    const css = getComputedStyle(document.documentElement);
    const bg = (css.getPropertyValue("--primary").trim() || "#0a57c8").replace(/[^#\w(),.% -]/g, "");
    const fg = (css.getPropertyValue("--on-accent").trim() || "#ffffff").replace(/[^#\w(),.% -]/g, "");
    const svg = document.querySelector(".side .brand-ic svg");
    if (!svg) return;
    const inner = svg.innerHTML;
    const out = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="8" fill="${bg}"/><g transform="translate(6 6) scale(.8333)" fill="none" stroke="${fg}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${inner}</g></svg>`;
    setFavicon(`data:image/svg+xml;charset=utf-8,${encodeURIComponent(out)}`, "image/svg+xml");
  }, [l, deps]);
}
