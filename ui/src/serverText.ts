import { t } from "./i18n";
import { LOCALES, currentLang } from "./i18n";

/** Teks yang dibuat server: terjemahkan lewat kunci stabil (id layanan, kode peringatan); bila belum ada terjemahannya pakai teks server apa adanya. */
const has = (k: string) => k in (LOCALES[currentLang()]?.messages ?? {});

export const svcDesc = (id: string, fallback: string) => (has(`svc.desc.${id}`) ? t(`svc.desc.${id}`) : fallback);

export interface AlertLike { title: string; detail: string; code?: string; args?: Record<string, string | number> }
export const alertTitle = (a: AlertLike) => (a.code && has(`alert.${a.code}.t`) ? t(`alert.${a.code}.t`, a.args) : a.title);
export const alertDetail = (a: AlertLike) => (a.code && has(`alert.${a.code}.d`) ? t(`alert.${a.code}.d`, a.args) : a.detail);
