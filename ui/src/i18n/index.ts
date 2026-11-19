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

let mem: string | null = null; // cadangan bila penyimpanan terblokir
const subs = new Set<() => void>();
export const isLang = (v: unknown): v is string => typeof v === "string" && Object.prototype.hasOwnProperty.call(LOCALES, v);

/** Urutan: pilihan tersimpan di browser (diselaraskan dengan akun saat masuk) > bahasa browser > bawaan. */
export function currentLang(): string {
  let s: string | null = null;
  try {
    s = localStorage.getItem(KEY);
  } catch {
    /* penyimpanan terblokir (mode privat) */
  }
  if (!s) s = mem;
  if (isLang(s)) return s;
  const nav = (typeof navigator === "undefined" ? "" : navigator.language || "").slice(0, 2).toLowerCase();
  return isLang(nav) ? nav : DEFAULT_LANG;
}

export function setLang(l: string) {
  if (!isLang(l)) return;
  mem = l;
  try {
    localStorage.setItem(KEY, l);
  } catch {
    /* tetap berlaku untuk sesi ini */
  }
  if (typeof document !== "undefined") document.documentElement.lang = l;
  subs.forEach((f) => f());
}

export function format(msg: string, vars?: Record<string, string | number>): string {
  return vars ? msg.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m)) : msg;
}

/** Jamak: bila vars.n angka dan ada kunci "<key>.one", bentuk itu dipakai saat Intl.PluralRules(bahasa) menyebut "one". Bahasa lain boleh menambah ".few"/".many". */
export function translate(lang: string, key: Key, vars?: Record<string, string | number>): string {
  const msgs = LOCALES[lang]?.messages ?? LOCALES[DEFAULT_LANG].messages;
  let k: string = key;
  const n = vars?.n;
  if (typeof n === "number") {
    try {
      const form = new Intl.PluralRules(lang).select(n);
      if (form !== "other" && `${key}.${form}` in msgs) k = `${key}.${form}`;
    } catch {
      /* kode bahasa tak dikenal Intl: pakai bentuk dasar */
    }
  }
  const m = (msgs as Record<string, string>)[k] ?? (LOCALES[DEFAULT_LANG].messages as Record<string, string>)[k] ?? key;
  return format(m, vars);
}

export const t = (key: Key, vars?: Record<string, string | number>) => translate(currentLang(), key, vars);

/** Hook: komponen dirender ulang saat bahasa berganti. */
export function useT() {
  const lang = useSyncExternalStore((f) => (subs.add(f), () => void subs.delete(f)), currentLang);
  return (key: Key, vars?: Record<string, string | number>) => translate(lang, key, vars);
}
export const useLang = () => useSyncExternalStore((f) => (subs.add(f), () => void subs.delete(f)), currentLang);

/** Tag locale untuk Intl/tanggal. id->id-ID, en->en-US; bahasa lain memakai kodenya sendiri. */
export const localeTag = () => ({ id: "id-ID", en: "en-US" } as Record<string, string>)[currentLang()] ?? currentLang();
/** Pemisah desimal bahasa saat ini (koma untuk id, titik untuk en). */
export const decimalSep = () => (1.5).toLocaleString(localeTag()).charAt(1);
