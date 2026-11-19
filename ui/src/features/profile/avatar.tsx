import { useSyncExternalStore } from "react";
import { Bot, Cat, Coffee, Dog, Fish, Gamepad2, Ghost, Music, Rabbit, Rocket, Smile, Star, User, type LucideIcon } from "lucide-react";

// Avatar bawaan: ikon dengan latar warna aksen. Gambar sendiri disimpan sebagai PNG kecil (data URL) di server, per akun.
export const AVATARS: { id: string; label: string; Icon: LucideIcon }[] = [
  { id: "user", label: "Orang", Icon: User }, { id: "smile", label: "Senyum", Icon: Smile }, { id: "cat", label: "Kucing", Icon: Cat }, { id: "dog", label: "Anjing", Icon: Dog },
  { id: "rabbit", label: "Kelinci", Icon: Rabbit }, { id: "fish", label: "Ikan", Icon: Fish }, { id: "bot", label: "Robot", Icon: Bot }, { id: "ghost", label: "Hantu", Icon: Ghost },
  { id: "rocket", label: "Roket", Icon: Rocket }, { id: "gamepad", label: "Gim", Icon: Gamepad2 }, { id: "music", label: "Musik", Icon: Music }, { id: "coffee", label: "Kopi", Icon: Coffee }, { id: "star", label: "Bintang", Icon: Star },
];

export const initials = (display: string, name: string) => {
  const w = (display || name).trim().split(/[\s._-]+/).filter(Boolean);
  return ((w[0]?.[0] ?? "?") + (w.length > 1 ? w[w.length - 1][0] : "")).toUpperCase();
};

/** Hanya PNG data kecil dan id ikon yang dikenal yang dirender; selain itu awalan nama. */
export const isPngAvatar = (v: string) => v.length <= 60000 && /^data:image\/png;base64,[A-Za-z0-9+/]+={0,2}$/.test(v);

export function Avatar({ name, display = "", avatar = "", size = 36 }: { name: string; display?: string; avatar?: string; size?: number }) {
  const style = { width: size, height: size, fontSize: Math.round(size * 0.4) };
  if (isPngAvatar(avatar)) return <img className="avatar" src={avatar} width={size} height={size} alt="" />;
  const ic = avatar.startsWith("icon:") ? AVATARS.find((a) => a.id === avatar.slice(5)) : undefined;
  if (ic) return <span className="avatar avatar-ic" style={style} aria-hidden="true"><ic.Icon size={Math.round(size * 0.55)} /></span>;
  return <span className="avatar avatar-ic" style={style} aria-hidden="true">{initials(display, name)}</span>;
}

export interface Me { user: string; display_name: string; avatar: string }
let me: Me = { user: "", display_name: "", avatar: "" };
const subs = new Set<() => void>();
export const setMe = (m: Partial<Me>) => { me = { ...me, ...m }; subs.forEach((f) => f()); };
export const useMe = () => useSyncExternalStore((f) => (subs.add(f), () => void subs.delete(f)), () => me);
