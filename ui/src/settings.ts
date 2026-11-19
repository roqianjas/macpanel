import { useSyncExternalStore } from "react";
import { ACCENT_VAR_NAMES, accentTokens, accentVars, DEFAULT_SKIN, isHex, isSkin, loadSkinFonts, normHex, SKINS, type Skin } from "./skins";

export type Pref = "system" | "light" | "dark";
const KEY = "panel-theme";
const subs = new Set<() => void>();
const emit = () => subs.forEach((f) => f());
const valid = (v: unknown): v is Pref => v === "system" || v === "light" || v === "dark";

let serverDefault: Pref = "system";
let panelName = "MacPanel";

function stored(): Pref | null {
  const v = localStorage.getItem(KEY);
  return valid(v) ? v : null;
}

export function currentPref(): Pref {
  return stored() ?? serverDefault;
}

const SKIN_KEY = "panel-skin";
const ACCENT_KEY = "panel-accent";

export function currentSkin(): Skin {
  const v = localStorage.getItem(SKIN_KEY);
  return isSkin(v) ? v : DEFAULT_SKIN;
}

/** Warna aksen pilihan perangkat ini, atau null = pakai aksen bawaan gaya. */
export function currentAccent(): string | null {
  const v = localStorage.getItem(ACCENT_KEY);
  return isHex(v) ? normHex(v) : null;
}

/** Terapkan gaya dan aksen ke <html>. Aksen dihitung ulang tiap gaya atau mode berubah supaya kontrasnya tetap aman. */
export function applyAppearance() {
  const el = document.documentElement;
  const skin = currentSkin();
  const dark = el.getAttribute("data-theme") === "dark";
  el.setAttribute("data-skin", skin);
  void loadSkinFonts(skin);
  const a = currentAccent();
  if (!a) {
    ACCENT_VAR_NAMES.forEach((n) => el.style.removeProperty(n));
    return;
  }
  const pv = SKINS.find((x) => x.id === skin)!.preview[dark ? "dark" : "light"];
  const vars = accentVars(accentTokens(a, pv.bg, pv.surface, dark));
  Object.entries(vars).forEach(([k, v]) => el.style.setProperty(k, v));
}

export function setSkin(s: Skin) {
  localStorage.setItem(SKIN_KEY, s);
  applyAppearance();
  emit();
}

export function setAccent(hex: string | null) {
  if (hex === null) localStorage.removeItem(ACCENT_KEY);
  else if (isHex(hex)) localStorage.setItem(ACCENT_KEY, normHex(hex));
  applyAppearance();
  emit();
}

export function applyTheme(p: Pref) {
  const dark = p === "dark" || (p === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  document.documentElement.setAttribute("data-pref", p);
  applyAppearance();
}

/** Pilihan tema perangkat ini (disimpan di browser). null = ikuti bawaan panel. */
export function setDevicePref(p: Pref | null) {
  if (p === null) localStorage.removeItem(KEY);
  else localStorage.setItem(KEY, p);
  applyTheme(currentPref());
  emit();
}

export function hasDevicePref(): boolean {
  return stored() !== null;
}

export function setPublicSettings(d: { name?: string; theme?: string }) {
  if (typeof d.name === "string" && d.name) {
    panelName = d.name;
    document.title = d.name;
  }
  if (valid(d.theme)) serverDefault = d.theme;
  applyTheme(currentPref());
  emit();
}

export async function loadPublicSettings() {
  try {
    const r = await fetch("/api/settings/public", { credentials: "same-origin" });
    if (r.ok) setPublicSettings(await r.json());
  } catch {
    /* tetap pakai bawaan */
  }
}

const subscribe = (f: () => void) => {
  subs.add(f);
  return () => {
    subs.delete(f);
  };
};

export const useSkin = () => useSyncExternalStore(subscribe, currentSkin);
export const useAccent = () => useSyncExternalStore(subscribe, currentAccent);
export const useThemePref = () => useSyncExternalStore(subscribe, currentPref);
export const usePanelName = () => useSyncExternalStore(subscribe, () => panelName);
export const useServerTheme = () => useSyncExternalStore(subscribe, () => serverDefault);

matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
  if (currentPref() === "system") applyTheme("system");
});
applyTheme(currentPref());
loadPublicSettings();
