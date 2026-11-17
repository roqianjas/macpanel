import { useSyncExternalStore } from "react";

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

export function applyTheme(p: Pref) {
  const dark = p === "dark" || (p === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  document.documentElement.setAttribute("data-pref", p);
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

export const useThemePref = () => useSyncExternalStore(subscribe, currentPref);
export const usePanelName = () => useSyncExternalStore(subscribe, () => panelName);
export const useServerTheme = () => useSyncExternalStore(subscribe, () => serverDefault);

matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
  if (currentPref() === "system") applyTheme("system");
});
applyTheme(currentPref());
loadPublicSettings();
