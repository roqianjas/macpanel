import { useEffect, useRef, useState } from "react";
import { decimalSep, localeTag, t } from "./i18n";

export type Pt = [number, number];

export const RANGES = [
  { key: "1h", get label() { return t("range.1h"); }, sec: 3600 },
  { key: "24h", get label() { return t("range.24h"); }, sec: 86400 },
  { key: "7d", get label() { return t("range.7d"); }, sec: 604800 },
] as const;
export type RangeKey = (typeof RANGES)[number]["key"];

export interface Geometry {
  line: string;
  area: string;
  xs: number[];
  ys: number[];
}

/** Ubah titik jadi path SVG. Domain y tetap [lo, hi]; x dari titik pertama ke terakhir. */
export function geometry(pts: Pt[], w: number, h: number, lo: number, hi: number, pad = { l: 0, r: 0, t: 6, b: 6 }): Geometry {
  const t0 = pts[0][0];
  const t1 = pts[pts.length - 1][0];
  const span = Math.max(1, t1 - t0);
  const iw = w - pad.l - pad.r;
  const ih = h - pad.t - pad.b;
  const xs = pts.map((p) => pad.l + ((p[0] - t0) / span) * iw);
  const ys = pts.map((p) => pad.t + ih - ((Math.min(hi, Math.max(lo, p[1])) - lo) / (hi - lo || 1)) * ih);
  const line = xs.map((x, i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${ys[i].toFixed(1)}`).join(" ");
  const area = `${line} L${xs[xs.length - 1].toFixed(1)} ${(pad.t + ih).toFixed(1)} L${xs[0].toFixed(1)} ${(pad.t + ih).toFixed(1)} Z`;
  return { line, area, xs, ys };
}

export interface Summary {
  min: number;
  max: number;
  last: number;
  n: number;
}
export function summarize(pts: Pt[]): Summary | null {
  if (pts.length === 0) return null;
  const vs = pts.map((p) => p[1]);
  return { min: Math.min(...vs), max: Math.max(...vs), last: vs[vs.length - 1], n: vs.length };
}

/** Batas atas sumbu berbentuk 1, 2, 5 x 10^n supaya empat interval bernilai bulat. */
export function niceCeil(v: number, floor = 1): number {
  const x = Math.max(v, floor);
  const p = Math.pow(10, Math.floor(Math.log10(x)));
  const f = x / p;
  const n = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10;
  return n * p;
}

/** Batas atas sumbu = 4 x langkah bulat (1, 2, atau 5 x 10^n) dalam satuan `unit`, supaya keempat interval berlabel bulat. */
export function niceTop(v: number, floor: number, unit = 1): number {
  const x = Math.max(v, floor) / unit;
  const raw = x / 4;
  const p = Math.pow(10, Math.floor(Math.log10(raw)));
  const m = raw / p;
  const step = (m <= 1 ? 1 : m <= 2 ? 2 : m <= 5 ? 5 : 10) * p;
  return 4 * step * unit;
}

/** Satuan byte terbesar (1000^k) yang masih <= v, untuk memilih langkah sumbu bandwidth. */
export function byteUnit(v: number): number {
  let u = 1;
  while (u * 1000 <= v && u < 1000 ** 4) u *= 1000;
  return u;
}

export function fmtClock(t: number, withDate: boolean): string {
  const d = new Date(t * 1000);
  const hm = d.toLocaleTimeString(localeTag(), { hour: "2-digit", minute: "2-digit" }).replace(".", ":");
  return withDate ? `${d.toLocaleDateString(localeTag(), { day: "numeric", month: "short" })} ${hm}` : hm;
}

export function fmtSpan(sec: number): string {
  if (sec < 90) return t("fmt.sec", { n: Math.round(sec) });
  if (sec < 5400) return t("fmt.min", { n: Math.round(sec / 60) });
  if (sec < 172800) return t("fmt.hour", { n: (sec / 3600).toFixed(1).replace(".", decimalSep()) });
  return t("fmt.day", { n: (sec / 86400).toFixed(1).replace(".", decimalSep()) });
}

export function fmtUptime(s: number | null): string {
  if (s == null || s < 0) return t("fmt.unknown");
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return t("fmt.dayhour", { d, h });
  if (h > 0) return t("fmt.hourmin", { h, m });
  return t("fmt.min", { n: m });
}

export function fmtAgo(sec: number): string {
  const s = Math.max(0, Math.floor(sec));
  const span = s < 60 ? t("fmt.sec", { n: s }) : s < 3600 ? t("fmt.min", { n: Math.floor(s / 60) }) : s < 86400 ? t("fmt.hour", { n: Math.floor(s / 3600) }) : t("fmt.day", { n: Math.floor(s / 86400) });
  return t("fmt.ago", { span });
}

const num = (v: number, d: number) => v.toFixed(d).replace(".", decimalSep());

function fmtScaled(b: number | null | undefined, base: number): string {
  if (b == null || !isFinite(b)) return "n/a";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let v = Math.max(0, b);
  let i = 0;
  while (v >= base && i < u.length - 1) {
    v /= base;
    i++;
  }
  return `${num(v, i === 0 || v >= 100 ? 0 : v >= 10 ? 1 : 2).replace(new RegExp(`\\${decimalSep()}(\\d*?)0+$`), (_m, d: string) => (d ? `${decimalSep()}${d}` : ""))} ${u[i]}`;
}

/** Ukuran penyimpanan, jaringan, dan berkas: basis 1000 seperti Finder dan Pengaturan Sistem macOS. */
export function fmtBytes(b: number | null | undefined): string {
  return fmtScaled(b, 1000);
}

/** Ukuran memori (RAM, RSS): basis 1024 dengan label GB/MB seperti Tentang Mac Ini dan Activity Monitor. */
export function fmtMem(b: number | null | undefined): string {
  return fmtScaled(b, 1024);
}

/** Laju dalam byte per detik. */
export function fmtRate(bps: number | null | undefined): string {
  if (bps == null || !isFinite(bps)) return "n/a";
  return `${fmtBytes(bps)}/s`;
}

export function prefersReducedMotion(): boolean {
  return typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/** Angka naik dari 0 ke target. Langsung ke target bila gerak dikurangi; nilai akhir tidak bergantung pada rAF. */
export function useCountUp(target: number | null, ms = 500): number | null {
  const [v, setV] = useState<number | null>(() => (target == null || prefersReducedMotion() ? target : 0));
  const prev = useRef<number | null>(null);
  useEffect(() => {
    if (target == null) {
      setV(null);
      return;
    }
    if (prefersReducedMotion()) {
      setV(target);
      prev.current = target;
      return;
    }
    const from = prev.current ?? 0;
    const t0 = performance.now();
    let raf = 0;
    const step = (t: number) => {
      const k = Math.min(1, (t - t0) / ms);
      const e = 1 - Math.pow(1 - k, 3);
      setV(from + (target - from) * e);
      if (k < 1) raf = requestAnimationFrame(step);
      else prev.current = target;
    };
    raf = requestAnimationFrame(step);
    const done = setTimeout(() => {
      setV(target);
      prev.current = target;
    }, ms + 150);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(done);
    };
  }, [target, ms]);
  return v;
}
