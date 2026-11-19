import { useSyncExternalStore } from "react";
import id from "./locales/id";
import en from "./locales/en";

// Menambah bahasa: salin locales/id.ts menjadi locales/<kode>.ts, terjemahkan nilainya, lalu daftarkan satu baris di LOCALES.
// Tes i18n.test.ts menjaga setiap bahasa punya kunci yang sama dengan id (bahasa dasar) dan placeholder {x} yang sama.
export type Messages = Record<keyof typeof id, string>;
export const LOCALES: Record<string, { label: string; native: string; messages: Messages }> = {
  id: { label: "Indonesian", native: "Bahasa Indonesia", messages: id },
  en: { label: "English", native: "English", messages: en },
};
export const DEFAULT_LANG = "id";
const KEY = "panel-lang";
export type Key = keyof typeof id;

const subs = new Set<() => void>();
export const isLang = (v: unknown): v is string => typeof v === "string" && Object.prototype.hasOwnProperty.call(LOCALES, v);

/** Urutan: pilihan tersimpan di browser (diselaraskan dengan akun saat masuk) > bahasa browser > bawaan. */
export function currentLang(): string {
  const s = localStorage.getItem(KEY);
  if (isLang(s)) return s;
  const nav = (navigator.language || "").slice(0, 2).toLowerCase();
  return isLang(nav) ? nav : DEFAULT_LANG;
}

export function setLang(l: string) {
  if (!isLang(l)) return;
  localStorage.setItem(KEY, l);
  document.documentElement.lang = l;
  subs.forEach((f) => f());
}

export function format(msg: string, vars?: Record<string, string | number>): string {
  return vars ? msg.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m)) : msg;
}

export function translate(lang: string, key: Key, vars?: Record<string, string | number>): string {
  const m = LOCALES[lang]?.messages[key] ?? LOCALES[DEFAULT_LANG].messages[key] ?? key;
  return format(m, vars);
}

export const t = (key: Key, vars?: Record<string, string | number>) => translate(currentLang(), key, vars);

/** Hook: komponen dirender ulang saat bahasa berganti. */
export function useT() {
  const lang = useSyncExternalStore((f) => (subs.add(f), () => void subs.delete(f)), currentLang);
  return (key: Key, vars?: Record<string, string | number>) => translate(lang, key, vars);
}
export const useLang = () => useSyncExternalStore((f) => (subs.add(f), () => void subs.delete(f)), currentLang);
